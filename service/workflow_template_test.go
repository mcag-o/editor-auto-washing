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
	upsert    *domain.WorkflowDefinition
	upsertErr error
	getErr    error
	listErr   error
	gotID     string
	gotLimit  int
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

func TestWorkflowTemplateServiceCreateAndList(t *testing.T) {
	repo := &stubWorkflowDefinitionRepo{}
	svc := NewWorkflowTemplateService(repo)
	wf := &domain.WorkflowDefinition{ID: "wf-1", Name: "默认流程", Version: "v1", EntryNodeID: "start-1", Nodes: []domain.WorkflowNode{{ID: "start-1", Type: "start", Name: "开始"}}}

	require.NoError(t, svc.Upsert(t.Context(), wf))
	list, err := svc.List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Same(t, wf, repo.upsert)
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

func TestWorkflowTemplateServicePropagatesRepoErrors(t *testing.T) {
	repo := &stubWorkflowDefinitionRepo{
		upsertErr: errors.New("upsert failed"),
		getErr:    errors.New("get failed"),
		listErr:   errors.New("list failed"),
	}
	svc := NewWorkflowTemplateService(repo)
	wf := &domain.WorkflowDefinition{ID: "wf-1", Name: "默认流程", Version: "v1", EntryNodeID: "start-1", Nodes: []domain.WorkflowNode{{ID: "start-1", Type: "start", Name: "开始"}}}

	require.ErrorIs(t, svc.Upsert(t.Context(), wf), repo.upsertErr)
	_, err := svc.GetByID(t.Context(), wf.ID)
	require.ErrorIs(t, err, repo.getErr)
	_, err = svc.List(t.Context(), 5)
	require.ErrorIs(t, err, repo.listErr)
}
