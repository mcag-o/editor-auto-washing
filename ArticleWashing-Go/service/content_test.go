package service

import (
	"content-hub/domain"
	"content-hub/infra/memory"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDocument_Valid(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewContentService(provider.ArticleRepo(), provider.PublishRepo())

	doc, err := svc.CreateDocument(context.Background(), "Test Article", "Body content", "markdown")

	require.NoError(t, err)
	assert.Equal(t, "Test Article", doc.Title)
	assert.Equal(t, "Body content", doc.Body)
	assert.Equal(t, "markdown", doc.Format)
	assert.NotEmpty(t, doc.ID)
}

func TestCreateDocument_EmptyTitle(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewContentService(provider.ArticleRepo(), provider.PublishRepo())

	doc, err := svc.CreateDocument(context.Background(), "", "Body content", "markdown")

	assert.Error(t, err)
	assert.Nil(t, doc)
}

func TestGetDocument_NotFound(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewContentService(provider.ArticleRepo(), provider.PublishRepo())

	doc, err := svc.GetDocument(context.Background(), "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, doc)
}

func TestGetDocument_Found(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewContentService(provider.ArticleRepo(), provider.PublishRepo())

	created, _ := svc.CreateDocument(context.Background(), "Test", "Body", "markdown")
	doc, err := svc.GetDocument(context.Background(), created.ID)

	require.NoError(t, err)
	assert.Equal(t, "Test", doc.Title)
}

func TestUpdateDocument(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewContentService(provider.ArticleRepo(), provider.PublishRepo())

	created, _ := svc.CreateDocument(context.Background(), "Test", "Original", "markdown")
	updated, err := svc.UpdateDocument(context.Background(), created.ID, "Updated body")

	require.NoError(t, err)
	assert.Equal(t, "Updated body", updated.Body)
}

func TestDeleteDocument(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewContentService(provider.ArticleRepo(), provider.PublishRepo())

	created, _ := svc.CreateDocument(context.Background(), "Test", "Body", "markdown")
	err := svc.DeleteDocument(context.Background(), created.ID)

	require.NoError(t, err)

	_, err = svc.GetDocument(context.Background(), created.ID)
	assert.Error(t, err)
}

func TestListDocuments(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewContentService(provider.ArticleRepo(), provider.PublishRepo())

	svc.CreateDocument(context.Background(), "Alpha", "Body", "markdown")
	svc.CreateDocument(context.Background(), "Beta", "Body", "markdown")

	docs, err := svc.ListDocuments(context.Background(), domain.ListQuery{TitleQuery: "Alpha"})

	require.NoError(t, err)
	assert.Len(t, docs, 1)
	assert.Equal(t, "Alpha", docs[0].Title)
}

func TestGetPublishHistory(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewContentService(provider.ArticleRepo(), provider.PublishRepo())

	history, err := svc.GetPublishHistory(context.Background(), "Test")

	require.NoError(t, err)
	assert.Empty(t, history)
}
