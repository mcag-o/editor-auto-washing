package service

import (
	"content-hub/collector/plugin"
	"content-hub/collector/plugin/sources"
	"content-hub/infra/config"
)

func NewDefaultRegistry() (*plugin.Registry, error) {
	return NewRegistryFromCollectorConfig(config.DefaultCollectorConfig())
}

func NewRegistryFromCollectorConfig(cfg config.CollectorConfig) (*plugin.Registry, error) {
	registry := plugin.NewRegistry()
	realBuilders := map[string]func() (plugin.SourcePlugin, error){
		"baidu":         sources.NewBaidu,
		"bilibili":      sources.NewBilibili,
		"github":        sources.NewGitHub,
		"stackoverflow": sources.NewStackOverflow,
		"v2ex":          sources.NewV2EX,
		"weibo":         sources.NewWeibo,
	}
	for _, sourceID := range cfg.CanonicalSourceIDs() {
		definition, ok := cfg.SourceOrDefault(sourceID)
		if !ok {
			continue
		}
		builder, hasRealImplementation := realBuilders[sourceID]
		var item plugin.SourcePlugin
		var err error
		if hasRealImplementation {
			item, err = builder()
		} else {
			item = sources.NewPlaceholder(definition, sourceID)
		}
		if err != nil {
			return nil, err
		}
		if err := registry.Register(item); err != nil {
			return nil, err
		}
	}
	return registry, nil
}
