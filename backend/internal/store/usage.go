package store

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	"time"
)

// ExternalUsage contains accounting metadata only, never conversation content.
type ExternalUsage = events.ExternalUsage

const usageSchema = `CREATE TABLE IF NOT EXISTS external_usage (
 source TEXT NOT NULL, event_id TEXT NOT NULL, ts INTEGER NOT NULL, model TEXT NOT NULL,
 input_tokens INTEGER NOT NULL, output_tokens INTEGER NOT NULL,
 cache_read_tokens INTEGER NOT NULL, cache_write_tokens INTEGER NOT NULL,
 cost_usd REAL, cost_kind TEXT NOT NULL DEFAULT '', PRIMARY KEY(source,event_id)
);
CREATE INDEX IF NOT EXISTS idx_external_usage_ts ON external_usage(ts);`

func (s *Store) ImportUsage(events []ExternalUsage) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO external_usage VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
 ON CONFLICT(source,event_id) DO UPDATE SET
 input_tokens=excluded.input_tokens,output_tokens=excluded.output_tokens,
 cache_read_tokens=excluded.cache_read_tokens,cache_write_tokens=excluded.cache_write_tokens,
 cost_usd=excluded.cost_usd,cost_kind=excluded.cost_kind,model=excluded.model
 WHERE excluded.input_tokens+excluded.output_tokens+excluded.cache_read_tokens+excluded.cache_write_tokens >=
 external_usage.input_tokens+external_usage.output_tokens+external_usage.cache_read_tokens+external_usage.cache_write_tokens`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range events {
		if r.Source == "" || r.ID == "" || r.TS.IsZero() {
			return fmt.Errorf("invalid usage identity")
		}
		for _, alias := range r.Aliases {
			if alias != r.ID {
				if _, err = tx.Exec(`DELETE FROM external_usage WHERE source=$1 AND event_id=$2`, r.Source, alias); err != nil {
					return err
				}
			}
		}
		if _, err = stmt.Exec(r.Source, r.ID, r.TS.Unix(), r.Model, r.Usage.Input, r.Usage.Output, r.Usage.CacheRead, r.Usage.CacheWrite, r.CostUSD, r.CostKind); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type UsageRow struct {
	FallbackCostCalls  int64   `json:"fallback_cost_calls"`
	Source             string  `json:"source"`
	Model              string  `json:"model"`
	Calls              int64   `json:"calls"`
	Input              int64   `json:"input_tokens"`
	Output             int64   `json:"output_tokens"`
	CacheRead          int64   `json:"cache_read_tokens"`
	CacheWrite         int64   `json:"cache_write_tokens"`
	KnownCostUSD       float64 `json:"known_cost_usd"`
	UnknownCostCalls   int64   `json:"unknown_cost_calls"`
	EstimatedCostCalls int64   `json:"estimated_cost_calls"`
	FirstTS            int64   `json:"first_ts"`
	LastTS             int64   `json:"last_ts"`
}

// Claude transcripts are the canonical source when available. Gateway calls
// remain in their original tables for proxy budgets and diagnostics, but are
// not added again to transcript usage.
func (s *Store) UnifiedUsageSince(cutoff time.Time) ([]UsageRow, error) {
	return unifiedUsageSince(s.db, cutoff)
}

type usageQuerier interface {
	Query(string, ...any) (*sql.Rows, error)
}

func unifiedUsageSince(db usageQuerier, cutoff time.Time) ([]UsageRow, error) {
	rows, err := db.Query(`WITH ledger AS (
 SELECT source,model,ts,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,cost_usd,cost_kind FROM external_usage
 UNION ALL SELECT 'claude_gateway',model,ts,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,est_cost_usd,'estimated'
 FROM calls WHERE NOT EXISTS (SELECT 1 FROM external_usage WHERE source='claude_code')
 ) SELECT source,model,COUNT(*),SUM(input_tokens),SUM(output_tokens),SUM(cache_read_tokens),SUM(cache_write_tokens),
 COALESCE(SUM(cost_usd),0),SUM(CASE WHEN cost_usd IS NULL THEN 1 ELSE 0 END),
 SUM(CASE WHEN cost_kind='estimated' THEN 1 ELSE 0 END),MIN(ts),MAX(ts)
 FROM ledger WHERE ts>=$1 GROUP BY source,model ORDER BY source,SUM(input_tokens+output_tokens+cache_read_tokens+cache_write_tokens) DESC`, cutoff.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UsageRow{}
	for rows.Next() {
		var r UsageRow
		if err := rows.Scan(&r.Source, &r.Model, &r.Calls, &r.Input, &r.Output, &r.CacheRead, &r.CacheWrite, &r.KnownCostUSD, &r.UnknownCostCalls, &r.EstimatedCostCalls, &r.FirstTS, &r.LastTS); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CodexEstimatesSince reprices retained requests using one immutable catalog
// snapshot. Never apply context tiers to monthly aggregate token counts.
func (s *Store) CodexEstimatesSince(cutoff time.Time, cost func(string, int64, int64, int64, int64) (float64, bool)) (map[string]float64, map[string]int64, error) {
	return codexEstimatesSince(s.db, cutoff, cost)
}
func codexEstimatesSince(db usageQuerier, cutoff time.Time, cost func(string, int64, int64, int64, int64) (float64, bool)) (map[string]float64, map[string]int64, error) {
	rows, err := db.Query(`SELECT model,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens FROM external_usage WHERE source='codex' AND ts>=$1`, cutoff.Unix())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	totals := map[string]float64{}
	fallbacks := map[string]int64{}
	for rows.Next() {
		var model string
		var in, out, read, write int64
		if err = rows.Scan(&model, &in, &out, &read, &write); err != nil {
			return nil, nil, err
		}
		value, fallback := cost(model, in, out, read, write)
		totals[model] += value
		if fallback {
			fallbacks[model]++
		}
	}
	return totals, fallbacks, rows.Err()
}

// PricedUsageSince holds a consistent read snapshot while collectors import.
func (s *Store) PricedUsageSince(cutoff time.Time, cost func(string, int64, int64, int64, int64) (float64, bool)) ([]UsageRow, error) {
	var opts *sql.TxOptions
	if s.postgres {
		opts = &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}
	}
	tx, err := s.db.BeginTx(context.Background(), opts)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := unifiedUsageSince(tx, cutoff)
	if err != nil {
		return nil, err
	}
	costs, fallbacks, err := codexEstimatesSince(tx, cutoff, cost)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].Source == "codex" {
			rows[i].KnownCostUSD = costs[rows[i].Model]
			rows[i].UnknownCostCalls = 0
			rows[i].EstimatedCostCalls = rows[i].Calls
			rows[i].FallbackCostCalls = fallbacks[rows[i].Model]
		}
	}
	return rows, tx.Commit()
}
