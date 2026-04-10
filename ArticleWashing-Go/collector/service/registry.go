package service

import (
	"content-hub/collector/plugin"
	"content-hub/collector/plugin/sources"
)

func NewDefaultRegistry() (*plugin.Registry, error) {
	registry := plugin.NewRegistry()
	builders := []func() (plugin.SourcePlugin, error){
		sources.NewBaidu,
		sources.NewBilibili,
		sources.NewGitHub,
		sources.NewStackOverflow,
		sources.NewV2EX,
		sources.NewWeibo,
	}
	for _, build := range builders {
		item, err := build()
		if err != nil {
			return nil, err
		}
		if err := registry.Register(item); err != nil {
			return nil, err
		}
	}
	return registry, nil
}
