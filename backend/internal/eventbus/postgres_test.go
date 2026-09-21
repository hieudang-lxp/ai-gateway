package eventbus

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	"github.com/nats-io/nats.go"
)

func postgresOutbox(t *testing.T) (*Outbox, string) {
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
	schema := "test_outbox_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	dsn = u.String()
	o, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { o.Close() })
	return o, dsn
}

func TestPostgresOutboxSurvivesRestartWithOrderedEvents(t *testing.T) {
	o, dsn := postgresOutbox(t)
	rows := make([]events.ExternalUsage, 205)
	for i := range rows {
		rows[i] = events.ExternalUsage{ID: "request", Source: "codex", TS: time.Now()}
	}
	if err := o.ImportUsage(rows); err != nil {
		t.Fatal(err)
	}
	if err := o.RecordStatus("codex", events.SourceStatus{State: "ok", LastAttempt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := o.Close(); err != nil {
		t.Fatal(err)
	}
	o, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if err = o.ImportUsage([]events.ExternalUsage{{ID: "invalid", Source: "codex"}}); err == nil {
		t.Fatal("accepted missing timestamp")
	}
	results, err := o.db.Query(`SELECT id,subject,payload FROM event_outbox ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer results.Close()
	seen := map[string]bool{}
	count := 0
	for results.Next() {
		var id, subject string
		var body []byte
		if err = results.Scan(&id, &subject, &body); err != nil {
			t.Fatal(err)
		}
		var e events.Envelope
		if err = json.Unmarshal(body, &e); err != nil {
			t.Fatal(err)
		}
		if e.ID != id || id == "" || seen[id] || e.Version != 1 {
			t.Fatalf("bad event identity: %+v", e)
		}
		seen[id] = true
		if count < 3 {
			want := []int{100, 100, 5}[count]
			if subject != events.Subject || len(e.Rows) != want {
				t.Fatalf("usage batch %d: subject=%q rows=%d", count, subject, len(e.Rows))
			}
		} else if subject != events.StatusSubject || e.Status == nil || e.Status.State != "ok" {
			t.Fatalf("status did not follow usage: %+v", e)
		}
		count++
	}
	if err = results.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("pending events=%d, want 4", count)
	}
}

func TestPostgresEnqueueParticipatesInCallerTransaction(t *testing.T) {
	o, _ := postgresOutbox(t)
	e := events.Envelope{ID: "stable-event", Rows: []events.ExternalUsage{{ID: "request", Source: "codex", TS: time.Now()}}}
	tx, err := o.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = Enqueue(tx, events.Subject, e); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if n, err := o.Pending(); err != nil || n != 0 {
		t.Fatalf("rolled-back event persisted: pending=%d err=%v", n, err)
	}
	tx, err = o.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = Enqueue(tx, events.Subject, e); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var id string
	if err = o.db.QueryRow(`SELECT id FROM event_outbox`).Scan(&id); err != nil || id != "stable-event" {
		t.Fatalf("committed identity=%q err=%v", id, err)
	}
}

type outboxPublisher struct {
	nats.JetStreamContext
	publish func(string, []byte, ...nats.PubOpt) (*nats.PubAck, error)
}

func (p outboxPublisher) Publish(subject string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error) {
	return p.publish(subject, data, opts...)
}

func TestPostgresPublishDeletesOnlyAcknowledgedEvents(t *testing.T) {
	o, _ := postgresOutbox(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, id := range []string{"first", "second"} {
		tx, err := o.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		err = Enqueue(tx, events.Subject, events.Envelope{ID: id, Rows: []events.ExternalUsage{{ID: id, Source: "codex", TS: time.Now()}}})
		if err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	unavailable := errors.New("publish not acknowledged")
	publisher := outboxPublisher{publish: func(string, []byte, ...nats.PubOpt) (*nats.PubAck, error) {
		return nil, unavailable
	}}
	if err := o.publish(ctx, publisher); !errors.Is(err, unavailable) {
		t.Fatalf("publish error=%v", err)
	}
	if n, err := o.Pending(); err != nil || n != 2 {
		t.Fatalf("unacknowledged event lost: pending=%d err=%v", n, err)
	}
	publisher.publish = func(subject string, data []byte, _ ...nats.PubOpt) (*nats.PubAck, error) {
		var e events.Envelope
		if err := json.Unmarshal(data, &e); err != nil {
			t.Fatal(err)
		}
		if subject != events.Subject {
			t.Fatalf("subject=%q", subject)
		}
		if e.ID == "first" {
			return &nats.PubAck{Stream: events.Stream, Sequence: 1}, nil
		}
		return nil, unavailable
	}
	if err := o.publish(ctx, publisher); !errors.Is(err, unavailable) {
		t.Fatalf("publish error=%v", err)
	}
	var id string
	if n, err := o.Pending(); err != nil || n != 1 {
		t.Fatalf("acknowledged event retained: pending=%d err=%v", n, err)
	}
	if err := o.db.QueryRow(`SELECT id FROM event_outbox`).Scan(&id); err != nil || id != "second" {
		t.Fatalf("remaining event=%q err=%v", id, err)
	}
}
