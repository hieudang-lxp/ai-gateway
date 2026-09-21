// Package api serves the dashboard's Connect RPC StatsService over a Store —
// the local one in `gateway serve`, the Turso replica in `gateway api`.
package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"connectrpc.com/connect"

	gatewayv1 "github.com/hieudang-lxp/ai-gateway/backend/gen/gateway/v1"
	"github.com/hieudang-lxp/ai-gateway/backend/gen/gateway/v1/gatewayv1connect"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

type server struct {
	st     *store.Store
	limits func() control.BudgetConfig
}

// New returns an http.Handler serving StatsService. token == "" disables auth
// (local use); otherwise requests need "Authorization: Bearer <token>".
func New(st *store.Store, limits func() control.BudgetConfig, token string) http.Handler {
	s := &server{st: st, limits: limits}
	mux := http.NewServeMux()
	path, h := gatewayv1connect.NewStatsServiceHandler(s,
		connect.WithInterceptors(authInterceptor(token)))
	mux.Handle(path, h)
	return mux
}

func authInterceptor(token string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if token != "" && req.Header().Get("Authorization") != "Bearer "+token {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing or bad token"))
			}
			return next(ctx, req)
		}
	}
}

func (s *server) Overview(ctx context.Context, _ *connect.Request[gatewayv1.OverviewRequest]) (*connect.Response[gatewayv1.OverviewResponse], error) {
	dayS, weekS, monthS := control.PeriodStarts(time.Now())
	window := func(cutoff time.Time, lim control.Limit) (*gatewayv1.BudgetWindow, error) {
		spent, err := s.st.SpendSince(cutoff)
		if err != nil {
			return nil, err
		}
		return &gatewayv1.BudgetWindow{SpentUsd: spent, WarnUsd: lim.Warn, HardUsd: lim.Hard}, nil
	}
	cfg := s.limits()
	today, err := window(dayS, cfg.Daily)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	week, err := window(weekS, cfg.Weekly)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	month, err := window(monthS, cfg.Monthly)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	hits, saved, err := s.st.CacheSavings()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	total, err := s.st.TotalCalls()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&gatewayv1.OverviewResponse{
		Today: today, Week: week, Month: month,
		CacheSavedUsd: saved, CacheHits: hits, TotalCalls: total,
	}), nil
}

func (s *server) SpendSeries(ctx context.Context, req *connect.Request[gatewayv1.SpendSeriesRequest]) (*connect.Response[gatewayv1.SpendSeriesResponse], error) {
	days := int(req.Msg.Days)
	if days <= 0 {
		days = 30
	}
	dayS, _, _ := control.PeriodStarts(time.Now())
	cutoff := dayS.AddDate(0, 0, -(days - 1))
	events, err := s.st.CostEvents(cutoff)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	loc := dayS.Location()
	byDate := map[string]*gatewayv1.SpendPoint{}
	var order []string
	for i := 0; i < days; i++ {
		d := cutoff.AddDate(0, 0, i).Format("2006-01-02")
		byDate[d] = &gatewayv1.SpendPoint{Date: d}
		order = append(order, d)
	}
	for _, e := range events {
		d := time.Unix(e.TS, 0).In(loc).Format("2006-01-02")
		if p, ok := byDate[d]; ok {
			p.CostUsd += e.CostUSD
			p.Calls++
		}
	}
	resp := &gatewayv1.SpendSeriesResponse{}
	for _, d := range order {
		resp.Points = append(resp.Points, byDate[d])
	}
	return connect.NewResponse(resp), nil
}

func (s *server) ModelBreakdown(ctx context.Context, req *connect.Request[gatewayv1.ModelBreakdownRequest]) (*connect.Response[gatewayv1.ModelBreakdownResponse], error) {
	days := int(req.Msg.Days)
	if days <= 0 {
		days = 30
	}
	rows, err := s.st.StatsSince(time.Now().AddDate(0, 0, -days))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &gatewayv1.ModelBreakdownResponse{}
	for _, r := range rows {
		resp.Rows = append(resp.Rows, &gatewayv1.ModelRow{
			Model: r.Model, Calls: r.Calls,
			InputTokens: r.Input, OutputTokens: r.Output,
			CacheReadTokens: r.CacheRead, CacheWriteTokens: r.CacheWrite,
			CostUsd: r.CostUSD,
		})
	}
	return connect.NewResponse(resp), nil
}

func (s *server) RecentCalls(ctx context.Context, req *connect.Request[gatewayv1.RecentCallsRequest]) (*connect.Response[gatewayv1.RecentCallsResponse], error) {
	limit := int(req.Msg.Limit)
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	calls, err := s.st.RecentDashboardCalls(limit, req.Msg.BeforeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &gatewayv1.RecentCallsResponse{}
	for _, c := range calls {
		resp.Calls = append(resp.Calls, &gatewayv1.Call{
			Id: c.ID, TsUnix: c.TS.Unix(), Model: c.Model, RoutedFrom: c.RoutedFrom,
			InputTokens: c.Usage.Input, OutputTokens: c.Usage.Output,
			CacheReadTokens: c.Usage.CacheRead, CacheWriteTokens: c.Usage.CacheWrite,
			CostUsd: c.CostUSD, LatencyMs: c.LatencyMS, Status: int32(c.Status),
			CacheHit: c.CacheHit, SavedUsd: c.SavedUSD,
			RequestId: c.RequestID, RequestModel: c.RequestModel, RequestPath: c.RequestPath, ModelSource: c.ModelSource, UpstreamRequestId: c.UpstreamRequestID,
		})
	}
	return connect.NewResponse(resp), nil
}
