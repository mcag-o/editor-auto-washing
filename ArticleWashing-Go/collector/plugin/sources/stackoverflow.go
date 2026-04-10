package sources

import (
	"encoding/json"
	"fmt"
	"time"

	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
)

const stackOverflowHotlistPath = "/2.3/questions?order=desc&sort=hot&site=stackoverflow"

type stackOverflowEntry struct {
	QuestionID   int64    `json:"question_id"`
	Title        string   `json:"title"`
	Link         string   `json:"link"`
	Score        int      `json:"score"`
	CreationDate int64    `json:"creation_date"`
	Tags         []string `json:"tags"`
	Owner        struct {
		DisplayName string `json:"display_name"`
	} `json:"owner"`
}

func NewStackOverflow() (plugin.SourcePlugin, error) {
	return NewStackOverflowWithOptions(httpclient.Options{BaseURL: "https://api.stackexchange.com"})
}

func NewStackOverflowWithOptions(opts httpclient.Options) (plugin.SourcePlugin, error) {
	client, err := httpclient.New(opts)
	if err != nil {
		return nil, err
	}
	return NewStackOverflowWithClient(client), nil
}

func NewStackOverflowWithClient(client *httpclient.Client) plugin.SourcePlugin {
	return &apiPlugin{
		sourceID:      "stackoverflow",
		displayName:   "Stack Overflow",
		aliases:       []string{"stackoverflow", "StackOverflow"},
		hotlistPath:   stackOverflowHotlistPath,
		client:        client,
		decodeHotlist: decodeStackOverflowHotlist,
		normalizeHot:  normalizeStackOverflowEntry,
	}
}

func decodeStackOverflowHotlist(body []byte) ([]any, error) {
	var payload struct {
		Items []stackOverflowEntry `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]any, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, item)
	}
	return items, nil
}

func normalizeStackOverflowEntry(raw any) (plugin.HotEntry, error) {
	item, ok := raw.(stackOverflowEntry)
	if !ok {
		return plugin.HotEntry{}, fmt.Errorf("unexpected stackoverflow entry type %T", raw)
	}
	publishedAt := time.Unix(item.CreationDate, 0).UTC()
	rawJSON, err := marshalRawJSON(item)
	if err != nil {
		return plugin.HotEntry{}, err
	}
	return plugin.HotEntry{
		SourceID:     "stackoverflow",
		ExternalID:   fmt.Sprintf("%d", item.QuestionID),
		CanonicalURL: item.Link,
		Title:        item.Title,
		Author:       item.Owner.DisplayName,
		Tags:         append([]string(nil), item.Tags...),
		PublishedAt:  &publishedAt,
		RawJSON:      rawJSON,
		Metadata: map[string]any{
			"score": item.Score,
		},
	}, nil
}
