package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	_ "github.com/tursodatabase/libsql-client-go/libsql"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/api"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/control"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/proxy"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
	gwsync "github.com/hieudang-lxp/ai-gateway/backend/internal/sync"
)

func main() {
	log.SetFlags(log.LstdFlags)
	log.SetOutput(os.Stderr)
	if len(os.Args) > 1 && os.Args[1] == "stats" {
		runStats(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "api" {
		runAPI(os.Args[2:])
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
  gateway [serve flags]    Run the proxy + local dashboard API (default)
  gateway stats [flags]    Show usage & cost totals
  gateway api [flags]      Run the cloud dashboard API (Turso-backed, for Render)

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
	config := fs.String("config", configPath("gateway.yaml"), "path to gateway.yaml")
	tursoURL := fs.String("turso-url", os.Getenv("TURSO_DATABASE_URL"), "libsql URL; empty disables sync")
	tursoToken := fs.String("turso-token", os.Getenv("TURSO_AUTH_TOKEN"), "Turso auth token")
	fs.Parse(args)

	pr := pricing.Load(*pricingPath)
	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	ctl := control.NewWatcher(*config)
	g, err := proxy.New(*upstream, st, pr, ctl)
	if err != nil {
		log.Fatalf("build gateway: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/_stats", g.HandleStats)
	mux.Handle("/rpc/", http.StripPrefix("/rpc", api.New(st, func() control.BudgetConfig {
		return ctl.Current().Budget
	}, "")))
	mux.Handle("/", g)
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 30 * time.Second}

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

	log.Printf("ai-gateway listening on http://%s -> %s", *addr, *upstream)
	log.Printf("set: export ANTHROPIC_BASE_URL=http://%s", *addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

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

func runStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	dbPath := fs.String("db", dataPath("gateway.db"), "path to the SQLite store")
	fs.Parse(args)

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	windows := []struct {
		label  string
		cutoff time.Time
	}{
		{"Today", todayStart},
		{"Last 7 days", now.AddDate(0, 0, -7)},
		{"All time", time.Unix(0, 0)},
	}

	for i, w := range windows {
		rows, err := st.StatsSince(w.cutoff)
		if err != nil {
			log.Fatalf("query stats: %v", err)
		}
		printWindow(w.label, rows)
		if i < len(windows)-1 {
			fmt.Println()
		}
	}
}

func printWindow(label string, rows []store.StatRow) {
	fmt.Printf("=== %s ===\n", label)
	if len(rows) == 0 {
		fmt.Println("  (no calls)")
		return
	}
	fmt.Printf("  %-22s %7s %11s %11s %11s %11s %10s\n",
		"model", "calls", "input", "output", "cache_rd", "cache_wr", "cost($)")
	var tc, ti, to, cr, cw int64
	var cost float64
	for _, r := range rows {
		fmt.Printf("  %-22s %7d %11d %11d %11d %11d %10.4f\n",
			trunc(r.Model, 22), r.Calls, r.Input, r.Output, r.CacheRead, r.CacheWrite, r.CostUSD)
		tc += r.Calls
		ti += r.Input
		to += r.Output
		cr += r.CacheRead
		cw += r.CacheWrite
		cost += r.CostUSD
	}
	fmt.Printf("  %-22s %7d %11d %11d %11d %11d %10.4f\n",
		"TOTAL", tc, ti, to, cr, cw, cost)
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
