package aictl

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"
)

type Check struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

func diagnose(s Summary, now time.Time) []Check {
	checks := []Check{}
	for _, name := range sourceNames {
		source, ok := s.Sources[name]
		check := Check{Name: name, State: "ok", Detail: "Last sync: " + timestamp(source.LastSuccess)}
		fix := "Check source directory overrides and Docker read-only mounts; inspect docker compose logs --tail 50."
		if name == "cursor" {
			fix = "Sign in to Cursor; check CURSOR_STATE_DIR and gateway logs for API/account errors."
		}
		switch {
		case !ok:
			check.State = "error"
			check.Detail = "Collector missing from gateway response"
			check.Fix = "Enable local collection and update the gateway."
		case source.State != "ok":
			check.State = "error"
			check.Detail = source.State + ": " + source.Error
			check.Fix = fix
		case source.LastSuccess == nil || source.LastSuccess.IsZero():
			check.State = "warning"
			check.Detail = "No successful sync yet"
			check.Fix = fix
		case source.PollSeconds <= 0:
			check.State = "warning"
			check.Detail = "Missing poll interval"
			check.Fix = "Update the gateway."
		case now.Sub(*source.LastSuccess) > time.Duration(source.PollSeconds)*3*time.Second:
			check.State = "warning"
			check.Detail = "Sync is older than three poll intervals"
			check.Fix = fix
		case name != "cursor" && source.Files == 0:
			check.State = "warning"
			check.Detail = "No local JSONL session files found"
			check.Fix = "Use the tool once and verify the mounted session directory."
		}
		checks = append(checks, check)
	}
	price := Check{Name: "pricing", State: "ok", Detail: "Updated: " + timestamp(&s.Pricing.UpdatedAt)}
	if s.Pricing.Stale || s.Pricing.UpdatedAt.IsZero() || now.Sub(s.Pricing.UpdatedAt) > 2*time.Hour || s.Pricing.Error != "" {
		price.State = "warning"
		price.Detail = "Stale or failed price refresh: " + s.Pricing.Error
		price.Fix = "Check gateway access to raw.githubusercontent.com; cached prices remain available and refresh retries hourly."
	}
	checks = append(checks, price)
	return checks
}

func writeChecks(w io.Writer, checks []Check) error {
	t := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(t, "CHECK\tSTATE\tDETAIL / NEXT STEP")
	for _, check := range checks {
		fmt.Fprintf(t, "%s\t%s\t%s\n", check.Name, check.State, check.Detail)
		if check.Fix != "" {
			fmt.Fprintf(t, "\t\t%s\n", check.Fix)
		}
	}
	fmt.Fprintln(t, "Diagnostics use gateway-reported sync state; they do not inspect host files or credentials directly.")
	return t.Flush()
}
