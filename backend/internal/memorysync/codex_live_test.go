package memorysync

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Opt-in compatibility audit. This reads only local transcripts, sends nothing,
// and prints aggregate counts rather than conversation text or identifiers.
func TestCodexLiveHumanCoverage(t *testing.T) {
	home := os.Getenv("CODEX_MEMORYSYNC_TEST_HOME")
	if home == "" {
		t.Skip("set CODEX_MEMORYSYNC_TEST_HOME for a read-only local transcript audit")
	}
	ctx := context.Background()
	subagents := map[string]bool{}
	files, excludedFiles, vscodeFiles, messages := 0, 0, 0, 0
	roots := []string{filepath.Join(home, "sessions"), filepath.Join(home, "archived_sessions")}
	for _, root := range roots {
		if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(path) != ".jsonl" {
				return nil
			}
			metadata, err := inspectCodexMetadata(ctx, path)
			if err != nil {
				return err
			}
			files++
			if metadata.subagent {
				excludedFiles++
				subagents[metadata.session] = true
			} else if metadata.responseDialect {
				vscodeFiles++
			}
			return nil
		}); err != nil {
			t.Fatal("metadata audit failed")
		}
	}
	for _, root := range roots {
		if err := NewCodexSource(root).Scan(ctx, func(message Message) error {
			if subagents[message.SessionID] {
				t.Error("explicit subagent session emitted as human")
			}
			if err := message.validate(); err != nil {
				t.Error("invalid human message emitted")
			}
			messages++
			return nil
		}); err != nil {
			t.Fatalf("source audit failed: %v", err)
		}
	}
	if vscodeFiles > 0 && messages == 0 {
		t.Fatal("all human conversations omitted")
	}
	t.Logf("files=%d vscode_human_files=%d excluded_subagent_files=%d emitted_human_messages=%d", files, vscodeFiles, excludedFiles, messages)
}
