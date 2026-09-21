package aictl

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

var sourceNames = []string{"claude_code", "codex", "cursor"}

func writeJSON(w io.Writer, value any) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(value)
}
func timestamp(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "never"
	}
	return t.Format(time.RFC3339)
}

func writeStatus(w io.Writer, s Summary) error {
	t := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(t, "Gateway: ok\nSOURCE\tSTATE\tLAST SYNC\tPOLL\tDETAIL")
	for _, name := range sourceNames {
		source, ok := s.Sources[name]
		if !ok {
			source.State = "missing"
		}
		fmt.Fprintf(t, "%s\t%s\t%s\t%ds\t%s\n", name, source.State, timestamp(source.LastSuccess), source.PollSeconds, source.Error)
	}
	fmt.Fprintf(t, "Prices\tstale=%t\t%s\t\t%s\n", s.Pricing.Stale, timestamp(&s.Pricing.UpdatedAt), s.Pricing.Error)
	return t.Flush()
}

func writeUsage(w io.Writer, s Summary) error {
	t := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(t, "Period: %s — %s\n", s.Since.Format(time.RFC3339), s.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintln(t, "SOURCE / MODEL\tEVENTS\tTOKENS (incl. cache)\tVALUE USD\tUNPRICED\tESTIMATED\tFALLBACK")
	var calls, tokens, unpriced, estimated, fallback int64
	var cost float64
	for _, r := range s.Rows {
		count := r.Input + r.Output + r.CacheRead + r.CacheWrite
		fmt.Fprintf(t, "%s / %s\t%d\t%d\t%.4f\t%d\t%d\t%d\n", r.Source, r.Model, r.Calls, count, r.Cost, r.Unpriced, r.Estimated, r.Fallback)
		calls += r.Calls
		tokens += count
		cost += r.Cost
		unpriced += r.Unpriced
		estimated += r.Estimated
		fallback += r.Fallback
	}
	fmt.Fprintf(t, "TOTAL\t%d\t%d\t%.4f\t%d\t%d\t%d\n", calls, tokens, cost, unpriced, estimated, fallback)
	fmt.Fprintln(t, "Usage value includes API estimates, not subscription fees. Unpriced events are excluded from value.")
	fmt.Fprintf(t, "Codex basis: %s; fallback model: %s; prices updated: %s (stale=%t).\n", s.Pricing.Basis, s.Pricing.FallbackModel, timestamp(&s.Pricing.UpdatedAt), s.Pricing.Stale)
	for _, name := range sourceNames {
		source, ok := s.Sources[name]
		if !ok || source.State != "ok" {
			fmt.Fprintf(t, "Warning: %s collector %s; %s\n", name, source.State, source.Error)
		}
	}
	return t.Flush()
}

func csvText(s string) string {
	if strings.ContainsAny(strings.TrimLeft(s, " "), "\t\r\n") {
		return "'" + s
	}
	if v := strings.TrimSpace(s); v != "" && strings.ContainsRune("=+-@", rune(v[0])) {
		return "'" + s
	}
	return s
}
func writeCSV(w io.Writer, s Summary) error {
	c := csv.NewWriter(w)
	header := []string{"since", "generated_at", "source", "model", "events", "input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens", "known_cost_usd", "unpriced_events", "estimated_events", "fallback_events", "prices_updated_at", "prices_stale", "price_basis", "fallback_model"}
	if err := c.Write(header); err != nil {
		return err
	}
	for _, r := range s.Rows {
		row := []string{s.Since.Format(time.RFC3339), s.GeneratedAt.Format(time.RFC3339), csvText(r.Source), csvText(r.Model), strconv.FormatInt(r.Calls, 10), strconv.FormatInt(r.Input, 10), strconv.FormatInt(r.Output, 10), strconv.FormatInt(r.CacheRead, 10), strconv.FormatInt(r.CacheWrite, 10), strconv.FormatFloat(r.Cost, 'f', 6, 64), strconv.FormatInt(r.Unpriced, 10), strconv.FormatInt(r.Estimated, 10), strconv.FormatInt(r.Fallback, 10), timestamp(&s.Pricing.UpdatedAt), strconv.FormatBool(s.Pricing.Stale), csvText(s.Pricing.Basis), csvText(s.Pricing.FallbackModel)}
		if err := c.Write(row); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}
