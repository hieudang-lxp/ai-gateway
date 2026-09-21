package explore

import (
	"context"
	"testing"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
)

func TestDailyUsageUsesLocalDatesAndReconcilesTotals(t *testing.T) {
	s := testProjection(t, false)
	zone := time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	since := time.Date(2026, 9, 1, 0, 0, 0, 0, zone)
	until := since.Add(48 * time.Hour)
	applyRows(t, s,
		events.ExternalUsage{ID: "a", Source: "codex", Model: "tier", TS: since.Add(time.Minute), Usage: events.Usage{Input: 10}},
		events.ExternalUsage{ID: "b", Source: "codex", Model: "tier", TS: since.Add(24 * time.Hour), Usage: events.Usage{Input: 20}},
		events.ExternalUsage{ID: "c", Source: "cursor", Model: "missing", TS: since.Add(time.Hour), Usage: events.Usage{Output: 5}},
		events.ExternalUsage{ID: "old", Source: "codex", Model: "tier", TS: since.Add(-time.Second), Usage: events.Usage{Input: 100}},
	)
	got, err := s.Insights(context.Background(), since, until, testPrices())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Daily) != 3 || got.Daily[0].Date != "2026-09-01" || got.Daily[2].Date != "2026-09-02" || got.TimeZone != "Asia/Ho_Chi_Minh" {
		t.Fatalf("bad local buckets: %+v", got.Daily)
	}
	var sum Totals
	for _, day := range got.Daily {
		sum.Calls += day.Calls
		sum.Tokens += day.Tokens
		sum.KnownCost += day.KnownCost
		sum.Unknown += day.Unknown
	}
	if sum != got.Totals {
		t.Fatalf("daily totals=%+v overall=%+v", sum, got.Totals)
	}
	var networkSum Totals
	for _, node := range got.Network {
		networkSum.Calls += node.Calls
		networkSum.Tokens += node.Tokens
		networkSum.KnownCost += node.KnownCost
		networkSum.Unknown += node.Unknown
	}
	if networkSum != got.Totals || len(got.Network) != 2 {
		t.Fatalf("network totals=%+v overall=%+v nodes=%+v", networkSum, got.Totals, got.Network)
	}
	all, err := s.Insights(context.Background(), time.Unix(0, 0), until, testPrices())
	if err != nil || all.DateFrom != "2026-08-31" {
		t.Fatalf("all-history start=%s err=%v", all.DateFrom, err)
	}
}
