package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const CatalogURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

// TokenRates uses USD per token, as published in LiteLLM's public catalog.
type TokenRates map[string]float64
type CatalogSnapshot struct {
	Models    map[string]TokenRates `json:"models"`
	UpdatedAt time.Time             `json:"updated_at"`
	Error     string                `json:"error,omitempty"`
}
type Catalog struct {
	mu             sync.RWMutex
	snapshot       CatalogSnapshot
	path, endpoint string
}

func NewCatalog(path string) *Catalog {
	// Verified official standard API rates, 2026-09-17. Used only before first fetch.
	models := map[string]TokenRates{}
	for name, r := range map[string]Rates{"gpt-6-astra": {10, 50, 1, 12.5}, "gpt-5.6-sol": {4, 20, 0.4, 5}} {
		models[name] = TokenRates{"input_cost_per_token": r.Input / 1e6, "output_cost_per_token": r.Output / 1e6, "cache_read_input_token_cost": r.CacheRead / 1e6, "cache_creation_input_token_cost": r.CacheWrite / 1e6, "input_cost_per_token_above_272k_tokens": r.Input * 2 / 1e6, "output_cost_per_token_above_272k_tokens": r.Output * 1.5 / 1e6, "cache_read_input_token_cost_above_272k_tokens": r.CacheRead * 2 / 1e6, "cache_creation_input_token_cost_above_272k_tokens": r.CacheWrite * 2 / 1e6}
	}
	c := &Catalog{path: path, endpoint: CatalogURL, snapshot: CatalogSnapshot{Models: models}}
	if data, err := os.ReadFile(path); err == nil {
		var cached CatalogSnapshot
		if json.Unmarshal(data, &cached) == nil && validRates(cached.Models["gpt-5.6-sol"]) {
			c.snapshot = cached
		}
	}
	return c
}
func validRates(r TokenRates) bool {
	return r["input_cost_per_token"] > 0 && r["output_cost_per_token"] > 0 && r["cache_read_input_token_cost"] > 0
}
func (c *Catalog) Snapshot() CatalogSnapshot { c.mu.RLock(); defer c.mu.RUnlock(); return c.snapshot }
func (c *Catalog) Run(ctx context.Context) {
	c.Refresh(ctx)
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.Refresh(ctx)
		}
	}
}
func (c *Catalog) Refresh(ctx context.Context) {
	snapshot, err := c.fetch(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.snapshot.Error = err.Error()
		return
	}
	c.snapshot = snapshot
	if c.path != "" {
		data, _ := json.Marshal(snapshot)
		if err = os.MkdirAll(filepath.Dir(c.path), 0700); err == nil {
			err = os.WriteFile(c.path+".tmp", data, 0600)
		}
		if err == nil {
			err = os.Rename(c.path+".tmp", c.path)
		}
		if err != nil {
			c.snapshot.Error = "prices refreshed; disk cache write failed"
		}
	}
}
func (c *Catalog) fetch(ctx context.Context) (CatalogSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return CatalogSnapshot{}, err
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return CatalogSnapshot{}, fmt.Errorf("price refresh failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return CatalogSnapshot{}, fmt.Errorf("price source HTTP %d", resp.StatusCode)
	}
	var raw map[string]map[string]json.RawMessage
	if err = json.NewDecoder(io.LimitReader(resp.Body, 20<<20)).Decode(&raw); err != nil {
		return CatalogSnapshot{}, fmt.Errorf("invalid price catalog")
	}
	models := map[string]TokenRates{}
	for name, fields := range raw {
		rates := TokenRates{}
		for key, value := range fields {
			var n float64
			if json.Unmarshal(value, &n) == nil && n >= 0 {
				rates[key] = n
			}
		}
		if validRates(rates) {
			models[name] = rates
		}
	}
	if !validRates(models["gpt-5.6-sol"]) {
		return CatalogSnapshot{}, fmt.Errorf("price catalog missing fallback model; retaining previous prices")
	}
	return CatalogSnapshot{Models: models, UpdatedAt: time.Now()}, nil
}

// Cost estimates standard API-equivalent value at the current catalog rates,
// not subscription billing. Each request is priced separately for context tiers.
func (s CatalogSnapshot) Cost(model string, in, out, read, write int64) (float64, bool) {
	rates, ok := s.Models[model]
	fallback := !ok
	if !ok {
		rates = s.Models["gpt-5.6-sol"]
	}
	totalInput := in + read + write
	rate := func(key string) float64 {
		if _, exists := rates[key]; !exists {
			key = "input_cost_per_token"
		}
		// Some models have 128K/200K thresholds instead of 272K.
		for _, tier := range []struct {
			tokens int64
			suffix string
		}{{272000, "_above_272k_tokens"}, {200000, "_above_200k_tokens"}, {128000, "_above_128k_tokens"}} {
			if v, exists := rates[key+tier.suffix]; exists && totalInput > tier.tokens {
				return v
			}
		}
		if v, exists := rates[key]; exists {
			return v
		}
		// Missing cache-write rates use regular input, with assumption disclosed in UI.
		return rates["input_cost_per_token"]
	}
	return float64(in)*rate("input_cost_per_token") + float64(out)*rate("output_cost_per_token") + float64(read)*rate("cache_read_input_token_cost") + float64(write)*rate("cache_creation_input_token_cost"), fallback
}
