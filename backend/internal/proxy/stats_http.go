package proxy

import (
	"encoding/json"
	"net/http"
	"time"
)

func (g *Gateway) HandleStats(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	windows := map[string]time.Time{
		"today":    todayStart,
		"last_7d":  now.AddDate(0, 0, -7),
		"all_time": time.Unix(0, 0),
	}
	out := map[string]any{}
	for name, cutoff := range windows {
		rows, err := g.store.StatsSince(cutoff)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		models := make([]map[string]any, 0, len(rows))
		var totalCost float64
		var totalCalls int64
		for _, s := range rows {
			models = append(models, map[string]any{
				"model":              s.Model,
				"calls":              s.Calls,
				"input_tokens":       s.Input,
				"output_tokens":      s.Output,
				"cache_read_tokens":  s.CacheRead,
				"cache_write_tokens": s.CacheWrite,
				"est_cost_usd":       s.CostUSD,
			})
			totalCost += s.CostUSD
			totalCalls += s.Calls
		}
		out[name] = map[string]any{
			"total_calls":    totalCalls,
			"total_cost_usd": totalCost,
			"by_model":       models,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
