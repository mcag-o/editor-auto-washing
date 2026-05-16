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

	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			_ = cfg.ResolveLLMRuntime()
			l.current = cfg
			l.mu.Unlock()
			return cfg, nil
		}
		l.mu.Unlock()
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		l.mu.Unlock()
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	applyDefaults(&cfg)
	if err := cfg.ResolveLLMRuntime(); err != nil {
		l.mu.Unlock()
		return Config{}, fmt.Errorf("config validation failed: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		l.mu.Unlock()
		return Config{}, fmt.Errorf("config validation failed: %w", err)
	}

	oldConfig := l.current
	l.current = cfg
	listeners := append([]func(old, new Config, changes Changes){}, l.listeners...)
	l.mu.Unlock()

	if !isZeroConfig(oldConfig) {
		changelog, _ := Diff(oldConfig, cfg)
		for _, listener := range listeners {
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

func (l *Loader) Get() Config {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.current
}

func (l *Loader) SetCurrent(cfg Config) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.current = cfg
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

	if _, err := os.Stat(l.path); err == nil {
		backupPath := l.path + ".bak"
		if data, readErr := os.ReadFile(l.path); readErr == nil {
			os.WriteFile(backupPath, data, 0o644)
		}
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
	if cfg.LLM.DefaultProfile == "" {
		cfg.LLM.DefaultProfile = def.LLM.DefaultProfile
	}
	cfg.LLM.Profiles = mergeLLMProfiles(def.LLM.Profiles, cfg.LLM.Profiles)
	if cfg.LLM.Provider == "" {
		cfg.LLM.Provider = def.LLM.Provider
	}
	if cfg.LLM.BaseURL == "" {
		cfg.LLM.BaseURL = def.LLM.BaseURL
	}
	if cfg.LLM.Model == "" {
		cfg.LLM.Model = def.LLM.Model
	}
	if cfg.LLM.Temperature == 0 {
		cfg.LLM.Temperature = def.LLM.Temperature
	}
	if cfg.LLM.MaxTokens == 0 {
		cfg.LLM.MaxTokens = def.LLM.MaxTokens
	}
	if cfg.LLM.TimeoutSec == 0 {
		cfg.LLM.TimeoutSec = def.LLM.TimeoutSec
	}
	if cfg.Template.PromptDir == "" {
		cfg.Template.PromptDir = def.Template.PromptDir
	}
	if cfg.Template.DefaultPrompt == "" {
		cfg.Template.DefaultPrompt = def.Template.DefaultPrompt
	}
}

func isZeroConfig(cfg Config) bool {
	return cfg.HTTP.Host == "" && cfg.HTTP.Port == 0
}

func mergeLLMProfiles(defaults map[string]LLMProfileDef, overrides map[string]LLMProfileDef) map[string]LLMProfileDef {
	if len(defaults) == 0 && len(overrides) == 0 {
		return nil
	}
	merged := make(map[string]LLMProfileDef, len(defaults)+len(overrides))
	for key, value := range defaults {
		merged[key] = value
	}
	for key, value := range overrides {
		base, ok := merged[key]
		if !ok {
			merged[key] = value
			continue
		}
		merged[key] = mergeLLMProfile(base, value)
	}
	return merged
}
func mergeLLMProfile(base LLMProfileDef, override LLMProfileDef) LLMProfileDef {
	merged := base
	if override.Provider != "" {
		merged.Provider = override.Provider
	}
	if override.APIKey != "" {
		merged.APIKey = override.APIKey
	}
	if override.APIKeyRef != "" {
		merged.APIKeyRef = override.APIKeyRef
	}
	if override.BaseURL != "" {
		merged.BaseURL = override.BaseURL
	}
	if override.BaseURLRef != "" {
		merged.BaseURLRef = override.BaseURLRef
	}
	if override.Model != "" {
		merged.Model = override.Model
	}
	if override.Temperature != 0 {
		merged.Temperature = override.Temperature
	}
	if override.MaxTokens != 0 {
		merged.MaxTokens = override.MaxTokens
	}
	if override.TimeoutSec != 0 {
		merged.TimeoutSec = override.TimeoutSec
	}
	return merged
}
