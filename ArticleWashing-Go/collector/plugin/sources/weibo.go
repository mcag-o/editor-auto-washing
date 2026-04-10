package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
	"content-hub/domain"
)

const weiboHotlistPath = "/ajax/side/hotSearch"
const defaultWeiboCookieRef = "env.WEIBO_COOKIE"

type SecretResolver interface {
	Resolve(ref string) (string, error)
}

type SecretResolverFunc func(ref string) (string, error)

func (f SecretResolverFunc) Resolve(ref string) (string, error) { return f(ref) }

type envSecretResolver struct{}

func (envSecretResolver) Resolve(ref string) (string, error) {
	if !strings.HasPrefix(ref, "env.") {
		return "", nil
	}
	return strings.TrimSpace(os.Getenv(strings.TrimPrefix(ref, "env."))), nil
}

type weiboPlugin struct {
	client          *httpclient.Client
	cookieSecretRef string
	resolver        SecretResolver
}

type weiboEntry struct {
	Word   string `json:"word"`
	RawHot string `json:"raw_hot"`
	Note   string `json:"note"`
}

func NewWeibo() (plugin.SourcePlugin, error) {
	return NewWeiboWithOptions(httpclient.Options{BaseURL: "https://weibo.com"}, defaultWeiboCookieRef, envSecretResolver{})
}

func NewWeiboWithOptions(opts httpclient.Options, cookieSecretRef string, resolver SecretResolver) (plugin.SourcePlugin, error) {
	client, err := httpclient.New(opts)
	if err != nil {
		return nil, err
	}
	return NewWeiboWithClient(client, cookieSecretRef, resolver), nil
}

func NewWeiboWithClient(client *httpclient.Client, cookieSecretRef string, resolver SecretResolver) plugin.SourcePlugin {
	if resolver == nil {
		resolver = envSecretResolver{}
	}
	if strings.TrimSpace(cookieSecretRef) == "" {
		cookieSecretRef = defaultWeiboCookieRef
	}
	return &weiboPlugin{client: client, cookieSecretRef: cookieSecretRef, resolver: resolver}
}

func (p *weiboPlugin) SourceID() string { return "weibo" }

func (p *weiboPlugin) DisplayName() string { return "微博热搜" }

func (p *weiboPlugin) Aliases() []string { return []string{"weibo"} }

func (p *weiboPlugin) FetchHotlist(ctx context.Context, req plugin.FetchHotlistRequest) ([]plugin.HotEntry, error) {
	cookie, err := p.resolveCookie()
	if err != nil {
		return nil, err
	}
	if cookie == "" {
		return nil, fmt.Errorf("missing cookie secret %s", p.cookieSecretRef)
	}
	resp, requestErr := p.client.Do(ctx, httpclient.Request{Method: http.MethodGet, Path: weiboHotlistPath, Headers: mergeHeaders(req.Headers, map[string]string{"Cookie": cookie})})
	if requestErr != nil {
		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return nil, fmt.Errorf("weibo auth expired: status %d", resp.StatusCode)
		}
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
	cookie, err := p.resolveCookie()
	if err != nil {
		health.Code = plugin.HealthCodeUnavailable
		health.Message = err.Error()
		return health, err
	}
	if cookie == "" {
		health.Code = plugin.HealthCodeAuthMissing
		health.Message = fmt.Sprintf("missing cookie secret %s", p.cookieSecretRef)
		return health, nil
	}
	resp, requestErr := p.client.Do(ctx, httpclient.Request{Method: http.MethodGet, Path: weiboHotlistPath, Headers: map[string]string{"Cookie": cookie}})
	if requestErr != nil {
		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			health.Code = plugin.HealthCodeAuthExpired
			health.Message = fmt.Sprintf("cookie expired for %s", p.cookieSecretRef)
			return health, nil
		}
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

func (p *weiboPlugin) WithSourceConfig(source domain.CollectorSource) plugin.SourcePlugin {
	cookieSecretRef := strings.TrimSpace(source.CookieSecretRef)
	if cookieSecretRef == "" {
		cookieSecretRef = p.cookieSecretRef
	}
	return &weiboPlugin{
		client:          p.client,
		cookieSecretRef: cookieSecretRef,
		resolver:        p.resolver,
	}
}

func (p *weiboPlugin) resolveCookie() (string, error) {
	value, err := p.resolver.Resolve(p.cookieSecretRef)
	if err != nil {
		return "", fmt.Errorf("resolve cookie secret %s: %w", p.cookieSecretRef, err)
	}
	return strings.TrimSpace(value), nil
}

func mergeHeaders(base map[string]string, extra map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
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
