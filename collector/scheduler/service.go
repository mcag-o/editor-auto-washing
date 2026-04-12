package scheduler

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"sync"
	"time"
)

type runService interface {
	RunHotlist(ctx context.Context, trigger string) (*domain.CollectorRunSummary, error)
}

type Service struct {
	repo     repo.CollectorSchedulerStateRepo
	runs     runService
	interval time.Duration

	mu         sync.Mutex
	running    bool
	stopping   bool
	stopCh     chan struct{}
	stopAckCh  chan struct{}
	loopCtx    context.Context
	loopCancel context.CancelFunc
}

func NewService(repo repo.CollectorSchedulerStateRepo, runs runService, interval time.Duration) *Service {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	return &Service{repo: repo, runs: runs, interval: interval}
}

func (s *Service) RunOnce(ctx context.Context) (*domain.CollectorRunSummary, error) {
	now := time.Now().UTC()
	state := domain.NewCollectorSchedulerState(domain.DefaultCollectorSchedulerName)
	state.Status = domain.CollectorSchedulerRunning
	state.LastHeartbeat = &now
	if err := s.repo.Upsert(ctx, state); err != nil {
		return nil, err
	}

	result, err := s.runs.RunHotlist(ctx, "scheduler_run_once")
	completed := time.Now().UTC()
	state.LastHeartbeat = &completed
	state.LastRunAt = &completed
	state.NextRunAt = timePtr(completed.Add(s.interval))
	if result != nil {
		state.LastRunID = result.RunID
	}
	if err != nil {
		state.Status = domain.CollectorSchedulerFailed
		state.ErrorMessage = err.Error()
		_ = s.repo.Upsert(ctx, state)
		return nil, err
	}
	state.Status = domain.CollectorSchedulerIdle
	state.ErrorMessage = ""
	if err := s.repo.Upsert(ctx, state); err != nil {
		return nil, err
	}
	return result, nil
}

// StartDaemon persists the initial scheduler state using the caller context,
// then detaches the long-lived loop onto an internal background context.
// Cancelling the request context after StartDaemon returns does not stop the
// daemon; callers must use Stop to shut it down explicitly.
func (s *Service) StartDaemon(ctx context.Context) (*domain.CollectorSchedulerControlResult, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil, domain.NewConflictErr("collector scheduler already running")
	}
	s.running = true
	s.stopping = false
	s.stopCh = make(chan struct{})
	s.stopAckCh = make(chan struct{})
	loopCtx, loopCancel := context.WithCancel(context.Background())
	s.loopCtx = loopCtx
	s.loopCancel = loopCancel
	s.mu.Unlock()

	now := time.Now().UTC()
	state := domain.NewCollectorSchedulerState(domain.DefaultCollectorSchedulerName)
	state.Status = domain.CollectorSchedulerRunning
	state.LastHeartbeat = &now
	state.NextRunAt = timePtr(now.Add(s.interval))
	if err := s.repo.Upsert(ctx, state); err != nil {
		s.finish()
		return nil, err
	}

	_ = ctx
	go s.loop(loopCtx)
	return &domain.CollectorSchedulerControlResult{Started: true, State: domain.CollectorSchedulerRunning, UpdatedAt: now}, nil
}

func (s *Service) Stop(ctx context.Context) (*domain.CollectorSchedulerControlResult, error) {
	s.mu.Lock()
	if !s.running || s.stopCh == nil {
		s.mu.Unlock()
		return nil, domain.NewConflictErr("collector scheduler is not running")
	}
	if s.stopping {
		ackCh := s.stopAckCh
		s.mu.Unlock()
		select {
		case <-ackCh:
			now := time.Now().UTC()
			return &domain.CollectorSchedulerControlResult{Stopped: true, State: domain.CollectorSchedulerStopped, Reason: "operator request", UpdatedAt: now}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	s.stopping = true
	stopCh := s.stopCh
	ackCh := s.stopAckCh
	loopCancel := s.loopCancel
	s.mu.Unlock()

	select {
	case <-stopCh:
	default:
		close(stopCh)
	}
	if loopCancel != nil {
		loopCancel()
	}
	select {
	case <-ackCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	s.markStopped(ctx)
	now := time.Now().UTC()
	return &domain.CollectorSchedulerControlResult{Stopped: true, State: domain.CollectorSchedulerStopped, Reason: "operator request", UpdatedAt: now}, nil
}

func (s *Service) Status(ctx context.Context) (*domain.CollectorSchedulerStatus, error) {
	state, err := s.getState(ctx)
	if err != nil {
		return nil, err
	}
	return &domain.CollectorSchedulerStatus{
		Name:          state.Name,
		State:         state.Status,
		Running:       s.isRunning(),
		LastRunID:     state.LastRunID,
		LastRunAt:     state.LastRunAt,
		LastHeartbeat: state.LastHeartbeat,
		NextRunAt:     state.NextRunAt,
		UpdatedAt:     state.UpdatedAt,
	}, nil
}

func (s *Service) Health(ctx context.Context) (*domain.CollectorSchedulerHealthReport, error) {
	status, err := s.Status(ctx)
	if err != nil {
		return nil, err
	}
	report := &domain.CollectorSchedulerHealthReport{
		Status:    "healthy",
		Checks:    map[string]string{"state": status.State, "loop": "stopped"},
		UpdatedAt: time.Now().UTC(),
	}
	if status.Running {
		report.Checks["loop"] = "running"
	}
	if status.State == domain.CollectorSchedulerFailed {
		report.Status = "degraded"
	}
	return report, nil
}

func (s *Service) loop(ctx context.Context) {
	defer s.finish()
	for {
		if _, err := s.RunOnce(ctx); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			s.markStopped(context.Background())
			return
		case <-s.stopCh:
			s.markStopped(context.Background())
			return
		case <-time.After(s.interval):
		}
	}
}

func (s *Service) markStopped(ctx context.Context) {
	state, err := s.getState(ctx)
	if err != nil {
		state = domain.NewCollectorSchedulerState(domain.DefaultCollectorSchedulerName)
	}
	now := time.Now().UTC()
	state.Status = domain.CollectorSchedulerStopped
	state.LastHeartbeat = &now
	state.NextRunAt = nil
	_ = s.repo.Upsert(ctx, state)
}

func (s *Service) getState(ctx context.Context) (*domain.CollectorSchedulerState, error) {
	state, err := s.repo.GetByName(ctx, domain.DefaultCollectorSchedulerName)
	if err == nil {
		return state, nil
	}
	if !isNotFound(err) {
		return nil, err
	}
	state = domain.NewCollectorSchedulerState(domain.DefaultCollectorSchedulerName)
	if upsertErr := s.repo.Upsert(ctx, state); upsertErr != nil {
		return nil, upsertErr
	}
	return state, nil
}

func (s *Service) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loopCancel != nil {
		s.loopCancel()
	}
	if s.stopAckCh != nil {
		close(s.stopAckCh)
	}
	s.running = false
	s.stopping = false
	s.stopCh = nil
	s.stopAckCh = nil
	s.loopCtx = nil
	s.loopCancel = nil
}

func (s *Service) isRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func isNotFound(err error) bool {
	appErr, ok := err.(*domain.AppError)
	return ok && appErr.Code == domain.ErrNotFound
}

func timePtr(value time.Time) *time.Time {
	return &value
}
