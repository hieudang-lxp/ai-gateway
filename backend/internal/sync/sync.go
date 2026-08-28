// Package sync pushes local call rows to the Turso replica. Strictly
// fail-open: any error is logged and retried next tick; the proxy never
// depends on it.
package sync

import (
	"context"
	"log"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

type Syncer struct {
	Local    *store.Store
	Remote   *store.Store
	Limits   func() control.BudgetConfig
	Interval time.Duration // default 60s
	Batch    int           // default 500
}

// SyncOnce pushes at most Batch rows past the watermark, then refreshes the
// budget snapshot. Returns how many rows were pushed.
func (s *Syncer) SyncOnce() (int, error) {
	batch := s.Batch
	if batch <= 0 {
		batch = 500
	}
	last, err := s.Local.LastSyncedID()
	if err != nil {
		return 0, err
	}
	calls, err := s.Local.CallsAfter(last, batch)
	if err != nil {
		return 0, err
	}
	for _, c := range calls {
		if err := s.Remote.InsertSynced(c); err != nil {
			return 0, err
		}
		last = c.ID
	}
	if len(calls) > 0 {
		if err := s.Local.SetLastSyncedID(last); err != nil {
			return 0, err
		}
	}
	cfg := s.Limits()
	if err := s.Remote.WriteBudgetSnapshot(
		cfg.Daily.Warn, cfg.Daily.Hard,
		cfg.Weekly.Warn, cfg.Weekly.Hard,
		cfg.Monthly.Warn, cfg.Monthly.Hard); err != nil {
		return len(calls), err
	}
	return len(calls), nil
}

// Run loops until ctx is done. Errors are logged, never fatal.
func (s *Syncer) Run(ctx context.Context) {
	interval := s.Interval
	if interval <= 0 {
		interval = 60 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := s.SyncOnce(); err != nil {
				log.Printf("sync: %v", err)
			} else if n > 0 {
				log.Printf("sync: pushed %d rows", n)
			}
		}
	}
}
