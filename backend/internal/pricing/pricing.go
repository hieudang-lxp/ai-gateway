package pricing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Rates are US dollars per 1,000,000 tokens for one model family.
//
// Cache pricing follows Anthropic's published multipliers: cache reads cost
// ~0.1x the input rate, and cache writes (5-minute TTL) cost 1.25x. We price
// every cache-write token at the 5m rate — the vast majority of usage — and
// accept a small overcount on the rare 1h-TTL write. Update this table (or the
// on-disk pricing.json) whenever Anthropic changes pricing.
type Rates struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
}

// Pricing maps a lookup key to its rates. Keys are matched in this order:
//  1. exact model ID (e.g. "claude-opus-4-8")
//  2. family substring (e.g. "opus", matched against the model ID)
//  3. "default"
//
// Seeding with family keys means new point releases (opus-4-9, sonnet-6, ...)
// are priced correctly without editing the file.
type Pricing map[string]Rates

// defaultPricing reflects Anthropic list pricing as of 2026-08-27, sourced from
// the claude-api skill's model catalog. Cache rates are derived (input*0.1 read,
// input*1.25 write-5m).
//
// Note: Sonnet 5 has introductory pricing of $2/$10 per MTok through
// 2026-08-31; the standard $3/$15 is used here. Edit pricing.json if you want
// the intro rate reflected for that window.
var defaultPricing = Pricing{
	"fable":   {Input: 10, Output: 50, CacheRead: 1.0, CacheWrite: 12.5},
	"mythos":  {Input: 10, Output: 50, CacheRead: 1.0, CacheWrite: 12.5},
	"opus":    {Input: 5, Output: 25, CacheRead: 0.5, CacheWrite: 6.25},
	"sonnet":  {Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75},
	"haiku":   {Input: 1, Output: 5, CacheRead: 0.1, CacheWrite: 1.25},
	"default": {Input: 5, Output: 25, CacheRead: 0.5, CacheWrite: 6.25},
}

// families is the substring match order for step 2. Longest/most-specific
// families first so "opus" never shadows a more specific key.
var families = []string{"fable", "mythos", "opus", "sonnet", "haiku"}

// rates resolves the rates for a model ID, falling back family -> default.
func (p Pricing) rates(model string) Rates {
	if r, ok := p[model]; ok {
		return r
	}
	m := strings.ToLower(model)
	for _, fam := range families {
		if strings.Contains(m, fam) {
			if r, ok := p[fam]; ok {
				return r
			}
		}
	}
	return p["default"]
}

// Cost returns the estimated USD cost of one call.
func (p Pricing) Cost(model string, in, out, cacheRead, cacheWrite int64) float64 {
	r := p.rates(model)
	return (float64(in)*r.Input +
		float64(out)*r.Output +
		float64(cacheRead)*r.CacheRead +
		float64(cacheWrite)*r.CacheWrite) / 1_000_000
}

// Load reads pricing.json from path, creating it from defaults if it
// doesn't exist. A malformed or unreadable file falls back to defaults (this is
// a cost estimate, never a reason to break the proxy).
func Load(path string) Pricing {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			writeDefaultPricing(path)
		}
		return defaultPricing
	}
	var p Pricing
	if err := json.Unmarshal(data, &p); err != nil || len(p) == 0 {
		return defaultPricing
	}
	if _, ok := p["default"]; !ok {
		p["default"] = defaultPricing["default"]
	}
	return p
}

func writeDefaultPricing(path string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(defaultPricing, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}
