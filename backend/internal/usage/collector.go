package usage

import (
	"context"
	"fmt"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	"io/fs"
	"log"
	"os"
	"path/filepath"
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
type SourceStatus = events.SourceStatus
type Sink interface {
	ImportUsage([]store.ExternalUsage) error
}
type SummaryStore interface {
	PricedUsageSince(time.Time, func(string, int64, int64, int64, int64) (float64, bool)) ([]store.UsageRow, error)
}
type Collector struct {
	store      SummaryStore
	sink       Sink
	statusSink func(string, SourceStatus) error
	statuses   func() (map[string]SourceStatus, error)
	prices     pricing.Pricing
	catalog    *pricing.Catalog
	config     Config
	mu         sync.RWMutex
	status     map[string]SourceStatus
	files      map[string]string
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
	return &Collector{catalog: pricing.NewCatalog(c.PriceCache), store: s, sink: s, prices: p, config: c, status: status, files: map[string]string{}}
}

func NewWorker(sink Sink, statusSink func(string, SourceStatus) error, p pricing.Pricing, cfg Config) *Collector {
	c := New(nil, p, cfg)
	c.store = nil
	c.sink = sink
	c.statusSink = statusSink
	return c
}
func NewSummary(s SummaryStore, statuses func() (map[string]SourceStatus, error), cfg Config) *Collector {
	c := New(nil, nil, cfg)
	c.store = s
	c.statuses = statuses
	return c
}
func (c *Collector) RunPrices(ctx context.Context) { go c.catalog.Run(ctx) }

func (c *Collector) Run(ctx context.Context) {
	if c.store != nil {
		c.RunPrices(ctx)
	}
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
	if c.statusSink != nil {
		if err := c.statusSink(source, s); err != nil {
			log.Printf("status outbox failed: %v", err)
		}
	}
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
				if err = c.sink.ImportUsage(rows); err != nil {
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
			err = c.sink.ImportUsage(rows)
		}
	}
	c.finish("cursor", 0, err)
}
