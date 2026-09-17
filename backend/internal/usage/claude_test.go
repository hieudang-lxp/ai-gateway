package usage

import (
	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
	"strings"
	"testing"
)

func TestClaudeDeduplicatesStreamSnapshotsAndPricesOneHourCache(t *testing.T) {
	data := `{"type":"assistant","timestamp":"2026-09-17T01:00:00Z","message":{"id":"msg1","model":"claude-test","usage":{"input_tokens":10,"cache_read_input_tokens":20,"cache_creation_input_tokens":100,"cache_creation":{"ephemeral_1h_input_tokens":100},"output_tokens":1}}}
{"type":"assistant","timestamp":"2026-09-17T01:00:01Z","message":{"id":"msg1","model":"claude-test","usage":{"input_tokens":10,"cache_read_input_tokens":20,"cache_creation_input_tokens":100,"cache_creation":{"ephemeral_1h_input_tokens":100},"output_tokens":8}}}
`
	rows, err := ParseClaude(strings.NewReader(data), pricing.Pricing{"claude-test": {Input: 1, Output: 5, CacheRead: 0.1, CacheWrite: 1.25}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Usage.Output != 8 || rows[0].CostUSD == nil {
		t.Fatalf("bad rows %+v", rows)
	}
	if got := *rows[0].CostUSD; got < 0.0002519 || got > 0.0002521 {
		t.Fatalf("cost %v", got)
	}
}
