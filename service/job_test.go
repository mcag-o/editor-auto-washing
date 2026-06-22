package service

import (
	"content-hub/domain"
	"content-hub/infra/memory"
	"context"
	"testing"
	"time"

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
	require.Len(t, events, 1)
	assert.Equal(t, "failed", events[0].Status)
	assert.Contains(t, events[0].Message, "job queue closed before execution")
}

func TestJob_QueueLength(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), &mockExecutor{})

	svc.Submit(context.Background(), "topic-1")
	svc.Submit(context.Background(), "topic-2")

	assert.Equal(t, 2, svc.QueueLength())
	svc.CloseQueue()
}

type blockingExecutor struct {
	started chan struct{}
	stopped chan struct{}
}

func (b *blockingExecutor) Execute(ctx context.Context, wf *domain.WorkflowDefinition, wc *domain.WorkflowContext) error {
	close(b.started)
	<-ctx.Done()
	close(b.stopped)
	return ctx.Err()
}

func TestJob_RunWorkerStopsOnContextCancellation(t *testing.T) {
	provider := memory.NewProvider()
	exec := &blockingExecutor{started: make(chan struct{}), stopped: make(chan struct{})}
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), exec)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunWorker(ctx)
		close(done)
	}()

	_, err := svc.Submit(context.Background(), "run-once")
	require.NoError(t, err)
	<-exec.started
	cancel()

	select {
	case <-exec.stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop executor on cancellation")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit on cancellation")
	}

	jobList, listErr := svc.ListJobs(context.Background(), nil)
	require.NoError(t, listErr)
	require.Len(t, jobList, 1)
	assert.Equal(t, "failed", jobList[0].Status)
}

func TestJob_SubmitAfterCloseReturnsError(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), &mockExecutor{})
	svc.CloseQueue()

	job, err := svc.Submit(context.Background(), "closed")

	require.Error(t, err)
	assert.Nil(t, job)
	assert.Contains(t, err.Error(), "job queue closed")
	jobs, listErr := svc.ListJobs(context.Background(), nil)
	require.NoError(t, listErr)
	assert.Empty(t, jobs)
}

func TestJob_CloseQueueIsIdempotent(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), &mockExecutor{})

	require.NotPanics(t, func() { svc.CloseQueue() })
	require.NotPanics(t, func() { svc.CloseQueue() })
}

func TestJob_SubmitWhenQueueIsFullDoesNotPersistJob(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), &mockExecutor{})
	svc.queue = make(chan *domain.JobRun, 1)
	_, err := svc.Submit(context.Background(), "first")
	require.NoError(t, err)

	job, err := svc.Submit(context.Background(), "second")

	require.Error(t, err)
	assert.Nil(t, job)
	assert.Contains(t, err.Error(), "job queue full")
	jobs, listErr := svc.ListJobs(context.Background(), nil)
	require.NoError(t, listErr)
	assert.Len(t, jobs, 1)
	assert.Equal(t, "first", jobs[0].Topic)
}

func TestJob_CloseQueueMarksPendingJobsFailed(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), &mockExecutor{})
	job, err := svc.Submit(context.Background(), "pending")
	require.NoError(t, err)

	svc.CloseQueue()

	stored, getErr := svc.GetJob(context.Background(), job.ID)
	require.NoError(t, getErr)
	assert.Equal(t, "failed", stored.Status)
	require.NotNil(t, stored.Result)
	assert.Contains(t, *stored.Result, "job queue closed before execution")
}

type failingJobRepo struct {
	createErr error
	updateErr error
	deleteErr error
	jobs      map[string]*domain.JobRun
}

func (r *failingJobRepo) Create(_ context.Context, j *domain.JobRun) error {
	if r.createErr != nil {
		return r.createErr
	}
	if r.jobs == nil {
		r.jobs = map[string]*domain.JobRun{}
	}
	r.jobs[j.ID] = j
	return nil
}

func (r *failingJobRepo) GetByID(_ context.Context, id string) (*domain.JobRun, error) {
	job, ok := r.jobs[id]
	if !ok {
		return nil, domain.NewNotFoundErr("job", id)
	}
	return job, nil
}

func (r *failingJobRepo) List(_ context.Context, status *string) ([]domain.JobRun, error) {
	result := []domain.JobRun{}
	for _, job := range r.jobs {
		result = append(result, *job)
	}
	return result, nil
}

func (r *failingJobRepo) Update(_ context.Context, id string, fn func(*domain.JobRun)) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	job, ok := r.jobs[id]
	if !ok {
		return domain.NewNotFoundErr("job", id)
	}
	fn(job)
	return nil
}

func (r *failingJobRepo) Delete(_ context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	delete(r.jobs, id)
	return nil
}

type failingJobEventRepo struct {
	addErr error
	events []*domain.JobEvent
}

func (r *failingJobEventRepo) Add(_ context.Context, evt *domain.JobEvent) error {
	if r.addErr != nil {
		return r.addErr
	}
	r.events = append(r.events, evt)
	return nil
}

func (r *failingJobEventRepo) ListByJob(_ context.Context, jobID string) ([]domain.JobEvent, error) {
	result := []domain.JobEvent{}
	for _, evt := range r.events {
		if evt.JobID == jobID {
			result = append(result, *evt)
		}
	}
	return result, nil
}

func TestJob_CloseQueueReturnsErrorWhenPendingJobUpdateFails(t *testing.T) {
	repo := &failingJobRepo{updateErr: domain.NewInternalErr("update failed", nil), jobs: map[string]*domain.JobRun{}}
	events := &failingJobEventRepo{}
	svc := NewJobService(repo, events, &mockExecutor{})
	job, err := svc.Submit(context.Background(), "pending")
	require.NoError(t, err)

	err = svc.CloseQueueWithContext(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "update failed")
	stored, getErr := svc.GetJob(context.Background(), job.ID)
	require.NoError(t, getErr)
	assert.Equal(t, "pending", stored.Status)
}

func TestJob_CloseQueueReturnsErrorWhenPendingJobEventWriteFails(t *testing.T) {
	repo := &failingJobRepo{jobs: map[string]*domain.JobRun{}}
	events := &failingJobEventRepo{addErr: domain.NewInternalErr("event failed", nil)}
	svc := NewJobService(repo, events, &mockExecutor{})
	_, err := svc.Submit(context.Background(), "pending")
	require.NoError(t, err)

	err = svc.CloseQueueWithContext(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "event failed")
}

type executorCalled struct {
	called bool
}

type failingExecutor struct{ err error }

func (e *executorCalled) Execute(ctx context.Context, wf *domain.WorkflowDefinition, wc *domain.WorkflowContext) error {
	e.called = true
	return nil
}

func (e failingExecutor) Execute(ctx context.Context, wf *domain.WorkflowDefinition, wc *domain.WorkflowContext) error {
	return e.err
}

func TestJob_ExecuteJobDoesNotRunWorkflowWhenRunningStatePersistFails(t *testing.T) {
	repo := &failingJobRepo{updateErr: domain.NewInternalErr("running update failed", nil), jobs: map[string]*domain.JobRun{}}
	events := &failingJobEventRepo{}
	exec := &executorCalled{}
	svc := NewJobService(repo, events, exec)
	job := domain.NewJobRun("blocked")
	require.NoError(t, repo.Create(context.Background(), job))

	svc.executeJob(context.Background(), job)

	assert.False(t, exec.called)
	stored, err := repo.GetByID(context.Background(), job.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", stored.Status)
}

func TestJob_ExecuteJobSurfacesTerminalStatePersistFailureAsEvent(t *testing.T) {
	repo := &failingJobRepo{jobs: map[string]*domain.JobRun{}}
	events := &failingJobEventRepo{}
	svc := NewJobService(repo, events, &mockExecutor{})
	job := domain.NewJobRun("run-once")
	require.NoError(t, repo.Create(context.Background(), job))
	require.NoError(t, repo.Update(context.Background(), job.ID, func(j *domain.JobRun) { j.Status = "running" }))
	repo.updateErr = domain.NewInternalErr("complete update failed", nil)

	svc.executeJob(context.Background(), job)

	stored, err := repo.GetByID(context.Background(), job.ID)
	require.NoError(t, err)
	assert.Equal(t, "running", stored.Status)
	eventList, listErr := events.ListByJob(context.Background(), job.ID)
	require.NoError(t, listErr)
	require.Len(t, eventList, 1)
	assert.Contains(t, eventList[0].Message, "complete update failed")
}

func TestJob_ExecuteJobDoesNotWriteCompletedEventWhenTerminalUpdateFails(t *testing.T) {
	repo := &failingJobRepo{jobs: map[string]*domain.JobRun{}}
	events := &failingJobEventRepo{}
	svc := NewJobService(repo, events, &mockExecutor{})
	job := domain.NewJobRun("run-once")
	require.NoError(t, repo.Create(context.Background(), job))
	require.NoError(t, repo.Update(context.Background(), job.ID, func(j *domain.JobRun) { j.Status = "running" }))
	repo.updateErr = domain.NewInternalErr("complete update failed", nil)

	svc.executeJob(context.Background(), job)

	eventList, listErr := events.ListByJob(context.Background(), job.ID)
	require.NoError(t, listErr)
	require.Len(t, eventList, 1)
	assert.Equal(t, "failed", eventList[0].Status)
	assert.NotContains(t, eventList[0].Message, "job completed")
}

func TestJob_ExecuteJobDoesNotRunFailureEventBeforeFailureStatePersists(t *testing.T) {
	repo := &failingJobRepo{jobs: map[string]*domain.JobRun{}}
	events := &failingJobEventRepo{}
	svc := NewJobService(repo, events, failingExecutor{err: domain.NewInternalErr("boom", nil)})
	job := domain.NewJobRun("boom")
	require.NoError(t, repo.Create(context.Background(), job))
	require.NoError(t, repo.Update(context.Background(), job.ID, func(j *domain.JobRun) { j.Status = "running" }))
	repo.updateErr = domain.NewInternalErr("failure update failed", nil)

	svc.executeJob(context.Background(), job)

	eventList, listErr := events.ListByJob(context.Background(), job.ID)
	require.NoError(t, listErr)
	require.Len(t, eventList, 1)
	assert.Equal(t, "failed", eventList[0].Status)
	assert.Contains(t, eventList[0].Message, "failure update failed")
	assert.NotContains(t, eventList[0].Message, "boom")
}

func TestJobServiceExposesAutomationStatusAndHealthSnapshots(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), &mockExecutor{})

	status, err := svc.AutomationStatus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "idle", status.State)
	assert.Equal(t, 0, status.QueueDepth)

	health, err := svc.AutomationHealth(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, "running", health.Checks["queue"])
}

func TestJobServiceCancelMarksPendingJobCancelledAndRecordsEvent(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), &mockExecutor{})
	job, err := svc.Submit(context.Background(), "cancel-me")
	require.NoError(t, err)

	updated, err := svc.Cancel(context.Background(), job.ID, "operator request")
	require.NoError(t, err)
	assert.Equal(t, "cancelled", updated.Status)
	require.NotNil(t, updated.Result)
	assert.Contains(t, *updated.Result, "operator request")

	events, err := svc.GetEvents(context.Background(), job.ID)
	require.NoError(t, err)
	require.NotEmpty(t, events)
	assert.Equal(t, "cancelled", events[len(events)-1].Status)
}

type externalIntakeProcessorStub struct {
	workspaceID string
}

func (p *externalIntakeProcessorStub) ProcessWorkspace(_ context.Context, workspaceArticleID string) error {
	p.workspaceID = workspaceArticleID
	return nil
}

func TestJob_SubmitWithArtifactPersistsArtifactPath(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), &mockExecutor{})

	job, err := svc.SubmitWithArtifact(context.Background(), ExternalIntakeProcessTopic, "workspace-1")

	require.NoError(t, err)
	require.NotNil(t, job.ArtifactPath)
	assert.Equal(t, "workspace-1", *job.ArtifactPath)
}

func TestJob_ExternalIntakeTopicDispatchesProcessor(t *testing.T) {
	provider := memory.NewProvider()
	processor := &externalIntakeProcessorStub{}
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), &mockExecutor{})
	svc.SetExternalIntakeProcessor(processor)
	job, err := svc.SubmitWithArtifact(context.Background(), ExternalIntakeProcessTopic, "workspace-1")
	require.NoError(t, err)

	svc.executeJob(context.Background(), job)

	assert.Equal(t, "workspace-1", processor.workspaceID)
	stored, err := svc.GetJob(context.Background(), job.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", stored.Status)
}

func TestJob_UnknownTopicFailsJob(t *testing.T) {
	provider := memory.NewProvider()
	svc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), &mockExecutor{})
	job, err := svc.Submit(context.Background(), "unknown")
	require.NoError(t, err)

	svc.executeJob(context.Background(), job)

	stored, err := svc.GetJob(context.Background(), job.ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", stored.Status)
	require.NotNil(t, stored.Result)
	assert.Contains(t, *stored.Result, "unknown job topic unknown")
}
