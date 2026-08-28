package control

import (
	"fmt"
	"time"
)

type Verdict int

const (
	VerdictOK Verdict = iota
	VerdictWarn
	VerdictBlock
)

type BudgetStatus struct {
	Verdict Verdict
	Reason  string
}

// vnLoc is the timezone budget periods are anchored to. LoadLocation needs
// tzdata: present on macOS/Linux; the Docker image installs the tzdata pkg.
var vnLoc = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("ICT", 7*3600)
	}
	return loc
}()

// EvaluateBudget compares spends against limits. A zero limit is disabled.
// Hard anywhere → Block; otherwise Warn anywhere → Warn.
func EvaluateBudget(cfg BudgetConfig, day, week, month float64) BudgetStatus {
	type check struct {
		name  string
		spent float64
		lim   Limit
	}
	checks := []check{
		{"daily", day, cfg.Daily},
		{"weekly", week, cfg.Weekly},
		{"monthly", month, cfg.Monthly},
	}
	var warn string
	for _, c := range checks {
		if c.lim.Hard > 0 && c.spent >= c.lim.Hard {
			return BudgetStatus{VerdictBlock,
				fmt.Sprintf("%s budget hard limit reached: $%.2f spent >= $%.2f", c.name, c.spent, c.lim.Hard)}
		}
		if warn == "" && c.lim.Warn > 0 && c.spent >= c.lim.Warn {
			warn = fmt.Sprintf("%s budget warn threshold: $%.2f spent >= $%.2f", c.name, c.spent, c.lim.Warn)
		}
	}
	if warn != "" {
		return BudgetStatus{VerdictWarn, warn}
	}
	return BudgetStatus{VerdictOK, ""}
}

// PeriodStarts returns the calendar day/Mon-week/month starts containing now,
// in Asia/Ho_Chi_Minh.
func PeriodStarts(now time.Time) (day, week, month time.Time) {
	n := now.In(vnLoc)
	day = time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, vnLoc)
	week = day.AddDate(0, 0, -((int(n.Weekday()) + 6) % 7)) // Monday start
	month = time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, vnLoc)
	return
}
