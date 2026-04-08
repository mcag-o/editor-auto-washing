package service

import (
	"content-hub/infra/memory"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspace_ValidTransition(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewWorkspaceArticleService(provider.WorkspaceRepo())

	article, err := svc.CreateArticle(context.Background(), "ws-1", "Test Article")
	require.NoError(t, err)
	assert.Equal(t, "draft", article.Status)

	updated, err := svc.TransitionArticle(context.Background(), article.ID, "rendered", "rendering done")
	require.NoError(t, err)
	assert.Equal(t, "rendered", updated.Status)

	updated, err = svc.TransitionArticle(context.Background(), article.ID, "review_pending", "ready for review")
	require.NoError(t, err)
	assert.Equal(t, "review_pending", updated.Status)

	updated, err = svc.TransitionArticle(context.Background(), article.ID, "approved", "looks good")
	require.NoError(t, err)
	assert.Equal(t, "approved", updated.Status)

	updated, err = svc.TransitionArticle(context.Background(), article.ID, "published", "live")
	require.NoError(t, err)
	assert.Equal(t, "published", updated.Status)
}

func TestWorkspace_InvalidTransition(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewWorkspaceArticleService(provider.WorkspaceRepo())

	article, err := svc.CreateArticle(context.Background(), "ws-2", "Test Article")
	require.NoError(t, err)

	_, err = svc.TransitionArticle(context.Background(), article.ID, "published", "skip steps")
	assert.Error(t, err)
}

func TestWorkspace_TransitionFromApprovedToPublished(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewWorkspaceArticleService(provider.WorkspaceRepo())

	article, _ := svc.CreateArticle(context.Background(), "ws-3", "Test")
	svc.TransitionArticle(context.Background(), article.ID, "rendered", "")
	svc.TransitionArticle(context.Background(), article.ID, "review_pending", "")
	svc.TransitionArticle(context.Background(), article.ID, "approved", "")

	updated, err := svc.TransitionArticle(context.Background(), article.ID, "published", "go live")
	require.NoError(t, err)
	assert.Equal(t, "published", updated.Status)
}

func TestWorkspace_ListArticles(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewWorkspaceArticleService(provider.WorkspaceRepo())

	svc.CreateArticle(context.Background(), "ws-4", "Article 1")
	svc.CreateArticle(context.Background(), "ws-5", "Article 2")

	articles, err := svc.ListArticles(context.Background(), "draft")
	require.NoError(t, err)
	assert.Len(t, articles, 2)
}
