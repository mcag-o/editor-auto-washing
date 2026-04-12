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

type MemoryTx struct {
	mu         sync.RWMutex
	articles   map[string]*domain.ContentDocument
	templates  map[string]*domain.TemplateAsset
	drafts     map[string]*domain.ArticleDraft
	assets     map[string]*domain.RenderedAssetRecord
	reviews    map[string]*domain.ReviewTask
	publishes  map[string]*domain.PublishRecord
	jobs       map[string]*domain.JobRun
	jobEvents  map[string][]*domain.JobEvent
	ingestions map[string]*domain.IngestionRecord
	workspaces map[string]*domain.ArticleWorkspaceRecord
	provider   *Provider
	rolled     bool
}

var _ repo.BundleImportTx = (*MemoryTx)(nil)

func (p *Provider) BeginTx() *MemoryTx {
	p.mu.RLock()
	defer p.mu.RUnlock()

	tx := &MemoryTx{
		provider:   p,
		articles:   copyMapPtr(p.articles, cloneContentDocument),
		templates:  copyMapPtr(p.templates, cloneTemplateAsset),
		drafts:     copyMapPtr(p.drafts, cloneArticleDraft),
		assets:     copyMapMap(p.assets),
		reviews:    copyMapPtr(p.reviews, cloneReviewTask),
		publishes:  copyMapPtr(p.publishes, clonePublishRecord),
		jobs:       copyMapPtr(p.jobs, cloneJobRun),
		jobEvents:  copyJobEvents(p.jobEvents),
		ingestions: copyMapPtr(p.ingestions, cloneIngestionRecord),
		workspaces: copyMapPtr(p.workspaces, cloneWorkspaceArticle),
	}
	return tx
}

func (t *MemoryTx) ArticleRepo() repo.ArticleRepo     { return &txArticleRepo{tx: t} }
func (t *MemoryTx) TemplateRepo() repo.TemplateRepo   { return &txTemplateRepo{tx: t} }
func (t *MemoryTx) DraftRepo() repo.DraftRepo         { return &txDraftRepo{tx: t} }
func (t *MemoryTx) AssetRepo() repo.AssetRepo         { return &txAssetRepo{tx: t} }
func (t *MemoryTx) ReviewRepo() repo.ReviewRepo       { return &txReviewRepo{tx: t} }
func (t *MemoryTx) PublishRepo() repo.PublishRepo     { return &txPublishRepo{tx: t} }
func (t *MemoryTx) JobRepo() repo.JobRepo             { return &txJobRepo{tx: t} }
func (t *MemoryTx) JobEventRepo() repo.JobEventRepo   { return &txJobEventRepo{tx: t} }
func (t *MemoryTx) IngestionRepo() repo.IngestionRepo { return &txIngestionRepo{tx: t} }
func (t *MemoryTx) WorkspaceRepo() repo.WorkspaceRepo { return &txWorkspaceRepo{tx: t} }

func (t *MemoryTx) Commit() error {
	if t.rolled {
		return nil
	}
	t.provider.mu.Lock()
	defer t.provider.mu.Unlock()

	t.provider.articles = t.articles
	t.provider.templates = t.templates
	t.provider.drafts = t.drafts
	t.provider.assets = t.assets
	t.provider.reviews = t.reviews
	t.provider.publishes = t.publishes
	t.provider.jobs = t.jobs
	t.provider.jobEvents = t.jobEvents
	t.provider.ingestions = t.ingestions
	t.provider.workspaces = t.workspaces
	return nil
}

func (t *MemoryTx) Rollback() error {
	t.rolled = true
	return nil
}

func (t *MemoryTx) CreateWorkspaceArticle(ctx context.Context, record *domain.ArticleWorkspaceRecord) error {
	return t.WorkspaceRepo().Create(ctx, record)
}

func (t *MemoryTx) RecordIngestion(ctx context.Context, record *domain.IngestionRecord) error {
	return t.IngestionRepo().Record(ctx, record)
}

type txArticleRepo struct{ tx *MemoryTx }

func (r *txArticleRepo) Create(_ context.Context, doc *domain.ContentDocument) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	r.tx.articles[doc.ID] = doc
	return nil
}

func (r *txArticleRepo) GetByID(_ context.Context, id string) (*domain.ContentDocument, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()
	doc, ok := r.tx.articles[id]
	if !ok {
		return nil, domain.NewNotFoundErr("article", id)
	}
	return doc, nil
}

func (r *txArticleRepo) List(_ context.Context, q domain.ListQuery) ([]domain.ContentDocument, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()

	var docs []domain.ContentDocument
	for _, doc := range r.tx.articles {
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

func (r *txArticleRepo) Update(_ context.Context, id string, body string) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	doc, ok := r.tx.articles[id]
	if !ok {
		return domain.NewNotFoundErr("article", id)
	}
	doc.Body = body
	doc.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *txArticleRepo) Delete(_ context.Context, id string) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	if _, ok := r.tx.articles[id]; !ok {
		return domain.NewNotFoundErr("article", id)
	}
	delete(r.tx.articles, id)
	return nil
}

type txTemplateRepo struct{ tx *MemoryTx }

func (r *txTemplateRepo) Create(_ context.Context, t *domain.TemplateAsset) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	r.tx.templates[t.ID] = t
	return nil
}

func (r *txTemplateRepo) GetByID(_ context.Context, id string) (*domain.TemplateAsset, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()
	t, ok := r.tx.templates[id]
	if !ok {
		return nil, domain.NewNotFoundErr("template", id)
	}
	return t, nil
}

func (r *txTemplateRepo) List(_ context.Context, category string) ([]domain.TemplateAsset, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()

	var templates []domain.TemplateAsset
	for _, t := range r.tx.templates {
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

func (r *txTemplateRepo) ListCategories(_ context.Context) ([]string, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()

	seen := make(map[string]bool)
	var categories []string
	for _, t := range r.tx.templates {
		if !seen[t.Category] {
			seen[t.Category] = true
			categories = append(categories, t.Category)
		}
	}
	sort.Strings(categories)
	return categories, nil
}

func (r *txTemplateRepo) Update(_ context.Context, id string, content string) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	t, ok := r.tx.templates[id]
	if !ok {
		return domain.NewNotFoundErr("template", id)
	}
	t.Content = content
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *txTemplateRepo) Delete(_ context.Context, id string) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	if _, ok := r.tx.templates[id]; !ok {
		return domain.NewNotFoundErr("template", id)
	}
	delete(r.tx.templates, id)
	return nil
}

type txDraftRepo struct{ tx *MemoryTx }

func (r *txDraftRepo) Create(_ context.Context, d *domain.ArticleDraft) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	r.tx.drafts[d.ID] = d
	return nil
}

func (r *txDraftRepo) GetByID(_ context.Context, id string) (*domain.ArticleDraft, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()
	d, ok := r.tx.drafts[id]
	if !ok {
		return nil, domain.NewNotFoundErr("draft", id)
	}
	return d, nil
}

func (r *txDraftRepo) List(_ context.Context, status *string) ([]domain.ArticleDraft, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()

	var drafts []domain.ArticleDraft
	for _, d := range r.tx.drafts {
		if status != nil && d.Status != *status {
			continue
		}
		drafts = append(drafts, *d)
	}
	return drafts, nil
}

func (r *txDraftRepo) Update(_ context.Context, id string, fn func(*domain.ArticleDraft)) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	d, ok := r.tx.drafts[id]
	if !ok {
		return domain.NewNotFoundErr("draft", id)
	}
	fn(d)
	d.UpdatedAt = time.Now().UTC()
	return nil
}

type txAssetRepo struct{ tx *MemoryTx }

func (r *txAssetRepo) Create(_ context.Context, a *domain.RenderedAssetRecord) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	if a == nil || a.AssetID == "" {
		return domain.NewValidationErr("asset missing id", nil)
	}
	r.tx.assets[a.AssetID] = cloneRenderedAsset(a)
	return nil
}

func (r *txAssetRepo) GetByID(_ context.Context, id string) (*domain.RenderedAssetRecord, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()
	a, ok := r.tx.assets[id]
	if !ok {
		return nil, domain.NewNotFoundErr("asset", id)
	}
	return cloneRenderedAsset(a), nil
}

func (r *txAssetRepo) List(_ context.Context, articleID, platform string) ([]domain.RenderedAssetRecord, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()

	var assets []domain.RenderedAssetRecord
	for _, a := range r.tx.assets {
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

func (r *txAssetRepo) Delete(_ context.Context, id string) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	if _, ok := r.tx.assets[id]; !ok {
		return domain.NewNotFoundErr("asset", id)
	}
	delete(r.tx.assets, id)
	return nil
}

type txReviewRepo struct{ tx *MemoryTx }

func (r *txReviewRepo) Create(_ context.Context, rev *domain.ReviewTask) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	r.tx.reviews[rev.ID] = rev
	return nil
}

func (r *txReviewRepo) GetByID(_ context.Context, id string) (*domain.ReviewTask, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()
	rev, ok := r.tx.reviews[id]
	if !ok {
		return nil, domain.NewNotFoundErr("review", id)
	}
	return cloneReviewTask(rev), nil
}

func (r *txReviewRepo) ListByArticle(_ context.Context, articleID string) ([]domain.ReviewTask, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()

	var tasks []domain.ReviewTask
	for _, rev := range r.tx.reviews {
		if rev.ArticleID == articleID {
			tasks = append(tasks, *rev)
		}
	}
	return tasks, nil
}

func (r *txReviewRepo) UpdateStatus(_ context.Context, id string, status, reviewer, notes string) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	rev, ok := r.tx.reviews[id]
	if !ok {
		return domain.NewNotFoundErr("review", id)
	}
	rev.Status = status
	rev.Reviewer = reviewer
	rev.Notes = notes
	rev.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *txReviewRepo) Delete(_ context.Context, id string) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	if _, ok := r.tx.reviews[id]; !ok {
		return domain.NewNotFoundErr("review", id)
	}
	delete(r.tx.reviews, id)
	return nil
}

type txPublishRepo struct{ tx *MemoryTx }

func (r *txPublishRepo) Record(_ context.Context, rec *domain.PublishRecord) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	r.tx.publishes[rec.ID] = clonePublishRecord(rec)
	return nil
}

func (r *txPublishRepo) ListByArticle(_ context.Context, articleID string) ([]domain.PublishRecord, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()

	var records []domain.PublishRecord
	for _, rec := range r.tx.publishes {
		if rec.ArticleID == articleID {
			records = append(records, *clonePublishRecord(rec))
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
	return records, nil
}

func (r *txPublishRepo) Delete(_ context.Context, id string) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	if _, ok := r.tx.publishes[id]; !ok {
		return domain.NewNotFoundErr("publish_record", id)
	}
	delete(r.tx.publishes, id)
	return nil
}

type txJobRepo struct{ tx *MemoryTx }

func (r *txJobRepo) Create(_ context.Context, j *domain.JobRun) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	r.tx.jobs[j.ID] = j
	return nil
}

func (r *txJobRepo) GetByID(_ context.Context, id string) (*domain.JobRun, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()
	j, ok := r.tx.jobs[id]
	if !ok {
		return nil, domain.NewNotFoundErr("job", id)
	}
	return j, nil
}

func (r *txJobRepo) List(_ context.Context, status *string) ([]domain.JobRun, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()

	var jobs []domain.JobRun
	for _, j := range r.tx.jobs {
		if status != nil && j.Status != *status {
			continue
		}
		jobs = append(jobs, *j)
	}
	return jobs, nil
}

func (r *txJobRepo) Update(_ context.Context, id string, fn func(*domain.JobRun)) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	j, ok := r.tx.jobs[id]
	if !ok {
		return domain.NewNotFoundErr("job", id)
	}
	fn(j)
	j.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *txJobRepo) Delete(_ context.Context, id string) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	if _, ok := r.tx.jobs[id]; !ok {
		return domain.NewNotFoundErr("job", id)
	}
	delete(r.tx.jobs, id)
	delete(r.tx.jobEvents, id)
	return nil
}

type txJobEventRepo struct{ tx *MemoryTx }

func (r *txJobEventRepo) Add(_ context.Context, evt *domain.JobEvent) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	r.tx.jobEvents[evt.JobID] = append(r.tx.jobEvents[evt.JobID], evt)
	return nil
}

func (r *txJobEventRepo) ListByJob(_ context.Context, jobID string) ([]domain.JobEvent, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()

	events := r.tx.jobEvents[jobID]
	result := make([]domain.JobEvent, len(events))
	for i, e := range events {
		result[i] = *e
	}
	return result, nil
}

type txIngestionRepo struct{ tx *MemoryTx }

func (r *txIngestionRepo) Record(_ context.Context, rec *domain.IngestionRecord) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	r.tx.ingestions[rec.ID] = rec
	return nil
}

func (r *txIngestionRepo) GetByID(_ context.Context, id string) (*domain.IngestionRecord, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()
	rec, ok := r.tx.ingestions[id]
	if !ok {
		return nil, domain.NewNotFoundErr("ingestion", id)
	}
	return rec, nil
}

func (r *txIngestionRepo) List(_ context.Context, status string) ([]domain.IngestionRecord, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()

	var records []domain.IngestionRecord
	for _, rec := range r.tx.ingestions {
		if status != "" && rec.Status != status {
			continue
		}
		records = append(records, *rec)
	}
	return records, nil
}

func (r *txIngestionRepo) Update(_ context.Context, id string, fn func(*domain.IngestionRecord)) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	rec, ok := r.tx.ingestions[id]
	if !ok {
		return domain.NewNotFoundErr("ingestion", id)
	}
	fn(rec)
	rec.UpdatedAt = time.Now().UTC()
	return nil
}

type txWorkspaceRepo struct{ tx *MemoryTx }

func (r *txWorkspaceRepo) Create(_ context.Context, w *domain.ArticleWorkspaceRecord) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	r.tx.workspaces[w.ID] = w
	return nil
}

func (r *txWorkspaceRepo) GetByID(_ context.Context, id string) (*domain.ArticleWorkspaceRecord, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()
	w, ok := r.tx.workspaces[id]
	if !ok {
		return nil, domain.NewNotFoundErr("workspace", id)
	}
	return w, nil
}

func (r *txWorkspaceRepo) List(_ context.Context, status *string) ([]domain.ArticleWorkspaceRecord, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()

	var articles []domain.ArticleWorkspaceRecord
	for _, w := range r.tx.workspaces {
		if status != nil && w.Status != *status {
			continue
		}
		articles = append(articles, *w)
	}
	return articles, nil
}

func (r *txWorkspaceRepo) ListByIngestionID(_ context.Context, ingestionID string) ([]domain.ArticleWorkspaceRecord, error) {
	r.tx.mu.RLock()
	defer r.tx.mu.RUnlock()

	var articles []domain.ArticleWorkspaceRecord
	for _, w := range r.tx.workspaces {
		if w.Source.IngestionID != ingestionID {
			continue
		}
		articles = append(articles, *w)
	}
	return articles, nil
}

func (r *txWorkspaceRepo) TransitionStatus(_ context.Context, id string, newStatus, notes string) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	w, ok := r.tx.workspaces[id]
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

func (r *txWorkspaceRepo) Delete(_ context.Context, id string) error {
	r.tx.mu.Lock()
	defer r.tx.mu.Unlock()
	if _, ok := r.tx.workspaces[id]; !ok {
		return domain.NewNotFoundErr("workspace", id)
	}
	delete(r.tx.workspaces, id)
	return nil
}

func copyMapPtr[K comparable, V any](src map[K]*V, clone func(*V) *V) map[K]*V {
	dst := make(map[K]*V, len(src))
	for k, v := range src {
		dst[k] = clone(v)
	}
	return dst
}

func copyMapMap(src map[string]*domain.RenderedAssetRecord) map[string]*domain.RenderedAssetRecord {
	dst := make(map[string]*domain.RenderedAssetRecord, len(src))
	for k, v := range src {
		dst[k] = cloneRenderedAsset(v)
	}
	return dst
}

func copyJobEvents(src map[string][]*domain.JobEvent) map[string][]*domain.JobEvent {
	dst := make(map[string][]*domain.JobEvent, len(src))
	for k, v := range src {
		cp := make([]*domain.JobEvent, len(v))
		for i, e := range v {
			cp[i] = &domain.JobEvent{
				ID:        e.ID,
				JobID:     e.JobID,
				Status:    e.Status,
				Message:   e.Message,
				Detail:    e.Detail,
				CreatedAt: e.CreatedAt,
			}
		}
		dst[k] = cp
	}
	return dst
}

func cloneContentDocument(v *domain.ContentDocument) *domain.ContentDocument {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Metadata = copyAnyMap(v.Metadata)
	return &cp
}

func cloneTemplateAsset(v *domain.TemplateAsset) *domain.TemplateAsset {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Metadata = copyAnyMap(v.Metadata)
	return &cp
}

func cloneArticleDraft(v *domain.ArticleDraft) *domain.ArticleDraft {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Meta = copyAnyMap(v.Meta)
	cp.Headline = copyAnyMap(v.Headline)
	cp.Sections = append([]any{}, v.Sections...)
	cp.SourceRefs = append([]any{}, v.SourceRefs...)
	cp.TargetPlatforms = append([]string{}, v.TargetPlatforms...)
	return &cp
}

func cloneReviewTask(v *domain.ReviewTask) *domain.ReviewTask {
	if v == nil {
		return nil
	}
	cp := *v
	cp.AssetIDs = append([]string{}, v.AssetIDs...)
	return &cp
}

func clonePublishRecord(v *domain.PublishRecord) *domain.PublishRecord {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Metadata = copyAnyMap(v.Metadata)
	return &cp
}

func cloneRenderedAsset(v *domain.RenderedAssetRecord) *domain.RenderedAssetRecord {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Metadata = copyAnyMap(v.Metadata)
	return &cp
}

func cloneJobRun(v *domain.JobRun) *domain.JobRun {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneIngestionRecord(v *domain.IngestionRecord) *domain.IngestionRecord {
	if v == nil {
		return nil
	}
	cp := *v
	if v.Payload != nil {
		cp.Payload = append([]byte{}, v.Payload...)
	}
	return &cp
}

func cloneWorkspaceArticle(v *domain.ArticleWorkspaceRecord) *domain.ArticleWorkspaceRecord {
	if v == nil {
		return nil
	}
	cp := *v
	cp.StatusHistory = append([]string{}, v.StatusHistory...)
	cp.LifecycleHistory = append([]domain.ArticleWorkspaceLifecycleEntry{}, v.LifecycleHistory...)
	cp.Metadata = copyAnyMap(v.Metadata)
	return &cp
}

func copyAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
