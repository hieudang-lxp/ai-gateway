package store

import (
	"path/filepath"
	"testing"
	"time"
)

func open(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestSpendSince(t *testing.T) {
	st := open(t)
	base := time.Now()
	for i, cost := range []float64{1.5, 2.5, 4.0} {
		r := Record{TS: base.Add(time.Duration(-i) * time.Hour), Model: "m", CostUSD: cost, Status: 200}
		if err := st.Insert(r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := st.SpendSince(base.Add(-90 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if got != 4.0 {
		t.Fatalf("spend = %v want 4.0", got)
	}
	all, _ := st.SpendSince(time.Unix(0, 0))
	if all != 8.0 {
		t.Fatalf("all = %v want 8.0", all)
	}
}
