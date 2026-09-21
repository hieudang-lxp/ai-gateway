package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

const codexSessionFixture = `{"type":"session_meta","payload":{"id":"thread-1","session_id":"root-1","cwd":"/workspace/repo"}}
{"type":"turn_context","payload":{"model":"gpt-test"}}
{"timestamp":"2026-09-21T01:00:00Z","type":"token_usage_record","payload":{"response_id":"r1","thread_id":"thread-1","usage":{"input_tokens":10,"output_tokens":2}}}
`

func TestCodexSessionIdentityUsesThreadNotRootAndPreservesUsage(t *testing.T) {
	rows, err := ParseCodex(strings.NewReader(codexSessionFixture))
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if rows[0].SessionID != "thread-1" || rows[0].Project != "/workspace/repo" || rows[0].Usage.Input != 10 {
		t.Fatalf("metadata changed identity or tokens: %+v", rows[0])
	}
	rows, err = ParseCodex(strings.NewReader(strings.Replace(codexSessionFixture, `"thread_id":"thread-1"`, `"thread_id":"original-thread"`, 1)))
	if err != nil || rows[0].SessionID != "original-thread" || rows[0].Project != "" {
		t.Fatalf("inherited response attributed to fork: %+v %v", rows, err)
	}
}

func TestClaudeLateTitleAndCustomTitleDoNotReadPromptContent(t *testing.T) {
	data := `{"type":"user","sessionId":"s1","cwd":"/workspace/repo","message":{"content":"PRIVATE PROMPT MUST NOT BECOME TITLE"}}
{"type":"assistant","sessionId":"s1","timestamp":"2026-09-21T01:00:00Z","message":{"id":"m1","model":"claude-test","usage":{"input_tokens":3,"output_tokens":2}}}
{"type":"ai-title","sessionId":"s1","aiTitle":"Generated title"}
{"type":"custom-title","sessionId":"s1","customTitle":"My session"}
{"type":"ai-title","sessionId":"s1","aiTitle":"Later AI title"}
`
	rows, err := ParseClaude(strings.NewReader(data), pricing.Pricing{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if rows[0].SessionTitle != "My session" || rows[0].SessionID != "s1" || rows[0].Project != "/workspace/repo" || rows[0].Usage.Output != 2 {
		t.Fatalf("bad metadata: %+v", rows[0])
	}
}

type metadataSink struct{ rows []store.ExternalUsage }

func TestClaudeChildSlugDoesNotRenameSharedSession(t *testing.T) {
	for _, child := range []string{`"isSidechain":true`, `"agentId":"child-1"`} {
		data := `{"type":"assistant","sessionId":"parent","slug":"random-child-slug",` + child + `,"timestamp":"2026-09-21T01:00:00Z","message":{"id":"m1","model":"claude-test","usage":{"input_tokens":3}}}` + "\n"
		rows, err := ParseClaude(strings.NewReader(data), pricing.Pricing{})
		if err != nil || len(rows) != 1 || rows[0].SessionID != "parent" || rows[0].SessionTitle != "" {
			t.Fatalf("child slug replaced shared session title: %+v %v", rows, err)
		}
	}
}

func (s *metadataSink) ImportUsage(rows []store.ExternalUsage) error {
	s.rows = append(s.rows, rows...)
	return nil
}

func TestSessionRenameReimportsAnUnchangedRollout(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "sessions"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "sessions", "rollout.jsonl"), []byte(codexSessionFixture), 0600); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(home, "session_index.jsonl")
	writeIndex := func(name string, stamp time.Time) {
		t.Helper()
		content := `{"id":"thread-1","thread_name":"` + name + `","updated_at":"` + stamp.Format(time.RFC3339) + `"}` + "\n"
		if err := os.WriteFile(index, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(index, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	stamp := time.Now().Add(time.Hour)
	writeIndex("First name", stamp)
	sink := &metadataSink{}
	c := NewWorker(sink, nil, pricing.Pricing{}, Config{CodexHome: home, ClaudeProjects: t.TempDir()})
	c.CollectLocal()
	c.CollectLocal()
	if len(sink.rows) != 1 || sink.rows[0].SessionTitle != "First name" {
		t.Fatalf("initial index: %+v", sink.rows)
	}
	writeIndex("Renamed session", stamp.Add(time.Hour))
	c.CollectLocal()
	if len(sink.rows) != 2 || sink.rows[1].ID != sink.rows[0].ID || sink.rows[1].SessionTitle != "Renamed session" || !sink.rows[1].SessionUpdatedAt.After(sink.rows[0].SessionUpdatedAt) {
		t.Fatalf("rename not published as enrichment: %+v", sink.rows)
	}
}
