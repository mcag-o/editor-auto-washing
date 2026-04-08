package repo

import (
	"content-hub/domain"
	"context"
)

type ArticleRepo interface {
	Create(ctx context.Context, doc *domain.ContentDocument) error
	GetByID(ctx context.Context, id string) (*domain.ContentDocument, error)
	List(ctx context.Context, q domain.ListQuery) ([]domain.ContentDocument, error)
	Update(ctx context.Context, id string, body string) error
	Delete(ctx context.Context, id string) error
}

type TemplateRepo interface {
	Create(ctx context.Context, t *domain.TemplateAsset) error
	GetByID(ctx context.Context, id string) (*domain.TemplateAsset, error)
	List(ctx context.Context, category string) ([]domain.TemplateAsset, error)
	ListCategories(ctx context.Context) ([]string, error)
	Update(ctx context.Context, id string, content string) error
	Delete(ctx context.Context, id string) error
}

type DraftRepo interface {
	Create(ctx context.Context, d *domain.ArticleDraft) error
	GetByID(ctx context.Context, id string) (*domain.ArticleDraft, error)
	List(ctx context.Context, status *string) ([]domain.ArticleDraft, error)
	Update(ctx context.Context, id string, fn func(*domain.ArticleDraft)) error
}

type AssetRepo interface {
	Create(ctx context.Context, a map[string]any) error
	GetByID(ctx context.Context, id string) (map[string]any, error)
	List(ctx context.Context, articleID, platform string) ([]map[string]any, error)
}

type ReviewRepo interface {
	Create(ctx context.Context, r *domain.ReviewTask) error
	GetByID(ctx context.Context, id string) (*domain.ReviewTask, error)
	ListByArticle(ctx context.Context, articleID string) ([]domain.ReviewTask, error)
	UpdateStatus(ctx context.Context, id string, status, reviewer, notes string) error
}

type PublishRepo interface {
	Record(ctx context.Context, r *domain.PublishRecord) error
	ListByArticle(ctx context.Context, title string) ([]domain.PublishRecord, error)
}

type JobRepo interface {
	Create(ctx context.Context, j *domain.JobRun) error
	GetByID(ctx context.Context, id string) (*domain.JobRun, error)
	List(ctx context.Context, status *string) ([]domain.JobRun, error)
	Update(ctx context.Context, id string, fn func(*domain.JobRun)) error
}

type JobEventRepo interface {
	Add(ctx context.Context, evt *domain.JobEvent) error
	ListByJob(ctx context.Context, jobID string) ([]domain.JobEvent, error)
}

type IngestionRepo interface {
	Record(ctx context.Context, r *domain.IngestionRecord) error
	List(ctx context.Context, t string) ([]domain.IngestionRecord, error)
}

type WorkspaceRepo interface {
	Create(ctx context.Context, w *domain.WorkspaceArticle) error
	GetByID(ctx context.Context, id string) (*domain.WorkspaceArticle, error)
	List(ctx context.Context, status *string) ([]domain.WorkspaceArticle, error)
	TransitionStatus(ctx context.Context, id string, newStatus, notes string) error
}

type LLMProvider interface {
	Generate(ctx context.Context, messages []domain.ChatMessage, opts domain.LLMOptions) (*domain.LLMResponse, error)
	Models(ctx context.Context) ([]string, error)
	Name() string
}

type CollectorProvider interface {
	Collect(ctx context.Context, platforms []string) (*domain.CollectResult, error)
	ListPlatforms(ctx context.Context) ([]domain.PlatformInfo, error)
}

type PublisherProvider interface {
	Publish(ctx context.Context, req domain.PublishRequest) (*domain.PublishResult, error)
	Platforms() []string
}
