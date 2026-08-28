# AI Gateway Backend Implementation Plan (Plan 1 of 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Repurpose this repo into the `ai-gateway` monorepo backend: local Go proxy in front of the Anthropic API with budget limits, model routing, response cache, Connect RPC stats API, and Turso sync.

**Architecture:** The proven MVP proxy (reverse proxy + SSE usage tap, from `~/Leapxpert/tools/personal-ai-gateway`) moves into `backend/` split into internal packages. Control features intercept `POST /v1/messages` before proxying. A Connect RPC `StatsService` (proto-first via buf) serves the dashboard, both locally (`gateway serve`) and in the cloud (`gateway api` on Render, reading Turso).

**Tech Stack:** Go ≥1.22 (pure Go, `CGO_ENABLED=0` must always work), `modernc.org/sqlite`, `connectrpc.com/connect`, buf (remote plugins), `gopkg.in/yaml.v3`, `github.com/tursodatabase/libsql-client-go`, `github.com/rs/cors`, `golang.org/x/net/http2/h2c`.

Spec: `docs/superpowers/specs/2026-08-28-ai-gateway-design.md`. Frontend + Netlify/Render deploy is **Plan 2**, written after this plan lands.

## Global Constraints

- Go module path: `github.com/hieudang-lxp/ai-gateway/backend` (repo renamed to `ai-gateway` in Task 1).
- **Fail-open invariant:** control/sync/logging failures must never break a proxied Claude Code request. Only budget `hard` and model block-list reject requests, always with an Anthropic-shaped JSON error.
- Budget periods are **calendar** day / Mon-start week / month in **Asia/Ho_Chi_Minh**.
- DB stores **USD only**.
- Git commits: NO `Co-Authored-By: Claude` trailer, ever (user rule).
- MVP behavior must be preserved: default listen `localhost:8788`, `Accept-Encoding` stripped, custom `usageTap` (never `io.TeeReader`), finalize on body close, store failures only log.
- The repo dir after Task 1 is `~/Leapxpert/tools/ai-gateway`. All commands below run from there unless stated.

---

### Task 1: Tag scan-svc, clear repo, rename to ai-gateway

**Files:**
- Delete (git rm): `auth.go`, `cmd/`, `gen/`, `internal/`, `lib/`, `main.go`, `Makefile`, `proto/`, `server.go`, `go.mod`, `go.sum`, `README.md`
- Create: `README.md`, `.gitignore`
- Keep: `docs/`

**Interfaces:**
- Produces: a clean repo named `ai-gateway` at `~/Leapxpert/tools/ai-gateway`, scan-svc recoverable via tag `scan-svc-final`.

- [ ] **Step 1: Tag and push the scan-svc final state**

```bash
cd ~/Leapxpert/tools/lxp-scan-svc
git tag scan-svc-final
git push origin main --tags
```
Expected: tag visible on `git ls-remote --tags origin`.

- [ ] **Step 2: Rename the GitHub repo and local dir**

```bash
gh repo rename ai-gateway --repo hieudang-lxp/lxp-scan-svc --yes
cd ~/Leapxpert/tools && mv lxp-scan-svc ai-gateway && cd ai-gateway
git remote -v   # gh rewrites origin to .../ai-gateway.git automatically; if not: git remote set-url origin https://github.com/hieudang-lxp/ai-gateway.git
```

- [ ] **Step 3: Remove scan-svc code**

```bash
git rm -r auth.go cmd gen internal lib main.go Makefile proto server.go go.mod go.sum README.md
rm -f lxp-scan-svc   # untracked local binary if present
```

- [ ] **Step 4: Write new README.md and .gitignore**

`README.md`:
```markdown
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
```

`.gitignore`:
```
*.db
*.db-shm
*.db-wal
.env
node_modules/
frontend/dist/
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: remove scan service (tagged scan-svc-final), repo becomes ai-gateway"
```

---

### Task 2: Import MVP gateway into backend/ with package layout

**Files:**
- Source (read-only): `~/Leapxpert/tools/personal-ai-gateway/{main,proxy,usage,pricing,server,store}.go`
- Create: `backend/go.mod`, `backend/cmd/gateway/main.go`, `backend/internal/proxy/proxy.go`, `backend/internal/proxy/usage.go`, `backend/internal/proxy/stats_http.go`, `backend/internal/pricing/pricing.go`, `backend/internal/store/store.go`

**Interfaces (Produces — later tasks rely on these exact signatures):**
- `store.Usage{Input, Output, CacheRead, CacheWrite int64}`
- `store.Record{TS time.Time; Model string; Usage Usage; CostUSD float64; LatencyMS int64; Status int}`
- `store.Open(path string) (*Store, error)`, `(*Store).Insert(Record) error`, `(*Store).Close() error`
- `store.StatRow{Model string; Calls, Input, Output, CacheRead, CacheWrite int64; CostUSD float64}`, `(*Store).StatsSince(cutoff time.Time) ([]StatRow, error)`
- `pricing.Load(path string) Pricing`, `(Pricing).Cost(model string, in, out, cacheRead, cacheWrite int64) float64`
- `proxy.New(upstream string, st *store.Store, pr pricing.Pricing) (*Gateway, error)`, `(*Gateway).ServeHTTP`, `(*Gateway).HandleStats(w, r)` (legacy `/_stats` JSON)

- [ ] **Step 1: Create module and copy files**

```bash
mkdir -p backend/cmd/gateway backend/internal/{proxy,pricing,store}
cd backend && go mod init github.com/hieudang-lxp/ai-gateway/backend && cd ..
cp ~/Leapxpert/tools/personal-ai-gateway/proxy.go   backend/internal/proxy/proxy.go
cp ~/Leapxpert/tools/personal-ai-gateway/usage.go   backend/internal/proxy/usage.go
cp ~/Leapxpert/tools/personal-ai-gateway/server.go  backend/internal/proxy/stats_http.go
cp ~/Leapxpert/tools/personal-ai-gateway/pricing.go backend/internal/pricing/pricing.go
cp ~/Leapxpert/tools/personal-ai-gateway/store.go   backend/internal/store/store.go
cp ~/Leapxpert/tools/personal-ai-gateway/main.go    backend/cmd/gateway/main.go
```

- [ ] **Step 2: Mechanical renames per file**

Apply exactly. Behavior must not change.

`backend/internal/store/store.go` — `package store`; export everything:
| old | new |
|---|---|
| `record` | `Record` (fields `TS, Model, Usage, CostUSD, LatencyMS, Status`) |
| `store` (type) | `Store` |
| `openStore` | `Open` |
| `(s *store) insert` | `(s *Store) Insert` |
| `(s *store) close` | `(s *Store) Close` |
| `statRow` | `StatRow` (fields `Model, Calls, Input, Output, CacheRead, CacheWrite, CostUSD`) |
| `statsSince` | `StatsSince` |

Also **move the `usage` struct here** from usage.go as:
```go
// Usage is the token counts extracted from one Anthropic response.
type Usage struct {
	Input      int64
	Output     int64
	CacheRead  int64
	CacheWrite int64
}
```
`Record.Usage` has type `Usage`; `Insert` uses `r.Usage.Input` etc.

`backend/internal/pricing/pricing.go` — `package pricing`; rename `loadPricing`→`Load`; replace the `cost` method (drop the `usage` dependency):
```go
// Cost returns the estimated USD cost of one call.
func (p Pricing) Cost(model string, in, out, cacheRead, cacheWrite int64) float64 {
	r := p.rates(model)
	return (float64(in)*r.Input +
		float64(out)*r.Output +
		float64(cacheRead)*r.CacheRead +
		float64(cacheWrite)*r.CacheWrite) / 1_000_000
}
```
Keep `rates`, `families`, `defaultPricing`, `writeDefaultPricing` unexported/as-is.

`backend/internal/proxy/proxy.go` — `package proxy`; imports `".../internal/pricing"`, `".../internal/store"`:
| old | new |
|---|---|
| `gateway` (type) | `Gateway` (fields `proxy`, `store *store.Store`, `pricing pricing.Pricing`) |
| `newGateway` | `New` |
| `record{...}` in `finalize` | `store.Record{...}` |
| `g.pricing.cost(model, p.u)` | `g.pricing.Cost(model, p.u.Input, p.u.Output, p.u.CacheRead, p.u.CacheWrite)` |
Keep `ctxKey`/`startKey`, `usageTap`, `modifyResponse`, `finalize`, `ServeHTTP` logic byte-for-byte otherwise.

`backend/internal/proxy/usage.go` — `package proxy`; delete the local `usage` struct (moved to store); `parser.u` becomes `store.Usage`; field refs `p.u.input`→`p.u.Input`, `.output`→`.Output`, `.cacheRead`→`.CacheRead`, `.cacheWrite`→`.CacheWrite`. `apiUsage`, `parser`, `sseEvent`, all funcs stay unexported.

`backend/internal/proxy/stats_http.go` — `package proxy`; delete `serve()` (moves to cmd); rename `handleStats`→`HandleStats` (method on `*Gateway`); keep JSON shape identical.

`backend/cmd/gateway/main.go` — `package main`; new default paths (`ai-gateway` instead of `personal-ai-gateway`) and wiring:
```go
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/proxy"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

func main() {
	log.SetFlags(log.LstdFlags)
	log.SetOutput(os.Stderr)
	if len(os.Args) > 1 && os.Args[1] == "stats" {
		runStats(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		printUsage()
		return
	}
	runServe(os.Args[1:])
}

func printUsage() {
	fmt.Fprint(os.Stderr, `ai-gateway — local Anthropic API proxy: usage/cost log, budgets, routing, cache

Usage:
  gateway [serve flags]    Run the proxy (default)
  gateway stats [flags]    Show usage & cost totals

Point your tools at it:
  export ANTHROPIC_BASE_URL=http://localhost:8788
`)
}

func dataPath(file string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return file
	}
	return filepath.Join(home, ".local", "share", "ai-gateway", file)
}

func configPath(file string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return file
	}
	return filepath.Join(home, ".config", "ai-gateway", file)
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "localhost:8788", "listen address")
	upstream := fs.String("upstream", "https://api.anthropic.com", "upstream Anthropic base URL")
	dbPath := fs.String("db", dataPath("gateway.db"), "path to the SQLite store")
	pricingPath := fs.String("pricing", configPath("pricing.json"), "path to pricing.json")
	fs.Parse(args)

	pr := pricing.Load(*pricingPath)
	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	g, err := proxy.New(*upstream, st, pr)
	if err != nil {
		log.Fatalf("build gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/_stats", g.HandleStats)
	mux.Handle("/", g)
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 30 * time.Second}

	log.Printf("ai-gateway listening on http://%s -> %s", *addr, *upstream)
	log.Printf("set: export ANTHROPIC_BASE_URL=http://%s", *addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
```
`runStats`, `printWindow`, `trunc` copy over unchanged except `openStore(...)`→`store.Open(...)`, `statRow`→`store.StatRow` field renames (`r.model`→`r.Model` etc.), and db flag default `dataPath("gateway.db")`.

- [ ] **Step 3: Build and verify**

```bash
cd backend && go mod tidy && go build ./... && go vet ./...
go run ./cmd/gateway --help
```
Expected: builds clean; usage text prints. `go version` must show ≥1.22.

- [ ] **Step 4: Preserve MVP history (optional but do it)**

```bash
mkdir -p ~/.local/share/ai-gateway
cp ~/.local/share/personal-ai-gateway/gateway.db ~/.local/share/ai-gateway/gateway.db 2>/dev/null || true
mkdir -p ~/.config/ai-gateway
cp ~/.config/personal-ai-gateway/pricing.json ~/.config/ai-gateway/pricing.json 2>/dev/null || true
```

- [ ] **Step 5: Commit**

```bash
git add backend && git commit -m "feat: import personal-ai-gateway MVP as backend with package layout"
```

---

### Task 3: Proxy integration test harness (parity evidence)

**Files:**
- Test: `backend/internal/proxy/proxy_test.go`

**Interfaces:**
- Consumes: `proxy.New`, `store.Open/StatsSince`, `pricing.Load` from Task 2.
- Produces: `waitForStats(t, st, wantModels int) []store.StatRow` helper reused by Task 8 tests.

- [ ] **Step 1: Write failing tests (file doesn't exist yet — they fail to even run until written; then run to see PASS/FAIL truthfully)**

`backend/internal/proxy/proxy_test.go`:
```go
package proxy_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/proxy"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func testPricing(t *testing.T) pricing.Pricing {
	t.Helper()
	return pricing.Load(filepath.Join(t.TempDir(), "pricing.json"))
}

// waitForStats polls until StatsSince(epoch) returns wantModels rows (finalize
// runs async on body close).
func waitForStats(t *testing.T, st *store.Store, wantModels int) []store.StatRow {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rows, err := st.StatsSince(time.Unix(0, 0))
		if err != nil {
			t.Fatalf("stats: %v", err)
		}
		if len(rows) >= wantModels {
			return rows
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d stat rows", wantModels)
	return nil
}

func TestProxyJSONResponseLogged(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"msg_1","model":"claude-sonnet-5","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":10,"cache_creation_input_tokens":4}}`)
	}))
	defer upstream.Close()

	st := newTestStore(t)
	g, err := proxy.New(upstream.URL, st, testPricing(t))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(g)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/messages", "application/json",
		strings.NewReader(`{"model":"claude-sonnet-5","messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"msg_1"`) {
		t.Fatalf("bad passthrough: %d %s", resp.StatusCode, body)
	}

	rows := waitForStats(t, st, 1)
	r := rows[0]
	if r.Model != "claude-sonnet-5" || r.Input != 100 || r.Output != 50 || r.CacheRead != 10 || r.CacheWrite != 4 {
		t.Fatalf("bad row: %+v", r)
	}
	// sonnet default rates: (100*3 + 50*15 + 10*0.3 + 4*3.75)/1e6
	want := (100*3.0 + 50*15.0 + 10*0.3 + 4*3.75) / 1e6
	if diff := r.CostUSD - want; diff > 1e-12 || diff < -1e-12 {
		t.Fatalf("cost = %v want %v", r.CostUSD, want)
	}
}

func TestProxySSEResponseLogged(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-opus-4-8\",\"usage\":{\"input_tokens\":200,\"output_tokens\":1,\"cache_read_input_tokens\":0,\"cache_creation_input_tokens\":0}}}\n\n")
		f.Flush()
		fmt.Fprint(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":80}}\n\n")
		f.Flush()
	}))
	defer upstream.Close()

	st := newTestStore(t)
	g, err := proxy.New(upstream.URL, st, testPricing(t))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(g)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/messages", "application/json",
		strings.NewReader(`{"model":"claude-opus-4-8","stream":true,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "message_delta") {
		t.Fatalf("stream not passed through: %s", body)
	}

	rows := waitForStats(t, st, 1)
	r := rows[0]
	if r.Model != "claude-opus-4-8" || r.Input != 200 || r.Output != 80 {
		t.Fatalf("bad row: %+v", r)
	}
}
```

- [ ] **Step 2: Run tests**

Run: `cd backend && go test ./internal/proxy/ -v`
Expected: **PASS** both (this is a parity check on moved code — if either fails, the Task 2 move broke behavior; fix the move, not the test).

- [ ] **Step 3: Commit**

```bash
git add backend/internal/proxy/proxy_test.go
git commit -m "test: proxy integration harness (JSON + SSE usage logging parity)"
```

---

### Task 4: Config loader with hot reload (internal/control)

**Files:**
- Create: `backend/internal/control/config.go`
- Test: `backend/internal/control/config_test.go`

**Interfaces (Produces):**
```go
type Limit struct{ Warn, Hard float64 }                       // yaml: warn, hard
type BudgetConfig struct{ Daily, Weekly, Monthly Limit }      // yaml: daily_usd, weekly_usd, monthly_usd
type Rule struct{ Match, To string }                          // yaml: match, to
type RoutingConfig struct{ Rules []Rule; Block []string }     // yaml: rules, block
type CacheConfig struct{ Enabled bool; TTL time.Duration }    // yaml: enabled, ttl ("1h")
type Config struct{ Budget BudgetConfig; Routing RoutingConfig; Cache CacheConfig }
func LoadConfig(path string) (*Config, error)
func NewWatcher(path string) *Watcher
func (w *Watcher) Current() *Config   // never nil; zero Config = everything off
```

- [ ] **Step 1: Write failing tests**

`backend/internal/control/config_test.go`:
```go
package control

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const sampleYAML = `
budget:
  daily_usd:   {warn: 10, hard: 20}
  weekly_usd:  {warn: 50, hard: 100}
  monthly_usd: {warn: 150, hard: 300}
routing:
  rules:
    - match: "claude-opus-*"
      to: "claude-sonnet-5"
  block:
    - "claude-fable-*"
cache:
  enabled: true
  ttl: 1h
`

func writeCfg(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "gateway.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadConfig(t *testing.T) {
	cfg, err := LoadConfig(writeCfg(t, sampleYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Budget.Daily != (Limit{10, 20}) || cfg.Budget.Monthly != (Limit{150, 300}) {
		t.Fatalf("budget: %+v", cfg.Budget)
	}
	if len(cfg.Routing.Rules) != 1 || cfg.Routing.Rules[0] != (Rule{"claude-opus-*", "claude-sonnet-5"}) {
		t.Fatalf("rules: %+v", cfg.Routing.Rules)
	}
	if len(cfg.Routing.Block) != 1 || cfg.Routing.Block[0] != "claude-fable-*" {
		t.Fatalf("block: %+v", cfg.Routing.Block)
	}
	if !cfg.Cache.Enabled || cfg.Cache.TTL != time.Hour {
		t.Fatalf("cache: %+v", cfg.Cache)
	}
}

func TestLoadConfigBadTTL(t *testing.T) {
	if _, err := LoadConfig(writeCfg(t, "cache:\n  enabled: true\n  ttl: banana\n")); err == nil {
		t.Fatal("want error for bad ttl")
	}
}

func TestWatcherMissingFileIsZeroConfig(t *testing.T) {
	w := NewWatcher(filepath.Join(t.TempDir(), "nope.yaml"))
	cfg := w.Current()
	if cfg == nil || cfg.Cache.Enabled || len(cfg.Routing.Rules) != 0 || cfg.Budget.Daily.Hard != 0 {
		t.Fatalf("want zero config, got %+v", cfg)
	}
}

func TestWatcherReloadsOnChange(t *testing.T) {
	p := writeCfg(t, "cache:\n  enabled: false\n")
	w := NewWatcher(p)
	w.recheck = 0 // test hook: re-stat every call
	if w.Current().Cache.Enabled {
		t.Fatal("want disabled")
	}
	if err := os.WriteFile(p, []byte("cache:\n  enabled: true\n  ttl: 5m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// mtime granularity can be 1s on some filesystems; force it
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(p, future, future); err != nil {
		t.Fatal(err)
	}
	if !w.Current().Cache.Enabled {
		t.Fatal("want reload to pick up enabled=true")
	}
}

func TestWatcherKeepsOldConfigOnParseError(t *testing.T) {
	p := writeCfg(t, "cache:\n  enabled: true\n  ttl: 5m\n")
	w := NewWatcher(p)
	w.recheck = 0
	if !w.Current().Cache.Enabled {
		t.Fatal("want enabled")
	}
	if err := os.WriteFile(p, []byte(":::not yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(p, future, future)
	if !w.Current().Cache.Enabled {
		t.Fatal("fail-open: keep last good config on parse error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/control/ -v`
Expected: FAIL — package doesn't compile (`LoadConfig` undefined).

- [ ] **Step 3: Implement**

`backend/internal/control/config.go`:
```go
// Package control holds the gateway's control plane: config, budget
// decisions, model routing and cache keying. Everything here is pure and
// fail-open: a broken config never breaks the proxy.
package control

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Limit struct {
	Warn float64 `yaml:"warn"`
	Hard float64 `yaml:"hard"`
}

type BudgetConfig struct {
	Daily   Limit `yaml:"daily_usd"`
	Weekly  Limit `yaml:"weekly_usd"`
	Monthly Limit `yaml:"monthly_usd"`
}

type Rule struct {
	Match string `yaml:"match"`
	To    string `yaml:"to"`
}

type RoutingConfig struct {
	Rules []Rule   `yaml:"rules"`
	Block []string `yaml:"block"`
}

type CacheConfig struct {
	Enabled bool          `yaml:"enabled"`
	TTL     time.Duration `yaml:"-"`
	TTLRaw  string        `yaml:"ttl"`
}

type Config struct {
	Budget  BudgetConfig  `yaml:"budget"`
	Routing RoutingConfig `yaml:"routing"`
	Cache   CacheConfig   `yaml:"cache"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Cache.TTLRaw != "" {
		ttl, err := time.ParseDuration(cfg.Cache.TTLRaw)
		if err != nil {
			return nil, fmt.Errorf("cache.ttl: %w", err)
		}
		cfg.Cache.TTL = ttl
	}
	return &cfg, nil
}

// Watcher hands out the current config, re-reading the file when its mtime
// changes. Stat is rate-limited to once per recheck interval.
type Watcher struct {
	path    string
	recheck time.Duration

	mu      sync.Mutex
	cfg     *Config
	mtime   time.Time
	statted time.Time
}

func NewWatcher(path string) *Watcher {
	w := &Watcher{path: path, recheck: 2 * time.Second, cfg: &Config{}}
	w.reload()
	return w
}

// Current returns the latest config; never nil. Missing file → zero config.
// Parse errors keep the previous config (fail-open).
func (w *Watcher) Current() *Config {
	w.mu.Lock()
	defer w.mu.Unlock()
	if time.Since(w.statted) >= w.recheck {
		w.reloadLocked()
	}
	return w.cfg
}

func (w *Watcher) reload() { w.mu.Lock(); defer w.mu.Unlock(); w.reloadLocked() }

func (w *Watcher) reloadLocked() {
	w.statted = time.Now()
	fi, err := os.Stat(w.path)
	if err != nil {
		return // missing file: keep whatever we have (zero config at start)
	}
	if fi.ModTime().Equal(w.mtime) {
		return
	}
	cfg, err := LoadConfig(w.path)
	if err != nil {
		log.Printf("config reload failed, keeping previous: %v", err)
		w.mtime = fi.ModTime() // don't re-log every recheck
		return
	}
	w.cfg = cfg
	w.mtime = fi.ModTime()
	log.Printf("config loaded from %s", w.path)
}
```

```bash
cd backend && go get gopkg.in/yaml.v3 && go mod tidy
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/control/ -v`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/control backend/go.mod backend/go.sum
git commit -m "feat: control config loader with mtime hot reload"
```

---

### Task 5: Budget engine

**Files:**
- Create: `backend/internal/control/budget.go`
- Modify: `backend/internal/store/store.go` (add `SpendSince`)
- Test: `backend/internal/control/budget_test.go`, `backend/internal/store/store_test.go`

**Interfaces (Produces):**
```go
// control
type Verdict int
const (VerdictOK Verdict = iota; VerdictWarn; VerdictBlock)
type BudgetStatus struct{ Verdict Verdict; Reason string }
func EvaluateBudget(cfg BudgetConfig, day, week, month float64) BudgetStatus
func PeriodStarts(now time.Time) (day, week, month time.Time) // Asia/Ho_Chi_Minh calendar
// store
func (s *Store) SpendSince(cutoff time.Time) (float64, error)
```

- [ ] **Step 1: Write failing tests**

`backend/internal/control/budget_test.go`:
```go
package control

import (
	"testing"
	"time"
)

func TestEvaluateBudget(t *testing.T) {
	cfg := BudgetConfig{
		Daily:   Limit{Warn: 10, Hard: 20},
		Weekly:  Limit{Warn: 50, Hard: 100},
		Monthly: Limit{Warn: 150, Hard: 300},
	}
	cases := []struct {
		name             string
		day, week, month float64
		want             Verdict
	}{
		{"all under", 5, 20, 100, VerdictOK},
		{"day warn", 10, 20, 100, VerdictWarn},
		{"day hard", 20, 20, 100, VerdictBlock},
		{"week hard beats day warn", 12, 100, 100, VerdictBlock},
		{"month warn", 5, 20, 200, VerdictWarn},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EvaluateBudget(cfg, c.day, c.week, c.month)
			if got.Verdict != c.want {
				t.Fatalf("verdict = %v (%q) want %v", got.Verdict, got.Reason, c.want)
			}
			if got.Verdict != VerdictOK && got.Reason == "" {
				t.Fatal("want a reason for warn/block")
			}
		})
	}
}

func TestEvaluateBudgetZeroConfigDisabled(t *testing.T) {
	got := EvaluateBudget(BudgetConfig{}, 1e9, 1e9, 1e9)
	if got.Verdict != VerdictOK {
		t.Fatalf("zero limits must disable budgets, got %v", got.Verdict)
	}
}

func TestPeriodStarts(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	// Friday 2026-08-28 10:30 +07
	now := time.Date(2026, 8, 28, 10, 30, 0, 0, loc)
	day, week, month := PeriodStarts(now)
	if !day.Equal(time.Date(2026, 8, 28, 0, 0, 0, 0, loc)) {
		t.Fatalf("day = %v", day)
	}
	if !week.Equal(time.Date(2026, 8, 24, 0, 0, 0, 0, loc)) { // Monday
		t.Fatalf("week = %v", week)
	}
	if !month.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, loc)) {
		t.Fatalf("month = %v", month)
	}
	// Sunday must belong to the week started the previous Monday
	sun := time.Date(2026, 8, 30, 23, 0, 0, 0, loc)
	_, week2, _ := PeriodStarts(sun)
	if !week2.Equal(time.Date(2026, 8, 24, 0, 0, 0, 0, loc)) {
		t.Fatalf("sunday week = %v", week2)
	}
}
```

`backend/internal/store/store_test.go`:
```go
package store

import (
	"path/filepath"
	"testing"
	"time"
)

func open(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestSpendSince(t *testing.T) {
	st := open(t)
	base := time.Now()
	for i, cost := range []float64{1.5, 2.5, 4.0} {
		r := Record{TS: base.Add(time.Duration(-i) * time.Hour), Model: "m", CostUSD: cost, Status: 200}
		if err := st.Insert(r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := st.SpendSince(base.Add(-90 * time.Minute)) // includes cost 1.5 (now) and 2.5 (-1h)
	if err != nil {
		t.Fatal(err)
	}
	if got != 4.0 {
		t.Fatalf("spend = %v want 4.0", got)
	}
	all, _ := st.SpendSince(time.Unix(0, 0))
	if all != 8.0 {
		t.Fatalf("all = %v want 8.0", all)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/control/ ./internal/store/ -run 'Budget|Period|Spend' -v`
Expected: FAIL — `EvaluateBudget`, `PeriodStarts`, `SpendSince` undefined.

- [ ] **Step 3: Implement**

`backend/internal/control/budget.go`:
```go
package control

import (
	"fmt"
	"time"
)

type Verdict int

const (
	VerdictOK Verdict = iota
	VerdictWarn
	VerdictBlock
)

type BudgetStatus struct {
	Verdict Verdict
	Reason  string
}

// vnLoc is the timezone budget periods are anchored to. LoadLocation needs
// tzdata: present on macOS/Linux; the Docker image installs the tzdata pkg.
var vnLoc = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("ICT", 7*3600)
	}
	return loc
}()

// EvaluateBudget compares spends against limits. A zero limit is disabled.
// Hard anywhere → Block; otherwise Warn anywhere → Warn.
func EvaluateBudget(cfg BudgetConfig, day, week, month float64) BudgetStatus {
	type check struct {
		name  string
		spent float64
		lim   Limit
	}
	checks := []check{
		{"daily", day, cfg.Daily},
		{"weekly", week, cfg.Weekly},
		{"monthly", month, cfg.Monthly},
	}
	var warn string
	for _, c := range checks {
		if c.lim.Hard > 0 && c.spent >= c.lim.Hard {
			return BudgetStatus{VerdictBlock,
				fmt.Sprintf("%s budget hard limit reached: $%.2f spent >= $%.2f", c.name, c.spent, c.lim.Hard)}
		}
		if warn == "" && c.lim.Warn > 0 && c.spent >= c.lim.Warn {
			warn = fmt.Sprintf("%s budget warn threshold: $%.2f spent >= $%.2f", c.name, c.spent, c.lim.Warn)
		}
	}
	if warn != "" {
		return BudgetStatus{VerdictWarn, warn}
	}
	return BudgetStatus{VerdictOK, ""}
}

// PeriodStarts returns the calendar day/Mon-week/month starts containing now,
// in Asia/Ho_Chi_Minh.
func PeriodStarts(now time.Time) (day, week, month time.Time) {
	n := now.In(vnLoc)
	day = time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, vnLoc)
	week = day.AddDate(0, 0, -((int(n.Weekday()) + 6) % 7)) // Monday start
	month = time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, vnLoc)
	return
}
```

Add to `backend/internal/store/store.go`:
```go
// SpendSince returns total estimated cost of calls at or after the cutoff.
func (s *Store) SpendSince(cutoff time.Time) (float64, error) {
	var v float64
	err := s.db.QueryRow(
		`SELECT COALESCE(SUM(est_cost_usd), 0) FROM calls WHERE ts >= ?`,
		cutoff.Unix(),
	).Scan(&v)
	return v, err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/control/ ./internal/store/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/control backend/internal/store
git commit -m "feat: budget engine (calendar periods in Asia/Ho_Chi_Minh) + store.SpendSince"
```

---

### Task 6: Model routing

**Files:**
- Create: `backend/internal/control/routing.go`
- Test: `backend/internal/control/routing_test.go`

**Interfaces (Produces):**
```go
func (r RoutingConfig) Route(model string) (to string, blocked bool)
// blocked=true → reject. Otherwise to is the (possibly rewritten) model.
```

- [ ] **Step 1: Write failing tests**

`backend/internal/control/routing_test.go`:
```go
package control

import "testing"

func TestRoute(t *testing.T) {
	r := RoutingConfig{
		Rules: []Rule{
			{Match: "claude-opus-4-8", To: "claude-haiku-4-5-20251001"}, // exact, first
			{Match: "claude-opus-*", To: "claude-sonnet-5"},
		},
		Block: []string{"claude-fable-*"},
	}
	cases := []struct {
		in, want string
		blocked  bool
	}{
		{"claude-fable-5", "", true},                          // block wins
		{"claude-opus-4-8", "claude-haiku-4-5-20251001", false}, // first match wins
		{"claude-opus-4-7", "claude-sonnet-5", false},         // glob
		{"claude-sonnet-5", "claude-sonnet-5", false},         // passthrough
	}
	for _, c := range cases {
		to, blocked := r.Route(c.in)
		if blocked != c.blocked || (!blocked && to != c.want) {
			t.Fatalf("Route(%q) = (%q,%v) want (%q,%v)", c.in, to, blocked, c.want, c.blocked)
		}
	}
}

func TestRouteBadPatternIgnored(t *testing.T) {
	r := RoutingConfig{Rules: []Rule{{Match: "[", To: "x"}}, Block: []string{"["}}
	to, blocked := r.Route("claude-sonnet-5")
	if blocked || to != "claude-sonnet-5" {
		t.Fatalf("bad pattern must be ignored, got (%q,%v)", to, blocked)
	}
}

func TestRouteEmptyConfigPassthrough(t *testing.T) {
	to, blocked := RoutingConfig{}.Route("m")
	if blocked || to != "m" {
		t.Fatal("empty config must pass through")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/control/ -run Route -v`
Expected: FAIL — `Route` undefined.

- [ ] **Step 3: Implement**

`backend/internal/control/routing.go`:
```go
package control

import "path"

// Route applies the block list, then the first matching rewrite rule.
// Patterns use path.Match globs (e.g. "claude-opus-*"); malformed patterns
// are skipped (fail-open).
func (r RoutingConfig) Route(model string) (string, bool) {
	for _, pat := range r.Block {
		if ok, err := path.Match(pat, model); err == nil && ok {
			return "", true
		}
	}
	for _, rule := range r.Rules {
		if ok, err := path.Match(rule.Match, model); err == nil && ok && rule.To != "" {
			return rule.To, false
		}
	}
	return model, false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/control/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/control
git commit -m "feat: model routing (glob block list + rewrite rules)"
```

---

### Task 7: Response cache (store layer + key)

**Files:**
- Create: `backend/internal/control/cachekey.go`
- Modify: `backend/internal/store/store.go` (cache table + Get/Put)
- Test: `backend/internal/store/cache_test.go`, `backend/internal/control/cachekey_test.go`

**Interfaces (Produces):**
```go
// control
func CacheKey(body []byte) string // sha256 hex of the exact outgoing body
// store
type CachedResponse struct{ Status int; ContentType string; Body []byte; CostUSD float64; Model string }
func (s *Store) CacheGet(key string, maxAge time.Duration) (*CachedResponse, bool, error)
func (s *Store) CachePut(key string, c CachedResponse) error
```

- [ ] **Step 1: Write failing tests**

`backend/internal/control/cachekey_test.go`:
```go
package control

import "testing"

func TestCacheKey(t *testing.T) {
	a := CacheKey([]byte(`{"model":"m","messages":[]}`))
	b := CacheKey([]byte(`{"model":"m","messages":[]}`))
	c := CacheKey([]byte(`{"model":"m2","messages":[]}`))
	if a != b {
		t.Fatal("same body must hash equal")
	}
	if a == c {
		t.Fatal("different body must hash different")
	}
	if len(a) != 64 {
		t.Fatalf("want sha256 hex (64 chars), got %d", len(a))
	}
}
```

`backend/internal/store/cache_test.go`:
```go
package store

import (
	"bytes"
	"testing"
	"time"
)

func TestCachePutGet(t *testing.T) {
	st := open(t)
	c := CachedResponse{Status: 200, ContentType: "application/json",
		Body: []byte(`{"ok":true}`), CostUSD: 0.0123, Model: "claude-sonnet-5"}
	if err := st.CachePut("k1", c); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.CacheGet("k1", time.Hour)
	if err != nil || !ok {
		t.Fatalf("miss: ok=%v err=%v", ok, err)
	}
	if got.Status != 200 || got.ContentType != c.ContentType ||
		!bytes.Equal(got.Body, c.Body) || got.CostUSD != c.CostUSD || got.Model != c.Model {
		t.Fatalf("roundtrip: %+v", got)
	}
}

func TestCacheMissAndExpiry(t *testing.T) {
	st := open(t)
	if _, ok, _ := st.CacheGet("absent", time.Hour); ok {
		t.Fatal("want miss")
	}
	if err := st.CachePut("k", CachedResponse{Status: 200, Body: []byte("x")}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, ok, _ := st.CacheGet("k", time.Nanosecond); ok {
		t.Fatal("want expired entry to miss")
	}
}

func TestCachePutOverwrites(t *testing.T) {
	st := open(t)
	_ = st.CachePut("k", CachedResponse{Status: 200, Body: []byte("v1")})
	if err := st.CachePut("k", CachedResponse{Status: 200, Body: []byte("v2")}); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := st.CacheGet("k", time.Hour)
	if !ok || string(got.Body) != "v2" {
		t.Fatal("want upsert to v2")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/store/ ./internal/control/ -run Cache -v`
Expected: FAIL — `CachedResponse`, `CacheKey` undefined.

- [ ] **Step 3: Implement**

`backend/internal/control/cachekey.go`:
```go
package control

import (
	"crypto/sha256"
	"encoding/hex"
)

// CacheKey is an exact-match key over the outgoing request body (after any
// routing rewrite). Byte-identical bodies — and nothing else — share a key.
func CacheKey(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
```

In `backend/internal/store/store.go`, append to the `schema` const:
```sql
CREATE TABLE IF NOT EXISTS cache (
	key          TEXT PRIMARY KEY,
	created      INTEGER NOT NULL,          -- unix seconds
	status       INTEGER NOT NULL,
	content_type TEXT    NOT NULL,
	body         BLOB    NOT NULL,
	cost_usd     REAL    NOT NULL,
	model        TEXT    NOT NULL
);
```
and add:
```go
// CachedResponse is one stored upstream response, replayable byte-for-byte.
type CachedResponse struct {
	Status      int
	ContentType string
	Body        []byte
	CostUSD     float64
	Model       string
}

func (s *Store) CachePut(key string, c CachedResponse) error {
	_, err := s.db.Exec(
		`INSERT INTO cache (key, created, status, content_type, body, cost_usd, model)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET created=excluded.created, status=excluded.status,
		   content_type=excluded.content_type, body=excluded.body,
		   cost_usd=excluded.cost_usd, model=excluded.model`,
		key, time.Now().Unix(), c.Status, c.ContentType, c.Body, c.CostUSD, c.Model,
	)
	return err
}

// CacheGet returns the entry if it exists and is younger than maxAge.
// Stale entries are deleted lazily.
func (s *Store) CacheGet(key string, maxAge time.Duration) (*CachedResponse, bool, error) {
	var c CachedResponse
	var created int64
	err := s.db.QueryRow(
		`SELECT created, status, content_type, body, cost_usd, model FROM cache WHERE key = ?`, key,
	).Scan(&created, &c.Status, &c.ContentType, &c.Body, &c.CostUSD, &c.Model)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if time.Since(time.Unix(created, 0)) > maxAge {
		_, _ = s.db.Exec(`DELETE FROM cache WHERE key = ?`, key)
		return nil, false, nil
	}
	return &c, true, nil
}
```
(`database/sql` is already imported as `sql`.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/store/ ./internal/control/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/store backend/internal/control
git commit -m "feat: response cache storage (sqlite table, TTL at read) + exact-match key"
```

---

### Task 8: Wire control into the proxy

**Files:**
- Modify: `backend/internal/proxy/proxy.go` (control interception), `backend/internal/store/store.go` (new columns), `backend/cmd/gateway/main.go` (config flag), `backend/internal/proxy/proxy_test.go` (update `proxy.New` call sites)
- Create: `backend/internal/proxy/messages.go`
- Test: `backend/internal/proxy/control_test.go`

**Interfaces:**
- Consumes: `control.Watcher/EvaluateBudget/PeriodStarts/Route/CacheKey`, `store.SpendSince/CacheGet/CachePut` from Tasks 4–7.
- Produces: `proxy.New(upstream string, st *store.Store, pr pricing.Pricing, ctl *control.Watcher) (*Gateway, error)` — **signature change**; `store.Record` gains `RoutedFrom string; CacheHit bool; SavedUSD float64`.

- [ ] **Step 1: Extend store schema (migration for existing DBs)**

In `store.go`: add fields to `Record` (`RoutedFrom string`, `CacheHit bool`, `SavedUSD float64`); extend `Insert` to write them (`cache_hit` via `boolToInt(r.CacheHit)`); add the helper here (Task 10's `InsertSynced` reuses it):
```go
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
```
In `Open`, after the `schema` exec, run best-effort migrations:
```go
// Migrate pre-existing DBs; "duplicate column" errors are expected and ignored.
for _, ddl := range []string{
	`ALTER TABLE calls ADD COLUMN routed_from TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE calls ADD COLUMN cache_hit INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE calls ADD COLUMN saved_usd REAL NOT NULL DEFAULT 0`,
} {
	_, _ = db.Exec(ddl)
}
```
and append the three columns to the `CREATE TABLE calls` statement in `schema` (fresh DBs get them directly; ALTERs then no-op).

- [ ] **Step 2: Write failing integration tests**

`backend/internal/proxy/control_test.go`:
```go
package proxy_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/proxy"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

func watcherFor(t *testing.T, yaml string) *control.Watcher {
	t.Helper()
	p := filepath.Join(t.TempDir(), "gateway.yaml")
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return control.NewWatcher(p)
}

// upstreamJSON returns a counting upstream that answers a fixed JSON message
// and reports the model it received in the request body.
func upstreamJSON(t *testing.T, gotModel *atomic.Value, hits *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		var req struct {
			Model string `json:"model"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		gotModel.Store(req.Model)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"msg_1","model":%q,"usage":{"input_tokens":100,"output_tokens":50}}`, req.Model)
	}))
}

func gatewayFor(t *testing.T, upstreamURL string, st *store.Store, ctl *control.Watcher) *httptest.Server {
	t.Helper()
	g, err := proxy.New(upstreamURL, st, testPricing(t), ctl)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	return srv
}

func postMessages(t *testing.T, url, body string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Post(url+"/v1/messages", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(b)
}

func TestBudgetHardBlocks(t *testing.T) {
	var gotModel atomic.Value
	var hits atomic.Int64
	up := upstreamJSON(t, &gotModel, &hits)
	defer up.Close()
	st := newTestStore(t)
	// seed spend over the daily hard limit
	if err := st.Insert(store.Record{TS: time.Now(), Model: "m", CostUSD: 100, Status: 200}); err != nil {
		t.Fatal(err)
	}
	ctl := watcherFor(t, "budget:\n  daily_usd: {warn: 10, hard: 50}\n")
	srv := gatewayFor(t, up.URL, st, ctl)

	resp, body := postMessages(t, srv.URL, `{"model":"claude-sonnet-5","messages":[]}`)
	if resp.StatusCode != 429 {
		t.Fatalf("status = %d want 429", resp.StatusCode)
	}
	if !strings.Contains(body, `"type":"error"`) || !strings.Contains(body, "rate_limit_error") {
		t.Fatalf("want anthropic-shaped error, got %s", body)
	}
	if hits.Load() != 0 {
		t.Fatal("upstream must not be called on hard block")
	}
}

func TestRoutingRewritesModel(t *testing.T) {
	var gotModel atomic.Value
	var hits atomic.Int64
	up := upstreamJSON(t, &gotModel, &hits)
	defer up.Close()
	st := newTestStore(t)
	ctl := watcherFor(t, "routing:\n  rules:\n    - match: \"claude-opus-*\"\n      to: \"claude-sonnet-5\"\n")
	srv := gatewayFor(t, up.URL, st, ctl)

	resp, _ := postMessages(t, srv.URL, `{"model":"claude-opus-4-8","max_tokens":16,"messages":[]}`)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if gotModel.Load() != "claude-sonnet-5" {
		t.Fatalf("upstream saw model %v, want rewrite", gotModel.Load())
	}
	rows := waitForStats(t, st, 1)
	if rows[0].Model != "claude-sonnet-5" {
		t.Fatalf("logged model = %s (cost must follow the actual model)", rows[0].Model)
	}
}

func TestBlockedModelRejected(t *testing.T) {
	var gotModel atomic.Value
	var hits atomic.Int64
	up := upstreamJSON(t, &gotModel, &hits)
	defer up.Close()
	ctl := watcherFor(t, "routing:\n  block: [\"claude-fable-*\"]\n")
	srv := gatewayFor(t, up.URL, newTestStore(t), ctl)

	resp, body := postMessages(t, srv.URL, `{"model":"claude-fable-5","messages":[]}`)
	if resp.StatusCode != 400 || !strings.Contains(body, "invalid_request_error") {
		t.Fatalf("want 400 invalid_request_error, got %d %s", resp.StatusCode, body)
	}
	if hits.Load() != 0 {
		t.Fatal("upstream must not be called for blocked model")
	}
}

func TestCacheHitServesStoredResponse(t *testing.T) {
	var gotModel atomic.Value
	var hits atomic.Int64
	up := upstreamJSON(t, &gotModel, &hits)
	defer up.Close()
	st := newTestStore(t)
	ctl := watcherFor(t, "cache:\n  enabled: true\n  ttl: 1h\n")
	srv := gatewayFor(t, up.URL, st, ctl)

	req := `{"model":"claude-sonnet-5","messages":[{"role":"user","content":"hi"}]}`
	_, first := postMessages(t, srv.URL, req)
	waitForStats(t, st, 1) // ensure finalize (and CachePut) ran
	_, second := postMessages(t, srv.URL, req)
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d want 1", hits.Load())
	}
	if first != second {
		t.Fatalf("cached response differs:\n%s\n%s", first, second)
	}
	// second row must be a cache hit with savings and zero cost
	deadline := time.Now().Add(3 * time.Second)
	for {
		hitsN, saved, err := st.CacheSavings()
		if err != nil {
			t.Fatal(err)
		}
		if hitsN == 1 && saved > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("cache savings not recorded: hits=%d saved=%v", hitsN, saved)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestNonMessagesPathsBypassControl(t *testing.T) {
	var hits atomic.Int64
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(200)
	}))
	defer up.Close()
	// hard block active, but GET /v1/models must pass through
	ctl := watcherFor(t, "budget:\n  daily_usd: {hard: 0.000001}\n")
	st := newTestStore(t)
	_ = st.Insert(store.Record{TS: time.Now(), Model: "m", CostUSD: 1, Status: 200})
	srv := gatewayFor(t, up.URL, st, ctl)
	resp, err := http.Get(srv.URL + "/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || hits.Load() != 1 {
		t.Fatalf("bypass failed: %d hits=%d", resp.StatusCode, hits.Load())
	}
}
```

This test file needs `store.CacheSavings` — add it in Step 3 (also used by Task 9):
```go
// CacheSavings returns how many cache hits were served and the total USD saved.
func (s *Store) CacheSavings() (hits int64, saved float64, err error) {
	err = s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(saved_usd), 0) FROM calls WHERE cache_hit = 1`,
	).Scan(&hits, &saved)
	return
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd backend && go test ./internal/proxy/ -v`
Expected: FAIL — compile error (`proxy.New` arity, `CacheSavings` undefined).

- [ ] **Step 4: Implement**

4a. `store.go`: `Record` fields, `Insert` columns, migrations (Step 1), `CacheSavings`.

4b. `proxy.go`: `Gateway` gains `ctl *control.Watcher`; `New(upstream string, st *store.Store, pr pricing.Pricing, ctl *control.Watcher)`. Replace `startKey` context plumbing with one struct:
```go
type reqInfo struct {
	start      time.Time
	routedFrom string // original model if rewritten, else ""
	cacheKey   string // non-empty → capture & store the response on success
}
const infoKey ctxKey = 0
```
`ServeHTTP` becomes a dispatcher:
```go
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/v1/messages" {
		g.handleMessages(w, r)
		return
	}
	info := &reqInfo{start: time.Now()}
	g.proxy.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), infoKey, info)))
}
```
`modifyResponse` reads `info, _ := resp.Request.Context().Value(infoKey).(*reqInfo)`; when `info.cacheKey != ""` and `resp.StatusCode == 200`, sets `tap.capture = &bytes.Buffer{}`; `onClose` calls `g.finalize(p, status, contentType, info, tap.capture)`.

`finalize` extended (still recover-guarded, still fail-open):
```go
func (g *Gateway) finalize(p *parser, status int, contentType string, info *reqInfo, captured *bytes.Buffer) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("finalize panic: %v", r)
		}
	}()
	p.finalize()
	model := p.model
	if model == "" {
		model = "unknown"
	}
	cost := g.pricing.Cost(model, p.u.Input, p.u.Output, p.u.CacheRead, p.u.CacheWrite)
	// CachePut BEFORE Insert: once the calls row is visible, the cache entry
	// must already exist (tests use the row as the "finalize done" signal).
	if captured != nil && status == 200 {
		err := g.store.CachePut(info.cacheKey, store.CachedResponse{
			Status: status, ContentType: contentType,
			Body: captured.Bytes(), CostUSD: cost, Model: model,
		})
		if err != nil {
			log.Printf("cache put failed: %v", err)
		}
	}
	rec := store.Record{
		TS: time.Now(), Model: model, Usage: p.u, CostUSD: cost,
		LatencyMS: time.Since(info.start).Milliseconds(), Status: status,
		RoutedFrom: info.routedFrom,
	}
	if err := g.store.Insert(rec); err != nil {
		log.Printf("store insert failed: %v", err)
	}
}
```
`usageTap` gains `capture *bytes.Buffer`; in `Read`, after `parser.feed`: `if t.capture != nil { t.capture.Write(p[:n]) }`.

4c. `backend/internal/proxy/messages.go` (new):
```go
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

// handleMessages runs the control plane for POST /v1/messages: budget check,
// model routing, cache lookup — then proxies. Every internal failure falls
// through to plain proxying (fail-open).
func (g *Gateway) handleMessages(w http.ResponseWriter, r *http.Request) {
	cfg := g.ctl.Current()
	info := &reqInfo{start: time.Now()}

	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
	r.Body.Close()
	if err != nil {
		log.Printf("read body failed: %v", err)
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "gateway: unreadable request body")
		return
	}

	// Budget.
	dayS, weekS, monthS := control.PeriodStarts(time.Now())
	day, err1 := g.store.SpendSince(dayS)
	week, err2 := g.store.SpendSince(weekS)
	month, err3 := g.store.SpendSince(monthS)
	if err1 == nil && err2 == nil && err3 == nil {
		switch st := control.EvaluateBudget(cfg.Budget, day, week, month); st.Verdict {
		case control.VerdictBlock:
			log.Printf("budget block: %s", st.Reason)
			writeAnthropicError(w, http.StatusTooManyRequests, "rate_limit_error", "ai-gateway: "+st.Reason)
			return
		case control.VerdictWarn:
			log.Printf("budget warn: %s", st.Reason)
		}
	} else {
		log.Printf("budget check skipped (store error): %v %v %v", err1, err2, err3)
	}

	// Routing: rewrite the model field, preserving all other fields verbatim.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err == nil {
		var model string
		_ = json.Unmarshal(fields["model"], &model)
		if model != "" {
			to, blocked := cfg.Routing.Route(model)
			if blocked {
				log.Printf("model blocked: %s", model)
				writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error",
					"ai-gateway: model "+model+" is blocked by routing config")
				return
			}
			if to != model {
				raw, err := json.Marshal(to)
				if err == nil {
					fields["model"] = raw
					if nb, err := json.Marshal(fields); err == nil {
						body = nb
						info.routedFrom = model
						log.Printf("routed model %s -> %s", model, to)
					}
				}
			}
		}
	}

	// Cache.
	if cfg.Cache.Enabled && cfg.Cache.TTL > 0 {
		key := control.CacheKey(body)
		if hit, ok, err := g.store.CacheGet(key, cfg.Cache.TTL); err == nil && ok {
			g.serveCached(w, hit, info)
			return
		} else if err != nil {
			log.Printf("cache get failed: %v", err)
		}
		info.cacheKey = key // miss: capture the response for next time
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Del("Content-Length")
	g.proxy.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), infoKey, info)))
}

// serveCached replays a stored response and logs a zero-cost row with savings.
// SSE bodies replay as one write — fine for a cache hit.
func (g *Gateway) serveCached(w http.ResponseWriter, hit *store.CachedResponse, info *reqInfo) {
	w.Header().Set("Content-Type", hit.ContentType)
	w.WriteHeader(hit.Status)
	_, _ = w.Write(hit.Body)
	rec := store.Record{
		TS: time.Now(), Model: hit.Model, CostUSD: 0,
		LatencyMS: time.Since(info.start).Milliseconds(), Status: hit.Status,
		CacheHit: true, SavedUSD: hit.CostUSD,
	}
	if err := g.store.Insert(rec); err != nil {
		log.Printf("store insert (cache hit) failed: %v", err)
	}
}

func writeAnthropicError(w http.ResponseWriter, status int, typ, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":  "error",
		"error": map[string]string{"type": typ, "message": msg},
	})
}
```
Content-Type for `modifyResponse`: capture `resp.Header.Get("Content-Type")` into a local before building `onClose`.

4d. Update call sites: `proxy_test.go` tests pass `control.NewWatcher(filepath.Join(t.TempDir(), "absent.yaml"))` (zero config); `cmd/gateway/main.go` adds `config := fs.String("config", configPath("gateway.yaml"), "path to gateway.yaml")` and `proxy.New(*upstream, st, pr, control.NewWatcher(*config))`. Also commit a sample: `gateway.yaml.example` at repo root with the Task 4 sample YAML content.

- [ ] **Step 5: Run all tests**

Run: `cd backend && go test ./... -v`
Expected: PASS — including Task 3 parity tests (now with 4-arg `New`).

- [ ] **Step 6: Commit**

```bash
git add backend gateway.yaml.example
git commit -m "feat: wire budget/routing/cache control plane into the proxy"
```

---

### Task 9: Proto schema + Connect StatsService

**Files:**
- Create: `proto/gateway/v1/stats.proto`, `buf.yaml`, `buf.gen.yaml`, `backend/gen/**` (generated, committed), `backend/internal/api/api.go`, `backend/internal/store/queries.go`
- Test: `backend/internal/api/api_test.go`

**Interfaces:**
- Consumes: `store` queries (new ones below), `control.BudgetConfig/PeriodStarts`.
- Produces:
```go
// api
func New(st *store.Store, limits func() control.BudgetConfig, token string) http.Handler
// store (queries.go)
type CostEvent struct{ TS int64; CostUSD float64 }
func (s *Store) CostEvents(cutoff time.Time) ([]CostEvent, error)
type Call struct{ ID int64; Record }
func (s *Store) RecentCalls(limit int, beforeID int64) ([]Call, error)
func (s *Store) TotalCalls() (int64, error)
```
- Generated Go packages: `backend/gen/gateway/v1` (`gatewayv1`) and `backend/gen/gateway/v1/gatewayv1connect`.

- [ ] **Step 1: Install buf and write the schema**

```bash
which buf || brew install bufbuild/buf/buf
mkdir -p proto/gateway/v1
```

`proto/gateway/v1/stats.proto`:
```proto
syntax = "proto3";

package gateway.v1;

option go_package = "github.com/hieudang-lxp/ai-gateway/backend/gen/gateway/v1;gatewayv1";

service StatsService {
  rpc Overview(OverviewRequest) returns (OverviewResponse);
  rpc SpendSeries(SpendSeriesRequest) returns (SpendSeriesResponse);
  rpc ModelBreakdown(ModelBreakdownRequest) returns (ModelBreakdownResponse);
  rpc RecentCalls(RecentCallsRequest) returns (RecentCallsResponse);
}

message OverviewRequest {}

message BudgetWindow {
  double spent_usd = 1;
  double warn_usd = 2; // 0 = disabled
  double hard_usd = 3; // 0 = disabled
}

message OverviewResponse {
  BudgetWindow today = 1;
  BudgetWindow week = 2;
  BudgetWindow month = 3;
  double cache_saved_usd = 4;
  int64 cache_hits = 5;
  int64 total_calls = 6;
}

message SpendSeriesRequest {
  int32 days = 1; // <=0 -> 30
}

message SpendPoint {
  string date = 1; // YYYY-MM-DD in Asia/Ho_Chi_Minh
  double cost_usd = 2;
  int64 calls = 3;
}

message SpendSeriesResponse {
  repeated SpendPoint points = 1;
}

message ModelBreakdownRequest {
  int32 days = 1; // <=0 -> 30
}

message ModelRow {
  string model = 1;
  int64 calls = 2;
  int64 input_tokens = 3;
  int64 output_tokens = 4;
  int64 cache_read_tokens = 5;
  int64 cache_write_tokens = 6;
  double cost_usd = 7;
}

message ModelBreakdownResponse {
  repeated ModelRow rows = 1;
}

message RecentCallsRequest {
  int32 limit = 1;     // <=0 -> 50, max 200
  int64 before_id = 2; // pagination cursor; 0 = latest
}

message Call {
  int64 id = 1;
  int64 ts_unix = 2;
  string model = 3;
  string routed_from = 4;
  int64 input_tokens = 5;
  int64 output_tokens = 6;
  double cost_usd = 7;
  int64 latency_ms = 8;
  int32 status = 9;
  bool cache_hit = 10;
  double saved_usd = 11;
}

message RecentCallsResponse {
  repeated Call calls = 1;
}
```

`buf.yaml` (repo root):
```yaml
version: v2
modules:
  - path: proto
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

`buf.gen.yaml` (repo root):
```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: backend/gen
    opt: paths=source_relative
  - remote: buf.build/connectrpc/go
    out: backend/gen
    opt: paths=source_relative
```

- [ ] **Step 2: Generate and take deps**

```bash
buf lint && buf generate
cd backend && go get connectrpc.com/connect google.golang.org/protobuf && go mod tidy && go build ./...
```
Expected: `backend/gen/gateway/v1/stats.pb.go` and `.../gatewayv1connect/stats.connect.go` exist; everything builds. Generated code is **committed** (Render's Docker build must not need buf).

- [ ] **Step 3: Write failing API tests**

`backend/internal/api/api_test.go`:
```go
package api_test

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/api"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
	gatewayv1 "github.com/hieudang-lxp/ai-gateway/backend/gen/gateway/v1"
	"github.com/hieudang-lxp/ai-gateway/backend/gen/gateway/v1/gatewayv1connect"
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
		// -1min not -1h: near midnight Asia/Ho_Chi_Minh a -1h row would fall
		// into yesterday and flake the "today spent" assertion.
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
	if m.Today.SpentUsd != 3.0 { // 1.0 + 2.0 today (cache hit row costs 0)
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
	if total != 7.0 { // 1+2+4 within 7 days
		t.Fatalf("series total = %v want 7.0", total)
	}
	b, err := c.ModelBreakdown(context.Background(), connect.NewRequest(&gatewayv1.ModelBreakdownRequest{Days: 7}))
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Msg.Rows) != 2 { // sonnet + opus
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

	// no token → Unauthenticated
	c := gatewayv1connect.NewStatsServiceClient(srv.Client(), srv.URL)
	_, err := c.Overview(context.Background(), connect.NewRequest(&gatewayv1.OverviewRequest{}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}

	// correct token → OK
	req := connect.NewRequest(&gatewayv1.OverviewRequest{})
	req.Header().Set("Authorization", "Bearer s3cret")
	if _, err := c.Overview(context.Background(), req); err != nil {
		t.Fatalf("with token: %v", err)
	}
}
```
Note: per-request auth headers are set via `req.Header().Set(...)`, not client options.

- [ ] **Step 4: Run tests to verify they fail**

Run: `cd backend && go test ./internal/api/ -v`
Expected: FAIL — `api.New` undefined (store queries too).

- [ ] **Step 5: Implement store queries**

`backend/internal/store/queries.go`:
```go
package store

import "time"

// CostEvent is a (timestamp, cost) pair for time-series bucketing done in Go
// (keeps timezone logic out of SQL).
type CostEvent struct {
	TS      int64
	CostUSD float64
}

func (s *Store) CostEvents(cutoff time.Time) ([]CostEvent, error) {
	rows, err := s.db.Query(
		`SELECT ts, est_cost_usd FROM calls WHERE ts >= ? ORDER BY ts ASC`, cutoff.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CostEvent
	for rows.Next() {
		var e CostEvent
		if err := rows.Scan(&e.TS, &e.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Call is a Record with its row id, for pagination and sync.
type Call struct {
	ID int64
	Record
}

const callCols = `id, ts, model, routed_from, input_tokens, output_tokens,
	cache_read_tokens, cache_write_tokens, est_cost_usd, latency_ms, status, cache_hit, saved_usd`

func (s *Store) scanCalls(query string, args ...any) ([]Call, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Call
	for rows.Next() {
		var c Call
		var ts int64
		var cacheHit int
		if err := rows.Scan(&c.ID, &ts, &c.Model, &c.RoutedFrom,
			&c.Usage.Input, &c.Usage.Output, &c.Usage.CacheRead, &c.Usage.CacheWrite,
			&c.CostUSD, &c.LatencyMS, &c.Status, &cacheHit, &c.SavedUSD); err != nil {
			return nil, err
		}
		c.TS = time.Unix(ts, 0)
		c.CacheHit = cacheHit == 1
		out = append(out, c)
	}
	return out, rows.Err()
}

// RecentCalls pages newest-first. beforeID <= 0 starts from the latest row.
func (s *Store) RecentCalls(limit int, beforeID int64) ([]Call, error) {
	if beforeID > 0 {
		return s.scanCalls(`SELECT `+callCols+` FROM calls WHERE id < ? ORDER BY id DESC LIMIT ?`, beforeID, limit)
	}
	return s.scanCalls(`SELECT `+callCols+` FROM calls ORDER BY id DESC LIMIT ?`, limit)
}

// CallsAfter returns rows with id > afterID, oldest first (sync batches).
func (s *Store) CallsAfter(afterID int64, limit int) ([]Call, error) {
	return s.scanCalls(`SELECT `+callCols+` FROM calls WHERE id > ? ORDER BY id ASC LIMIT ?`, afterID, limit)
}

func (s *Store) TotalCalls() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM calls`).Scan(&n)
	return n, err
}
```

- [ ] **Step 6: Implement the Connect handler**

`backend/internal/api/api.go`:
```go
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
	calls, err := s.st.RecentCalls(limit, req.Msg.BeforeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &gatewayv1.RecentCallsResponse{}
	for _, c := range calls {
		resp.Calls = append(resp.Calls, &gatewayv1.Call{
			Id: c.ID, TsUnix: c.TS.Unix(), Model: c.Model, RoutedFrom: c.RoutedFrom,
			InputTokens: c.Usage.Input, OutputTokens: c.Usage.Output,
			CostUsd: c.CostUSD, LatencyMs: c.LatencyMS, Status: int32(c.Status),
			CacheHit: c.CacheHit, SavedUsd: c.SavedUSD,
		})
	}
	return connect.NewResponse(resp), nil
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd backend && go test ./internal/api/ -v && go test ./...`
Expected: PASS all.

- [ ] **Step 8: Commit**

```bash
git add proto buf.yaml buf.gen.yaml backend
git commit -m "feat: Connect RPC StatsService (proto-first via buf) with bearer auth"
```

---

### Task 10: Turso sync

**Files:**
- Create: `backend/internal/sync/sync.go`
- Modify: `backend/internal/store/store.go` (`OpenDSN`, sync schema/watermark/snapshot helpers)
- Test: `backend/internal/sync/sync_test.go`

**Interfaces:**
- Consumes: `store.CallsAfter` (Task 9).
- Produces:
```go
// store
func OpenDSN(driver, dsn string) (*Store, error)       // Open(path) stays; libsql remotes use this
func (s *Store) EnsureSyncSchema() error               // remote: calls(+local_id UNIQUE), budget_snapshot
func (s *Store) InsertSynced(c Call) error             // INSERT OR IGNORE keyed on local_id
func (s *Store) LastSyncedID() (int64, error)          // local watermark (sync_state table)
func (s *Store) SetLastSyncedID(id int64) error
func (s *Store) WriteBudgetSnapshot(dw, dh, ww, wh, mw, mh float64) error
func (s *Store) ReadBudgetSnapshot() (dw, dh, ww, wh, mw, mh float64, ok bool, err error)
// sync
type Syncer struct{ Local, Remote *store.Store; Limits func() control.BudgetConfig; Interval time.Duration; Batch int }
func (s *Syncer) SyncOnce() (pushed int, err error)
func (s *Syncer) Run(ctx context.Context)              // ticker loop; logs errors, never crashes
```

- [ ] **Step 1: Write failing tests**

`backend/internal/sync/sync_test.go`:
```go
package sync_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
	gwsync "github.com/hieudang-lxp/ai-gateway/backend/internal/sync"
)

// The "remote" is a plain local SQLite store — same driver family as Turso
// (libsql speaks the sqlite dialect), so SQL behavior matches.
func pair(t *testing.T) (*store.Store, *store.Store) {
	t.Helper()
	local, err := store.Open(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatal(err)
	}
	remote, err := store.Open(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { local.Close(); remote.Close() })
	if err := remote.EnsureSyncSchema(); err != nil {
		t.Fatal(err)
	}
	return local, remote
}

func seed(t *testing.T, st *store.Store, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := st.Insert(store.Record{TS: time.Now(), Model: "m", CostUSD: 1, Status: 200}); err != nil {
			t.Fatal(err)
		}
	}
}

func limits() control.BudgetConfig {
	return control.BudgetConfig{Daily: control.Limit{Warn: 10, Hard: 20}}
}

func TestSyncOncePushesAndAdvancesWatermark(t *testing.T) {
	local, remote := pair(t)
	seed(t, local, 3)
	s := &gwsync.Syncer{Local: local, Remote: remote, Limits: limits, Batch: 500}

	pushed, err := s.SyncOnce()
	if err != nil || pushed != 3 {
		t.Fatalf("pushed=%d err=%v", pushed, err)
	}
	if n, _ := remote.TotalCalls(); n != 3 {
		t.Fatalf("remote rows = %d", n)
	}
	if id, _ := local.LastSyncedID(); id != 3 {
		t.Fatalf("watermark = %d", id)
	}
	// budget snapshot written
	dw, dh, _, _, _, _, ok, err := remote.ReadBudgetSnapshot()
	if err != nil || !ok || dw != 10 || dh != 20 {
		t.Fatalf("snapshot: %v %v %v %v", dw, dh, ok, err)
	}
}

func TestSyncOnceIsIdempotent(t *testing.T) {
	local, remote := pair(t)
	seed(t, local, 2)
	s := &gwsync.Syncer{Local: local, Remote: remote, Limits: limits, Batch: 500}
	if _, err := s.SyncOnce(); err != nil {
		t.Fatal(err)
	}
	// re-run with a reset watermark: INSERT OR IGNORE must dedupe on local_id
	if err := local.SetLastSyncedID(0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncOnce(); err != nil {
		t.Fatal(err)
	}
	if n, _ := remote.TotalCalls(); n != 2 {
		t.Fatalf("remote rows = %d want 2 (deduped)", n)
	}
}

func TestSyncOnlyNewRows(t *testing.T) {
	local, remote := pair(t)
	seed(t, local, 2)
	s := &gwsync.Syncer{Local: local, Remote: remote, Limits: limits, Batch: 500}
	_, _ = s.SyncOnce()
	seed(t, local, 1)
	pushed, err := s.SyncOnce()
	if err != nil || pushed != 1 {
		t.Fatalf("pushed=%d err=%v", pushed, err)
	}
	if n, _ := remote.TotalCalls(); n != 3 {
		t.Fatalf("remote rows = %d", n)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/sync/ -v`
Expected: FAIL — package/methods undefined.

- [ ] **Step 3: Implement store side**

In `store.go`, refactor `Open` to call the new generalized opener (local behavior unchanged — WAL pragmas only apply to the sqlite driver):
```go
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	return OpenDSN("sqlite", dsn)
}

// OpenDSN opens any database/sql driver speaking the sqlite dialect (local
// "sqlite", remote "libsql" for Turso) and ensures the base schema.
func OpenDSN(driver, dsn string) (*Store, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	for _, ddl := range []string{
		`ALTER TABLE calls ADD COLUMN routed_from TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE calls ADD COLUMN cache_hit INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE calls ADD COLUMN saved_usd REAL NOT NULL DEFAULT 0`,
	} {
		_, _ = db.Exec(ddl)
	}
	return &Store{db: db}, nil
}
```
Add sync helpers:
```go
// EnsureSyncSchema prepares a REMOTE store: dedupe column on calls plus the
// budget snapshot table the cloud API reads limits from.
func (s *Store) EnsureSyncSchema() error {
	_, _ = s.db.Exec(`ALTER TABLE calls ADD COLUMN local_id INTEGER`)
	if _, err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_calls_local_id ON calls(local_id)`); err != nil {
		return err
	}
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS budget_snapshot (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		daily_warn REAL NOT NULL, daily_hard REAL NOT NULL,
		weekly_warn REAL NOT NULL, weekly_hard REAL NOT NULL,
		monthly_warn REAL NOT NULL, monthly_hard REAL NOT NULL,
		updated_at INTEGER NOT NULL
	)`)
	return err
}

// InsertSynced writes one local row into a remote store, deduped on local_id.
func (s *Store) InsertSynced(c Call) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO calls
		 (local_id, ts, model, routed_from, input_tokens, output_tokens,
		  cache_read_tokens, cache_write_tokens, est_cost_usd, latency_ms, status, cache_hit, saved_usd)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.TS.Unix(), c.Model, c.RoutedFrom,
		c.Usage.Input, c.Usage.Output, c.Usage.CacheRead, c.Usage.CacheWrite,
		c.CostUSD, c.LatencyMS, c.Status, boolToInt(c.CacheHit), c.SavedUSD,
	)
	return err
}

func (s *Store) LastSyncedID() (int64, error) {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS sync_state (id INTEGER PRIMARY KEY CHECK (id = 1), last_synced INTEGER NOT NULL)`); err != nil {
		return 0, err
	}
	var v int64
	err := s.db.QueryRow(`SELECT last_synced FROM sync_state WHERE id = 1`).Scan(&v)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return v, err
}

func (s *Store) SetLastSyncedID(id int64) error {
	_, err := s.db.Exec(
		`INSERT INTO sync_state (id, last_synced) VALUES (1, ?)
		 ON CONFLICT(id) DO UPDATE SET last_synced = excluded.last_synced`, id)
	return err
}

func (s *Store) WriteBudgetSnapshot(dw, dh, ww, wh, mw, mh float64) error {
	_, err := s.db.Exec(
		`INSERT INTO budget_snapshot (id, daily_warn, daily_hard, weekly_warn, weekly_hard, monthly_warn, monthly_hard, updated_at)
		 VALUES (1, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET daily_warn=excluded.daily_warn, daily_hard=excluded.daily_hard,
		   weekly_warn=excluded.weekly_warn, weekly_hard=excluded.weekly_hard,
		   monthly_warn=excluded.monthly_warn, monthly_hard=excluded.monthly_hard,
		   updated_at=excluded.updated_at`,
		dw, dh, ww, wh, mw, mh, time.Now().Unix())
	return err
}

func (s *Store) ReadBudgetSnapshot() (dw, dh, ww, wh, mw, mh float64, ok bool, err error) {
	err = s.db.QueryRow(
		`SELECT daily_warn, daily_hard, weekly_warn, weekly_hard, monthly_warn, monthly_hard FROM budget_snapshot WHERE id = 1`,
	).Scan(&dw, &dh, &ww, &wh, &mw, &mh)
	if err == sql.ErrNoRows {
		return 0, 0, 0, 0, 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, 0, 0, 0, 0, false, err
	}
	return dw, dh, ww, wh, mw, mh, true, nil
}
```
Note: `sync_state` creation is lazy inside `LastSyncedID` so remote stores never grow it.

- [ ] **Step 4: Implement the syncer**

`backend/internal/sync/sync.go`:
```go
// Package sync pushes local call rows to the Turso replica. Strictly
// fail-open: any error is logged and retried next tick; the proxy never
// depends on it.
package sync

import (
	"context"
	"log"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

type Syncer struct {
	Local    *store.Store
	Remote   *store.Store
	Limits   func() control.BudgetConfig
	Interval time.Duration // default 60s
	Batch    int           // default 500
}

// SyncOnce pushes at most Batch rows past the watermark, then refreshes the
// budget snapshot. Returns how many rows were pushed.
func (s *Syncer) SyncOnce() (int, error) {
	batch := s.Batch
	if batch <= 0 {
		batch = 500
	}
	last, err := s.Local.LastSyncedID()
	if err != nil {
		return 0, err
	}
	calls, err := s.Local.CallsAfter(last, batch)
	if err != nil {
		return 0, err
	}
	for _, c := range calls {
		if err := s.Remote.InsertSynced(c); err != nil {
			return 0, err
		}
		last = c.ID
	}
	if len(calls) > 0 {
		if err := s.Local.SetLastSyncedID(last); err != nil {
			return 0, err
		}
	}
	cfg := s.Limits()
	if err := s.Remote.WriteBudgetSnapshot(
		cfg.Daily.Warn, cfg.Daily.Hard,
		cfg.Weekly.Warn, cfg.Weekly.Hard,
		cfg.Monthly.Warn, cfg.Monthly.Hard); err != nil {
		return len(calls), err
	}
	return len(calls), nil
}

// Run loops until ctx is done. Errors are logged, never fatal.
func (s *Syncer) Run(ctx context.Context) {
	interval := s.Interval
	if interval <= 0 {
		interval = 60 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := s.SyncOnce(); err != nil {
				log.Printf("sync: %v", err)
			} else if n > 0 {
				log.Printf("sync: pushed %d rows", n)
			}
		}
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd backend && go test ./internal/sync/ ./internal/store/ -v && go test ./...`
Expected: PASS all.

- [ ] **Step 6: Commit**

```bash
git add backend
git commit -m "feat: watermarked batch sync to remote store + budget snapshot"
```

---

### Task 11: Wire roles — serve mounts API + sync; new `gateway api` command; Docker/Render

**Files:**
- Modify: `backend/cmd/gateway/main.go`
- Create: `Dockerfile`, `render.yaml`
- Test: `backend/cmd/gateway/main_test.go` (smoke: api role over a local db)

**Interfaces:**
- Consumes: everything above. `gateway api` env: `PORT` (Render sets it), `TURSO_DATABASE_URL`, `TURSO_AUTH_TOKEN`, `DASHBOARD_TOKEN`, `CORS_ORIGIN`. `gateway serve` gains flags `-turso-url` / `-turso-token` (env fallbacks same names) enabling sync, and mounts the Connect API at the same listener (no auth locally).

- [ ] **Step 1: Add deps**

```bash
cd backend
go get github.com/tursodatabase/libsql-client-go/libsql github.com/rs/cors golang.org/x/net/http2/h2c golang.org/x/net/http2
go mod tidy
```
NOTE: verify the libsql import path against https://github.com/tursodatabase/libsql-client-go at implementation time; register with `import _ ".../libsql"` and `sql.Open("libsql", "libsql://<db>.turso.io?authToken=<token>")`. `CGO_ENABLED=0 go build ./...` must still pass.

- [ ] **Step 2: Extend main.go**

In `runServe`: after building `mux`, mount the API:
```go
mux.Handle("/rpc/", http.StripPrefix("/rpc", api.New(st, func() control.BudgetConfig {
	return ctl.Current().Budget
}, "")))
```
(`ctl` is the `control.NewWatcher(*config)` from Task 8; local API is auth-free.)
Add sync startup:
```go
tursoURL := fs.String("turso-url", os.Getenv("TURSO_DATABASE_URL"), "libsql URL; empty disables sync")
tursoToken := fs.String("turso-token", os.Getenv("TURSO_AUTH_TOKEN"), "Turso auth token")
// after store/gateway construction:
if *tursoURL != "" {
	remote, err := store.OpenDSN("libsql", *tursoURL+"?authToken="+*tursoToken)
	if err != nil {
		log.Printf("sync disabled (remote open failed): %v", err)
	} else if err := remote.EnsureSyncSchema(); err != nil {
		log.Printf("sync disabled (remote schema): %v", err)
	} else {
		syncer := &gwsync.Syncer{Local: st, Remote: remote,
			Limits: func() control.BudgetConfig { return ctl.Current().Budget }}
		go syncer.Run(context.Background())
		log.Printf("sync: pushing to %s every 60s", *tursoURL)
	}
} else {
	log.Printf("sync: disabled (no -turso-url / TURSO_DATABASE_URL)")
}
```

New `runAPI` + dispatch (`os.Args[1] == "api"`):
```go
func runAPI(args []string) {
	fs := flag.NewFlagSet("api", flag.ExitOnError)
	dbFlag := fs.String("db", "", "local sqlite path (dev/tests) — overrides Turso")
	fs.Parse(args)

	var st *store.Store
	var err error
	if *dbFlag != "" {
		st, err = store.Open(*dbFlag)
	} else {
		url := os.Getenv("TURSO_DATABASE_URL")
		if url == "" {
			log.Fatal("api: set TURSO_DATABASE_URL (or -db for local dev)")
		}
		st, err = store.OpenDSN("libsql", url+"?authToken="+os.Getenv("TURSO_AUTH_TOKEN"))
	}
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	limits := func() control.BudgetConfig {
		dw, dh, ww, wh, mw, mh, ok, err := st.ReadBudgetSnapshot()
		if err != nil || !ok {
			return control.BudgetConfig{}
		}
		return control.BudgetConfig{
			Daily:   control.Limit{Warn: dw, Hard: dh},
			Weekly:  control.Limit{Warn: ww, Hard: wh},
			Monthly: control.Limit{Warn: mw, Hard: mh},
		}
	}

	handler := api.New(st, limits, os.Getenv("DASHBOARD_TOKEN"))

	origin := os.Getenv("CORS_ORIGIN") // e.g. https://<site>.netlify.app
	allowed := []string{"http://localhost:5173"}
	if origin != "" {
		allowed = append(allowed, origin)
	}
	c := cors.New(cors.Options{
		AllowedOrigins: allowed,
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "Connect-Protocol-Version", "Connect-Timeout-Ms"},
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8790"
	}
	addr := ":" + port
	log.Printf("gateway api listening on %s", addr)
	// h2c so plain-gRPC clients work over cleartext; browsers use Connect/JSON.
	h := h2c.NewHandler(c.Handler(handler), &http2.Server{})
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 30 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("api server: %v", err)
	}
}
```
Update `printUsage` to document `serve | api | stats`.

- [ ] **Step 3: Smoke test the api role**

`backend/cmd/gateway/main_test.go`:
```go
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
```

Run: `cd backend && go test ./... && CGO_ENABLED=0 go build ./...`
Expected: PASS; pure-Go build clean.

- [ ] **Step 4: Dockerfile + render.yaml**

`Dockerfile` (repo root):
```dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY backend/ backend/
WORKDIR /src/backend
RUN CGO_ENABLED=0 go build -o /gateway ./cmd/gateway

FROM alpine:3.20
# tzdata: budget periods are computed in Asia/Ho_Chi_Minh
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /gateway /usr/local/bin/gateway
CMD ["gateway", "api"]
```

`render.yaml` (repo root):
```yaml
services:
  - type: web
    name: ai-gateway-api
    runtime: docker
    plan: free
    dockerfilePath: ./Dockerfile
    dockerContext: .
    envVars:
      - key: TURSO_DATABASE_URL
        sync: false
      - key: TURSO_AUTH_TOKEN
        sync: false
      - key: DASHBOARD_TOKEN
        sync: false
      - key: CORS_ORIGIN
        sync: false
```

Verify the image builds: `docker build -t ai-gateway-api .` (skip if Docker isn't running locally; Render builds it anyway — note it in the commit message if skipped).

- [ ] **Step 5: Commit**

```bash
git add backend Dockerfile render.yaml
git commit -m "feat: gateway api role (Turso/CORS/h2c), serve mounts local API + sync"
```

---

### Task 12: End-to-end verify + push

**Files:** none new (verification only), README polish allowed.

- [ ] **Step 1: Full test suite + vet**

Run: `cd backend && go vet ./... && go test ./...`
Expected: all PASS.

- [ ] **Step 2: Live local e2e against a mock upstream**

Terminal A (mock upstream + gateway):
```bash
cd ~/Leapxpert/tools/ai-gateway
mkdir -p /tmp/ai-gw-e2e
cat > /tmp/ai-gw-e2e/gateway.yaml <<'EOF'
budget:
  daily_usd: {warn: 1000, hard: 2000}
routing:
  rules:
    - match: "claude-opus-*"
      to: "claude-sonnet-5"
cache:
  enabled: true
  ttl: 1h
EOF
go run ./backend/cmd/gateway -addr localhost:8788 \
  -db /tmp/ai-gw-e2e/e2e.db -config /tmp/ai-gw-e2e/gateway.yaml \
  -upstream https://api.anthropic.com
```
Terminal B (real call through the gateway — uses your normal Claude auth):
```bash
export ANTHROPIC_BASE_URL=http://localhost:8788
# fire one real small call via claude CLI or curl with your API key, then:
curl -s http://localhost:8788/_stats | head -30
curl -s http://localhost:8788/rpc/gateway.v1.StatsService/Overview \
  -H 'Content-Type: application/json' -d '{}'
```
Expected: `_stats` shows the call; the Connect endpoint returns JSON with `today.spentUsd > 0` and budget limits from the config.

- [ ] **Step 3: (Optional, needs Turso account) real Turso sync check**

```bash
# after `turso db create ai-gateway && turso db tokens create ai-gateway`
TURSO_DATABASE_URL=libsql://<db>.turso.io TURSO_AUTH_TOKEN=<token> \
  go run ./backend/cmd/gateway -db /tmp/ai-gw-e2e/e2e.db -config /tmp/ai-gw-e2e/gateway.yaml
# wait ~70s, then: turso db shell ai-gateway "select count(*) from calls;"
```
Expected: row count > 0. If no Turso account yet, defer to Plan 2 deploy phase.

- [ ] **Step 4: Push**

```bash
git push origin main
```

- [ ] **Step 5: Update memory + hand off to Plan 2 (frontend + deploys)**

Plan 2 covers: Vite React scaffold in `frontend/`, buf TS generation (`@connectrpc/connect-web`), dashboard views, USD/VND toggle (`useExchangeRate` on open.er-api.com), Netlify config, Render + Turso provisioning, final cloud e2e.
