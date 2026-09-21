package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/eventbus"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func OpenPostgres(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err = db.ExecContext(ctx, postgresSchema+eventbus.PostgresSchema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, postgres: true}, nil
}

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

const postgresSchema = `
CREATE TABLE IF NOT EXISTS calls (
 id BIGSERIAL PRIMARY KEY, ts BIGINT NOT NULL, model TEXT NOT NULL,
 input_tokens BIGINT NOT NULL, output_tokens BIGINT NOT NULL,
 cache_read_tokens BIGINT NOT NULL, cache_write_tokens BIGINT NOT NULL,
 est_cost_usd DOUBLE PRECISION NOT NULL, latency_ms BIGINT NOT NULL, status INTEGER NOT NULL,
 routed_from TEXT NOT NULL DEFAULT '', cache_hit INTEGER NOT NULL DEFAULT 0,
 saved_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
 request_id TEXT NOT NULL DEFAULT '', request_model TEXT NOT NULL DEFAULT '',
 request_path TEXT NOT NULL DEFAULT '', model_source TEXT NOT NULL DEFAULT '',
 upstream_request_id TEXT NOT NULL DEFAULT '', local_id BIGINT UNIQUE
);
CREATE INDEX IF NOT EXISTS idx_calls_ts ON calls(ts);
CREATE TABLE IF NOT EXISTS cache (
 key TEXT PRIMARY KEY, created BIGINT NOT NULL, status INTEGER NOT NULL,
 content_type TEXT NOT NULL, body BYTEA NOT NULL, cost_usd DOUBLE PRECISION NOT NULL, model TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS gateway_event_state (
 id INTEGER PRIMARY KEY CHECK(id=1), origin TEXT NOT NULL, last_id BIGINT NOT NULL
);
CREATE TABLE IF NOT EXISTS external_usage (
 source TEXT NOT NULL, event_id TEXT NOT NULL, ts BIGINT NOT NULL, model TEXT NOT NULL,
 input_tokens BIGINT NOT NULL, output_tokens BIGINT NOT NULL,
 cache_read_tokens BIGINT NOT NULL, cache_write_tokens BIGINT NOT NULL,
 cost_usd DOUBLE PRECISION, cost_kind TEXT NOT NULL DEFAULT '', PRIMARY KEY(source,event_id)
);
CREATE INDEX IF NOT EXISTS idx_external_usage_ts ON external_usage(ts);
CREATE TABLE IF NOT EXISTS sync_state (
 id INTEGER PRIMARY KEY CHECK(id=1), last_synced BIGINT NOT NULL
);
CREATE TABLE IF NOT EXISTS budget_snapshot (
 id INTEGER PRIMARY KEY CHECK(id=1),
 daily_warn DOUBLE PRECISION NOT NULL, daily_hard DOUBLE PRECISION NOT NULL,
 weekly_warn DOUBLE PRECISION NOT NULL, weekly_hard DOUBLE PRECISION NOT NULL,
 monthly_warn DOUBLE PRECISION NOT NULL, monthly_hard DOUBLE PRECISION NOT NULL,
 updated_at BIGINT NOT NULL
);`
