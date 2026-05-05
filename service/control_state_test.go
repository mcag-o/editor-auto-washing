package service

import (
	"content-hub/domain"
	"context"
	"testing"

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
	require.Equal(t, domain.SystemStateStopped, paused.State)
	require.Equal(t, "paused", paused.Reason)

	resumed, err := svc.Resume(t.Context(), "local-admin")
	require.NoError(t, err)
	require.Equal(t, domain.SystemStateRunning, resumed.State)
	require.Equal(t, "resumed", resumed.Reason)
	require.Equal(t, 3, resumed.Metadata["concurrency_limit"])
}
