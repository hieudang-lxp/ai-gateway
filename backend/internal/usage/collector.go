package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

type Config struct {
	CodexHome, ClaudeProjects, CursorState string
	LocalInterval, CursorInterval          time.Duration
	CursorHistoryDays                      int
	PriceCache                             string
}
type SourceStatus struct {
	State       string     `json:"state"`
	LastAttempt time.Time  `json:"last_attempt"`
	LastSuccess *time.Time `json:"last_success"`
	Error       string     `json:"error,omitempty"`
	Files       int        `json:"files,omitempty"`
	PollSeconds int        `json:"poll_seconds"`
}
type Collector struct {
	store   *store.Store
	prices  pricing.Pricing
	catalog *pricing.Catalog
	config  Config
	mu      sync.RWMutex
	status  map[string]SourceStatus
	files   map[string]string
}

func New(s *store.Store, p pricing.Pricing, c Config) *Collector {
	if c.LocalInterval <= 0 {
		c.LocalInterval = time.Minute
	}
	if c.CursorInterval <= 0 {
		c.CursorInterval = 5 * time.Minute
	}
	if c.CursorHistoryDays <= 0 {
		c.CursorHistoryDays = 365
	}
	status := map[string]SourceStatus{}
	for _, source := range []string{"codex", "claude_code", "cursor"} {
		interval := c.LocalInterval
		if source == "cursor" {
			interval = c.CursorInterval
		}
		status[source] = SourceStatus{State: "starting", PollSeconds: int(interval.Seconds())}
	}
	return &Collector{catalog: pricing.NewCatalog(c.PriceCache), store: s, prices: p, config: c, status: status, files: map[string]string{}}
}

func (c *Collector) Run(ctx context.Context) {
	go c.catalog.Run(ctx)
	go func() {
		c.CollectLocal()
		ticker := time.NewTicker(c.config.LocalInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.CollectLocal()
			}
		}
	}()
	go func() {
		c.CollectCursor(ctx)
		ticker := time.NewTicker(c.config.CursorInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.CollectCursor(ctx)
			}
		}
	}()
}

func (c *Collector) finish(source string, files int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.status[source]
	s.LastAttempt = time.Now()
	s.Files = files
	if err != nil {
		s.State = "error"
		s.Error = err.Error()
		log.Printf("usage collector %s: %s", source, s.Error)
	} else {
		s.State = "ok"
		s.Error = ""
		now := time.Now()
		s.LastSuccess = &now
	}
	c.status[source] = s
}

func (c *Collector) CollectLocal() {
	for _, source := range []string{"codex", "claude_code"} {
		roots := []string{c.config.ClaudeProjects}
		if source == "codex" {
			roots = []string{filepath.Join(c.config.CodexHome, "sessions"), filepath.Join(c.config.CodexHome, "archived_sessions")}
		}
		count := 0
		var firstErr error
		foundRoot := false
		for _, root := range roots {
			if _, err := os.Stat(root); os.IsNotExist(err) {
				continue
			} else if err != nil {
				firstErr = err
				continue
			}
			foundRoot = true
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
					return nil
				}
				count++
				info, err := d.Info()
				if err != nil {
					return err
				}
				signature := fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
				if c.files[path] == signature {
					return nil
				}
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				var rows []store.ExternalUsage
				if source == "codex" {
					rows, err = ParseCodex(f)
				} else {
					rows, err = ParseClaude(f, c.prices)
				}
				f.Close()
				if err != nil {
					if firstErr == nil {
						firstErr = fmt.Errorf("%s: %w", filepath.Base(path), err)
					}
					return nil
				}
				if err = c.store.ImportUsage(rows); err != nil {
					return err
				}
				c.files[path] = signature
				return nil
			})
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if !foundRoot {
			firstErr = fmt.Errorf("source directory unavailable; check read-only volume mounts")
		}
		c.finish(source, count, firstErr)
	}
}

func (c *Collector) CollectCursor(ctx context.Context) {
	creds, err := ReadCursorCredentials(c.config.CursorState)
	if err == nil {
		var rows []store.ExternalUsage
		now := time.Now()
		rows, err = NewCursorClient().Fetch(ctx, creds, now.AddDate(0, 0, -c.config.CursorHistoryDays), now)
		if err == nil {
			err = c.store.ImportUsage(rows)
		}
	}
	c.finish("cursor", 0, err)
}

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
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]any{"pricing": map[string]any{"source": pricing.CatalogURL, "updated_at": snapshot.UpdatedAt, "stale": snapshot.UpdatedAt.IsZero() || time.Since(snapshot.UpdatedAt) > 2*time.Hour, "error": snapshot.Error, "refresh_seconds": 3600, "basis": "current standard API rates", "fallback_model": "gpt-5.6-sol"}, "rows": rows, "sources": statuses, "days": days, "since": cutoff, "generated_at": now, "cursor_history_days": c.config.CursorHistoryDays})
}
