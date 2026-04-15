package service

import (
	"context"
	"errors"
	"testing"

	"content-hub/domain"
	"content-hub/infra/memory"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDraftMaterializerCreatesDraftAndTransitionsWorkspace(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-1", "Original Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	materializer := NewDraftMaterializer(provider.DraftRepo(), provider.WorkspaceRepo())

	draft, err := materializer.Materialize(t.Context(), workspace.ID, map[string]any{
		"title":    "New Title",
		"body":     "Body",
		"template": "daily-intelligence",
	})

	require.NoError(t, err)
	require.NotNil(t, draft)
	assert.Equal(t, workspace.ID, draft.ID)
	assert.Equal(t, "daily-intelligence", draft.Template)
	assert.Equal(t, "New Title", draft.Meta["title"])
	assert.Equal(t, "New Title", draft.Headline["title"])
	assert.Equal(t, []string{"Body"}, draft.Headline["body"])

	storedDraft, err := provider.DraftRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	assert.Equal(t, draft.ID, storedDraft.ID)
	assert.Equal(t, draft.Template, storedDraft.Template)
	assert.Equal(t, draft.Meta["title"], storedDraft.Meta["title"])
	assert.Equal(t, draft.Headline["body"], storedDraft.Headline["body"])

	storedWorkspace, err := provider.WorkspaceRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ArticleWorkspaceStatusDraft, storedWorkspace.Status)
	assert.Equal(t, "rewrite draft materialized", storedWorkspace.Notes)
	assert.Equal(t, []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusDraft}, storedWorkspace.StatusHistory)
}

func TestDraftMaterializerDoesNotTransitionWorkspaceWhenDraftCreateFails(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-2", "Original Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	materializer := NewDraftMaterializer(failingDraftRepo{err: errors.New("draft create failed")}, provider.WorkspaceRepo())

	draft, err := materializer.Materialize(t.Context(), workspace.ID, map[string]any{
		"title":    "New Title",
		"body":     "Body",
		"template": "daily-intelligence",
	})

	require.Error(t, err)
	assert.Nil(t, draft)
	assert.Contains(t, err.Error(), "draft create failed")

	storedWorkspace, getErr := provider.WorkspaceRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, getErr)
	assert.Equal(t, domain.ArticleWorkspaceStatusImported, storedWorkspace.Status)
	assert.Equal(t, []string{domain.ArticleWorkspaceStatusImported}, storedWorkspace.StatusHistory)
	assert.Equal(t, "", storedWorkspace.Notes)
	_, getDraftErr := provider.DraftRepo().GetByID(t.Context(), workspace.ID)
	require.Error(t, getDraftErr)
	assert.Contains(t, getDraftErr.Error(), "draft article-2 not found")
}

func TestDraftMaterializerReturnsWorkspaceTransitionFailureAfterDraftPersist(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-3", "Original Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	materializer := NewDraftMaterializer(provider.DraftRepo(), failingWorkspaceRepoForDraftMaterializer{err: errors.New("workspace transition failed")})

	draft, err := materializer.Materialize(t.Context(), workspace.ID, map[string]any{
		"title":    "New Title",
		"body":     "Body",
		"template": "daily-intelligence",
	})

	require.Error(t, err)
	assert.Nil(t, draft)
	assert.Contains(t, err.Error(), "workspace transition failed")

	storedDraft, getErr := provider.DraftRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, getErr)
	assert.Equal(t, "daily-intelligence", storedDraft.Template)
	assert.Equal(t, "New Title", storedDraft.Headline["title"])

	storedWorkspace, workspaceErr := provider.WorkspaceRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, workspaceErr)
	assert.Equal(t, domain.ArticleWorkspaceStatusImported, storedWorkspace.Status)
}

type failingDraftRepo struct {
	err error
}

func (f failingDraftRepo) Create(context.Context, *domain.ArticleDraft) error {
	return f.err
}

func (f failingDraftRepo) GetByID(context.Context, string) (*domain.ArticleDraft, error) {
	return nil, f.err
}

func (f failingDraftRepo) List(context.Context, *string) ([]domain.ArticleDraft, error) {
	return nil, nil
}

func (f failingDraftRepo) Update(context.Context, string, func(*domain.ArticleDraft)) error {
	return f.err
}

type failingWorkspaceRepoForDraftMaterializer struct {
	err error
}

func (failingWorkspaceRepoForDraftMaterializer) Create(context.Context, *domain.ArticleWorkspaceRecord) error {
	return nil
}

func (f failingWorkspaceRepoForDraftMaterializer) GetByID(context.Context, string) (*domain.ArticleWorkspaceRecord, error) {
	return nil, f.err
}

func (failingWorkspaceRepoForDraftMaterializer) List(context.Context, *string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (failingWorkspaceRepoForDraftMaterializer) ListByIngestionID(context.Context, string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (f failingWorkspaceRepoForDraftMaterializer) TransitionStatus(context.Context, string, string, string) error {
	return f.err
}

func (failingWorkspaceRepoForDraftMaterializer) Delete(context.Context, string) error {
	return nil
}
