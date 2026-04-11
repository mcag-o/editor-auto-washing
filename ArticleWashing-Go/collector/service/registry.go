package service

import (
	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
	"content-hub/collector/plugin/sources"
	collectorruntime "content-hub/collector/runtime"
	"content-hub/domain"
	"content-hub/infra/config"
)

func NewDefaultRegistry() (*plugin.Registry, error) {
	return NewRegistryFromCollectorConfig(config.DefaultCollectorConfig())
}

func NewRegistryFromCollectorConfig(cfg config.CollectorConfig) (*plugin.Registry, error) {
	registry := plugin.NewRegistry()
	resolver := collectorruntime.NewPolicyResolver()
	runtimeCfg := config.DefaultConfig()
	runtimeCfg.Collector = cfg
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
			if err != nil {
				return nil, err
			}
		} else {
			item = sources.NewPlaceholder(definition, sourceID)
		}
		item, err = withResolvedSourceDescriptor(runtimeCfg, resolver, definition, item)
		if err != nil {
			return nil, err
		}
		if err := registry.Register(item); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func withResolvedSourceDescriptor(cfg config.Config, resolver *collectorruntime.PolicyResolver, definition config.CollectorSourceDef, item plugin.SourcePlugin) (plugin.SourcePlugin, error) {
	describer, ok := item.(plugin.SourceDescriptor)
	if !ok {
		return item, nil
	}
	resolved, err := resolver.ResolveSource(cfg, item.SourceID())
	if err != nil {
		return nil, err
	}
	return resolvedDescriptorPlugin{SourcePlugin: item, descriptor: mergeResolvedDescriptor(describer.Descriptor(), definition, resolved)}, nil
}

type resolvedDescriptorPlugin struct {
	plugin.SourcePlugin
	descriptor plugin.SourceDefinition
}

func (p resolvedDescriptorPlugin) Descriptor() plugin.SourceDefinition {
	return p.descriptor
}

func (p resolvedDescriptorPlugin) WithSourceConfig(source domain.CollectorSource) plugin.SourcePlugin {
	configurable, ok := p.SourcePlugin.(plugin.SourceConfigurablePlugin)
	if !ok {
		return p
	}
	return resolvedDescriptorPlugin{SourcePlugin: configurable.WithSourceConfig(source), descriptor: p.descriptor}
}

func (p resolvedDescriptorPlugin) WithHTTPClient(client *httpclient.Client) plugin.SourcePlugin {
	configurable, ok := p.SourcePlugin.(plugin.SourceHTTPClientConfigurable)
	if !ok {
		return p
	}
	return resolvedDescriptorPlugin{SourcePlugin: configurable.WithHTTPClient(client), descriptor: p.descriptor}
}

func mergeResolvedDescriptor(base plugin.SourceDefinition, definition config.CollectorSourceDef, resolved collectorruntime.ResolvedSourceRuntimeConfig) plugin.SourceDefinition {
	base.Enabled = definition.Enabled
	base.ScheduleEnabled = definition.ScheduleEnabled
	base.IntervalMinutes = definition.IntervalMinutes
	base.TimeoutMS = int(resolved.Timeout / 1e6)
	base.HotlistLimit = resolved.HotlistLimit
	base.DetailFetchEnabled = definition.DetailFetchEnabled
	base.Concurrency = definition.Concurrency
	base.AuthMode = definition.AuthMode
	base.CookieSecretRef = definition.CookieSecretRef
	base.HeaderSecretRef = definition.HeaderSecretRef
	base.Headers = cloneResolvedHeaders(resolved.Headers)
	base.RetryPolicy = map[string]any{
		"max_attempts": resolved.RetryPolicy.MaxAttempts,
		"wait":         resolved.RetryPolicy.Wait.String(),
		"max_wait":     resolved.RetryPolicy.MaxWait.String(),
	}
	base.Options = mergeResolvedOptions(base.Options, resolved.Options)
	return base
}

func cloneResolvedHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

func mergeResolvedOptions(base map[string]any, resolved map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(resolved))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range resolved {
		merged[key] = value
	}
	return merged
}
