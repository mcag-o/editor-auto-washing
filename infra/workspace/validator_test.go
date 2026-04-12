package workspace

import (
	"content-hub/domain"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatorDetectsMissingSecrets(t *testing.T) {
	root := t.TempDir()
	settings := domain.DefaultWorkspaceSettings()
	resolved := domain.ResolvedWorkspaceSettings{
		Workspace: settings,
		Paths: domain.ResolvedWorkspacePaths{
			Root:              root,
			ConfigFile:        filepath.Join(root, WorkspaceConfigFileName),
			SecretsFile:       filepath.Join(root, WorkspaceSecretsFileName),
			DataDir:           filepath.Join(root, settings.Paths.DataDir),
			IncomingDir:       filepath.Join(root, settings.Paths.IncomingDir),
			ProcessedDir:      filepath.Join(root, settings.Paths.IncomingDir, "processed"),
			FailedDir:         filepath.Join(root, settings.Paths.IncomingDir, "failed"),
			ArticlesDir:       filepath.Join(root, settings.Paths.ArticlesDir),
			DraftsDir:         filepath.Join(root, settings.Paths.DraftsDir),
			RenderedDir:       filepath.Join(root, settings.Paths.RenderedDir),
			ReviewsDir:        filepath.Join(root, settings.Paths.ReviewsDir),
			PublishRecordsDir: filepath.Join(root, settings.Paths.PublishRecordsDir),
			LogsDir:           filepath.Join(root, settings.Paths.LogsDir),
		},
		Secrets: map[string]string{},
	}

	report := NewValidator().Validate(resolved)

	assert.True(t, report.HasErrors())
	assert.Contains(t, report.Errors(), "missing secret for provider profile default: env.LLM_API_KEY")
	assert.Contains(t, report.Errors(), "missing secret for publish profile wechat-review: wechat.main")
}

func TestValidatorDetectsInvalidDefaultProfiles(t *testing.T) {
	settings := domain.DefaultWorkspaceSettings()
	settings.DefaultProviderProfile = "missing"
	settings.DefaultArticleProfile = "missing-article"
	settings.DefaultPublishProfile = "missing-publish"

	report := NewValidator().Validate(domain.ResolvedWorkspaceSettings{Workspace: settings})

	assert.True(t, report.HasErrors())
	assert.Contains(t, report.Errors(), "missing default provider profile: missing")
	assert.Contains(t, report.Errors(), "missing default article profile: missing-article")
	assert.Contains(t, report.Errors(), "missing default publish profile: missing-publish")
}
