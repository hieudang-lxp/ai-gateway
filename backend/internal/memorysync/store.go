package memorysync

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// Store is private sender state, never the receiver's job store. Receipt hashes
// remain after delivery; delivered plaintext is cleared to limit retention.
type Store struct{ db *sql.DB }

func OpenPostgres(dsn string) (*Store, error) { return openStore("pgx", dsn) }
func OpenSQLite(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: path}
	return openStore("sqlite", u.String()+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
}
func openStore(driver, dsn string) (*Store, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS memory_deliveries (
 identity TEXT PRIMARY KEY, source TEXT NOT NULL, payload_hash TEXT NOT NULL,
 payload TEXT NOT NULL, state TEXT NOT NULL, created_at TEXT NOT NULL, delivered_at TEXT NOT NULL DEFAULT ''
 ); CREATE INDEX IF NOT EXISTS memory_pending ON memory_deliveries(state,created_at);
 CREATE TABLE IF NOT EXISTS memory_delivery_errors (identity TEXT PRIMARY KEY, error TEXT NOT NULL);`); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func messageIdentity(m Message) string {
	b, _ := json.Marshal([]string{m.GroupID, m.Source, m.SessionID, m.UUID})
	return digest(string(b))
}
func (s *Store) Enqueue(ctx context.Context, m Message) error {
	if err := m.validate(); err != nil {
		return err
	}
	m.Text = strings.TrimSpace(m.Text)
	stamp, _ := time.Parse(time.RFC3339Nano, m.Timestamp)
	m.Timestamp = stamp.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
	payload, err := json.Marshal(m)
	if err != nil {
		return err
	}
	identity, hash := messageIdentity(m), digest(string(payload))
	_, err = s.db.ExecContext(ctx, `INSERT INTO memory_deliveries(identity,source,payload_hash,payload,state,created_at) VALUES($1,$2,$3,$4,'pending',$5) ON CONFLICT(identity) DO NOTHING`, identity, m.Source, hash, string(payload), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return errors.New("sender database enqueue failed")
	}
	var existing string
	if err = s.db.QueryRowContext(ctx, `SELECT payload_hash FROM memory_deliveries WHERE identity=$1`, identity).Scan(&existing); err != nil {
		return errors.New("sender receipt lookup failed")
	}
	if existing != hash {
		return errors.New("local identity conflict: original payload retained")
	}
	return nil
}
func (s *Store) Pending(ctx context.Context, limit int) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM memory_deliveries WHERE state='pending' ORDER BY created_at,identity LIMIT $1`, limit)
	if err != nil {
		return nil, errors.New("sender pending query failed")
	}
	defer rows.Close()
	messages := []Message{}
	for rows.Next() {
		var payload string
		if err = rows.Scan(&payload); err != nil {
			return nil, err
		}
		var m Message
		if err = json.Unmarshal([]byte(payload), &m); err != nil {
			return nil, errors.New("sender pending payload corrupt")
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
func (s *Store) Delivered(ctx context.Context, messages []Message) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("sender delivery transaction failed")
	}
	defer tx.Rollback()
	for _, m := range messages {
		if _, err = tx.ExecContext(ctx, `UPDATE memory_deliveries SET state='delivered',payload='',delivered_at=$1 WHERE identity=$2`, time.Now().UTC().Format(time.RFC3339Nano), messageIdentity(m)); err != nil {
			return errors.New("sender delivery checkpoint failed")
		}
	}
	if err = tx.Commit(); err != nil {
		return errors.New("sender delivery commit failed")
	}
	return nil
}

// Block retains the original payload and identity for operator inspection. A
// permanent invalid record must not prevent unrelated messages from delivery.
func (s *Store) Block(ctx context.Context, m Message, reason string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("sender quarantine transaction failed")
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE memory_deliveries SET state='blocked' WHERE identity=$1 AND state='pending'`, messageIdentity(m)); err != nil {
		return errors.New("sender quarantine failed")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO memory_delivery_errors(identity,error) VALUES($1,$2) ON CONFLICT(identity) DO UPDATE SET error=excluded.error`, messageIdentity(m), reason); err != nil {
		return errors.New("sender quarantine reason failed")
	}
	return tx.Commit()
}

type Counts struct {
	Pending   int    `json:"pending"`
	Delivered int    `json:"delivered"`
	Blocked   int    `json:"blocked"`
	Error     string `json:"error,omitempty"`
}

func (s *Store) Counts(ctx context.Context) (map[string]Counts, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT source,state,COUNT(*),COALESCE(MAX(e.error),'') FROM memory_deliveries d LEFT JOIN memory_delivery_errors e ON d.identity=e.identity GROUP BY source,state`)
	if err != nil {
		return nil, errors.New("sender database unavailable")
	}
	defer rows.Close()
	counts := map[string]Counts{}
	for rows.Next() {
		var source, state, reason string
		var n int
		if err = rows.Scan(&source, &state, &n, &reason); err != nil {
			return nil, fmt.Errorf("sender count decode failed")
		}
		c := counts[source]
		if state == "pending" {
			c.Pending += n
		} else if state == "blocked" {
			c.Pending += n
			c.Blocked += n
			c.Error = reason
		} else if state == "delivered" {
			c.Delivered = n
		}
		counts[source] = c
	}
	return counts, rows.Err()
}
