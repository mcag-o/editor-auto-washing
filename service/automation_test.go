package service

import (
	"content-hub/domain"
	"content-hub/infra/memory"
	workspaceinfra "content-hub/infra/workspace"
	"context"
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

func TestRunWorkerExecutesRegisteredAutomationNodes(t *testing.T) {
	root := newAutomationWorkspace(t)
	provider := memory.NewProvider()
	ingestionSvc := NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), provider, workspaceinfra.NewLoader())
	automationSvc := NewAutomationService(NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator()), ingestionSvc, nil)
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
	automationSvc := NewAutomationService(NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator()), ingestionSvc, nil)
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
	automationSvc := NewAutomationService(NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator()), ingestionSvc, jobSvc)
	_ = root
	return automationSvc
}
