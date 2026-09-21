package store

import (
	"testing"
	"time"
)

func TestDashboardCallsFilterBeforePaginationAndPreserveHistory(t *testing.T) {
	st := open(t)
	records := []Record{
		{Status: 200},
		{Status: 429},
		{Status: 429, Usage: Usage{Input: 1}},
		{Status: 429, Usage: Usage{Output: 1}},
		{Status: 429, Usage: Usage{CacheRead: 1}},
		{Status: 429, Usage: Usage{CacheWrite: 1}},
		{Status: 429, CostUSD: 0.01},
		{Status: 404},
		{Status: 429},
		{Status: 429},
	}
	for _, r := range records {
		r.TS = time.Now()
		r.Model = "test-model"
		if err := st.Insert(r); err != nil {
			t.Fatal(err)
		}
	}
	var ids []int64
	var before int64
	for {
		rows, err := st.RecentDashboardCalls(2, before)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		before = rows[len(rows)-1].ID
	}
	want := []int64{8, 7, 6, 5, 4, 3, 1}
	if len(ids) != len(want) {
		t.Fatalf("visible ids=%v want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("visible ids=%v want %v", ids, want)
		}
	}
	raw, err := st.CallsAfter(0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != len(records) {
		t.Fatalf("filter changed sync history: %d rows", len(raw))
	}
}
