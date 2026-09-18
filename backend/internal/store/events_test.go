package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestProxyOutboxBackfillAndAtomicInsert(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rec := Record{TS: time.Now(), Model: "model", Status: 200}
	if err = s.Insert(rec); err != nil {
		t.Fatal(err)
	}
	if err = s.EnableEvents(); err != nil {
		t.Fatal(err)
	}
	if err = s.EnableEvents(); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow(`SELECT count(*) FROM event_outbox`).Scan(&n)
	if n != 1 {
		t.Fatalf("backfill repeated: %d", n)
	}
	if _, err = s.db.Exec(`CREATE TRIGGER fail_outbox BEFORE INSERT ON event_outbox BEGIN SELECT RAISE(ABORT,'test failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if err = s.Insert(rec); err == nil {
		t.Fatal("expected transactional outbox failure")
	}
	count, err := s.TotalCalls()
	if err != nil || count != 1 {
		t.Fatalf("partial call committed: %d %v", count, err)
	}
}
