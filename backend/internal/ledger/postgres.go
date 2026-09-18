// Package ledger owns the usage service database. Other services never query it.
package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS usage_events (
 source TEXT NOT NULL,event_id TEXT NOT NULL,ts BIGINT NOT NULL,model TEXT NOT NULL,
 input_tokens BIGINT NOT NULL,output_tokens BIGINT NOT NULL,cache_read_tokens BIGINT NOT NULL,cache_write_tokens BIGINT NOT NULL,
 cost_usd DOUBLE PRECISION,cost_kind TEXT NOT NULL,PRIMARY KEY(source,event_id));
CREATE INDEX IF NOT EXISTS usage_events_ts ON usage_events(ts);
CREATE TABLE IF NOT EXISTS usage_aliases (source TEXT NOT NULL,event_id TEXT NOT NULL,PRIMARY KEY(source,event_id));
CREATE TABLE IF NOT EXISTS event_receipts (id TEXT PRIMARY KEY,received_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS collector_status (source TEXT PRIMARY KEY,attempt TIMESTAMPTZ NOT NULL,payload JSONB NOT NULL);
CREATE TABLE IF NOT EXISTS rejected_events (payload_hash TEXT PRIMARY KEY,subject TEXT NOT NULL,reason TEXT NOT NULL,rejected_at TIMESTAMPTZ NOT NULL DEFAULT now());
`

func Open(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(6)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err = db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize usage database: %w", err)
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *Store) ImportUsage(rows []events.ExternalUsage) error {
	return s.Apply(events.Envelope{Version: 1, ID: uuid.NewString(), Rows: rows})
}

func (s *Store) Apply(e events.Envelope) error {
	if err := events.Validate(e); err != nil {
		return err
	}
	if e.Version != 1 || e.ID == "" {
		return fmt.Errorf("invalid event envelope")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO event_receipts(id) VALUES($1) ON CONFLICT DO NOTHING`, e.ID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return tx.Commit()
	}
	for _, r := range e.Rows {
		if r.ID == "" || r.Source == "" || r.TS.IsZero() || r.Usage.Input < 0 || r.Usage.Output < 0 || r.Usage.CacheRead < 0 || r.Usage.CacheWrite < 0 {
			return fmt.Errorf("invalid usage record")
		}
		for _, alias := range r.Aliases {
			if alias == r.ID {
				continue
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO usage_aliases(source,event_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, r.Source, alias); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `DELETE FROM usage_events WHERE source=$1 AND event_id=$2`, r.Source, alias); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO usage_events SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10
   WHERE NOT EXISTS(SELECT 1 FROM usage_aliases WHERE source=$1 AND event_id=$2)
   ON CONFLICT(source,event_id) DO UPDATE SET model=excluded.model,input_tokens=excluded.input_tokens,output_tokens=excluded.output_tokens,
   cache_read_tokens=excluded.cache_read_tokens,cache_write_tokens=excluded.cache_write_tokens,cost_usd=excluded.cost_usd,cost_kind=excluded.cost_kind
   WHERE excluded.input_tokens+excluded.output_tokens+excluded.cache_read_tokens+excluded.cache_write_tokens>=usage_events.input_tokens+usage_events.output_tokens+usage_events.cache_read_tokens+usage_events.cache_write_tokens`,
			r.Source, r.ID, r.TS.Unix(), r.Model, r.Usage.Input, r.Usage.Output, r.Usage.CacheRead, r.Usage.CacheWrite, r.CostUSD, r.CostKind)
		if err != nil {
			return err
		}
	}
	if e.Status != nil {
		if e.Source != "codex" && e.Source != "claude_code" && e.Source != "cursor" {
			return fmt.Errorf("invalid collector source")
		}
		b, err := json.Marshal(e.Status)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO collector_status(source,attempt,payload) VALUES($1,$2,$3) ON CONFLICT(source) DO UPDATE SET attempt=excluded.attempt,payload=excluded.payload WHERE excluded.attempt>=collector_status.attempt`, e.Source, e.Status.LastAttempt, b); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Keep only a fingerprint and safe reason, never malformed message content.
func (s *Store) Quarantine(hash, subject, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `INSERT INTO rejected_events(payload_hash,subject,reason) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, hash, subject, reason)
	return err
}

func (s *Store) Statuses() (map[string]events.SourceStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT source,payload FROM collector_status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]events.SourceStatus{}
	for _, id := range []string{"codex", "claude_code", "cursor"} {
		interval := 60
		if id == "cursor" {
			interval = 300
		}
		out[id] = events.SourceStatus{State: "starting", PollSeconds: interval}
	}
	for rows.Next() {
		var id string
		var b []byte
		var st events.SourceStatus
		if err = rows.Scan(&id, &b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &st); err != nil {
			return nil, err
		}
		out[id] = st
	}
	return out, rows.Err()
}

// One SELECT gives a consistent snapshot; pricing is applied per request.
func (s *Store) PricedUsageSince(cutoff time.Time, cost func(string, int64, int64, int64, int64) (float64, bool)) ([]store.UsageRow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT source,model,ts,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,cost_usd,cost_kind FROM usage_events WHERE ts >= $1 AND (source!='claude_gateway' OR NOT EXISTS(SELECT 1 FROM usage_events WHERE source='claude_code')) ORDER BY source,event_id`, cutoff.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grouped := map[[2]string]*store.UsageRow{}
	for rows.Next() {
		var src, model, kind string
		var ts, in, out, read, write int64
		var price sql.NullFloat64
		if err = rows.Scan(&src, &model, &ts, &in, &out, &read, &write, &price, &kind); err != nil {
			return nil, err
		}
		key := [2]string{src, model}
		r := grouped[key]
		if r == nil {
			r = &store.UsageRow{Source: src, Model: model, FirstTS: ts, LastTS: ts}
			grouped[key] = r
		}
		r.Calls++
		r.Input += in
		r.Output += out
		r.CacheRead += read
		r.CacheWrite += write
		r.FirstTS = min(r.FirstTS, ts)
		r.LastTS = max(r.LastTS, ts)
		if src == "codex" {
			v, fallback := cost(model, in, out, read, write)
			r.KnownCostUSD += v
			r.EstimatedCostCalls++
			if fallback {
				r.FallbackCostCalls++
			}
		} else if price.Valid {
			r.KnownCostUSD += price.Float64
			if kind == "estimated" {
				r.EstimatedCostCalls++
			}
		} else {
			r.UnknownCostCalls++
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := make([]store.UsageRow, 0, len(grouped))
	for _, r := range grouped {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		at, bt := a.Input+a.Output+a.CacheRead+a.CacheWrite, b.Input+b.Output+b.CacheRead+b.CacheWrite
		if at == bt {
			return a.Model < b.Model
		}
		return at > bt
	})
	return out, nil
}
