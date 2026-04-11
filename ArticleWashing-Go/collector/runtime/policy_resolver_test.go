package runtime

import (
	"fmt"
	"testing"
	"time"

	"content-hub/domain"
	"content-hub/infra/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type secretStubResolver map[string]string

func (s secretStubResolver) Resolve(ref string) (string, error) {
	value, ok := s[ref]
	if !ok {
		return "", fmt.Errorf("missing secret for %s", ref)
	}
	return value, nil
}

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

	resolver := NewPolicyResolver(secretStubResolver{})
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
	assert.Empty(t, resolved.Auth.Cookie)
	assert.Empty(t, resolved.Auth.Headers)
	assert.Equal(t, map[string]any{
		"detail_fetch_enabled": true,
		"goal":                 "补齐知乎抓取实现",
		"placeholder_required": true,
		"source_type":          "json-api",
		"status":               "placeholder",
		"supports_article":     true,
	}, resolved.Options)
}

func TestPolicyResolver_ResolvesHeaderAuthProfileSecrets(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Collector.AuthProfiles["token_header"] = config.AuthProfileConfig{
		Mode:              domain.CollectorAuthModeHeader,
		HeaderName:        "X-API-Key",
		HeaderValuePrefix: "Token ",
	}
	cfg.Collector.Sources["zhihu"] = config.CollectorSourceDef{
		DisplayName:     "知乎热榜",
		SourceType:      "json-api",
		SourceURL:       "https://www.zhihu.com/api/v3/explore/guest/feeds",
		AuthProfile:     "token_header",
		HeaderSecretRef: "env.ZHIHU_TOKEN",
	}

	resolver := NewPolicyResolver(secretStubResolver{"env.ZHIHU_TOKEN": "secret-value"})
	resolved, err := resolver.ResolveSource(cfg, "zhihu")
	require.NoError(t, err)
	assert.Equal(t, domain.CollectorAuthModeHeader, resolved.Auth.Mode)
	assert.Equal(t, map[string]string{"X-API-Key": "Token secret-value"}, resolved.Auth.Headers)
	assert.Empty(t, resolved.Auth.Cookie)
}
