package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

// handleMessages runs the control plane for POST /v1/messages: budget check,
// model routing, cache lookup — then proxies. Every internal failure falls
// through to plain proxying (fail-open).
func (g *Gateway) handleMessages(w http.ResponseWriter, r *http.Request) {
	cfg := g.ctl.Current()
	info := &reqInfo{start: time.Now()}

	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
	r.Body.Close()
	if err != nil {
		log.Printf("read body failed: %v", err)
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "gateway: unreadable request body")
		return
	}

	// Budget.
	dayS, weekS, monthS := control.PeriodStarts(time.Now())
	day, err1 := g.store.SpendSince(dayS)
	week, err2 := g.store.SpendSince(weekS)
	month, err3 := g.store.SpendSince(monthS)
	if err1 == nil && err2 == nil && err3 == nil {
		switch st := control.EvaluateBudget(cfg.Budget, day, week, month); st.Verdict {
		case control.VerdictBlock:
			log.Printf("budget block: %s", st.Reason)
			writeAnthropicError(w, http.StatusTooManyRequests, "rate_limit_error", "ai-gateway: "+st.Reason)
			return
		case control.VerdictWarn:
			log.Printf("budget warn: %s", st.Reason)
		}
	} else {
		log.Printf("budget check skipped (store error): %v %v %v", err1, err2, err3)
	}

	// Routing: rewrite the model field, preserving all other fields verbatim.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err == nil {
		var model string
		_ = json.Unmarshal(fields["model"], &model)
		if model != "" {
			to, blocked := cfg.Routing.Route(model)
			if blocked {
				log.Printf("model blocked: %s", model)
				writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error",
					"ai-gateway: model "+model+" is blocked by routing config")
				return
			}
			if to != model {
				raw, err := json.Marshal(to)
				if err == nil {
					fields["model"] = raw
					if nb, err := json.Marshal(fields); err == nil {
						body = nb
						info.routedFrom = model
						log.Printf("routed model %s -> %s", model, to)
					}
				}
			}
		}
	}

	// Cache.
	if cfg.Cache.Enabled && cfg.Cache.TTL > 0 {
		key := control.CacheKey(body)
		if hit, ok, err := g.store.CacheGet(key, cfg.Cache.TTL); err == nil && ok {
			g.serveCached(w, hit, info)
			return
		} else if err != nil {
			log.Printf("cache get failed: %v", err)
		}
		info.cacheKey = key // miss: capture the response for next time
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Del("Content-Length")
	g.proxy.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), infoKey, info)))
}

// serveCached replays a stored response and logs a zero-cost row with savings.
// SSE bodies replay as one write — fine for a cache hit.
func (g *Gateway) serveCached(w http.ResponseWriter, hit *store.CachedResponse, info *reqInfo) {
	w.Header().Set("Content-Type", hit.ContentType)
	w.WriteHeader(hit.Status)
	_, _ = w.Write(hit.Body)
	rec := store.Record{
		TS: time.Now(), Model: hit.Model, CostUSD: 0,
		LatencyMS: time.Since(info.start).Milliseconds(), Status: hit.Status,
		CacheHit: true, SavedUSD: hit.CostUSD,
	}
	if err := g.store.Insert(rec); err != nil {
		log.Printf("store insert (cache hit) failed: %v", err)
	}
}

func writeAnthropicError(w http.ResponseWriter, status int, typ, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":  "error",
		"error": map[string]string{"type": typ, "message": msg},
	})
}
