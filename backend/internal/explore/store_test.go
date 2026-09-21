package explore

import (
	"database/sql"
	"testing"
)

func TestMetadataRevisionAndEqualUsageRepricing(t *testing.T) {
	original := record{in: 100, session: "session", title: "Current", project: "/project", revision: 20, cost: sql.NullFloat64{Float64: 1, Valid: true}}
	newer := record{in: 100, title: "Renamed", revision: 30, cost: sql.NullFloat64{Float64: 2, Valid: true}}
	got := merge(original, newer)
	if got.title != "Renamed" || got.session != "session" || got.project != "/project" || got.cost.Float64 != 2 {
		t.Fatalf("new revision did not enrich existing snapshot: %+v", got)
	}
	stale := record{in: 100, title: "Stale", session: "wrong", project: "/old", revision: 10, cost: sql.NullFloat64{Float64: .5, Valid: true}}
	if replay := merge(got, stale); replay != got {
		t.Fatalf("old revision regressed metadata or cost: %+v", replay)
	}
	metadata := record{in: 1, title: "Newest", revision: 40}
	if enriched := merge(got, metadata); enriched.in != 100 || enriched.cost.Float64 != 2 || enriched.title != "Newest" {
		t.Fatalf("metadata-only enrichment altered usage: %+v", enriched)
	}
}

func TestNewerEqualTotalSnapshotCorrectsBreakdownWithoutSession(t *testing.T) {
	original := record{model: "old-model", in: 100, revision: 10, cost: sql.NullFloat64{Float64: 1, Valid: true}}
	corrected := record{model: "correct-model", read: 100, revision: 20, cost: sql.NullFloat64{Float64: .25, Valid: true}}
	got := merge(original, corrected)
	if got.model != "correct-model" || got.in != 0 || got.read != 100 || got.cost.Float64 != .25 || got.revision != 20 || got.session != "" {
		t.Fatalf("newer snapshot did not correct unassigned usage: %+v", got)
	}
	if replay := merge(got, original); replay != got {
		t.Fatalf("stale snapshot reverted correction: %+v", replay)
	}
	if sameRevision := merge(got, record{in: 100, model: "conflict", revision: 20}); sameRevision != got {
		t.Fatalf("equal revision changed existing breakdown: %+v", sameRevision)
	}
}

func TestSessionChoosesLatestNonemptyMetadataInAnyRowOrder(t *testing.T) {
	rows := []record{
		{title: "Old title", project: "/old-project", revision: 10, ts: 1},
		{title: "Latest title", revision: 20, ts: 2},
		{project: "/latest-project", revision: 30, ts: 3},
		{revision: 40, ts: 4},
	}
	for _, order := range [][]int{{0, 1, 2, 3}, {3, 0, 1, 2}, {2, 1, 0, 3}, {3, 2, 1, 0}} {
		var session Session
		for _, i := range order {
			session.add(rows[i], testPrices())
		}
		if session.Title != "Latest title" || session.Project != "/latest-project" {
			t.Fatalf("order %v selected title=%q project=%q", order, session.Title, session.Project)
		}
	}
}
