package eventbus

import (
	"encoding/json"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	"path/filepath"
	"testing"
	"time"
)

func TestOutboxSurvivesRestartAndQueuesStatusAfterUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.db")
	o, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]events.ExternalUsage, 205)
	for i := range rows {
		rows[i] = events.ExternalUsage{ID: "request", Source: "codex", TS: time.Now()}
	}
	if err = o.ImportUsage(rows); err != nil {
		t.Fatal(err)
	}
	if err = o.RecordStatus("codex", events.SourceStatus{State: "ok", LastAttempt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	o.Close()
	o, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if err := o.ImportUsage([]events.ExternalUsage{{ID: "invalid", Source: "claude_code"}}); err == nil {
		t.Fatal("queued missing timestamp")
	}
	n, err := o.Pending()
	if err != nil || n != 4 {
		t.Fatalf("pending=%d err=%v", n, err)
	}
	results, err := o.db.Query(`SELECT subject,payload FROM event_outbox ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer results.Close()
	count := 0
	seen := map[string]bool{}
	for results.Next() {
		var subject string
		var body []byte
		results.Scan(&subject, &body)
		var e events.Envelope
		if err = json.Unmarshal(body, &e); err != nil {
			t.Fatal(err)
		}
		if e.Version != 1 || e.ID == "" || seen[e.ID] {
			t.Fatalf("bad identity: %+v", e)
		}
		seen[e.ID] = true
		count++
		if count < 4 && subject != events.Subject {
			t.Fatal("status overtook usage")
		}
		if count == 4 && (subject != events.StatusSubject || e.Status == nil) {
			t.Fatal("status not queued last")
		}
	}
	if err = results.Err(); err != nil {
		t.Fatal(err)
	}
}
