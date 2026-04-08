package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type Validator interface {
	Validate() error
}

type Loader struct {
	mu        sync.RWMutex
	path      string
	current   Config
	listeners []func(old, new Config, changes Changes)
}

func NewLoader(path string) *Loader {
	return &Loader{
		path: path,
	}
}

func (l *Loader) Load() (Config, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			cfg.ResolveSecrets()
			l.current = cfg
			return cfg, nil
		}
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	applyDefaults(&cfg)
	cfg.ResolveSecrets()

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("config validation failed: %w", err)
	}

	oldConfig := l.current
	l.current = cfg

	if !isZeroConfig(oldConfig) {
		changelog, _ := Diff(oldConfig, cfg)
		for _, listener := range l.listeners {
			listener(oldConfig, cfg, changelog)
		}
	}

	return cfg, nil
}

func (l *Loader) Save(cfg Config) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.saveLocked(cfg)
}

func (l *Loader) Reload() (Config, error) {
	return l.Load()
}

func (l *Loader) Current() Config {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.current
}

func (l *Loader) OnChange(fn func(old, new Config, changes Changes)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.listeners = append(l.listeners, fn)
}

func (l *Loader) SetPath(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.path = path
}

func (l *Loader) saveLocked(cfg Config) error {
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	success := false
	defer func() {
		tmpFile.Close()
		if !success {
			os.Remove(tmpPath)
		}
	}()

	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, l.path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	success = true

	if runtime.GOOS == "windows" {
		return nil
	}

	if err := os.Chmod(l.path, 0o600); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	return nil
}

func applyDefaults(cfg *Config) {
	def := DefaultConfig()
	if cfg.HTTP.Host == "" {
		cfg.HTTP.Host = def.HTTP.Host
	}
	if cfg.HTTP.Port == 0 {
		cfg.HTTP.Port = def.HTTP.Port
	}
	if cfg.HTTP.ReadTimeoutSec == 0 {
		cfg.HTTP.ReadTimeoutSec = def.HTTP.ReadTimeoutSec
	}
	if cfg.HTTP.WriteTimeoutSec == 0 {
		cfg.HTTP.WriteTimeoutSec = def.HTTP.WriteTimeoutSec
	}
	if cfg.HTTP.ShutdownSec == 0 {
		cfg.HTTP.ShutdownSec = def.HTTP.ShutdownSec
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = def.Log.Level
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = def.Log.Format
	}
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = def.Database.Driver
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = def.Database.Path
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = def.Database.MaxOpenConns
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = def.Database.MaxIdleConns
	}
	if cfg.Database.ConnMaxLifeMin == 0 {
		cfg.Database.ConnMaxLifeMin = def.Database.ConnMaxLifeMin
	}
	if cfg.Storage.BasePath == "" {
		cfg.Storage.BasePath = def.Storage.BasePath
	}
	if cfg.Storage.MaxSizeMB == 0 {
		cfg.Storage.MaxSizeMB = def.Storage.MaxSizeMB
	}
	if cfg.Workflow.MaxConcurrentJobs == 0 {
		cfg.Workflow.MaxConcurrentJobs = def.Workflow.MaxConcurrentJobs
	}
	if cfg.Workflow.RetryMaxAttempts == 0 {
		cfg.Workflow.RetryMaxAttempts = def.Workflow.RetryMaxAttempts
	}
	if cfg.Workflow.TimeoutSec == 0 {
		cfg.Workflow.TimeoutSec = def.Workflow.TimeoutSec
	}
	if cfg.Secrets.EnvPrefix == "" {
		cfg.Secrets.EnvPrefix = def.Secrets.EnvPrefix
	}
}

func isZeroConfig(cfg Config) bool {
	return cfg.HTTP.Host == "" && cfg.HTTP.Port == 0
}
