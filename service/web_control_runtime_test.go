package service

import (
	"content-hub/domain"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildWebControlRuntimeReturnsReadyServices(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	runtime, err := BuildWebControlRuntime(repos)

	require.NoError(t, err)
	require.NotNil(t, runtime)
	require.NotNil(t, runtime.Config)
	require.NotNil(t, runtime.Control)
	require.NotNil(t, runtime.Audit)
	require.NotNil(t, runtime.Intake)
	require.NotNil(t, runtime.Articles)
}

func TestBuildWebControlRuntimeRequiresRepos(t *testing.T) {
	runtime, err := BuildWebControlRuntime(nil)

	require.Nil(t, runtime)
	require.Error(t, err)
	require.ErrorContains(t, err, "web control runtime repos are required")
}

type stubWebControlProcessingCycleRunner struct {
	called           bool
	concurrencyLimit int
	err              error
}

func (r *stubWebControlProcessingCycleRunner) ProcessPending(ctx context.Context, concurrencyLimit int) error {
	r.called = true
	r.concurrencyLimit = concurrencyLimit
	_ = ctx
	return r.err
}

func TestWebControlPlaneServiceStartPassesConfiguredConcurrencyToProcessingCycle(t *testing.T) {
	controlRepo := &stubSystemControlStateRepo{}
	auditRepo := &stubAuditLogRepo{}
	runner := &stubWebControlProcessingCycleRunner{}
	svc := NewWebControlPlaneService(
		NewControlStateService(controlRepo),
		NewAuditLogService(auditRepo),
		runner,
	)

	state, err := svc.Start(t.Context(), "local-admin", 3)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.True(t, runner.called)
	require.Equal(t, 3, runner.concurrencyLimit)
	require.Len(t, auditRepo.logs, 1)
	require.Equal(t, "control_plane.started", auditRepo.logs[0].Action)
	require.Equal(t, "success", auditRepo.logs[0].Result)
	require.Equal(t, 3, auditRepo.logs[0].Metadata["concurrency_limit"])
}

func TestWebControlPlaneServiceStartReturnsFailureSemanticsWhenProcessingCycleFails(t *testing.T) {
	controlRepo := &stubSystemControlStateRepo{}
	auditRepo := &stubAuditLogRepo{}
	runner := &stubWebControlProcessingCycleRunner{err: errors.New("cycle failed")}
	svc := NewWebControlPlaneService(
		NewControlStateService(controlRepo),
		NewAuditLogService(auditRepo),
		runner,
	)

	state, err := svc.Start(t.Context(), "local-admin", 2)

	require.Nil(t, state)
	require.Error(t, err)
	require.ErrorContains(t, err, "process pending source documents")
	require.True(t, runner.called)
	require.Len(t, auditRepo.logs, 1)
	require.Equal(t, "control_plane.started", auditRepo.logs[0].Action)
	require.Equal(t, "failure", auditRepo.logs[0].Result)
	require.Contains(t, auditRepo.logs[0].Message, "cycle failed")
	require.Equal(t, 2, auditRepo.logs[0].Metadata["concurrency_limit"])
	storedState, getErr := controlRepo.Get(t.Context())
	require.NoError(t, getErr)
	require.Equal(t, domain.SystemStateRunning, storedState.State)
}
