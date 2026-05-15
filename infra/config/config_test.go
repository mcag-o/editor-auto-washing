package config

import (
	"encoding/json"
	"testing"

	"content-hub/domain"

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
	cfg.LLM.APIKey = "sk-test-1234567890abcdef"

	redacted := cfg.Redacted()

	assert.Equal(t, "****", redacted.Platforms.Baidu.Cookie)
	assert.Equal(t, "this************************alue", redacted.Platforms.WeChat.Cookie)
	assert.Equal(t, "1234********cdef", redacted.Platforms.Zhihu.Token)
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

func TestDefaultConfig_CollectorSourceCatalogIncludesTwentyTwoPlatforms(t *testing.T) {
	cfg := DefaultConfig()

	assert.Len(t, cfg.Collector.Sources, 22)
	assert.Equal(t, "百度热搜", cfg.Collector.Sources["baidu"].DisplayName)
	assert.Equal(t, "json-api", cfg.Collector.Sources["zhihu"].SourceType)
	assert.Equal(t, "html", cfg.Collector.Sources["hackernews"].SourceType)
	assert.Equal(t, "env.XUEQIU_COOKIE", cfg.Collector.Sources["xueqiu"].CookieSecretRef)
	assert.Contains(t, cfg.Collector.Sources["36kr"].Aliases, "tskr")
}

func TestDefaultConfig_CollectorPoliciesAreConfigDriven(t *testing.T) {
	cfg := DefaultConfig()

	assert.Contains(t, cfg.Collector.HTTPClients, "default_api_client")
	assert.Contains(t, cfg.Collector.RetryPolicies, "default_api")
	assert.Contains(t, cfg.Collector.AuthProfiles, "none")
	assert.Equal(t, "default_api_client", cfg.Collector.Defaults.HTTPClient)
	assert.Equal(t, "default_api", cfg.Collector.Defaults.RetryPolicy)
	assert.Equal(t, "none", cfg.Collector.Defaults.AuthProfile)
}

func TestConfigValidate_RejectsMissingCollectorPolicyReference(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Collector.Sources["zhihu"] = CollectorSourceDef{
		DisplayName: "知乎热榜",
		SourceType:  "json-api",
		SourceURL:   "https://www.zhihu.com/api/v3/explore/guest/feeds",
		HTTPClient:  "missing-client",
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector.sources.zhihu.http_client")
}

func TestConfigValidate_RejectsMissingCollectorDefaultPolicyReference(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "missing default http client",
			mutate: func(cfg *Config) {
				cfg.Collector.Defaults.HTTPClient = "missing-client"
			},
			wantErr: "collector.defaults.http_client",
		},
		{
			name: "missing default retry policy",
			mutate: func(cfg *Config) {
				cfg.Collector.Defaults.RetryPolicy = "missing-policy"
			},
			wantErr: "collector.defaults.retry_policy",
		},
		{
			name: "missing default auth profile",
			mutate: func(cfg *Config) {
				cfg.Collector.Defaults.AuthProfile = "missing-auth"
			},
			wantErr: "collector.defaults.auth_profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.mutate(&cfg)

			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestConfigValidate_RejectsMissingCollectorSourceRetryPolicyReference(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Collector.Sources["zhihu"] = CollectorSourceDef{
		DisplayName: "知乎热榜",
		SourceType:  "json-api",
		SourceURL:   "https://www.zhihu.com/api/v3/explore/guest/feeds",
		RetryPolicy: "missing-policy",
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector.sources.zhihu.retry_policy")
}

func TestConfigValidate_RejectsMissingCollectorSourceAuthProfileReference(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Collector.Sources["zhihu"] = CollectorSourceDef{
		DisplayName: "知乎热榜",
		SourceType:  "json-api",
		SourceURL:   "https://www.zhihu.com/api/v3/explore/guest/feeds",
		AuthProfile: "missing-auth",
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector.sources.zhihu.auth_profile")
}

func TestCollectorConfig_SourceOrDefault_DerivesAuthModeFromProfile(t *testing.T) {
	cfg := DefaultCollectorConfig()
	cfg.Sources["zhihu"] = CollectorSourceDef{
		DisplayName: "知乎热榜",
		SourceType:  "json-api",
		SourceURL:   "https://www.zhihu.com/api/v3/explore/guest/feeds",
		AuthProfile: "cookie",
		AuthMode:    "none",
	}

	source, ok := cfg.SourceOrDefault("zhihu")
	require.True(t, ok)
	assert.Equal(t, "cookie", source.AuthProfile)
	assert.Equal(t, "cookie", source.AuthMode)
}

func TestDefaultCollectorConfig_SourceSchemaOmitsNonRuntimeMetadataFields(t *testing.T) {
	cfg := DefaultCollectorConfig()

	data, err := json.Marshal(cfg.Sources["baidu"])
	require.NoError(t, err)

	assert.NotContains(t, string(data), `"status"`)
	assert.NotContains(t, string(data), `"goal"`)
	assert.NotContains(t, string(data), `"todo"`)
	assert.NotContains(t, string(data), `"notes"`)
	assert.NotContains(t, string(data), `"implementation_reference"`)
	assert.NotContains(t, string(data), `"placeholder_required"`)
}

func TestConfigValidate_RejectsCollectorSourceAuthModeProfileConflict(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Collector.Sources["zhihu"] = CollectorSourceDef{
		DisplayName: "知乎热榜",
		SourceType:  "json-api",
		SourceURL:   "https://www.zhihu.com/api/v3/explore/guest/feeds",
		AuthProfile: "cookie",
		AuthMode:    "header",
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector.sources.zhihu.auth_mode")
}

func TestConfigValidate_RejectsInvalidUnreferencedCollectorRetryPolicy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Collector.RetryPolicies["broken_retry"] = RetryPolicyProfile{
		MaxAttempts: 0,
		BaseWaitMS:  10,
		MaxWaitMS:   20,
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector.retry_policies.broken_retry.max_attempts")
}

func TestConfigValidate_RejectsInvalidUnreferencedCollectorAuthProfileMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Collector.AuthProfiles["broken_auth"] = AuthProfileConfig{
		Mode: "oauth",
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector.auth_profiles.broken_auth.mode")
}

func TestConfigValidate_RejectsHeaderAuthProfileWithoutHeaderName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Collector.AuthProfiles["broken_auth"] = AuthProfileConfig{
		Mode: domain.CollectorAuthModeHeader,
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector.auth_profiles.broken_auth.header_name")
}

func TestConfigValidate_RejectsMissingDefaultLLMProfile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.DefaultProfile = "missing"

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "llm.default_profile")
}

func TestConfigValidate_RejectsCollectorSourceAuthModeWithoutAuthProfile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Collector.Sources["zhihu"] = CollectorSourceDef{
		DisplayName: "知乎热榜",
		SourceType:  "json-api",
		SourceURL:   "https://www.zhihu.com/api/v3/explore/guest/feeds",
		AuthMode:    "cookie",
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector.sources.zhihu.auth_mode")
}
