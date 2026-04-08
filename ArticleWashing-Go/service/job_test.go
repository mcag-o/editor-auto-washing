package service

import (
	"content-hub/domain"
	"content-hub/infra/memory"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockExecutor struct{}

func (m *mockExecutor) Execute(ctx context.Context, wf *domain.WorkflowDefinition, wc *domain.WorkflowContext) error {
	return nil
}

func TestJob_SubmitGetList(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), &mockExecutor{})

	job, err := svc.Submit(context.Background(), "test-topic")
	require.NoError(t, err)
	assert.Equal(t, "test-topic", job.Topic)
	assert.Equal(t, "pending", job.Status)

	got, err := svc.GetJob(context.Background(), job.ID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, got.ID)

	jobs, err := svc.ListJobs(context.Background(), nil)
	require.NoError(t, err)
	assert.Len(t, jobs, 1)
}

func TestJob_GetEvents(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), &mockExecutor{})

	job, _ := svc.Submit(context.Background(), "event-test")

	svc.CloseQueue()

	events, err := svc.GetEvents(context.Background(), job.ID)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestJob_QueueLength(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), &mockExecutor{})

	svc.Submit(context.Background(), "topic-1")
	svc.Submit(context.Background(), "topic-2")

	assert.Equal(t, 2, svc.QueueLength())
	svc.CloseQueue()
}
