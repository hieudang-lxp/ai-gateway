package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/memorysync"
)

func main() {
	addr := flag.String("addr", ":8794", "internal health/status address")
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "private memory-sync PostgreSQL database URL")
	db := flag.String("db", "", "explicit SQLite sender state path for standalone/testing use")
	graphitiURL := flag.String("graphiti-url", os.Getenv("GRAPHITI_URL"), "independently deployed Graphiti API URL; empty disables transcript access")
	claude := flag.String("claude-projects", "/sources/claude", "read-only Claude transcript directory")
	codex := flag.String("codex-home", "/sources/codex", "read-only Codex home with sessions and archived_sessions")
	cursor := flag.String("cursor-state", "/sources/cursor/state.vscdb", "read-only Cursor global storage database")
	interval := flag.Duration("interval", 30*time.Second, "readiness and transcript polling interval")
	flag.Parse()
	var store *memorysync.Store
	var err error
	// Disabled mode needs neither a database nor readable source mounts.
	if *graphitiURL != "" {
		switch {
		case *databaseURL != "":
			store, err = memorysync.OpenPostgres(*databaseURL)
		case *db != "":
			store, err = memorysync.OpenSQLite(*db)
		default:
			log.Fatal("DATABASE_URL is required when GRAPHITI_URL is configured (or explicitly pass -db for SQLite)")
		}
		if err != nil {
			log.Fatal("memory-sync sender database initialization failed")
		}
		defer store.Close()
	}
	worker := memorysync.NewWorker(memorysync.Config{URL: *graphitiURL, Token: os.Getenv("GRAPHITI_API_TOKEN"), GroupID: os.Getenv("GRAPHITI_GROUP_ID"), Interval: *interval}, store, memorysync.NewClaudeSource(*claude), memorysync.NewCodexSource(*codex), memorysync.NewCursorSource(*cursor))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	worker.Run(ctx)
	server := &http.Server{Addr: *addr, Handler: worker.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		server.Shutdown(shutdown)
	}()
	log.Printf("memory-sync listening on %s (enabled=%t)", *addr, *graphitiURL != "")
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
