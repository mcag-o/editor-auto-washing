package sources

import (
	"context"
	"fmt"
	"time"

	"content-hub/collector/plugin"
	"content-hub/infra/config"
)

// placeholderPlugin 用于尚未完成迁移的平台占位。
//
// 设计意图：
// - 把目标平台纳入统一 registry，避免后续开发者遗漏平台清单；
// - 在 source registry、health、配置落库层面先形成完整骨架，再逐个平台替换为真实实现。
type placeholderPlugin struct {
	definition config.CollectorSourceDef
	sourceID   string
}

func NewPlaceholder(definition config.CollectorSourceDef, sourceID string) plugin.SourcePlugin {
	return &placeholderPlugin{definition: definition, sourceID: sourceID}
}

func (p *placeholderPlugin) SourceID() string { return p.sourceID }

func (p *placeholderPlugin) DisplayName() string { return p.definition.DisplayName }

func (p *placeholderPlugin) Aliases() []string { return append([]string(nil), p.definition.Aliases...) }

func (p *placeholderPlugin) FetchHotlist(_ context.Context, _ plugin.FetchHotlistRequest) ([]plugin.HotEntry, error) {
	return nil, p.placeholderError("热榜抓取")
}

func (p *placeholderPlugin) FetchArticle(_ context.Context, _ plugin.FetchArticleRequest) (*plugin.RawArticle, error) {
	if !p.definition.SupportsArticle {
		return nil, plugin.ErrArticleFetchNotSupported(p.sourceID)
	}
	return nil, p.placeholderError("详情抓取")
}

func (p *placeholderPlugin) NormalizeHotEntry(_ any) (plugin.HotEntry, error) {
	return plugin.HotEntry{}, p.placeholderError("热榜标准化")
}

func (p *placeholderPlugin) NormalizeArticle(_ any) (*plugin.NormalizedArticle, error) {
	if !p.definition.SupportsArticle {
		return nil, plugin.ErrArticleFetchNotSupported(p.sourceID)
	}
	return nil, p.placeholderError("详情标准化")
}

func (p *placeholderPlugin) HealthCheck(_ context.Context) (plugin.SourceHealth, error) {
	message := fmt.Sprintf("平台 %s 当前为骨架占位，尚未接入真实抓取实现", p.sourceID)
	return plugin.SourceHealth{
		SourceID:  p.sourceID,
		OK:        false,
		Code:      plugin.HealthCodeUnavailable,
		CheckedAt: time.Now().UTC(),
		Message:   message,
	}, nil
}

func (p *placeholderPlugin) Capabilities() plugin.SourceCapabilities {
	return plugin.SourceCapabilities{
		SupportsHotlist: true,
		SupportsArticle: p.definition.SupportsArticle,
		AuthModes:       []string{p.definition.AuthMode},
	}
}

func (p *placeholderPlugin) Descriptor() plugin.SourceDefinition {
	return plugin.SourceDefinition{
		Enabled:            p.definition.Enabled,
		ScheduleEnabled:    p.definition.ScheduleEnabled,
		IntervalMinutes:    p.definition.IntervalMinutes,
		TimeoutMS:          p.definition.TimeoutMS,
		HotlistLimit:       p.definition.HotlistLimit,
		DetailFetchEnabled: p.definition.DetailFetchEnabled,
		Concurrency:        p.definition.Concurrency,
		AuthMode:           p.definition.AuthMode,
		CookieSecretRef:    p.definition.CookieSecretRef,
		HeaderSecretRef:    p.definition.HeaderSecretRef,
		Headers:            cloneStringMap(p.definition.Headers),
		RetryPolicy:        map[string]any{},
		Options: map[string]any{
			"source_type":      p.definition.SourceType,
			"source_url":       p.definition.SourceURL,
			"supports_article": p.definition.SupportsArticle,
		},
		Metadata: map[string]any{
			"aliases": append([]string(nil), p.definition.Aliases...),
		},
	}
}

func (p *placeholderPlugin) placeholderError(stage string) error {
	return fmt.Errorf("%s 平台尚未完成 %s：当前仅提供占位实现", p.sourceID, stage)
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
