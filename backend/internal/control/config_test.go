package control

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const sampleYAML = `
budget:
  daily_usd:   {warn: 10, hard: 20}
  weekly_usd:  {warn: 50, hard: 100}
  monthly_usd: {warn: 150, hard: 300}
routing:
  rules:
    - match: "claude-opus-*"
      to: "claude-sonnet-5"
  block:
    - "claude-fable-*"
cache:
  enabled: true
  ttl: 1h
`

func writeCfg(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "gateway.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadConfig(t *testing.T) {
	cfg, err := LoadConfig(writeCfg(t, sampleYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Budget.Daily != (Limit{10, 20}) || cfg.Budget.Monthly != (Limit{150, 300}) {
		t.Fatalf("budget: %+v", cfg.Budget)
	}
	if len(cfg.Routing.Rules) != 1 || cfg.Routing.Rules[0] != (Rule{"claude-opus-*", "claude-sonnet-5"}) {
		t.Fatalf("rules: %+v", cfg.Routing.Rules)
	}
	if len(cfg.Routing.Block) != 1 || cfg.Routing.Block[0] != "claude-fable-*" {
		t.Fatalf("block: %+v", cfg.Routing.Block)
	}
	if !cfg.Cache.Enabled || cfg.Cache.TTL != time.Hour {
		t.Fatalf("cache: %+v", cfg.Cache)
	}
}

func TestLoadConfigBadTTL(t *testing.T) {
	if _, err := LoadConfig(writeCfg(t, "cache:\n  enabled: true\n  ttl: banana\n")); err == nil {
		t.Fatal("want error for bad ttl")
	}
}

func TestWatcherMissingFileIsZeroConfig(t *testing.T) {
	w := NewWatcher(filepath.Join(t.TempDir(), "nope.yaml"))
	cfg := w.Current()
	if cfg == nil || cfg.Cache.Enabled || len(cfg.Routing.Rules) != 0 || cfg.Budget.Daily.Hard != 0 {
		t.Fatalf("want zero config, got %+v", cfg)
	}
}

func TestWatcherReloadsOnChange(t *testing.T) {
	p := writeCfg(t, "cache:\n  enabled: false\n")
	w := NewWatcher(p)
	w.recheck = 0
	if w.Current().Cache.Enabled {
		t.Fatal("want disabled")
	}
	if err := os.WriteFile(p, []byte("cache:\n  enabled: true\n  ttl: 5m\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(p, future, future); err != nil {
		t.Fatal(err)
	}
	if !w.Current().Cache.Enabled {
		t.Fatal("want reload to pick up enabled=true")
	}
}

func TestWatcherKeepsOldConfigOnParseError(t *testing.T) {
	p := writeCfg(t, "cache:\n  enabled: true\n  ttl: 5m\n")
	w := NewWatcher(p)
	w.recheck = 0
	if !w.Current().Cache.Enabled {
		t.Fatal("want enabled")
	}
	if err := os.WriteFile(p, []byte(":::not yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(p, future, future)
	if !w.Current().Cache.Enabled {
		t.Fatal("fail-open: keep last good config on parse error")
	}
}
