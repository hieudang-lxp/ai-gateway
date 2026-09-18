// Package events defines the versioned, content-free service boundary.
package events

import (
	"errors"
	"fmt"
	"math"
	"time"
)

const Subject = "usage.ingested.v1"
const StatusSubject = "collector.status.v1"
const Stream = "AI_USAGE"

type Usage struct {
	Input      int64
	Output     int64
	CacheRead  int64
	CacheWrite int64
}

type ExternalUsage struct {
	Aliases  []string
	Source   string
	ID       string
	TS       time.Time
	Model    string
	Usage    Usage
	CostUSD  *float64
	CostKind string
}

type SourceStatus struct {
	State       string     `json:"state"`
	LastAttempt time.Time  `json:"last_attempt"`
	LastSuccess *time.Time `json:"last_success"`
	Error       string     `json:"error,omitempty"`
	Files       int        `json:"files,omitempty"`
	PollSeconds int        `json:"poll_seconds"`
}

type Envelope struct {
	Version int             `json:"version"`
	ID      string          `json:"id"`
	Rows    []ExternalUsage `json:"rows,omitempty"`
	Source  string          `json:"source,omitempty"`
	Status  *SourceStatus   `json:"status,omitempty"`
}

var ErrInvalid = errors.New("invalid accounting event")

func ValidateRows(rows []ExternalUsage) error {
	for _, r := range rows {
		switch r.Source {
		case "codex", "claude_code", "cursor", "claude_gateway":
		default:
			return fmt.Errorf("%w: unsupported source", ErrInvalid)
		}
		if r.ID == "" || r.TS.IsZero() {
			return fmt.Errorf("%w: missing identity or timestamp", ErrInvalid)
		}
		if r.Usage.Input < 0 || r.Usage.Output < 0 || r.Usage.CacheRead < 0 || r.Usage.CacheWrite < 0 {
			return fmt.Errorf("%w: negative token count", ErrInvalid)
		}
		if r.CostUSD != nil && (math.IsNaN(*r.CostUSD) || math.IsInf(*r.CostUSD, 0)) {
			return fmt.Errorf("%w: non-finite cost", ErrInvalid)
		}
	}
	return nil
}
func Validate(e Envelope) error {
	if e.Version != 1 || e.ID == "" {
		return fmt.Errorf("%w: version or envelope identity", ErrInvalid)
	}
	if len(e.Rows) == 0 && e.Status == nil {
		return fmt.Errorf("%w: empty payload", ErrInvalid)
	}
	if err := ValidateRows(e.Rows); err != nil {
		return err
	}
	if e.Status != nil {
		if e.Source != "codex" && e.Source != "claude_code" && e.Source != "cursor" {
			return fmt.Errorf("%w: status source", ErrInvalid)
		}
		if e.Status.LastAttempt.IsZero() {
			return fmt.Errorf("%w: status timestamp", ErrInvalid)
		}
	}
	return nil
}
