package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"time"
)

type ControlStateService struct {
	repo repo.SystemControlStateRepo
}

func NewControlStateService(r repo.SystemControlStateRepo) *ControlStateService {
	return &ControlStateService{repo: r}
}

func (s *ControlStateService) Get(ctx context.Context) (*domain.SystemControlState, error) {
	return s.repo.Get(ctx)
}

func (s *ControlStateService) Start(ctx context.Context, updatedBy string, concurrencyLimit int) (*domain.SystemControlState, error) {
	if concurrencyLimit <= 0 {
		return nil, domain.NewValidationErr("concurrency limit must be greater than zero", nil)
	}
	state, err := s.loadOrCreate(ctx, updatedBy)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	state.State = domain.SystemStateRunning
	state.Reason = "started"
	state.UpdatedBy = updatedBy
	state.RequestedAt = &now
	state.UpdatedAt = now
	if state.Metadata == nil {
		state.Metadata = map[string]any{}
	}
	state.Metadata["concurrency_limit"] = concurrencyLimit
	if err := s.repo.Upsert(ctx, state); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx)
}

func (s *ControlStateService) Pause(ctx context.Context, updatedBy string) (*domain.SystemControlState, error) {
	state, err := s.loadOrCreate(ctx, updatedBy)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	state.State = domain.SystemStatePaused
	state.Reason = "paused"
	state.UpdatedBy = updatedBy
	state.RequestedAt = &now
	state.UpdatedAt = now
	if err := s.repo.Upsert(ctx, state); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx)
}

func (s *ControlStateService) Resume(ctx context.Context, updatedBy string) (*domain.SystemControlState, error) {
	state, err := s.loadOrCreate(ctx, updatedBy)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	state.State = domain.SystemStateRunning
	state.Reason = "resumed"
	state.UpdatedBy = updatedBy
	state.RequestedAt = &now
	state.UpdatedAt = now
	if err := s.repo.Upsert(ctx, state); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx)
}

func (s *ControlStateService) loadOrCreate(ctx context.Context, updatedBy string) (*domain.SystemControlState, error) {
	state, err := s.repo.Get(ctx)
	if err == nil {
		return state, nil
	}
	if appErr, ok := err.(*domain.AppError); ok && appErr.Code == domain.ErrNotFound {
		return domain.NewSystemControlState(updatedBy), nil
	}
	return nil, err
}
