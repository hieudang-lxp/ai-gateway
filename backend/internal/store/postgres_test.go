package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
)

func postgresStore(t *testing.T) *Store {
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
	schema := "test_store_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	s, err := OpenPostgres(u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPostgresCallAndOutboxCommitAtomically(t *testing.T) {
	s := postgresStore(t)
	rec := Record{TS: time.Unix(1700000000, 0), Model: "model", RequestID: "request", RequestModel: "requested", RequestPath: "/v1/messages", ModelSource: "response", UpstreamRequestID: "upstream", Usage: Usage{Input: 10, Output: 2}, CostUSD: .25, Status: 200}
	if err := s.Insert(rec); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := s.EnableEvents(); err != nil {
			t.Fatal(err)
		}
	}
	var originalOrigin string
	var initialCursor int64
	if err := s.db.QueryRow(`SELECT origin,last_id FROM gateway_event_state`).Scan(&originalOrigin, &initialCursor); err != nil {
		t.Fatal(err)
	}
	if originalOrigin == "" || initialCursor != 1 {
		t.Fatalf("initial origin=%q cursor=%d", originalOrigin, initialCursor)
	}
	if _, err := s.db.Exec(`ALTER TABLE event_outbox ADD CONSTRAINT reject_new_events CHECK (id = '') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(rec); err == nil {
		t.Fatal("expected outbox failure")
	}
	var calls, pending int
	var cursor int64
	if err := s.db.QueryRow(`SELECT (SELECT count(*) FROM calls),(SELECT count(*) FROM event_outbox),last_id FROM gateway_event_state`).Scan(&calls, &pending, &cursor); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || pending != 1 || cursor != initialCursor {
		t.Fatalf("partial transaction: calls=%d events=%d cursor=%d", calls, pending, cursor)
	}
	if _, err := s.db.Exec(`ALTER TABLE event_outbox DROP CONSTRAINT reject_new_events`); err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(rec); err != nil {
		t.Fatal(err)
	}
	got, err := s.RecentCalls(1, 0)
	if err != nil || len(got) != 1 {
		t.Fatalf("recent calls=%+v err=%v", got, err)
	}
	if got[0].ID <= 1 || got[0].RequestID != "request" || got[0].RequestModel != "requested" || got[0].RequestPath != "/v1/messages" || got[0].ModelSource != "response" || got[0].UpstreamRequestID != "upstream" {
		t.Fatalf("call trace lost: %+v", got[0])
	}
	var body []byte
	if err := s.db.QueryRow(`SELECT payload FROM event_outbox ORDER BY seq DESC LIMIT 1`).Scan(&body); err != nil {
		t.Fatal(err)
	}
	var e events.Envelope
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatal(err)
	}
	if len(e.Rows) != 1 || e.Rows[0].ID != fmt.Sprintf("%s:%d", originalOrigin, got[0].ID) || e.Rows[0].Usage.Input != 10 {
		t.Fatalf("wrong call event: %+v", e)
	}
	if err := s.db.QueryRow(`SELECT last_id FROM gateway_event_state`).Scan(&cursor); err != nil || cursor != got[0].ID {
		t.Fatalf("cursor=%d err=%v", cursor, err)
	}
	if err := s.EnableEvents(); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM event_outbox`).Scan(&pending); err != nil || pending != 2 {
		t.Fatalf("backfill repeated: events=%d err=%v", pending, err)
	}
}
