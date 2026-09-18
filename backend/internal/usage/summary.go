package usage

import (
	"encoding/json"
	"fmt"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func summaryCutoff(now time.Time, query url.Values) (time.Time, int, error) {
	period, rawDays := query.Get("period"), query.Get("days")
	if period != "" && period != "month" {
		return time.Time{}, 0, fmt.Errorf("period must be month")
	}
	if period != "" && rawDays != "" {
		return time.Time{}, 0, fmt.Errorf("choose period or days, not both")
	}
	if rawDays == "" {
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), now.Day(), nil
	}
	days, err := strconv.Atoi(rawDays)
	if err != nil || days < 0 || days > 3650 {
		return time.Time{}, 0, fmt.Errorf("days must be 0–3650")
	}
	if days == 0 {
		return time.Unix(0, 0), 0, nil
	}
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -days+1), days, nil
}

func (c *Collector) HandleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", 405)
		return
	}
	now := time.Now()
	cutoff, days, err := summaryCutoff(now, r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	snapshot := c.catalog.Snapshot()
	rows, err := c.store.PricedUsageSince(cutoff, snapshot.Cost)
	if err != nil {
		http.Error(w, "usage database query failed", 500)
		return
	}
	c.mu.RLock()
	statuses := make(map[string]SourceStatus, len(c.status))
	for k, v := range c.status {
		statuses[k] = v
	}
	c.mu.RUnlock()
	if c.statuses != nil {
		statuses, err = c.statuses()
		if err != nil {
			http.Error(w, "collector status query failed", 503)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]any{"pricing": map[string]any{"source": pricing.CatalogURL, "updated_at": snapshot.UpdatedAt, "stale": snapshot.UpdatedAt.IsZero() || time.Since(snapshot.UpdatedAt) > 2*time.Hour, "error": snapshot.Error, "refresh_seconds": 3600, "basis": "current standard API rates", "fallback_model": "gpt-5.6-sol"}, "rows": rows, "sources": statuses, "days": days, "since": cutoff, "generated_at": now, "cursor_history_days": c.config.CursorHistoryDays})
}
