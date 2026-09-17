package pricing

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestCatalogRefreshCacheAndFailure(t *testing.T) {
	body := `{"gpt-5.6-sol":{"input_cost_per_token":0.000004,"output_cost_per_token":0.00002,"cache_read_input_token_cost":0.0000004,"cache_creation_input_token_cost":0.000005,"input_cost_per_token_above_272k_tokens":0.000008,"output_cost_per_token_above_272k_tokens":0.00003,"cache_read_input_token_cost_above_272k_tokens":0.0000008}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "prices.json")
	c := NewCatalog(path)
	c.endpoint = server.URL
	c.Refresh(context.Background())
	s := c.Snapshot()
	cost, fallback := s.Cost("gpt-5.6-sol", 1000, 100, 10000, 0)
	if fallback || math.Abs(cost-0.01) > 1e-9 {
		t.Fatalf("cost=%g fallback=%v", cost, fallback)
	}
	cost, _ = s.Cost("gpt-5.6-sol", 1000, 100, 272000, 0)
	if math.Abs(cost-0.2286) > 1e-9 {
		t.Fatalf("long context=%g", cost)
	}
	if _, fallback = s.Cost("codex-auto-review", 1, 1, 1, 0); !fallback {
		t.Fatal("missing fallback label")
	}
	body = `{"bad":{}}`
	c.Refresh(context.Background())
	if c.Snapshot().Error == "" || c.Snapshot().UpdatedAt.IsZero() {
		t.Fatal("failure discarded good rates")
	}
	restored := NewCatalog(path).Snapshot()
	cost, _ = restored.Cost("gpt-5.6-sol", 1000, 100, 10000, 0)
	if math.Abs(cost-0.01) > 1e-9 || restored.UpdatedAt.IsZero() {
		t.Fatal("cache not restored")
	}
}

func TestMissingWritePriceUsesLongContextInputRate(t *testing.T) {
	s := CatalogSnapshot{Models: map[string]TokenRates{"m": {"input_cost_per_token": 0.000004, "output_cost_per_token": 0.00002, "cache_read_input_token_cost": 0.0000004, "input_cost_per_token_above_272k_tokens": 0.000008}}}
	cost, fallback := s.Cost("m", 0, 0, 0, 300000)
	if fallback || math.Abs(cost-2.4) > 1e-9 {
		t.Fatalf("cost=%g fallback=%v", cost, fallback)
	}
}
