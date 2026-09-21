package aictl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Row struct {
	Source     string  `json:"source"`
	Model      string  `json:"model"`
	Calls      int64   `json:"calls"`
	Input      int64   `json:"input_tokens"`
	Output     int64   `json:"output_tokens"`
	CacheRead  int64   `json:"cache_read_tokens"`
	CacheWrite int64   `json:"cache_write_tokens"`
	Cost       float64 `json:"known_cost_usd"`
	Unpriced   int64   `json:"unknown_cost_calls"`
	Estimated  int64   `json:"estimated_cost_calls"`
	Fallback   int64   `json:"fallback_cost_calls"`
}

type Source struct {
	State       string     `json:"state"`
	LastSuccess *time.Time `json:"last_success"`
	PollSeconds int        `json:"poll_seconds"`
	Files       int        `json:"files"`
	Error       string     `json:"error,omitempty"`
}

type Prices struct {
	Source        string    `json:"source"`
	UpdatedAt     time.Time `json:"updated_at"`
	Stale         bool      `json:"stale"`
	Error         string    `json:"error,omitempty"`
	Basis         string    `json:"basis"`
	FallbackModel string    `json:"fallback_model"`
}

type Summary struct {
	Rows        []Row             `json:"rows"`
	Sources     map[string]Source `json:"sources"`
	Pricing     Prices            `json:"pricing"`
	Since       time.Time         `json:"since"`
	GeneratedAt time.Time         `json:"generated_at"`
}

type Client struct {
	base *url.URL
	http *http.Client
}

func newClient(address string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(address)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("--url must be an HTTP(S) origin, e.g. http://localhost:8788")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("--timeout must be positive")
	}
	return &Client{base: u, http: &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *Client) get(ctx context.Context, path, query string, out any) error {
	u := *c.base
	u.Path = path
	u.RawQuery = query
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("contact gateway: %w (check --url and docker compose ps)", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d; use the local gateway with collection enabled", path, resp.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 16<<20))
	if err = decoder.Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func (c *Client) summary(ctx context.Context, query string) (Summary, error) {
	var s Summary
	if err := c.get(ctx, "/_usage", query, &s); err != nil {
		return s, err
	}
	if s.Sources == nil || s.GeneratedAt.IsZero() || s.Since.IsZero() {
		return s, fmt.Errorf("incomplete usage response; update the gateway")
	}
	sort.Slice(s.Rows, func(i, j int) bool {
		if s.Rows[i].Source != s.Rows[j].Source {
			return s.Rows[i].Source < s.Rows[j].Source
		}
		return s.Rows[i].Model < s.Rows[j].Model
	})
	return s, nil
}

func (c *Client) health(ctx context.Context) error {
	var result struct {
		Status string `json:"status"`
	}
	if err := c.get(ctx, "/healthz", "", &result); err != nil {
		return err
	}
	if strings.ToLower(result.Status) != "ok" {
		return fmt.Errorf("gateway reports health %q", result.Status)
	}
	return nil
}
