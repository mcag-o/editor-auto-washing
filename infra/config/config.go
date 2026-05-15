package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	defaultHTTPHost = "0.0.0.0"
	defaultHTTPPort = 8123
	defaultLogLevel = "info"
	defaultDBPath   = "./data/content-hub.db"

	defaultCollectorHTTPClientProfileID  = "default_api_client"
	defaultCollectorRetryPolicyProfileID = "default_api"
	defaultCollectorAuthProfileID        = "none"
)

type Config struct {
	HTTP      HTTPConfig      `json:"http"`
	Log       LogConfig       `json:"log"`
	Database  DatabaseConfig  `json:"database"`
	Storage   StorageConfig   `json:"storage"`
	Workflow  WorkflowConfig  `json:"workflow"`
	Collector CollectorConfig `json:"collector"`
	Platforms PlatformsConfig `json:"platforms"`
	Secrets   SecretsConfig   `json:"secrets,omitempty"`
	LLM       LLMConfig       `json:"llm"`
	Template  TemplateConfig  `json:"template"`
}

type HTTPConfig struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	ReadTimeoutSec  int    `json:"read_timeout_sec"`
	WriteTimeoutSec int    `json:"write_timeout_sec"`
	ShutdownSec     int    `json:"shutdown_timeout_sec"`
}

type LogConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

type DatabaseConfig struct {
	Driver         string `json:"driver"`
	Path           string `json:"path"`
	MaxOpenConns   int    `json:"max_open_conns"`
	MaxIdleConns   int    `json:"max_idle_conns"`
	ConnMaxLifeMin int    `json:"conn_max_life_min"`
}

type StorageConfig struct {
	BasePath  string `json:"base_path"`
	MaxSizeMB int    `json:"max_size_mb"`
}

type WorkflowConfig struct {
	MaxConcurrentJobs int `json:"max_concurrent_jobs"`
	RetryMaxAttempts  int `json:"retry_max_attempts"`
	TimeoutSec        int `json:"timeout_sec"`
}

type LLMConfig struct {
	DefaultProfile string                   `json:"default_profile,omitempty"`
	Profiles       map[string]LLMProfileDef `json:"profiles,omitempty"`
	Provider       string                   `json:"provider,omitempty"`
	APIKey         string                   `json:"api_key,omitempty"`
	BaseURL        string                   `json:"base_url,omitempty"`
	Model          string                   `json:"model,omitempty"`
	Temperature    float64                  `json:"temperature"`
	MaxTokens      int                      `json:"max_tokens"`
	TimeoutSec     int                      `json:"timeout_sec"`
}

type LLMProfileDef struct {
	Provider    string  `json:"provider"`
	APIKey      string  `json:"api_key,omitempty"`
	APIKeyRef   string  `json:"api_key_ref,omitempty"`
	BaseURL     string  `json:"base_url,omitempty"`
	BaseURLRef  string  `json:"base_url_ref,omitempty"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	TimeoutSec  int     `json:"timeout_sec"`
}

type TemplateConfig struct {
	PromptDir     string `json:"prompt_dir"`
	DefaultPrompt string `json:"default_prompt"`
	CacheEnabled  bool   `json:"cache_enabled"`
}

type PlatformsConfig struct {
	Baidu   PlatformEntry `json:"baidu"`
	WeChat  PlatformEntry `json:"wechat"`
	Zhihu   PlatformEntry `json:"zhihu"`
	Toutiao PlatformEntry `json:"toutiao"`
	CSDN    PlatformEntry `json:"csdn"`
}

type PlatformEntry struct {
	Enabled bool   `json:"enabled"`
	Cookie  string `json:"cookie,omitempty"`
	Token   string `json:"token,omitempty"`
}

type SecretsConfig struct {
	EnvPrefix string `json:"env_prefix"`
}

func DefaultConfig() Config {
	return Config{
		HTTP: HTTPConfig{
			Host:            defaultHTTPHost,
			Port:            defaultHTTPPort,
			ReadTimeoutSec:  15,
			WriteTimeoutSec: 30,
			ShutdownSec:     10,
		},
		Log: LogConfig{
			Level:  defaultLogLevel,
			Format: "json",
		},
		Database: DatabaseConfig{
			Driver:         "sqlite",
			Path:           defaultDBPath,
			MaxOpenConns:   10,
			MaxIdleConns:   5,
			ConnMaxLifeMin: 60,
		},
		Storage: StorageConfig{
			BasePath:  "./data/storage",
			MaxSizeMB: 1024,
		},
		Workflow: WorkflowConfig{
			MaxConcurrentJobs: 5,
			RetryMaxAttempts:  3,
			TimeoutSec:        300,
		},
		Collector: DefaultCollectorConfig(),
		Platforms: PlatformsConfig{
			Baidu:   PlatformEntry{Enabled: false},
			WeChat:  PlatformEntry{Enabled: false},
			Zhihu:   PlatformEntry{Enabled: false},
			Toutiao: PlatformEntry{Enabled: false},
			CSDN:    PlatformEntry{Enabled: false},
		},
		Secrets: SecretsConfig{
			EnvPrefix: "CONTENTHUB",
		},
		LLM: LLMConfig{
			DefaultProfile: "default_openai",
			Profiles: map[string]LLMProfileDef{
				"default_openai": {
					Provider:    "openai",
					APIKeyRef:   "env.OPENAI_API_KEY",
					BaseURLRef:  "env.OPENAI_BASE_URL",
					Model:       "gpt-4.1",
					Temperature: 0.7,
					MaxTokens:   4096,
					TimeoutSec:  60,
				},
			},
			Provider:    "openai",
			Model:       "gpt-4",
			Temperature: 0.7,
			MaxTokens:   4096,
			TimeoutSec:  60,
		},
		Template: TemplateConfig{
			PromptDir:     "./prompts",
			DefaultPrompt: "default",
			CacheEnabled:  true,
		},
	}
}

func (c *Config) Validate() error {
	if c.HTTP.Host == "" {
		return fmt.Errorf("http.host cannot be empty")
	}
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http.port must be between 1 and 65535, got %d", c.HTTP.Port)
	}
	if c.HTTP.ReadTimeoutSec <= 0 {
		return fmt.Errorf("http.read_timeout_sec must be positive")
	}
	if c.HTTP.WriteTimeoutSec <= 0 {
		return fmt.Errorf("http.write_timeout_sec must be positive")
	}
	if c.Log.Level == "" {
		return fmt.Errorf("log.level cannot be empty")
	}
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Log.Level] {
		return fmt.Errorf("log.level must be one of debug,info,warn,error, got %q", c.Log.Level)
	}
	if c.Database.Driver == "" {
		return fmt.Errorf("database.driver cannot be empty")
	}
	if c.Database.Path == "" {
		return fmt.Errorf("database.path cannot be empty")
	}
	if c.Database.MaxOpenConns <= 0 {
		return fmt.Errorf("database.max_open_conns must be positive")
	}
	if c.Storage.BasePath == "" {
		return fmt.Errorf("storage.base_path cannot be empty")
	}
	if c.Workflow.MaxConcurrentJobs <= 0 {
		return fmt.Errorf("workflow.max_concurrent_jobs must be positive")
	}
	if c.Workflow.RetryMaxAttempts < 0 {
		return fmt.Errorf("workflow.retry_max_attempts cannot be negative")
	}
	if err := c.Collector.Validate(); err != nil {
		return err
	}
	if len(c.Collector.Sources) == 0 {
		return fmt.Errorf("collector.sources cannot be empty")
	}
	if err := c.validateLLM(); err != nil {
		return err
	}
	return nil
}

func (c *Config) ResolveSecrets() {
	c.resolvePlatformSecrets(&c.Platforms.Baidu, "BAIDU")
	c.resolvePlatformSecrets(&c.Platforms.WeChat, "WECHAT")
	c.resolvePlatformSecrets(&c.Platforms.Zhihu, "ZHIHU")
	c.resolvePlatformSecrets(&c.Platforms.Toutiao, "TOUTIAO")
	c.resolvePlatformSecrets(&c.Platforms.CSDN, "CSDN")
	c.resolveLLMSecrets()
}

func (c *Config) resolvePlatformSecrets(p *PlatformEntry, platformKey string) {
	prefix := c.Secrets.EnvPrefix
	if p.Cookie == "" {
		p.Cookie = os.Getenv(fmt.Sprintf("%s_%s_COOKIE", prefix, platformKey))
	}
	if p.Token == "" {
		p.Token = os.Getenv(fmt.Sprintf("%s_%s_TOKEN", prefix, platformKey))
	}
}

func (c *Config) resolveLLMSecrets() {
	_ = c.ResolveLLMRuntime()
}

func (c *Config) ResolveLLMRuntime() error {
	if c == nil {
		return nil
	}
	if c.LLM.DefaultProfile == "" {
		return nil
	}
	profile, ok := c.LLM.Profiles[c.LLM.DefaultProfile]
	if !ok {
		return fmt.Errorf("llm.default_profile %q does not exist", c.LLM.DefaultProfile)
	}
	resolvedProfile := resolvedLLMProfile(profile)
	if profile.Provider == "" {
		return fmt.Errorf("llm.profiles.%s.provider cannot be empty", c.LLM.DefaultProfile)
	}
	if profile.Model == "" {
		return fmt.Errorf("llm.profiles.%s.model cannot be empty", c.LLM.DefaultProfile)
	}
	c.LLM.Provider = resolvedProfile.Provider
	c.LLM.APIKey = resolvedProfile.APIKey
	c.LLM.BaseURL = resolvedProfile.BaseURL
	c.LLM.Model = resolvedProfile.Model
	c.LLM.Temperature = resolvedProfile.Temperature
	c.LLM.MaxTokens = resolvedProfile.MaxTokens
	c.LLM.TimeoutSec = resolvedProfile.TimeoutSec
	return nil
}

func (c *Config) validateLLM() error {
	if c.LLM.DefaultProfile != "" {
		profile, ok := c.LLM.Profiles[c.LLM.DefaultProfile]
		if !ok {
			return fmt.Errorf("llm.default_profile %q does not exist", c.LLM.DefaultProfile)
		}
		if profile.Provider == "" {
			return fmt.Errorf("llm.profiles.%s.provider cannot be empty", c.LLM.DefaultProfile)
		}
		if profile.Model == "" {
			return fmt.Errorf("llm.profiles.%s.model cannot be empty", c.LLM.DefaultProfile)
		}
	}
	if c.LLM.Provider == "" {
		return fmt.Errorf("llm.provider cannot be empty")
	}
	if c.LLM.Model == "" {
		return fmt.Errorf("llm.model cannot be empty")
	}
	if c.LLM.MaxTokens <= 0 {
		return fmt.Errorf("llm.max_tokens must be positive")
	}
	if c.LLM.TimeoutSec <= 0 {
		return fmt.Errorf("llm.timeout_sec must be positive")
	}
	return nil
}

func resolveEnvSecretRef(ref string) string {
	if !strings.HasPrefix(ref, "env.") {
		return ""
	}
	name := strings.TrimPrefix(ref, "env.")
	return os.Getenv(name)
}

func resolvedLLMProfile(profile LLMProfileDef) LLMProfileDef {
	resolved := profile
	if resolved.APIKey == "" {
		resolved.APIKey = resolveEnvSecretRef(resolved.APIKeyRef)
	}
	if resolved.BaseURL == "" {
		resolved.BaseURL = resolveEnvSecretRef(resolved.BaseURLRef)
	}
	return resolved
}

func (c *Config) Hash() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", fnvHash(data)), nil
}

func (c *Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func fnvHash(data []byte) uint64 {
	var hash uint64 = 14695981039346656037
	for _, b := range data {
		hash ^= uint64(b)
		hash *= 1099511628211
	}
	return hash
}

func (c *Config) PlatformStatus() map[string]bool {
	return map[string]bool{
		"baidu":   c.Platforms.Baidu.Enabled,
		"wechat":  c.Platforms.WeChat.Enabled,
		"zhihu":   c.Platforms.Zhihu.Enabled,
		"toutiao": c.Platforms.Toutiao.Enabled,
		"csdn":    c.Platforms.CSDN.Enabled,
	}
}

func (c *Config) EnabledPlatforms() []string {
	var enabled []string
	for name, active := range c.PlatformStatus() {
		if active {
			enabled = append(enabled, name)
		}
	}
	return enabled
}

func (c *Config) Redacted() Config {
	redacted := *c
	redacted.Platforms.Baidu = redactPlatform(redacted.Platforms.Baidu)
	redacted.Platforms.WeChat = redactPlatform(redacted.Platforms.WeChat)
	redacted.Platforms.Zhihu = redactPlatform(redacted.Platforms.Zhihu)
	redacted.Platforms.Toutiao = redactPlatform(redacted.Platforms.Toutiao)
	redacted.Platforms.CSDN = redactPlatform(redacted.Platforms.CSDN)
	if redacted.LLM.APIKey != "" {
		redacted.LLM.APIKey = maskSecret(redacted.LLM.APIKey)
	}
	return redacted
}

func redactPlatform(p PlatformEntry) PlatformEntry {
	redacted := p
	if redacted.Cookie != "" {
		redacted.Cookie = maskSecret(redacted.Cookie)
	}
	if redacted.Token != "" {
		redacted.Token = maskSecret(redacted.Token)
	}
	return redacted
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}
