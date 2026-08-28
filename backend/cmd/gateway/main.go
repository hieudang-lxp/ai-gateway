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
