package service

import (
	"content-hub/domain"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubArticleIntakeWorkspaceRepo struct {
	created []*domain.ArticleWorkspaceRecord
	err     error
}

func (r *stubArticleIntakeWorkspaceRepo) Create(_ context.Context, record *domain.ArticleWorkspaceRecord) error {
	if r.err != nil {
		return r.err
	}
	copyValue := *record
	r.created = append(r.created, &copyValue)
	return nil
}

func (r *stubArticleIntakeWorkspaceRepo) GetByID(context.Context, string) (*domain.ArticleWorkspaceRecord, error) {
	return nil, domain.NewNotFoundErr("workspace", "missing")
}

func (r *stubArticleIntakeWorkspaceRepo) List(context.Context, *string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (r *stubArticleIntakeWorkspaceRepo) ListByIngestionID(context.Context, string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (r *stubArticleIntakeWorkspaceRepo) TransitionStatus(context.Context, string, string, string) error {
	return nil
}

func (r *stubArticleIntakeWorkspaceRepo) Delete(context.Context, string) error {
	return nil
}

type stubArticleIntakeRewriteRunner struct {
	called  bool
	lastReq RewriteRunRequest
	err     error
}

func (r *stubArticleIntakeRewriteRunner) Run(_ context.Context, req RewriteRunRequest) (*domain.RewritePipelineRun, error) {
	r.called = true
	r.lastReq = req
	if r.err != nil {
		return nil, r.err
	}
	return &domain.RewritePipelineRun{ID: "run-1"}, nil
}

type stubArticleIntakeRSSItemRepo struct {
	stored    *domain.RSSItemRecord
	items     []*domain.RSSItemRecord
	updateErr error
}

func (r *stubArticleIntakeRSSItemRepo) Create(context.Context, *domain.RSSItemRecord) error {
	return nil
}

func (r *stubArticleIntakeRSSItemRepo) Update(_ context.Context, item *domain.RSSItemRecord) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	copyValue := *item
	r.items = append(r.items, &copyValue)
	return nil
}

func (r *stubArticleIntakeRSSItemRepo) FindDuplicate(context.Context, domain.RSSDuplicateKey) (*domain.RSSItemRecord, error) {
	if r.stored == nil {
		return nil, nil
	}
	copyValue := *r.stored
	return &copyValue, nil
}

func (r *stubArticleIntakeRSSItemRepo) GetByID(context.Context, string) (*domain.RSSItemRecord, error) {
	return nil, nil
}

func (r *stubArticleIntakeRSSItemRepo) List(context.Context, int) ([]domain.RSSItemRecord, error) {
	return nil, nil
}

func TestArticleIntakeServiceCreatesWorkspaceArticleAndTriggersRewrite(t *testing.T) {
	workspaceRepo := &stubArticleIntakeWorkspaceRepo{}
	rewrite := &stubArticleIntakeRewriteRunner{}
	itemRepo := &stubArticleIntakeRSSItemRepo{stored: domain.NewRSSItemRecord("sub-1", "pull-1", "guid-1", "https://example.com/a", "hash-1", "Title")}
	svc := NewArticleIntakeService(itemRepo, workspaceRepo, rewrite)
	publishedAt := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	article := domain.IntakeArticle{
		ExternalID:            "guid-1",
		SourceType:            "rss",
		SubscriptionID:        "sub-1",
		Title:                 "Title",
		Body:                  "Body",
		Summary:               "Summary",
		OriginalURL:           "https://example.com/a",
		PublishedAt:           &publishedAt,
		TargetType:            "wechat-longform",
		SourceProfile:         "sspai",
		RewriteProfileVersion: "latest",
	}

	err := svc.Intake(t.Context(), article)

	require.NoError(t, err)
	require.Len(t, workspaceRepo.created, 1)
	workspace := workspaceRepo.created[0]
	require.Equal(t, "Title", workspace.Title)
	require.Equal(t, "Summary", workspace.Summary)
	require.Equal(t, "rss", workspace.Source.SourceType)
	require.Equal(t, "https://example.com/a", workspace.Source.URL)
	require.Equal(t, "guid-1", workspace.Metadata["rss_guid"])
	require.Equal(t, "guid-1", workspace.Metadata["collector_article_id"])
	require.Equal(t, "sub-1", workspace.Metadata["rss_subscription_id"])
	require.Equal(t, "Body", workspace.Metadata["source_body"])
	require.True(t, rewrite.called)
	require.Equal(t, RewriteRunRequest{
		WorkspaceArticleID: workspace.ID,
		CollectorArticleID: "guid-1",
		Title:              "Title",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "latest",
	}, rewrite.lastReq)
	require.Len(t, itemRepo.items, 1)
	require.Equal(t, domain.RSSItemStatusImported, itemRepo.items[0].Status)
	require.Equal(t, "guid-1", itemRepo.items[0].GUID)
	require.Equal(t, workspace.ID, itemRepo.items[0].WorkspaceArticleID)
	require.NotNil(t, itemRepo.items[0].ImportedAt)
}
