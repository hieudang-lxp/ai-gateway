package memorysync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexVSCodeResponseDialectFiltersContextAndKeepsHumanAnswers(t *testing.T) {
	got, err := scanFixture(t, "codex", `{"type":"session_meta","payload":{"id":"human-session","source":"vscode","cli_version":"0.153.4"}}
{"type":"response_item","timestamp":"2026-09-22T01:00:00Z","payload":{"type":"message","role":"user","id":"human-1","content":[{"type":"input_text","text":"<in-app-browser-context>private page context</in-app-browser-context>\n\nok"}]}}
{"type":"response_item","timestamp":"2026-09-22T01:00:01Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<recommended_plugins>internal config</recommended_plugins>"},{"type":"input_text","text":"<environment_context>internal context</environment_context>"}]}}
{"type":"response_item","timestamp":"2026-09-22T01:00:02Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"yes"},{"type":"input_text","text":"<image>private attachment location</image>"},{"type":"input_image","image_url":"private-image"}]}}
{"type":"response_item","timestamp":"2026-09-22T01:00:03Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<send_user_message_question_reply>[{\"questionItemId\":\"q1\",\"question\":\"model-generated question\",\"answer\":\"please continue\"}]</send_user_message_question_reply>"}]}}
{"type":"response_item","timestamp":"2026-09-22T01:00:04Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<task-notification>agent update</task-notification>"}]}}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Text != "ok" || got[0].UUID != "human-1" || got[1].Text != "yes" || got[2].Text != "please continue" {
		t.Fatalf("wrong human filtering: count=%d", len(got))
	}
}

func TestCodexSubagentMetadataExcludesBothDialectsEvenWhenLate(t *testing.T) {
	for _, source := range []string{`{"subagent":{"thread_spawn":{"parent_thread_id":"parent"}}}`, `{"subagent":"review"}`} {
		got, err := scanFixture(t, "codex", `{"type":"event_msg","timestamp":"2026-09-22T01:00:00Z","payload":{"type":"user_message","message":"machine-generated task"}}
{"type":"response_item","timestamp":"2026-09-22T01:00:01Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"another machine-generated task"}]}}
{"type":"session_meta","payload":{"id":"agent-session","source":`+source+`}}
`)
		if err != nil || len(got) != 0 {
			t.Fatalf("subagent input emitted: error=%v count=%d", err, len(got))
		}
	}
}

func TestCodexVSCodeIdentityStableWhenEventArrivesOnLaterPoll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	initial := `{"type":"session_meta","payload":{"id":"human-session","source":"vscode","cli_version":"0.154.0-alpha.6.2"}}
{"type":"response_item","timestamp":"2026-09-22T01:00:00Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"ok"}]}}
`
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	source := NewCodexSource(dir)
	var first, second []Message
	if err := source.Scan(context.Background(), func(m Message) error { first = append(first, m); return nil }); err != nil {
		t.Fatal(err)
	}
	appended := initial + `{"type":"event_msg","timestamp":"2026-09-22T01:00:01Z","payload":{"type":"user_message","message":"ok"}}
{"type":"response_item","timestamp":"2026-09-22T01:00:02Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"ok"}]}}
`
	if err := os.WriteFile(path, []byte(appended), 0600); err != nil {
		t.Fatal(err)
	}
	if err := source.Scan(context.Background(), func(m Message) error { second = append(second, m); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 2 || first[0] != second[0] || second[0].UUID == second[1].UUID {
		t.Fatalf("append changed identity or duplicated event: first=%d second=%d", len(first), len(second))
	}
}

func TestSourceErrorsAggregateRepeatedUnsupportedFiles(t *testing.T) {
	dir := t.TempDir()
	data := `{"type":"response_item","timestamp":"2026-09-22T01:00:00Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"ok"}]}}` + "\n"
	for _, name := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(dir, name+".jsonl"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	err := NewCodexSource(dir).Scan(context.Background(), func(Message) error { return nil })
	if err == nil || strings.Count(err.Error(), "unsupported Codex") != 1 || !strings.Contains(err.Error(), "3 files") {
		t.Fatalf("expected compact source errors: %v", err)
	}
}

func scanFixture(t *testing.T, source string, text string) ([]Message, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "session.jsonl"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	var s Source = NewClaudeSource(dir)
	if source == "codex" {
		s = NewCodexSource(dir)
	}
	var got []Message
	err := s.Scan(context.Background(), func(m Message) error { got = append(got, m); return nil })
	return got, err
}

func TestClaudePreservesHumanIdentityAndShortMessages(t *testing.T) {
	got, err := scanFixture(t, "claude", `{"type":"user","uuid":"u1","sessionId":"s1","timestamp":"2026-09-22T01:00:00Z","message":{"role":"user","content":"ok"}}
{"type":"assistant","uuid":"a","message":{"content":"answer"}}
{"type":"user","uuid":"tool","message":{"content":[{"type":"tool_result","content":"private tool output"}]}}
{"type":"user","isMeta":true,"message":{"content":"injected"}}
{"type":"user","message":{"content":"<system-reminder>injected</system-reminder>"}}
{"type":"user","uuid":"u2","sessionId":"s1","timestamp":"2026-09-22T01:00:01Z","message":{"content":[{"type":"text","text":"hi"},{"type":"text","text":"there"}]}}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != "ok" || got[0].UUID != "u1" || got[0].SessionID != "s1" || got[0].Speaker != "user" || got[1].Text != "hi there" {
		t.Fatalf("unexpected parsed messages: %+v", got)
	}
}

func TestCodexEventsExcludeHarnessAndResponseDuplicates(t *testing.T) {
	got, err := scanFixture(t, "codex", `{"type":"session_meta","payload":{"id":"thread-1"}}
{"type":"response_item","timestamp":"2026-09-22T01:00:00Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>injected</environment_context>"}]}}
{"type":"response_item","timestamp":"2026-09-22T01:00:01Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"ok"}]}}
{"type":"event_msg","timestamp":"2026-09-22T01:00:01Z","payload":{"type":"user_message","message":"ok"}}
{"type":"event_msg","timestamp":"2026-09-22T01:00:02Z","payload":{"type":"agent_message","message":"answer"}}
{"type":"event_msg","timestamp":"2026-09-22T01:00:03Z","payload":{"type":"user_message","message":"ok"}}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != "ok" || got[1].Text != "ok" || got[0].UUID == got[1].UUID || got[0].SessionID != "thread-1" {
		t.Fatalf("unexpected messages: %+v", got)
	}
}

func TestCodexLegacyResponseOnlyReportedAsUnsupported(t *testing.T) {
	got, err := scanFixture(t, "codex", `{"type":"session_meta","payload":{"id":"old-thread"}}
{"type":"response_item","timestamp":"2026-09-22T01:00:01Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"thanks"}]}}
{"type":"response_item","timestamp":"2026-09-22T01:00:02Z","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"welcome"}]}}
`)
	if err == nil || len(got) != 0 {
		t.Fatalf("response-only log must be explicit unsupported without unsafe delivery: %v %+v", err, got)
	}
}

func TestPartialTrailingRecordRetriesAndMalformedRecordIsVisible(t *testing.T) {
	got, err := scanFixture(t, "claude", `{"type":"user","uuid":"u1","sessionId":"s1","timestamp":"2026-09-22T01:00:00Z","message":{"content":"ok"}}
{"type":"user"`)
	if err != nil || len(got) != 1 {
		t.Fatalf("partial tail: %v, %d", err, len(got))
	}
	_, err = scanFixture(t, "claude", "{broken}\n")
	if err == nil {
		t.Fatal("malformed complete record must be reported")
	}
}

func TestMissingSourceIsAnError(t *testing.T) {
	if err := NewClaudeSource(filepath.Join(t.TempDir(), "missing")).Scan(context.Background(), func(Message) error { return nil }); err == nil {
		t.Fatal("missing source reported as healthy")
	}
}

func TestCompleteFinalJSONWithoutNewlineIsIncluded(t *testing.T) {
	got, err := scanFixture(t, "claude", `{"type":"user","uuid":"u1","sessionId":"s1","timestamp":"2026-09-22T01:00:00Z","message":{"content":"ok"}}`)
	if err != nil || len(got) != 1 {
		t.Fatalf("complete final record lost: %v %+v", err, got)
	}
}

func TestMalformedRecordDoesNotHideLaterMessages(t *testing.T) {
	got, err := scanFixture(t, "claude", "{broken}\n"+`{"type":"user","uuid":"u1","sessionId":"s1","timestamp":"2026-09-22T01:00:00Z","message":{"content":"ok"}}`+"\n")
	if err == nil || len(got) != 1 {
		t.Fatalf("expected visible malformed error and surviving message: %v %+v", err, got)
	}
}
