package usage

import (
	"encoding/json"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
	"math"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestSummaryPricesIndividualRequestsAndLabelsFallback(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Aggregate >272K, but neither request exceeds the context threshold.
	events := []store.ExternalUsage{
		{Source: "codex", ID: "a", TS: time.Now(), Model: "gpt-6-astra", Usage: store.Usage{Input: 200000}},
		{Source: "codex", ID: "b", TS: time.Now(), Model: "gpt-6-astra", Usage: store.Usage{Input: 200000}},
		{Source: "codex", ID: "c", TS: time.Now(), Model: "codex-auto-review", Usage: store.Usage{Input: 100000}},
	}
	if err = s.ImportUsage(events); err != nil {
		t.Fatal(err)
	}
	c := New(s, nil, Config{})
	w := httptest.NewRecorder()
	c.HandleSummary(w, httptest.NewRequest("GET", "/_usage", nil))
	var result struct {
		Rows []store.UsageRow `json:"rows"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("rows=%s", w.Body.String())
	}
	for _, r := range result.Rows {
		want := 4.0
		if r.Model == "codex-auto-review" {
			want = 0.4
			if r.FallbackCostCalls != 1 {
				t.Fatal("missing fallback")
			}
		}
		if math.Abs(r.KnownCostUSD-want) > 1e-9 || r.UnknownCostCalls != 0 || r.EstimatedCostCalls != r.Calls {
			t.Fatalf("bad summary: %+v", r)
		}
	}
}
