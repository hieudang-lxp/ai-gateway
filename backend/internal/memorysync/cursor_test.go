package memorysync

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func cursorFixture(t *testing.T) (string, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.vscdb")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; CREATE TABLE cursorDiskKV (key TEXT PRIMARY KEY, value BLOB);`); err != nil {
		t.Fatal(err)
	}
	return path, db
}

func cursorPut(t *testing.T, db *sql.DB, key, value string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO cursorDiskKV(key,value) VALUES(?,?)`, key, value); err != nil {
		t.Fatal(err)
	}
}

func TestCursorReadsAllHumanHistoryAndPreservesStableIdentity(t *testing.T) {
	path, db := cursorFixture(t)
	cursorPut(t, db, "composerData:session-a", `{"composerId":"session-a","text":"unsent draft","createdAt":1720000000000,"fullConversationHeadersOnly":[{"bubbleId":"user-a","type":1}]}`)
	cursorPut(t, db, "composerData:child", `{"composerId":"child","subagentInfo":{"parentComposerId":"session-a"}}`)
	for key, value := range map[string]string{
		"bubbleId:session-a:user-a":    `{"bubbleId":"user-a","type":1,"text":"ok","createdAt":"2026-09-22T05:00:00.000Z"}`,
		"bubbleId:session-a:user-b":    `{"bubbleId":"user-b","type":1,"text":"y","createdAt":"2026-09-22T05:01:00.000Z"}`,
		"bubbleId:session-a:assistant": `{"bubbleId":"assistant","type":2,"text":"private assistant","createdAt":"2026-09-22T05:00:01.000Z"}`,
		"bubbleId:session-a:tool":      `{"bubbleId":"tool","type":2,"text":"private tool","toolFormerData":{},"createdAt":"2026-09-22T05:00:02.000Z"}`,
		"bubbleId:session-a:simulated": `{"bubbleId":"simulated","type":1,"text":"system injected","isSimulatedMsg":true,"createdAt":"2026-09-22T05:00:03.000Z"}`,
		"bubbleId:session-a:empty":     `{"bubbleId":"empty","type":1,"text":"  ","createdAt":"2026-09-22T05:00:03.000Z"}`,
		"bubbleId:child:generated":     `{"bubbleId":"generated","type":1,"text":"agent task","createdAt":"2026-09-22T05:00:03.000Z"}`,
	} {
		cursorPut(t, db, key, value)
	}
	want := []Message{
		{Source: "cursor", SessionID: "session-a", UUID: "user-a", Speaker: "user", Text: "ok", Timestamp: "2026-09-22T05:00:00Z", GroupID: "personal"},
		{Source: "cursor", SessionID: "session-a", UUID: "user-b", Speaker: "user", Text: "y", Timestamp: "2026-09-22T05:01:00Z", GroupID: "personal"},
	}
	for i := 0; i < 2; i++ {
		var got []Message
		err := NewCursorSource(path).Scan(context.Background(), func(m Message) error { got = append(got, m); return nil })
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected messages: %#v", got)
		}
	}
}

func TestCursorReportsMissingUnsupportedAndMalformedWithoutContent(t *testing.T) {
	t.Run("missing does not create database", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.vscdb")
		if err := NewCursorSource(path).Scan(context.Background(), func(Message) error { return nil }); err == nil {
			t.Fatal("expected missing storage error")
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("database was created: %v", err)
		}
	})
	t.Run("unsupported", func(t *testing.T) {
		path, db := cursorFixture(t)
		if _, err := db.Exec(`DROP TABLE cursorDiskKV; CREATE TABLE ItemTable(key TEXT,value BLOB); INSERT INTO ItemTable VALUES('workbench.panel.aichat.view.aichat.chatdata','{"tabs":[]}');`); err != nil {
			t.Fatal(err)
		}
		if err := NewCursorSource(path).Scan(context.Background(), func(Message) error { return nil }); err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("expected unsupported storage error: %v", err)
		}
	})
	for _, tc := range []struct{ name, key, value string }{
		{"malformed JSON", "bubbleId:s:b", `{"text":"SECRET`},
		{"missing composer", "bubbleId:orphan:b", `{"bubbleId":"b","type":1,"text":"SECRET","createdAt":"2026-09-22T05:00:00Z"}`},
		{"bad timestamp", "bubbleId:s:b", `{"bubbleId":"b","type":1,"text":"SECRET","createdAt":"SECRET"}`},
		{"identity mismatch", "bubbleId:s:b", `{"bubbleId":"other","type":1,"text":"SECRET","createdAt":"2026-09-22T05:00:00Z"}`},
		{"unknown type", "bubbleId:s:b", `{"bubbleId":"b","type":17,"text":"SECRET","createdAt":"2026-09-22T05:00:00Z"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, db := cursorFixture(t)
			cursorPut(t, db, "composerData:s", `{"composerId":"s"}`)
			cursorPut(t, db, tc.key, tc.value)
			err := NewCursorSource(path).Scan(context.Background(), func(Message) error { return nil })
			if err == nil {
				t.Fatal("expected source error")
			}
			if strings.Contains(err.Error(), "SECRET") {
				t.Fatal("error leaked source content")
			}
		})
	}
}

func TestCursorPropagatesDeliveryAndCancellation(t *testing.T) {
	path, db := cursorFixture(t)
	cursorPut(t, db, "composerData:s", `{"composerId":"s"}`)
	cursorPut(t, db, "bubbleId:s:b", `{"bubbleId":"b","type":1,"text":"ok","createdAt":"2026-09-22T05:00:00Z"}`)
	want := errors.New("delivery failed")
	if err := NewCursorSource(path).Scan(context.Background(), func(Message) error { return want }); !errors.Is(err, want) {
		t.Fatalf("delivery error lost: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewCursorSource(path).Scan(ctx, func(Message) error { t.Fatal("emitted after cancellation"); return nil }); err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestCursorRejectsUnknownSubagentMetadata(t *testing.T) {
	path, db := cursorFixture(t)
	cursorPut(t, db, "composerData:s", `{"composerId":"s","subagentInfo":"unrecognized"}`)
	cursorPut(t, db, "bubbleId:s:b", `{"bubbleId":"b","type":1,"text":"ok","createdAt":"2026-09-22T05:00:00Z"}`)
	err := NewCursorSource(path).Scan(context.Background(), func(Message) error {
		t.Fatal("unknown authorship emitted as human")
		return nil
	})
	if err == nil {
		t.Fatal("unknown authorship was silently skipped")
	}
}

// Opt-in compatibility check against a local database. Only the count and
// sanitized adapter errors are reported; no message fields are logged.
func TestCursorLocalStorageReadOnly(t *testing.T) {
	path := os.Getenv("CURSOR_MEMORYSYNC_TEST_STATE")
	if path == "" {
		t.Skip("set CURSOR_MEMORYSYNC_TEST_STATE for the read-only compatibility check")
	}
	count := 0
	err := NewCursorSource(path).Scan(context.Background(), func(m Message) error {
		if m.Source != "cursor" || m.Speaker != "user" || m.SessionID == "" || m.UUID == "" || strings.TrimSpace(m.Text) == "" {
			return errors.New("invalid emitted message shape")
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("read-only compatibility check failed: %v", err)
	}
	t.Logf("read-only compatibility check: %d human messages", count)
}
