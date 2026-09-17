package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestUnifiedUsageDeduplicatesAndDoesNotChangeProxyBudget(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now()
	cost := 2.5
	if err = s.Insert(Record{TS: now, Model: "claude", Usage: Usage{Input: 10, Output: 5}, CostUSD: 1}); err != nil {
		t.Fatal(err)
	}
	events := []ExternalUsage{
		{Source: "codex", ID: "response-1", TS: now, Model: "gpt", Usage: Usage{Input: 40, CacheRead: 60, Output: 20}},
		{Source: "cursor", ID: "event-1", TS: now, Model: "claude", Usage: Usage{Input: 5, CacheWrite: 10, Output: 5}, CostUSD: &cost},
	}
	for i := 0; i < 2; i++ {
		if err = s.ImportUsage(events); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.UnifiedUsageSince(time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	var count, tokens, unknown int64
	var total float64
	for _, r := range rows {
		count += r.Calls
		tokens += r.Input + r.Output + r.CacheRead + r.CacheWrite
		unknown += r.UnknownCostCalls
		total += r.KnownCostUSD
	}
	if count != 3 || tokens != 155 || unknown != 1 || total != 3.5 {
		t.Fatalf("count=%d tokens=%d unknown=%d cost=%v", count, tokens, unknown, total)
	}
	budget, err := s.SpendSince(time.Unix(0, 0))
	if err != nil || budget != 1 {
		t.Fatalf("external usage changed proxy budget: %v %v", budget, err)
	}
}

func TestDirectUsageReplacesPreviouslyImportedLegacyIdentity(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := ExternalUsage{Source: "codex", ID: "legacy:s1:100:0:0:20", TS: time.Now(), Model: "gpt", Usage: Usage{Input: 100, Output: 20}}
	if err = s.ImportUsage([]ExternalUsage{r}); err != nil {
		t.Fatal(err)
	}
	r.Aliases = []string{r.ID}
	r.ID = "response-1"
	if err = s.ImportUsage([]ExternalUsage{r}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.UnifiedUsageSince(time.Unix(0, 0))
	if err != nil || len(rows) != 1 || rows[0].Calls != 1 || rows[0].Input != 100 {
		t.Fatalf("duplicate migration rows: %+v %v", rows, err)
	}
}
