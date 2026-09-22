# ai-gateway

Local Docker dashboard that automatically collects Claude Code, Codex, and
Cursor usage across IDEs on your machine, plus a Go proxy in front of the Anthropic API
(point `ANTHROPIC_BASE_URL` at it) with usage/cost logging, budget limits,
model routing and response caching — plus a Connect RPC stats API and a React
dashboard. The separate cloud deployment supports Netlify, Render, and Turso
for proxy statistics; the three-tool collector runs locally.

- `backend/` — Go services `gateway`, `collector`, `usage`, `sessions`, `insights`, `memory-sync` and the `aictl` CLI
- `proto/` — buf-managed Connect RPC schema
- `frontend/` — React + Vite dashboard, shadcn/ui + Tailwind CSS

The setup and maintenance guide below describes the current implementation.
Historical design plans and the former scan service remain available in Git history.

## Local Docker services: all three tools

This monorepo runs six independent services, NATS JetStream and PostgreSQL:

| Service | Responsibility | Owned state |
| --- | --- | --- |
| `gateway` | Dashboard/API entry point, HTTP Anthropic proxy, cache and budgets | PostgreSQL `gateway`: calls, cache, traces and transactional outbox |
| `collector` | Claude/Codex logs and Cursor API polling | PostgreSQL `collector`: durable event outbox |
| `usage` | Deduplicate, price and aggregate events | PostgreSQL `usage` database and `usage-prices` cache |
| `sessions` | Index session metadata, search and request timelines | PostgreSQL `sessions` database and `sessions-prices` cache |
| `insights` | Period comparisons and session-level evidence | PostgreSQL `insights` database and `insights-prices` cache |
| `memory-sync` | Optional user-message delivery to an independently deployed Graphiti API | PostgreSQL `memory_sync`: delivery identities/checkpoints |
| `nats` | Durable event transport | `nats-data` JetStream volume |
| `postgres` | Six service databases in one local PostgreSQL 17 cluster | `usage-data` volume |

Only gateway port 8788 is published, bound to localhost. The gateway forwards
`/_usage`, `/_sessions` (including `/detail`), and `/_insights` to their owning
services over HTTP; Anthropic streaming also stays HTTP.
Producers publish content-free metadata on `usage.ingested.v1`; collection
status uses `collector.status.v1`. The collector and optional memory-sync service
mount source data read-only. Services do not read each other's databases during normal use.

Producers queue in their own PostgreSQL databases before publishing and delete only after JetStream
confirms storage. Gateway call/event writes share one transaction. Usage
acknowledges after PostgreSQL commits; event receipts, source/event identities
and alias tombstones prevent replay from double-counting. Statuses follow
their imported batches. Dashboard totals are eventually consistent; overdue
sync timestamps expose an outage while saved usage remains available.
Invalid source records fail validation before enqueueing. Permanently invalid
messages from the bus are acknowledged only after their fingerprint and reason
are stored in `rejected_events`, so a poison message cannot block other sources.
Database failures remain unacknowledged and retry. Inspect quarantine reasons
with `docker compose exec -T postgres psql -U usage -d usage -c 'SELECT subject,
reason,rejected_at FROM rejected_events ORDER BY rejected_at DESC LIMIT 20'`.

JetStream retains up to 30 days / 2 GiB and rejects new messages at its size
limit, leaving them queued locally. Monitor disk space and sync freshness.
This is a single-node local deployment. The Compose PostgreSQL password default
is for local development; override `POSTGRES_PASSWORD` before initializing a
different environment. Alert rules and historical price snapshots are not implemented.

### Optional local memory sync

The separately deployed `graphiti-memory` service can ingest all available
human-authored user messages from Claude Code, Codex, and modern Cursor local
conversation storage. Short messages are included; assistant replies, tool output,
and recognized injected messages are excluded. This is separate from usage
accounting and sends message content only to the configured Graphiti API.
See [the architecture and HTTP contract](docs/architectural.md).

Deploy Graphiti's API and worker first, with its Ollama extraction/embedding models
and Neo4j configuration. Set these values in ai-gateway's gitignored `.env`:

```dotenv
GRAPHITI_URL=http://host.docker.internal:8793
GRAPHITI_API_TOKEN=the-same-token-configured-on-graphiti
GRAPHITI_GROUP_ID=personal
```

Then rebuild and start with `docker compose up -d --build`. With no `GRAPHITI_URL`,
memory-sync reports `disabled` and does not read transcripts. With a configured
URL it checks Graphiti readiness, scans historical/local updates, and sends
deduplicated batches. A dependency outage pauses delivery and retries later.
Durable acceptance by Graphiti means queued, not successfully extracted yet.
Invalid or conflicting individual messages are retained in sender quarantine,
with `blocked` counts, while valid messages continue. Status `pending` includes
these blocked records. Inspect safe rejection reasons with:

```sh
docker compose exec -T postgres psql -U usage -d memory_sync -c "SELECT source,state,error,count(*) FROM memory_deliveries d LEFT JOIN memory_delivery_errors e USING(identity) WHERE state='blocked' GROUP BY source,state,error"
```

```sh
curl http://localhost:8788/_memory
docker compose logs --tail 50 memory-sync
```

Open **Local Memory** in the dashboard (`/dashboard/#local-memory`) for optional
configuration status, local dependency readiness, per-source delivery counts,
and the Graphiti extraction queue. Delivery acceptance is distinct from completed
extraction; Graphiti queue counts also include historical and hook-origin jobs.
The view refreshes every 30 seconds, retains clearly labeled last-known counts
on failures, and warns when the service snapshot is over two minutes old.
Dependency readiness is not a worker heartbeat. The header's **Usage synced**
indicator only describes usage collectors, not memory extraction.

`/_memory` exposes per-source collection/delivery status without chat text.
Use Graphiti's authenticated `/v1/status` to inspect dependency readiness and
extraction backlog. The default group `personal` matches the existing Claude
memory hooks. An unavailable source, unsupported Cursor schema, or remote/deleted
conversation absent from local storage cannot be treated as successfully synced.

`CURSOR_STATE_DIR` selects modern Cursor global conversation storage; legacy-only
workspace storage is reported as unsupported. Graphiti is reached over HTTP, never by mounting
its queue or reading its `.env`. For a remote Graphiti deployment, use a trusted
TLS endpoint and configure the same API token on both services.

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
umask 077
test -e "$HOME/.codex/session_index.jsonl" || touch "$HOME/.codex/session_index.jsonl"
test -f "$HOME/Library/Application Support/Cursor/User/globalStorage/state.vscdb"
```

Each `test` should exit successfully. If a source is elsewhere, configure its
path below. Do not create an empty Cursor database to bypass this check.
No `pricing.json` or `gateway.yaml` is required for collection: built-in
defaults apply when these optional files are absent.

4. Build and start the services (existing users: migrate below first):

```sh
docker compose up -d --build
```

Open **http://localhost:8788/dashboard/**. The services run continuously with
`restart: unless-stopped`. Docker must be running and the host must be awake.
Closing the dashboard does not stop collection. Initial imports run at startup;
large local histories can take longer than a normal poll.

The header links to **Overview** (usage and proxy diagnostics), **Sessions**
(search and request timelines), **Insights** (comparisons and evidence), and
**Data & Pricing** (collector status, coverage and price assumptions).
Links use `#overview`, `#sessions`, `#insights`, and `#data-pricing`, so refresh and browser
Back/Forward work without server routing changes. Switching views preserves the
selected period and currency; a fresh page load defaults to the current month.
The shared sync indicator flags missing/overdue collectors and connection errors.
“Refresh status” reloads the API snapshot; it does not trigger a collector poll.
Session workspace attribution uses the recorded working directory; it is not a
verified repository identity. Historical price snapshots are not implemented.
Data & Pricing leads with sync health, each tool's amount and its pricing basis,
then highlights unpriced events and substitute Codex prices. Collection details
are expandable. The shared typography scale in `frontend/src/index.css` keeps
all pages readable with larger text.
The header shows one sync timestamp with a green status dot when all collectors
are healthy; warnings remain visible if collection falls behind.

### Cost calculations

| Source | Cost basis | Updates |
| --- | --- | --- |
| Claude Code | Token estimate from optional `/config/pricing.json`, otherwise bundled rates | Edit the host file and restart; retained transcript estimates are recomputed on startup |
| Codex | Per-request token estimate, including cache and context tiers | LiteLLM fetched hourly; historical requests valued at current standard rates |
| Cursor | API-reported `chargedCents / 100` | Every usage poll; missing charges remain unpriced |

The total combines these usage values. It does not add your monthly subscription
fee. Event counts are provider-specific and should not be compared as equal units.

### Host paths and configuration

Configure host paths through `GATEWAY_CONFIG_DIR`,
`CODEX_SESSIONS_DIR`, `CODEX_ARCHIVED_DIR`, `CODEX_SESSION_INDEX`, `CLAUDE_PROJECTS_DIR`,
`CURSOR_STATE_DIR`, and `GATEWAY_PORT`. See `compose.yaml` for macOS defaults;
set the Cursor state directory for Linux/Windows hosts. Mount its directory, not
only the database file, so SQLite WAL updates and refreshed sessions stay visible.
The collector reads only `cursorAuth/accessToken`, `cursorAuth/cachedEmail` and
`cursorAuth/cachedTeam`; enable this only for your own authorized session.

| Variable | Default host path/value | Purpose |
| --- | --- | --- |
| `MIGRATION_DIR` | `$HOME/.local/share/ai-gateway/postgres-migration` | Offline SQLite snapshots mounted only by the migration profile |
| `POSTGRES_PASSWORD` | `local-ai-gateway` | Local development database password; set before creating the PostgreSQL volume |
| `GATEWAY_CONFIG_DIR` | `$HOME/.config/ai-gateway` | Optional `pricing.json` and `gateway.yaml`; mounted read-only |
| `CODEX_SESSIONS_DIR` | `$HOME/.codex/sessions` | Active Codex sessions |
| `CODEX_ARCHIVED_DIR` | `$HOME/.codex/archived_sessions` | Archived Codex sessions |
| `CODEX_SESSION_INDEX` | `$HOME/.codex/session_index.jsonl` | Read-only thread names; must be a regular file (an empty file is valid) |
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
curl 'http://localhost:8788/_sessions?period=month&limit=25'
curl 'http://localhost:8788/_insights?period=month'
```

`/healthz` checks HTTP and the gateway database; `/_usage` includes each collector's health.
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
| All sources stop updating | Check `docker compose logs usage collector nats` and PostgreSQL health. `docker compose exec -T collector wget -qO- http://127.0.0.1:8789/healthz` reports queued envelopes as `pending_events`; it should drain after NATS recovers. |

```sh
docker compose up -d --build   # rebuild after pulling code updates
docker compose restart       # reload host configuration
docker compose stop          # pause; preserves PostgreSQL volumes
docker compose start         # resume
docker compose down          # remove containers/network; database volumes remain
```

All runtime application tables now live in PostgreSQL: separate databases
`gateway`, `collector`, `usage`, `sessions`, `insights`, and `memory_sync` on the existing `usage-data` volume. The
`postgres-init` one-shot service creates missing databases on both new and
existing clusters; it never drops an existing database. Port 5432 is not
published. This local setup uses the same development owner for all six;
database separation does not provide role-level isolation.

Back up all databases and retain JetStream state:

```sh
umask 077
docker compose exec -T postgres pg_dumpall -U usage > postgres-backup.sql
```

Restore into a stopped, empty replacement cluster with `psql -U usage -d postgres`
and that dump; do not load a full dump over live service databases. Preserve
`nats-data` too: it may hold acknowledged producer events awaiting consumption.
`docker compose down` preserves volumes; `down -v` deletes them and must not be
used as a routine restart. Original SQLite files and the legacy
`collector-data` volume are retained for rollback, not mounted by normal services.
The collector still reads Cursor's SQLite session file as an external read-only
source; it does not control or migrate Cursor's own database.
The only authenticated collector network destination is `api2.cursor.sh`;
public price downloads go to `raw.githubusercontent.com` without credentials.

### Migrate an existing SQLite installation to PostgreSQL

For a fresh install, just use `docker compose up -d --build`. For existing
SQLite gateway/collector installations, migrate **before** starting the new
runtime configuration. Run these commands in the repo root; macOS includes
`sqlite3` (install it first on other hosts). Use the same Compose project name
and `POSTGRES_PASSWORD` as the existing deployment.

1. Build, provision the service databases, then pause writers. Back up the
existing PostgreSQL usage ledger as well as both SQLite files:

```sh
docker compose build gateway
docker compose up -d postgres nats postgres-init
docker compose stop gateway collector usage
umask 077
export MIGRATION_DIR="$HOME/.local/share/ai-gateway/postgres-migration-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$MIGRATION_DIR/collector-raw"
docker compose exec -T postgres pg_dumpall -U usage > "$MIGRATION_DIR/postgres-before.sql"
gateway_data="${GATEWAY_DATA_DIR:-$HOME/.local/share/ai-gateway}"
sqlite3 "$gateway_data/gateway.db" ".backup '$MIGRATION_DIR/gateway.db'"
docker compose cp collector:/data/. "$MIGRATION_DIR/collector-raw/"
sqlite3 "$MIGRATION_DIR/collector-raw/collector.db" ".backup '$MIGRATION_DIR/collector.db'"
```

The collector copy includes its WAL files and is made only after stopping it.
For the older single-process version, there is no collector container/outbox:
skip its copy and collector migration. Never pass a live WAL database as a
migration source; use the completed `.backup` snapshots.

2. Copy gateway history and any queued collector events:

```sh
docker compose --profile migration run --rm migrate
docker compose --profile migration run --rm \
  -e 'DATABASE_URL=postgres://usage@postgres:5432/collector?sslmode=disable' \
  migrate -kind collector -source /migration/collector.db
```

Each migration copies and verifies every row of the supported tables in one
transaction, including original call IDs, trace fields, cache bodies, sync
checkpoints, outbox payloads/IDs, and the gateway event origin. Output reports
row counts and SHA-256 checksums per table. Destination tables must be empty;
a mismatch or unknown source table rolls back the copy. An identical completed
snapshot can be rerun safely after services resume; a changed snapshot is rejected.
Original sources are never deleted. New PostgreSQL IDs continue above the copied
IDs, and preserved event origins prevent duplicate backfills into usage.

3. If upgrading from the **single-process** version, import the old unified
usage ledger too (skip this when the `usage` service already owns your history):

```sh
docker compose run --rm --no-deps \
  -v "$MIGRATION_DIR/gateway.db:/legacy.db:ro" usage -import-sqlite /legacy.db
```

4. Start and verify:

```sh
docker compose up -d
docker compose exec -T gateway aictl doctor
docker compose exec -T postgres psql -U usage -d gateway -c 'SELECT count(*) FROM calls'
docker compose exec -T postgres psql -U usage -d collector -c 'SELECT count(*) FROM event_outbox'
```

## Terminal companion: `aictl`

The same Go module builds six services and a companion CLI:

```text
backend/
├── services/
│   ├── gateway/   # HTTP entry point and Anthropic proxy
│   ├── collector/ # collection and durable publishing
│   ├── usage/     # PostgreSQL ledger and summary API
│   ├── sessions/  # searchable session projection and timeline API
│   ├── insights/  # independent analytics projection and API
│   └── memory-sync/ # optional user-message synchronization to Graphiti
├── cli/aictl/     # read-only commands using gateway HTTP API
├── contracts/events/ # versioned service event types
└── internal/     # service implementations and shared packages
```

The image includes all binaries; Compose runs separate containers with their
own entrypoints and state. No host Go installation is needed:

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
go install ./cli/aictl
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
go build ./services/... ./cli/aictl
cd ../frontend
npm ci
npm test
npm run build
npm run lint
cd ..
docker compose up -d --build
```

Run PostgreSQL and isolated NATS integration tests with:

```sh
docker compose --profile test run --build --rm tests
```

They create and remove temporary PostgreSQL schemas without changing real usage.
NATS tests use the separate `nats-test` service, never the production stream. Plain
`go test ./...` skips PostgreSQL/NATS tests unless `TEST_DATABASE_URL` /
`TEST_NATS_URL` are set, respectively.
Coverage includes outbox rollback/restart, duplicate and alias replay, stale
status ordering, independent durable fan-out/restart, indexed search and pagination,
metadata updates, per-request pricing, insights thresholds, and proxy regressions. The optional test
service does not run during a normal Compose start.

For frontend iteration, leave Docker running and run `npm run dev` in
`frontend/`; its API defaults to localhost:8788. The production dashboard uses
the same origin.

### Dashboard languages and motion

The dashboard supports English, Vietnamese, Korean, Simplified Chinese and
German. The header language picker changes labels and number/date formatting
without reloading or changing filters, currency, time ranges or user content.
The choice is stored locally; blocked browser storage falls back to in-memory
preferences. Initial language follows supported browser preferences, otherwise
English. Traditional Chinese is not treated as Simplified Chinese.

Bundled translations live in `frontend/src/i18n/`, split into `common`, `usage`,
`sessions` and `memory`. Add matching keys and interpolation placeholders to all
five dictionaries; resource parity tests prevent silently missing translations.
Keep API/query keys, model IDs and user-generated content language-independent.
Known errors use stable codes translated at render time. Number/date helpers use
`Intl`; changing language does not change the selected currency or data timezone.

Page transitions, navigation indicators and control feedback use short native
animations and respect `prefers-reduced-motion`. Polling and language changes
do not replay page entrances or reset the network camera. Explicit camera reset
and view controls still reposition the network. The original sky-blue theme,
typography, spacing and rounded cards are retained; i18n/motion do not replace
the visual design. Language selection uses the same shadcn/Radix Select as other
dashboard filters, including keyboard navigation, focus restoration and menu
animations. Motion uses the existing Tailwind + `tw-animate-css` stack; no new
animation dependency is required.

Named `data-motion` regions reveal once per page visit with capped stagger delays.
Use stable IDs (never translated labels); `data-motion-reveal="false"` opts a
region into update feedback only. Optional `data-motion-update` signatures must
contain raw values, not localized output or polling timestamps, so unchanged
polls and language changes do not flash. `motion-card` enables hover lift only on
selected leaf cards; `motion-loading` is reserved for real pending states. The
observer removes timers/listeners on page changes and honors reduced motion.
See the
[design spec](docs/superpowers/specs/2026-09-22-dashboard-i18n-motion-design.md)
and [implementation plan](docs/superpowers/plans/2026-09-22-dashboard-i18n-motion.md).

UI primitives come from [shadcn/ui](https://ui.shadcn.com): Select, Button, Card,
Table, Badge, Input, Label, Alert, Accordion, Progress, Toggle Group, Navigation
Menu and Chart. The chart uses shadcn's Recharts integration. Use these shared
components instead of building another native control or custom UI primitive.
Feature components still own fetching, formatting and domain logic.

To add a component, run `npx shadcn@latest add <component>` from `frontend/`.
`components.json` configures the registry and `@/` aliases. Generated source is
checked into `src/components/ui/`; it is normal for shadcn to live in the repo.
Theme colors, radius and typography live in `src/index.css`. Shared controls
use a 44px default height to accommodate the larger type. Verify imports use
`@/lib/utils`, review dependency changes, and run the frontend checks above.

Relevant implementation entry points:

- `backend/internal/usage/`: provider parsers, `collector.go` for polling and `summary.go` for the HTTP summary API.
- `backend/internal/aictl/`: CLI logic; `cli/aictl/main.go` handles process exit only.
- `backend/internal/eventbus/`: PostgreSQL outbox, JetStream publisher and durable consumer; explicit legacy SQLite support.
- `backend/internal/ledger/`: PostgreSQL schema, usage reads and explicit legacy import.
- `backend/internal/explore/`: independent sessions/insights projections, search indexes, queries and HTTP APIs.
- `backend/internal/store/`: gateway PostgreSQL schema, calls/cache/outbox, plus legacy SQLite compatibility.
- `backend/internal/migration/`: transactional SQLite snapshot copy, checksum verification and safe reruns.
- `backend/internal/pricing/catalog.go`: hourly catalog refresh, disk cache and Codex estimates.
- `frontend/src/features/usage/`: unified usage, period controls and source status.
- `frontend/src/features/sessions/` and `features/insights/`: indexed search, timelines, comparisons and evidence links.
- `frontend/src/features/proxy/`: proxy budgets, cache, charts and request diagnostics.
- `frontend/src/features/auth/`: dashboard token handling and authentication boundary.
- `frontend/src/features/currency/`: currency context, exchange rates and formatting.
- `frontend/src/components/`: shared header; `components/ui/`: shadcn primitives; `lib/`: shared transport/format helpers and `cn` class merging.
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
Follow the PostgreSQL migration steps above to import gateway history (and the
old unified ledger, if present) before starting the new services. The source
SQLite file remains a rollback snapshot; Docker no longer writes to it.

For the existing `com.hieudang.ai-gateway` installation (adjust the label/path
if yours differs):

```sh
launchctl disable gui/$(id -u)/com.hieudang.ai-gateway
launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.hieudang.ai-gateway.plist
# Complete the SQLite-to-PostgreSQL migration above, then start Compose.
```

Before any new PostgreSQL writes, rollback to the previous installed binary:

```sh
docker compose down
launchctl enable gui/$(id -u)/com.hieudang.ai-gateway
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.hieudang.ai-gateway.plist
```
