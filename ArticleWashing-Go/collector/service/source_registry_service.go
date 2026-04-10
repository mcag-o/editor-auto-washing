package service

import (
	"content-hub/collector/plugin"
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"errors"
	"sort"
	"strings"
)

type SourceRegistryService struct {
	sources  repo.CollectorSourceRepo
	registry *plugin.Registry
}

func NewSourceRegistryService(sources repo.CollectorSourceRepo, registry *plugin.Registry) *SourceRegistryService {
	return &SourceRegistryService{sources: sources, registry: registry}
}

func (s *SourceRegistryService) Sync(ctx context.Context) error {
	for _, item := range s.registry.List() {
		if _, err := s.sources.GetByID(ctx, item.SourceID()); err == nil {
			continue
		} else if !isNotFound(err) {
			return err
		}

		source := domain.NewCollectorSource(item.SourceID(), item.DisplayName())
		if err := s.sources.Create(ctx, source); err != nil && !isAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (s *SourceRegistryService) ListSources(ctx context.Context) ([]domain.CollectorSource, error) {
	items, err := s.sources.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (s *SourceRegistryService) Health(ctx context.Context) ([]domain.CollectorSourceHealthStatus, error) {
	items, err := s.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	statuses := make([]domain.CollectorSourceHealthStatus, 0, len(items))
	for _, item := range items {
		status := domain.CollectorSourceHealthStatus{
			SourceID:     item.ID,
			DisplayName:  item.DisplayName,
			Enabled:      item.Enabled,
			Capabilities: domain.CollectorSourceCapabilities{},
		}
		p, pluginErr := s.registry.Get(item.ID)
		if pluginErr != nil {
			status.Health = domain.CollectorHealthInfo{OK: false, Code: plugin.HealthCodeUnavailable, Message: pluginErr.Error()}
			statuses = append(statuses, status)
			continue
		}
		p = pluginForSourceConfig(p, item)
		caps := p.Capabilities()
		status.Capabilities = domain.CollectorSourceCapabilities{
			SupportsHotlist: caps.SupportsHotlist,
			SupportsArticle: caps.SupportsArticle,
			AuthModes:       append([]string(nil), caps.AuthModes...),
		}
		health, healthErr := p.HealthCheck(ctx)
		status.Health = domain.CollectorHealthInfo{OK: health.OK, Code: health.Code, Message: health.Message, CheckedAt: health.CheckedAt}
		if healthErr != nil && status.Health.Message == "" {
			status.Health.Message = healthErr.Error()
		}
		if status.Health.Code == "" {
			status.Health.Code = plugin.HealthCodeUnavailable
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func isNotFound(err error) bool {
	var appErr *domain.AppError
	return errors.As(err, &appErr) && appErr.Code == domain.ErrNotFound
}

func isAlreadyExists(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "already exists"))
}

func pluginForSourceConfig(p plugin.SourcePlugin, source domain.CollectorSource) plugin.SourcePlugin {
	configurable, ok := p.(plugin.SourceConfigurablePlugin)
	if !ok {
		return p
	}
	return configurable.WithSourceConfig(source)
}
