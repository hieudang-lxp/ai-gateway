package control

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Limit struct {
	Warn float64 `yaml:"warn"`
	Hard float64 `yaml:"hard"`
}

type BudgetConfig struct {
	Daily   Limit `yaml:"daily_usd"`
	Weekly  Limit `yaml:"weekly_usd"`
	Monthly Limit `yaml:"monthly_usd"`
}

type Rule struct {
	Match string `yaml:"match"`
	To    string `yaml:"to"`
}

type RoutingConfig struct {
	Rules []Rule   `yaml:"rules"`
	Block []string `yaml:"block"`
}

type CacheConfig struct {
	Enabled bool          `yaml:"enabled"`
	TTL     time.Duration `yaml:"-"`
	TTLRaw  string        `yaml:"ttl"`
}

type Config struct {
	Budget  BudgetConfig  `yaml:"budget"`
	Routing RoutingConfig `yaml:"routing"`
	Cache   CacheConfig   `yaml:"cache"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Cache.TTLRaw != "" {
		ttl, err := time.ParseDuration(cfg.Cache.TTLRaw)
		if err != nil {
			return nil, fmt.Errorf("cache.ttl: %w", err)
		}
		cfg.Cache.TTL = ttl
	}
	return &cfg, nil
}

type Watcher struct {
	path    string
	recheck time.Duration

	mu      sync.Mutex
	cfg     *Config
	mtime   time.Time
	statted time.Time
}

func NewWatcher(path string) *Watcher {
	w := &Watcher{path: path, recheck: 2 * time.Second, cfg: &Config{}}
	w.reload()
	return w
}

func (w *Watcher) Current() *Config {
	w.mu.Lock()
	defer w.mu.Unlock()
	if time.Since(w.statted) >= w.recheck {
		w.reloadLocked()
	}
	return w.cfg
}

func (w *Watcher) reload() { w.mu.Lock(); defer w.mu.Unlock(); w.reloadLocked() }

func (w *Watcher) reloadLocked() {
	w.statted = time.Now()
	fi, err := os.Stat(w.path)
	if err != nil {
		return
	}
	if fi.ModTime().Equal(w.mtime) {
		return
	}
	cfg, err := LoadConfig(w.path)
	if err != nil {
		log.Printf("config reload failed, keeping previous: %v", err)
		w.mtime = fi.ModTime()
		return
	}
	w.cfg = cfg
	w.mtime = fi.ModTime()
	log.Printf("config loaded from %s", w.path)
}
