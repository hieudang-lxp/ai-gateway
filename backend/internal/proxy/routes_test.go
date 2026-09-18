package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/proxy"
)

func TestUnknownLocalRoutesNeverReachUpstream(t *testing.T) {
	hits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; http.NotFound(w, r) }))
	defer upstream.Close()
	st := newTestStore(t)
	g, err := proxy.New(upstream.URL, st, testPricing(t), control.NewWatcher(filepath.Join(t.TempDir(), "none")))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/favicon.ico", "/assets/missing.js", "/unknown", "/v1/messages"} {
		w := httptest.NewRecorder()
		g.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 && w.Code != 405 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	if hits != 0 {
		t.Fatalf("forwarded %d local requests upstream", hits)
	}
	n, _ := st.TotalCalls()
	if n != 0 {
		t.Fatalf("recorded %d fake model calls", n)
	}
}

func TestUpstreamErrorKeepsRequestedModel(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		io.WriteString(w, `{"type":"error"}`)
	}))
	defer upstream.Close()
	st := newTestStore(t)
	g, err := proxy.New(upstream.URL, st, testPricing(t), control.NewWatcher(filepath.Join(t.TempDir(), "none")))
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"missing-model"}`)))
	rows := waitForStats(t, st, 1)
	if rows[0].Model != "missing-model" {
		t.Fatalf("lost request model: %+v", rows)
	}
}
