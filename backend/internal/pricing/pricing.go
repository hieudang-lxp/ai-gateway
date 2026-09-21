package pricing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Rates struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
}

type Pricing map[string]Rates

var defaultPricing = Pricing{
	"fable":   {Input: 10, Output: 50, CacheRead: 1.0, CacheWrite: 12.5},
	"mythos":  {Input: 10, Output: 50, CacheRead: 1.0, CacheWrite: 12.5},
	"opus":    {Input: 5, Output: 25, CacheRead: 0.5, CacheWrite: 6.25},
	"sonnet":  {Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75},
	"haiku":   {Input: 1, Output: 5, CacheRead: 0.1, CacheWrite: 1.25},
	"default": {Input: 5, Output: 25, CacheRead: 0.5, CacheWrite: 6.25},
}

var families = []string{"fable", "mythos", "opus", "sonnet", "haiku"}

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

func (p Pricing) Cost(model string, in, out, cacheRead, cacheWrite int64) float64 {
	r := p.rates(model)
	return (float64(in)*r.Input +
		float64(out)*r.Output +
		float64(cacheRead)*r.CacheRead +
		float64(cacheWrite)*r.CacheWrite) / 1_000_000
}

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
