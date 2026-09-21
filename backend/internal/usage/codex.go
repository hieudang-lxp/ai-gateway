package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

type tokenUsage struct {
	Input  int64 `json:"input_tokens"`
	Cached int64 `json:"cached_input_tokens"`
	Write  int64 `json:"cache_write_input_tokens"`
	Output int64 `json:"output_tokens"`
}

func (u tokenUsage) normalized() (store.Usage, error) {
	if u.Input < 0 || u.Cached < 0 || u.Write < 0 || u.Output < 0 || u.Cached+u.Write > u.Input {
		return store.Usage{}, fmt.Errorf("invalid Codex token counters")
	}
	return store.Usage{Input: u.Input - u.Cached - u.Write, Output: u.Output, CacheRead: u.Cached, CacheWrite: u.Write}, nil
}

func lines(r io.Reader, fn func([]byte) error) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err = fn(line); err != nil {
			return err
		}
	}
}

func ParseCodex(r io.Reader) ([]store.ExternalUsage, error) {
	var direct, legacy []store.ExternalUsage
	var session, model, project, title string
	var previous tokenUsage
	aliases := map[string]bool{}
	legacyID := func(total tokenUsage) string {
		return fmt.Sprintf("legacy:%s:%d:%d:%d:%d", session, total.Input, total.Cached, total.Write, total.Output)
	}
	err := lines(r, func(line []byte) error {
		if !bytes.Contains(line, []byte(`"token_usage_record"`)) && !bytes.Contains(line, []byte(`"token_count"`)) && !bytes.Contains(line, []byte(`"session_meta"`)) && !bytes.Contains(line, []byte(`"turn_context"`)) {
			return nil
		}
		var e struct {
			Timestamp time.Time       `json:"timestamp"`
			Type      string          `json:"type"`
			Payload   json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(line, &e); err != nil {
			return fmt.Errorf("invalid Codex JSON: %w", err)
		}
		switch e.Type {
		case "session_meta":
			var p struct {
				ID    string `json:"id"`
				CWD   string `json:"cwd"`
				Title string `json:"title"`
			}
			if err := json.Unmarshal(e.Payload, &p); err != nil {
				return err
			}
			session = p.ID
			project, title = metadataText(p.CWD, 4096), metadataText(p.Title, 512)
		case "turn_context":
			var p struct {
				Model string `json:"model"`
				CWD   string `json:"cwd"`
			}
			if err := json.Unmarshal(e.Payload, &p); err != nil {
				return err
			}
			model = p.Model
			if p.CWD != "" {
				project = metadataText(p.CWD, 4096)
			}
		case "token_usage_record":
			var p struct {
				ResponseID string      `json:"response_id"`
				ThreadID   string      `json:"thread_id"`
				Usage      tokenUsage  `json:"usage"`
				Total      *tokenUsage `json:"thread_token_usage"`
			}
			if err := json.Unmarshal(e.Payload, &p); err != nil {
				return err
			}
			if p.ResponseID == "" {
				return fmt.Errorf("Codex usage missing response ID")
			}
			u, err := p.Usage.normalized()
			if err != nil {
				return err
			}
			record := store.ExternalUsage{Source: "codex", ID: p.ResponseID, TS: e.Timestamp, Model: model, Usage: u}
			record.SessionID, record.Project, record.SessionTitle = session, project, title
			if p.ThreadID != "" && p.ThreadID != session {

				record.SessionID, record.Project, record.SessionTitle = p.ThreadID, "", ""
			}
			if p.Total != nil && session != "" {
				alias := legacyID(*p.Total)
				record.Aliases = []string{alias}
				aliases[alias] = true
			}
			direct = append(direct, record)
		case "event_msg":
			var p struct {
				Type string `json:"type"`
				Info *struct {
					Total tokenUsage `json:"total_token_usage"`
				} `json:"info"`
			}
			if err := json.Unmarshal(e.Payload, &p); err != nil {
				return err
			}
			if p.Type != "token_count" || p.Info == nil {
				return nil
			}
			total := p.Info.Total
			if total == previous {
				return nil
			}
			delta := tokenUsage{Input: total.Input - previous.Input, Cached: total.Cached - previous.Cached, Write: total.Write - previous.Write, Output: total.Output - previous.Output}
			if delta.Input < 0 || delta.Output < 0 || delta.Cached < 0 || delta.Write < 0 {
				delta = total
			}
			previous = total
			u, err := delta.normalized()
			if err != nil {
				return err
			}
			if session == "" {
				return fmt.Errorf("Codex usage missing session ID")
			}
			id := legacyID(total)
			legacy = append(legacy, store.ExternalUsage{Source: "codex", ID: id, TS: e.Timestamp, Model: model, Usage: u, SessionID: session, Project: project, SessionTitle: title})
		}
		return nil
	})
	firstDirect := time.Time{}
	for _, row := range direct {
		if firstDirect.IsZero() || row.TS.Before(firstDirect) {
			firstDirect = row.TS
		}
	}
	for _, row := range legacy {
		if aliases[row.ID] {
			continue
		}

		if !firstDirect.IsZero() && !row.TS.Before(firstDirect) {
			direct[0].Aliases = append(direct[0].Aliases, row.ID)
			continue
		}

		duplicate := false
		for _, d := range direct {
			if d.TS.Equal(row.TS) && d.Usage == row.Usage {
				duplicate = true
				break
			}
		}
		if !duplicate {
			direct = append(direct, row)
		}
	}
	return direct, err
}
