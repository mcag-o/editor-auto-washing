package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"strings"
	"time"
)

type sourceProcessingSchedulerWorker interface {
	Process(ctx context.Context, doc *domain.SourceDocument) error
}

type SourceProcessingScheduler struct {
	repo             repo.SourceDocumentRepo
	worker           sourceProcessingSchedulerWorker
	concurrencyLimit int
	claimedBy        string
	now              func() time.Time
}

func NewSourceProcessingScheduler(repo repo.SourceDocumentRepo, worker sourceProcessingSchedulerWorker, concurrencyLimit int, claimedBy string) *SourceProcessingScheduler {
	return &SourceProcessingScheduler{
		repo:             repo,
		worker:           worker,
		concurrencyLimit: concurrencyLimit,
		claimedBy:        strings.TrimSpace(claimedBy),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *SourceProcessingScheduler) ProcessPending(ctx context.Context) ([]domain.SourceDocument, error) {
	if s.repo == nil || s.worker == nil {
		return nil, domain.NewInternalErr("source processing scheduler is not configured", nil)
	}
	if s.concurrencyLimit <= 0 {
		return nil, domain.NewValidationErr("source processing concurrency limit must be greater than zero", nil)
	}
	if strings.TrimSpace(s.claimedBy) == "" {
		return nil, domain.NewValidationErr("source processing claimed by is required", nil)
	}
	nowFn := s.now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}

	claimed, err := s.repo.ClaimPending(ctx, s.concurrencyLimit, s.claimedBy, nowFn())
	if err != nil {
		return nil, fmt.Errorf("claim pending source documents: %w", err)
	}

	processed := make([]domain.SourceDocument, 0, len(claimed))
	var firstErr error
	for i := range claimed {
		doc := claimed[i]
		processed = append(processed, doc)
		if err := s.worker.Process(ctx, &doc); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("process source document %s: %w", strings.TrimSpace(doc.ID), err)
			}
		}
	}
	if firstErr != nil {
		return processed, firstErr
	}
	return processed, nil
}
