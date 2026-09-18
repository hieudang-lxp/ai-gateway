package eventbus

import (
	"encoding/json"
	"errors"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	"testing"
	"time"
)

func TestPoisonMessageQuarantinedButDatabaseErrorsRetried(t *testing.T) {
	valid := events.Envelope{Version: 1, ID: "valid", Rows: []events.ExternalUsage{{Source: "codex", ID: "response", TS: time.Now()}}}
	payload, _ := json.Marshal(valid)
	applied, rejected := 0, 0
	apply := func(events.Envelope) error { applied++; return nil }
	reject := func(string) error { rejected++; return nil }
	if err := process([]byte(`{broken`), apply, reject); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.Rows = []events.ExternalUsage{{Source: "claude_code", ID: "missing-time"}}
	bad, _ := json.Marshal(invalid)
	if err := process(bad, apply, reject); err != nil {
		t.Fatal(err)
	}
	if err := process(payload, apply, reject); err != nil {
		t.Fatal(err)
	}
	if applied != 1 || rejected != 2 {
		t.Fatalf("applied=%d rejected=%d", applied, rejected)
	}
	offline := errors.New("database offline")
	if err := process(payload, func(events.Envelope) error { return offline }, reject); !errors.Is(err, offline) {
		t.Fatalf("database failure acknowledged: %v", err)
	}
	if err := process(bad, apply, func(string) error { return offline }); !errors.Is(err, offline) {
		t.Fatalf("quarantine failure acknowledged: %v", err)
	}
}
