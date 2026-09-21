package proxy

import (
	"bytes"
	"encoding/json"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

type apiUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

type parser struct {
	sse   bool
	buf   []byte
	model string
	u     store.Usage

	gotOutput bool
}

func newParser(sse bool) *parser { return &parser{sse: sse} }

func (p *parser) feed(b []byte) {
	defer func() { _ = recover() }()
	p.buf = append(p.buf, b...)
	if !p.sse {
		return
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

func (p *parser) finalize() {
	defer func() { _ = recover() }()
	if p.sse {
		if len(p.buf) > 0 {
			p.handleSSELine(p.buf)
			p.buf = nil
		}
		return
	}

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

				p.applyUsage(*ev.Message.Usage, false)
			}
		}
	case "message_delta":
		if ev.Usage != nil {

			p.applyUsage(*ev.Usage, true)
		}
	}
}

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
