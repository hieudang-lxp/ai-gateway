package explore

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
)

type Totals struct {
	Sessions  int64   `json:"sessions"`
	Calls     int64   `json:"calls"`
	Tokens    int64   `json:"total_tokens"`
	KnownCost float64 `json:"known_cost_usd"`
	Unknown   int64   `json:"unknown_cost_calls"`
}
type Finding struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Metric      string `json:"metric"`
	Source      string `json:"source,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
}
type InsightsResponse struct {
	Since         string    `json:"since"`
	Until         string    `json:"until"`
	PreviousSince string    `json:"previous_since"`
	Totals        Totals    `json:"totals"`
	Previous      Totals    `json:"previous"`
	Top           []Session `json:"top_sessions"`
	Findings      []Finding `json:"findings"`
	Unassigned    int64     `json:"unassigned_events"`
	GeneratedAt   string    `json:"generated_at"`
	IndexedAt     string    `json:"indexed_at"`
}

func (s *Store) Insights(ctx context.Context, since, until time.Time, p pricing.CatalogSnapshot) (InsightsResponse, error) {
	previousSince := since.Add(-until.Sub(since))
	result := InsightsResponse{Since: since.UTC().Format(time.RFC3339Nano), Until: until.UTC().Format(time.RFC3339Nano), PreviousSince: previousSince.UTC().Format(time.RFC3339Nano), Top: []Session{}, Findings: []Finding{}, GeneratedAt: until.UTC().Format(time.RFC3339Nano)}
	tx, err := readTx(ctx, s.db)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	// Like the usage ledger, transcripts replace gateway accounting when present.
	// This decision is based exclusively on this service's own projection.
	rows, err := tx.QueryContext(ctx, `SELECT `+recordColumns+` FROM records WHERE ts >= $1 AND ts < $2 AND (source<>'claude_gateway' OR NOT EXISTS(SELECT 1 FROM records WHERE source='claude_code'))`, previousSince.Unix(), until.Unix())
	if err != nil {
		return result, err
	}
	current := map[[2]string]*Session{}
	previous := map[[2]string]bool{}
	for rows.Next() {
		r, scanErr := scanRecord(rows)
		if scanErr != nil {
			rows.Close()
			return result, scanErr
		}
		totals := &result.Totals
		isCurrent := r.ts >= since.Unix()
		if !isCurrent {
			totals = &result.Previous
		}
		e := price(r, p)
		totals.Calls++
		totals.Tokens += r.tokens()
		totals.KnownCost += e.KnownCost
		if e.Kind == "unknown" {
			totals.Unknown++
		}
		assigned := r.session != "" && r.source != "claude_gateway"
		if isCurrent {
			if assigned {
				key := [2]string{r.source, r.session}
				if current[key] == nil {
					current[key] = &Session{}
				}
				current[key].add(r, p)
			} else {
				result.Unassigned++
			}
		} else if assigned {
			previous[[2]string{r.source, r.session}] = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	result.Totals.Sessions = int64(len(current))
	result.Previous.Sessions = int64(len(previous))
	ordered := make([]Session, 0, len(current))
	for _, session := range current {
		ordered = append(ordered, *session)
	}
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.KnownCost != b.KnownCost {
			return a.KnownCost > b.KnownCost
		}
		if a.Calls != b.Calls {
			return a.Calls > b.Calls
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.ID < b.ID
	})
	result.Top = append(result.Top, ordered[:min(5, len(ordered))]...)
	if result.Previous.Calls > 0 {
		delta := 100 * float64(result.Totals.Calls-result.Previous.Calls) / float64(result.Previous.Calls)
		result.Findings = append(result.Findings, Finding{ID: "activity-change", Title: "Activity across equal elapsed periods", Description: fmt.Sprintf("%d recorded calls versus %d in the preceding equally long interval. Coverage depends on retained source history.", result.Totals.Calls, result.Previous.Calls), Metric: fmt.Sprintf("%+.1f%% calls", delta)})
	}
	// Evidence thresholds: >=5 calls and >=50K input-context tokens with <10%
	// cache reads; or at least one request with >=100K input-context tokens.
	// These are observations, not claims that changing a session saves money.
	for _, session := range ordered {
		if len(result.Findings) >= 10 {
			break
		}
		if session.Calls >= 5 && session.Input+session.CacheRead+session.CacheWrite >= 50000 && session.CacheRatio != nil && *session.CacheRatio < .1 {
			result.Findings = append(result.Findings, Finding{ID: "low-cache:" + session.Source + ":" + session.ID, Title: "Low observed cache reuse", Description: fmt.Sprintf("%d calls with at least 50,000 input-context tokens; fewer than 10%% were cache reads. Source behavior and cache eligibility can explain this.", session.Calls), Metric: fmt.Sprintf("%.1f%% cache reads", 100**session.CacheRatio), Source: session.Source, SessionID: session.ID})
		}
		if session.maxContext >= 100000 && len(result.Findings) < 10 {
			result.Findings = append(result.Findings, Finding{ID: "large-context:" + session.Source + ":" + session.ID, Title: "Large recorded request context", Description: "At least one request recorded 100,000 or more input, cache-read and cache-write tokens combined. Context is measured per request, not summed across the session for pricing.", Metric: fmt.Sprintf("%d tokens in one request", session.maxContext), Source: session.Source, SessionID: session.ID})
		}
	}
	result.IndexedAt, err = indexedAt(ctx, tx)
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}
