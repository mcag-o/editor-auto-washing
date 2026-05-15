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
// - 把 22 个目标平台全部纳入统一 registry，避免后续开发者遗漏平台清单；
// - 通过中文注释与错误信息直接提示当前工作目标；
// - 在 source registry、health、配置落库层面先形成完整骨架，再逐个平台替换为真实实现。
//
// 后续开发要求：
// 1. 先保留当前平台元数据与 Todo；
// 2. 新实现完成后，删除该平台在 registry 中的占位注册；
// 3. 保留这里的中文工作说明，或者迁移到真实实现文件顶部。
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
	return nil, p.todoError("热榜抓取")
}

func (p *placeholderPlugin) FetchArticle(_ context.Context, _ plugin.FetchArticleRequest) (*plugin.RawArticle, error) {
	if !p.definition.SupportsArticle {
		return nil, plugin.ErrArticleFetchNotSupported(p.sourceID)
	}
	return nil, p.todoError("详情抓取")
}

func (p *placeholderPlugin) NormalizeHotEntry(_ any) (plugin.HotEntry, error) {
	return plugin.HotEntry{}, p.todoError("热榜标准化")
}

func (p *placeholderPlugin) NormalizeArticle(_ any) (*plugin.NormalizedArticle, error) {
	if !p.definition.SupportsArticle {
		return nil, plugin.ErrArticleFetchNotSupported(p.sourceID)
	}
	return nil, p.todoError("详情标准化")
}

func (p *placeholderPlugin) HealthCheck(_ context.Context) (plugin.SourceHealth, error) {
	message := fmt.Sprintf("平台 %s 当前为骨架占位：%s。后续任务：%s", p.sourceID, p.definition.Goal, joinTodo(p.definition.Todo))
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
			"source_type":          p.definition.SourceType,
			"source_url":           p.definition.SourceURL,
			"status":               p.definition.Status,
			"goal":                 p.definition.Goal,
			"placeholder_required": p.definition.PlaceholderRequired,
			"supports_article":     p.definition.SupportsArticle,
		},
		Metadata: map[string]any{
			"aliases":             append([]string(nil), p.definition.Aliases...),
			"todo":                append([]string(nil), p.definition.Todo...),
			"notes":               append([]string(nil), p.definition.Notes...),
			"implementation_reference": p.definition.ImplementationReference,
		},
	}
}

func (p *placeholderPlugin) todoError(stage string) error {
	return fmt.Errorf("%s 平台尚未完成 %s：%s；后续开发事项：%s", p.sourceID, stage, p.definition.Goal, joinTodo(p.definition.Todo))
}

func joinTodo(items []string) string {
	if len(items) == 0 {
		return "请参考 collector 配置中的占位说明继续补齐实现"
	}
	result := items[0]
	for i := 1; i < len(items); i++ {
		result += "；" + items[i]
	}
	return result
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
