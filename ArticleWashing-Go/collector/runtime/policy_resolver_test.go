package runtime

import (
	"testing"
	"time"

	"content-hub/infra/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type secretStubResolver struct{}

func (secretStubResolver) Resolve(ref string) (string, error) {
	return "", nil
}

func TestPolicyResolver_MergesDefaultsProfilesAndSourceOverrides(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Collector.Defaults.TimeoutMS = 10000
	cfg.Collector.Sources["zhihu"] = config.CollectorSourceDef{
		DisplayName: "知乎热榜",
		SourceType:  "json-api",
		SourceURL:   "https://www.zhihu.com/api/v3/explore/guest/feeds",
		HTTPClient:  "default_api_client",
		RetryPolicy: "default_api",
		AuthProfile: "none",
		TimeoutMS:   12000,
	}

	resolver := NewPolicyResolver(secretStubResolver{})
	resolved, err := resolver.ResolveSource(cfg, "zhihu")
	require.NoError(t, err)
	assert.Equal(t, 12000*time.Millisecond, resolved.Timeout)
	assert.Equal(t, "https://www.zhihu.com/api/v3/explore/guest/feeds", resolved.BaseURL)
	assert.Equal(t, "zhihu", resolved.SourceID)
}
