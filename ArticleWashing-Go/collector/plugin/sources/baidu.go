package sources

import (
	"encoding/json"
	"fmt"

	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
)

const baiduHotlistPath = "/api/board?platform=wise&tab=realtime"
const baiduArticlePath = "/api/article/detail?id=%s"

type baiduEntry struct {
	Word     string `json:"word"`
	Query    string `json:"query"`
	URL      string `json:"url"`
	HotScore string `json:"hotScore"`
	Rank     int    `json:"rank"`
	NewsID   string `json:"newsId"`
}

type baiduArticle struct {
	Data struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		ContentText string `json:"content_text"`
		Summary     string `json:"summary"`
		Author      string `json:"author"`
	} `json:"data"`
}

func NewBaidu() (plugin.SourcePlugin, error) {
	return NewBaiduWithOptions(httpclient.Options{BaseURL: "https://top.baidu.com"})
}

func NewBaiduWithOptions(opts httpclient.Options) (plugin.SourcePlugin, error) {
	client, err := httpclient.New(opts)
	if err != nil {
		return nil, err
	}
	return NewBaiduWithClient(client), nil
}

func NewBaiduWithClient(client *httpclient.Client) plugin.SourcePlugin {
	return &apiPlugin{
		sourceID:      "baidu",
		displayName:   "百度热搜",
		aliases:       []string{"baidu"},
		hotlistPath:   baiduHotlistPath,
		articlePath:   baiduArticleRequestPath,
		client:        client,
		decodeHotlist: decodeBaiduHotlist,
		normalizeHot:  normalizeBaiduEntry,
		normalizeBody: normalizeBaiduArticle,
	}
}

func decodeBaiduHotlist(body []byte) ([]any, error) {
	var payload struct {
		Data struct {
			Cards []struct {
				Content []baiduEntry `json:"content"`
			} `json:"cards"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]any, 0)
	for _, card := range payload.Data.Cards {
		for _, item := range card.Content {
			items = append(items, item)
		}
	}
	return items, nil
}

func normalizeBaiduEntry(raw any) (plugin.HotEntry, error) {
	item, ok := raw.(baiduEntry)
	if !ok {
		return plugin.HotEntry{}, fmt.Errorf("unexpected baidu entry type %T", raw)
	}
	rawJSON, err := marshalRawJSON(item)
	if err != nil {
		return plugin.HotEntry{}, err
	}
	return plugin.HotEntry{
		SourceID:     "baidu",
		ExternalID:   item.NewsID,
		CanonicalURL: item.URL,
		Title:        item.Word,
		Summary:      item.Query,
		Rank:         intPointer(item.Rank),
		RawJSON:      rawJSON,
		Metadata: map[string]any{
			"query":     item.Query,
			"hot_score": item.HotScore,
		},
	}, nil
}

func baiduArticleRequestPath(entry plugin.HotEntry) string {
	return fmt.Sprintf(baiduArticlePath, entry.ExternalID)
}

func normalizeBaiduArticle(raw *plugin.RawArticle) (*plugin.NormalizedArticle, error) {
	var payload baiduArticle
	if err := json.Unmarshal(raw.Body, &payload); err != nil {
		return nil, fmt.Errorf("decode baidu article: %w", err)
	}
	return &plugin.NormalizedArticle{
		SourceID:     "baidu",
		ExternalID:   payload.Data.ID,
		CanonicalURL: raw.CanonicalURL,
		Title:        payload.Data.Title,
		Body:         payload.Data.ContentText,
		Summary:      payload.Data.Summary,
		Author:       payload.Data.Author,
		RawJSON:      append([]byte(nil), raw.Body...),
		Metadata: map[string]any{
			"request_path": raw.Metadata["request_path"],
		},
	}, nil
}
