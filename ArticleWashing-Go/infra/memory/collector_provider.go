package memory

import (
	"content-hub/domain"
	"context"
	"sort"
	"time"
)

type memCollectorSourceRepo struct{ p *Provider }
type memCollectorRunRepo struct{ p *Provider }
type memCollectorEntryRepo struct{ p *Provider }
type memCollectorArticleRepo struct{ p *Provider }
type memCollectorAttemptRepo struct{ p *Provider }
type memCollectorSchedulerRepo struct{ p *Provider }

func (r *memCollectorSourceRepo) Create(_ context.Context, source *domain.CollectorSource) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	copyValue := *source
	r.p.collectorSources[source.ID] = &copyValue
	return nil
}

func (r *memCollectorSourceRepo) GetByID(_ context.Context, id string) (*domain.CollectorSource, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	item, ok := r.p.collectorSources[id]
	if !ok {
		return nil, domain.NewNotFoundErr("collector_source", id)
	}
	copyValue := *item
	return &copyValue, nil
}

func (r *memCollectorSourceRepo) ListAll(_ context.Context) ([]domain.CollectorSource, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	items := make([]domain.CollectorSource, 0, len(r.p.collectorSources))
	for _, item := range r.p.collectorSources {
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (r *memCollectorSourceRepo) ListEnabled(ctx context.Context) ([]domain.CollectorSource, error) {
	items, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]domain.CollectorSource, 0, len(items))
	for _, item := range items {
		if item.Enabled {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (r *memCollectorRunRepo) Create(_ context.Context, run *domain.CollectorRun) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	copyValue := *run
	r.p.collectorRuns[run.ID] = &copyValue
	return nil
}

func (r *memCollectorRunRepo) GetByID(_ context.Context, id string) (*domain.CollectorRun, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	item, ok := r.p.collectorRuns[id]
	if !ok {
		return nil, domain.NewNotFoundErr("collector_run", id)
	}
	copyValue := *item
	return &copyValue, nil
}

func (r *memCollectorRunRepo) Update(_ context.Context, run *domain.CollectorRun) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	copyValue := *run
	copyValue.UpdatedAt = time.Now().UTC()
	r.p.collectorRuns[run.ID] = &copyValue
	return nil
}

func (r *memCollectorRunRepo) ListRecent(_ context.Context, limit int) ([]domain.CollectorRun, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	items := make([]domain.CollectorRun, 0, len(r.p.collectorRuns))
	for _, item := range r.p.collectorRuns {
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *memCollectorRunRepo) CreateSourceRun(_ context.Context, sourceRun *domain.CollectorSourceRun) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	copyValue := *sourceRun
	r.p.collectorSourceRuns[sourceRun.RunID] = append(r.p.collectorSourceRuns[sourceRun.RunID], &copyValue)
	return nil
}

func (r *memCollectorRunRepo) UpdateSourceRun(_ context.Context, sourceRun *domain.CollectorSourceRun) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	items := r.p.collectorSourceRuns[sourceRun.RunID]
	for idx, item := range items {
		if item.ID == sourceRun.ID {
			copyValue := *sourceRun
			copyValue.UpdatedAt = time.Now().UTC()
			items[idx] = &copyValue
			return nil
		}
	}
	return domain.NewNotFoundErr("collector_source_run", sourceRun.ID)
}

func (r *memCollectorRunRepo) ListSourceRuns(_ context.Context, runID string) ([]domain.CollectorSourceRun, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	items := r.p.collectorSourceRuns[runID]
	result := make([]domain.CollectorSourceRun, 0, len(items))
	for _, item := range items {
		result = append(result, *item)
	}
	return result, nil
}

func (r *memCollectorEntryRepo) Create(_ context.Context, entry *domain.CollectorEntry) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	copyValue := *entry
	r.p.collectorEntries[entry.RunID] = append(r.p.collectorEntries[entry.RunID], &copyValue)
	return nil
}

func (r *memCollectorEntryRepo) GetByID(_ context.Context, id string) (*domain.CollectorEntry, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	for _, entries := range r.p.collectorEntries {
		for _, item := range entries {
			if item.ID == id {
				copyValue := *item
				return &copyValue, nil
			}
		}
	}
	return nil, domain.NewNotFoundErr("collector_entry", id)
}

func (r *memCollectorEntryRepo) Update(_ context.Context, entry *domain.CollectorEntry) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	items := r.p.collectorEntries[entry.RunID]
	for idx, item := range items {
		if item.ID == entry.ID {
			copyValue := *entry
			copyValue.UpdatedAt = time.Now().UTC()
			items[idx] = &copyValue
			return nil
		}
	}
	return domain.NewNotFoundErr("collector_entry", entry.ID)
}

func (r *memCollectorEntryRepo) ListByRunID(_ context.Context, runID string) ([]domain.CollectorEntry, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	items := r.p.collectorEntries[runID]
	result := make([]domain.CollectorEntry, 0, len(items))
	for _, item := range items {
		result = append(result, *item)
	}
	return result, nil
}

func (r *memCollectorArticleRepo) Create(_ context.Context, article *domain.CollectorArticle) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	copyValue := *article
	r.p.collectorArticles[article.ID] = &copyValue
	return nil
}

func (r *memCollectorArticleRepo) GetByID(_ context.Context, id string) (*domain.CollectorArticle, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	item, ok := r.p.collectorArticles[id]
	if !ok {
		return nil, domain.NewNotFoundErr("collector_article", id)
	}
	copyValue := *item
	return &copyValue, nil
}

func (r *memCollectorArticleRepo) GetByEntryID(_ context.Context, entryID string) (*domain.CollectorArticle, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	for _, item := range r.p.collectorArticles {
		if item.EntryID == entryID {
			copyValue := *item
			return &copyValue, nil
		}
	}
	return nil, domain.NewNotFoundErr("collector_article", entryID)
}

func (r *memCollectorArticleRepo) Update(_ context.Context, article *domain.CollectorArticle) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	if _, ok := r.p.collectorArticles[article.ID]; !ok {
		return domain.NewNotFoundErr("collector_article", article.ID)
	}
	copyValue := *article
	copyValue.UpdatedAt = time.Now().UTC()
	r.p.collectorArticles[article.ID] = &copyValue
	return nil
}

func (r *memCollectorArticleRepo) Delete(_ context.Context, id string) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	if _, ok := r.p.collectorArticles[id]; !ok {
		return domain.NewNotFoundErr("collector_article", id)
	}
	delete(r.p.collectorArticles, id)
	return nil
}

func (r *memCollectorAttemptRepo) Create(_ context.Context, attempt *domain.CollectorAttempt) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	copyValue := *attempt
	r.p.collectorAttempts[attempt.SourceRunID] = append(r.p.collectorAttempts[attempt.SourceRunID], &copyValue)
	return nil
}

func (r *memCollectorAttemptRepo) ListBySourceRunID(_ context.Context, sourceRunID string) ([]domain.CollectorAttempt, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	items := r.p.collectorAttempts[sourceRunID]
	result := make([]domain.CollectorAttempt, 0, len(items))
	for _, item := range items {
		result = append(result, *item)
	}
	return result, nil
}

func (r *memCollectorSchedulerRepo) Upsert(_ context.Context, state *domain.CollectorSchedulerState) error {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	copyValue := *state
	copyValue.UpdatedAt = time.Now().UTC()
	r.p.collectorSchedulers[state.Name] = &copyValue
	return nil
}

func (r *memCollectorSchedulerRepo) GetByName(_ context.Context, name string) (*domain.CollectorSchedulerState, error) {
	r.p.mu.RLock()
	defer r.p.mu.RUnlock()
	item, ok := r.p.collectorSchedulers[name]
	if !ok {
		return nil, domain.NewNotFoundErr("collector_scheduler_state", name)
	}
	copyValue := *item
	return &copyValue, nil
}
