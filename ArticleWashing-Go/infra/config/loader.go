package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
			cfg.ResolveSecrets()
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

	presence, err := collectSourceFieldPresence(data)
	if err != nil {
		l.mu.Unlock()
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	applyDefaults(&cfg, presence)
	cfg.ResolveSecrets()

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

func applyDefaults(cfg *Config, sourcePresence map[string]sourceFieldPresence) {
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
	if cfg.Collector.Defaults.HTTPClient == "" {
		cfg.Collector.Defaults.HTTPClient = def.Collector.Defaults.HTTPClient
	}
	if cfg.Collector.Defaults.RetryPolicy == "" {
		cfg.Collector.Defaults.RetryPolicy = def.Collector.Defaults.RetryPolicy
	}
	if cfg.Collector.Defaults.AuthProfile == "" {
		cfg.Collector.Defaults.AuthProfile = def.Collector.Defaults.AuthProfile
	}
	if cfg.Collector.Defaults.IntervalMins == 0 {
		cfg.Collector.Defaults.IntervalMins = def.Collector.Defaults.IntervalMins
	}
	if cfg.Collector.Defaults.TimeoutMS == 0 {
		cfg.Collector.Defaults.TimeoutMS = def.Collector.Defaults.TimeoutMS
	}
	if cfg.Collector.Defaults.HotlistLimit == 0 {
		cfg.Collector.Defaults.HotlistLimit = def.Collector.Defaults.HotlistLimit
	}
	if cfg.Collector.Defaults.Concurrency == 0 {
		cfg.Collector.Defaults.Concurrency = def.Collector.Defaults.Concurrency
	}
	cfg.Collector.HTTPClients = mergeHTTPClientProfiles(def.Collector.HTTPClients, cfg.Collector.HTTPClients)
	cfg.Collector.RetryPolicies = mergeRetryPolicyProfiles(def.Collector.RetryPolicies, cfg.Collector.RetryPolicies)
	cfg.Collector.AuthProfiles = mergeAuthProfiles(def.Collector.AuthProfiles, cfg.Collector.AuthProfiles)
	cfg.Collector.Sources = mergeCollectorSources(def.Collector.Sources, cfg.Collector.Sources, sourcePresence)
	if cfg.Secrets.EnvPrefix == "" {
		cfg.Secrets.EnvPrefix = def.Secrets.EnvPrefix
	}
	if cfg.LLM.Provider == "" {
		cfg.LLM.Provider = def.LLM.Provider
	}
	if cfg.LLM.Model == "" {
		cfg.LLM.Model = def.LLM.Model
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

func mergeHTTPClientProfiles(defaults map[string]HTTPClientProfile, overrides map[string]HTTPClientProfile) map[string]HTTPClientProfile {
	if len(defaults) == 0 && len(overrides) == 0 {
		return nil
	}
	merged := make(map[string]HTTPClientProfile, len(defaults)+len(overrides))
	for key, value := range defaults {
		merged[key] = cloneHTTPClientProfile(value)
	}
	for key, value := range overrides {
		base, ok := merged[key]
		if !ok {
			merged[key] = cloneHTTPClientProfile(value)
			continue
		}
		merged[key] = mergeHTTPClientProfile(base, value)
	}
	return merged
}

func mergeRetryPolicyProfiles(defaults map[string]RetryPolicyProfile, overrides map[string]RetryPolicyProfile) map[string]RetryPolicyProfile {
	if len(defaults) == 0 && len(overrides) == 0 {
		return nil
	}
	merged := make(map[string]RetryPolicyProfile, len(defaults)+len(overrides))
	for key, value := range defaults {
		merged[key] = value
	}
	for key, value := range overrides {
		base, ok := merged[key]
		if !ok {
			merged[key] = value
			continue
		}
		merged[key] = mergeRetryPolicyProfile(base, value)
	}
	return merged
}

func mergeAuthProfiles(defaults map[string]AuthProfileConfig, overrides map[string]AuthProfileConfig) map[string]AuthProfileConfig {
	if len(defaults) == 0 && len(overrides) == 0 {
		return nil
	}
	merged := make(map[string]AuthProfileConfig, len(defaults)+len(overrides))
	for key, value := range defaults {
		merged[key] = value
	}
	for key, value := range overrides {
		base, ok := merged[key]
		if !ok {
			merged[key] = value
			continue
		}
		merged[key] = mergeAuthProfile(base, value)
	}
	return merged
}

func cloneHTTPClientProfile(profile HTTPClientProfile) HTTPClientProfile {
	cloned := HTTPClientProfile{
		UserAgent: profile.UserAgent,
	}
	if profile.Headers != nil {
		cloned.Headers = make(map[string]string, len(profile.Headers))
		for key, value := range profile.Headers {
			cloned.Headers[key] = value
		}
	}
	return cloned
}

func mergeHTTPClientProfile(base HTTPClientProfile, override HTTPClientProfile) HTTPClientProfile {
	merged := cloneHTTPClientProfile(base)
	if override.UserAgent != "" {
		merged.UserAgent = override.UserAgent
	}
	if override.Headers != nil {
		merged.Headers = make(map[string]string, len(override.Headers))
		for key, value := range override.Headers {
			merged.Headers[key] = value
		}
	}
	if merged.Headers == nil {
		merged.Headers = map[string]string{}
	}
	return merged
}

func mergeRetryPolicyProfile(base RetryPolicyProfile, override RetryPolicyProfile) RetryPolicyProfile {
	merged := base
	if override.MaxAttempts != 0 {
		merged.MaxAttempts = override.MaxAttempts
	}
	if override.BaseWaitMS != 0 {
		merged.BaseWaitMS = override.BaseWaitMS
	}
	if override.MaxWaitMS != 0 {
		merged.MaxWaitMS = override.MaxWaitMS
	}
	return merged
}

func mergeAuthProfile(base AuthProfileConfig, override AuthProfileConfig) AuthProfileConfig {
	merged := base
	if override.Mode != "" {
		merged.Mode = override.Mode
	}
	if override.HeaderName != "" {
		merged.HeaderName = override.HeaderName
	}
	if override.HeaderValuePrefix != "" {
		merged.HeaderValuePrefix = override.HeaderValuePrefix
	}
	return merged
}

type sourceFieldPresence struct {
	boolFields   map[string]bool
	stringFields map[string]bool
}

func collectSourceFieldPresence(data []byte) (map[string]sourceFieldPresence, error) {
	var raw struct {
		Collector struct {
			Sources map[string]map[string]json.RawMessage `json:"sources"`
		} `json:"collector"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if len(raw.Collector.Sources) == 0 {
		return nil, nil
	}
	presence := make(map[string]sourceFieldPresence, len(raw.Collector.Sources))
	for sourceID, fields := range raw.Collector.Sources {
		item := sourceFieldPresence{
			boolFields:   map[string]bool{},
			stringFields: map[string]bool{},
		}
		for fieldName := range fields {
			switch fieldName {
			case "enabled", "schedule_enabled", "detail_fetch_enabled", "supports_article", "placeholder_required":
				item.boolFields[fieldName] = true
			case "auth_mode", "auth_profile":
				item.stringFields[fieldName] = true
			}
		}
		presence[sourceID] = item
	}
	return presence, nil
}

func mergeCollectorSources(defaults map[string]CollectorSourceDef, overrides map[string]CollectorSourceDef, presence map[string]sourceFieldPresence) map[string]CollectorSourceDef {
	if len(defaults) == 0 && len(overrides) == 0 {
		return nil
	}
	merged := make(map[string]CollectorSourceDef, len(defaults)+len(overrides))
	for key, value := range defaults {
		merged[key] = cloneCollectorSourceDef(value)
	}
	for key, value := range overrides {
		base, ok := merged[key]
		if !ok {
			merged[key] = cloneCollectorSourceDef(value)
			continue
		}
		merged[key] = mergeCollectorSourceDef(base, value, presence[key])
	}
	return merged
}

func cloneCollectorSourceDef(source CollectorSourceDef) CollectorSourceDef {
	cloned := source
	if source.Aliases != nil {
		cloned.Aliases = append([]string(nil), source.Aliases...)
	}
	if source.Headers != nil {
		cloned.Headers = make(map[string]string, len(source.Headers))
		for key, value := range source.Headers {
			cloned.Headers[key] = value
		}
	}
	if source.Todo != nil {
		cloned.Todo = append([]string(nil), source.Todo...)
	}
	if source.Notes != nil {
		cloned.Notes = append([]string(nil), source.Notes...)
	}
	return cloned
}

func mergeCollectorSourceDef(base CollectorSourceDef, override CollectorSourceDef, presence sourceFieldPresence) CollectorSourceDef {
	merged := cloneCollectorSourceDef(base)
	if override.DisplayName != "" {
		merged.DisplayName = override.DisplayName
	}
	if override.Aliases != nil {
		merged.Aliases = append([]string(nil), override.Aliases...)
	}
	if override.SourceType != "" {
		merged.SourceType = override.SourceType
	}
	if override.SourceURL != "" {
		merged.SourceURL = override.SourceURL
	}
	if presence.boolFields["enabled"] {
		merged.Enabled = override.Enabled
	}
	if presence.boolFields["schedule_enabled"] {
		merged.ScheduleEnabled = override.ScheduleEnabled
	}
	if override.IntervalMinutes != 0 {
		merged.IntervalMinutes = override.IntervalMinutes
	}
	if override.TimeoutMS != 0 {
		merged.TimeoutMS = override.TimeoutMS
	}
	if override.HotlistLimit != 0 {
		merged.HotlistLimit = override.HotlistLimit
	}
	if presence.boolFields["detail_fetch_enabled"] {
		merged.DetailFetchEnabled = override.DetailFetchEnabled
	}
	if override.Concurrency != 0 {
		merged.Concurrency = override.Concurrency
	}
	if presence.stringFields["auth_mode"] {
		merged.AuthMode = strings.TrimSpace(override.AuthMode)
	} else if presence.stringFields["auth_profile"] {
		merged.AuthMode = ""
	}
	if override.HTTPClient != "" {
		merged.HTTPClient = override.HTTPClient
	}
	if override.RetryPolicy != "" {
		merged.RetryPolicy = override.RetryPolicy
	}
	if presence.stringFields["auth_profile"] {
		merged.AuthProfile = strings.TrimSpace(override.AuthProfile)
	} else if override.AuthProfile != "" {
		merged.AuthProfile = override.AuthProfile
	}
	if override.CookieSecretRef != "" {
		merged.CookieSecretRef = override.CookieSecretRef
	}
	if override.HeaderSecretRef != "" {
		merged.HeaderSecretRef = override.HeaderSecretRef
	}
	if override.Headers != nil {
		merged.Headers = make(map[string]string, len(override.Headers))
		for key, value := range override.Headers {
			merged.Headers[key] = value
		}
	}
	if override.Status != "" {
		merged.Status = override.Status
	}
	if override.Goal != "" {
		merged.Goal = override.Goal
	}
	if override.Todo != nil {
		merged.Todo = append([]string(nil), override.Todo...)
	}
	if override.Notes != nil {
		merged.Notes = append([]string(nil), override.Notes...)
	}
	if override.MigrationReference != "" {
		merged.MigrationReference = override.MigrationReference
	}
	if presence.boolFields["supports_article"] {
		merged.SupportsArticle = override.SupportsArticle
	}
	if presence.boolFields["placeholder_required"] {
		merged.PlaceholderRequired = override.PlaceholderRequired
	}
	return merged
}
