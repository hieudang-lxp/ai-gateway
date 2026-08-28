package proxy

import (
	"bytes"
	"encoding/json"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

// apiUsage mirrors the fields of the Anthropic `usage` object we care about.
type apiUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

// parser extracts model + usage from a response stream as bytes flow through.
//
// It is fed raw bytes via feed() (cheap: buffer + line scan for SSE) and
// finalized via finalize() once the body is fully read. All parse work is
// wrapped in recover() so a malformed event can never crash the proxy or break
// the client's stream.
type parser struct {
	sse   bool
	buf   []byte
	model string
	u     store.Usage
	// gotOutput tracks whether we've seen a message_delta output count so a
	// later, smaller value can't clobber the cumulative final.
	gotOutput bool
}

func newParser(sse bool) *parser { return &parser{sse: sse} }

// feed appends bytes and, for SSE, drains any complete lines. It must stay
// cheap and never error — it runs on the client's read path.
func (p *parser) feed(b []byte) {
	defer func() { _ = recover() }()
	p.buf = append(p.buf, b...)
	if !p.sse {
		return // JSON: parse the whole body once, at finalize
	}
	for {
		i := bytes.IndexByte(p.buf, '\n')
		if i < 0 {
			break
		}
		line := p.buf[:i]
		p.buf = p.buf[i+1:]
		p.handleSSELine(line)
	}
}

// finalize parses whatever remains once the body is closed.
func (p *parser) finalize() {
	defer func() { _ = recover() }()
	if p.sse {
		if len(p.buf) > 0 {
			p.handleSSELine(p.buf)
			p.buf = nil
		}
		return
	}
	// Non-streaming: the whole body is one JSON message.
	var m struct {
		Model string    `json:"model"`
		Usage *apiUsage `json:"usage"`
	}
	if json.Unmarshal(p.buf, &m) == nil {
		if m.Model != "" {
			p.model = m.Model
		}
		if m.Usage != nil {
			p.applyUsage(*m.Usage, true)
		}
	}
}

type sseEvent struct {
	Type    string `json:"type"`
	Message *struct {
		Model string    `json:"model"`
		Usage *apiUsage `json:"usage"`
	} `json:"message"`
	Usage *apiUsage `json:"usage"`
}

func (p *parser) handleSSELine(line []byte) {
	line = bytes.TrimSpace(line)
	if !bytes.HasPrefix(line, []byte("data:")) {
		return
	}
	data := bytes.TrimSpace(line[len("data:"):])
	if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
		return
	}
	var ev sseEvent
	if json.Unmarshal(data, &ev) != nil {
		return
	}
	switch ev.Type {
	case "message_start":
		if ev.Message != nil {
			if ev.Message.Model != "" {
				p.model = ev.Message.Model
			}
			if ev.Message.Usage != nil {
				// message_start carries input + cache tokens (and a token or
				// two of output); take input/cache from here.
				p.applyUsage(*ev.Message.Usage, false)
			}
		}
	case "message_delta":
		if ev.Usage != nil {
			// message_delta carries the cumulative final output_tokens.
			p.applyUsage(*ev.Usage, true)
		}
	}
}

// applyUsage merges an apiUsage into the accumulator. withOutput indicates the
// event is authoritative for output_tokens (message_delta / non-stream body).
func (p *parser) applyUsage(a apiUsage, withOutput bool) {
	if a.InputTokens > 0 {
		p.u.Input = a.InputTokens
	}
	if a.CacheReadInputTokens > 0 {
		p.u.CacheRead = a.CacheReadInputTokens
	}
	if a.CacheCreationInputTokens > 0 {
		p.u.CacheWrite = a.CacheCreationInputTokens
	}
	if withOutput && a.OutputTokens > 0 {
		p.u.Output = a.OutputTokens
		p.gotOutput = true
	} else if !p.gotOutput && a.OutputTokens > 0 {
		p.u.Output = a.OutputTokens
	}
}
