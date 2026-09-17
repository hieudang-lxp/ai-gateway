# ai-gateway

Local Docker dashboard that automatically collects Claude Code, Codex, and
Cursor usage across IDEs on your machine, plus a Go proxy in front of the Anthropic API
(point `ANTHROPIC_BASE_URL` at it) with usage/cost logging, budget limits,
model routing and response caching — plus a Connect RPC stats API and a React
dashboard. The separate cloud deployment supports Netlify, Render, and Turso
for proxy statistics; the three-tool collector runs locally.

- `backend/` — Go binaries: `gateway` (server) and `aictl` (usage/diagnostics CLI)
- `proto/` — buf-managed Connect RPC schema
- `frontend/` — React + Vite dashboard

The setup and maintenance guide below describes the current implementation.
Historical design plans and the former scan service remain available in Git history.

## Local Docker service: all three tools

### First-time setup (macOS)

Run these commands from the cloned repository root. Docker Desktop with Compose
must be running; Go and Node are only needed for development outside Docker.

1. Use Claude Code and Codex at least once so they create local session logs.
2. Sign in to the Cursor desktop app with the account you want to collect.
3. Prepare writable data/config folders and the optional Codex archive folder:

```sh
mkdir -p "$HOME/.local/share/ai-gateway" "$HOME/.config/ai-gateway" "$HOME/.codex/archived_sessions"
test -d "$HOME/.claude/projects"
test -d "$HOME/.codex/sessions"
test -f "$HOME/Library/Application Support/Cursor/User/globalStorage/state.vscdb"
```

Each `test` should exit successfully. If a source is elsewhere, configure its
path below. Do not create an empty Cursor database to bypass this check.
No `pricing.json` or `gateway.yaml` is required for collection: built-in
defaults apply when these optional files are absent.

4. Build and start the service:

```sh
docker compose up -d --build
```

Open **http://localhost:8788/dashboard/**. The container runs continuously with
`restart: unless-stopped`. Docker must be running and the host must be awake.
Closing the dashboard does not stop collection. Initial imports run at startup;
large local histories can take longer than a normal poll.

### What is collected

- **Claude Code**: reads retained `~/.claude/projects` transcripts every minute,
  including CLI, IDE extensions and subagents. Deduplicates message snapshots.
- **Codex**: reads `~/.codex/sessions` and `archived_sessions` every minute,
  deduplicates response IDs, and supports legacy cumulative token records.
- **Cursor**: uses the existing signed-in desktop session, reread from the local
  state database every five minutes. Calls Cursor's own `DashboardService/GetMe`
  and paginated `GetFilteredUsageEvents` on `api2.cursor.sh`, scoped to your user.
  **No recurring browser login or CSV export.** Normal session expiration still
  requires signing back into the Cursor app. Its internal RPC can change; errors
  appear on the dashboard with the last successful sync time.

Usage collection follows the tool's data, independently of the IDE hosting it.
Other AI tools and remote-only sessions without local logs are not captured.
Using Claude models through Cursor counts under Cursor, not Claude Code. This
service reads the three supported tools' usage sources; it is not a system-wide
network interceptor. Retained local logs determine Claude/Codex history coverage.
The existing Anthropic proxy remains at `localhost:8788`; Codex and Cursor do not
need their model endpoints changed. Host data/session directories are mounted
read-only. Only accounting metadata is persisted; prompts and access tokens are
not stored in the ledger or logs. API credentials are never baked into the image.

### Periods and cost calculations

The unified dashboard opens on **This month**, from the first day of the current
calendar month in `Asia/Ho_Chi_Minh`. Today, Last 7 days, Last 30 days, and All
collected history are also available. Filtering does not limit background
collection or delete older data.

The unified view separates cached tokens and shows missing prices explicitly.
Claude costs are API-rate estimates from `pricing.json`, Cursor costs are reported
charges where available. Codex costs are calculated per request from exclusive
input/cache-read/cache-write/output counts using the live LiteLLM public catalog:
https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json
The backend refreshes hourly, keeps an atomic disk cache alongside gateway.db,
and exposes refresh time/errors/staleness in `/_usage`. Failed refreshes retain
last good prices; initial offline defaults were verified on 2026-09-17 against
https://developers.openai.com/api/docs/models/gpt-6-astra and
https://developers.openai.com/api/docs/models/gpt-5.6-sol.
Historical usage is repriced at **current standard API-equivalent rates**, with
context thresholds applied to individual requests, excluding Fast/priority premiums.
Unknown model IDs (including codex-auto-review) use an explicitly labelled Sol
fallback; missing cache-write rates use regular input rates. Catalog updates
can lag provider changes: the displayed fetch timestamp is not a provider guarantee.
Subscription fees are not included: **this is not an invoice total**. Claude totals
use retained transcripts once available, avoiding double-counting proxy requests;
older gateway-only records remain in the clearly labelled proxy section. Importing
external usage does not change proxy budget enforcement. Cursor refreshes 365 days
of account history; imported history is retained in SQLite.

| Source | Cost basis | Updates |
| --- | --- | --- |
| Claude Code | Token estimate from optional `/config/pricing.json`, otherwise bundled rates | Edit the host file and restart; retained transcript estimates are recomputed on startup |
| Codex | Per-request token estimate, including cache and context tiers | LiteLLM fetched hourly; historical requests valued at current standard rates |
| Cursor | API-reported `chargedCents / 100` | Every usage poll; missing charges remain unpriced |

The total combines these usage values. It does not add your monthly subscription
fee. Event counts are provider-specific and should not be compared as equal units.

### Host paths and configuration

Configure host paths through `GATEWAY_DATA_DIR`, `GATEWAY_CONFIG_DIR`,
`CODEX_SESSIONS_DIR`, `CODEX_ARCHIVED_DIR`, `CLAUDE_PROJECTS_DIR`,
`CURSOR_STATE_DIR`, and `GATEWAY_PORT`. See `compose.yaml` for macOS defaults;
set the Cursor state directory for Linux/Windows hosts. Mount its directory, not
only the database file, so SQLite WAL updates and refreshed sessions stay visible.
The collector reads only `cursorAuth/accessToken`, `cursorAuth/cachedEmail` and
`cursorAuth/cachedTeam`; enable this only for your own authorized session.

| Variable | Default host path/value | Purpose |
| --- | --- | --- |
| `GATEWAY_DATA_DIR` | `$HOME/.local/share/ai-gateway` | Writable SQLite ledger and downloaded `model-prices.json` cache |
| `GATEWAY_CONFIG_DIR` | `$HOME/.config/ai-gateway` | Optional `pricing.json` and `gateway.yaml`; mounted read-only |
| `CODEX_SESSIONS_DIR` | `$HOME/.codex/sessions` | Active Codex sessions |
| `CODEX_ARCHIVED_DIR` | `$HOME/.codex/archived_sessions` | Archived Codex sessions |
| `CLAUDE_PROJECTS_DIR` | `$HOME/.claude/projects` | Claude Code transcripts, including subagents |
| `CURSOR_STATE_DIR` | `$HOME/Library/Application Support/Cursor/User/globalStorage` | Directory containing `state.vscdb` and its WAL files |
| `GATEWAY_PORT` | `8788` | Host port, bound to `127.0.0.1` |

Export overrides before invoking Compose, or place them in a local `.env`
(gitignored). Use absolute paths. For example:

```sh
export CURSOR_STATE_DIR="/absolute/path/to/Cursor/User/globalStorage"
export CODEX_SESSIONS_DIR="/absolute/path/to/.codex/sessions"
docker compose up -d --build
```

Linux/Windows users must locate their actual source directories and override
these macOS defaults. The verified deployment is macOS with Docker Desktop;
ensure Docker file sharing permits access to the selected folders.

### Verify all three collectors

```sh
docker compose ps
docker compose logs --tail 50
curl 'http://localhost:8788/_usage'          # current month (default)
curl 'http://localhost:8788/_usage?days=30'  # 0 = all collected history
curl http://localhost:8788/healthz
```

`/healthz` checks the HTTP service; `/_usage` includes each collector's health.
Confirm `sources.claude_code.state`, `sources.codex.state`, and
`sources.cursor.state` are `ok`, with non-null `last_success` values. Each source
with recorded activity should have entries in `rows`; `ok` with zero rows only
means the source was read successfully, not that activity was found. Run a small
interaction in each tool, wait for its polling interval, and check the token
counts and sync timestamps again. Cursor can report usage with an upstream delay.

`pricing.updated_at` identifies the latest successful catalog fetch;
`pricing.stale` becomes true when the cached rates are over two hours old or no
fetch has succeeded. The dashboard shows the same status. `?period=month` is an
explicit alternative to the default; do not combine it with `days`.

Proxy-only stats and Connect RPC stay at `/_stats` and `/rpc/`.
The separate `Dockerfile` still builds the existing cloud API deployment. Local
external usage is not synced to Turso by the proxy's existing sync mechanism.

### Troubleshooting and maintenance

| Symptom | Check/action |
| --- | --- |
| Port 8788 already in use | Stop the old gateway service, or export a different `GATEWAY_PORT` and use that port in the dashboard URL. |
| Claude/Codex shows no records or source errors | Confirm the mounted host folder contains JSONL sessions and Docker can read it. Remote sessions without local logs cannot be recovered by this collector. |
| Cursor session missing, HTTP 401/403, or account verification fails | Sign in to the intended account in Cursor, confirm `CURSOR_STATE_DIR`, and wait for the next poll. Never paste the token into configuration or logs. |
| Cursor API/schema error persists after signing in | Inspect the collector error; the private Cursor RPC may require a code update. Previously imported records remain available. |
| Prices show stale/error | Check container access to `raw.githubusercontent.com`. Last good rates remain usable and the hourly refresh retries automatically. |
| One source fails while others work | Read that source's `error` and `last_success` in `/_usage`; collectors operate independently. |

```sh
docker compose up -d --build   # rebuild after pulling code updates
docker compose restart       # reload host configuration
docker compose stop          # pause; preserves host database
docker compose start         # resume
docker compose down          # remove container/network; host data remains
```

Persistent state is in `GATEWAY_DATA_DIR`, outside the container. Back up a live
`gateway.db` with SQLite's `.backup` command (rather than copying only the main
file while WAL writes are active). Protect that directory as account usage data.
The only authenticated collector network destination is `api2.cursor.sh`;
public price downloads go to `raw.githubusercontent.com` without credentials.

## Terminal companion: `aictl`

The same repository builds two binaries with different responsibilities:

```text
backend/cmd/
├── gateway/    # long-running proxy, collectors, dashboard and cloud API
└── aictl/      # read-only commands that call the local gateway API
```

The local Docker image includes both. No host Go installation is needed:

```sh
docker compose exec -T gateway aictl status
docker compose exec -T gateway aictl usage --month
docker compose exec -T gateway aictl usage --days 30 --json
docker compose exec -T gateway aictl doctor
docker compose exec -T gateway aictl export --month --format csv > usage.csv
```

To install a native CLI (Go matching `backend/go.mod` required):

```sh
cd backend
go install ./cmd/aictl
cd ..
"$(go env GOPATH)/bin/aictl" status
```

Add Go's binary directory to PATH to use `aictl` directly. If `GOBIN` is set,
Go installs there instead. For a different host port, pass
`aictl --url http://localhost:8789 usage`. Within the gateway container, keep the
default URL because the service still listens on port 8788 internally.

| Command | Result |
| --- | --- |
| `aictl status` | HTTP health, collector states, last sync and price freshness |
| `aictl usage --month` | Per-source/model token counts and usage value, with unpriced/estimated/fallback counts |
| `aictl doctor` | Checks reported collector state, overdue syncs, empty local log sources, and stale prices; suggests fixes |
| `aictl export --month --format csv` | One aggregate row per source/model, including token categories, period and price metadata |

`usage` and `export` default to the current month in the gateway's timezone.
Use `--days N` for a rolling range or `--days 0` for all history; it cannot be
combined with `--month`. Every command accepts `--json`, `--url`, and
`--timeout 15s`. Export also supports `--format json`. Data goes to stdout,
errors to stderr; CSV goes directly to stdout so shell redirection chooses the
output file. CSV escapes spreadsheet formula prefixes in model/source names.

The CLI uses the same `/_usage` calculations as the dashboard. It does not read
Cursor credentials, change configuration, trigger collection, or calculate prices
independently. `doctor` diagnoses server-reported results; it does not directly
inspect host paths or Docker mounts. It exits 1 for warnings/errors, useful for
scripts; successful commands (including empty usage) exit 0. `status` displays
degraded collectors without failing when both APIs are reachable; use `doctor`
for a strict health check. Failed HTTP requests exit 1.

The existing `gateway stats` remains available for backward-compatible,
database-based proxy-only statistics. Use `aictl usage` for all three tools.

## Development

Use Go matching `backend/go.mod` and Node 24 for local development. Docker builds
both components, so these host runtimes are optional for normal use.

```sh
cd backend
go test ./...
go build ./cmd/gateway ./cmd/aictl
cd ../frontend
npm ci
npm test
npm run build
npm run lint
cd ..
docker compose up -d --build
```

For frontend iteration, leave Docker running and run `npm run dev` in
`frontend/`; its API defaults to localhost:8788. The production dashboard uses
the same origin. Relevant implementation entry points:

- `backend/internal/usage/`: provider parsers, `collector.go` for polling and `summary.go` for the HTTP summary API.
- `backend/internal/aictl/`: CLI commands, HTTP client, reports and diagnostics; `cmd/aictl/main.go` handles process exit only.
- `backend/internal/store/usage.go`: idempotent imports and consistent summaries.
- `backend/internal/pricing/catalog.go`: hourly catalog refresh, disk cache and Codex estimates.
- `frontend/src/features/usage/`: unified usage, period controls and source status.
- `frontend/src/features/proxy/`: proxy budgets, cache, charts and request diagnostics.
- `frontend/src/features/auth/`: dashboard token handling and authentication boundary.
- `frontend/src/features/currency/`: currency context, exchange rates and formatting.
- `frontend/src/components/`: shared layout; `lib/`: shared transport/format helpers.
- `backend/gen/` and `frontend/src/gen/`: generated clients; edit `proto/` and run `buf generate` to regenerate, rather than editing generated files.
- `compose.yaml` / `Dockerfile.local`: local deployment; `Dockerfile`: cloud API.

The Anthropic proxy is optional for collection. To use its routing, cache and
budget features, launch Claude Code with `ANTHROPIC_BASE_URL=http://localhost:8788`
and keep your normal Claude authentication. These proxy controls apply only to
requests routed through it; they do not intercept Codex or Cursor requests.

### Migrating from the macOS launch agent

Back up the live SQLite database using SQLite's backup API, then disable/unload
the previous agent to free port 8788 before starting Compose. For example, save
a timestamped backup under `~/.local/share/ai-gateway/gateway.before-docker-*.db`.
The container reuses that directory and preserves the existing calls database.

For the existing `com.hieudang.ai-gateway` installation (adjust the label/path
if yours differs):

```sh
launchctl disable gui/$(id -u)/com.hieudang.ai-gateway
launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.hieudang.ai-gateway.plist
docker compose up -d --build
```

Rollback to the previous installed binary (the additional usage table is harmless):

```sh
docker compose down
launchctl enable gui/$(id -u)/com.hieudang.ai-gateway
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.hieudang.ai-gateway.plist
```
