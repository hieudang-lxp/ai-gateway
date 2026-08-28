package proxy

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

type ctxKey int

const infoKey ctxKey = 0

// reqInfo travels with a request from dispatch through to finalize.
type reqInfo struct {
	start      time.Time
	routedFrom string // original model if rewritten, else ""
	cacheKey   string // non-empty → capture & store the response on success
}

// Gateway is the reverse proxy plus the store it logs to.
type Gateway struct {
	proxy   *httputil.ReverseProxy
	store   *store.Store
	pricing pricing.Pricing
	ctl     *control.Watcher
}

func New(upstream string, st *store.Store, pr pricing.Pricing, ctl *control.Watcher) (*Gateway, error) {
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, err
	}
	g := &Gateway{store: st, pricing: pr, ctl: ctl}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			// Force identity encoding so the tee'd copy is plaintext and usage
			// extraction works. On localhost the lost compression is irrelevant.
			req.Header.Del("Accept-Encoding")
		},
		ModifyResponse: g.modifyResponse,
		// Fail-open: on any transport error, log and hand the client a 502-ish
		// error rather than a silent hang. We never mutate the client's key.
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error: %v", err)
			w.WriteHeader(http.StatusBadGateway)
		},
		FlushInterval: -1, // flush immediately for SSE streaming
	}
	g.proxy = proxy
	return g, nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/v1/messages" {
		g.handleMessages(w, r)
		return
	}
	info := &reqInfo{start: time.Now()}
	g.proxy.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), infoKey, info)))
}

// modifyResponse wraps the response body so usage is extracted as it streams to
// the client. It must never return an error: doing so makes ReverseProxy send
// the client a 502. Any logging-setup problem is swallowed.
func (g *Gateway) modifyResponse(resp *http.Response) error {
	info, _ := resp.Request.Context().Value(infoKey).(*reqInfo)
	if info == nil {
		info = &reqInfo{start: time.Now()}
	}
	contentType := resp.Header.Get("Content-Type")
	sse := strings.Contains(contentType, "text/event-stream")
	p := newParser(sse)
	status := resp.StatusCode

	tap := &usageTap{rc: resp.Body, parser: p}
	if info.cacheKey != "" && status == http.StatusOK {
		tap.capture = &bytes.Buffer{}
	}
	tap.onClose = func() {
		g.finalize(p, status, contentType, info, tap.capture)
	}
	resp.Body = tap
	return nil
}

// finalize computes cost and writes the row. Runs on Body.Close(), after the
// client already has every byte — so a synchronous SQLite write adds no
// client-visible latency. Store failures log to stderr and never propagate.
func (g *Gateway) finalize(p *parser, status int, contentType string, info *reqInfo, captured *bytes.Buffer) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("finalize panic: %v", r)
		}
	}()
	p.finalize()
	model := p.model
	if model == "" {
		model = "unknown"
	}
	cost := g.pricing.Cost(model, p.u.Input, p.u.Output, p.u.CacheRead, p.u.CacheWrite)
	// CachePut BEFORE Insert: once the calls row is visible, the cache entry
	// must already exist (tests use the row as the "finalize done" signal).
	if captured != nil && status == http.StatusOK {
		err := g.store.CachePut(info.cacheKey, store.CachedResponse{
			Status: status, ContentType: contentType,
			Body: captured.Bytes(), CostUSD: cost, Model: model,
		})
		if err != nil {
			log.Printf("cache put failed: %v", err)
		}
	}
	rec := store.Record{
		TS: time.Now(), Model: model, Usage: p.u, CostUSD: cost,
		LatencyMS: time.Since(info.start).Milliseconds(), Status: status,
		RoutedFrom: info.routedFrom,
	}
	if err := g.store.Insert(rec); err != nil {
		log.Printf("store insert failed: %v", err)
	}
}

// usageTap wraps a response body, feeding a copy of every byte to a parser and
// firing onClose exactly once when the body is closed.
//
// We roll our own instead of io.TeeReader because TeeReader surfaces the
// parser's write path as a read error — a fail-open violation. Here the client
// read path is fully independent of the parser.
type usageTap struct {
	rc      io.ReadCloser
	parser  *parser
	capture *bytes.Buffer // non-nil → also buffer bytes for cache storage
	onClose func()
	closed  bool
}

func (t *usageTap) Read(p []byte) (int, error) {
	n, err := t.rc.Read(p)
	if n > 0 {
		t.parser.feed(p[:n]) // cheap + recover-guarded; never errors
		if t.capture != nil {
			t.capture.Write(p[:n])
		}
	}
	return n, err
}

func (t *usageTap) Close() error {
	err := t.rc.Close()
	if !t.closed {
		t.closed = true
		t.onClose()
	}
	return err
}
