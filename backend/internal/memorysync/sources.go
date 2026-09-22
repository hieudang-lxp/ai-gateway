package memorysync

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type JSONLSource struct{ root, name string }

func NewClaudeSource(root string) *JSONLSource { return &JSONLSource{root, "claude_code"} }
func NewCodexSource(root string) *JSONLSource  { return &JSONLSource{root, "codex"} }
func (s *JSONLSource) Name() string            { return s.name }

func (s *JSONLSource) Scan(ctx context.Context, emit func(Message) error) error {
	info, err := os.Stat(s.root)
	if err != nil {
		return fmt.Errorf("source directory unavailable: %w", err)
	}
	if !info.IsDir() {
		return errors.New("source path is not a directory")
	}
	var failures []error
	err = filepath.WalkDir(s.root, func(path string, entry fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			failures = append(failures, errors.New("source file inaccessible"))
			return nil
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		// Index/metadata JSONL files do not contain conversations.
		if filepath.Base(path) == "session_index.jsonl" {
			return nil
		}
		if parseErr := s.readFile(ctx, path, emit); parseErr != nil {
			failures = append(failures, parseErr)
		}
		return nil
	})
	if err != nil {
		failures = append(failures, err)
	}
	return summarizeSourceErrors(failures)
}

func summarizeSourceErrors(failures []error) error {
	if len(failures) == 0 {
		return nil
	}
	counts := map[string]int{}
	var ordered []string
	for _, failure := range failures {
		message := failure.Error()
		if counts[message] == 0 {
			ordered = append(ordered, message)
		}
		counts[message]++
	}
	var summary []string
	for i, message := range ordered {
		if i == 10 {
			summary = append(summary, fmt.Sprintf("%d additional distinct source errors", len(ordered)-i))
			break
		}
		if counts[message] > 1 {
			message = fmt.Sprintf("%s (%d files)", message, counts[message])
		}
		summary = append(summary, message)
	}
	return errors.New(strings.Join(summary, "; "))
}

// Complete JSON records are snapshots. An incomplete trailing record is retried
// on the next scan without advancing any durable cursor. Corrupt complete lines
// are reported without hiding later independently valid messages.
func readRecords(ctx context.Context, path string, visit func(int, []byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.New("source file cannot be opened")
	}
	defer f.Close()
	reader := bufio.NewReaderSize(f, 64*1024)
	var failures []error
	for index := 0; ; index++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var line []byte
		atEOF := false
		for {
			chunk, e := reader.ReadSlice('\n')
			if len(line)+len(chunk) > 16*1024*1024 {
				return errors.New("source record exceeds 16 MiB limit")
			}
			line = append(line, chunk...)
			if e == bufio.ErrBufferFull {
				continue
			}
			if e == io.EOF {
				atEOF = true
				break
			}
			if e != nil {
				return errors.New("source file read failed")
			}
			break
		}
		if atEOF && !json.Valid(line) {
			return errors.Join(failures...)
		}
		if strings.TrimSpace(string(line)) == "" {
			continue
		}
		if err := visit(index, line); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Bound status output even for extensively corrupt files.
			if len(failures) < 10 {
				failures = append(failures, fmt.Errorf("record %d: %w", index+1, err))
			}
		}
		if atEOF {
			return errors.Join(failures...)
		}
	}
}

type record struct {
	Type        string `json:"type"`
	UUID        string `json:"uuid"`
	SessionID   string `json:"sessionId"`
	Timestamp   string `json:"timestamp"`
	IsMeta      bool   `json:"isMeta"`
	IsSidechain bool   `json:"isSidechain"`
	Message     struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	Payload struct {
		Type    string          `json:"type"`
		ID      string          `json:"id"`
		Role    string          `json:"role"`
		Message string          `json:"message"`
		Content json.RawMessage `json:"content"`
	} `json:"payload"`
}

func contentText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var texts []string
	for _, part := range parts {
		if part.Type == "text" || part.Type == "input_text" {
			texts = append(texts, part.Text)
		}
	}
	return strings.TrimSpace(strings.Join(texts, " "))
}

func (s *JSONLSource) readFile(ctx context.Context, path string, emit func(Message) error) error {
	session := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	responseDialect := false
	if s.name == "codex" {
		metadata, err := inspectCodexMetadata(ctx, path)
		if err != nil {
			return err
		}
		if metadata.subagent {
			return nil
		}
		if metadata.session != "" {
			session = metadata.session
		}
		responseDialect = metadata.responseDialect
	}
	eventCount, responseCount := 0, 0
	occurrences := map[string]int{}
	err := readRecords(ctx, path, func(index int, line []byte) error {
		var r record
		if json.Unmarshal(line, &r) != nil {
			return errors.New("invalid JSON")
		}
		m := Message{Source: s.name, SessionID: session, Speaker: "user", Timestamp: r.Timestamp, GroupID: "personal"}
		if s.name == "claude_code" {
			if r.SessionID != "" {
				session = r.SessionID
				m.SessionID = session
			}
			if r.Type != "user" || r.IsMeta || r.IsSidechain {
				return nil
			}
			m.Text = contentText(r.Message.Content)
			if !humanText(m.Text) {
				return nil
			}
			m.UUID = r.UUID
			if m.UUID == "" {
				m.UUID = "sha256:" + digest(session+"\n"+strconv.Itoa(index)+"\n"+m.Text)
			}
		} else {
			if r.Type == "session_meta" && r.Payload.ID != "" {
				session = r.Payload.ID
				return nil
			}
			if r.Type == "response_item" && r.Payload.Role == "user" {
				m.Text = codexResponseText(r.Payload.Content)
				if !humanText(m.Text) {
					return nil
				}
				responseCount++
				if !responseDialect {
					return nil
				}
				m.UUID = r.Payload.ID
				if m.UUID == "" {
					m.UUID = "sha256:" + digest(session+"\nresponse_item\n"+strconv.Itoa(index)+"\n"+m.Text)
				}
			} else {
				if r.Type != "event_msg" || r.Payload.Type != "user_message" {
					return nil
				}
				m.Text = strings.TrimSpace(r.Payload.Message)
				if !humanText(m.Text) {
					return nil
				}
				eventCount++
				if responseDialect {
					return nil
				}
				m.UUID = r.Payload.ID
				if m.UUID == "" {
					key := session + "\n" + r.Timestamp + "\n" + m.Text
					occurrence := occurrences[key]
					occurrences[key]++
					m.UUID = "sha256:" + digest(key+"\n"+strconv.Itoa(occurrence))
				}
			}
			m.SessionID = session
		}
		if err := m.validate(); err != nil {
			return err
		}
		return emit(m)
	})
	if err != nil {
		return err
	}
	if s.name == "codex" && !responseDialect && eventCount == 0 && responseCount > 0 {
		return errors.New("unsupported Codex response-only transcript: authoritative user_message events unavailable")
	}
	if responseDialect && responseCount == 0 && eventCount > 0 {
		return errors.New("unsupported Codex vscode event-only transcript: primary response_item human messages unavailable")
	}
	return nil
}
