package store

import "time"

type CostEvent struct {
	TS      int64
	CostUSD float64
}

func (s *Store) CostEvents(cutoff time.Time) ([]CostEvent, error) {
	rows, err := s.db.Query(
		`SELECT ts, est_cost_usd FROM calls WHERE ts >= $1 ORDER BY ts ASC`, cutoff.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CostEvent
	for rows.Next() {
		var e CostEvent
		if err := rows.Scan(&e.TS, &e.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type Call struct {
	ID int64
	Record
}

const callCols = `id, ts, model, routed_from, input_tokens, output_tokens,
	cache_read_tokens, cache_write_tokens, est_cost_usd, latency_ms, status, cache_hit, saved_usd,request_id,request_model,request_path,model_source,upstream_request_id`

func (s *Store) scanCalls(query string, args ...any) ([]Call, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Call
	for rows.Next() {
		var c Call
		var ts int64
		var cacheHit int
		if err := rows.Scan(&c.ID, &ts, &c.Model, &c.RoutedFrom,
			&c.Usage.Input, &c.Usage.Output, &c.Usage.CacheRead, &c.Usage.CacheWrite,
			&c.CostUSD, &c.LatencyMS, &c.Status, &cacheHit, &c.SavedUSD, &c.RequestID, &c.RequestModel, &c.RequestPath, &c.ModelSource, &c.UpstreamRequestID); err != nil {
			return nil, err
		}
		c.TS = time.Unix(ts, 0)
		c.CacheHit = cacheHit == 1
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) RecentCalls(limit int, beforeID int64) ([]Call, error) {
	if beforeID > 0 {
		return s.scanCalls(`SELECT `+callCols+` FROM calls WHERE id < $1 ORDER BY id DESC LIMIT $2`, beforeID, limit)
	}
	return s.scanCalls(`SELECT `+callCols+` FROM calls ORDER BY id DESC LIMIT $1`, limit)
}

func (s *Store) RecentDashboardCalls(limit int, beforeID int64) ([]Call, error) {
	return s.scanCalls(`SELECT `+callCols+` FROM calls
		WHERE (CAST($1 AS BIGINT) <= 0 OR id < $2)
		AND NOT (status = 429 AND input_tokens = 0 AND output_tokens = 0
			AND cache_read_tokens = 0 AND cache_write_tokens = 0 AND est_cost_usd = 0)
		ORDER BY id DESC LIMIT $3`, beforeID, beforeID, limit)
}

func (s *Store) CallsAfter(afterID int64, limit int) ([]Call, error) {
	return s.scanCalls(`SELECT `+callCols+` FROM calls WHERE id > $1 ORDER BY id ASC LIMIT $2`, afterID, limit)
}

func (s *Store) TotalCalls() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM calls`).Scan(&n)
	return n, err
}
