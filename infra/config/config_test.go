package config

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, defaultHTTPHost, cfg.HTTP.Host)
	assert.Equal(t, defaultHTTPPort, cfg.HTTP.Port)
	assert.Equal(t, defaultLogLevel, cfg.Log.Level)
	assert.Equal(t, defaultDBPath, cfg.Database.Path)

	configType := reflect.TypeOf(cfg)
	_, hasCollector := configType.FieldByName("Collector")
	_, hasPlatforms := configType.FieldByName("Platforms")
	assert.False(t, hasCollector)
	assert.False(t, hasPlatforms)

	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"collector"`)
	assert.NotContains(t, string(data), `"platforms"`)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name:   "valid default config",
			mutate: func(*Config) {},
		},
		{
			name:    "empty host",
			mutate:  func(c *Config) { c.HTTP.Host = "" },
			wantErr: "http.host cannot be empty",
		},
		{
			name:    "port too low",
			mutate:  func(c *Config) { c.HTTP.Port = 0 },
			wantErr: "http.port must be between 1 and 65535",
		},
		{
			name:    "port too high",
			mutate:  func(c *Config) { c.HTTP.Port = 70000 },
			wantErr: "http.port must be between 1 and 65535",
		},
		{
			name:    "negative read timeout",
			mutate:  func(c *Config) { c.HTTP.ReadTimeoutSec = -1 },
			wantErr: "http.read_timeout_sec must be positive",
		},
		{
			name:    "negative write timeout",
			mutate:  func(c *Config) { c.HTTP.WriteTimeoutSec = 0 },
			wantErr: "http.write_timeout_sec must be positive",
		},
		{
			name:    "empty log level",
			mutate:  func(c *Config) { c.Log.Level = "" },
			wantErr: "log.level cannot be empty",
		},
		{
			name:    "invalid log level",
			mutate:  func(c *Config) { c.Log.Level = "verbose" },
			wantErr: "log.level must be one of debug,info,warn,error",
		},
		{
			name:    "empty database driver",
			mutate:  func(c *Config) { c.Database.Driver = "" },
			wantErr: "database.driver cannot be empty",
		},
		{
			name:    "empty database path",
			mutate:  func(c *Config) { c.Database.Path = "" },
			wantErr: "database.path cannot be empty",
		},
		{
			name:    "zero max open conns",
			mutate:  func(c *Config) { c.Database.MaxOpenConns = 0 },
			wantErr: "database.max_open_conns must be positive",
		},
		{
			name:    "empty storage path",
			mutate:  func(c *Config) { c.Storage.BasePath = "" },
			wantErr: "storage.base_path cannot be empty",
		},
		{
			name:    "zero max concurrent jobs",
			mutate:  func(c *Config) { c.Workflow.MaxConcurrentJobs = 0 },
			wantErr: "workflow.max_concurrent_jobs must be positive",
		},
		{
			name:    "negative retry attempts",
			mutate:  func(c *Config) { c.Workflow.RetryMaxAttempts = -1 },
			wantErr: "workflow.retry_max_attempts cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigResolveLLMRuntime(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-openai-key")
	t.Setenv("OPENAI_BASE_URL", "https://llm.example.test")

	cfg := DefaultConfig()
	require.NoError(t, cfg.ResolveLLMRuntime())

	assert.Equal(t, "env-openai-key", cfg.LLM.APIKey)
	assert.Equal(t, "https://llm.example.test", cfg.LLM.BaseURL)
}

func TestConfigHash(t *testing.T) {
	cfg := DefaultConfig()
	hash1, err := cfg.Hash()
	require.NoError(t, err)
	assert.NotEmpty(t, hash1)

	cfg.HTTP.Port = 9090
	hash2, err := cfg.Hash()
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

func TestConfigHashDeterministic(t *testing.T) {
	cfg1 := DefaultConfig()
	cfg2 := DefaultConfig()

	hash1, err := cfg1.Hash()
	require.NoError(t, err)
	hash2, err := cfg2.Hash()
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2)
}

func TestConfigRedacted(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.APIKey = "sk-test-1234567890abcdef"

	redacted := cfg.Redacted()

	assert.Equal(t, "sk-t****************cdef", redacted.LLM.APIKey)
	assert.Equal(t, cfg.LLM.Profiles["default_openai"].APIKeyRef, redacted.LLM.Profiles["default_openai"].APIKeyRef)
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "****"},
		{"short", "****"},
		{"12345678", "****"},
		{"abcdefghijklmnop", "abcd********mnop"},
		{"1234567890abcdef", "1234********cdef"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, maskSecret(tt.input))
		})
	}
}

func TestConfigValidate_RejectsMissingDefaultLLMProfile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.DefaultProfile = "missing"

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "llm.default_profile")
}
