package config

import (
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
	assert.False(t, cfg.Platforms.Baidu.Enabled)
	assert.False(t, cfg.Platforms.WeChat.Enabled)
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

func TestConfigResolveSecrets(t *testing.T) {
	t.Setenv("CONTENTHUB_BAIDU_COOKIE", "env-baidu-cookie")
	t.Setenv("CONTENTHUB_WECHAT_TOKEN", "env-wechat-token")

	cfg := DefaultConfig()
	cfg.Platforms.Baidu.Cookie = ""
	cfg.Platforms.WeChat.Token = ""
	cfg.ResolveSecrets()

	assert.Equal(t, "env-baidu-cookie", cfg.Platforms.Baidu.Cookie)
	assert.Equal(t, "env-wechat-token", cfg.Platforms.WeChat.Token)
}

func TestConfigResolveSecretsPreservesFile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Platforms.Baidu.Cookie = "file-cookie"
	cfg.ResolveSecrets()

	assert.Equal(t, "file-cookie", cfg.Platforms.Baidu.Cookie)
}

func TestConfigResolveSecretsCustomPrefix(t *testing.T) {
	t.Setenv("CUSTOM_BAIDU_COOKIE", "custom-cookie")

	cfg := DefaultConfig()
	cfg.Secrets.EnvPrefix = "CUSTOM"
	cfg.Platforms.Baidu.Cookie = ""
	cfg.ResolveSecrets()

	assert.Equal(t, "custom-cookie", cfg.Platforms.Baidu.Cookie)
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

func TestConfigPlatformStatus(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Platforms.Baidu.Enabled = true
	cfg.Platforms.Zhihu.Enabled = true

	status := cfg.PlatformStatus()
	assert.True(t, status["baidu"])
	assert.True(t, status["zhihu"])
	assert.False(t, status["wechat"])
}

func TestConfigEnabledPlatforms(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Platforms.Baidu.Enabled = true
	cfg.Platforms.CSDN.Enabled = true

	enabled := cfg.EnabledPlatforms()
	assert.Len(t, enabled, 2)
	assert.Contains(t, enabled, "baidu")
	assert.Contains(t, enabled, "csdn")
}

func TestConfigRedacted(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Platforms.Baidu.Cookie = "short"
	cfg.Platforms.WeChat.Cookie = "this-is-a-very-long-cookie-value"
	cfg.Platforms.Zhihu.Token = "1234567890abcdef"

	redacted := cfg.Redacted()

	assert.Equal(t, "****", redacted.Platforms.Baidu.Cookie)
	assert.Equal(t, "this************************alue", redacted.Platforms.WeChat.Cookie)
	assert.Equal(t, "1234********cdef", redacted.Platforms.Zhihu.Token)
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
