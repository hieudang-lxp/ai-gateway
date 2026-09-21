package api_test

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	gatewayv1 "github.com/hieudang-lxp/ai-gateway/backend/gen/gateway/v1"
	"github.com/hieudang-lxp/ai-gateway/backend/gen/gateway/v1/gatewayv1connect"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/api"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

func seededStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	now := time.Now()
	rows := []store.Record{
		{TS: now, Model: "claude-sonnet-5", Usage: store.Usage{Input: 100, Output: 50}, CostUSD: 1.0, LatencyMS: 800, Status: 200},

		{TS: now.Add(-time.Minute), Model: "claude-opus-4-8", Usage: store.Usage{Input: 10, Output: 5}, CostUSD: 2.0, LatencyMS: 900, Status: 200, RoutedFrom: "claude-fable-5"},
		{TS: now.Add(-25 * time.Hour), Model: "claude-sonnet-5", CostUSD: 4.0, Status: 200},
		{TS: now, Model: "claude-sonnet-5", CostUSD: 0, Status: 200, CacheHit: true, SavedUSD: 0.5},
	}
	for _, r := range rows {
		if err := st.Insert(r); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

func client(t *testing.T, st *store.Store, token string) gatewayv1connect.StatsServiceClient {
	t.Helper()
	limits := func() control.BudgetConfig {
		return control.BudgetConfig{Daily: control.Limit{Warn: 10, Hard: 20}}
	}
	srv := httptest.NewServer(api.New(st, limits, token))
	t.Cleanup(srv.Close)
	opts := []connect.ClientOption{}
	return gatewayv1connect.NewStatsServiceClient(srv.Client(), srv.URL, opts...)
}

func TestOverview(t *testing.T) {
	c := client(t, seededStore(t), "")
	resp, err := c.Overview(context.Background(), connect.NewRequest(&gatewayv1.OverviewRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	m := resp.Msg
	if m.Today.SpentUsd != 3.0 {
		t.Fatalf("today spent = %v want 3.0", m.Today.SpentUsd)
	}
	if m.Today.WarnUsd != 10 || m.Today.HardUsd != 20 {
		t.Fatalf("limits not surfaced: %+v", m.Today)
	}
	if m.CacheHits != 1 || m.CacheSavedUsd != 0.5 {
		t.Fatalf("cache: hits=%d saved=%v", m.CacheHits, m.CacheSavedUsd)
	}
	if m.TotalCalls != 4 {
		t.Fatalf("total = %d", m.TotalCalls)
	}
}

func TestSpendSeriesAndBreakdown(t *testing.T) {
	c := client(t, seededStore(t), "")
	s, err := c.SpendSeries(context.Background(), connect.NewRequest(&gatewayv1.SpendSeriesRequest{Days: 7}))
	if err != nil {
		t.Fatal(err)
	}
	var total float64
	for _, p := range s.Msg.Points {
		total += p.CostUsd
		if len(p.Date) != 10 {
			t.Fatalf("bad date %q", p.Date)
		}
	}
	if total != 7.0 {
		t.Fatalf("series total = %v want 7.0", total)
	}
	b, err := c.ModelBreakdown(context.Background(), connect.NewRequest(&gatewayv1.ModelBreakdownRequest{Days: 7}))
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Msg.Rows) != 2 {
		t.Fatalf("rows = %d", len(b.Msg.Rows))
	}
}

func TestRecentCallsPagination(t *testing.T) {
	c := client(t, seededStore(t), "")
	r1, err := c.RecentCalls(context.Background(), connect.NewRequest(&gatewayv1.RecentCallsRequest{Limit: 2}))
	if err != nil {
		t.Fatal(err)
	}
	if len(r1.Msg.Calls) != 2 || r1.Msg.Calls[0].Id <= r1.Msg.Calls[1].Id {
		t.Fatalf("want 2 calls desc, got %+v", r1.Msg.Calls)
	}
	r2, err := c.RecentCalls(context.Background(),
		connect.NewRequest(&gatewayv1.RecentCallsRequest{Limit: 10, BeforeId: r1.Msg.Calls[1].Id}))
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.Msg.Calls) != 2 {
		t.Fatalf("page 2 = %d calls want 2", len(r2.Msg.Calls))
	}
}

func TestAuthRequired(t *testing.T) {
	st := seededStore(t)
	limits := func() control.BudgetConfig { return control.BudgetConfig{} }
	srv := httptest.NewServer(api.New(st, limits, "s3cret"))
	defer srv.Close()

	c := gatewayv1connect.NewStatsServiceClient(srv.Client(), srv.URL)
	_, err := c.Overview(context.Background(), connect.NewRequest(&gatewayv1.OverviewRequest{}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}

	req := connect.NewRequest(&gatewayv1.OverviewRequest{})
	req.Header().Set("Authorization", "Bearer s3cret")
	if _, err := c.Overview(context.Background(), req); err != nil {
		t.Fatalf("with token: %v", err)
	}
}
