package ledger

import (
	"database/sql"
	"github.com/google/uuid"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func testDB(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = db.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec("DROP SCHEMA " + schema + " CASCADE"); db.Close() })
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, err := Open(u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestReplayAliasAndAtomicRollback(t *testing.T) {
	s := testDB(t)
	if err := s.Quarantine("hash", "usage.ingested.v1", "invalid event JSON"); err != nil {
		t.Fatal(err)
	}
	if err := s.Quarantine("hash", "usage.ingested.v1", "invalid event JSON"); err != nil {
		t.Fatal(err)
	}
	var rejected int
	if err := s.db.QueryRow(`SELECT count(*) FROM rejected_events`).Scan(&rejected); err != nil || rejected != 1 {
		t.Fatalf("quarantine not durable/idempotent: %d %v", rejected, err)
	}
	row := events.ExternalUsage{ID: "legacy", Source: "codex", TS: time.Now(), Model: "model", Usage: events.Usage{Input: 10}}
	send := func(rows ...events.ExternalUsage) {
		t.Helper()
		if err := s.ImportUsage(rows); err != nil {
			t.Fatal(err)
		}
	}
	send(row)
	row.ID = "response"
	row.Aliases = []string{"legacy"}
	row.Usage.Input = 20
	e := events.Envelope{Version: 1, ID: "delivery-1", Rows: []events.ExternalUsage{row}}
	for i := 0; i < 2; i++ {
		if err := s.Apply(e); err != nil {
			t.Fatal(err)
		}
	}
	row.ID = "legacy"
	row.Aliases = nil
	send(row) // old replay must not resurrect alias
	cost := func(string, int64, int64, int64, int64) (float64, bool) { return .25, false }
	got, err := s.PricedUsageSince(time.Unix(0, 0), cost)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Calls != 1 || got[0].Input != 20 || got[0].KnownCostUSD != .25 {
		t.Fatalf("replay changed total: %+v", got)
	}
	row.ID = "rollback"
	bad := row
	bad.ID = ""
	if err = s.ImportUsage([]events.ExternalUsage{row, bad}); err == nil {
		t.Fatal("accepted invalid record")
	}
	got, err = s.PricedUsageSince(time.Unix(0, 0), cost)
	if err != nil || got[0].Calls != 1 {
		t.Fatalf("partial transaction committed: %+v %v", got, err)
	}
	now := time.Now()
	status := events.SourceStatus{State: "ok", LastAttempt: now, LastSuccess: &now}
	if err = s.Apply(events.Envelope{Version: 1, ID: "status-new", Source: "codex", Status: &status}); err != nil {
		t.Fatal(err)
	}
	status.LastAttempt = now.Add(-time.Hour)
	status.State = "error"
	if err = s.Apply(events.Envelope{Version: 1, ID: "status-old", Source: "codex", Status: &status}); err != nil {
		t.Fatal(err)
	}
	statuses, err := s.Statuses()
	if err != nil || statuses["codex"].State != "ok" {
		t.Fatalf("stale status overwrote fresh: %+v %v", statuses, err)
	}
}
