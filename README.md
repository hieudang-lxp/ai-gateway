# ai-gateway

Personal AI gateway monorepo: a local Go proxy in front of the Anthropic API
(point `ANTHROPIC_BASE_URL` at it) with usage/cost logging, budget limits,
model routing and response caching — plus a Connect RPC stats API and a React
dashboard (deployed on Netlify; API on Render; data synced to Turso).

- `backend/` — Go: `gateway serve` (local proxy) / `gateway api` (cloud stats API) / `gateway stats` (CLI)
- `proto/` — buf-managed Connect RPC schema
- `frontend/` — React + Vite dashboard (Plan 2)

Design: `docs/superpowers/specs/2026-08-28-ai-gateway-design.md`.
Previous life of this repo (gRPC scan service): tag `scan-svc-final`.
