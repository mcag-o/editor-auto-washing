package repo

import (
	"content-hub/domain"
	"context"
)

type CollectorSourceRepo interface {
	Create(ctx context.Context, source *domain.CollectorSource) error
	GetByID(ctx context.Context, id string) (*domain.CollectorSource, error)
	Update(ctx context.Context, source *domain.CollectorSource) error
	ListAll(ctx context.Context) ([]domain.CollectorSource, error)
	ListEnabled(ctx context.Context) ([]domain.CollectorSource, error)
}

type CollectorRunRepo interface {
	Create(ctx context.Context, run *domain.CollectorRun) error
	GetByID(ctx context.Context, id string) (*domain.CollectorRun, error)
	Update(ctx context.Context, run *domain.CollectorRun) error
	ListRecent(ctx context.Context, limit int) ([]domain.CollectorRun, error)
	CreateSourceRun(ctx context.Context, sourceRun *domain.CollectorSourceRun) error
	UpdateSourceRun(ctx context.Context, sourceRun *domain.CollectorSourceRun) error
	ListSourceRuns(ctx context.Context, runID string) ([]domain.CollectorSourceRun, error)
}

type CollectorEntryRepo interface {
	Create(ctx context.Context, entry *domain.CollectorEntry) error
	GetByID(ctx context.Context, id string) (*domain.CollectorEntry, error)
	Update(ctx context.Context, entry *domain.CollectorEntry) error
	ListByRunID(ctx context.Context, runID string) ([]domain.CollectorEntry, error)
}

type CollectorArticleRepo interface {
	Create(ctx context.Context, article *domain.CollectorArticle) error
	GetByID(ctx context.Context, id string) (*domain.CollectorArticle, error)
	GetByEntryID(ctx context.Context, entryID string) (*domain.CollectorArticle, error)
	Update(ctx context.Context, article *domain.CollectorArticle) error
	Delete(ctx context.Context, id string) error
}

type CollectorSourceRunReader interface {
	ListSourceRuns(ctx context.Context, runID string) ([]domain.CollectorSourceRun, error)
}

type CollectorAttemptRepo interface {
	Create(ctx context.Context, attempt *domain.CollectorAttempt) error
	ListBySourceRunID(ctx context.Context, sourceRunID string) ([]domain.CollectorAttempt, error)
}

type CollectorSchedulerStateRepo interface {
	Upsert(ctx context.Context, state *domain.CollectorSchedulerState) error
	GetByName(ctx context.Context, name string) (*domain.CollectorSchedulerState, error)
}
