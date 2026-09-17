package aictl

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fixture() Summary {
	now := time.Now()
	s := Summary{Since: now.Add(-time.Hour), GeneratedAt: now, Sources: map[string]Source{}, Pricing: Prices{UpdatedAt: now, Basis: "current standard API rates", FallbackModel: "gpt-5.6-sol"}}
	for _, name := range sourceNames {
		s.Sources[name] = Source{State: "ok", LastSuccess: &now, PollSeconds: 60, Files: 1}
	}
	s.Rows = []Row{{Source: "codex", Model: "z-model", Calls: 1, Input: 10, Output: 2, CacheRead: 30, Cost: 0.25, Estimated: 1}, {Source: "claude_code", Model: "a-model", Calls: 2, Input: 100, Output: 20, Unpriced: 1}}
	return s
}

func TestCommands(t *testing.T) {
	for _, test := range []struct {
		name            string
		args            []string
		query, contains string
		wantError       bool
	}{
		{"month default", []string{"usage"}, "period=month", "TOTAL", false},
		{"explicit month", []string{"usage", "--month", "--json"}, "period=month", `"known_cost_usd": 0.25`, false},
		{"rolling", []string{"usage", "--days", "30"}, "days=30", "z-model", false},
		{"all history", []string{"export", "--days", "0"}, "days=0", "cache_read_tokens", false},
		{"json export", []string{"export", "--format", "json"}, "period=month", `"rows"`, false},
		{"status", []string{"status", "--json"}, "period=month", `"gateway": "ok"`, false},
		{"doctor", []string{"doctor"}, "period=month", "pricing", false},
		{"conflicting period", []string{"usage", "--month", "--days", "30"}, "", "", true},
		{"negative days", []string{"usage", "--days", "-1"}, "", "", true},
		{"bad format", []string{"export", "--format", "xml"}, "", "", true},
		{"conflicting format", []string{"export", "--format", "csv", "--json"}, "", "", true},
		{"extra arg", []string{"usage", "unexpected"}, "", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := fixture()
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path == "/healthz" {
					fmt.Fprint(w, `{"status":"ok"}`)
					return
				}
				if r.URL.Path != "/_usage" || r.URL.RawQuery != test.query {
					t.Errorf("unexpected request %s", r.URL)
				}
				json.NewEncoder(w).Encode(s)
			}))
			defer server.Close()
			var out, stderr bytes.Buffer
			cmd := NewCommand()
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			cmd.SetArgs(append([]string{"--url", server.URL}, test.args...))
			err := cmd.Execute()
			if (err != nil) != test.wantError {
				t.Fatalf("error=%v output=%s", err, out.String())
			}
			if !strings.Contains(out.String(), test.contains) {
				t.Fatalf("missing %q in %s", test.contains, out.String())
			}
			if test.wantError && requests != 0 {
				t.Fatal("invalid flags should not issue requests")
			}
			if !test.wantError && stderr.Len() != 0 {
				t.Fatalf("unexpected stderr %s", stderr.String())
			}
		})
	}
}

func TestUnhealthyDoctorStillWritesJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "unavailable", 503) }))
	defer server.Close()
	var out bytes.Buffer
	cmd := NewCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"doctor", "--url", server.URL, "--json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected failure exit")
	}
	var checks []Check
	if err := json.Unmarshal(out.Bytes(), &checks); err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 || checks[0].State != "error" {
		t.Fatalf("checks=%+v", checks)
	}
}

func TestClientFailuresAndDeterminism(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
		fail       bool
	}{
		{"missing fields", `{}`, 200, true}, {"invalid JSON", `oops`, 200, true}, {"disabled collector", `not found`, 404, true}, {"good", "", 200, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := fixture()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				if test.body != "" {
					fmt.Fprint(w, test.body)
				} else {
					json.NewEncoder(w).Encode(s)
				}
			}))
			defer server.Close()
			c, _ := newClient(server.URL, time.Second)
			a, err := c.summary(t.Context(), "")
			if (err != nil) != test.fail {
				t.Fatalf("err=%v", err)
			}
			if !test.fail {
				b, err := c.summary(t.Context(), "")
				if err != nil || !reflect.DeepEqual(a, b) {
					t.Fatal("non-deterministic result")
				}
				if a.Rows[0].Source != "claude_code" {
					t.Fatal("rows not sorted")
				}
			}
		})
	}
}

func TestCSVPreservesAccountingAndEscapesFormulas(t *testing.T) {
	s := fixture()
	s.Rows[0].Model = "=HYPERLINK(\"bad\")"
	var out bytes.Buffer
	if err := writeCSV(&out, s); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&out).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[1][3] != "'=HYPERLINK(\"bad\")" || rows[1][7] != "30" || rows[1][9] != "0.250000" || rows[2][10] != "1" {
		t.Fatalf("CSV=%v", rows)
	}
	s.Rows = nil
	out.Reset()
	writeCSV(&out, s)
	rows, _ = csv.NewReader(&out).ReadAll()
	if len(rows) != 1 {
		t.Fatal("empty export should contain header")
	}
}

func TestCSVGolden(t *testing.T) {
	s := fixture()
	s.Since = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	s.GeneratedAt = time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	s.Pricing.UpdatedAt = s.GeneratedAt
	s.Rows = s.Rows[:1]
	s.Rows[0].Model = "gpt-test"
	want, err := os.ReadFile("testdata/usage.csv")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		var out bytes.Buffer
		if err := writeCSV(&out, s); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out.Bytes(), want) {
			t.Fatalf("CSV differs from golden:\n%s", out.String())
		}
	}
}

func TestURLValidationAndTimeout(t *testing.T) {
	for _, address := range []string{"file:///tmp/test", "http://user:password@localhost", "http://localhost/dashboard/", "http://localhost?token=x"} {
		if _, err := newClient(address, time.Second); err == nil {
			t.Fatalf("accepted %s", address)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	c, _ := newClient(server.URL, 10*time.Millisecond)
	if err := c.health(t.Context()); err == nil {
		t.Fatal("expected request timeout")
	}
}

func TestDiagnostics(t *testing.T) {
	for _, test := range []struct {
		name         string
		mutate       func(*Summary)
		check, state string
	}{
		{"healthy", func(*Summary) {}, "codex", "ok"},
		{"missing", func(s *Summary) { delete(s.Sources, "codex") }, "codex", "error"},
		{"no logs", func(s *Summary) { v := s.Sources["codex"]; v.Files = 0; s.Sources["codex"] = v }, "codex", "warning"},
		{"stalled", func(s *Summary) {
			v := s.Sources["cursor"]
			old := time.Now().Add(-time.Hour)
			v.LastSuccess = &old
			s.Sources["cursor"] = v
		}, "cursor", "warning"},
		{"stale prices", func(s *Summary) { s.Pricing.Stale = true }, "pricing", "warning"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := fixture()
			test.mutate(&s)
			for _, c := range diagnose(s, time.Now()) {
				if c.Name == test.check {
					if c.State != test.state {
						t.Fatalf("check=%+v", c)
					}
					return
				}
			}
			t.Fatal("missing check")
		})
	}
}
