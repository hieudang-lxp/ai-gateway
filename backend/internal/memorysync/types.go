// Package memorysync owns opt-in, content-bearing delivery to the independent
// Graphiti ingestion API. It does not publish transcript text on the usage bus.
package memorysync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type Message struct {
	Source    string `json:"source"`
	SessionID string `json:"session_id"`
	UUID      string `json:"uuid"`
	Speaker   string `json:"speaker"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
	GroupID   string `json:"group_id"`
}

// Source scans a read-only snapshot. Repeated scans are safe: the sender's own
// database keeps durable identity receipts. An emit error must be propagated.
type Source interface {
	Name() string
	Scan(context.Context, func(Message) error) error
}

func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

func (m Message) validate() error {
	if m.Source != "claude_code" && m.Source != "codex" && m.Source != "cursor" {
		return errors.New("unsupported message source")
	}
	if m.SessionID == "" || m.UUID == "" || m.GroupID == "" || m.Speaker != "user" || strings.TrimSpace(m.Text) == "" {
		return errors.New("message missing required human identity or text")
	}
	if _, err := time.Parse(time.RFC3339Nano, m.Timestamp); err != nil {
		return errors.New("message timestamp is missing or invalid")
	}
	return nil
}

func humanText(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, prefix := range []string{"<task-notification>", "[SYSTEM NOTIFICATION", "<system-reminder>", "caveat:", "Caveat:", "<command-", "<local-command", "<user-memory-input>", "Base directory for this skill:", "<user-prompt-submit-hook>", "<environment_context>", "<permissions instructions>", "<collaboration_mode>", "<turn_aborted>", "<subagent_notification>", "# AGENTS.md instructions for ", "<app-context>"} {
		if strings.HasPrefix(text, prefix) {
			return false
		}
	}
	return true
}
