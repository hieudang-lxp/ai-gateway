package main

import (
	"context"
	"flag"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/eventbus"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/ledger"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/usage"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", ":8790", "internal API address")
	db := flag.String("database-url", os.Getenv("DATABASE_URL"), "usage PostgreSQL DSN")
	natsURL := flag.String("nats-url", os.Getenv("NATS_URL"), "NATS URL")
	cache := flag.String("price-cache", "/data/model-prices.json", "price catalog cache")
	legacy := flag.String("import-sqlite", "", "read-only legacy usage snapshot to import and exit")
	flag.Parse()
	s, err := ledger.Open(*db)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	if *legacy != "" {
		n, err := s.ImportSQLite(*legacy)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("imported %d usage identities", n)
		return
	}
	if *natsURL == "" {
		log.Fatal("NATS_URL is required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	reader := usage.NewSummary(s, s.Statuses, usage.Config{PriceCache: *cache})
	reader.RunPrices(ctx)
	go eventbus.Consume(ctx, *natsURL, s.Apply, s.Quarantine)
	mux := http.NewServeMux()
	mux.HandleFunc("/_usage", reader.HandleSummary)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if s.Ping(ctx) != nil {
			http.Error(w, "usage database unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		srv.Shutdown(shutdown)
	}()
	log.Printf("usage service listening on %s", *addr)
	if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
