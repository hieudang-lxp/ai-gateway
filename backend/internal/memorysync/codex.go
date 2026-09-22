package memorysync

import (
	"context"
	"encoding/json"
	"strings"
)

type codexMetadata struct {
	session                   string
	responseDialect, subagent bool
}

// Metadata is inspected before emitting anything: subagent ancestry can appear
// after copied response history. VSCode uses response_item for human input in
// observed 0.153.4 and 0.154 builds, while child-agent user_message events are
// machine-generated. Choose by source, never by whether an event has arrived in
// the current snapshot: that would duplicate messages across polling boundaries.
func inspectCodexMetadata(ctx context.Context, path string) (codexMetadata, error) {
	var metadata codexMetadata
	err := readRecords(ctx, path, func(_ int, line []byte) error {
		var record struct {
			Type    string `json:"type"`
			Payload struct {
				ID     string          `json:"id"`
				Source json.RawMessage `json:"source"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &record) != nil || record.Type != "session_meta" {
			return nil
		}
		if metadata.session == "" {
			metadata.session = record.Payload.ID
		}
		var source string
		if json.Unmarshal(record.Payload.Source, &source) == nil {
			if source == "vscode" {
				metadata.responseDialect = true
			}
			if source == "subagent" {
				metadata.subagent = true
			}
		} else {
			var source map[string]json.RawMessage
			if json.Unmarshal(record.Payload.Source, &source) == nil {
				if _, ok := source["subagent"]; ok {
					metadata.subagent = true
				}
			}
		}
		return nil
	})
	return metadata, err
}

func codexResponseText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return codexHumanBlock(text)
	}
	var content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &content) != nil {
		return ""
	}
	var blocks []string
	for _, block := range content {
		if block.Type != "input_text" && block.Type != "text" {
			continue
		}
		if text := codexHumanBlock(block.Text); text != "" {
			blocks = append(blocks, text)
		}
	}
	return strings.Join(blocks, " ")
}

func codexHumanBlock(text string) string {
	text = strings.TrimSpace(text)
	// These context blocks are inserted by the desktop harness. Some share one
	// text item with the actual user message, which must survive their removal.
	for {
		changed := false
		for _, tag := range []string{"in-app-browser-context", "environment_context", "recommended_plugins", "timestamp", "image", "image_files", "subagents"} {
			open, close := "<"+tag+">", "</"+tag+">"
			if strings.HasPrefix(text, open) {
				if end := strings.Index(text, close); end >= 0 {
					text = strings.TrimSpace(text[end+len(close):])
					changed = true
					break
				}
			}
			if strings.HasSuffix(text, close) {
				if start := strings.LastIndex(text, open); start >= 0 {
					text = strings.TrimSpace(text[:start])
					changed = true
					break
				}
			}
		}
		if !changed {
			break
		}
	}
	if strings.HasPrefix(text, "<user_query>") && strings.HasSuffix(text, "</user_query>") {
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "<user_query>"), "</user_query>"))
	}
	const replyOpen, replyClose = "<send_user_message_question_reply>", "</send_user_message_question_reply>"
	if strings.HasPrefix(text, replyOpen) && strings.HasSuffix(text, replyClose) {
		var answers []struct {
			Answer string `json:"answer"`
		}
		if json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(text, replyOpen), replyClose)), &answers) != nil {
			return ""
		}
		var values []string
		for _, answer := range answers {
			if strings.TrimSpace(answer.Answer) != "" {
				values = append(values, strings.TrimSpace(answer.Answer))
			}
		}
		text = strings.Join(values, "\n")
	}
	if !humanText(text) {
		return ""
	}
	return text
}
