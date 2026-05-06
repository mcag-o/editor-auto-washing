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
	upsert    *domain.TemplateDefinition
	upsertErr error
	getErr    error
	listErr   error
	gotID     string
	gotLimit  int
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

func TestTemplateDefinitionServiceCreateAndGet(t *testing.T) {
	repo := &stubTemplateDefinitionRepo{}
	svc := NewTemplateDefinitionService(repo)
	tpl := &domain.TemplateDefinition{ID: "tpl-1", Name: "标题模板", Type: "prompt", Version: "v1", Content: "标题：{{title}}"}

	require.NoError(t, svc.Upsert(t.Context(), tpl))
	stored, err := svc.GetByID(t.Context(), tpl.ID)
	require.NoError(t, err)
	require.Same(t, tpl, repo.upsert)
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

func TestTemplateDefinitionServicePropagatesRepoErrors(t *testing.T) {
	repo := &stubTemplateDefinitionRepo{
		upsertErr: errors.New("upsert failed"),
		getErr:    errors.New("get failed"),
		listErr:   errors.New("list failed"),
	}
	svc := NewTemplateDefinitionService(repo)
	tpl := &domain.TemplateDefinition{ID: "tpl-1", Name: "标题模板", Type: "prompt", Version: "v1", Content: "标题：{{title}}"}

	require.ErrorIs(t, svc.Upsert(t.Context(), tpl), repo.upsertErr)
	_, err := svc.GetByID(t.Context(), tpl.ID)
	require.ErrorIs(t, err, repo.getErr)
	_, err = svc.List(t.Context(), 5)
	require.ErrorIs(t, err, repo.listErr)
}
