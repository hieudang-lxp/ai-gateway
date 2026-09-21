package explore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
)

type API struct {
	Store   *Store
	Catalog *pricing.Catalog
}

func (a *API) Handler(sessions bool) http.Handler {
	mux := http.NewServeMux()
	if sessions {
		mux.HandleFunc("GET /_sessions", a.sessions)
		mux.HandleFunc("GET /_sessions/detail", a.detail)
	} else {
		mux.HandleFunc("GET /_insights", a.insights)
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if a.Store.Ping(ctx) != nil {
			http.Error(w, "projection unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	})
	return mux
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
func interval(r *http.Request, now time.Time) (time.Time, error) {
	days := r.URL.Query().Get("days")
	period := r.URL.Query().Get("period")
	if days != "" && period != "" {
		return time.Time{}, errors.New("choose period or days")
	}
	if period != "" && period != "month" {
		return time.Time{}, errors.New("period must be month")
	}
	switch days {
	case "":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), nil
	case "1", "7", "30":
		n, _ := strconv.Atoi(days)
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -n+1), nil
	case "0":
		return time.Unix(0, 0).UTC(), nil
	default:
		return time.Time{}, errors.New("days must be 1, 7, 30, or 0")
	}
}
func limitParam(r *http.Request, fallback int) (int, error) {
	v := r.URL.Query().Get("limit")
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 100 {
		return 0, errors.New("limit must be between 1 and 100")
	}
	return n, nil
}
func sourceValid(source string) bool {
	return source == "" || source == "codex" || source == "claude_code" || source == "cursor"
}
func requestError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadRequest)
}
func queryError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		http.Error(w, "session not found", http.StatusNotFound)
	} else {
		http.Error(w, "projection query unavailable", http.StatusServiceUnavailable)
	}
}
func (a *API) sessions(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	since, err := interval(r, now)
	if err != nil {
		requestError(w, err)
		return
	}
	limit, err := limitParam(r, 25)
	if err != nil {
		requestError(w, err)
		return
	}
	q := r.URL.Query()
	source := q.Get("source")
	if !sourceValid(source) {
		requestError(w, errors.New("unsupported source"))
		return
	}
	cursor := q.Get("cursor")
	if _, err = decodeCursor(cursor); err != nil {
		requestError(w, err)
		return
	}
	query := strings.TrimSpace(q.Get("q"))
	if len(query) > 512 || len(q.Get("model")) > 512 {
		requestError(w, errors.New("search value too long"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	result, err := a.Store.Sessions(ctx, Filter{Since: since, Until: now, Query: query, Source: source, Model: q.Get("model"), Cursor: cursor, Limit: limit}, a.Catalog.Snapshot())
	if err != nil {
		queryError(w, err)
		return
	}
	writeJSON(w, result)
}
func (a *API) detail(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	source, id, cursor := q.Get("source"), q.Get("session_id"), q.Get("cursor")
	if !sourceValid(source) || source == "" || id == "" || len(id) > 2048 {
		requestError(w, errors.New("source and session_id are required"))
		return
	}
	if _, err := decodeCursor(cursor); err != nil {
		requestError(w, err)
		return
	}
	limit, err := limitParam(r, 50)
	if err != nil {
		requestError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	result, err := a.Store.Detail(ctx, source, id, cursor, limit, a.Catalog.Snapshot())
	if err != nil {
		queryError(w, err)
		return
	}
	writeJSON(w, result)
}
func (a *API) insights(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	since, err := interval(r, now)
	if err != nil {
		requestError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	result, err := a.Store.Insights(ctx, since, now, a.Catalog.Snapshot())
	if err != nil {
		queryError(w, err)
		return
	}
	writeJSON(w, result)
}
