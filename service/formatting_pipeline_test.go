package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"content-hub/domain"
	"content-hub/infra/memory"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormattingPipelineRenderPersistsAssetAndTransitionsWorkspace(t *testing.T) {
	provider := memory.NewProvider()
	draft := buildFormattingDraft()
	require.NoError(t, provider.DraftRepo().Create(context.Background(), draft))
	workspace := domain.NewArticleWorkspaceRecord(draft.ID, "市场快讯", "摘要", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusDraft
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusDraft}
	require.NoError(t, provider.WorkspaceRepo().Create(context.Background(), workspace))

	pipeline := NewFormattingPipelineService(provider.DraftRepo(), provider.AssetRepo(), provider.WorkspaceRepo(), &stubFormatter{
		rendered: `<html><body><h1>市场快讯</h1></body></html>`,
	}).WithRenderedDir(t.TempDir())

	asset, err := pipeline.Render(context.Background(), draft.ID, "wechat", "daily-intelligence")

	require.NoError(t, err)
	assert.Equal(t, draft.ID, asset.ArticleID)
	assert.Equal(t, "wechat", asset.Platform)
	assert.Equal(t, "html", asset.OutputFormat)
	restored, err := provider.AssetRepo().GetByID(context.Background(), asset.AssetID)
	require.NoError(t, err)
	assert.Equal(t, asset.AssetID, restored.AssetID)
	assert.FileExists(t, filepath.Join(filepath.Dir(asset.ArtifactPath), filepath.Base(asset.ArtifactPath)))
	updatedWorkspace, err := provider.WorkspaceRepo().GetByID(context.Background(), draft.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ArticleWorkspaceStatusRendered, updatedWorkspace.Status)
}

func TestFormattingPipelineValidateReturnsErrorsAndWarnings(t *testing.T) {
	provider := memory.NewProvider()
	draft := buildFormattingDraft()
	require.NoError(t, provider.DraftRepo().Create(context.Background(), draft))
	pipeline := NewFormattingPipelineService(provider.DraftRepo(), provider.AssetRepo(), provider.WorkspaceRepo(), &stubFormatter{
		validation: domain.DraftValidationResult{
			Errors:   []string{"meta.title is required"},
			Warnings: []string{"meta.thumb_media_id is missing"},
		},
	})

	result, err := pipeline.Validate(context.Background(), draft.ID, "wechat", "daily-intelligence")

	require.NoError(t, err)
	assert.Equal(t, []string{"meta.title is required"}, result.Errors)
	assert.Equal(t, []string{"meta.thumb_media_id is missing"}, result.Warnings)
}

func TestFormattingPipelineValidateIncludesRenderedOutputErrors(t *testing.T) {
	provider := memory.NewProvider()
	draft := buildFormattingDraft()
	require.NoError(t, provider.DraftRepo().Create(context.Background(), draft))
	pipeline := NewFormattingPipelineService(provider.DraftRepo(), provider.AssetRepo(), provider.WorkspaceRepo(), &stubFormatter{
		rendered:   `<html><body>{{TITLE}}</body></html>`,
		outputErrs: []string{"rendered HTML still contains unresolved placeholders"},
	})

	result, err := pipeline.Validate(context.Background(), draft.ID, "wechat", "daily-intelligence")

	require.NoError(t, err)
	assert.Contains(t, result.Errors, "rendered HTML still contains unresolved placeholders")
}

func TestFormattingPipelineGetAssetReturnsNotFoundForUnknownAsset(t *testing.T) {
	provider := memory.NewProvider()
	pipeline := NewFormattingPipelineService(provider.DraftRepo(), provider.AssetRepo(), provider.WorkspaceRepo(), &stubFormatter{})

	_, err := pipeline.GetAsset(context.Background(), "missing")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "asset missing not found")
}

func TestFormattingPipelineRenderRollsBackAssetWhenWorkspaceTransitionFails(t *testing.T) {
	provider := memory.NewProvider()
	draft := buildFormattingDraft()
	require.NoError(t, provider.DraftRepo().Create(context.Background(), draft))
	renderedDir := t.TempDir()
	pipeline := NewFormattingPipelineService(provider.DraftRepo(), provider.AssetRepo(), failingWorkspaceRepoForFormatting{err: domain.NewConflictErr("cannot transition")}, &stubFormatter{
		rendered: `<html><body><h1>市场快讯</h1></body></html>`,
	}).WithRenderedDir(renderedDir)

	asset, err := pipeline.Render(context.Background(), draft.ID, "wechat", "daily-intelligence")

	require.Error(t, err)
	assert.Nil(t, asset)
	assets, listErr := provider.AssetRepo().List(context.Background(), draft.ID, "wechat")
	require.NoError(t, listErr)
	assert.Empty(t, assets)
	entries, readErr := os.ReadDir(renderedDir)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

func TestFormattingPipelineRenderReturnsRollbackDeleteFailureWhenWorkspaceTransitionFails(t *testing.T) {
	provider := memory.NewProvider()
	draft := buildFormattingDraft()
	require.NoError(t, provider.DraftRepo().Create(context.Background(), draft))
	renderedDir := t.TempDir()
	pipeline := NewFormattingPipelineService(provider.DraftRepo(), failingAssetRepo{deleteErr: domain.NewInternalErr("delete failed", nil)}, failingWorkspaceRepoForFormatting{err: domain.NewConflictErr("cannot transition")}, &stubFormatter{
		rendered: `<html><body><h1>市场快讯</h1></body></html>`,
	}).WithRenderedDir(renderedDir)

	asset, err := pipeline.Render(context.Background(), draft.ID, "wechat", "daily-intelligence")

	require.Error(t, err)
	assert.Nil(t, asset)
	assert.Contains(t, err.Error(), "cannot transition")
	assert.Contains(t, err.Error(), "delete failed")
}

func TestFormattingPipelineRenderReturnsCleanupFileFailureWhenWorkspaceTransitionFails(t *testing.T) {
	provider := memory.NewProvider()
	draft := buildFormattingDraft()
	require.NoError(t, provider.DraftRepo().Create(context.Background(), draft))
	pipeline := NewFormattingPipelineService(provider.DraftRepo(), provider.AssetRepo(), failingWorkspaceRepoForFormatting{err: domain.NewConflictErr("cannot transition")}, &stubFormatter{
		rendered: `<html><body><h1>市场快讯</h1></body></html>`,
	})
	pipeline.persistRenderedAsset = func(asset *domain.RenderedAssetRecord, dir string) error {
		asset.ArtifactPath = filepath.Join(t.TempDir(), "rendered.html")
		asset.Metadata["artifact_path"] = asset.ArtifactPath
		return nil
	}
	removeRenderedArtifact = func(path string) error {
		return domain.NewInternalErr("remove failed", nil)
	}
	defer func() { removeRenderedArtifact = os.Remove }()

	asset, err := pipeline.Render(context.Background(), draft.ID, "wechat", "daily-intelligence")

	require.Error(t, err)
	assert.Nil(t, asset)
	assert.Contains(t, err.Error(), "cannot transition")
	assert.Contains(t, err.Error(), "remove failed")
}

func TestFormattingPipelineRenderReturnsCleanupFileFailureWhenAssetCreateFails(t *testing.T) {
	provider := memory.NewProvider()
	draft := buildFormattingDraft()
	require.NoError(t, provider.DraftRepo().Create(context.Background(), draft))
	pipeline := NewFormattingPipelineService(provider.DraftRepo(), failingAssetRepo{err: domain.NewInternalErr("asset create failed", nil)}, provider.WorkspaceRepo(), &stubFormatter{
		rendered: `<html><body><h1>市场快讯</h1></body></html>`,
	})
	pipeline.persistRenderedAsset = func(asset *domain.RenderedAssetRecord, dir string) error {
		asset.ArtifactPath = filepath.Join(t.TempDir(), "rendered.html")
		asset.Metadata["artifact_path"] = asset.ArtifactPath
		return nil
	}
	removeRenderedArtifact = func(path string) error {
		return domain.NewInternalErr("remove failed", nil)
	}
	defer func() { removeRenderedArtifact = os.Remove }()

	asset, err := pipeline.Render(context.Background(), draft.ID, "wechat", "daily-intelligence")

	require.Error(t, err)
	assert.Nil(t, asset)
	assert.Contains(t, err.Error(), "asset create failed")
	assert.Contains(t, err.Error(), "remove failed")
}

func TestFormattingPipelineRenderLeavesWorkspaceDraftWhenFilePersistenceFails(t *testing.T) {
	provider := memory.NewProvider()
	draft := buildFormattingDraft()
	require.NoError(t, provider.DraftRepo().Create(context.Background(), draft))
	workspace := domain.NewArticleWorkspaceRecord(draft.ID, "市场快讯", "摘要", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusDraft
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusDraft}
	require.NoError(t, provider.WorkspaceRepo().Create(context.Background(), workspace))
	pipeline := NewFormattingPipelineService(provider.DraftRepo(), provider.AssetRepo(), provider.WorkspaceRepo(), &stubFormatter{
		rendered: `<html><body><h1>市场快讯</h1></body></html>`,
	})
	pipeline.persistRenderedAsset = func(*domain.RenderedAssetRecord, string) error {
		return domain.NewInternalErr("persist rendered asset failed", nil)
	}

	asset, err := pipeline.Render(context.Background(), draft.ID, "wechat", "daily-intelligence")

	require.Error(t, err)
	assert.Nil(t, asset)
	updatedWorkspace, getErr := provider.WorkspaceRepo().GetByID(context.Background(), draft.ID)
	require.NoError(t, getErr)
	assert.Equal(t, domain.ArticleWorkspaceStatusDraft, updatedWorkspace.Status)
	assets, listErr := provider.AssetRepo().List(context.Background(), draft.ID, "wechat")
	require.NoError(t, listErr)
	assert.Empty(t, assets)
}

func TestFormattingPipelineRenderRollsBackFileAndWorkspaceWhenAssetCreateFails(t *testing.T) {
	provider := memory.NewProvider()
	draft := buildFormattingDraft()
	require.NoError(t, provider.DraftRepo().Create(context.Background(), draft))
	workspace := domain.NewArticleWorkspaceRecord(draft.ID, "市场快讯", "摘要", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusDraft
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusDraft}
	require.NoError(t, provider.WorkspaceRepo().Create(context.Background(), workspace))
	renderedDir := t.TempDir()
	pipeline := NewFormattingPipelineService(provider.DraftRepo(), failingAssetRepo{err: domain.NewInternalErr("asset create failed", nil)}, provider.WorkspaceRepo(), &stubFormatter{
		rendered: `<html><body><h1>市场快讯</h1></body></html>`,
	}).WithRenderedDir(renderedDir)

	asset, err := pipeline.Render(context.Background(), draft.ID, "wechat", "daily-intelligence")

	require.Error(t, err)
	assert.Nil(t, asset)
	updatedWorkspace, getErr := provider.WorkspaceRepo().GetByID(context.Background(), draft.ID)
	require.NoError(t, getErr)
	assert.Equal(t, domain.ArticleWorkspaceStatusDraft, updatedWorkspace.Status)
	entries, readErr := os.ReadDir(renderedDir)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

type stubFormatter struct {
	rendered   string
	validation domain.DraftValidationResult
	renderErr  error
	outputErrs []string
}

func (s *stubFormatter) Render(draft *domain.ArticleDraft, templateName string) (string, error) {
	if s.renderErr != nil {
		return "", s.renderErr
	}
	return s.rendered, nil
}

func (s *stubFormatter) ValidateDraft(draft *domain.ArticleDraft, templateName string) domain.DraftValidationResult {
	return s.validation
}

func (s *stubFormatter) ValidateRenderedOutput(html string) []string {
	return s.outputErrs
}

func buildFormattingDraft() *domain.ArticleDraft {
	draft := domain.NewArticleDraft("daily-intelligence")
	draft.Meta["title"] = "市场快讯"
	draft.Meta["digest"] = "盘前摘要"
	draft.Meta["author"] = "编辑部"
	draft.Headline["title"] = "头条"
	draft.Headline["body"] = []string{"正文"}
	draft.Sections = []any{map[string]any{"cn": "版块", "blocks": []map[string]any{{"type": "card", "title": "观察", "body": []string{"内容"}}}}}
	return draft
}

type failingWorkspaceRepoForFormatting struct {
	err error
}

type failingAssetRepo struct {
	err       error
	deleteErr error
}

func (f failingAssetRepo) Create(context.Context, *domain.RenderedAssetRecord) error {
	return f.err
}

func (f failingAssetRepo) GetByID(context.Context, string) (*domain.RenderedAssetRecord, error) {
	return nil, f.err
}

func (f failingAssetRepo) List(context.Context, string, string) ([]domain.RenderedAssetRecord, error) {
	return nil, nil
}

func (f failingAssetRepo) Delete(context.Context, string) error {
	return f.deleteErr
}

func (failingWorkspaceRepoForFormatting) Create(context.Context, *domain.ArticleWorkspaceRecord) error {
	return nil
}

func (failingWorkspaceRepoForFormatting) Update(context.Context, *domain.ArticleWorkspaceRecord) error {
	return nil
}

func (f failingWorkspaceRepoForFormatting) GetByID(context.Context, string) (*domain.ArticleWorkspaceRecord, error) {
	return nil, f.err
}

func (failingWorkspaceRepoForFormatting) List(context.Context, *string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (failingWorkspaceRepoForFormatting) ListByIngestionID(context.Context, string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (f failingWorkspaceRepoForFormatting) TransitionStatus(context.Context, string, string, string) error {
	return f.err
}

func (failingWorkspaceRepoForFormatting) Delete(context.Context, string) error {
	return nil
}
