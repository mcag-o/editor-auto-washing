package service

import (
	"content-hub/domain"
	"content-hub/infra/memory"
	workspaceinfra "content-hub/infra/workspace"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunDaemonLoopsUntilStopped(t *testing.T) {
	root := newAutomationWorkspace(t)
	provider := memory.NewProvider()
	service := newAutomationServiceForTest(t, root, provider)

	incomingDir := filepath.Join(root, "incoming")
	require.NoError(t, os.WriteFile(filepath.Join(incomingDir, "bundle-1.json"), []byte(`{"items":[]}`), 0o644))

	done := make(chan *domain.AutomationRunResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := service.RunDaemon(context.Background(), root, 10*time.Millisecond)
		if err != nil {
			errCh <- err
			return
		}
		done <- result
	}()

	require.Eventually(t, func() bool {
		status, err := service.Status(context.Background(), root)
		return err == nil && status.State == "running"
	}, time.Second, 10*time.Millisecond)

	stopResult, err := service.Stop(context.Background(), root)
	require.NoError(t, err)
	assert.True(t, stopResult.Stopped)

	select {
	case err := <-errCh:
		t.Fatalf("unexpected daemon error: %v", err)
	case result := <-done:
		assert.Equal(t, "daemon", result.Mode)
		assert.True(t, result.Stopped)
		assert.GreaterOrEqual(t, result.RunsExecuted, 1)
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not stop after stop signal")
	}
}

func TestStopReturnsConflictWhenDaemonNotRunning(t *testing.T) {
	root := newAutomationWorkspace(t)
	provider := memory.NewProvider()
	service := newAutomationServiceForTest(t, root, provider)

	result, err := service.Stop(context.Background(), root)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "automation daemon is not running")
}

func TestRunOnceUsesFolderSourceProcessingWhenConfiguredAlongsideIngestion(t *testing.T) {
	root := newAutomationWorkspace(t)
	provider := memory.NewProvider()
	intake := &stubAutomationFolderIntake{
		runOnceResult: automationFolderRunSummary{
			ScannedFiles:       2,
			ImportedFiles:      1,
			SkippedFiles:       1,
			FailedFiles:        0,
			ProcessedPending:   1,
			ProcessedFailed:    0,
			CompletedDocuments: 1,
		},
	}
	service := newAutomationServiceWithIngestionAndFolderIntakeForTest(root, provider, intake)

	incomingDir := filepath.Join(root, "incoming")
	require.NoError(t, os.WriteFile(filepath.Join(incomingDir, "bundle-1.json"), []byte(`{"items":[]}`), 0o644))

	result, err := service.RunOnce(context.Background(), root)

	require.NoError(t, err)
	assert.Equal(t, 1, intake.runOnceCalls)
	assert.Equal(t, 0, intake.retryFailedCalls)
	assert.Equal(t, 1, result.Summary["imported_files"])
	assert.Equal(t, 2, result.Summary["scanned_files"])
	assert.Equal(t, 1, result.Summary["processed_pending_documents"])
	assert.Equal(t, 1, result.Summary["completed_documents"])
}

func TestRetryFailedUsesFolderSourceProcessingWhenConfiguredAlongsideIngestion(t *testing.T) {
	root := newAutomationWorkspace(t)
	provider := memory.NewProvider()
	intake := &stubAutomationFolderIntake{
		retryResult: automationFolderRunSummary{
			ProcessedPending:   0,
			ProcessedFailed:    2,
			CompletedDocuments: 2,
			FailedDocuments:    0,
		},
	}
	service := newAutomationServiceWithIngestionAndFolderIntakeForTest(root, provider, intake)

	result, err := service.RetryFailed(context.Background(), root)

	require.NoError(t, err)
	assert.Equal(t, 0, intake.runOnceCalls)
	assert.Equal(t, 1, intake.retryFailedCalls)
	assert.Equal(t, 2, result.Summary["processed_failed_documents"])
	assert.Equal(t, 2, result.Summary["completed_documents"])
}

func TestRetryFailedReturnsFolderIntakeError(t *testing.T) {
	root := newAutomationWorkspace(t)
	service := newAutomationServiceWithFolderIntakeForTest(root, &stubAutomationFolderIntake{retryErr: errors.New("retry exploded")})

	result, err := service.RetryFailed(context.Background(), root)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "retry exploded")
}

func TestRunWorkerExecutesRegisteredAutomationNodes(t *testing.T) {
	root := newAutomationWorkspace(t)
	provider := memory.NewProvider()
	ingestionSvc := NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), provider, workspaceinfra.NewLoader())
	automationSvc := NewAutomationService(NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator()), ingestionSvc, nil, nil)
	engine := BuildDefaultWorkflowEngine(root, automationSvc)
	jobSvc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), engine)
	automationSvc.SetJobService(jobSvc)

	incomingDir := filepath.Join(root, "incoming")
	require.NoError(t, os.WriteFile(filepath.Join(incomingDir, "bundle-1.json"), []byte(`{"items":[]}`), 0o644))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		jobSvc.RunWorker(ctx)
		close(done)
	}()

	job, err := jobSvc.Submit(context.Background(), "run-once")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		stored, err := jobSvc.GetJob(context.Background(), job.ID)
		return err == nil && stored.Status == "completed"
	}, time.Second, 10*time.Millisecond)

	status, err := automationSvc.Status(context.Background(), root)
	require.NoError(t, err)
	assert.Equal(t, "run-once", status.LastCommand)
	assert.True(t, status.LastRunSucceeded)

	cancel()
	<-done
}

func TestStartDaemonReturnsImmediatelyAndStopEndsBackgroundLoop(t *testing.T) {
	root := newAutomationWorkspace(t)
	provider := memory.NewProvider()
	service := newAutomationServiceForTest(t, root, provider)

	result, err := service.StartDaemon(context.Background(), root, 10*time.Millisecond)

	require.NoError(t, err)
	assert.Equal(t, "daemon", result.Mode)
	assert.Equal(t, "running", result.State)

	require.Eventually(t, func() bool {
		status, statusErr := service.Status(context.Background(), root)
		return statusErr == nil && status.State == "running"
	}, time.Second, 10*time.Millisecond)

	stopResult, stopErr := service.Stop(context.Background(), root)
	require.NoError(t, stopErr)
	assert.True(t, stopResult.Stopped)

	require.Eventually(t, func() bool {
		status, statusErr := service.Status(context.Background(), root)
		return statusErr == nil && status.State == "idle"
	}, time.Second, 10*time.Millisecond)
}

func TestStatusPrefersLiveDaemonStateOverPersistedIdleSnapshot(t *testing.T) {
	root := newAutomationWorkspace(t)
	provider := memory.NewProvider()
	service := newAutomationServiceForTest(t, root, provider)

	_, err := service.StartDaemon(context.Background(), root, 10*time.Millisecond)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		status, statusErr := service.Status(context.Background(), root)
		return statusErr == nil && status.State == "running"
	}, time.Second, 10*time.Millisecond)

	_, stopErr := service.Stop(context.Background(), root)
	require.NoError(t, stopErr)
}

func TestRunWorkerDaemonCommandCompletesWithoutBlockingWorkerIndefinitely(t *testing.T) {
	root := newAutomationWorkspace(t)
	provider := memory.NewProvider()
	ingestionSvc := NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), provider, workspaceinfra.NewLoader())
	automationSvc := NewAutomationService(NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator()), ingestionSvc, nil, nil)
	engine := BuildDefaultWorkflowEngine(root, automationSvc)
	jobSvc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), engine)
	automationSvc.SetJobService(jobSvc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go jobSvc.RunWorker(ctx)

	job, err := jobSvc.Submit(context.Background(), "daemon")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		stored, getErr := jobSvc.GetJob(context.Background(), job.ID)
		return getErr == nil && stored.Status == "completed"
	}, time.Second, 10*time.Millisecond)

	status, err := automationSvc.Status(context.Background(), root)
	require.NoError(t, err)
	assert.Equal(t, "running", status.State)

	_, stopErr := automationSvc.Stop(context.Background(), root)
	require.NoError(t, stopErr)
}

func TestRunWorkerExecutesRetryFailedWithFolderIntakeAutomation(t *testing.T) {
	root := newAutomationWorkspace(t)
	intake := &stubAutomationFolderIntake{retryResult: automationFolderRunSummary{ProcessedFailed: 1, CompletedDocuments: 1}}
	automationSvc := newAutomationServiceWithFolderIntakeForTest(root, intake)
	engine := BuildDefaultWorkflowEngine(root, automationSvc)
	jobSvc := NewJobService(memory.NewProvider().JobRepo(), memory.NewProvider().JobEventRepo(), engine)
	automationSvc.SetJobService(jobSvc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		jobSvc.RunWorker(ctx)
		close(done)
	}()

	job, err := jobSvc.Submit(context.Background(), "retry-failed")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		stored, getErr := jobSvc.GetJob(context.Background(), job.ID)
		return getErr == nil && stored.Status == "completed"
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, 1, intake.retryFailedCalls)
	status, err := automationSvc.Status(context.Background(), root)
	require.NoError(t, err)
	assert.Equal(t, "retry-failed", status.LastCommand)
	assert.True(t, status.LastRunSucceeded)

	cancel()
	<-done
}

func newAutomationWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceConfigFileName), []byte("name: automation\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))
	for _, dir := range []string{"incoming", "incoming/processed", "incoming/failed"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
	}
	return root
}

func newAutomationServiceForTest(t *testing.T, root string, provider *memory.Provider) *AutomationService {
	t.Helper()
	ingestionSvc := NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), provider, workspaceinfra.NewLoader())
	jobSvc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), NewWorkflowEngine())
	automationSvc := NewAutomationService(NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator()), ingestionSvc, nil, jobSvc)
	_ = root
	return automationSvc
}

func newAutomationServiceWithFolderIntakeForTest(root string, intake automationFolderIntake) *AutomationService {
	jobProvider := memory.NewProvider()
	jobSvc := NewJobService(jobProvider.JobRepo(), jobProvider.JobEventRepo(), NewWorkflowEngine())
	return NewAutomationService(NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator()), nil, intake, jobSvc)
}

func newAutomationServiceWithIngestionAndFolderIntakeForTest(root string, provider *memory.Provider, intake automationFolderIntake) *AutomationService {
	jobSvc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), NewWorkflowEngine())
	ingestionSvc := NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), provider, workspaceinfra.NewLoader())
	_ = root
	return NewAutomationService(NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator()), ingestionSvc, intake, jobSvc)
}

type stubAutomationFolderIntake struct {
	runOnceCalls     int
	retryFailedCalls int
	runOnceResult    automationFolderRunSummary
	retryResult      automationFolderRunSummary
	runOnceErr       error
	retryErr         error
}

func (s *stubAutomationFolderIntake) RunOnce(ctx context.Context, root string) (automationFolderRunSummary, error) {
	s.runOnceCalls++
	if s.runOnceErr != nil {
		return automationFolderRunSummary{}, s.runOnceErr
	}
	return s.runOnceResult, nil
}

func (s *stubAutomationFolderIntake) RetryFailed(ctx context.Context, root string) (automationFolderRunSummary, error) {
	s.retryFailedCalls++
	if s.retryErr != nil {
		return automationFolderRunSummary{}, s.retryErr
	}
	return s.retryResult, nil
}
