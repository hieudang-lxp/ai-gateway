package explore

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
)

type Session struct {
	Source                     string   `json:"source"`
	ID                         string   `json:"session_id"`
	Title                      string   `json:"title"`
	Project                    string   `json:"project"`
	Models                     []string `json:"models"`
	StartedAt                  int64    `json:"started_at"`
	LastAt                     int64    `json:"last_at"`
	Calls                      int64    `json:"calls"`
	Input                      int64    `json:"input_tokens"`
	Output                     int64    `json:"output_tokens"`
	CacheRead                  int64    `json:"cache_read_tokens"`
	CacheWrite                 int64    `json:"cache_write_tokens"`
	KnownCost                  float64  `json:"known_cost_usd"`
	Estimated                  int64    `json:"estimated_cost_calls"`
	Fallback                   int64    `json:"fallback_cost_calls"`
	Unknown                    int64    `json:"unknown_cost_calls"`
	CacheRatio                 *float64 `json:"cache_ratio"`
	titleRevision, titleTS     int64
	projectRevision, projectTS int64
	maxContext                 int64
}
type Event struct {
	ID         string  `json:"event_id"`
	TS         int64   `json:"ts"`
	Model      string  `json:"model"`
	Input      int64   `json:"input_tokens"`
	Output     int64   `json:"output_tokens"`
	CacheRead  int64   `json:"cache_read_tokens"`
	CacheWrite int64   `json:"cache_write_tokens"`
	KnownCost  float64 `json:"known_cost_usd"`
	Kind       string  `json:"cost_kind"`
	Fallback   bool    `json:"fallback"`
}
type SessionsResponse struct {
	Sessions   []Session `json:"sessions"`
	Next       string    `json:"next_cursor"`
	Total      int64     `json:"total"`
	Unassigned int64     `json:"unassigned_events"`
	IndexedAt  string    `json:"indexed_at"`
}
type DetailResponse struct {
	Session Session `json:"session"`
	Events  []Event `json:"events"`
	Next    string  `json:"next_cursor"`
}
type Filter struct {
	Since, Until                 time.Time
	Query, Source, Model, Cursor string
	Limit                        int
}
type pageCursor struct {
	TS     int64  `json:"t"`
	Source string `json:"s"`
	ID     string `json:"i"`
}

func encodeCursor(c pageCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}
func decodeCursor(v string) (pageCursor, error) {
	var c pageCursor
	if v == "" {
		return c, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return c, errors.New("invalid cursor")
	}
	if json.Unmarshal(b, &c) != nil || c.ID == "" || c.TS < 0 {
		return c, errors.New("invalid cursor")
	}
	return c, nil
}

type sqlFilter struct {
	where string
	args  []any
}

func (f *sqlFilter) arg(v any) string {
	f.args = append(f.args, v)
	return fmt.Sprintf("$%d", len(f.args))
}
func buildFilter(f Filter, search bool) sqlFilter {
	b := sqlFilter{where: "ts >= $1 AND ts < $2", args: []any{f.Since.Unix(), f.Until.Unix()}}
	if f.Source != "" {
		b.where += " AND source=" + b.arg(f.Source)
	}
	if f.Model != "" {
		b.where += " AND model=" + b.arg(f.Model)
	}
	if search && f.Query != "" {
		q := b.arg(f.Query)
		prefix := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(f.Query)) + "%"
		p := b.arg(prefix)
		b.where += ` AND (source,session_id) IN (SELECT source,session_id FROM records WHERE search_document @@ plainto_tsquery('simple',` + q + `) OR lower(session_id) LIKE ` + p + `)`
	}
	return b
}
func price(r record, p pricing.CatalogSnapshot) Event {
	e := Event{ID: r.id, TS: r.ts, Model: r.model, Input: r.in, Output: r.out, CacheRead: r.read, CacheWrite: r.write, Kind: r.kind}
	if r.source == "codex" {
		e.KnownCost, e.Fallback = p.Cost(r.model, r.in, r.out, r.read, r.write)
		e.Kind = "estimated"
	} else if r.cost.Valid {
		e.KnownCost = r.cost.Float64
		if e.Kind == "" {
			e.Kind = "reported"
		}
	} else {
		e.Kind = "unknown"
	}
	return e
}
func (s *Session) add(r record, p pricing.CatalogSnapshot) {
	if s.Calls == 0 {
		s.Source = r.source
		s.ID = r.session
		s.Models = []string{}
		s.StartedAt = r.ts
		s.LastAt = r.ts
	}
	s.Calls++
	s.StartedAt = min(s.StartedAt, r.ts)
	s.LastAt = max(s.LastAt, r.ts)

	if r.title != "" && (s.Title == "" || newerField(r.revision, r.ts, r.title, s.titleRevision, s.titleTS, s.Title)) {
		s.Title, s.titleRevision, s.titleTS = r.title, r.revision, r.ts
	}
	if r.project != "" && (s.Project == "" || newerField(r.revision, r.ts, r.project, s.projectRevision, s.projectTS, s.Project)) {
		s.Project, s.projectRevision, s.projectTS = r.project, r.revision, r.ts
	}
	found := false
	for _, m := range s.Models {
		if m == r.model {
			found = true
		}
	}
	if !found {
		s.Models = append(s.Models, r.model)
		sort.Strings(s.Models)
	}
	s.Input += r.in
	s.Output += r.out
	s.CacheRead += r.read
	s.CacheWrite += r.write
	s.maxContext = max(s.maxContext, r.in+r.read+r.write)
	e := price(r, p)
	s.KnownCost += e.KnownCost
	if e.Kind == "estimated" {
		s.Estimated++
	}
	if e.Kind == "unknown" {
		s.Unknown++
	}
	if e.Fallback {
		s.Fallback++
	}
	context := s.Input + s.CacheRead + s.CacheWrite
	if context > 0 {
		ratio := float64(s.CacheRead) / float64(context)
		s.CacheRatio = &ratio
	}
}

func newerField(revision, ts int64, value string, oldRevision, oldTS int64, oldValue string) bool {

	return revision > oldRevision || revision == oldRevision && (ts > oldTS || ts == oldTS && value > oldValue)
}
func indexedAt(ctx context.Context, tx *sql.Tx) (string, error) {
	var ts sql.NullTime
	err := tx.QueryRowContext(ctx, `SELECT MAX(received_at) FROM event_receipts`).Scan(&ts)
	if !ts.Valid {
		return "", err
	}
	return ts.Time.UTC().Format(time.RFC3339Nano), err
}
func readTx(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	return db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
}

func (s *Store) Sessions(ctx context.Context, f Filter, p pricing.CatalogSnapshot) (SessionsResponse, error) {
	result := SessionsResponse{Sessions: []Session{}}
	if f.Limit < 1 || f.Limit > 100 {
		return result, errors.New("limit must be between 1 and 100")
	}
	c, err := decodeCursor(f.Cursor)
	if err != nil {
		return result, err
	}
	tx, err := readTx(ctx, s.db)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	b := buildFilter(f, true)
	assigned := b.where + ` AND session_id<>'' AND source<>'claude_gateway'`
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM (SELECT source,session_id FROM records WHERE `+assigned+` GROUP BY source,session_id) sessions`, b.args...).Scan(&result.Total); err != nil {
		return result, err
	}
	unassigned := buildFilter(f, false)
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM records WHERE `+unassigned.where+` AND session_id='' AND source<>'claude_gateway'`, unassigned.args...).Scan(&result.Unassigned); err != nil {
		return result, err
	}
	query := `SELECT source,session_id,MAX(ts) AS last_at FROM records WHERE ` + assigned + ` GROUP BY source,session_id`
	if f.Cursor != "" {
		query += ` HAVING (-MAX(ts),source COLLATE "C",session_id COLLATE "C") > (` + b.arg(-c.TS) + `,` + b.arg(c.Source) + `,` + b.arg(c.ID) + `)`
	}
	query += ` ORDER BY MAX(ts) DESC,source COLLATE "C",session_id COLLATE "C" LIMIT ` + b.arg(f.Limit+1)
	rows, err := tx.QueryContext(ctx, query, b.args...)
	if err != nil {
		return result, err
	}
	keys := []pageCursor{}
	for rows.Next() {
		var k pageCursor
		if err = rows.Scan(&k.Source, &k.ID, &k.TS); err != nil {
			rows.Close()
			return result, err
		}
		keys = append(keys, k)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if len(keys) > f.Limit {
		keys = keys[:f.Limit]
		result.Next = encodeCursor(keys[len(keys)-1])
	}
	if len(keys) > 0 {
		records := buildFilter(f, false)
		pairs := []string{}
		for _, k := range keys {
			pairs = append(pairs, `(source=`+records.arg(k.Source)+` AND session_id=`+records.arg(k.ID)+`)`)
		}
		rows, err = tx.QueryContext(ctx, `SELECT `+recordColumns+` FROM records WHERE `+records.where+` AND (`+strings.Join(pairs, " OR ")+`)`, records.args...)
		if err != nil {
			return result, err
		}
		groups := map[[2]string]*Session{}
		for rows.Next() {
			r, scanErr := scanRecord(rows)
			if scanErr != nil {
				rows.Close()
				return result, scanErr
			}
			key := [2]string{r.source, r.session}
			if groups[key] == nil {
				groups[key] = &Session{}
			}
			groups[key].add(r, p)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return result, err
		}
		for _, k := range keys {
			result.Sessions = append(result.Sessions, *groups[[2]string{k.Source, k.ID}])
		}
	}
	result.IndexedAt, err = indexedAt(ctx, tx)
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}

var ErrNotFound = errors.New("session not found")

func (s *Store) Detail(ctx context.Context, source, id, cursor string, limit int, p pricing.CatalogSnapshot) (DetailResponse, error) {
	result := DetailResponse{Events: []Event{}}
	if limit < 1 || limit > 100 {
		return result, errors.New("limit must be between 1 and 100")
	}
	c, err := decodeCursor(cursor)
	if err != nil {
		return result, err
	}
	tx, err := readTx(ctx, s.db)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT `+recordColumns+` FROM records WHERE source=$1 AND session_id=$2 AND session_id<>'' AND source<>'claude_gateway'`, source, id)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		r, scanErr := scanRecord(rows)
		if scanErr != nil {
			rows.Close()
			return result, scanErr
		}
		result.Session.add(r, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if result.Session.Calls == 0 {
		return result, ErrNotFound
	}
	query := `SELECT ` + recordColumns + ` FROM records WHERE source=$1 AND session_id=$2`
	args := []any{source, id}
	if cursor != "" {
		query += ` AND (ts,event_id COLLATE "C") > ($3,$4)`
		args = append(args, c.TS, c.ID)
	}
	query += fmt.Sprintf(` ORDER BY ts,event_id COLLATE "C" LIMIT $%d`, len(args)+1)
	args = append(args, limit+1)
	rows, err = tx.QueryContext(ctx, query, args...)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		r, scanErr := scanRecord(rows)
		if scanErr != nil {
			rows.Close()
			return result, scanErr
		}
		result.Events = append(result.Events, price(r, p))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if len(result.Events) > limit {
		result.Events = result.Events[:limit]
		last := result.Events[len(result.Events)-1]
		result.Next = encodeCursor(pageCursor{TS: last.TS, Source: source, ID: last.ID})
	}
	return result, tx.Commit()
}
