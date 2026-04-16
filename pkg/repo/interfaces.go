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
	Create(ctx context.Context, a *domain.RenderedAssetRecord) error
	GetByID(ctx context.Context, id string) (*domain.RenderedAssetRecord, error)
	List(ctx context.Context, articleID, platform string) ([]domain.RenderedAssetRecord, error)
	Delete(ctx context.Context, id string) error
}

type ReviewRepo interface {
	Create(ctx context.Context, r *domain.ReviewTask) error
	GetByID(ctx context.Context, id string) (*domain.ReviewTask, error)
	ListByArticle(ctx context.Context, articleID string) ([]domain.ReviewTask, error)
	UpdateStatus(ctx context.Context, id string, status, reviewer, notes string) error
	Delete(ctx context.Context, id string) error
}

type PublishRepo interface {
	Record(ctx context.Context, r *domain.PublishRecord) error
	ListByArticle(ctx context.Context, articleID string) ([]domain.PublishRecord, error)
	Delete(ctx context.Context, id string) error
}

type JobRepo interface {
	Create(ctx context.Context, j *domain.JobRun) error
	GetByID(ctx context.Context, id string) (*domain.JobRun, error)
	List(ctx context.Context, status *string) ([]domain.JobRun, error)
	Update(ctx context.Context, id string, fn func(*domain.JobRun)) error
	Delete(ctx context.Context, id string) error
}

type JobEventRepo interface {
	Add(ctx context.Context, evt *domain.JobEvent) error
	// ListByJob returns events in durable append order.
	ListByJob(ctx context.Context, jobID string) ([]domain.JobEvent, error)
}

type IngestionRepo interface {
	Record(ctx context.Context, r *domain.IngestionRecord) error
	GetByID(ctx context.Context, id string) (*domain.IngestionRecord, error)
	List(ctx context.Context, status string) ([]domain.IngestionRecord, error)
	Update(ctx context.Context, id string, fn func(*domain.IngestionRecord)) error
}

type WorkspaceRepo interface {
	Create(ctx context.Context, w *domain.ArticleWorkspaceRecord) error
	GetByID(ctx context.Context, id string) (*domain.ArticleWorkspaceRecord, error)
	List(ctx context.Context, status *string) ([]domain.ArticleWorkspaceRecord, error)
	ListByIngestionID(ctx context.Context, ingestionID string) ([]domain.ArticleWorkspaceRecord, error)
	TransitionStatus(ctx context.Context, id string, newStatus, notes string) error
	Delete(ctx context.Context, id string) error
}

type RewritePipelineProfileRepo interface {
	Upsert(ctx context.Context, profile *domain.RewritePipelineProfile) error
	Get(ctx context.Context, targetType, sourceProfile, version string) (*domain.RewritePipelineProfile, error)
	List(ctx context.Context) ([]domain.RewritePipelineProfile, error)
}

type RewritePipelineRunRepo interface {
	Create(ctx context.Context, run *domain.RewritePipelineRun) error
	Update(ctx context.Context, run *domain.RewritePipelineRun) error
	GetByID(ctx context.Context, id string) (*domain.RewritePipelineRun, error)
	List(ctx context.Context, limit int) ([]domain.RewritePipelineRun, error)
}

type RewriteStageRunRepo interface {
	Create(ctx context.Context, run *domain.RewriteStageRun) error
	ListByPipelineRunID(ctx context.Context, pipelineRunID string) ([]domain.RewriteStageRun, error)
}

type RSSSubscriptionRepo interface {
	Create(ctx context.Context, subscription *domain.RSSSubscription) error
	Update(ctx context.Context, subscription *domain.RSSSubscription) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*domain.RSSSubscription, error)
	List(ctx context.Context) ([]domain.RSSSubscription, error)
}

type RSSPullRunRepo interface {
	Create(ctx context.Context, run *domain.RSSPullRun) error
	Update(ctx context.Context, run *domain.RSSPullRun) error
	GetByID(ctx context.Context, id string) (*domain.RSSPullRun, error)
	List(ctx context.Context, limit int) ([]domain.RSSPullRun, error)
}

type RSSItemRepo interface {
	Create(ctx context.Context, item *domain.RSSItemRecord) error
	Update(ctx context.Context, item *domain.RSSItemRecord) error
	FindDuplicate(ctx context.Context, key domain.RSSDuplicateKey) (*domain.RSSItemRecord, error)
	GetByID(ctx context.Context, id string) (*domain.RSSItemRecord, error)
	List(ctx context.Context, limit int) ([]domain.RSSItemRecord, error)
}

type PromptTemplateRepo interface {
	Upsert(ctx context.Context, prompt *domain.PromptTemplate) error
	Get(ctx context.Context, key, version string) (*domain.PromptTemplate, error)
	List(ctx context.Context) ([]domain.PromptTemplate, error)
}

type LLMProfileRepo interface {
	Upsert(ctx context.Context, profile *domain.LLMProfile) error
	GetByName(ctx context.Context, name string) (*domain.LLMProfile, error)
	List(ctx context.Context) ([]domain.LLMProfile, error)
}

type BundleImportTx interface {
	CreateWorkspaceArticle(ctx context.Context, record *domain.ArticleWorkspaceRecord) error
	RecordIngestion(ctx context.Context, record *domain.IngestionRecord) error
	Commit() error
	Rollback() error
}

type BundleImportTxStarter interface {
	BeginBundleImport(ctx context.Context) (BundleImportTx, error)
}

type LLMProvider interface {
	Generate(ctx context.Context, req domain.LLMGenerateRequest) (*domain.LLMGenerateResponse, error)
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
