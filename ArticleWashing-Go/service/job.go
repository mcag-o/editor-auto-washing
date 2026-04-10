package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type WorkflowExecutor interface {
	Execute(ctx context.Context, wf *domain.WorkflowDefinition, wc *domain.WorkflowContext) error
}

type JobService struct {
	repo      repo.JobRepo
	eventRepo repo.JobEventRepo
	executor  WorkflowExecutor
	queue     chan *domain.JobRun
	closed    chan struct{}
	closeOnce sync.Once
}

func NewJobService(r repo.JobRepo, er repo.JobEventRepo, exec WorkflowExecutor) *JobService {
	return &JobService{
		repo:      r,
		eventRepo: er,
		executor:  exec,
		queue:     make(chan *domain.JobRun, 100),
		closed:    make(chan struct{}),
	}
}

func (s *JobService) Submit(ctx context.Context, topic string) (job *domain.JobRun, err error) {
	job = domain.NewJobRun(topic)
	if err := s.repo.Create(ctx, job); err != nil {
		return nil, err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = s.repo.Delete(ctx, job.ID)
			err = domain.NewExternalErr("job queue closed", errors.New("submit on closed queue"))
			job = nil
		}
	}()
	select {
	case <-s.closed:
		_ = s.repo.Delete(ctx, job.ID)
		return nil, domain.NewExternalErr("job queue closed", nil)
	case s.queue <- job:
		return job, nil
	default:
		_ = s.repo.Delete(ctx, job.ID)
		return nil, domain.NewExternalErr("job queue full", nil)
	}
}

func (s *JobService) GetJob(ctx context.Context, id string) (*domain.JobRun, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *JobService) ListJobs(ctx context.Context, status *string) ([]domain.JobRun, error) {
	return s.repo.List(ctx, status)
}

func (s *JobService) GetEvents(ctx context.Context, jobID string) ([]domain.JobEvent, error) {
	return s.eventRepo.ListByJob(ctx, jobID)
}

func (s *JobService) Cancel(ctx context.Context, id, reason string) (*domain.JobRun, error) {
	if reason == "" {
		reason = "job cancelled"
	}
	if err := s.repo.Update(ctx, id, func(j *domain.JobRun) {
		j.Status = "cancelled"
		message := reason
		j.Result = &message
		completed := time.Now().UTC()
		j.CompletedAt = &completed
	}); err != nil {
		return nil, err
	}
	if err := s.eventRepo.Add(ctx, domain.NewJobEvent(id, "cancelled", reason)); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *JobService) AutomationStatus(ctx context.Context) (*domain.AutomationStatusSnapshot, error) {
	jobs, err := s.repo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	snapshot := &domain.AutomationStatusSnapshot{State: "idle", QueueDepth: len(s.queue), UpdatedAt: time.Now().UTC()}
	if len(jobs) == 0 {
		return snapshot, nil
	}
	latest := jobs[0]
	for _, job := range jobs[1:] {
		if job.UpdatedAt.After(latest.UpdatedAt) {
			latest = job
		}
	}
	snapshot.LastJobID = latest.ID
	snapshot.LastCommand = "run-once"
	snapshot.LastRunSucceeded = latest.Status == "completed"
	snapshot.Summary = map[string]any{"last_job_status": latest.Status, "topic": latest.Topic}
	if latest.Status == "failed" {
		snapshot.State = "failed"
	}
	if latest.Status == "running" {
		snapshot.State = "running"
	}
	return snapshot, nil
}

func (s *JobService) AutomationHealth(ctx context.Context) (*domain.AutomationHealthReport, error) {
	status, err := s.AutomationStatus(ctx)
	if err != nil {
		return nil, err
	}
	report := &domain.AutomationHealthReport{Status: "healthy", Checks: map[string]string{"queue": "running", "state": status.State}, UpdatedAt: time.Now().UTC()}
	if status.State == "failed" {
		report.Status = "degraded"
	}
	return report, nil
}

func (s *JobService) RunWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-s.queue:
			if !ok {
				return
			}
			s.executeJob(ctx, job)
		}
	}
}

func (s *JobService) executeJob(ctx context.Context, job *domain.JobRun) {
	if err := s.repo.Update(ctx, job.ID, func(j *domain.JobRun) {
		j.Status = "running"
	}); err != nil {
		_ = s.eventRepo.Add(ctx, domain.NewJobEvent(job.ID, "failed", "failed to persist running state: "+err.Error()))
		return
	}
	if err := s.eventRepo.Add(ctx, domain.NewJobEvent(job.ID, "started", "job started")); err != nil {
		_ = s.repo.Update(ctx, job.ID, func(j *domain.JobRun) {
			j.Status = "failed"
			result := "failed to persist started event: " + err.Error()
			j.Result = &result
		})
		return
	}

	wf := domain.DefaultWorkflowDefinition()
	wc := &domain.WorkflowContext{Command: job.Topic, Payload: map[string]any{"automation_command": job.Topic}}

	err := s.executor.Execute(ctx, wf, wc)

	if err != nil {
		updateErr := s.repo.Update(ctx, job.ID, func(j *domain.JobRun) {
			j.Status = "failed"
			result := err.Error()
			j.Result = &result
			completed := time.Now().UTC()
			j.CompletedAt = &completed
		})
		if updateErr != nil {
			_ = s.eventRepo.Add(ctx, domain.NewJobEvent(job.ID, "failed", "failed to persist terminal failure state: "+updateErr.Error()))
			return
		}
		if eventErr := s.eventRepo.Add(ctx, domain.NewJobEvent(job.ID, "failed", err.Error())); eventErr != nil {
			return
		}
	} else {
		updateErr := s.repo.Update(ctx, job.ID, func(j *domain.JobRun) {
			j.Status = "completed"
			completed := time.Now().UTC()
			j.CompletedAt = &completed
		})
		if updateErr != nil {
			_ = s.eventRepo.Add(ctx, domain.NewJobEvent(job.ID, "failed", "failed to persist terminal success state: "+updateErr.Error()))
			return
		}
		if eventErr := s.eventRepo.Add(ctx, domain.NewJobEvent(job.ID, "completed", "job completed")); eventErr != nil {
			return
		}
	}
}

func (s *JobService) QueueLength() int {
	return len(s.queue)
}

func (s *JobService) CloseQueue() {
	_ = s.CloseQueueWithContext(context.Background())
}

func (s *JobService) CloseQueueWithContext(ctx context.Context) error {
	var closeErr error
	s.closeOnce.Do(func() {
		for {
			select {
			case job := <-s.queue:
				if job == nil {
					goto closeChannels
				}
				if err := s.repo.Update(ctx, job.ID, func(j *domain.JobRun) {
					j.Status = "failed"
					result := fmt.Sprintf("job queue closed before execution")
					j.Result = &result
					completed := time.Now().UTC()
					j.CompletedAt = &completed
				}); err != nil && closeErr == nil {
					closeErr = err
				}
				if err := s.eventRepo.Add(ctx, domain.NewJobEvent(job.ID, "failed", "job queue closed before execution")); err != nil && closeErr == nil {
					closeErr = err
				}
			default:
				goto closeChannels
			}
		}
	closeChannels:
		close(s.closed)
		close(s.queue)
	})
	return closeErr
}
