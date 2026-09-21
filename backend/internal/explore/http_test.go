package explore

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestIntervalsAndRequestValidation(t *testing.T) {
	now := time.Date(2026, 9, 21, 14, 30, 0, 0, time.UTC)
	for _, tc := range []struct {
		query string
		want  time.Time
	}{{"days=1", time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)}, {"days=7", time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)}, {"days=30", time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)}, {"period=month", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}, {"days=0", time.Unix(0, 0).UTC()}} {
		r := httptest.NewRequest("GET", "/_sessions?"+tc.query, nil)
		got, err := interval(r, now)
		if err != nil || !got.Equal(tc.want) {
			t.Fatalf("interval %s=%v err=%v", tc.query, got, err)
		}
	}
	api := API{}
	for _, path := range []string{"/_sessions?limit=0", "/_sessions?limit=101", "/_sessions?cursor=invalid", "/_sessions?source=fake", "/_sessions?days=-1", "/_sessions?days=7&period=month", "/_sessions/detail?source=codex"} {
		rr := httptest.NewRecorder()
		api.Handler(true).ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
		if rr.Code != 400 {
			t.Fatalf("%s status=%d", path, rr.Code)
		}
	}
}

func TestIntervalsUseLocalDayAndMonthAtUTCBoundary(t *testing.T) {
	location := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	// Local October has begun while UTC is still in September.
	now := time.Date(2026, 10, 1, 1, 30, 0, 0, location)
	for _, query := range []string{"period=month", "days=1"} {
		got, err := interval(httptest.NewRequest("GET", "/_sessions?"+query, nil), now)
		want := time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)
		if err != nil || !got.Equal(want) {
			t.Fatalf("local %s cutoff=%v, want %v; err=%v", query, got, want, err)
		}
	}
}
