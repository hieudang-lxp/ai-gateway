package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/proxy"
)

func TestModelAttributionAndRequestTrace(t *testing.T) {
	for _, tc := range []struct {
		name, body, content, model, source string
		status                             int
	}{
		{"response", `{"model":"resolved-model","usage":{"input_tokens":1}}`, "application/json", "resolved-model", "response", 200},
		{"error", `{"type":"error"}`, "application/json", "requested-model", "request", 404},
		{"stream_without_start", "event: error\ndata: {\"type\":\"error\"}\n\n", "text/event-stream", "requested-model", "request", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.content)
				w.Header().Set("request-id", "provider-request-123")
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer up.Close()
			st := newTestStore(t)
			g, err := proxy.New(up.URL, st, testPricing(t), control.NewWatcher(filepath.Join(t.TempDir(), "none")))
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			g.ServeHTTP(w, httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"requested-model"}`)))
			waitForStats(t, st, 1)
			calls, err := st.RecentCalls(1, 0)
			if err != nil {
				t.Fatal(err)
			}
			c := calls[0]
			if c.Model != tc.model || c.ModelSource != tc.source || c.RequestModel != "requested-model" || c.RequestPath != "/v1/messages" || c.RequestID == "" || c.RequestID != w.Header().Get("X-Gateway-Request-Id") || c.UpstreamRequestID != "provider-request-123" {
				t.Fatalf("incomplete trace: %+v", c)
			}
		})
	}
}

func TestTransportFailureRetainsModelAndTrace(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	up.Close()
	st := newTestStore(t)
	g, err := proxy.New(up.URL, st, testPricing(t), control.NewWatcher(filepath.Join(t.TempDir(), "none")))
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"requested-model"}`)))
	calls, err := st.RecentCalls(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != 502 || len(calls) != 1 {
		t.Fatalf("status=%d calls=%d", w.Code, len(calls))
	}
	c := calls[0]
	if c.Model != "requested-model" || c.RequestModel != c.Model || c.ModelSource != "request" || c.Status != 502 || c.RequestID == "" || c.RequestID != w.Header().Get("X-Gateway-Request-Id") || c.CostUSD != 0 {
		t.Fatalf("transport failure lost attribution: %+v", c)
	}
}

func TestMissingModelRejectedWithoutFakeCall(t *testing.T) {
	hits := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer up.Close()
	st := newTestStore(t)
	g, err := proxy.New(up.URL, st, testPricing(t), control.NewWatcher(filepath.Join(t.TempDir(), "none")))
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{}`, `{"model":null}`, `{"model":12}`, `{"model":" "}`, `invalid-json`} {
		w := httptest.NewRecorder()
		g.ServeHTTP(w, httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body)))
		if w.Code != 400 || w.Header().Get("X-Gateway-Request-Id") == "" {
			t.Fatalf("untraceable validation failure: %d", w.Code)
		}
	}
	n, _ := st.TotalCalls()
	if n != 0 || hits != 0 {
		t.Fatalf("fake calls=%d upstream=%d", n, hits)
	}
}
