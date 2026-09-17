package usage

import (
	"strings"
	"testing"
)

func TestCodexPrefersResponseRecordsAndDoesNotAddCachedOrReasoningTwice(t *testing.T) {
	data := `{"timestamp":"2026-09-17T01:00:00Z","type":"session_meta","payload":{"id":"s1"}}
{"type":"turn_context","payload":{"model":"gpt-test"}}
{"timestamp":"2026-09-17T01:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":60,"output_tokens":20}}}}
{"timestamp":"2026-09-17T01:00:01Z","type":"token_usage_record","payload":{"response_id":"r1","usage":{"input_tokens":100,"cached_input_tokens":60,"output_tokens":20,"reasoning_output_tokens":10}}}
{"timestamp":"unfinished"`
	rows, err := ParseCodex(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows", len(rows))
	}
	r := rows[0]
	if r.ID != "r1" || r.Model != "gpt-test" || r.Usage.Input != 40 || r.Usage.CacheRead != 60 || r.Usage.Output != 20 || r.CostUSD != nil {
		t.Fatalf("bad record: %+v", r)
	}
}

func TestCodexLegacyRepeatedCountersDoNotDuplicate(t *testing.T) {
	data := `{"type":"session_meta","payload":{"id":"s1"}}
{"type":"turn_context","payload":{"model":"gpt-test"}}
{"timestamp":"2026-09-17T01:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":60,"output_tokens":20}}}}
{"timestamp":"2026-09-17T01:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":60,"output_tokens":20}}}}
{"timestamp":"2026-09-17T01:00:03Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":150,"cached_input_tokens":80,"output_tokens":30}}}}
`
	rows, err := ParseCodex(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1].Usage.Input != 30 || rows[1].Usage.CacheRead != 20 || rows[1].Usage.Output != 10 {
		t.Fatalf("bad deltas: %+v", rows)
	}
}

const csvHeader = "Date,User,Kind,Model,Input (w/ Cache Write),Input (w/o Cache Write),Cache Read,Output Tokens,Total Tokens,Cost\n"

func TestCodexMixedHistoryKeepsLegacyOnlyEvents(t *testing.T) {
	data := `{"type":"session_meta","payload":{"id":"s1"}}
{"type":"turn_context","payload":{"model":"gpt-test"}}
{"timestamp":"2026-09-17T01:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":60,"output_tokens":20}}}}
{"timestamp":"2026-09-17T01:00:02Z","type":"token_usage_record","payload":{"response_id":"r2","usage":{"input_tokens":50,"cached_input_tokens":20,"output_tokens":10},"thread_token_usage":{"input_tokens":150,"cached_input_tokens":80,"output_tokens":30}}}
{"timestamp":"2026-09-17T01:00:03Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":150,"cached_input_tokens":80,"output_tokens":30}}}}
`
	rows, err := ParseCodex(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	for _, r := range rows {
		total += r.Usage.Input + r.Usage.CacheRead + r.Usage.Output + r.Usage.CacheWrite
	}
	if len(rows) != 2 || total != 180 || len(rows[0].Aliases) != 1 {
		t.Fatalf("bad mixed history: %+v", rows)
	}
}

func TestCursorCSVNormalizesTokensAndUnknownCosts(t *testing.T) {
	data := csvHeader + "2026-09-09T15:15:20.531Z,me@example.com,On-Demand,model,70,4,74,2,150,5.17\n" +
		"2026-09-09T15:16:20.531Z,me@example.com,Included,model,0,1,2,3,6,Included\n" +
		"2026-09-09T15:17:20.531Z,other@example.com,On-Demand,model,0,1,2,3,6,1\n"
	rows, err := ParseCursorCSV(strings.NewReader(data), "me@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Usage.CacheWrite != 70 || rows[0].Usage.Input != 4 || rows[0].CostUSD == nil || *rows[0].CostUSD != 5.17 || rows[1].CostUSD != nil {
		t.Fatalf("bad CSV records: %+v", rows)
	}
	again, err := ParseCursorCSV(strings.NewReader(data), "me@example.com")
	if err != nil || again[0].ID != rows[0].ID {
		t.Fatal("unstable identity")
	}
}

func TestCursorRejectsBadCountsAndAmbiguousAccount(t *testing.T) {
	for _, row := range []string{
		"2026-09-09T15:15:20Z,me@example.com,Included,model,70,4,74,2,999,Included\n",
		"2026-09-09T15:15:20Z,me@example.com,Included,model,70,-4,74,2,142,Included\n",
	} {
		if _, err := ParseCursorCSV(strings.NewReader(csvHeader+row), "me@example.com"); err == nil {
			t.Fatal("accepted invalid token counts")
		}
	}
	data := csvHeader + "2026-09-09T15:15:20Z,a@example.com,Free,model,0,0,0,0,0,Free\n2026-09-09T15:15:20Z,b@example.com,Free,model,0,0,0,0,0,Free\n"
	if _, err := ParseCursorCSV(strings.NewReader(data), ""); err == nil {
		t.Fatal("accepted team-wide import without account filter")
	}
}
