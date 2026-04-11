package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderLoadNonExistentCreatesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	loader := NewLoader(path)
	cfg, err := loader.Load()

	require.NoError(t, err)
	assert.Equal(t, defaultHTTPPort, cfg.HTTP.Port)
	assert.Equal(t, defaultLogLevel, cfg.Log.Level)
}

func TestLoaderLoadExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := DefaultConfig()
	cfg.HTTP.Port = 9090
	cfg.Log.Level = "debug"

	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)
	loaded, err := loader.Load()

	require.NoError(t, err)
	assert.Equal(t, 9090, loaded.HTTP.Port)
	assert.Equal(t, "debug", loaded.Log.Level)
}

func TestLoaderLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	require.NoError(t, os.WriteFile(path, []byte("{invalid json"), 0o644))

	loader := NewLoader(path)
	_, err := loader.Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestLoaderLoadValidationFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	invalid := map[string]interface{}{
		"http": map[string]interface{}{
			"host": "",
			"port": -1,
		},
	}
	data, _ := json.Marshal(invalid)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)
	_, err := loader.Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "config validation failed")
}

func TestLoaderSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	loader := NewLoader(path)
	cfg := DefaultConfig()
	cfg.HTTP.Port = 7070

	err := loader.Save(cfg)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var loaded Config
	require.NoError(t, json.Unmarshal(data, &loaded))
	assert.Equal(t, 7070, loaded.HTTP.Port)
}

func TestLoaderSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "config.json")

	loader := NewLoader(path)
	cfg := DefaultConfig()

	err := loader.Save(cfg)
	require.NoError(t, err)

	_, err = os.Stat(path)
	require.NoError(t, err)
}

func TestLoaderSaveAtomicOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := DefaultConfig()
	original.HTTP.Port = 1111

	data, _ := json.Marshal(original)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)

	badPath := filepath.Join(dir, string([]byte{0}), "config.json")
	loader.SetPath(badPath)

	cfg := DefaultConfig()
	cfg.HTTP.Port = 2222
	err := loader.Save(cfg)
	require.Error(t, err)

	_, err = os.Stat(path)
	require.NoError(t, err)
}

func TestLoaderReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := DefaultConfig()
	cfg.HTTP.Port = 8080
	data, _ := json.Marshal(cfg)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)
	_, err := loader.Load()
	require.NoError(t, err)

	cfg.HTTP.Port = 9090
	data, _ = json.Marshal(cfg)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	reloaded, err := loader.Reload()
	require.NoError(t, err)
	assert.Equal(t, 9090, reloaded.HTTP.Port)
}

func TestLoaderCurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := DefaultConfig()
	cfg.HTTP.Port = 5555
	data, _ := json.Marshal(cfg)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)
	_, err := loader.Load()
	require.NoError(t, err)

	current := loader.Current()
	assert.Equal(t, 5555, current.HTTP.Port)
}

func TestLoaderOnChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg1 := DefaultConfig()
	cfg1.HTTP.Port = 8080
	data, _ := json.Marshal(cfg1)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)
	_, err := loader.Load()
	require.NoError(t, err)

	var changeReceived bool
	var oldCfg, newCfg Config
	loader.OnChange(func(old, new Config, changes Changes) {
		changeReceived = true
		oldCfg = old
		newCfg = new
	})

	cfg2 := DefaultConfig()
	cfg2.HTTP.Port = 9090
	data, _ = json.Marshal(cfg2)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	_, err = loader.Reload()
	require.NoError(t, err)

	assert.True(t, changeReceived)
	assert.Equal(t, 8080, oldCfg.HTTP.Port)
	assert.Equal(t, 9090, newCfg.HTTP.Port)
}

func TestLoaderOnChangeAllowsReentrantCurrentRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg1 := DefaultConfig()
	data, _ := json.Marshal(cfg1)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)
	_, err := loader.Load()
	require.NoError(t, err)

	done := make(chan struct{})
	loader.OnChange(func(old, new Config, changes Changes) {
		_ = loader.Current()
		close(done)
	})

	cfg2 := DefaultConfig()
	cfg2.HTTP.Port = 9191
	data, _ = json.Marshal(cfg2)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	_, err = loader.Reload()
	require.NoError(t, err)
	<-done
}

func TestLoaderApplyDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	partial := map[string]interface{}{
		"http": map[string]interface{}{
			"port": 3000,
		},
	}
	data, _ := json.Marshal(partial)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)
	cfg, err := loader.Load()

	require.NoError(t, err)
	assert.Equal(t, 3000, cfg.HTTP.Port)
	assert.Equal(t, defaultHTTPHost, cfg.HTTP.Host)
	assert.Equal(t, defaultLogLevel, cfg.Log.Level)
}

func TestLoaderApplyDefaults_MergesBuiltInCollectorPolicyMaps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	partial := map[string]any{
		"collector": map[string]any{
			"http_clients": map[string]any{
				"custom_client": map[string]any{
					"headers": map[string]any{"X-Test": "1"},
				},
			},
			"retry_policies": map[string]any{
				"custom_retry": map[string]any{
					"max_attempts": 5,
					"base_wait_ms": 100,
					"max_wait_ms":  500,
				},
			},
			"auth_profiles": map[string]any{
				"custom_header": map[string]any{
					"mode": "header",
				},
			},
		},
	}
	data, _ := json.Marshal(partial)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)
	cfg, err := loader.Load()

	require.NoError(t, err)
	assert.Contains(t, cfg.Collector.HTTPClients, defaultCollectorHTTPClientProfileID)
	assert.Contains(t, cfg.Collector.HTTPClients, "custom_client")
	assert.Contains(t, cfg.Collector.RetryPolicies, defaultCollectorRetryPolicyProfileID)
	assert.Contains(t, cfg.Collector.RetryPolicies, "custom_retry")
	assert.Contains(t, cfg.Collector.AuthProfiles, defaultCollectorAuthProfileID)
	assert.Contains(t, cfg.Collector.AuthProfiles, "custom_header")
	assert.Equal(t, "1", cfg.Collector.HTTPClients["custom_client"].Headers["X-Test"])
	assert.Equal(t, 5, cfg.Collector.RetryPolicies["custom_retry"].MaxAttempts)
	assert.Equal(t, "header", cfg.Collector.AuthProfiles["custom_header"].Mode)
}

func TestLoaderApplyDefaults_PartiallyOverridesBuiltInRetryPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	partial := map[string]any{
		"collector": map[string]any{
			"retry_policies": map[string]any{
				defaultCollectorRetryPolicyProfileID: map[string]any{
					"max_attempts": 7,
				},
			},
		},
	}
	data, _ := json.Marshal(partial)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)
	cfg, err := loader.Load()

	require.NoError(t, err)
	policy := cfg.Collector.RetryPolicies[defaultCollectorRetryPolicyProfileID]
	assert.Equal(t, 7, policy.MaxAttempts)
	assert.Equal(t, 500, policy.BaseWaitMS)
	assert.Equal(t, 5000, policy.MaxWaitMS)
}

func TestLoaderApplyDefaults_PartiallyOverridesBuiltInAuthProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	partial := map[string]any{
		"collector": map[string]any{
			"auth_profiles": map[string]any{
				"cookie": map[string]any{
					"mode": "cookie",
				},
			},
		},
	}
	data, _ := json.Marshal(partial)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)
	cfg, err := loader.Load()

	require.NoError(t, err)
	profile := cfg.Collector.AuthProfiles["cookie"]
	assert.Equal(t, "cookie", profile.Mode)
}

func TestLoaderApplyDefaults_PartiallyOverridesBuiltInHTTPClientProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	partial := map[string]any{
		"collector": map[string]any{
			"http_clients": map[string]any{
				defaultCollectorHTTPClientProfileID: map[string]any{
					"user_agent": "custom-agent/1.0",
				},
			},
		},
	}
	data, _ := json.Marshal(partial)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)
	cfg, err := loader.Load()

	require.NoError(t, err)
	profile := cfg.Collector.HTTPClients[defaultCollectorHTTPClientProfileID]
	assert.Equal(t, "custom-agent/1.0", profile.UserAgent)
	assert.NotNil(t, profile.Headers)
	assert.Empty(t, profile.Headers)
}

func TestLoaderApplyDefaults_MergesCollectorSourceOverridesWithBuiltInCatalog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	partial := map[string]any{
		"collector": map[string]any{
			"sources": map[string]any{
				"zhihu": map[string]any{
					"display_name":         "知乎热榜-自定义",
					"interval_minutes":     15,
					"detail_fetch_enabled": true,
				},
			},
		},
	}
	data, _ := json.Marshal(partial)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)
	cfg, err := loader.Load()

	require.NoError(t, err)
	assert.Len(t, cfg.Collector.Sources, 22)
	assert.Contains(t, cfg.Collector.Sources, "baidu")
	assert.Contains(t, cfg.Collector.Sources, "zhihu")
	zhihu := cfg.Collector.Sources["zhihu"]
	assert.Equal(t, "知乎热榜-自定义", zhihu.DisplayName)
	assert.Equal(t, 15, zhihu.IntervalMinutes)
	assert.True(t, zhihu.DetailFetchEnabled)
	assert.Equal(t, "json-api", zhihu.SourceType)
	assert.Equal(t, "https://www.zhihu.com/api/v3/explore/guest/feeds?limit=30&ws_qiangzhisafe=0", zhihu.SourceURL)
}

func TestLoaderApplyDefaults_SourceOverrideCanDisableBuiltInBooleanFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	partial := map[string]any{
		"collector": map[string]any{
			"sources": map[string]any{
				"baidu": map[string]any{
					"enabled":              false,
					"schedule_enabled":     false,
					"detail_fetch_enabled": false,
				},
			},
		},
	}
	data, _ := json.Marshal(partial)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loader := NewLoader(path)
	cfg, err := loader.Load()

	require.NoError(t, err)
	baidu := cfg.Collector.Sources["baidu"]
	assert.False(t, baidu.Enabled)
	assert.False(t, baidu.ScheduleEnabled)
	assert.False(t, baidu.DetailFetchEnabled)
	assert.Equal(t, "百度热搜", baidu.DisplayName)
}

func TestLoaderSetCurrent(t *testing.T) {
	loader := NewLoader("")
	cfg := DefaultConfig()
	cfg.HTTP.Port = 8181

	loader.SetCurrent(cfg)

	assert.Equal(t, 8181, loader.Current().HTTP.Port)
	require.NotNil(t, loader.Get())
	assert.Equal(t, 8181, loader.Get().HTTP.Port)
}

func TestLoaderGetReturnsCopy(t *testing.T) {
	loader := NewLoader("")
	cfg := DefaultConfig()
	cfg.HTTP.Port = 8181
	loader.SetCurrent(cfg)

	loaded := loader.Get()
	loaded.HTTP.Port = 9191

	assert.Equal(t, 8181, loader.Current().HTTP.Port)
}
