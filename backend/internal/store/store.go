package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Usage is the token counts extracted from one Anthropic response.
type Usage struct {
	Input      int64
	Output     int64
	CacheRead  int64
	CacheWrite int64
}

// Record is one persisted call.
type Record struct {
	TS        time.Time
	Model     string
	Usage     Usage
	CostUSD   float64
	LatencyMS int64
	Status    int
}

// Store wraps the SQLite connection holding the calls table.
type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	// WAL + busy_timeout keep concurrent stream finalizations from tripping
	// "database is locked"; a single open connection serializes writes.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS calls (
	id                 INTEGER PRIMARY KEY AUTOINCREMENT,
	ts                 INTEGER NOT NULL,           -- unix seconds
	model              TEXT    NOT NULL,
	input_tokens       INTEGER NOT NULL,
	output_tokens      INTEGER NOT NULL,
	cache_read_tokens  INTEGER NOT NULL,
	cache_write_tokens INTEGER NOT NULL,
	est_cost_usd       REAL    NOT NULL,
	latency_ms         INTEGER NOT NULL,
	status             INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_calls_ts ON calls(ts);
`

func (s *Store) Insert(r Record) error {
	_, err := s.db.Exec(
		`INSERT INTO calls
		 (ts, model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, est_cost_usd, latency_ms, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.TS.Unix(), r.Model,
		r.Usage.Input, r.Usage.Output, r.Usage.CacheRead, r.Usage.CacheWrite,
		r.CostUSD, r.LatencyMS, r.Status,
	)
	return err
}

func (s *Store) Close() error { return s.db.Close() }

// StatRow is one model's aggregate over a time window.
type StatRow struct {
	Model      string
	Calls      int64
	Input      int64
	Output     int64
	CacheRead  int64
	CacheWrite int64
	CostUSD    float64
}

// StatsSince returns per-model aggregates for calls at or after the cutoff,
// ordered by cost descending.
func (s *Store) StatsSince(cutoff time.Time) ([]StatRow, error) {
	rows, err := s.db.Query(
		`SELECT model, COUNT(*), SUM(input_tokens), SUM(output_tokens),
		        SUM(cache_read_tokens), SUM(cache_write_tokens), SUM(est_cost_usd)
		 FROM calls WHERE ts >= ?
		 GROUP BY model ORDER BY SUM(est_cost_usd) DESC`,
		cutoff.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StatRow
	for rows.Next() {
		var r StatRow
		if err := rows.Scan(&r.Model, &r.Calls, &r.Input, &r.Output,
			&r.CacheRead, &r.CacheWrite, &r.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
