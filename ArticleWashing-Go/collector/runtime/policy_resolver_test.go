package runtime

import (
	"testing"
	"time"

	"content-hub/domain"
	"content-hub/infra/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyResolver_MergesDefaultsProfilesAndSourceOverrides(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Collector.Defaults.TimeoutMS = 10000
	cfg.Collector.Defaults.HotlistLimit = 25
	cfg.Collector.HTTPClients["custom_client"] = config.HTTPClientProfile{
		UserAgent: "collector-test/1.0",
		Headers: map[string]string{
			"X-Default":  "from-profile",
			"X-Override": "from-profile",
		},
	}
	cfg.Collector.RetryPolicies["aggressive"] = config.RetryPolicyProfile{
		MaxAttempts: 5,
		BaseWaitMS:  750,
		MaxWaitMS:   9000,
	}
	cfg.Collector.Sources["zhihu"] = config.CollectorSourceDef{
		DisplayName:         "知乎热榜",
		SourceType:          "json-api",
		SourceURL:           "https://www.zhihu.com/api/v3/explore/guest/feeds",
		HTTPClient:          "custom_client",
		RetryPolicy:         "aggressive",
		AuthProfile:         "none",
		TimeoutMS:           12000,
		Headers:             map[string]string{"X-Override": "from-source", "X-Source": "set"},
		DetailFetchEnabled:  true,
		SupportsArticle:     true,
		PlaceholderRequired: true,
		Status:              "placeholder",
		Goal:                "补齐知乎抓取实现",
	}

	resolver := NewPolicyResolver()
	resolved, err := resolver.ResolveSource(cfg, "zhihu")
	require.NoError(t, err)
	assert.Equal(t, 12000*time.Millisecond, resolved.Timeout)
	assert.Equal(t, 25, resolved.HotlistLimit)
	assert.Equal(t, "https://www.zhihu.com/api/v3/explore/guest/feeds", resolved.BaseURL)
	assert.Equal(t, "zhihu", resolved.SourceID)
	assert.Equal(t, map[string]string{
		"User-Agent": "collector-test/1.0",
		"X-Default":  "from-profile",
		"X-Override": "from-source",
		"X-Source":   "set",
	}, resolved.Headers)
	assert.Equal(t, 5, resolved.RetryPolicy.MaxAttempts)
	assert.Equal(t, 750*time.Millisecond, resolved.RetryPolicy.Wait)
	assert.Equal(t, 9*time.Second, resolved.RetryPolicy.MaxWait)
	assert.Equal(t, domain.CollectorAuthModeNone, resolved.Auth.Mode)
	assert.Empty(t, resolved.Auth.CookieSecretRef)
	assert.Empty(t, resolved.Auth.HeaderSecretRef)
	assert.Empty(t, resolved.Auth.HeaderName)
	assert.Empty(t, resolved.Auth.HeaderValuePrefix)
	assert.Equal(t, map[string]any{
		"detail_fetch_enabled": true,
		"goal":                 "补齐知乎抓取实现",
		"placeholder_required": true,
		"source_type":          "json-api",
		"status":               "placeholder",
		"supports_article":     true,
	}, resolved.Options)
}

func TestPolicyResolver_InheritsDefaultProfilesWhenSourceOmitsRefs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Collector.Defaults.HTTPClient = "custom_client"
	cfg.Collector.Defaults.RetryPolicy = "aggressive"
	cfg.Collector.Defaults.AuthProfile = "token_header"
	cfg.Collector.Defaults.TimeoutMS = 8000
	cfg.Collector.Defaults.HotlistLimit = 40
	cfg.Collector.HTTPClients["custom_client"] = config.HTTPClientProfile{
		UserAgent: "default-agent/2.0",
		Headers: map[string]string{
			"X-Default": "from-default-profile",
		},
	}
	cfg.Collector.RetryPolicies["aggressive"] = config.RetryPolicyProfile{
		MaxAttempts: 4,
		BaseWaitMS:  600,
		MaxWaitMS:   2400,
	}
	cfg.Collector.AuthProfiles["token_header"] = config.AuthProfileConfig{
		Mode:              domain.CollectorAuthModeHeader,
		HeaderName:        "X-API-Key",
		HeaderValuePrefix: "Token ",
	}
	cfg.Collector.Sources["zhihu"] = config.CollectorSourceDef{
		DisplayName:     "知乎热榜",
		SourceType:      "json-api",
		SourceURL:       "https://www.zhihu.com/api/v3/explore/guest/feeds",
		HeaderSecretRef: "env.ZHIHU_TOKEN",
	}

	resolver := NewPolicyResolver()
	resolved, err := resolver.ResolveSource(cfg, "zhihu")
	require.NoError(t, err)
	assert.Equal(t, 8*time.Second, resolved.Timeout)
	assert.Equal(t, 40, resolved.HotlistLimit)
	assert.Equal(t, map[string]string{
		"User-Agent": "default-agent/2.0",
		"X-Default":  "from-default-profile",
	}, resolved.Headers)
	assert.Equal(t, 4, resolved.RetryPolicy.MaxAttempts)
	assert.Equal(t, 600*time.Millisecond, resolved.RetryPolicy.Wait)
	assert.Equal(t, 2400*time.Millisecond, resolved.RetryPolicy.MaxWait)
	assert.Equal(t, domain.CollectorAuthModeHeader, resolved.Auth.Mode)
	assert.Equal(t, "X-API-Key", resolved.Auth.HeaderName)
	assert.Equal(t, "Token ", resolved.Auth.HeaderValuePrefix)
	assert.Equal(t, "env.ZHIHU_TOKEN", resolved.Auth.HeaderSecretRef)
	assert.Empty(t, resolved.Auth.CookieSecretRef)
}

func TestPolicyResolver_ReturnsErrorWhenSourceMissing(t *testing.T) {
	resolver := NewPolicyResolver()
	_, err := resolver.ResolveSource(config.DefaultConfig(), "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector source missing not found")
}
