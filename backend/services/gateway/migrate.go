package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/eventbus"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/migration"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

func runMigrateSQLite(args []string) {
	fs := flag.NewFlagSet("migrate-sqlite", flag.ExitOnError)
	source := fs.String("source", "", "offline SQLite backup (required)")
	kind := fs.String("kind", "gateway", "gateway or collector")
	dsn := fs.String("database-url", os.Getenv("DATABASE_URL"), "destination PostgreSQL URL")
	fs.Parse(args)
	if *source == "" || *dsn == "" {
		log.Fatal("-source and DATABASE_URL/-database-url are required")
	}
	switch *kind {
	case "gateway":
		s, err := store.OpenPostgres(*dsn)
		if err != nil {
			log.Fatal(err)
		}
		s.Close()
	case "collector":
		s, err := eventbus.OpenPostgres(*dsn)
		if err != nil {
			log.Fatal(err)
		}
		s.Close()
	default:
		log.Fatal("-kind must be gateway or collector")
	}
	report, err := migration.SQLite(context.Background(), *source, *dsn, *kind)
	if err != nil {
		log.Fatal(err)
	}
	json.NewEncoder(os.Stdout).Encode(report)
}
