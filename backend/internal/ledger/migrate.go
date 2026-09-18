package ledger

import (
	"database/sql"
	"fmt"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	_ "modernc.org/sqlite"
	"time"
)

// ImportSQLite is an explicit, read-only migration boundary, never a runtime
// cross-service database dependency. Existing identities make reruns safe.
func (s *Store) ImportSQLite(path string) (int, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT source,event_id,ts,model,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,cost_usd,cost_kind FROM external_usage ORDER BY source,event_id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	batch := []events.ExternalUsage{}
	for rows.Next() {
		var r events.ExternalUsage
		var ts int64
		var price sql.NullFloat64
		if err = rows.Scan(&r.Source, &r.ID, &ts, &r.Model, &r.Usage.Input, &r.Usage.Output, &r.Usage.CacheRead, &r.Usage.CacheWrite, &price, &r.CostKind); err != nil {
			return n, err
		}
		r.TS = time.Unix(ts, 0)
		if price.Valid {
			r.CostUSD = &price.Float64
		}
		batch = append(batch, r)
		n++
		if len(batch) == 100 {
			if err = s.ImportUsage(batch); err != nil {
				return n, err
			}
			batch = nil
		}
	}
	if err = rows.Err(); err != nil {
		return n, err
	}
	if len(batch) > 0 {
		err = s.ImportUsage(batch)
	}
	if err != nil {
		return n, fmt.Errorf("import usage: %w", err)
	}
	return n, nil
}
