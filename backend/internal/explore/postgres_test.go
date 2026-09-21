package explore

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
)

func testProjection(t *testing.T, sessions bool) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	schema := "test_explore_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = db.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, err := Open(u.String(), sessions)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func applyRows(t *testing.T, s *Store, rows ...events.ExternalUsage) {
	t.Helper()
	if err := s.Apply(events.Envelope{Version: 1, ID: uuid.NewString(), Rows: rows}); err != nil {
		t.Fatal(err)
	}
}
func testPrices() pricing.CatalogSnapshot {
	return pricing.CatalogSnapshot{Models: map[string]pricing.TokenRates{"tier": {"input_cost_per_token": .001, "output_cost_per_token": .002, "cache_read_input_token_cost": .0001, "cache_creation_input_token_cost": .001, "input_cost_per_token_above_200k_tokens": .1}, "gpt-5.6-sol": {"input_cost_per_token": .001, "output_cost_per_token": .002, "cache_read_input_token_cost": .0001}}}
}

func TestProjectionReplayAliasesAndLateMetadata(t *testing.T) {
	s := testProjection(t, true)
	ts := time.Unix(1700000000, 0)
	row := events.ExternalUsage{ID: "legacy", Source: "codex", TS: ts, Model: "tier", Usage: events.Usage{Input: 1000}}
	envelope := events.Envelope{Version: 1, ID: "duplicate", Rows: []events.ExternalUsage{row}}
	for i := 0; i < 2; i++ {
		if err := s.Apply(envelope); err != nil {
			t.Fatal(err)
		}
	}
	row.ID = "canonical"
	row.Aliases = []string{"legacy"}
	row.Usage.Input = 200
	applyRows(t, s, row)
	row.Aliases = nil
	row.Usage.Input = 10
	row.SessionID = "session-1"
	row.SessionTitle = "Current title"
	row.Project = "/repo"
	row.SessionUpdatedAt = ts.Add(time.Hour)
	applyRows(t, s, row)
	row.Usage.Input = 200
	row.SessionTitle = "Stale title"
	row.Project = "old"
	row.SessionUpdatedAt = ts
	applyRows(t, s, row)
	row.SessionTitle = "Renamed title"
	row.Project = "/new-repo"
	row.SessionUpdatedAt = ts.Add(2 * time.Hour)
	applyRows(t, s, row)
	stale := row
	stale.ID = "legacy"
	stale.Aliases = []string{"canonical"}
	stale.Usage.Input = 999
	applyRows(t, s, stale)
	row.SessionID = ""
	row.SessionTitle = ""
	row.Project = ""
	row.SessionUpdatedAt = time.Time{}
	row.Usage.Input = 1
	applyRows(t, s, row)
	var count int
	if err := s.db.QueryRow(`SELECT count(*) FROM records`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("alias resurrected or canonical lost: %d %v", count, err)
	}
	got, err := scanRecord(s.db.QueryRow(`SELECT ` + recordColumns + ` FROM records`))
	if err != nil {
		t.Fatal(err)
	}
	if got.id != "canonical" || got.in != 200 || got.session != "session-1" || got.title != "Renamed title" || got.project != "/new-repo" {
		t.Fatalf("replay regressed projection: %+v", got)
	}
	before := count
	bad := row
	bad.ID = "invalid"
	bad.Usage.Input = -1
	if err := s.Apply(events.Envelope{Version: 1, ID: "bad-batch", Rows: []events.ExternalUsage{row, bad}}); err == nil {
		t.Fatal("accepted invalid batch")
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM records`).Scan(&count); err != nil || count != before {
		t.Fatalf("partial invalid batch: %d %v", count, err)
	}
	var receipt bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM event_receipts WHERE id='bad-batch')`).Scan(&receipt); err != nil || receipt {
		t.Fatalf("invalid batch acknowledged: %v %v", receipt, err)
	}
	if _, err := s.db.Exec(`ALTER TABLE records ADD CONSTRAINT reject_test_model CHECK(model <> 'reject')`); err != nil {
		t.Fatal(err)
	}
	first, second := row, row
	first.ID, second.ID, second.Model = "first-in-transaction", "second-in-transaction", "reject"
	if err := s.Apply(events.Envelope{Version: 1, ID: "database-failure", Rows: []events.ExternalUsage{first, second}}); err == nil {
		t.Fatal("expected database constraint failure")
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM records`).Scan(&count); err != nil || count != before {
		t.Fatalf("database failure partially committed: %d %v", count, err)
	}
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM event_receipts WHERE id='database-failure')`).Scan(&receipt); err != nil || receipt {
		t.Fatalf("database failure acknowledged: %v %v", receipt, err)
	}
	gateway := row
	gateway.Source = "claude_gateway"
	gateway.ID = "gateway"
	gateway.SessionID = "fabricated"
	applyRows(t, s, gateway)
	if err := s.db.QueryRow(`SELECT count(*) FROM records WHERE source='claude_gateway'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("gateway indexed as session: %d %v", count, err)
	}
}

func TestProjectionSearchFiltersAndKeysetPagination(t *testing.T) {
	s := testProjection(t, true)
	ts := time.Unix(1700000000, 0)
	fixtures := []events.ExternalUsage{
		{Source: "codex", ID: "a", TS: ts, Model: "tier", SessionID: "S-alpha", SessionTitle: "Fix login", Project: "/repo/auth", Usage: events.Usage{Input: 10, CacheRead: 10}},
		{Source: "codex", ID: "b", TS: ts, Model: "tier", SessionID: "S-alpha", SessionTitle: "Fix login", Project: "/repo/auth", Usage: events.Usage{Input: 20}},
		{Source: "claude_code", ID: "c", TS: ts, Model: "claude", SessionID: "S-beta", SessionTitle: "Billing bug"},
		{Source: "codex", ID: "d", TS: ts, Model: "tier", SessionID: "S-gamma", SessionTitle: "Cleanup"},
		{Source: "cursor", ID: "unknown", TS: ts, Model: "unknown"},
	}
	applyRows(t, s, fixtures...)
	f := Filter{Since: ts.Add(-time.Hour), Until: ts.Add(time.Hour), Limit: 2}
	first, err := s.Sessions(context.Background(), f, testPrices())
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 3 || len(first.Sessions) != 2 || first.Next == "" || first.Unassigned != 1 || first.IndexedAt == "" {
		t.Fatalf("bad first page: %+v", first)
	}
	f.Cursor = first.Next
	second, err := s.Sessions(context.Background(), f, testPrices())
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Sessions) != 1 || second.Next != "" || second.Total != 3 {
		t.Fatalf("bad second page: %+v", second)
	}
	seen := map[string]bool{}
	for _, session := range append(first.Sessions, second.Sessions...) {
		if seen[session.ID] {
			t.Fatalf("duplicate page item %s", session.ID)
		}
		seen[session.ID] = true
	}
	f.Cursor = ""
	f.Limit = 25
	for _, q := range []string{"login", "S-AL", "auth"} {
		f.Query = q
		got, err := s.Sessions(context.Background(), f, testPrices())
		if err != nil || got.Total != 1 || got.Sessions[0].ID != "S-alpha" {
			t.Fatalf("search %q: %+v %v", q, got, err)
		}
	}
	f.Query = "lph"
	got, err := s.Sessions(context.Background(), f, testPrices())
	if err != nil || got.Total != 0 {
		t.Fatalf("arbitrary substring matched: %+v %v", got, err)
	}
	f.Query = ""
	f.Source = "claude_code"
	got, err = s.Sessions(context.Background(), f, testPrices())
	if err != nil || got.Total != 1 || got.Sessions[0].ID != "S-beta" {
		t.Fatalf("source filter: %+v %v", got, err)
	}
	f.Source = ""
	f.Model = "tier"
	got, err = s.Sessions(context.Background(), f, testPrices())
	if err != nil || got.Total != 2 {
		t.Fatalf("model filter: %+v %v", got, err)
	}
	f.Model = ""
	f.Since = ts.Add(time.Second)
	got, err = s.Sessions(context.Background(), f, testPrices())
	if err != nil || got.Total != 0 {
		t.Fatalf("time filter: %+v %v", got, err)
	}
	detail, err := s.Detail(context.Background(), "codex", "S-alpha", "", 1, testPrices())
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Events) != 1 || detail.Events[0].ID != "a" || detail.Next == "" || detail.Session.Calls != 2 || detail.Session.CacheRatio == nil || *detail.Session.CacheRatio != .25 {
		t.Fatalf("bad timeline: %+v", detail)
	}
	tail, err := s.Detail(context.Background(), "codex", "S-alpha", detail.Next, 1, testPrices())
	if err != nil || len(tail.Events) != 1 || tail.Events[0].ID != "b" || tail.Next != "" {
		t.Fatalf("timeline cursor: %+v %v", tail, err)
	}
	for _, name := range []string{"records_search", "records_session_prefix", "records_source_time_model", "records_time", "records_model_time", "records_timeline"} {
		var exists bool
		err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname=current_schema() AND indexname=$1)`, name).Scan(&exists)
		if err != nil || !exists {
			t.Fatalf("missing index %s: %v", name, err)
		}
	}
}

func TestInsightsPricePerRequestAndEqualElapsedPeriods(t *testing.T) {
	s := testProjection(t, false)
	since := time.Unix(1700000000, 0)
	until := since.Add(24 * time.Hour)
	rows := []events.ExternalUsage{
		{ID: "tier-1", Source: "codex", TS: since.Add(time.Hour), Model: "tier", SessionID: "large", Usage: events.Usage{Input: 150000}},
		{ID: "tier-2", Source: "codex", TS: since.Add(2 * time.Hour), Model: "tier", SessionID: "large", Usage: events.Usage{Input: 150000}},
		{ID: "unassigned", Source: "cursor", TS: since.Add(time.Hour), Model: "unknown"},
		{ID: "previous", Source: "codex", TS: since.Add(-time.Hour), Model: "tier", SessionID: "previous", Usage: events.Usage{Input: 100}},
		{ID: "too-old", Source: "codex", TS: since.Add(-25 * time.Hour), Model: "tier", SessionID: "old", Usage: events.Usage{Input: 100}},
		{ID: "gateway", Source: "claude_gateway", TS: since.Add(time.Hour), Model: "gateway", Usage: events.Usage{Input: 1000000}},
	}
	for i := 0; i < 5; i++ {
		rows = append(rows, events.ExternalUsage{ID: fmt.Sprintf("low-%d", i), Source: "claude_code", TS: since.Add(time.Hour), Model: "claude", SessionID: "low-cache", Usage: events.Usage{Input: 10000}})
	}
	applyRows(t, s, rows...)
	got, err := s.Insights(context.Background(), since, until, testPrices())
	if err != nil {
		t.Fatal(err)
	}
	if got.Totals.Sessions != 2 || got.Totals.Calls != 8 || got.Totals.Tokens != 350000 || got.Totals.KnownCost != 300 || got.Totals.Unknown != 6 || got.Unassigned != 1 {
		t.Fatalf("bad current totals (context tiers must be per request): %+v", got)
	}
	if got.Previous.Calls != 1 || got.Previous.Sessions != 1 || got.Previous.KnownCost != .1 || got.PreviousSince != since.Add(-24*time.Hour).UTC().Format(time.RFC3339Nano) {
		t.Fatalf("bad previous interval: %+v", got)
	}
	if len(got.Top) != 2 || got.Top[0].ID != "large" || got.Top[0].Estimated != 2 || got.Top[0].Fallback != 0 {
		t.Fatalf("bad top sessions: %+v", got.Top)
	}
	findings := map[string]bool{}
	for _, finding := range got.Findings {
		findings[finding.ID] = true
	}
	if !findings["large-context:codex:large"] || !findings["low-cache:claude_code:low-cache"] || !findings["activity-change"] {
		t.Fatalf("missing evidence: %+v", got.Findings)
	}
}
