package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
)

type apiPlugin struct {
	sourceID      string
	displayName   string
	aliases       []string
	hotlistPath   string
	articlePath   func(plugin.HotEntry) string
	client        *httpclient.Client
	decodeHotlist func([]byte) ([]any, error)
	normalizeHot  func(any) (plugin.HotEntry, error)
	normalizeBody func(*plugin.RawArticle) (*plugin.NormalizedArticle, error)
}

func (p *apiPlugin) SourceID() string {
	return p.sourceID
}

func (p *apiPlugin) DisplayName() string {
	return p.displayName
}

func (p *apiPlugin) Aliases() []string {
	return append([]string(nil), p.aliases...)
}

func (p *apiPlugin) FetchHotlist(ctx context.Context, req plugin.FetchHotlistRequest) ([]plugin.HotEntry, error) {
	resp, err := p.client.Do(ctx, httpclient.Request{
		Method:  http.MethodGet,
		Path:    p.hotlistPath,
		Headers: req.Headers,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch hotlist for %s: %w", p.sourceID, err)
	}

	items, err := p.decodeHotlist(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decode hotlist for %s: %w", p.sourceID, err)
	}

	limit := req.Limit
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	entries := make([]plugin.HotEntry, 0, len(items))
	for index, item := range items {
		entry, err := p.NormalizeHotEntry(item)
		if err != nil {
			return nil, fmt.Errorf("normalize hotlist entry for %s: %w", p.sourceID, err)
		}
		if entry.Rank == nil {
			entry.Rank = intPointer(index + 1)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (p *apiPlugin) FetchArticle(ctx context.Context, req plugin.FetchArticleRequest) (*plugin.RawArticle, error) {
	if p.articlePath == nil {
		return nil, plugin.ErrArticleFetchNotSupported(p.sourceID)
	}
	path := p.articlePath(req.Entry)
	resp, err := p.client.Do(ctx, httpclient.Request{
		Method: http.MethodGet,
		Path:   path,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch article for %s: %w", p.sourceID, err)
	}
	return &plugin.RawArticle{
		SourceID:     p.sourceID,
		ExternalID:   req.Entry.ExternalID,
		CanonicalURL: req.Entry.CanonicalURL,
		Body:         resp.Body,
		ContentType:  resp.Headers.Get("Content-Type"),
		Metadata: map[string]any{
			"request_path": path,
		},
	}, nil
}

func (p *apiPlugin) NormalizeHotEntry(raw any) (plugin.HotEntry, error) {
	return p.normalizeHot(raw)
}

func (p *apiPlugin) NormalizeArticle(raw any) (*plugin.NormalizedArticle, error) {
	if p.normalizeBody == nil {
		return nil, plugin.ErrArticleFetchNotSupported(p.sourceID)
	}
	rawArticle, ok := raw.(*plugin.RawArticle)
	if !ok {
		return nil, fmt.Errorf("unexpected raw article type %T", raw)
	}
	return p.normalizeBody(rawArticle)
}

func (p *apiPlugin) HealthCheck(ctx context.Context) (plugin.SourceHealth, error) {
	_, err := p.FetchHotlist(ctx, plugin.FetchHotlistRequest{Limit: 1})
	health := plugin.SourceHealth{
		SourceID:  p.sourceID,
		OK:        err == nil,
		Code:      plugin.HealthCodeHealthy,
		CheckedAt: time.Now().UTC(),
	}
	if err != nil {
		health.Code = plugin.HealthCodeUnavailable
		health.Message = err.Error()
	}
	return health, err
}

func (p *apiPlugin) Capabilities() plugin.SourceCapabilities {
	return plugin.SourceCapabilities{
		SupportsHotlist: true,
		SupportsArticle: p.articlePath != nil && p.normalizeBody != nil,
		AuthModes:       []string{"none", "header"},
	}
}

func marshalRawJSON(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal raw entry: %w", err)
	}
	return body, nil
}

func intPointer(value int) *int {
	return &value
}
