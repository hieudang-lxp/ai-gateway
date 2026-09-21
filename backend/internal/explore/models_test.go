package explore

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
)

func TestModelsListsDistinctObservedSourcePairs(t *testing.T) {
	s := testProjection(t, false)
	empty, err := s.Models(context.Background())
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty models: %+v %v", empty, err)
	}
	for i, pair := range []ModelOption{{"codex", "shared"}, {"codex", "shared"}, {"cursor", "shared"}, {"claude_code", "claude-model"}, {"codex", ""}, {"claude_gateway", "proxy-only"}} {
		applyRows(t, s, events.ExternalUsage{Source: pair.Source, Model: pair.Model, ID: string(rune('a' + i)), TS: time.Unix(1, 0)})
	}
	rr := httptest.NewRecorder()
	api := API{Store: s}
	api.Handler(true).ServeHTTP(rr, httptest.NewRequest("GET", "/_sessions/models", nil))
	var response struct {
		Models []ModelOption `json:"models"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil || rr.Code != 200 {
		t.Fatalf("models response: status=%d err=%v", rr.Code, err)
	}
	want := []ModelOption{{"claude_code", "claude-model"}, {"codex", "shared"}, {"cursor", "shared"}}
	if !reflect.DeepEqual(response.Models, want) {
		t.Fatalf("models=%+v want=%+v", response.Models, want)
	}
}
