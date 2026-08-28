# AI Gateway + Dashboard — Design

Date: 2026-08-28
Status: approved-pending-review
Repo: github.com/hieudang-lxp/lxp-scan-svc → will be renamed **ai-gateway**

## Goal

Repurpose this repo into Hieu's personal **AI gateway** monorepo: a local Go proxy in
front of the Anthropic API that all his Claude Code traffic routes through
(`ANTHROPIC_BASE_URL`), with control features (budget limits, model routing, response
cache), usage/cost logging, cloud sync, and a deployed web dashboard.

The existing **personal-ai-gateway MVP** (`~/Leapxpert/tools/personal-ai-gateway`,
~720 LOC Go: reverse proxy + SSE tee, usage/cost parse, pricing.json, SQLite store,
stats CLI) is **merged into this repo** and extended — the proxy/SSE core is done and
must not be rewritten.

The existing gRPC scan service in this repo is **removed** (tagged `scan-svc-final`
first; recoverable from git history).

## Architecture

```
Claude Code ──ANTHROPIC_BASE_URL──▶ gateway serve (Go, localhost:8080) ──▶ Anthropic API
                                        │  HTTP proxy (impersonates Anthropic API)
                                        │  • usage/cost log → local SQLite (exists)
                                        │  • budget check / model routing / cache (new)
                                        ▼  background batch sync, fail-open
                                   Turso (cloud SQLite/libSQL, free tier)
                                        ▲ read-only, server-side token
                                gateway api (same Go binary, deployed on Render free)
                                        ▲ Connect RPC (gRPC-Web/HTTP-JSON compatible)
                                Dashboard (React + Vite, deployed on Netlify)
```

- The **proxy** stays plain HTTP — required, it impersonates the Anthropic API.
- The **dashboard API** is proto-first via **Connect RPC** (connectrpc.com): one Go
  handler serves gRPC + gRPC-Web + Connect JSON; the FE uses TypeScript clients
  generated from the same protos (buf + @connectrpc/connect-web). No grpc-web proxy
  needed.
- One Go binary, two roles:
  - `gateway serve` — local: proxy + control + local SQLite + Turso sync (+ serves
    the Connect API locally too, so dashboard dev works without cloud).
  - `gateway api` — cloud (Render): Connect API only, reads Turso.

## Monorepo layout

```
ai-gateway/
├── proto/gateway/v1/         # buf-managed: StatsService, ConfigService
├── backend/                  # Go module
│   ├── cmd/gateway/          # main: serve | api | stats (CLI)
│   ├── internal/proxy/       # reverse proxy + SSE tee (moved from MVP, as-is)
│   ├── internal/pricing/     # pricing.json cost calc (moved from MVP)
│   ├── internal/store/       # SQLite store + sync watermark (moved, extended)
│   ├── internal/control/     # budget, routing, cache (new)
│   ├── internal/sync/        # batch push → Turso (new)
│   ├── internal/api/         # Connect RPC handlers (new)
│   └── gen/                  # buf-generated Go
├── frontend/                 # React + Vite + TS + Tailwind
│   └── src/gen/              # buf-generated TS Connect clients
├── netlify.toml              # deploys frontend/
├── render.yaml + Dockerfile  # deploys `gateway api`
└── gateway.yaml.example      # control config
```

## Control features (`gateway.yaml`, hot-reload on change)

**Budget** — USD thresholds per day/week/month (calendar periods in Asia/Ho_Chi_Minh:
midnight-to-midnight day, Mon-start week, calendar month), each with a `warn` and a
`hard` level.
Spend is computed from local SQLite (works offline). Over `warn`: log + flag exposed to
dashboard. Over `hard`: reject the request with an Anthropic-shaped JSON error (so
Claude Code renders it cleanly) and HTTP 429.

**Model routing** — ordered rules rewriting the request body `model` field before
forwarding (e.g. `claude-opus-* → claude-sonnet-5`), plus a block-list (blocked model →
Anthropic-shaped error). Cost is computed from the model **actually returned** in the
response, not the requested one.

**Cache** — exact-match: hash(model + messages + system + tools + params) → stored
response, configurable TTL, off by default per config. Streaming responses are stored
and replayed as SSE on hit. Hits record `saved_usd` for the dashboard. Requests with
`stream` and identical bodies hit the same key.

All control features are **fail-open**: any internal error → forward the request
untouched.

## Data & sync

- Local SQLite `calls` table (exists) gains: `routed_from` (original model when
  rewritten), `cache_hit` (bool), `saved_usd`. No request/response bodies are synced.
- `internal/sync`: goroutine every ~60s pushes rows with `id > last_synced_id` to Turso
  over its HTTP API (batch insert, then advance watermark). Turso unreachable → skip,
  retry next tick; the gateway never blocks on sync.
- Turso schema mirrors local `calls` (+ a `budget_status` snapshot table so the cloud
  API can show budget state without recomputing config).

## Connect API (proto sketch)

```proto
service StatsService {
  rpc Overview(OverviewRequest) returns (OverviewResponse);      // spend today/week/month, budget status, cache savings
  rpc SpendSeries(SpendSeriesRequest) returns (SpendSeriesResponse); // time-bucketed spend for charts
  rpc ModelBreakdown(ModelBreakdownRequest) returns (ModelBreakdownResponse);
  rpc RecentCalls(RecentCallsRequest) returns (RecentCallsResponse); // paginated
}
```

Auth: a single shared bearer token (env `DASHBOARD_TOKEN` on Render; entered once in
the FE, kept in localStorage) via a Connect interceptor. Personal-grade, not multi-user.

## Dashboard (frontend/)

Views: spend today/week/month vs budget (progress bars), spend-over-time chart,
per-model breakdown, cache hit rate + money saved, recent calls table.

**USD → VND**: a global currency toggle (USD/VND) applied to every money display,
formatted with `Intl.NumberFormat('vi-VN')`. Rate comes from `useExchangeRate`:
fetch `https://open.er-api.com/v6/latest/USD` (free, no key, CORS-enabled, has VND),
refresh hourly, last-good rate cached in localStorage as fallback. The active rate and
its fetch time are shown next to the toggle. **The DB stores USD only** — VND is a
display layer.

## Deploy

- **Netlify**: builds `frontend/` (static). Env: API base URL.
- **Render (free)**: Docker deploy of `gateway api`. Env: `TURSO_URL`, `TURSO_TOKEN`,
  `DASHBOARD_TOKEN`. Known trade-off: free tier sleeps after 15 min idle → ~30 s cold
  start on first dashboard load.
- **Turso (free)**: single database; gateway pushes with a write token, Render API
  reads with the same or a read token.

## Error handling principles

- Proxy path is sacred: control/sync/logging failures must never break a Claude Code
  request (fail-open, same as MVP tee design).
- Budget `hard` block and model block-list are the only intentional request rejections,
  and both return Anthropic-shaped errors.

## Testing

- Go: unit tests for budget math, routing rules, cache keying/TTL; integration tests
  against a mock upstream (reuse MVP's verified mock-upstream approach) covering SSE
  passthrough, rewrite-then-forward, hard-block, cache replay.
- Sync: test against a local libSQL/sqlite file standing in for Turso; then one manual
  verify against real Turso.
- FE: vitest for currency conversion/formatting + data transforms; manual e2e with the
  local gateway.
- Final e2e: real Claude Code call → local gateway → row in Turso → visible on deployed
  Netlify dashboard, in both USD and VND.

## Implementation order

1. **Repo restructure** — tag `scan-svc-final`, remove scan-svc code, move MVP gateway
   code into `backend/` layout, rename repo to `ai-gateway`; smoke-test proxy parity.
2. **Control features** — budget → routing → cache, each with tests.
3. **Proto + Connect API** — buf setup, StatsService served by `gateway serve` locally.
4. **Turso sync** + `gateway api` role.
5. **Frontend** — dashboard views + USD/VND toggle, against local Connect API.
6. **Deploy** — Render (api), Netlify (frontend), end-to-end verify.

## Out of scope

- Multi-user/team anything (this is a personal tool; a team gateway was explicitly
  rejected earlier).
- Request/response body storage or replay (P4 idea from the old MVP — deferred).
- lxp-scan integration (explicitly deferred long ago).
- Intraday-precise VND rates — the free API updates daily; good enough for display.
