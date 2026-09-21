package explore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const schema = `
CREATE TABLE IF NOT EXISTS records (
 source TEXT NOT NULL,event_id TEXT NOT NULL,ts BIGINT NOT NULL,model TEXT NOT NULL,
 input_tokens BIGINT NOT NULL,output_tokens BIGINT NOT NULL,cache_read_tokens BIGINT NOT NULL,cache_write_tokens BIGINT NOT NULL,
 cost_usd DOUBLE PRECISION,cost_kind TEXT NOT NULL,
 session_id TEXT NOT NULL DEFAULT '',title TEXT NOT NULL DEFAULT '',project TEXT NOT NULL DEFAULT '',metadata_revision BIGINT NOT NULL DEFAULT 0,
 search_document TSVECTOR GENERATED ALWAYS AS (to_tsvector('simple',title || ' ' || replace(project,'/',' ') || ' ' || session_id || ' ' || model)) STORED,
 PRIMARY KEY(source,event_id));
CREATE INDEX IF NOT EXISTS records_search ON records USING GIN(search_document);
CREATE INDEX IF NOT EXISTS records_session_prefix ON records(lower(session_id) text_pattern_ops);
CREATE INDEX IF NOT EXISTS records_source_time_model ON records(source,ts,model);
CREATE INDEX IF NOT EXISTS records_time ON records(ts);
CREATE INDEX IF NOT EXISTS records_model_time ON records(model,ts);
CREATE INDEX IF NOT EXISTS records_timeline ON records(source,session_id,ts,event_id COLLATE "C");
CREATE TABLE IF NOT EXISTS event_receipts(id TEXT PRIMARY KEY,received_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS aliases(source TEXT NOT NULL,event_id TEXT NOT NULL,PRIMARY KEY(source,event_id));
CREATE TABLE IF NOT EXISTS rejected_events(payload_hash TEXT PRIMARY KEY,subject TEXT NOT NULL,reason TEXT NOT NULL,rejected_at TIMESTAMPTZ NOT NULL DEFAULT now());`

type Store struct {
	db           *sql.DB
	sessionsOnly bool
}

func Open(dsn string, sessionsOnly bool) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err = db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize exploration projection: %w", err)
	}
	return &Store{db: db, sessionsOnly: sessionsOnly}, nil
}
func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *Store) Quarantine(hash, subject, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `INSERT INTO rejected_events(payload_hash,subject,reason) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, hash, subject, reason)
	return err
}

const recordColumns = `source,event_id,ts,model,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,cost_usd,cost_kind,session_id,title,project,metadata_revision`

type record struct {
	source, id, model, kind, session, title, project string
	ts, in, out, read, write, revision               int64
	cost                                             sql.NullFloat64
}
type scanner interface{ Scan(...any) error }

func scanRecord(s scanner) (record, error) {
	var r record
	err := s.Scan(&r.source, &r.id, &r.ts, &r.model, &r.in, &r.out, &r.read, &r.write, &r.cost, &r.kind, &r.session, &r.title, &r.project, &r.revision)
	return r, err
}
func (r record) tokens() int64 { return r.in + r.out + r.read + r.write }
func fromEvent(e events.ExternalUsage) record {
	r := record{source: e.Source, id: e.ID, model: e.Model, kind: e.CostKind, session: e.SessionID, title: e.SessionTitle, project: e.Project, ts: e.TS.Unix(), in: e.Usage.Input, out: e.Usage.Output, read: e.Usage.CacheRead, write: e.Usage.CacheWrite}
	if e.CostUSD != nil {
		r.cost = sql.NullFloat64{Float64: *e.CostUSD, Valid: true}
	}
	if !e.SessionUpdatedAt.IsZero() {
		r.revision = e.SessionUpdatedAt.UnixNano()
	}
	return r
}

func merge(existing, incoming record) record {
	result := existing
	if incoming.tokens() > existing.tokens() || (incoming.tokens() == existing.tokens() && incoming.revision > existing.revision) {
		result.model = incoming.model
		result.in = incoming.in
		result.out = incoming.out
		result.read = incoming.read
		result.write = incoming.write
		result.cost = incoming.cost
		result.kind = incoming.kind
	}
	if incoming.tokens() == existing.tokens() {
		if result.model == "" {
			result.model = incoming.model
		}
		if incoming.cost.Valid && (!result.cost.Valid || incoming.revision > existing.revision) {
			result.cost = incoming.cost
			result.kind = incoming.kind
		}
	}
	return mergeMetadata(result, incoming)
}

func mergeMetadata(result, incoming record) record {
	newer := incoming.revision > result.revision
	if incoming.session != "" && (result.session == "" || newer) {
		result.session = incoming.session
	}
	if incoming.title != "" && (result.title == "" || newer) {
		result.title = incoming.title
	}
	if incoming.project != "" && (result.project == "" || newer) {
		result.project = incoming.project
	}
	result.revision = max(result.revision, incoming.revision)
	return result
}
func (s *Store) Apply(e events.Envelope) error {
	if err := events.Validate(e); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(724619)`); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO event_receipts(id) VALUES($1) ON CONFLICT DO NOTHING`, e.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return tx.Commit()
	}
	for _, event := range e.Rows {
		if s.sessionsOnly && event.Source == "claude_gateway" {
			continue
		}
		var retired bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM aliases WHERE source=$1 AND event_id=$2)`, event.Source, event.ID).Scan(&retired); err != nil {
			return err
		}
		if retired {
			continue
		}
		incoming := fromEvent(event)
		current := incoming
		stored, loadErr := scanRecord(tx.QueryRowContext(ctx, `SELECT `+recordColumns+` FROM records WHERE source=$1 AND event_id=$2`, event.Source, event.ID))
		if loadErr == nil {
			current = merge(stored, incoming)
		} else if loadErr != sql.ErrNoRows {
			return loadErr
		}
		for _, alias := range event.Aliases {
			if alias == event.ID || alias == "" {
				continue
			}
			previous, loadErr := scanRecord(tx.QueryRowContext(ctx, `SELECT `+recordColumns+` FROM records WHERE source=$1 AND event_id=$2`, event.Source, alias))
			if loadErr == nil {
				current = mergeMetadata(current, previous)
			} else if loadErr != sql.ErrNoRows {
				return loadErr
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO aliases(source,event_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, event.Source, alias); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `DELETE FROM records WHERE source=$1 AND event_id=$2`, event.Source, alias); err != nil {
				return err
			}
		}
		if loadErr == nil && current == stored {
			continue
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO records(`+recordColumns+`) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
 ON CONFLICT(source,event_id) DO UPDATE SET model=excluded.model,input_tokens=excluded.input_tokens,output_tokens=excluded.output_tokens,cache_read_tokens=excluded.cache_read_tokens,cache_write_tokens=excluded.cache_write_tokens,cost_usd=excluded.cost_usd,cost_kind=excluded.cost_kind,session_id=excluded.session_id,title=excluded.title,project=excluded.project,metadata_revision=excluded.metadata_revision`, current.source, current.id, current.ts, current.model, current.in, current.out, current.read, current.write, current.cost, current.kind, current.session, current.title, current.project, current.revision)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
