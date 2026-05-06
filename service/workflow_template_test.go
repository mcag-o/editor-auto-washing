package service

import (
	"content-hub/domain"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubWorkflowDefinitionRepo struct {
	stored    *domain.WorkflowDefinition
	list      []domain.WorkflowDefinition
	created   *domain.WorkflowDefinition
	updated   *domain.WorkflowDefinition
	upsert    *domain.WorkflowDefinition
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

func (r *stubWorkflowDefinitionRepo) Create(_ context.Context, workflow *domain.WorkflowDefinition) error {
	r.created = workflow
	if r.createErr != nil {
		return r.createErr
	}
	r.stored = workflow
	if workflow != nil {
		r.list = []domain.WorkflowDefinition{*workflow}
	}
	return nil
}

func (r *stubWorkflowDefinitionRepo) Upsert(_ context.Context, workflow *domain.WorkflowDefinition) error {
	r.upsert = workflow
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.stored = workflow
	if workflow != nil {
		r.list = []domain.WorkflowDefinition{*workflow}
	}
	return nil
}

func (r *stubWorkflowDefinitionRepo) Update(_ context.Context, workflow *domain.WorkflowDefinition) error {
	r.updated = workflow
	if r.updateErr != nil {
		return r.updateErr
	}
	r.stored = workflow
	if workflow != nil {
		r.list = []domain.WorkflowDefinition{*workflow}
	}
	return nil
}

func (r *stubWorkflowDefinitionRepo) GetByID(_ context.Context, id string) (*domain.WorkflowDefinition, error) {
	r.gotID = id
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.stored, nil
}

func (r *stubWorkflowDefinitionRepo) List(_ context.Context, limit int) ([]domain.WorkflowDefinition, error) {
	r.gotLimit = limit
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.list, nil
}

func (r *stubWorkflowDefinitionRepo) Delete(_ context.Context, id string) error {
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

func TestWorkflowTemplateServiceCreateAndList(t *testing.T) {
	repo := &stubWorkflowDefinitionRepo{}
	svc := NewWorkflowTemplateService(repo)
	wf := &domain.WorkflowDefinition{ID: "wf-1", Name: "默认流程", Version: "v1", EntryNodeID: "start-1", Nodes: []domain.WorkflowNode{{ID: "start-1", Type: "start", Name: "开始"}}}

	require.NoError(t, svc.Create(t.Context(), wf))
	list, err := svc.List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Same(t, wf, repo.created)
	require.Equal(t, 10, repo.gotLimit)
	require.Equal(t, wf.ID, list[0].ID)
}

func TestWorkflowTemplateServiceGetByID(t *testing.T) {
	wf := &domain.WorkflowDefinition{ID: "wf-1", Name: "默认流程", Version: "v1", EntryNodeID: "start-1", Nodes: []domain.WorkflowNode{{ID: "start-1", Type: "start", Name: "开始"}}}
	repo := &stubWorkflowDefinitionRepo{stored: wf}
	svc := NewWorkflowTemplateService(repo)

	stored, err := svc.GetByID(t.Context(), wf.ID)
	require.NoError(t, err)
	require.Equal(t, wf.ID, repo.gotID)
	require.Same(t, wf, stored)
}

func TestWorkflowTemplateServiceDelete(t *testing.T) {
	wf := &domain.WorkflowDefinition{ID: "wf-1", Name: "默认流程", Version: "v1", EntryNodeID: "start-1", Nodes: []domain.WorkflowNode{{ID: "start-1", Type: "start", Name: "开始"}}}
	repo := &stubWorkflowDefinitionRepo{stored: wf, list: []domain.WorkflowDefinition{*wf}}
	svc := NewWorkflowTemplateService(repo)

	require.NoError(t, svc.Delete(t.Context(), wf.ID))
	require.Equal(t, wf.ID, repo.deletedID)
	require.Nil(t, repo.stored)
	require.Empty(t, repo.list)
}

func TestWorkflowTemplateServiceUpdate(t *testing.T) {
	wf := &domain.WorkflowDefinition{ID: "wf-1", Name: "更新流程", Version: "v2", EntryNodeID: "start-1", Nodes: []domain.WorkflowNode{{ID: "start-1", Type: "start", Name: "开始"}}}
	repo := &stubWorkflowDefinitionRepo{}
	svc := NewWorkflowTemplateService(repo)

	require.NoError(t, svc.Update(t.Context(), wf))
	require.Same(t, wf, repo.updated)
	require.Same(t, wf, repo.stored)
}

func TestWorkflowTemplateServicePropagatesRepoErrors(t *testing.T) {
	repo := &stubWorkflowDefinitionRepo{
		createErr: errors.New("create failed"),
		updateErr: errors.New("update failed"),
		upsertErr: errors.New("upsert failed"),
		deleteErr: errors.New("delete failed"),
		getErr:    errors.New("get failed"),
		listErr:   errors.New("list failed"),
	}
	svc := NewWorkflowTemplateService(repo)
	wf := &domain.WorkflowDefinition{ID: "wf-1", Name: "默认流程", Version: "v1", EntryNodeID: "start-1", Nodes: []domain.WorkflowNode{{ID: "start-1", Type: "start", Name: "开始"}}}

	require.ErrorIs(t, svc.Create(t.Context(), wf), repo.createErr)
	require.ErrorIs(t, svc.Update(t.Context(), wf), repo.updateErr)
	require.ErrorIs(t, svc.Upsert(t.Context(), wf), repo.upsertErr)
	require.ErrorIs(t, svc.Delete(t.Context(), wf.ID), repo.deleteErr)
	_, err := svc.GetByID(t.Context(), wf.ID)
	require.ErrorIs(t, err, repo.getErr)
	_, err = svc.List(t.Context(), 5)
	require.ErrorIs(t, err, repo.listErr)
}
