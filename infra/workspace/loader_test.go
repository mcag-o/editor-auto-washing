package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderLoadDefaultsWhenWorkspaceFileMissing(t *testing.T) {
	root := t.TempDir()

	loader := NewLoader()
	settings, err := loader.Load(root)

	require.NoError(t, err)
	assert.Equal(t, "content-workspace", settings.Name)
	assert.Equal(t, "default", settings.DefaultProviderProfile)
	assert.Equal(t, "wechat-daily", settings.DefaultArticleProfile)
	assert.Equal(t, "wechat-review", settings.DefaultPublishProfile)
	assert.Equal(t, "workspace_data", settings.Paths.DataDir)
	assert.Equal(t, "incoming", settings.Paths.IncomingDir)
}

func TestLoaderResolveConfigExpandsRelativePathsAndEnvironmentSecrets(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LLM_API_KEY", "env-secret")

	writeWorkspaceFile(t, root, `
name: qa-workspace
paths:
  data_dir: data
  incoming_dir: inbox
default_provider_profile: openai-main
default_article_profile: wechat-daily
default_publish_profile: wechat-main
provider_profiles:
  openai-main:
    provider: openai-compatible
    model: gpt-4.1
    secret_ref: env.LLM_API_KEY
article_profiles:
  wechat-daily:
    style: news-rewrite
    output_format: html
    template: daily-intelligence
publish_profiles:
  wechat-main:
    platform: wechat
    account: main
    secret_ref: wechat.main
automation:
  incoming_dir: automation/incoming
`)
	writeSecretsFile(t, root, "wechat:\n  main: publish-secret\n")

	loader := NewLoader()
	resolved, err := loader.Resolve(root)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "data"), resolved.Paths.DataDir)
	assert.Equal(t, filepath.Join(root, "inbox"), resolved.Paths.IncomingDir)
	assert.Equal(t, filepath.Join(root, "automation", "incoming"), resolved.Paths.AutomationIncomingDir)
	assert.Equal(t, filepath.Join(root, "inbox", "processed"), resolved.Paths.ProcessedDir)
	assert.Equal(t, filepath.Join(root, "inbox", "failed"), resolved.Paths.FailedDir)
	assert.Equal(t, "env-secret", resolved.Secrets["env.LLM_API_KEY"])
	assert.Equal(t, "publish-secret", resolved.Secrets["wechat.main"])
}

func TestLoaderResolveConfigPreservesAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	absIncoming := filepath.Join(t.TempDir(), "incoming")

	writeWorkspaceFile(t, root, "paths:\n  incoming_dir: "+absIncoming+"\n")

	loader := NewLoader()
	resolved, err := loader.Resolve(root)

	require.NoError(t, err)
	assert.Equal(t, absIncoming, resolved.Paths.IncomingDir)
	assert.Equal(t, filepath.Join(absIncoming, "processed"), resolved.Paths.ProcessedDir)
	assert.Equal(t, filepath.Join(absIncoming, "failed"), resolved.Paths.FailedDir)
}

func TestLoaderResolveConfigResolvesPublishProfileEnvironmentSecrets(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WECHAT_TOKEN", "wechat-env-token")

	writeWorkspaceFile(t, root, `
default_provider_profile: openai-main
default_article_profile: wechat-daily
default_publish_profile: wechat-main
provider_profiles:
  openai-main:
    provider: openai-compatible
    model: gpt-4.1
    secret_ref: env.LLM_API_KEY
article_profiles:
  wechat-daily:
    style: news-rewrite
    output_format: html
    template: daily-intelligence
publish_profiles:
  wechat-main:
    platform: wechat
    account: main
    secret_ref: env.WECHAT_TOKEN
`)

	loader := NewLoader()
	resolved, err := loader.Resolve(root)

	require.NoError(t, err)
	assert.Equal(t, "wechat-env-token", resolved.Secrets["env.WECHAT_TOKEN"])
}

func TestLoaderLoadAppliesGranularDefaultsToPartialNestedConfig(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, `
review_policy:
  default_mode: manual
`)

	loader := NewLoader()
	settings, err := loader.Load(root)

	require.NoError(t, err)
	assert.Equal(t, "manual", settings.ReviewPolicy.DefaultMode)
	assert.Equal(t, []string{"missing_secret", "render_failed", "validation_failed"}, settings.ReviewPolicy.BlockingErrors)
}

func TestLoaderLoadPreservesExplicitFalseProviderEnabled(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, `
default_provider_profile: openai-main
provider_profiles:
  openai-main:
    provider: openai-compatible
    model: gpt-4.1
    secret_ref: env.LLM_API_KEY
    enabled: false
`)

	loader := NewLoader()
	settings, err := loader.Load(root)

	require.NoError(t, err)
	assert.False(t, settings.ProviderProfiles["openai-main"].Enabled)
}

func TestLoaderLoadPreservesExplicitFalsePublishFallbackToReview(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, `
default_publish_profile: wechat-main
publish_profiles:
  wechat-main:
    platform: wechat
    account: main
    secret_ref: wechat.main
    fallback_to_review: false
`)

	loader := NewLoader()
	settings, err := loader.Load(root)

	require.NoError(t, err)
	assert.False(t, settings.PublishProfiles["wechat-main"].FallbackToReview)
}

func writeWorkspaceFile(t *testing.T, root, content string) {
	t.Helper()
	require.NoError(t, osWriteFile(filepath.Join(root, WorkspaceConfigFileName), content))
}

func writeSecretsFile(t *testing.T, root, content string) {
	t.Helper()
	require.NoError(t, osWriteFile(filepath.Join(root, WorkspaceSecretsFileName), content))
}

func osWriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
