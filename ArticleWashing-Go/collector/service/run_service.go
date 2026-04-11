package service

import (
	"content-hub/collector/plugin"
	collectorruntime "content-hub/collector/runtime"
	"content-hub/domain"
	"content-hub/infra/config"
	"content-hub/pkg/repo"
	"context"
	"encoding/json"
	"sort"
	"time"
)

type RunService struct {
	sources repo.CollectorSourceRepo
	runs    repo.CollectorRunRepo
	entries repo.CollectorEntryRepo
	plugins *plugin.Registry
	runtime *runtimePluginAdapter
}

func NewRunService(sources repo.CollectorSourceRepo, runs repo.CollectorRunRepo, entries repo.CollectorEntryRepo, plugins *plugin.Registry) *RunService {
	return &RunService{sources: sources, runs: runs, entries: entries, plugins: plugins}
}

func NewRunServiceWithRuntime(sources repo.CollectorSourceRepo, runs repo.CollectorRunRepo, entries repo.CollectorEntryRepo, plugins *plugin.Registry, cfg config.Config, secrets collectorruntime.SecretResolver) *RunService {
	return &RunService{sources: sources, runs: runs, entries: entries, plugins: plugins, runtime: newRuntimePluginAdapter(cfg, secrets)}
}

func (s *RunService) RunHotlist(ctx context.Context, trigger string) (*domain.CollectorRunSummary, error) {
	run := domain.NewCollectorRun(trigger)
	now := time.Now().UTC()
	run.Status = domain.CollectorRunRunning
	run.StartedAt = &now
	if err := s.runs.Create(ctx, run); err != nil {
		return nil, err
	}

	sources, err := s.sources.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].ID < sources[j].ID })

	summary := &domain.CollectorRunSummary{RunID: run.ID, Trigger: trigger, Status: domain.CollectorRunSucceeded, SourceCount: len(sources), StartedAt: now}
	for _, source := range sources {
		sourceRun := domain.NewCollectorSourceRun(run.ID, source.ID, domain.CollectorStageHotlist)
		started := time.Now().UTC()
		sourceRun.Status = domain.CollectorSourceRunRunning
		sourceRun.StartedAt = &started
		if err := s.runs.CreateSourceRun(ctx, sourceRun); err != nil {
			return nil, err
		}

		count, runErr := s.runSource(ctx, run.ID, sourceRun, source)
		completed := time.Now().UTC()
		sourceRun.CompletedAt = &completed
		if runErr != nil {
			sourceRun.Status = domain.CollectorSourceRunFailed
			sourceRun.ErrorMessage = runErr.Error()
			summary.FailedSources++
			summary.Status = domain.CollectorRunFailed
		} else {
			sourceRun.Status = domain.CollectorSourceRunSucceeded
			summary.SuccessfulSources++
			summary.EntryCount += count
		}
		if err := s.runs.UpdateSourceRun(ctx, sourceRun); err != nil {
			return nil, err
		}
	}
	completed := time.Now().UTC()
	run.CompletedAt = &completed
	run.Status = summary.Status
	if summary.Status == domain.CollectorRunFailed {
		run.ErrorMessage = "one or more collector sources failed"
	}
	if err := s.runs.Update(ctx, run); err != nil {
		return nil, err
	}
	summary.CompletedAt = completed
	return summary, nil
}

func (s *RunService) runSource(ctx context.Context, runID string, sourceRun *domain.CollectorSourceRun, source domain.CollectorSource) (int, error) {
	p, err := s.plugins.Get(source.ID)
	if err != nil {
		return 0, err
	}
	p, err = s.configurePlugin(p, source)
	if err != nil {
		return 0, err
	}
	entries, err := p.FetchHotlist(ctx, plugin.FetchHotlistRequest{Limit: source.HotlistLimit})
	if err != nil {
		return 0, err
	}
	stored := 0
	for _, item := range entries {
		entry := domain.NewCollectorEntry(runID, source.ID, item.ExternalID, item.Title, item.CanonicalURL)
		entry.Summary = item.Summary
		entry.Author = item.Author
		entry.PublishedAt = item.PublishedAt
		entry.Rank = item.Rank
		entry.RawJSON = cloneBytes(item.RawJSON)
		normalized, marshalErr := json.Marshal(item)
		if marshalErr == nil {
			entry.NormalizedJSON = normalized
		}
		if err := s.entries.Create(ctx, entry); err != nil {
			return stored, err
		}
		stored++
	}
	sourceRun.DiscoveredCount = len(entries)
	sourceRun.StoredCount = stored
	return stored, nil
}

func (s *RunService) configurePlugin(p plugin.SourcePlugin, source domain.CollectorSource) (plugin.SourcePlugin, error) {
	if s.runtime == nil {
		return pluginForSourceConfig(p, source), nil
	}
	return s.runtime.configure(p, source)
}

func (s *RunService) ListRuns(ctx context.Context, limit int) ([]domain.CollectorRun, error) {
	return s.runs.ListRecent(ctx, limit)
}

func (s *RunService) GetRun(ctx context.Context, runID string) (*domain.CollectorRunDetail, error) {
	run, err := s.runs.GetByID(ctx, runID)
	if err != nil {
		return nil, err
	}
	sourceRuns, err := s.runs.ListSourceRuns(ctx, runID)
	if err != nil {
		return nil, err
	}
	return &domain.CollectorRunDetail{Run: *run, SourceRuns: sourceRuns}, nil
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	dup := make([]byte, len(value))
	copy(dup, value)
	return dup
}
