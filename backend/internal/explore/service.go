package explore

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/eventbus"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
)

func Run(name, defaultAddr, durable string, sessions bool) {
	addr := flag.String("addr", defaultAddr, "internal HTTP address")
	dsn := flag.String("database-url", os.Getenv("DATABASE_URL"), "service PostgreSQL DSN")
	natsURL := flag.String("nats-url", os.Getenv("NATS_URL"), "NATS URL")
	cache := flag.String("price-cache", "/data/model-prices.json", "price catalog cache")
	flag.Parse()
	if *dsn == "" || *natsURL == "" {
		log.Fatal("DATABASE_URL and NATS_URL are required")
	}
	s, err := Open(*dsn, sessions)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	catalog := pricing.NewCatalog(*cache)
	go catalog.Run(ctx)
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		eventbus.ConsumeNamed(ctx, *natsURL, durable, s.Apply, s.Quarantine)
	}()
	api := API{Store: s, Catalog: catalog}
	srv := &http.Server{Addr: *addr, Handler: api.Handler(sessions), ReadHeaderTimeout: 5 * time.Second}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		_ = srv.Shutdown(shutdown)
	}()
	log.Printf("%s service listening on %s", name, *addr)
	if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("%s HTTP server: %v", name, err)
	}
	cancel()
	<-shutdownDone
	select {
	case <-consumerDone:
	case <-time.After(35 * time.Second):
		log.Printf("%s consumer shutdown timed out", name)
	}
}
