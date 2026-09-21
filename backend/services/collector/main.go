package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/eventbus"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/usage"
)

func main() {
	addr := flag.String("addr", ":8789", "internal health address")
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL collector database URL")
	db := flag.String("db", "", "legacy SQLite outbox path (used only without DATABASE_URL)")
	natsURL := flag.String("nats-url", os.Getenv("NATS_URL"), "NATS URL")
	codex := flag.String("codex-home", "/sources/codex", "Codex logs")
	claude := flag.String("claude-projects", "/sources/claude", "Claude logs")
	cursor := flag.String("cursor-state", "/sources/cursor/state.vscdb", "read-only Cursor session database")
	price := flag.String("pricing", "/config/pricing.json", "Claude pricing config")
	flag.Parse()
	if *natsURL == "" {
		log.Fatal("NATS_URL is required")
	}
	var outbox *eventbus.Outbox
	var err error
	switch {
	case *databaseURL != "":
		outbox, err = eventbus.OpenPostgres(*databaseURL)
	case *db != "":
		outbox, err = eventbus.Open(*db)
	default:
		log.Fatal("DATABASE_URL is required (or explicitly pass -db for legacy SQLite)")
	}
	if err != nil {
		log.Fatal(err)
	}
	defer outbox.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	worker := usage.NewWorker(outbox, outbox.RecordStatus, pricing.Load(*price), usage.Config{CodexHome: *codex, ClaudeProjects: *claude, CursorState: *cursor})
	worker.Run(ctx)
	go outbox.Pump(ctx, *natsURL)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		n, err := outbox.Pending()
		if err != nil {
			http.Error(w, "outbox unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "pending_events": n})
	})
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		srv.Shutdown(shutdown)
	}()
	log.Printf("collector listening on %s", *addr)
	if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
