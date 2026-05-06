package memory

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	_ repo.ArticleRepo                 = (*memArticleRepo)(nil)
	_ repo.TemplateRepo                = (*memTemplateRepo)(nil)
	_ repo.DraftRepo                   = (*memDraftRepo)(nil)
	_ repo.AssetRepo                   = (*memAssetRepo)(nil)
	_ repo.ReviewRepo                  = (*memReviewRepo)(nil)
	_ repo.PublishRepo                 = (*memPublishRepo)(nil)
	_ repo.JobRepo                     = (*memJobRepo)(nil)
	_ repo.JobEventRepo                = (*memJobEventRepo)(nil)
	_ repo.IngestionRepo               = (*memIngestionRepo)(nil)
	_ repo.WorkspaceRepo               = (*memWorkspaceRepo)(nil)
	_ repo.RewritePipelineRunRepo      = (*memRewritePipelineRunRepo)(nil)
	_ repo.RewriteStageRunRepo         = (*memRewriteStageRunRepo)(nil)
	_ repo.CollectorSourceRepo         = (*memCollectorSourceRepo)(nil)
	_ repo.CollectorRunRepo            = (*memCollectorRunRepo)(nil)
	_ repo.CollectorEntryRepo          = (*memCollectorEntryRepo)(nil)
	_ repo.CollectorArticleRepo        = (*memCollectorArticleRepo)(nil)
	_ repo.CollectorAttemptRepo        = (*memCollectorAttemptRepo)(nil)
	_ repo.CollectorSchedulerStateRepo = (*memCollectorSchedulerRepo)(nil)
	_ repo.BundleImportTxStarter       = (*Provider)(nil)
)

type Provider struct {
	mu                  sync.RWMutex
	articles            map[string]*domain.ContentDocument
	templates           map[string]*domain.TemplateAsset
	drafts              map[string]*domain.ArticleDraft
	assets              map[string]*domain.RenderedAssetRecord
	reviews             map[string]*domain.ReviewTask
	publishes           map[string]*domain.PublishRecord
	jobs                map[string]*domain.JobRun
	jobEvents           map[string][]*domain.JobEvent
	ingestions          map[string]*domain.IngestionRecord
	workspaces          map[string]*domain.ArticleWorkspaceRecord
	rewritePipelineRuns map[string]*domain.RewritePipelineRun
	rewriteStageRuns    map[string][]*domain.RewriteStageRun
	collectorSources    map[string]*domain.CollectorSource
	collectorRuns       map[string]*domain.CollectorRun
	collectorSourceRuns map[string][]*domain.CollectorSourceRun
	collectorEntries    map[string][]*domain.CollectorEntry
	collectorArticles   map[string]*domain.CollectorArticle
	collectorAttempts   map[string][]*domain.CollectorAttempt
	collectorSchedulers map[string]*domain.CollectorSchedulerState
}

func NewProvider() *Provider {
	return &Provider{
		articles:            make(map[string]*domain.ContentDocument),
		templates:           make(map[string]*domain.TemplateAsset),
		drafts:              make(map[string]*domain.ArticleDraft),
		assets:              make(map[string]*domain.RenderedAssetRecord),
		reviews:             make(map[string]*domain.ReviewTask),
		publishes:           make(map[string]*domain.PublishRecord),
		jobs:                make(map[string]*domain.JobRun),
		jobEvents:           make(map[string][]*domain.JobEvent),
		ingestions:          make(map[string]*domain.IngestionRecord),
		workspaces:          make(map[string]*domain.ArticleWorkspaceRecord),
		rewritePipelineRuns: make(map[string]*domain.RewritePipelineRun),
		rewriteStageRuns:    make(map[string][]*domain.RewriteStageRun),
		collectorSources:    make(map[string]*domain.CollectorSource),
		collectorRuns:       make(map[string]*domain.CollectorRun),
		collectorSourceRuns: make(map[string][]*domain.CollectorSourceRun),
		collectorEntries:    make(map[string][]*domain.CollectorEntry),
		collectorArticles:   make(map[string]*domain.CollectorArticle),
		collectorAttempts:   make(map[string][]*domain.CollectorAttempt),
		collectorSchedulers: make(map[string]*domain.CollectorSchedulerState),
	}
}

func (p *Provider) ArticleRepo() repo.ArticleRepo     { return &memArticleRepo{p: p} }
func (p *Provider) TemplateRepo() repo.TemplateRepo   { return &memTemplateRepo{p: p} }
func (p *Provider) DraftRepo() repo.DraftRepo         { return &memDraftRepo{p: p} }
func (p *Provider) AssetRepo() repo.AssetRepo         { return &memAssetRepo{p: p} }
func (p *Provider) ReviewRepo() repo.ReviewRepo       { return &memReviewRepo{p: p} }
func (p *Provider) PublishRepo() repo.PublishRepo     { return &memPublishRepo{p: p} }
func (p *Provider) JobRepo() repo.JobRepo             { return &memJobRepo{p: p} }
func (p *Provider) JobEventRepo() repo.JobEventRepo   { return &memJobEventRepo{p: p} }
func (p *Provider) IngestionRepo() repo.IngestionRepo { return &memIngestionRepo{p: p} }
func (p *Provider) WorkspaceRepo() repo.WorkspaceRepo { return &memWorkspaceRepo{p: p} }
func (p *Provider) RewritePipelineRunRepo() repo.RewritePipelineRunRepo {
	return &memRewritePipelineRunRepo{p: p}
}
func (p *Provider) RewriteStageRunRepo() repo.RewriteStageRunRepo {
	return &memRewriteStageRunRepo{p: p}
}
func (p *Provider) CollectorSourceRepo() repo.CollectorSourceRepo {
	return &memCollectorSourceRepo{p: p}
}
func (p *Provider) CollectorRunRepo() repo.CollectorRunRepo     { return &memCollectorRunRepo{p: p} }
func (p *Provider) CollectorEntryRepo() repo.CollectorEntryRepo { return &memCollectorEntryRepo{p: p} }
func (p *Provider) CollectorArticleRepo() repo.CollectorArticleRepo {
	return &memCollectorArticleRepo{p: p}
}
func (p *Provider) CollectorAttemptRepo() repo.CollectorAttemptRepo {
	return &memCollectorAttemptRepo{p: p}
}
func (p *Provider) CollectorSchedulerRepo() repo.CollectorSchedulerStateRepo {
	return &memCollectorSchedulerRepo{p: p}
}

func (p *Provider) BeginBundleImport(_ context.Context) (repo.BundleImportTx, error) {
	return p.BeginTx(), nil
}

type memArticleRepo struct{ p *Provider }

func (r *memArticleRepo) Create(_ context.Context, doc *domain.ContentDocument) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	r.p.articles[doc.ID] = doc
	return nil
}

func (r *memArticleRepo) GetByID(_ context.Context, id string) (*domain.ContentDocument, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	doc, ok := r.p.articles[id]
	if !ok {
		return nil, domain.NewNotFoundErr("article", id)
	}
	return doc, nil
}

func (r *memArticleRepo) List(_ context.Context, q domain.ListQuery) ([]domain.ContentDocument, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	var docs []domain.ContentDocument
	for _, doc := range r.p.articles {
		if q.TitleQuery != "" && !strings.Contains(strings.ToLower(doc.Title), strings.ToLower(q.TitleQuery)) {
			continue
		}
		docs = append(docs, *doc)
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].CreatedAt.After(docs[j].CreatedAt)
	})

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := q.Offset
	if offset >= len(docs) {
		return []domain.ContentDocument{}, nil
	}
	end := offset + limit
	if end > len(docs) {
		end = len(docs)
	}
	return docs[offset:end], nil
}

func (r *memArticleRepo) Update(_ context.Context, id string, body string) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	doc, ok := r.p.articles[id]
	if !ok {
		return domain.NewNotFoundErr("article", id)
	}
	doc.Body = body
	doc.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *memArticleRepo) Delete(_ context.Context, id string) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	if _, ok := r.p.articles[id]; !ok {
		return domain.NewNotFoundErr("article", id)
	}
	delete(r.p.articles, id)
	return nil
}

type memTemplateRepo struct{ p *Provider }

func (r *memTemplateRepo) Create(_ context.Context, t *domain.TemplateAsset) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	r.p.templates[t.ID] = t
	return nil
}

func (r *memTemplateRepo) GetByID(_ context.Context, id string) (*domain.TemplateAsset, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	t, ok := r.p.templates[id]
	if !ok {
		return nil, domain.NewNotFoundErr("template", id)
	}
	return t, nil
}

func (r *memTemplateRepo) List(_ context.Context, category string) ([]domain.TemplateAsset, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	var templates []domain.TemplateAsset
	for _, t := range r.p.templates {
		if category != "" && t.Category != category {
			continue
		}
		templates = append(templates, *t)
	}

	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Category == templates[j].Category {
			return templates[i].Name < templates[j].Name
		}
		return templates[i].Category < templates[j].Category
	})

	return templates, nil
}

func (r *memTemplateRepo) ListCategories(_ context.Context) ([]string, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	seen := make(map[string]bool)
	var categories []string
	for _, t := range r.p.templates {
		if !seen[t.Category] {
			seen[t.Category] = true
			categories = append(categories, t.Category)
		}
	}
	sort.Strings(categories)
	return categories, nil
}

func (r *memTemplateRepo) Update(_ context.Context, id string, content string) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	t, ok := r.p.templates[id]
	if !ok {
		return domain.NewNotFoundErr("template", id)
	}
	t.Content = content
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *memTemplateRepo) Delete(_ context.Context, id string) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	if _, ok := r.p.templates[id]; !ok {
		return domain.NewNotFoundErr("template", id)
	}
	delete(r.p.templates, id)
	return nil
}

type memDraftRepo struct{ p *Provider }

func (r *memDraftRepo) Create(_ context.Context, d *domain.ArticleDraft) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	r.p.drafts[d.ID] = d
	return nil
}

func (r *memDraftRepo) GetByID(_ context.Context, id string) (*domain.ArticleDraft, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	d, ok := r.p.drafts[id]
	if !ok {
		return nil, domain.NewNotFoundErr("draft", id)
	}
	return d, nil
}

func (r *memDraftRepo) List(_ context.Context, status *string) ([]domain.ArticleDraft, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	var drafts []domain.ArticleDraft
	for _, d := range r.p.drafts {
		if status != nil && d.Status != *status {
			continue
		}
		drafts = append(drafts, *d)
	}
	return drafts, nil
}

func (r *memDraftRepo) Update(_ context.Context, id string, fn func(*domain.ArticleDraft)) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	d, ok := r.p.drafts[id]
	if !ok {
		return domain.NewNotFoundErr("draft", id)
	}
	fn(d)
	d.UpdatedAt = time.Now().UTC()
	return nil
}

type memAssetRepo struct{ p *Provider }

func (r *memAssetRepo) Create(_ context.Context, a *domain.RenderedAssetRecord) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	if a == nil || a.AssetID == "" {
		return domain.NewValidationErr("asset missing id", nil)
	}
	r.p.assets[a.AssetID] = cloneRenderedAsset(a)
	return nil
}

func (r *memAssetRepo) GetByID(_ context.Context, id string) (*domain.RenderedAssetRecord, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	a, ok := r.p.assets[id]
	if !ok {
		return nil, domain.NewNotFoundErr("asset", id)
	}
	return cloneRenderedAsset(a), nil
}

func (r *memAssetRepo) List(_ context.Context, articleID, platform string) ([]domain.RenderedAssetRecord, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	var assets []domain.RenderedAssetRecord
	for _, a := range r.p.assets {
		if articleID != "" && a.ArticleID != articleID {
			continue
		}
		if platform != "" && a.Platform != platform {
			continue
		}
		assets = append(assets, *cloneRenderedAsset(a))
	}
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].CreatedAt.After(assets[j].CreatedAt)
	})
	return assets, nil
}

func (r *memAssetRepo) Delete(_ context.Context, id string) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	if _, ok := r.p.assets[id]; !ok {
		return domain.NewNotFoundErr("asset", id)
	}
	delete(r.p.assets, id)
	return nil
}

type memReviewRepo struct{ p *Provider }

func (r *memReviewRepo) Create(_ context.Context, rev *domain.ReviewTask) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	r.p.reviews[rev.ID] = rev
	return nil
}

func (r *memReviewRepo) GetByID(_ context.Context, id string) (*domain.ReviewTask, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	rev, ok := r.p.reviews[id]
	if !ok {
		return nil, domain.NewNotFoundErr("review", id)
	}
	return cloneReviewTask(rev), nil
}

func (r *memReviewRepo) ListByArticle(_ context.Context, articleID string) ([]domain.ReviewTask, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	var tasks []domain.ReviewTask
	for _, rev := range r.p.reviews {
		if rev.ArticleID == articleID {
			tasks = append(tasks, *rev)
		}
	}
	return tasks, nil
}

func (r *memReviewRepo) UpdateStatus(_ context.Context, id string, status, reviewer, notes string) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	rev, ok := r.p.reviews[id]
	if !ok {
		return domain.NewNotFoundErr("review", id)
	}
	rev.Status = status
	rev.Reviewer = reviewer
	rev.Notes = notes
	rev.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *memReviewRepo) Delete(_ context.Context, id string) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	if _, ok := r.p.reviews[id]; !ok {
		return domain.NewNotFoundErr("review", id)
	}
	delete(r.p.reviews, id)
	return nil
}

type memPublishRepo struct{ p *Provider }

func (r *memPublishRepo) Record(_ context.Context, rec *domain.PublishRecord) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	r.p.publishes[rec.ID] = clonePublishRecord(rec)
	return nil
}

func (r *memPublishRepo) ListByArticle(_ context.Context, articleID string) ([]domain.PublishRecord, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	var records []domain.PublishRecord
	for _, rec := range r.p.publishes {
		if rec.ArticleID == articleID {
			records = append(records, *clonePublishRecord(rec))
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
	return records, nil
}

func (r *memPublishRepo) Delete(_ context.Context, id string) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	if _, ok := r.p.publishes[id]; !ok {
		return domain.NewNotFoundErr("publish_record", id)
	}
	delete(r.p.publishes, id)
	return nil
}

type memJobRepo struct{ p *Provider }

func (r *memJobRepo) Create(_ context.Context, j *domain.JobRun) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	r.p.jobs[j.ID] = j
	return nil
}

func (r *memJobRepo) GetByID(_ context.Context, id string) (*domain.JobRun, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	j, ok := r.p.jobs[id]
	if !ok {
		return nil, domain.NewNotFoundErr("job", id)
	}
	return j, nil
}

func (r *memJobRepo) List(_ context.Context, status *string) ([]domain.JobRun, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	var jobs []domain.JobRun
	for _, j := range r.p.jobs {
		if status != nil && j.Status != *status {
			continue
		}
		jobs = append(jobs, *j)
	}
	return jobs, nil
}

func (r *memJobRepo) Update(_ context.Context, id string, fn func(*domain.JobRun)) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	j, ok := r.p.jobs[id]
	if !ok {
		return domain.NewNotFoundErr("job", id)
	}
	fn(j)
	j.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *memJobRepo) Delete(_ context.Context, id string) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	if _, ok := r.p.jobs[id]; !ok {
		return domain.NewNotFoundErr("job", id)
	}
	delete(r.p.jobs, id)
	delete(r.p.jobEvents, id)
	return nil
}

type memJobEventRepo struct{ p *Provider }

func (r *memJobEventRepo) Add(_ context.Context, evt *domain.JobEvent) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	r.p.jobEvents[evt.JobID] = append(r.p.jobEvents[evt.JobID], evt)
	return nil
}

func (r *memJobEventRepo) ListByJob(_ context.Context, jobID string) ([]domain.JobEvent, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	events := r.p.jobEvents[jobID]
	result := make([]domain.JobEvent, len(events))
	for i, e := range events {
		result[i] = *e
	}
	return result, nil
}

type memIngestionRepo struct{ p *Provider }

func (r *memIngestionRepo) Record(_ context.Context, rec *domain.IngestionRecord) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	r.p.ingestions[rec.ID] = rec
	return nil
}

func (r *memIngestionRepo) GetByID(_ context.Context, id string) (*domain.IngestionRecord, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	rec, ok := r.p.ingestions[id]
	if !ok {
		return nil, domain.NewNotFoundErr("ingestion", id)
	}
	return rec, nil
}

func (r *memIngestionRepo) List(_ context.Context, status string) ([]domain.IngestionRecord, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	var records []domain.IngestionRecord
	for _, rec := range r.p.ingestions {
		if status != "" && rec.Status != status {
			continue
		}
		records = append(records, *rec)
	}
	return records, nil
}

func (r *memIngestionRepo) Update(_ context.Context, id string, fn func(*domain.IngestionRecord)) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	rec, ok := r.p.ingestions[id]
	if !ok {
		return domain.NewNotFoundErr("ingestion", id)
	}
	fn(rec)
	rec.UpdatedAt = time.Now().UTC()
	return nil
}

type memWorkspaceRepo struct{ p *Provider }

func (r *memWorkspaceRepo) Create(_ context.Context, w *domain.ArticleWorkspaceRecord) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	r.p.workspaces[w.ID] = w
	return nil
}

func (r *memWorkspaceRepo) GetByID(_ context.Context, id string) (*domain.ArticleWorkspaceRecord, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	w, ok := r.p.workspaces[id]
	if !ok {
		return nil, domain.NewNotFoundErr("workspace", id)
	}
	return w, nil
}

func (r *memWorkspaceRepo) List(_ context.Context, status *string) ([]domain.ArticleWorkspaceRecord, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	var articles []domain.ArticleWorkspaceRecord
	for _, w := range r.p.workspaces {
		if status != nil && w.Status != *status {
			continue
		}
		articles = append(articles, *w)
	}
	return articles, nil
}

func (r *memWorkspaceRepo) ListByIngestionID(_ context.Context, ingestionID string) ([]domain.ArticleWorkspaceRecord, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	var articles []domain.ArticleWorkspaceRecord
	for _, w := range r.p.workspaces {
		if w.Source.IngestionID != ingestionID {
			continue
		}
		articles = append(articles, *w)
	}
	return articles, nil
}

func (r *memWorkspaceRepo) TransitionStatus(_ context.Context, id string, newStatus, notes string) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	w, ok := r.p.workspaces[id]
	if !ok {
		return domain.NewNotFoundErr("workspace", id)
	}
	if !w.CanTransitionTo(newStatus) {
		return domain.NewConflictErr(fmt.Sprintf("cannot transition from %s to %s", w.Status, newStatus))
	}
	w.Status = newStatus
	w.StatusHistory = append(w.StatusHistory, newStatus)
	w.LifecycleHistory = append(w.LifecycleHistory, domain.ArticleWorkspaceLifecycleEntry{Status: newStatus, Notes: notes, CreatedAt: time.Now().UTC()})
	w.Notes = notes
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *memWorkspaceRepo) Delete(_ context.Context, id string) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	if _, ok := r.p.workspaces[id]; !ok {
		return domain.NewNotFoundErr("workspace", id)
	}
	delete(r.p.workspaces, id)
	return nil
}

type memRewritePipelineRunRepo struct{ p *Provider }

func (r *memRewritePipelineRunRepo) Create(_ context.Context, run *domain.RewritePipelineRun) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	r.p.rewritePipelineRuns[run.ID] = cloneRewritePipelineRun(run)
	return nil
}

func (r *memRewritePipelineRunRepo) Update(_ context.Context, run *domain.RewritePipelineRun) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	if _, ok := r.p.rewritePipelineRuns[run.ID]; !ok {
		return domain.NewNotFoundErr("rewrite_pipeline_run", run.ID)
	}
	r.p.rewritePipelineRuns[run.ID] = cloneRewritePipelineRun(run)
	return nil
}

func (r *memRewritePipelineRunRepo) GetByID(_ context.Context, id string) (*domain.RewritePipelineRun, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	run, ok := r.p.rewritePipelineRuns[id]
	if !ok {
		return nil, domain.NewNotFoundErr("rewrite_pipeline_run", id)
	}
	return cloneRewritePipelineRun(run), nil
}

func (r *memRewritePipelineRunRepo) List(_ context.Context, limit int) ([]domain.RewritePipelineRun, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	runs := make([]domain.RewritePipelineRun, 0, len(r.p.rewritePipelineRuns))
	for _, run := range r.p.rewritePipelineRuns {
		runs = append(runs, *cloneRewritePipelineRun(run))
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	if limit <= 0 || limit >= len(runs) {
		return runs, nil
	}
	return runs[:limit], nil
}

type memRewriteStageRunRepo struct{ p *Provider }

func (r *memRewriteStageRunRepo) Create(_ context.Context, run *domain.RewriteStageRun) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	r.p.rewriteStageRuns[run.PipelineRunID] = append(r.p.rewriteStageRuns[run.PipelineRunID], cloneRewriteStageRun(run))
	return nil
}

func (r *memRewriteStageRunRepo) Update(_ context.Context, run *domain.RewriteStageRun) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	for pipelineRunID, runs := range r.p.rewriteStageRuns {
		for i := range runs {
			if runs[i].ID == run.ID {
				r.p.rewriteStageRuns[pipelineRunID][i] = cloneRewriteStageRun(run)
				return nil
			}
		}
	}
	return domain.NewNotFoundErr("rewrite_stage_run", run.ID)
}

func (r *memRewriteStageRunRepo) ListByPipelineRunID(_ context.Context, pipelineRunID string) ([]domain.RewriteStageRun, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()

	runs := r.p.rewriteStageRuns[pipelineRunID]
	result := make([]domain.RewriteStageRun, len(runs))
	for i, run := range runs {
		result[i] = *cloneRewriteStageRun(run)
	}
	return result, nil
}

func cloneRewritePipelineRun(run *domain.RewritePipelineRun) *domain.RewritePipelineRun {
	if run == nil {
		return nil
	}
	clone := *run
	clone.Metadata = cloneMap(run.Metadata)
	if run.CompletedAt != nil {
		completedAt := *run.CompletedAt
		clone.CompletedAt = &completedAt
	}
	return &clone
}

func cloneRewriteStageRun(run *domain.RewriteStageRun) *domain.RewriteStageRun {
	if run == nil {
		return nil
	}
	clone := *run
	clone.Metadata = cloneMap(run.Metadata)
	if run.CompletedAt != nil {
		completedAt := *run.CompletedAt
		clone.CompletedAt = &completedAt
	}
	return &clone
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return map[string]any{}
	}
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
