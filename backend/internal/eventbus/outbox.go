package eventbus

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
	_ "modernc.org/sqlite"
)

const Schema = `CREATE TABLE IF NOT EXISTS event_outbox (seq INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT UNIQUE NOT NULL, subject TEXT NOT NULL, payload BLOB NOT NULL);`

const PostgresSchema = `CREATE TABLE IF NOT EXISTS event_outbox (seq BIGSERIAL PRIMARY KEY, id TEXT UNIQUE NOT NULL, subject TEXT NOT NULL, payload BYTEA NOT NULL);`

type Outbox struct{ db *sql.DB }

func Open(path string) (*Outbox, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(Schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Outbox{db: db}, nil
}

func OpenPostgres(dsn string) (*Outbox, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err = db.ExecContext(ctx, PostgresSchema); err != nil {
		db.Close()
		return nil, err
	}
	return &Outbox{db: db}, nil
}

func Wrap(db *sql.DB) *Outbox  { return &Outbox{db: db} }
func (o *Outbox) Close() error { return o.db.Close() }

func Enqueue(tx *sql.Tx, subject string, e events.Envelope) error {
	e.Version = 1
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if err := events.Validate(e); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO event_outbox(id,subject,payload) VALUES($1,$2,$3)`, e.ID, subject, b)
	return err
}

func (o *Outbox) ImportUsage(rows []events.ExternalUsage) error {
	if err := events.ValidateRows(rows); err != nil {
		return err
	}
	tx, err := o.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for start := 0; start < len(rows); start += 100 {
		end := min(start+100, len(rows))
		if err := Enqueue(tx, events.Subject, events.Envelope{Rows: rows[start:end]}); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (o *Outbox) RecordStatus(source string, status events.SourceStatus) error {
	tx, err := o.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := Enqueue(tx, events.StatusSubject, events.Envelope{Source: source, Status: &status}); err != nil {
		return err
	}
	return tx.Commit()
}
func (o *Outbox) Pending() (int, error) {
	var n int
	err := o.db.QueryRow(`SELECT count(*) FROM event_outbox`).Scan(&n)
	return n, err
}

func Connect(url string) (*nats.Conn, nats.JetStreamContext, error) {
	nc, err := nats.Connect(url, nats.Timeout(5*time.Second), nats.MaxReconnects(-1))
	if err != nil {
		return nil, nil, err
	}
	js, err := nc.JetStream(nats.MaxWait(5 * time.Second))
	if err != nil {
		nc.Close()
		return nil, nil, err
	}
	_, err = js.StreamInfo(events.Stream)
	if err == nats.ErrStreamNotFound {
		_, err = js.AddStream(&nats.StreamConfig{Name: events.Stream, Subjects: []string{events.Subject, events.StatusSubject}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, MaxAge: 30 * 24 * time.Hour, MaxBytes: 2 << 30, Discard: nats.DiscardNew})
		if err != nil {
			_, err = js.StreamInfo(events.Stream)
		}
	}
	if err != nil {
		nc.Close()
		return nil, nil, err
	}
	return nc, js, nil
}

func (o *Outbox) Pump(ctx context.Context, url string) {
	for ctx.Err() == nil {
		nc, js, err := Connect(url)
		if err == nil {
			err = o.publish(ctx, js)
			nc.Close()
		}
		if err != nil && ctx.Err() == nil {
			log.Printf("outbox waiting for NATS: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}
func (o *Outbox) publish(ctx context.Context, js nats.JetStreamContext) error {
	for ctx.Err() == nil {
		var seq int64
		var id, subject string
		var body []byte
		err := o.db.QueryRowContext(ctx, `SELECT seq,id,subject,payload FROM event_outbox ORDER BY seq LIMIT 1`).Scan(&seq, &id, &subject, &body)
		if err == sql.ErrNoRows {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
			continue
		}
		if err != nil {
			return err
		}
		if _, err = js.Publish(subject, body, nats.MsgId(id), nats.Context(ctx)); err != nil {
			return err
		}
		if _, err = o.db.ExecContext(ctx, `DELETE FROM event_outbox WHERE seq=$1`, seq); err != nil {
			return err
		}
	}
	return ctx.Err()
}
