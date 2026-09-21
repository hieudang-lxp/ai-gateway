package proxy_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/proxy"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

func watcherFor(t *testing.T, yaml string) *control.Watcher {
	t.Helper()
	p := filepath.Join(t.TempDir(), "gateway.yaml")
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return control.NewWatcher(p)
}

func upstreamJSON(t *testing.T, gotModel *atomic.Value, hits *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		var req struct {
			Model string `json:"model"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		gotModel.Store(req.Model)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"msg_1","model":%q,"usage":{"input_tokens":100,"output_tokens":50}}`, req.Model)
	}))
}

func gatewayFor(t *testing.T, upstreamURL string, st *store.Store, ctl *control.Watcher) *httptest.Server {
	t.Helper()
	g, err := proxy.New(upstreamURL, st, testPricing(t), ctl)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	return srv
}

func postMessages(t *testing.T, url, body string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Post(url+"/v1/messages", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(b)
}

func TestBudgetHardBlocks(t *testing.T) {
	var gotModel atomic.Value
	var hits atomic.Int64
	up := upstreamJSON(t, &gotModel, &hits)
	defer up.Close()
	st := newTestStore(t)

	if err := st.Insert(store.Record{TS: time.Now(), Model: "m", CostUSD: 100, Status: 200}); err != nil {
		t.Fatal(err)
	}
	ctl := watcherFor(t, "budget:\n  daily_usd: {warn: 10, hard: 50}\n")
	srv := gatewayFor(t, up.URL, st, ctl)

	resp, body := postMessages(t, srv.URL, `{"model":"claude-sonnet-5","messages":[]}`)
	if resp.StatusCode != 429 {
		t.Fatalf("status = %d want 429", resp.StatusCode)
	}
	if !strings.Contains(body, `"type":"error"`) || !strings.Contains(body, "rate_limit_error") {
		t.Fatalf("want anthropic-shaped error, got %s", body)
	}
	if hits.Load() != 0 {
		t.Fatal("upstream must not be called on hard block")
	}
}

func TestRoutingRewritesModel(t *testing.T) {
	var gotModel atomic.Value
	var hits atomic.Int64
	up := upstreamJSON(t, &gotModel, &hits)
	defer up.Close()
	st := newTestStore(t)
	ctl := watcherFor(t, "routing:\n  rules:\n    - match: \"claude-opus-*\"\n      to: \"claude-sonnet-5\"\n")
	srv := gatewayFor(t, up.URL, st, ctl)

	resp, _ := postMessages(t, srv.URL, `{"model":"claude-opus-4-8","max_tokens":16,"messages":[]}`)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if gotModel.Load() != "claude-sonnet-5" {
		t.Fatalf("upstream saw model %v, want rewrite", gotModel.Load())
	}
	rows := waitForStats(t, st, 1)
	if rows[0].Model != "claude-sonnet-5" {
		t.Fatalf("logged model = %s (cost must follow the actual model)", rows[0].Model)
	}
	calls, err := st.RecentCalls(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if calls[0].RequestModel != "claude-opus-4-8" || calls[0].ModelSource != "response" || calls[0].RoutedFrom != "claude-opus-4-8" {
		t.Fatalf("routing lost original model attribution: %+v", calls[0])
	}
}

func TestBlockedModelRejected(t *testing.T) {
	var gotModel atomic.Value
	var hits atomic.Int64
	up := upstreamJSON(t, &gotModel, &hits)
	defer up.Close()
	ctl := watcherFor(t, "routing:\n  block: [\"claude-fable-*\"]\n")
	srv := gatewayFor(t, up.URL, newTestStore(t), ctl)

	resp, body := postMessages(t, srv.URL, `{"model":"claude-fable-5","messages":[]}`)
	if resp.StatusCode != 400 || !strings.Contains(body, "invalid_request_error") {
		t.Fatalf("want 400 invalid_request_error, got %d %s", resp.StatusCode, body)
	}
	if hits.Load() != 0 {
		t.Fatal("upstream must not be called for blocked model")
	}
}

func TestCacheHitServesStoredResponse(t *testing.T) {
	var gotModel atomic.Value
	var hits atomic.Int64
	up := upstreamJSON(t, &gotModel, &hits)
	defer up.Close()
	st := newTestStore(t)
	ctl := watcherFor(t, "cache:\n  enabled: true\n  ttl: 1h\n")
	srv := gatewayFor(t, up.URL, st, ctl)

	req := `{"model":"claude-sonnet-5","messages":[{"role":"user","content":"hi"}]}`
	_, first := postMessages(t, srv.URL, req)
	waitForStats(t, st, 1)
	_, second := postMessages(t, srv.URL, req)
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d want 1", hits.Load())
	}
	if first != second {
		t.Fatalf("cached response differs:\n%s\n%s", first, second)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		hitsN, saved, err := st.CacheSavings()
		if err != nil {
			t.Fatal(err)
		}
		if hitsN == 1 && saved > 0 {
			rows, err := st.RecentCalls(2, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 || rows[0].ModelSource != "cache" || rows[0].RequestModel != "claude-sonnet-5" || rows[0].RequestID == "" || rows[0].RequestID == rows[1].RequestID {
				t.Fatalf("cache hit lost distinct request trace: %+v", rows)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("cache savings not recorded: hits=%d saved=%v", hitsN, saved)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestNonMessagesPathsBypassControl(t *testing.T) {
	var hits atomic.Int64
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(200)
	}))
	defer up.Close()

	ctl := watcherFor(t, "budget:\n  daily_usd: {hard: 0.000001}\n")
	st := newTestStore(t)
	_ = st.Insert(store.Record{TS: time.Now(), Model: "m", CostUSD: 1, Status: 200})
	srv := gatewayFor(t, up.URL, st, ctl)
	resp, err := http.Get(srv.URL + "/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || hits.Load() != 1 {
		t.Fatalf("bypass failed: %d hits=%d", resp.StatusCode, hits.Load())
	}
}
