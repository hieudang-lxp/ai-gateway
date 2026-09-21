package sync_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
	gwsync "github.com/hieudang-lxp/ai-gateway/backend/internal/sync"
)

func pair(t *testing.T) (*store.Store, *store.Store) {
	t.Helper()
	local, err := store.Open(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatal(err)
	}
	remote, err := store.Open(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { local.Close(); remote.Close() })
	if err := remote.EnsureSyncSchema(); err != nil {
		t.Fatal(err)
	}
	return local, remote
}

func seed(t *testing.T, st *store.Store, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := st.Insert(store.Record{TS: time.Now(), Model: "m", CostUSD: 1, Status: 200}); err != nil {
			t.Fatal(err)
		}
	}
}

func limits() control.BudgetConfig {
	return control.BudgetConfig{Daily: control.Limit{Warn: 10, Hard: 20}}
}

func TestSyncOncePushesAndAdvancesWatermark(t *testing.T) {
	local, remote := pair(t)
	seed(t, local, 3)
	s := &gwsync.Syncer{Local: local, Remote: remote, Limits: limits, Batch: 500}

	pushed, err := s.SyncOnce()
	if err != nil || pushed != 3 {
		t.Fatalf("pushed=%d err=%v", pushed, err)
	}
	if n, _ := remote.TotalCalls(); n != 3 {
		t.Fatalf("remote rows = %d", n)
	}
	if id, _ := local.LastSyncedID(); id != 3 {
		t.Fatalf("watermark = %d", id)
	}

	dw, dh, _, _, _, _, ok, err := remote.ReadBudgetSnapshot()
	if err != nil || !ok || dw != 10 || dh != 20 {
		t.Fatalf("snapshot: %v %v %v %v", dw, dh, ok, err)
	}
}

func TestSyncOnceIsIdempotent(t *testing.T) {
	local, remote := pair(t)
	seed(t, local, 2)
	s := &gwsync.Syncer{Local: local, Remote: remote, Limits: limits, Batch: 500}
	if _, err := s.SyncOnce(); err != nil {
		t.Fatal(err)
	}

	if err := local.SetLastSyncedID(0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncOnce(); err != nil {
		t.Fatal(err)
	}
	if n, _ := remote.TotalCalls(); n != 2 {
		t.Fatalf("remote rows = %d want 2 (deduped)", n)
	}
}

func TestSyncOnlyNewRows(t *testing.T) {
	local, remote := pair(t)
	seed(t, local, 2)
	s := &gwsync.Syncer{Local: local, Remote: remote, Limits: limits, Batch: 500}
	_, _ = s.SyncOnce()
	seed(t, local, 1)
	pushed, err := s.SyncOnce()
	if err != nil || pushed != 1 {
		t.Fatalf("pushed=%d err=%v", pushed, err)
	}
	if n, _ := remote.TotalCalls(); n != 3 {
		t.Fatalf("remote rows = %d", n)
	}
}
