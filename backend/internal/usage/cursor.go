package usage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

type CursorCredentials struct {
	Token, Email string
	TeamID       int
	UserID       int
}

// Read on every poll so the IDE remains responsible for refreshing its own
// session. Only three explicitly allowed keys are read. Never read browser cookies.
func ReadCursorCredentials(path string) (CursorCredentials, error) {
	var out CursorCredentials
	uri := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro&_pragma=busy_timeout(5000)"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return out, fmt.Errorf("open Cursor state database")
	}
	defer db.Close()
	values := map[string]string{}
	rows, err := db.Query(`SELECT key,value FROM ItemTable WHERE key IN ('cursorAuth/accessToken','cursorAuth/cachedEmail','cursorAuth/cachedTeam')`)
	if err != nil {
		return out, fmt.Errorf("read Cursor session: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err = rows.Scan(&k, &v); err != nil {
			return out, err
		}
		values[k] = v
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	out.Token = values["cursorAuth/accessToken"]
	out.Email = values["cursorAuth/cachedEmail"]
	var team struct {
		TeamID int `json:"teamId"`
	}
	_ = json.Unmarshal([]byte(values["cursorAuth/cachedTeam"]), &team)
	out.TeamID = team.TeamID
	if out.Token == "" || out.Email == "" {
		return out, fmt.Errorf("Cursor session missing; sign in to the Cursor app")
	}
	return out, nil
}

type CursorClient struct {
	client   *http.Client
	endpoint string
	pageSize int
}

func NewCursorClient() *CursorClient {
	return &CursorClient{
		client:   &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		endpoint: "https://api2.cursor.sh/aiserver.v1.DashboardService/GetFilteredUsageEvents", pageSize: 500,
	}
}

// This is the same Connect RPC endpoint/schema shipped by the Cursor desktop
// app. It is version-sensitive; errors are surfaced instead of reported as zero.
func (c *CursorClient) Fetch(ctx context.Context, creds CursorCredentials, start, end time.Time) ([]store.ExternalUsage, error) {
	if creds.UserID == 0 {
		body, _ := json.Marshal(map[string]int{"teamId": creds.TeamID})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(c.endpoint, "GetFilteredUsageEvents")+"GetMe", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+creds.Token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Connect-Protocol-Version", "1")
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Cursor identity API connection failed")
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return nil, fmt.Errorf("Cursor identity API: HTTP %d", resp.StatusCode)
		}
		var me struct {
			UserID int    `json:"userId"`
			Email  string `json:"email"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&me)
		resp.Body.Close()
		if err != nil || me.UserID == 0 || !strings.EqualFold(me.Email, creds.Email) {
			return nil, fmt.Errorf("Cursor account identity could not be verified")
		}
		creds.UserID = me.UserID
	}
	out := []store.ExternalUsage{}
	seen := 0
	occurrences := map[string]int{}
	for page := 1; page <= 1000; page++ {
		body, _ := json.Marshal(map[string]any{"teamId": creds.TeamID, "userId": creds.UserID, "startDate": strconv.FormatInt(start.UnixMilli(), 10), "endDate": strconv.FormatInt(end.UnixMilli(), 10), "page": page, "pageSize": c.pageSize})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+creds.Token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Connect-Protocol-Version", "1")
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Cursor usage API connection failed")
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return nil, fmt.Errorf("Cursor usage API: HTTP %d (sign in to Cursor if expired)", resp.StatusCode)
		}
		var result struct {
			Total  int `json:"totalUsageEventsCount"`
			Events []struct {
				Timestamp    string   `json:"timestamp"`
				Model        string   `json:"model"`
				Email        string   `json:"userEmail"`
				Owner        string   `json:"owningUser"`
				Kind         string   `json:"kind"`
				ChargedCents *float64 `json:"chargedCents"`
				Tokens       *struct {
					Input  int64 `json:"inputTokens"`
					Output int64 `json:"outputTokens"`
					Write  int64 `json:"cacheWriteTokens"`
					Read   int64 `json:"cacheReadTokens"`
				} `json:"tokenUsage"`
			} `json:"usageEventsDisplay"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&result)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("invalid Cursor usage API response")
		}
		for _, e := range result.Events {
			if !strings.EqualFold(e.Email, creds.Email) && e.Owner != strconv.Itoa(creds.UserID) {
				continue
			}
			ms, err := strconv.ParseInt(e.Timestamp, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid Cursor event timestamp")
			}
			u := store.Usage{}
			if e.Tokens != nil {
				u = store.Usage{Input: e.Tokens.Input, Output: e.Tokens.Output, CacheRead: e.Tokens.Read, CacheWrite: e.Tokens.Write}
			}
			if u.Input < 0 || u.Output < 0 || u.CacheRead < 0 || u.CacheWrite < 0 {
				return nil, fmt.Errorf("invalid Cursor event tokens")
			}
			var cost *float64
			if e.ChargedCents != nil {
				v := *e.ChargedCents / 100
				cost = &v
			}
			key := fmt.Sprintf("%s|%s|%s|%s|%d|%d|%d|%d", e.Timestamp, strings.ToLower(creds.Email), e.Model, e.Kind, u.Input, u.Output, u.CacheRead, u.CacheWrite)
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
			occurrences[hash]++
			out = append(out, store.ExternalUsage{Source: "cursor", ID: fmt.Sprintf("api:%s:%d", hash, occurrences[hash]), TS: time.UnixMilli(ms), Model: e.Model, Usage: u, CostUSD: cost, CostKind: "reported"})
		}
		seen += len(result.Events)
		if seen >= result.Total {
			if seen > 0 && len(out) == 0 {
				return nil, fmt.Errorf("Cursor returned events without matching account ownership")
			}
			return out, nil
		}
		if len(result.Events) == 0 {
			return nil, fmt.Errorf("Cursor pagination ended before reported total")
		}
	}
	return nil, fmt.Errorf("Cursor pagination safety limit exceeded")
}
