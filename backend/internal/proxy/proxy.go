package proxy

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

type ctxKey int

const startKey ctxKey = 0

// Gateway is the reverse proxy plus the store it logs to.
type Gateway struct {
	proxy   *httputil.ReverseProxy
	store   *store.Store
	pricing pricing.Pricing
}

func New(upstream string, st *store.Store, pr pricing.Pricing) (*Gateway, error) {
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, err
	}
	g := &Gateway{store: st, pricing: pr}

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
	ctx := context.WithValue(r.Context(), startKey, time.Now())
	g.proxy.ServeHTTP(w, r.WithContext(ctx))
}

// modifyResponse wraps the response body so usage is extracted as it streams to
// the client. It must never return an error: doing so makes ReverseProxy send
// the client a 502. Any logging-setup problem is swallowed.
func (g *Gateway) modifyResponse(resp *http.Response) error {
	start, _ := resp.Request.Context().Value(startKey).(time.Time)
	if start.IsZero() {
		start = time.Now()
	}
	sse := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")
	p := newParser(sse)
	status := resp.StatusCode

	resp.Body = &usageTap{
		rc:     resp.Body,
		parser: p,
		onClose: func() {
			g.finalize(p, status, start)
		},
	}
	return nil
}

// finalize computes cost and writes the row. Runs on Body.Close(), after the
// client already has every byte — so a synchronous SQLite write adds no
// client-visible latency. Store failures log to stderr and never propagate.
func (g *Gateway) finalize(p *parser, status int, start time.Time) {
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
	r := store.Record{
		TS:        time.Now(),
		Model:     model,
		Usage:     p.u,
		CostUSD:   g.pricing.Cost(model, p.u.Input, p.u.Output, p.u.CacheRead, p.u.CacheWrite),
		LatencyMS: time.Since(start).Milliseconds(),
		Status:    status,
	}
	if err := g.store.Insert(r); err != nil {
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
	onClose func()
	closed  bool
}

func (t *usageTap) Read(p []byte) (int, error) {
	n, err := t.rc.Read(p)
	if n > 0 {
		t.parser.feed(p[:n]) // cheap + recover-guarded; never errors
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
