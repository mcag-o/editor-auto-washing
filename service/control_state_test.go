package service

import (
	"content-hub/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubSystemControlStateRepo struct {
	state *domain.SystemControlState
}

func (r *stubSystemControlStateRepo) Get(context.Context) (*domain.SystemControlState, error) {
	if r.state == nil {
		return nil, domain.NewNotFoundErr("system_control_state", "singleton")
	}
	copyState := *r.state
	return &copyState, nil
}

func (r *stubSystemControlStateRepo) Upsert(_ context.Context, state *domain.SystemControlState) error {
	if err := state.Validate(); err != nil {
		return err
	}
	copyState := *state
	r.state = &copyState
	return nil
}

func TestControlStateServiceStartPauseResume(t *testing.T) {
	repo := &stubSystemControlStateRepo{}
	svc := NewControlStateService(repo)

	started, err := svc.Start(t.Context(), "local-admin", 3)
	require.NoError(t, err)
	require.Equal(t, domain.SystemStateRunning, started.State)
	require.Equal(t, "local-admin", started.UpdatedBy)
	require.Equal(t, 3, started.Metadata["concurrency_limit"])

	paused, err := svc.Pause(t.Context(), "local-admin")
	require.NoError(t, err)
	require.Equal(t, domain.SystemStatePaused, paused.State)
	require.NotEqual(t, domain.SystemStateStopped, paused.State)
	require.Equal(t, "paused", paused.Reason)
	require.Equal(t, domain.SystemStatePaused, repo.state.State)

	resumed, err := svc.Resume(t.Context(), "local-admin")
	require.NoError(t, err)
	require.Equal(t, domain.SystemStateRunning, resumed.State)
	require.Equal(t, "resumed", resumed.Reason)
	require.Equal(t, 3, resumed.Metadata["concurrency_limit"])
}

func TestControlStateServiceGetReturnsDefaultStoppedWhenRepoEmpty(t *testing.T) {
	repo := &stubSystemControlStateRepo{}
	svc := NewControlStateService(repo)

	state, err := svc.Get(t.Context())

	require.NoError(t, err)
	require.Equal(t, domain.SystemStateStopped, state.State)
	require.Equal(t, map[string]any{}, state.Metadata)
	require.Equal(t, "", state.Reason)
}

func TestControlStateServiceStartRejectsInvalidConcurrency(t *testing.T) {
	repo := &stubSystemControlStateRepo{}
	svc := NewControlStateService(repo)

	state, err := svc.Start(t.Context(), "local-admin", 0)

	require.Error(t, err)
	assert.Nil(t, state)
	require.ErrorAs(t, err, new(*domain.AppError))
	assert.Nil(t, repo.state)
}

func TestControlStateServicePauseBeforeStartFails(t *testing.T) {
	repo := &stubSystemControlStateRepo{}
	svc := NewControlStateService(repo)

	state, err := svc.Pause(t.Context(), "local-admin")

	require.Error(t, err)
	assert.Nil(t, state)
	assert.Nil(t, repo.state)
}

func TestControlStateServiceResumeBeforeStartFails(t *testing.T) {
	repo := &stubSystemControlStateRepo{}
	svc := NewControlStateService(repo)

	state, err := svc.Resume(t.Context(), "local-admin")

	require.Error(t, err)
	assert.Nil(t, state)
	assert.Nil(t, repo.state)
}

func TestControlStateServiceResumeFromStoppedFails(t *testing.T) {
	repo := &stubSystemControlStateRepo{state: domain.NewSystemControlState("seed")}
	svc := NewControlStateService(repo)

	state, err := svc.Resume(t.Context(), "local-admin")

	require.Error(t, err)
	assert.Nil(t, state)
	require.NotNil(t, repo.state)
	assert.Equal(t, domain.SystemStateStopped, repo.state.State)
}
