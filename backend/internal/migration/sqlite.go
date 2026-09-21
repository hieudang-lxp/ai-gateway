package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type TableResult struct {
	Rows     int64  `json:"rows"`
	SHA256   string `json:"sha256"`
	Sequence int64  `json:"sequence,omitempty"`
}
type table struct{ name, order string }

var gatewayTables = []table{
	{"calls", "id"}, {"cache", "key"}, {"external_usage", "source,event_id"},
	{"gateway_event_state", "id"}, {"event_outbox", "seq"}, {"sync_state", "id"}, {"budget_snapshot", "id"},
}
var identifier = regexp.MustCompile(`^[a-z_][a-z_0-9]*$`)

func SQLite(ctx context.Context, source, dsn, kind string) (map[string]TableResult, error) {
	tables := gatewayTables
	if kind == "collector" {
		tables = []table{{"event_outbox", "seq"}}
	} else if kind != "gateway" {
		return nil, fmt.Errorf("kind must be gateway or collector")
	}
	if _, err := os.Stat(source); err != nil {
		return nil, err
	}
	if wal, err := os.Stat(source + "-wal"); err == nil && wal.Size() > 0 {
		return nil, fmt.Errorf("source has live WAL; create an offline SQLite backup first")
	}
	u := &url.URL{Scheme: "file", Path: source, RawQuery: "mode=ro&immutable=1"}
	src, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	defer src.Close()
	dst, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	defer dst.Close()
	tx, err := dst.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(824781)`); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS sqlite_migrations(kind TEXT PRIMARY KEY, report TEXT NOT NULL, completed_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return nil, err
	}
	var completed string
	err = tx.QueryRowContext(ctx, `SELECT report FROM sqlite_migrations WHERE kind=$1`, kind).Scan(&completed)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	names, err := src.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, err
	}
	present := map[string]bool{}
	for names.Next() {
		var name string
		if err = names.Scan(&name); err != nil {
			names.Close()
			return nil, err
		}
		known := false
		for _, t := range tables {
			if t.name == name {
				known = true
			}
		}
		if !known {
			names.Close()
			return nil, fmt.Errorf("unsupported source table %q; migration stopped", name)
		}
		present[name] = true
	}
	err = names.Err()
	names.Close()
	if err != nil {
		return nil, err
	}
	if kind == "gateway" && !present["calls"] || kind == "collector" && !present["event_outbox"] {
		return nil, fmt.Errorf("source is not a %s database", kind)
	}
	if completed == "" {
		var locked []string
		for _, t := range tables {
			locked = append(locked, t.name)
		}
		if _, err = tx.ExecContext(ctx, `LOCK TABLE `+strings.Join(locked, ",")+` IN ACCESS EXCLUSIVE MODE`); err != nil {
			return nil, err
		}
		for _, t := range tables {
			var count int64
			if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM `+t.name).Scan(&count); err != nil {
				return nil, err
			}
			if count != 0 {
				return nil, fmt.Errorf("destination table %s is not empty; refusing to merge histories", t.name)
			}
		}
	}
	report := map[string]TableResult{}
	for _, t := range tables {
		if !present[t.name] {
			continue
		}
		rows, err := src.QueryContext(ctx, `SELECT * FROM `+t.name+` ORDER BY `+t.order)
		if err != nil {
			return nil, err
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			return nil, err
		}
		quoted, placeholders := make([]string, len(columns)), make([]string, len(columns))
		for i, col := range columns {
			if !identifier.MatchString(col) {
				rows.Close()
				return nil, fmt.Errorf("unsupported column %q", col)
			}
			quoted[i] = `"` + col + `"`
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		var stmt *sql.Stmt
		if completed == "" {
			stmt, err = tx.PrepareContext(ctx, `INSERT INTO `+t.name+` (`+strings.Join(quoted, ",")+`) VALUES (`+strings.Join(placeholders, ",")+`)`)
			if err != nil {
				rows.Close()
				return nil, err
			}
		}
		result, err := digestRows(ctx, rows, stmt)
		rows.Close()
		if stmt != nil {
			stmt.Close()
		}
		if err != nil {
			return nil, fmt.Errorf("copy %s: %w", t.name, err)
		}
		if t.name == "calls" || t.name == "event_outbox" {
			var hasSequences int
			if err = src.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='sqlite_sequence'`).Scan(&hasSequences); err != nil {
				return nil, err
			}
			if t.name == "calls" {
				for _, checkpoint := range []struct{ table, column string }{{"gateway_event_state", "last_id"}, {"sync_state", "last_synced"}} {
					if !present[checkpoint.table] {
						continue
					}
					var high int64
					if err = src.QueryRowContext(ctx, `SELECT COALESCE(MAX(`+checkpoint.column+`),0) FROM `+checkpoint.table).Scan(&high); err != nil {
						return nil, err
					}
					result.Sequence = max(result.Sequence, high)
				}
			}
			if hasSequences > 0 {
				err = src.QueryRowContext(ctx, `SELECT seq FROM sqlite_sequence WHERE name=?`, t.name).Scan(&result.Sequence)
				if err != nil && err != sql.ErrNoRows {
					return nil, err
				}
			}
		}
		report[t.name] = result
		if completed != "" {
			continue
		}
		pgOrder := t.order
		if t.name == "cache" {
			pgOrder = `key COLLATE "C"`
		}
		if t.name == "external_usage" {
			pgOrder = `source COLLATE "C",event_id COLLATE "C"`
		}
		verify, err := tx.QueryContext(ctx, `SELECT `+strings.Join(quoted, ",")+` FROM `+t.name+` ORDER BY `+pgOrder)
		if err != nil {
			return nil, err
		}
		got, err := digestRows(ctx, verify, nil)
		verify.Close()
		if err != nil {
			return nil, err
		}
		if got.Rows != result.Rows || got.SHA256 != result.SHA256 {
			return nil, fmt.Errorf("verification mismatch in %s: source=%+v destination=%+v", t.name, result, got)
		}
	}
	b, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	if completed != "" {
		if string(b) != completed {
			return nil, fmt.Errorf("snapshot differs from completed migration; refusing to overwrite live PostgreSQL data")
		}
		return report, nil
	}
	for _, t := range tables {
		col := ""
		if t.name == "calls" {
			col = "id"
		}
		if t.name == "event_outbox" {
			col = "seq"
		}
		if col == "" {
			continue
		}

		if _, err = tx.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence($1,$2),GREATEST(COALESCE(MAX(`+col+`),0),$3::bigint,1),COUNT(*)>0 OR $3::bigint>0) FROM `+t.name, t.name, col, report[t.name].Sequence); err != nil {
			return nil, err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO sqlite_migrations(kind,report) VALUES($1,$2)`, kind, string(b)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return report, nil
}

func digestRows(ctx context.Context, rows *sql.Rows, insert *sql.Stmt) (TableResult, error) {
	columns, err := rows.Columns()
	if err != nil {
		return TableResult{}, err
	}
	h := sha256.New()
	encoder := json.NewEncoder(h)
	var n int64
	for rows.Next() {
		values, pointers := make([]any, len(columns)), make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err = rows.Scan(pointers...); err != nil {
			return TableResult{}, err
		}
		if insert != nil {
			if _, err = insert.ExecContext(ctx, values...); err != nil {
				return TableResult{}, err
			}
		}
		if err = encoder.Encode(values); err != nil {
			return TableResult{}, err
		}
		n++
	}
	return TableResult{Rows: n, SHA256: hex.EncodeToString(h.Sum(nil))}, rows.Err()
}
