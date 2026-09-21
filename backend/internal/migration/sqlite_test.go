package migration_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/migration"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

func destination(t *testing.T) (string, *store.Store, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	schema := "test_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
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
	dsn = u.String()
	s, err := store.OpenPostgres(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return dsn, s, db
}

func execSQL(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func snapshot(t *testing.T) (string, []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gateway.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.EnsureSyncSchema(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.LastSyncedID(); err != nil {
		t.Fatal(err)
	}
	if err = s.EnableEvents(); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	execSQL(t, db, `INSERT INTO calls(id,ts,model,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,est_cost_usd,latency_ms,status,routed_from,cache_hit,saved_usd,request_id,request_model,request_path,model_source,upstream_request_id,local_id) VALUES(41,1700000000,'actual',11,12,13,14,0.25,123,200,'route',1,0.5,'request','requested','/v1/messages','response','upstream',9)`)
	execSQL(t, db, `INSERT INTO calls(id,ts,model,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,est_cost_usd,latency_ms,status) VALUES(99,1700000000,'deleted',0,0,0,0,0,0,200)`)
	execSQL(t, db, `DELETE FROM calls WHERE id=99`)
	execSQL(t, db, `INSERT INTO cache VALUES('cache-key',1700000000,200,'application/octet-stream',$1,0.25,'actual')`, []byte{0, 255, 10, 34})
	execSQL(t, db, `UPDATE gateway_event_state SET origin='retained-origin',last_id=41 WHERE id=1`)
	payload, err := json.Marshal(events.Envelope{Version: 1, ID: "retained-envelope", Rows: []events.ExternalUsage{{ID: "retained-origin:41", Source: "claude_gateway", TS: time.Unix(1700000000, 0), Model: "actual", Usage: events.Usage{Input: 11}}}})
	if err != nil {
		t.Fatal(err)
	}
	execSQL(t, db, `INSERT INTO event_outbox(seq,id,subject,payload) VALUES(73,'retained-envelope',$1,$2)`, events.Subject, payload)
	execSQL(t, db, `INSERT INTO event_outbox(seq,id,subject,payload) VALUES(199,'deleted-envelope',$1,$2)`, events.Subject, payload)
	execSQL(t, db, `DELETE FROM event_outbox WHERE seq=199`)
	execSQL(t, db, `INSERT INTO external_usage VALUES('codex','legacy',1700000000,'legacy-model',10,20,30,40,NULL,'')`)
	execSQL(t, db, `INSERT INTO external_usage VALUES('codex','A',1700000000,'legacy-model',1,0,0,0,0.25,'estimated'),('codex','a',1700000000,'legacy-model',2,0,0,0,0.5,'estimated')`)
	execSQL(t, db, `INSERT INTO sync_state VALUES(1,40)`)
	execSQL(t, db, `INSERT INTO budget_snapshot VALUES(1,1,2,3,4,5,6,1700000000)`)
	execSQL(t, db, `PRAGMA wal_checkpoint(TRUNCATE)`)
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	return path, payload
}

func TestSQLiteMigrationPreservesHistoryAndAllowsExactReplay(t *testing.T) {
	dsn, s, db := destination(t)
	path, payload := snapshot(t)
	report, err := migration.SQLite(context.Background(), path, dsn, "gateway")
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"calls", "cache", "external_usage", "gateway_event_state", "event_outbox", "sync_state", "budget_snapshot"} {
		wantRows := int64(1)
		if table == "external_usage" {
			wantRows = 3
		}
		if report[table].Rows != wantRows || len(report[table].SHA256) != 64 {
			t.Fatalf("missing verification for %s: %+v", table, report[table])
		}
	}
	calls, err := s.CallsAfter(0, 10)
	if err != nil || len(calls) != 1 {
		t.Fatalf("calls=%+v err=%v", calls, err)
	}
	c := calls[0]
	if c.ID != 41 || c.TS.Unix() != 1700000000 || c.Model != "actual" || c.RequestID != "request" || c.RequestModel != "requested" || c.RequestPath != "/v1/messages" || c.ModelSource != "response" || c.UpstreamRequestID != "upstream" || c.Usage != (store.Usage{Input: 11, Output: 12, CacheRead: 13, CacheWrite: 14}) || c.CostUSD != .25 || !c.CacheHit || c.SavedUSD != .5 || c.RoutedFrom != "route" || c.LatencyMS != 123 || c.Status != 200 {
		t.Fatalf("call changed: %+v", c)
	}
	var localID int64
	if err = db.QueryRow(`SELECT local_id FROM calls WHERE id=41`).Scan(&localID); err != nil || localID != 9 {
		t.Fatalf("local ID=%d err=%v", localID, err)
	}
	var body []byte
	if err = db.QueryRow(`SELECT body FROM cache WHERE key='cache-key'`).Scan(&body); err != nil || !bytes.Equal(body, []byte{0, 255, 10, 34}) {
		t.Fatalf("cache changed: %v %v", body, err)
	}
	var origin, eventID, subject string
	var cursor, seq int64
	if err = db.QueryRow(`SELECT origin,last_id FROM gateway_event_state`).Scan(&origin, &cursor); err != nil || origin != "retained-origin" || cursor != 41 {
		t.Fatalf("event state=%q/%d err=%v", origin, cursor, err)
	}
	if err = db.QueryRow(`SELECT seq,id,subject,payload FROM event_outbox`).Scan(&seq, &eventID, &subject, &body); err != nil || seq != 73 || eventID != "retained-envelope" || subject != events.Subject || !bytes.Equal(body, payload) {
		t.Fatalf("outbox changed: seq=%d id=%q subject=%q payload=%s err=%v", seq, eventID, subject, body, err)
	}
	var source, legacyID string
	var tokens int64
	var cost sql.NullFloat64
	if err = db.QueryRow(`SELECT source,event_id,input_tokens+output_tokens+cache_read_tokens+cache_write_tokens,cost_usd FROM external_usage WHERE event_id='legacy'`).Scan(&source, &legacyID, &tokens, &cost); err != nil || source != "codex" || legacyID != "legacy" || tokens != 100 || cost.Valid {
		t.Fatalf("legacy usage changed: %q %q %d %+v %v", source, legacyID, tokens, cost, err)
	}
	if id, err := s.LastSyncedID(); err != nil || id != 40 {
		t.Fatalf("sync cursor=%d err=%v", id, err)
	}
	dw, dh, ww, wh, mw, mh, ok, err := s.ReadBudgetSnapshot()
	if err != nil || !ok || dw != 1 || dh != 2 || ww != 3 || wh != 4 || mw != 5 || mh != 6 {
		t.Fatalf("budget snapshot changed: %v %v %v %v %v %v %v %v", dw, dh, ww, wh, mw, mh, ok, err)
	}
	if err = s.EnableEvents(); err != nil {
		t.Fatal(err)
	}
	if err = s.Insert(store.Record{TS: time.Now(), Model: "new", Status: 200}); err != nil {
		t.Fatal(err)
	}
	fresh, err := s.RecentCalls(1, 0)
	if err != nil || len(fresh) != 1 || fresh[0].ID <= 99 {
		t.Fatalf("sequence not restored: %+v %v", fresh, err)
	}
	if err = db.QueryRow(`SELECT MAX(seq) FROM event_outbox`).Scan(&seq); err != nil || seq <= 199 {
		t.Fatalf("outbox sequence=%d err=%v", seq, err)
	}

	execSQL(t, db, `DELETE FROM event_outbox WHERE seq=73`)
	repeated, err := migration.SQLite(context.Background(), path, dsn, "gateway")
	if err != nil || !reflect.DeepEqual(report, repeated) {
		t.Fatalf("exact replay failed: %+v %v", repeated, err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM calls`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("replay changed live calls: %d %v", count, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM event_outbox`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("replay resurrected sent event: %d %v", count, err)
	}
	changed, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	execSQL(t, changed, `UPDATE calls SET request_id='changed' WHERE id=41`)
	execSQL(t, changed, `PRAGMA wal_checkpoint(TRUNCATE)`)
	if err = changed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = migration.SQLite(context.Background(), path, dsn, "gateway"); err == nil || !strings.Contains(err.Error(), "snapshot differs") {
		t.Fatalf("changed snapshot accepted: %v", err)
	}
	if err = db.QueryRow(`SELECT request_id FROM calls WHERE id=41`).Scan(&eventID); err != nil || eventID != "request" {
		t.Fatalf("changed snapshot overwrote original: %q %v", eventID, err)
	}
}

func TestSQLiteMigrationRejectsNonemptyDestination(t *testing.T) {
	dsn, s, db := destination(t)
	path, _ := snapshot(t)
	if err := s.Insert(store.Record{TS: time.Now(), Model: "already-present", Status: 200}); err != nil {
		t.Fatal(err)
	}
	if _, err := migration.SQLite(context.Background(), path, dsn, "gateway"); err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("nonempty destination accepted: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM calls`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("existing history changed: %d %v", count, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial migration committed: %d %v", count, err)
	}
}
