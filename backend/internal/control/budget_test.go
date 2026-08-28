package control

import (
	"testing"
	"time"
)

func TestEvaluateBudget(t *testing.T) {
	cfg := BudgetConfig{
		Daily:   Limit{Warn: 10, Hard: 20},
		Weekly:  Limit{Warn: 50, Hard: 100},
		Monthly: Limit{Warn: 150, Hard: 300},
	}
	cases := []struct {
		name             string
		day, week, month float64
		want             Verdict
	}{
		{"all under", 5, 20, 100, VerdictOK},
		{"day warn", 10, 20, 100, VerdictWarn},
		{"day hard", 20, 20, 100, VerdictBlock},
		{"week hard beats day warn", 12, 100, 100, VerdictBlock},
		{"month warn", 5, 20, 200, VerdictWarn},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EvaluateBudget(cfg, c.day, c.week, c.month)
			if got.Verdict != c.want {
				t.Fatalf("verdict = %v (%q) want %v", got.Verdict, got.Reason, c.want)
			}
			if got.Verdict != VerdictOK && got.Reason == "" {
				t.Fatal("want a reason for warn/block")
			}
		})
	}
}

func TestEvaluateBudgetZeroConfigDisabled(t *testing.T) {
	got := EvaluateBudget(BudgetConfig{}, 1e9, 1e9, 1e9)
	if got.Verdict != VerdictOK {
		t.Fatalf("zero limits must disable budgets, got %v", got.Verdict)
	}
}

func TestPeriodStarts(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	// Friday 2026-08-28 10:30 +07
	now := time.Date(2026, 8, 28, 10, 30, 0, 0, loc)
	day, week, month := PeriodStarts(now)
	if !day.Equal(time.Date(2026, 8, 28, 0, 0, 0, 0, loc)) {
		t.Fatalf("day = %v", day)
	}
	if !week.Equal(time.Date(2026, 8, 24, 0, 0, 0, 0, loc)) { // Monday
		t.Fatalf("week = %v", week)
	}
	if !month.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, loc)) {
		t.Fatalf("month = %v", month)
	}
	// Sunday must belong to the week started the previous Monday
	sun := time.Date(2026, 8, 30, 23, 0, 0, 0, loc)
	_, week2, _ := PeriodStarts(sun)
	if !week2.Equal(time.Date(2026, 8, 24, 0, 0, 0, 0, loc)) {
		t.Fatalf("sunday week = %v", week2)
	}
}
