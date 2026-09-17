package usage

import (
	"net/url"
	"testing"
	"time"
)

func TestSummaryPeriodUsesCalendarMonthInLocalTimezone(t *testing.T) {
	zone := time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	for _, test := range []struct{ now, query, want string }{
		{"2026-09-17T15:00:00+07:00", "", "2026-09-01T00:00:00+07:00"},
		{"2026-10-01T00:01:00+07:00", "period=month", "2026-10-01T00:00:00+07:00"},
		{"2024-03-01T00:01:00+07:00", "days=30", "2024-02-01T00:00:00+07:00"},
		{"2026-09-17T15:00:00+07:00", "days=30", "2026-08-19T00:00:00+07:00"},
		{"2026-09-17T15:00:00+07:00", "days=1", "2026-09-17T00:00:00+07:00"},
	} {
		now, _ := time.Parse(time.RFC3339, test.now)
		q, _ := url.ParseQuery(test.query)
		cutoff, _, err := summaryCutoff(now.In(zone), q)
		if err != nil || cutoff.Format(time.RFC3339) != test.want {
			t.Errorf("%s %s: got %s %v", test.now, test.query, cutoff, err)
		}
	}
}

func TestSummaryPeriodRejectsAmbiguousOrInvalidFilters(t *testing.T) {
	for _, query := range []string{"period=year", "period=month&days=30", "days=-1", "days=abc"} {
		q, _ := url.ParseQuery(query)
		if _, _, err := summaryCutoff(time.Now(), q); err == nil {
			t.Errorf("accepted %s", query)
		}
	}
	q, _ := url.ParseQuery("days=0")
	cutoff, _, err := summaryCutoff(time.Now(), q)
	if err != nil || cutoff.Unix() != 0 {
		t.Fatalf("all history cutoff: %v %v", cutoff, err)
	}
}
