package service_test

import (
	collectorsvc "content-hub/collector/service"
	"content-hub/domain"
	"content-hub/infra/memory"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBridgeService_CreatesWorkspaceArticleIdempotently(t *testing.T) {
	provider := memory.NewProvider()
	article := domain.NewCollectorArticle("entry-1", "run-1", "github", "openai/gpt-oss", "openai/gpt-oss", "https://github.com/openai/gpt-oss")
	article.Summary = "Open foundation model release"
	article.Body = "# README\n\nDetails"
	require.NoError(t, provider.CollectorArticleRepo().Create(t.Context(), article))

	bridge := collectorsvc.NewBridgeService(provider.CollectorArticleRepo(), provider.WorkspaceRepo())
	first, err := bridge.PushToWorkspace(t.Context(), article.ID)
	require.NoError(t, err)
	second, err := bridge.PushToWorkspace(t.Context(), article.ID)
	require.NoError(t, err)

	assert.Equal(t, first.WorkspaceArticleID, second.WorkspaceArticleID)
	assert.Equal(t, article.ID, first.CollectorArticleID)
	assert.Equal(t, article.ID, first.WorkspaceArticleID)
	workspace, err := provider.WorkspaceRepo().GetByID(t.Context(), first.WorkspaceArticleID)
	require.NoError(t, err)
	assert.Equal(t, "collector", workspace.Source.SourceType)
	assert.Equal(t, "github", workspace.Source.Platform)
	assert.Equal(t, article.CanonicalURL, workspace.Source.URL)
	assert.Equal(t, article.ID, workspace.Metadata["collector_article_id"])
	assert.Equal(t, article.EntryID, workspace.Metadata["collector_entry_id"])
	assert.Equal(t, article.RunID, workspace.Metadata["collector_run_id"])
	assert.Equal(t, article.SourceID, workspace.Metadata["collector_source_id"])
	assert.Equal(t, domain.CollectorArticleBridgeSucceeded, mustCollectorArticle(t, provider, article.ID).BridgeStatus)
	assert.Equal(t, first.WorkspaceArticleID, mustCollectorArticle(t, provider, article.ID).WorkspaceID)

	workspaceItems, err := provider.WorkspaceRepo().List(t.Context(), nil)
	require.NoError(t, err)
	assert.Len(t, workspaceItems, 1)
}

func TestBridgeService_PropagatesNonNotFoundWorkspaceLookupError(t *testing.T) {
	articleRepo := &bridgeStubCollectorArticleRepo{article: domain.NewCollectorArticle("entry-1", "run-1", "github", "openai/gpt-oss", "openai/gpt-oss", "https://github.com/openai/gpt-oss")}
	workspaceRepo := &bridgeStubWorkspaceRepo{getErr: fmt.Errorf("workspace storage unavailable")}

	bridge := collectorsvc.NewBridgeService(articleRepo, workspaceRepo)
	result, err := bridge.PushToWorkspace(t.Context(), articleRepo.article.ID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "workspace storage unavailable")
	assert.Empty(t, workspaceRepo.created)
}

func TestBridgeService_MarksBridgeFailedWhenArticleUpdateFailsAfterWorkspaceCreate(t *testing.T) {
	article := domain.NewCollectorArticle("entry-1", "run-1", "github", "openai/gpt-oss", "openai/gpt-oss", "https://github.com/openai/gpt-oss")
	articleRepo := &bridgeStubCollectorArticleRepo{article: article, updateErr: fmt.Errorf("collector article update failed")}
	workspaceRepo := &bridgeStubWorkspaceRepo{}

	bridge := collectorsvc.NewBridgeService(articleRepo, workspaceRepo)
	result, err := bridge.PushToWorkspace(t.Context(), article.ID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Len(t, workspaceRepo.created, 1)
	assert.Len(t, articleRepo.updates, 2)
	assert.Equal(t, domain.CollectorArticleBridgeFailed, articleRepo.updates[1].BridgeStatus)
	assert.ErrorContains(t, err, "workspace article created but collector article link update failed")
}

func TestBridgeService_SurfacesWorkspaceCompensationFailure(t *testing.T) {
	article := domain.NewCollectorArticle("entry-1", "run-1", "github", "openai/gpt-oss", "openai/gpt-oss", "https://github.com/openai/gpt-oss")
	articleRepo := &bridgeStubCollectorArticleRepo{article: article, updateErr: fmt.Errorf("collector article update failed")}
	workspaceRepo := &bridgeStubWorkspaceRepo{deleteErr: fmt.Errorf("workspace rollback failed")}

	bridge := collectorsvc.NewBridgeService(articleRepo, workspaceRepo)
	result, err := bridge.PushToWorkspace(t.Context(), article.ID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "collector article update failed")
	assert.ErrorContains(t, err, "workspace rollback failed")
}

func TestBridgeService_ReusesExistingWorkspaceLinkIdempotently(t *testing.T) {
	article := domain.NewCollectorArticle("entry-1", "run-1", "github", "openai/gpt-oss", "openai/gpt-oss", "https://github.com/openai/gpt-oss")
	article.WorkspaceID = article.ID
	article.BridgeStatus = domain.CollectorArticleBridgeSucceeded
	articleRepo := &bridgeStubCollectorArticleRepo{article: article}
	workspaceRepo := &bridgeStubWorkspaceRepo{stored: map[string]*domain.ArticleWorkspaceRecord{article.ID: domain.NewArticleWorkspaceRecord(article.ID, article.Title, article.Summary, domain.ArticleWorkspaceSource{}, nil)}}

	bridge := collectorsvc.NewBridgeService(articleRepo, workspaceRepo)
	result, err := bridge.PushToWorkspace(t.Context(), article.ID)

	require.NoError(t, err)
	assert.False(t, result.Created)
	assert.Equal(t, article.ID, result.WorkspaceArticleID)
	assert.Equal(t, article.ID, result.CollectorArticleID)
	assert.Empty(t, workspaceRepo.created)
}

func mustCollectorArticle(t *testing.T, provider *memory.Provider, articleID string) *domain.CollectorArticle {
	t.Helper()
	article, err := provider.CollectorArticleRepo().GetByID(t.Context(), articleID)
	require.NoError(t, err)
	return article
}

type bridgeStubCollectorArticleRepo struct {
	article    *domain.CollectorArticle
	updates    []*domain.CollectorArticle
	updateErr  error
	updateHits int
}

func (r *bridgeStubCollectorArticleRepo) Create(context.Context, *domain.CollectorArticle) error {
	return nil
}
func (r *bridgeStubCollectorArticleRepo) GetByID(context.Context, string) (*domain.CollectorArticle, error) {
	copyValue := *r.article
	return &copyValue, nil
}
func (r *bridgeStubCollectorArticleRepo) GetByEntryID(context.Context, string) (*domain.CollectorArticle, error) {
	copyValue := *r.article
	return &copyValue, nil
}
func (r *bridgeStubCollectorArticleRepo) Update(_ context.Context, article *domain.CollectorArticle) error {
	r.updateHits++
	copyValue := *article
	r.updates = append(r.updates, &copyValue)
	r.article = &copyValue
	if r.updateErr != nil && r.updateHits == 1 {
		return r.updateErr
	}
	return nil
}
func (r *bridgeStubCollectorArticleRepo) Delete(context.Context, string) error { return nil }

type bridgeStubWorkspaceRepo struct {
	stored    map[string]*domain.ArticleWorkspaceRecord
	created   []*domain.ArticleWorkspaceRecord
	getErr    error
	deleteErr error
}

func (r *bridgeStubWorkspaceRepo) Create(_ context.Context, w *domain.ArticleWorkspaceRecord) error {
	if r.stored == nil {
		r.stored = map[string]*domain.ArticleWorkspaceRecord{}
	}
	copyValue := *w
	r.created = append(r.created, &copyValue)
	r.stored[w.ID] = &copyValue
	return nil
}
func (r *bridgeStubWorkspaceRepo) GetByID(_ context.Context, id string) (*domain.ArticleWorkspaceRecord, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if item, ok := r.stored[id]; ok {
		copyValue := *item
		return &copyValue, nil
	}
	return nil, domain.NewNotFoundErr("workspace", id)
}
func (r *bridgeStubWorkspaceRepo) List(context.Context, *string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}
func (r *bridgeStubWorkspaceRepo) ListByIngestionID(context.Context, string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}
func (r *bridgeStubWorkspaceRepo) TransitionStatus(context.Context, string, string, string) error {
	return nil
}
func (r *bridgeStubWorkspaceRepo) Delete(_ context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	delete(r.stored, id)
	return nil
}

var _ repo.WorkspaceRepo = (*bridgeStubWorkspaceRepo)(nil)
