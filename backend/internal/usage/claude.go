package usage

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/pricing"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

func ParseClaude(r io.Reader, prices pricing.Pricing) ([]store.ExternalUsage, error) {
	byID := map[string]store.ExternalUsage{}
	order := []string{}
	err := lines(r, func(line []byte) error {
		if !bytes.Contains(line, []byte(`"usage"`)) {
			return nil
		}
		var e struct {
			Type    string    `json:"type"`
			TS      time.Time `json:"timestamp"`
			Message struct {
				ID    string `json:"id"`
				Model string `json:"model"`
				Usage *struct {
					Input         int64 `json:"input_tokens"`
					Output        int64 `json:"output_tokens"`
					Read          int64 `json:"cache_read_input_tokens"`
					Write         int64 `json:"cache_creation_input_tokens"`
					CacheCreation struct {
						OneHour int64 `json:"ephemeral_1h_input_tokens"`
					} `json:"cache_creation"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(line, &e); err != nil {
			return err
		}
		m := e.Message
		if e.Type != "assistant" || m.Usage == nil || m.ID == "" || m.Model == "<synthetic>" {
			return nil
		}
		u := m.Usage
		record := store.ExternalUsage{Source: "claude_code", ID: m.ID, TS: e.TS, Model: m.Model, Usage: store.Usage{Input: u.Input, Output: u.Output, CacheRead: u.Read, CacheWrite: u.Write}}
		// Use configured rates only for known Claude families; never price arbitrary
		// Codex/Cursor model names through the proxy's default fallback.
		_, known := prices[m.Model]
		for _, family := range []string{"opus", "sonnet", "haiku", "fable", "mythos"} {
			if strings.Contains(m.Model, family) {
				if _, ok := prices[family]; ok {
					known = true
				}
			}
		}
		if known {
			cost := prices.Cost(m.Model, u.Input, u.Output, u.Read, u.Write)
			// Anthropic 1h cache writes are 2x input, versus 1.25x for 5m.
			if u.CacheCreation.OneHour > 0 {
				cost += prices.Cost(m.Model, u.CacheCreation.OneHour, 0, 0, 0) * 0.75
			}
			record.CostUSD = &cost
			record.CostKind = "estimated"
		}
		old, exists := byID[m.ID]
		if !exists {
			order = append(order, m.ID)
		} else {
			record.TS = old.TS
			if old.Usage.Output > record.Usage.Output {
				return nil
			}
		}
		byID[m.ID] = record
		return nil
	})
	out := make([]store.ExternalUsage, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out, err
}
