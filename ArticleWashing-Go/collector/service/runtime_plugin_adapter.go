package service

import (
	"encoding/json"
	"fmt"
	"net/http"

	"content-hub/collector/plugin"
	collectorruntime "content-hub/collector/runtime"
	"content-hub/domain"
	"content-hub/infra/config"
)

type runtimePluginAdapter struct {
	cfg      config.Config
	policies *collectorruntime.PolicyResolver
	auth     *collectorruntime.AuthFactory
	http     *collectorruntime.HTTPFactory
	enabled  bool
}

func newRuntimePluginAdapter(cfg config.Config, secrets collectorruntime.SecretResolver) *runtimePluginAdapter {
	if len(cfg.Collector.Sources) == 0 || secrets == nil {
		return &runtimePluginAdapter{}
	}
	return &runtimePluginAdapter{
		cfg:      cfg,
		policies: collectorruntime.NewPolicyResolver(),
		auth:     collectorruntime.NewAuthFactory(secrets),
		http:     collectorruntime.NewHTTPFactory(),
		enabled:  true,
	}
}

func (a *runtimePluginAdapter) configure(p plugin.SourcePlugin, source domain.CollectorSource) (plugin.SourcePlugin, error) {
	p = pluginForSourceConfig(p, source)
	if a == nil || !a.enabled {
		return p, nil
	}
	if _, ok := a.cfg.Collector.SourceOrDefault(source.ID); !ok {
		return p, nil
	}
	clientConfigurable, ok := p.(plugin.SourceHTTPClientConfigurable)
	if !ok {
		return nil, fmt.Errorf("plugin %s does not support runtime http client injection", source.ID)
	}
	resolved, err := a.resolveSource(source)
	if err != nil {
		return nil, err
	}
	var injector func(req *http.Request) error
	if a.auth != nil {
		built, buildErr := a.auth.Build(resolved.Auth)
		if buildErr != nil {
			return nil, buildErr
		}
		injector = built
	}
	client, err := a.http.Build(resolved, injector)
	if err != nil {
		return nil, err
	}
	return clientConfigurable.WithHTTPClient(client), nil
}

func (a *runtimePluginAdapter) resolveSource(source domain.CollectorSource) (collectorruntime.ResolvedSourceRuntimeConfig, error) {
	cfg := a.cfg
	def, ok := cfg.Collector.SourceOrDefault(source.ID)
	if !ok {
		return collectorruntime.ResolvedSourceRuntimeConfig{}, fmt.Errorf("collector source %s not found", source.ID)
	}
	def.AuthMode = source.AuthMode
	def.CookieSecretRef = source.CookieSecretRef
	def.HeaderSecretRef = source.HeaderSecretRef
	if source.TimeoutMS > 0 {
		def.TimeoutMS = source.TimeoutMS
	}
	if source.HotlistLimit > 0 {
		def.HotlistLimit = source.HotlistLimit
	}
	if headers := decodeHeadersJSON(source.HeadersJSON); len(headers) > 0 {
		def.Headers = headers
	}
	if cfg.Collector.Sources == nil {
		cfg.Collector.Sources = map[string]config.CollectorSourceDef{}
	}
	cfg.Collector.Sources[source.ID] = def
	return a.policies.ResolveSource(cfg, source.ID)
}

func decodeHeadersJSON(data []byte) map[string]string {
	if len(data) == 0 {
		return nil
	}
	var headers map[string]string
	if err := json.Unmarshal(data, &headers); err != nil {
		return nil
	}
	return headers
}
