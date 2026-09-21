package proxy

import (
	"bytes"
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

type reqInfo struct {
	requestID         string
	requestModel      string
	requestPath       string
	upstreamRequestID string
	model             string
	start             time.Time
	routedFrom        string
	cacheKey          string
}

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

			req.Header.Del("Accept-Encoding")
		},
		ModifyResponse: g.modifyResponse,

		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error: %v", err)
			w.WriteHeader(http.StatusBadGateway)
			if info, ok := r.Context().Value(infoKey).(*reqInfo); ok {
				g.finalize(newParser(false), http.StatusBadGateway, "", info, nil)
			}
		},
		FlushInterval: -1,
	}
	g.proxy = proxy
	return g, nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/v1/messages" {
		g.handleMessages(w, r)
		return
	}
	if (r.Method == http.MethodGet && (r.URL.Path == "/v1/models" || strings.HasPrefix(r.URL.Path, "/v1/models/"))) || (r.Method == http.MethodPost && r.URL.Path == "/v1/messages/count_tokens") {
		g.proxy.ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/v1/messages" {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.NotFound(w, r)
}

func (g *Gateway) modifyResponse(resp *http.Response) error {
	info, _ := resp.Request.Context().Value(infoKey).(*reqInfo)
	if info == nil {
		return nil
	}
	info.upstreamRequestID = resp.Header.Get("request-id")
	if info.upstreamRequestID == "" {
		info.upstreamRequestID = resp.Header.Get("x-request-id")
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

func (g *Gateway) finalize(p *parser, status int, contentType string, info *reqInfo, captured *bytes.Buffer) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("finalize panic: %v", r)
		}
	}()
	p.finalize()
	model := p.model
	modelSource := "response"
	if model == "" {
		model = info.model
		modelSource = "request"
	}
	if model == "" {
		log.Printf("skip non-model accounting request_id=%s path=%s", info.requestID, info.requestPath)
		return
	}
	cost := g.pricing.Cost(model, p.u.Input, p.u.Output, p.u.CacheRead, p.u.CacheWrite)

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
		RequestID: info.requestID, RequestModel: info.requestModel, RequestPath: info.requestPath, ModelSource: modelSource, UpstreamRequestID: info.upstreamRequestID,
		TS: time.Now(), Model: model, Usage: p.u, CostUSD: cost,
		LatencyMS: time.Since(info.start).Milliseconds(), Status: status,
		RoutedFrom: info.routedFrom,
	}
	if err := g.store.Insert(rec); err != nil {
		log.Printf("store insert failed: %v", err)
	}
}

type usageTap struct {
	rc      io.ReadCloser
	parser  *parser
	capture *bytes.Buffer
	onClose func()
	closed  bool
}

func (t *usageTap) Read(p []byte) (int, error) {
	n, err := t.rc.Read(p)
	if n > 0 {
		t.parser.feed(p[:n])
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
