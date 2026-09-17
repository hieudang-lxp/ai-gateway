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
	TS         time.Time
	Model      string
	Usage      Usage
	CostUSD    float64
	LatencyMS  int64
	Status     int
	RoutedFrom string
	CacheHit   bool
	SavedUSD   float64
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
	return OpenDSN("sqlite", dsn)
}

// OpenDSN opens any database/sql driver speaking the sqlite dialect (local
// "sqlite", remote "libsql" for Turso) and ensures the base schema.
func OpenDSN(driver, dsn string) (*Store, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(usageSchema); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	// Migrate pre-existing DBs; "duplicate column" errors are expected and ignored.
	for _, ddl := range []string{
		`ALTER TABLE calls ADD COLUMN routed_from TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE calls ADD COLUMN cache_hit INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE calls ADD COLUMN saved_usd REAL NOT NULL DEFAULT 0`,
	} {
		_, _ = db.Exec(ddl)
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
	status             INTEGER NOT NULL,
	routed_from        TEXT    NOT NULL DEFAULT '',
	cache_hit          INTEGER NOT NULL DEFAULT 0,
	saved_usd          REAL    NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_calls_ts ON calls(ts);
CREATE TABLE IF NOT EXISTS cache (
	key          TEXT PRIMARY KEY,
	created      INTEGER NOT NULL,          -- unix seconds
	status       INTEGER NOT NULL,
	content_type TEXT    NOT NULL,
	body         BLOB    NOT NULL,
	cost_usd     REAL    NOT NULL,
	model        TEXT    NOT NULL
);
`

func (s *Store) Insert(r Record) error {
	_, err := s.db.Exec(
		`INSERT INTO calls
		 (ts, model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, est_cost_usd, latency_ms, status, routed_from, cache_hit, saved_usd)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.TS.Unix(), r.Model,
		r.Usage.Input, r.Usage.Output, r.Usage.CacheRead, r.Usage.CacheWrite,
		r.CostUSD, r.LatencyMS, r.Status,
		r.RoutedFrom, boolToInt(r.CacheHit), r.SavedUSD,
	)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
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

// SpendSince returns total estimated cost of calls at or after the cutoff.
func (s *Store) SpendSince(cutoff time.Time) (float64, error) {
	var v float64
	err := s.db.QueryRow(
		`SELECT COALESCE(SUM(est_cost_usd), 0) FROM calls WHERE ts >= ?`,
		cutoff.Unix(),
	).Scan(&v)
	return v, err
}

// CachedResponse is one stored upstream response, replayable byte-for-byte.
type CachedResponse struct {
	Status      int
	ContentType string
	Body        []byte
	CostUSD     float64
	Model       string
}

func (s *Store) CachePut(key string, c CachedResponse) error {
	body := c.Body
	if body == nil {
		body = []byte{}
	}
	_, err := s.db.Exec(
		`INSERT INTO cache (key, created, status, content_type, body, cost_usd, model)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET created=excluded.created, status=excluded.status,
		   content_type=excluded.content_type, body=excluded.body,
		   cost_usd=excluded.cost_usd, model=excluded.model`,
		key, time.Now().Unix(), c.Status, c.ContentType, body, c.CostUSD, c.Model,
	)
	return err
}

// CacheGet returns the entry if it exists and is younger than maxAge.
// Stale entries are deleted lazily.
func (s *Store) CacheGet(key string, maxAge time.Duration) (*CachedResponse, bool, error) {
	var c CachedResponse
	var created int64
	err := s.db.QueryRow(
		`SELECT created, status, content_type, body, cost_usd, model FROM cache WHERE key = ?`, key,
	).Scan(&created, &c.Status, &c.ContentType, &c.Body, &c.CostUSD, &c.Model)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if time.Since(time.Unix(created, 0)) > maxAge {
		_, _ = s.db.Exec(`DELETE FROM cache WHERE key = ?`, key)
		return nil, false, nil
	}
	return &c, true, nil
}

// CacheSavings returns how many cache hits were served and the total USD saved.
func (s *Store) CacheSavings() (hits int64, saved float64, err error) {
	err = s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(saved_usd), 0) FROM calls WHERE cache_hit = 1`,
	).Scan(&hits, &saved)
	return
}

// EnsureSyncSchema prepares a REMOTE store: dedupe column on calls plus the
// budget snapshot table the cloud API reads limits from.
func (s *Store) EnsureSyncSchema() error {
	_, _ = s.db.Exec(`ALTER TABLE calls ADD COLUMN local_id INTEGER`)
	if _, err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_calls_local_id ON calls(local_id)`); err != nil {
		return err
	}
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS budget_snapshot (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		daily_warn REAL NOT NULL, daily_hard REAL NOT NULL,
		weekly_warn REAL NOT NULL, weekly_hard REAL NOT NULL,
		monthly_warn REAL NOT NULL, monthly_hard REAL NOT NULL,
		updated_at INTEGER NOT NULL
	)`)
	return err
}

// InsertSynced writes one local row into a remote store, deduped on local_id.
func (s *Store) InsertSynced(c Call) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO calls
		 (local_id, ts, model, routed_from, input_tokens, output_tokens,
		  cache_read_tokens, cache_write_tokens, est_cost_usd, latency_ms, status, cache_hit, saved_usd)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.TS.Unix(), c.Model, c.RoutedFrom,
		c.Usage.Input, c.Usage.Output, c.Usage.CacheRead, c.Usage.CacheWrite,
		c.CostUSD, c.LatencyMS, c.Status, boolToInt(c.CacheHit), c.SavedUSD,
	)
	return err
}

func (s *Store) LastSyncedID() (int64, error) {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS sync_state (id INTEGER PRIMARY KEY CHECK (id = 1), last_synced INTEGER NOT NULL)`); err != nil {
		return 0, err
	}
	var v int64
	err := s.db.QueryRow(`SELECT last_synced FROM sync_state WHERE id = 1`).Scan(&v)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return v, err
}

func (s *Store) SetLastSyncedID(id int64) error {
	_, err := s.db.Exec(
		`INSERT INTO sync_state (id, last_synced) VALUES (1, ?)
		 ON CONFLICT(id) DO UPDATE SET last_synced = excluded.last_synced`, id)
	return err
}

func (s *Store) WriteBudgetSnapshot(dw, dh, ww, wh, mw, mh float64) error {
	_, err := s.db.Exec(
		`INSERT INTO budget_snapshot (id, daily_warn, daily_hard, weekly_warn, weekly_hard, monthly_warn, monthly_hard, updated_at)
		 VALUES (1, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET daily_warn=excluded.daily_warn, daily_hard=excluded.daily_hard,
		   weekly_warn=excluded.weekly_warn, weekly_hard=excluded.weekly_hard,
		   monthly_warn=excluded.monthly_warn, monthly_hard=excluded.monthly_hard,
		   updated_at=excluded.updated_at`,
		dw, dh, ww, wh, mw, mh, time.Now().Unix())
	return err
}

func (s *Store) ReadBudgetSnapshot() (dw, dh, ww, wh, mw, mh float64, ok bool, err error) {
	err = s.db.QueryRow(
		`SELECT daily_warn, daily_hard, weekly_warn, weekly_hard, monthly_warn, monthly_hard FROM budget_snapshot WHERE id = 1`,
	).Scan(&dw, &dh, &ww, &wh, &mw, &mh)
	if err == sql.ErrNoRows {
		return 0, 0, 0, 0, 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, 0, 0, 0, 0, false, err
	}
	return dw, dh, ww, wh, mw, mh, true, nil
}
