package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

type WorkflowExecutor interface {
	Execute(ctx context.Context, wf *domain.WorkflowDefinition, wc *domain.WorkflowContext) error
}

type JobService struct {
	repo      repo.JobRepo
	eventRepo repo.JobEventRepo
	executor  WorkflowExecutor
	queue     chan *domain.JobRun
}

func NewJobService(r repo.JobRepo, er repo.JobEventRepo, exec WorkflowExecutor) *JobService {
	return &JobService{
		repo:      r,
		eventRepo: er,
		executor:  exec,
		queue:     make(chan *domain.JobRun, 100),
	}
}

func (s *JobService) Submit(ctx context.Context, topic string) (*domain.JobRun, error) {
	job := domain.NewJobRun(topic)
	if err := s.repo.Create(ctx, job); err != nil {
		return nil, err
	}
	select {
	case s.queue <- job:
		return job, nil
	default:
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

func (s *JobService) RunWorker(ctx context.Context) {
	for job := range s.queue {
		s.executeJob(ctx, job)
	}
}

func (s *JobService) executeJob(ctx context.Context, job *domain.JobRun) {
	s.repo.Update(ctx, job.ID, func(j *domain.JobRun) {
		j.Status = "running"
	})
	s.eventRepo.Add(ctx, domain.NewJobEvent(job.ID, "started", "job started"))

	wf := domain.DefaultWorkflowDefinition()
	wc := &domain.WorkflowContext{Payload: map[string]any{"topic": job.Topic}}

	err := s.executor.Execute(ctx, wf, wc)

	if err != nil {
		s.repo.Update(ctx, job.ID, func(j *domain.JobRun) {
			j.Status = "failed"
			result := err.Error()
			j.Result = &result
		})
		s.eventRepo.Add(ctx, domain.NewJobEvent(job.ID, "failed", err.Error()))
	} else {
		s.repo.Update(ctx, job.ID, func(j *domain.JobRun) {
			j.Status = "completed"
		})
		s.eventRepo.Add(ctx, domain.NewJobEvent(job.ID, "completed", "job completed"))
	}
}

func (s *JobService) QueueLength() int {
	return len(s.queue)
}

func (s *JobService) CloseQueue() {
	close(s.queue)
}
