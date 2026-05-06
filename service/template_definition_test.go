package service

import (
	"content-hub/domain"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubTemplateDefinitionRepo struct {
	stored    *domain.TemplateDefinition
	list      []domain.TemplateDefinition
	created   *domain.TemplateDefinition
	updated   *domain.TemplateDefinition
	upsert    *domain.TemplateDefinition
	createErr error
	updateErr error
	deleteErr error
	upsertErr error
	getErr    error
	listErr   error
	deletedID string
	gotID     string
	gotLimit  int
}

func (r *stubTemplateDefinitionRepo) Create(_ context.Context, template *domain.TemplateDefinition) error {
	r.created = template
	if r.createErr != nil {
		return r.createErr
	}
	r.stored = template
	if template != nil {
		r.list = []domain.TemplateDefinition{*template}
	}
	return nil
}

func (r *stubTemplateDefinitionRepo) Upsert(_ context.Context, template *domain.TemplateDefinition) error {
	r.upsert = template
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.stored = template
	if template != nil {
		r.list = []domain.TemplateDefinition{*template}
	}
	return nil
}

func (r *stubTemplateDefinitionRepo) Update(_ context.Context, template *domain.TemplateDefinition) error {
	r.updated = template
	if r.updateErr != nil {
		return r.updateErr
	}
	r.stored = template
	if template != nil {
		r.list = []domain.TemplateDefinition{*template}
	}
	return nil
}

func (r *stubTemplateDefinitionRepo) GetByID(_ context.Context, id string) (*domain.TemplateDefinition, error) {
	r.gotID = id
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.stored, nil
}

func (r *stubTemplateDefinitionRepo) List(_ context.Context, limit int) ([]domain.TemplateDefinition, error) {
	r.gotLimit = limit
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.list, nil
}

func (r *stubTemplateDefinitionRepo) Delete(_ context.Context, id string) error {
	r.deletedID = id
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if r.stored != nil && r.stored.ID == id {
		r.stored = nil
	}
	filtered := r.list[:0]
	for _, item := range r.list {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	r.list = filtered
	return nil
}

func TestTemplateDefinitionServiceCreateAndGet(t *testing.T) {
	repo := &stubTemplateDefinitionRepo{}
	svc := NewTemplateDefinitionService(repo)
	tpl := &domain.TemplateDefinition{ID: "tpl-1", Name: "标题模板", Type: "prompt", Version: "v1", Content: "标题：{{title}}"}

	require.NoError(t, svc.Create(t.Context(), tpl))
	stored, err := svc.GetByID(t.Context(), tpl.ID)
	require.NoError(t, err)
	require.Same(t, tpl, repo.created)
	require.Equal(t, tpl.ID, repo.gotID)
	require.Equal(t, tpl.Name, stored.Name)
}

func TestTemplateDefinitionServiceList(t *testing.T) {
	tpl := domain.TemplateDefinition{ID: "tpl-1", Name: "标题模板", Type: "prompt", Version: "v1", Content: "标题：{{title}}"}
	repo := &stubTemplateDefinitionRepo{list: []domain.TemplateDefinition{tpl}}
	svc := NewTemplateDefinitionService(repo)

	list, err := svc.List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, 10, repo.gotLimit)
	require.Equal(t, tpl.ID, list[0].ID)
}

func TestTemplateDefinitionServiceDelete(t *testing.T) {
	tpl := &domain.TemplateDefinition{ID: "tpl-1", Name: "标题模板", Type: "prompt", Version: "v1", Content: "标题：{{title}}"}
	repo := &stubTemplateDefinitionRepo{stored: tpl, list: []domain.TemplateDefinition{*tpl}}
	svc := NewTemplateDefinitionService(repo)

	require.NoError(t, svc.Delete(t.Context(), tpl.ID))
	require.Equal(t, tpl.ID, repo.deletedID)
	require.Nil(t, repo.stored)
	require.Empty(t, repo.list)
}

func TestTemplateDefinitionServiceUpdate(t *testing.T) {
	tpl := &domain.TemplateDefinition{ID: "tpl-1", Name: "修复模板", Type: "stage", Version: "v2", Content: "标题：{{title}}"}
	repo := &stubTemplateDefinitionRepo{}
	svc := NewTemplateDefinitionService(repo)

	require.NoError(t, svc.Update(t.Context(), tpl))
	require.Same(t, tpl, repo.updated)
	require.Same(t, tpl, repo.stored)
}

func TestTemplateDefinitionServicePropagatesRepoErrors(t *testing.T) {
	repo := &stubTemplateDefinitionRepo{
		createErr: errors.New("create failed"),
		updateErr: errors.New("update failed"),
		upsertErr: errors.New("upsert failed"),
		deleteErr: errors.New("delete failed"),
		getErr:    errors.New("get failed"),
		listErr:   errors.New("list failed"),
	}
	svc := NewTemplateDefinitionService(repo)
	tpl := &domain.TemplateDefinition{ID: "tpl-1", Name: "标题模板", Type: "prompt", Version: "v1", Content: "标题：{{title}}"}

	require.ErrorIs(t, svc.Create(t.Context(), tpl), repo.createErr)
	require.ErrorIs(t, svc.Update(t.Context(), tpl), repo.updateErr)
	require.ErrorIs(t, svc.Upsert(t.Context(), tpl), repo.upsertErr)
	require.ErrorIs(t, svc.Delete(t.Context(), tpl.ID), repo.deleteErr)
	_, err := svc.GetByID(t.Context(), tpl.ID)
	require.ErrorIs(t, err, repo.getErr)
	_, err = svc.List(t.Context(), 5)
	require.ErrorIs(t, err, repo.listErr)
}
