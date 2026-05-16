package service

import (
	workspaceinfra "content-hub/infra/workspace"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceConfigServiceInitCreatesWorkspaceFilesAndDirectories(t *testing.T) {
	root := t.TempDir()
	svc := NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator())

	resolved, err := svc.Init(root)

	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(root, workspaceinfra.WorkspaceConfigFileName))
	assert.FileExists(t, filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName))
	assert.DirExists(t, resolved.Paths.DataDir)
	assert.DirExists(t, resolved.Paths.IncomingDir)
	assert.DirExists(t, resolved.Paths.ArticlesDir)
	assert.DirExists(t, resolved.Paths.DraftsDir)
	assert.DirExists(t, resolved.Paths.RenderedDir)
	assert.DirExists(t, resolved.Paths.ReviewsDir)
	assert.DirExists(t, resolved.Paths.PublishRecordsDir)
	assert.DirExists(t, resolved.Paths.LogsDir)
}

func TestWorkspaceConfigServiceDoctorReturnsErrorsForMissingSecrets(t *testing.T) {
	root := t.TempDir()
	svc := NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator())

	_, err := svc.Init(root)
	require.NoError(t, err)

	report, err := svc.Doctor(root)

	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	assert.Contains(t, report.Errors(), "missing secret for provider profile default: env.LLM_API_KEY")
	assert.Contains(t, report.Errors(), "missing secret for publish profile wechat-review: wechat.main")
}

func TestWorkspaceConfigServiceResolveConfigBuildsRuntimeConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LLM_API_KEY", "runtime-secret")
	svc := NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator())

	_, err := svc.Init(root)
	require.NoError(t, err)

	runtimeCfg, err := svc.RuntimeConfig(root)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "workspace_data"), runtimeCfg.Storage.BasePath)
	assert.Equal(t, filepath.Join(root, "workspace_data", "content-hub.db"), runtimeCfg.Database.Path)
	assert.Equal(t, "openai-compatible", runtimeCfg.LLM.Provider)
	assert.Equal(t, "runtime-secret", runtimeCfg.LLM.APIKey)
	assert.Equal(t, "daily-intelligence", runtimeCfg.Template.DefaultPrompt)
	assert.Empty(t, runtimeCfg.LLM.DefaultProfile)
	assert.Nil(t, runtimeCfg.LLM.Profiles)
}

func TestWorkspaceConfigServiceResolveConfigRejectsUnsupportedPublishPlatform(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LLM_API_KEY", "runtime-secret")
	svc := NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator())

	_, err := svc.Init(root)
	require.NoError(t, err)

	configPath := filepath.Join(root, workspaceinfra.WorkspaceConfigFileName)
	require.NoError(t, os.WriteFile(configPath, []byte(`name: content-workspace
publish_profiles:
  wechat-review:
    platform: unsupported
    account: main
    secret_ref: wechat.main
default_publish_profile: wechat-review
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("wechat:\n  main: publish-secret\n"), 0o600))

	_, err = svc.RuntimeConfig(root)

	require.Error(t, err)
	assert.ErrorContains(t, err, `unsupported publish platform: "unsupported"`)
}

func TestWorkspaceConfigServiceInitPreservesExistingWorkspaceConfig(t *testing.T) {
	root := t.TempDir()
	originalConfig := []byte("name: preserved-workspace\npaths:\n  data_dir: custom-data\n")
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceConfigFileName), originalConfig, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("existing: secret\n"), 0o600))
	svc := NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator())

	resolved, err := svc.Init(root)

	require.NoError(t, err)
	storedConfig, readErr := os.ReadFile(filepath.Join(root, workspaceinfra.WorkspaceConfigFileName))
	require.NoError(t, readErr)
	assert.Equal(t, string(originalConfig), string(storedConfig))
	assert.Equal(t, "preserved-workspace", resolved.Workspace.Name)
	assert.Equal(t, filepath.Join(root, "custom-data"), resolved.Paths.DataDir)
}
