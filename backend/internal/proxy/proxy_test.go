package proxy_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/proxy"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func testPricing(t *testing.T) pricing.Pricing {
	t.Helper()
	return pricing.Load(filepath.Join(t.TempDir(), "pricing.json"))
}

// waitForStats polls until StatsSince(epoch) returns wantModels rows (finalize
// runs async on body close).
func waitForStats(t *testing.T, st *store.Store, wantModels int) []store.StatRow {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rows, err := st.StatsSince(time.Unix(0, 0))
		if err != nil {
			t.Fatalf("stats: %v", err)
		}
		if len(rows) >= wantModels {
			return rows
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d stat rows", wantModels)
	return nil
}

func TestProxyJSONResponseLogged(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"msg_1","model":"claude-sonnet-5","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":10,"cache_creation_input_tokens":4}}`)
	}))
	defer upstream.Close()

	st := newTestStore(t)
	g, err := proxy.New(upstream.URL, st, testPricing(t))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(g)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/messages", "application/json",
		strings.NewReader(`{"model":"claude-sonnet-5","messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"msg_1"`) {
		t.Fatalf("bad passthrough: %d %s", resp.StatusCode, body)
	}

	rows := waitForStats(t, st, 1)
	r := rows[0]
	if r.Model != "claude-sonnet-5" || r.Input != 100 || r.Output != 50 || r.CacheRead != 10 || r.CacheWrite != 4 {
		t.Fatalf("bad row: %+v", r)
	}
	// sonnet default rates: (100*3 + 50*15 + 10*0.3 + 4*3.75)/1e6
	want := (100*3.0 + 50*15.0 + 10*0.3 + 4*3.75) / 1e6
	if diff := r.CostUSD - want; diff > 1e-12 || diff < -1e-12 {
		t.Fatalf("cost = %v want %v", r.CostUSD, want)
	}
}

func TestProxySSEResponseLogged(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-opus-4-8\",\"usage\":{\"input_tokens\":200,\"output_tokens\":1,\"cache_read_input_tokens\":0,\"cache_creation_input_tokens\":0}}}\n\n")
		f.Flush()
		fmt.Fprint(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":80}}\n\n")
		f.Flush()
	}))
	defer upstream.Close()

	st := newTestStore(t)
	g, err := proxy.New(upstream.URL, st, testPricing(t))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(g)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/messages", "application/json",
		strings.NewReader(`{"model":"claude-opus-4-8","stream":true,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "message_delta") {
		t.Fatalf("stream not passed through: %s", body)
	}

	rows := waitForStats(t, st, 1)
	r := rows[0]
	if r.Model != "claude-opus-4-8" || r.Input != 200 || r.Output != 80 {
		t.Fatalf("bad row: %+v", r)
	}
}
