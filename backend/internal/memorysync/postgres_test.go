package memorysync

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestPostgresSenderReceiptsSurviveReopen(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "test_memorysync_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := OpenPostgres(u.String())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m := fixtureMessage()
	if err = db.Enqueue(ctx, m); err != nil {
		t.Fatal(err)
	}
	if err = db.Enqueue(ctx, m); err != nil {
		t.Fatal(err)
	}
	pending, err := db.Pending(ctx, 100)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending=%v error=%v", pending, err)
	}
	if err = db.Delivered(ctx, pending); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = OpenPostgres(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Enqueue(ctx, m); err != nil {
		t.Fatal(err)
	}
	counts, err := db.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if counts["claude_code"].Delivered != 1 || counts["claude_code"].Pending != 0 {
		t.Fatalf("replay lost receipt: %+v", counts)
	}
	var payload string
	if err = db.db.QueryRow(`SELECT payload FROM memory_deliveries`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if payload != "" {
		t.Fatal("delivered plaintext retained")
	}
	m.Text = "changed"
	if err = db.Enqueue(ctx, m); err == nil {
		t.Fatal("changed delivered identity accepted")
	}
}
