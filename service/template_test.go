package service

import (
	"content-hub/infra/memory"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplate_CreateGetListDelete(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewTemplateService(provider.TemplateRepo())

	created, err := svc.CreateTemplate(context.Background(), "tech", "AI Intro", "Content here")
	require.NoError(t, err)
	assert.Equal(t, "tech", created.Category)
	assert.Equal(t, "AI Intro", created.Name)

	got, err := svc.GetTemplate(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	list, err := svc.ListTemplates(context.Background(), "tech")
	require.NoError(t, err)
	assert.Len(t, list, 1)

	categories, err := svc.ListCategories(context.Background())
	require.NoError(t, err)
	assert.Contains(t, categories, "tech")

	updated, err := svc.UpdateTemplate(context.Background(), created.ID, "New content")
	require.NoError(t, err)
	assert.Equal(t, "New content", updated.Content)

	err = svc.DeleteTemplate(context.Background(), created.ID)
	require.NoError(t, err)

	_, err = svc.GetTemplate(context.Background(), created.ID)
	assert.Error(t, err)
}

func TestTemplate_EmptyCategory(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewTemplateService(provider.TemplateRepo())

	tpl, err := svc.CreateTemplate(context.Background(), "", "Name", "Content")
	assert.Error(t, err)
	assert.Nil(t, tpl)
}
