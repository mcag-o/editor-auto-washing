package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
	"content-hub/domain"
)

const weiboHotlistPath = "/ajax/side/hotSearch"

type weiboPlugin struct {
	client *httpclient.Client
}

type weiboEntry struct {
	Word   string `json:"word"`
	RawHot string `json:"raw_hot"`
	Note   string `json:"note"`
}

func NewWeibo() (plugin.SourcePlugin, error) {
	return NewWeiboWithOptions(httpclient.Options{BaseURL: "https://weibo.com"})
}

func NewWeiboWithOptions(opts httpclient.Options) (plugin.SourcePlugin, error) {
	client, err := httpclient.New(opts)
	if err != nil {
		return nil, err
	}
	return NewWeiboWithClient(client), nil
}

func NewWeiboWithClient(client *httpclient.Client) plugin.SourcePlugin {
	return &weiboPlugin{client: client}
}

func (p *weiboPlugin) SourceID() string { return "weibo" }

func (p *weiboPlugin) DisplayName() string { return "微博热搜" }

func (p *weiboPlugin) Aliases() []string { return []string{"weibo"} }

func (p *weiboPlugin) FetchHotlist(ctx context.Context, req plugin.FetchHotlistRequest) ([]plugin.HotEntry, error) {
	resp, requestErr := p.client.Do(ctx, httpclient.Request{Method: http.MethodGet, Path: weiboHotlistPath, Headers: req.Headers})
	if requestErr != nil {
		return nil, fmt.Errorf("fetch hotlist for weibo: %w", requestErr)
	}
	var payload struct {
		Data struct {
			Realtime []weiboEntry `json:"realtime"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, fmt.Errorf("decode hotlist for weibo: %w", err)
	}
	entries := make([]plugin.HotEntry, 0, len(payload.Data.Realtime))
	for index, item := range payload.Data.Realtime {
		if strings.TrimSpace(item.Word) == "" {
			continue
		}
		query := url.QueryEscape("#" + item.Word + "#")
		rawJSON, err := marshalRawJSON(item)
		if err != nil {
			return nil, err
		}
		entries = append(entries, plugin.HotEntry{SourceID: "weibo", ExternalID: item.Word, CanonicalURL: "https://s.weibo.com/weibo?q=" + query, Title: item.Word, Summary: firstNonEmpty(item.Note, item.Word), Rank: intPointer(index + 1), RawJSON: rawJSON, Metadata: map[string]any{"hot": item.RawHot}})
	}
	if req.Limit > 0 && req.Limit < len(entries) {
		entries = entries[:req.Limit]
	}
	return entries, nil
}

func (p *weiboPlugin) FetchArticle(_ context.Context, _ plugin.FetchArticleRequest) (*plugin.RawArticle, error) {
	return nil, plugin.ErrArticleFetchNotSupported(p.SourceID())
}

func (p *weiboPlugin) NormalizeHotEntry(raw any) (plugin.HotEntry, error) {
	entry, ok := raw.(plugin.HotEntry)
	if !ok {
		return plugin.HotEntry{}, fmt.Errorf("unexpected weibo entry type %T", raw)
	}
	return entry, nil
}

func (p *weiboPlugin) NormalizeArticle(any) (*plugin.NormalizedArticle, error) {
	return nil, plugin.ErrArticleFetchNotSupported(p.SourceID())
}

func (p *weiboPlugin) HealthCheck(ctx context.Context) (plugin.SourceHealth, error) {
	health := plugin.SourceHealth{SourceID: p.SourceID(), CheckedAt: time.Now().UTC()}
	_, requestErr := p.client.Do(ctx, httpclient.Request{Method: http.MethodGet, Path: weiboHotlistPath})
	if requestErr != nil {
		health.Code = plugin.HealthCodeUnavailable
		health.Message = requestErr.Error()
		return health, requestErr
	}
	health.OK = true
	health.Code = plugin.HealthCodeHealthy
	return health, nil
}

func (p *weiboPlugin) Capabilities() plugin.SourceCapabilities {
	return plugin.SourceCapabilities{SupportsHotlist: true, SupportsArticle: false, AuthModes: []string{domain.CollectorAuthModeCookie}}
}

// Descriptor 用于把微博平台的运行时采集配置同步到持久化 source 配置。
// 该平台当前仍是 hotlist-only，因此显式声明 DetailFetchEnabled=false。
func (p *weiboPlugin) Descriptor() plugin.SourceDefinition {
	return plugin.SourceDefinition{
		Enabled:            true,
		ScheduleEnabled:    true,
		IntervalMinutes:    30,
		TimeoutMS:          10000,
		HotlistLimit:       50,
		DetailFetchEnabled: false,
		Concurrency:        1,
		AuthMode:           domain.CollectorAuthModeCookie,
		Headers:            map[string]string{},
		RetryPolicy:        map[string]any{},
		Options: map[string]any{
			"source_type": "json-api",
			"source_url":  "https://weibo.com/ajax/side/hotSearch",
		},
		Metadata: map[string]any{
			"implementation": "weibo_cookie_hotlist",
		},
	}
}

func (p *weiboPlugin) WithSourceConfig(source domain.CollectorSource) plugin.SourcePlugin {
	return &weiboPlugin{client: p.client}
}

func (p *weiboPlugin) WithHTTPClient(client *httpclient.Client) plugin.SourcePlugin {
	return &weiboPlugin{client: client}
}

func (p *weiboPlugin) HTTPClient() *httpclient.Client {
	return p.client
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
