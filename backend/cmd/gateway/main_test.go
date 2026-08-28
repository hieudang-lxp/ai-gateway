package main

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

// TestAPIOverLocalDB exercises the api-role wiring (store -> snapshot limits
// -> Connect handler) against a local db, mirroring `gateway api -db`.
func TestAPIOverLocalDB(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.EnsureSyncSchema(); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteBudgetSnapshot(10, 20, 0, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := st.Insert(store.Record{TS: time.Now(), Model: "m", CostUSD: 1.5, Status: 200}); err != nil {
		t.Fatal(err)
	}

	limits := func() control.BudgetConfig {
		dw, dh, ww, wh, mw, mh, ok, err := st.ReadBudgetSnapshot()
		if err != nil || !ok {
			return control.BudgetConfig{}
		}
		return control.BudgetConfig{Daily: control.Limit{Warn: dw, Hard: dh},
			Weekly: control.Limit{Warn: ww, Hard: wh}, Monthly: control.Limit{Warn: mw, Hard: mh}}
	}
	srv := httptest.NewServer(api.New(st, limits, ""))
	defer srv.Close()

	c := gatewayv1connect.NewStatsServiceClient(srv.Client(), srv.URL)
	resp, err := c.Overview(context.Background(), connect.NewRequest(&gatewayv1.OverviewRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.Today.SpentUsd != 1.5 || resp.Msg.Today.WarnUsd != 10 {
		t.Fatalf("overview: %+v", resp.Msg.Today)
	}
}
