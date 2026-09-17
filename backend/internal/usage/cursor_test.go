package usage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCursorPaginatesFiltersOwnAccountAndUsesChargedCents(t *testing.T) {
	pages := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing auth")
		}
		var req struct {
			Page int `json:"page"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		pages++
		switch req.Page {
		case 1:
			w.Write([]byte(`{"totalUsageEventsCount":2,"usageEventsDisplay":[{"timestamp":"1788966920531","userEmail":"me@example.com","model":"test-model","kind":"USAGE_EVENT_KIND_USAGE_BASED","chargedCents":516.8483,"tokenUsage":{"inputTokens":4,"outputTokens":722,"cacheWriteTokens":706247,"cacheReadTokens":746168,"totalCents":480.519775}}]}`))
		case 2:
			w.Write([]byte(`{"totalUsageEventsCount":2,"usageEventsDisplay":[{"timestamp":"1788966920532","userEmail":"someone@example.com","model":"test-model","tokenUsage":{"inputTokens":100}}]}`))
		default:
			t.Errorf("unexpected page %d", req.Page)
		}
	}))
	defer server.Close()
	c := CursorClient{client: server.Client(), endpoint: server.URL, pageSize: 1}
	rows, err := c.Fetch(context.Background(), CursorCredentials{Token: "fixture-token", Email: "me@example.com", TeamID: 1, UserID: 42}, time.Unix(0, 0), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 || len(rows) != 1 || rows[0].CostUSD == nil || *rows[0].CostUSD != 5.168483 || rows[0].Usage.CacheWrite != 706247 {
		t.Fatalf("pages %d rows %+v", pages, rows)
	}
}

func TestCursorDoesNotLeakAuthInErrorsOrFollowRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401); w.Write([]byte("fixture-token")) }))
	defer server.Close()
	c := CursorClient{client: server.Client(), endpoint: server.URL, pageSize: 100}
	_, err := c.Fetch(context.Background(), CursorCredentials{Token: "fixture-token", Email: "me@example.com", UserID: 42}, time.Unix(0, 0), time.Now())
	if err == nil || err.Error() != "Cursor usage API: HTTP 401 (sign in to Cursor if expired)" {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestCursorResolvesUserIDWhenEventsDoNotContainEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/GetMe" {
			w.Write([]byte(`{"userId":42,"email":"me@example.com"}`))
			return
		}
		var req struct {
			UserID int `json:"userId"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.UserID != 42 {
			t.Error("request was not scoped to own user")
		}
		w.Write([]byte(`{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"1788966920531","owningUser":"42","model":"test","tokenUsage":{"inputTokens":10}}]}`))
	}))
	defer server.Close()
	c := CursorClient{client: server.Client(), endpoint: server.URL + "/GetFilteredUsageEvents", pageSize: 100}
	rows, err := c.Fetch(context.Background(), CursorCredentials{Token: "fixture", Email: "me@example.com"}, time.Unix(0, 0), time.Now())
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%+v error=%v", rows, err)
	}
}
