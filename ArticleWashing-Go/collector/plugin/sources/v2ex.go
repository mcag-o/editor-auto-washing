package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	htmlparser "content-hub/collector/html"
	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
	"content-hub/domain"
)

const v2exHotlistPath = "/?tab=hot"

func NewV2EX() (plugin.SourcePlugin, error) {
	return NewV2EXWithOptions(httpclient.Options{BaseURL: "https://www.v2ex.com"})
}

func NewV2EXWithOptions(opts httpclient.Options) (plugin.SourcePlugin, error) {
	client, err := httpclient.New(opts)
	if err != nil {
		return nil, err
	}
	return NewV2EXWithClient(client), nil
}

func NewV2EXWithClient(client *httpclient.Client) plugin.SourcePlugin {
	return &v2exPlugin{client: client}
}

type v2exPlugin struct {
	client *httpclient.Client
}

func (p *v2exPlugin) SourceID() string { return "v2ex" }

func (p *v2exPlugin) DisplayName() string { return "V2EX" }

func (p *v2exPlugin) Aliases() []string { return []string{"v2ex", "vtex"} }

func (p *v2exPlugin) FetchHotlist(ctx context.Context, req plugin.FetchHotlistRequest) ([]plugin.HotEntry, error) {
	resp, err := p.client.Do(ctx, httpclient.Request{Method: http.MethodGet, Path: v2exHotlistPath, Headers: req.Headers})
	if err != nil {
		return nil, fmt.Errorf("fetch hotlist for v2ex: %w", err)
	}
	entries, err := parseV2EXHotlist(resp.Body)
	if err != nil {
		return nil, err
	}
	if req.Limit > 0 && req.Limit < len(entries) {
		entries = entries[:req.Limit]
	}
	return entries, nil
}

func (p *v2exPlugin) FetchArticle(ctx context.Context, req plugin.FetchArticleRequest) (*plugin.RawArticle, error) {
	path := "/t/" + req.Entry.ExternalID
	if strings.TrimSpace(req.Entry.CanonicalURL) != "" {
		parsedURL, err := url.Parse(req.Entry.CanonicalURL)
		if err == nil && parsedURL.Path != "" {
			path = parsedURL.Path
		}
	}
	if path == "/t/" {
		path = "/t/" + req.Entry.ExternalID
	}
	resp, err := p.client.Do(ctx, httpclient.Request{Method: http.MethodGet, Path: path})
	if err != nil {
		return nil, fmt.Errorf("fetch article for v2ex: %w", err)
	}
	return &plugin.RawArticle{
		SourceID:     "v2ex",
		ExternalID:   req.Entry.ExternalID,
		CanonicalURL: req.Entry.CanonicalURL,
		Body:         resp.Body,
		ContentType:  resp.Headers.Get("Content-Type"),
		Metadata: map[string]any{
			"request_path": path,
		},
	}, nil
}

func (p *v2exPlugin) NormalizeHotEntry(raw any) (plugin.HotEntry, error) {
	entry, ok := raw.(plugin.HotEntry)
	if !ok {
		return plugin.HotEntry{}, fmt.Errorf("unexpected v2ex entry type %T", raw)
	}
	return entry, nil
}

func (p *v2exPlugin) NormalizeArticle(raw any) (*plugin.NormalizedArticle, error) {
	rawArticle, ok := raw.(*plugin.RawArticle)
	if !ok {
		return nil, fmt.Errorf("unexpected raw article type %T", raw)
	}
	return parseV2EXArticle(rawArticle)
}

func (p *v2exPlugin) HealthCheck(ctx context.Context) (plugin.SourceHealth, error) {
	_, err := p.FetchHotlist(ctx, plugin.FetchHotlistRequest{Limit: 1})
	health := plugin.SourceHealth{SourceID: p.SourceID(), OK: err == nil, Code: plugin.HealthCodeHealthy, CheckedAt: time.Now().UTC()}
	if err != nil {
		health.Code = plugin.HealthCodeUnavailable
		health.Message = err.Error()
	}
	return health, err
}

func (p *v2exPlugin) Capabilities() plugin.SourceCapabilities {
	return plugin.SourceCapabilities{SupportsHotlist: true, SupportsArticle: true, AuthModes: []string{domain.CollectorAuthModeNone}}
}

func parseV2EXHotlist(body []byte) ([]plugin.HotEntry, error) {
	doc, err := htmlparser.Parse(body)
	if err != nil {
		return nil, err
	}
	items := doc.FindAll("div", "item")
	entries := make([]plugin.HotEntry, 0, len(items))
	for index, item := range items {
		titleNode := htmlparser.FindFirst(item, "span", "item_title")
		link := htmlparser.FindFirst(titleNode, "a", "")
		title := htmlparser.Text(link)
		href := htmlparser.Attr(link, "href")
		if title == "" || href == "" {
			continue
		}
		externalID := strings.TrimPrefix(strings.Trim(strings.Split(href, "?")[0], "/"), "t/")
		canonicalURL := href
		if strings.HasPrefix(canonicalURL, "/") {
			canonicalURL = "https://www.v2ex.com" + canonicalURL
		}
		summary := htmlparser.Text(htmlparser.FindFirst(item, "span", "topic_info"))
		rawJSON, err := marshalRawJSON(map[string]any{"title": title, "url": canonicalURL, "summary": summary})
		if err != nil {
			return nil, err
		}
		entries = append(entries, plugin.HotEntry{SourceID: "v2ex", ExternalID: externalID, CanonicalURL: canonicalURL, Title: title, Summary: summary, Rank: intPointer(index + 1), RawJSON: rawJSON})
	}
	return entries, nil
}

func parseV2EXArticle(raw *plugin.RawArticle) (*plugin.NormalizedArticle, error) {
	doc, err := htmlparser.Parse(raw.Body)
	if err != nil {
		return nil, err
	}
	title := htmlparser.Text(htmlparser.FindFirstNode(doc.FindAll("h1", "")))
	author := ""
	for _, link := range doc.FindAll("a", "") {
		href := htmlparser.Attr(link, "href")
		if strings.HasPrefix(href, "/member/") {
			author = htmlparser.Text(link)
			break
		}
	}
	body := htmlparser.Text(htmlparser.FindFirstNode(doc.FindAll("div", "topic_content")))
	if title == "" || body == "" {
		return nil, fmt.Errorf("parse v2ex article: missing title or body")
	}
	return &plugin.NormalizedArticle{SourceID: "v2ex", ExternalID: raw.ExternalID, CanonicalURL: raw.CanonicalURL, Title: title, Body: body, Summary: firstSentence(body), Author: author, RawJSON: append([]byte(nil), raw.Body...), Metadata: map[string]any{"request_path": raw.Metadata["request_path"]}}, nil
}

func firstSentence(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	parts := strings.SplitN(body, ".", 2)
	return strings.TrimSpace(parts[0])
}
