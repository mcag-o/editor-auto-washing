package sources

import (
	"encoding/json"
	"fmt"

	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
)

const bilibiliHotlistPath = "/x/web-interface/popular"

type bilibiliEntry struct {
	AID   int64  `json:"aid"`
	BVID  string `json:"bvid"`
	Title string `json:"title"`
	Desc  string `json:"desc"`
	Owner struct {
		Name string `json:"name"`
	} `json:"owner"`
}

func NewBilibili() (plugin.SourcePlugin, error) {
	return NewBilibiliWithOptions(httpclient.Options{BaseURL: "https://api.bilibili.com"})
}

func NewBilibiliWithOptions(opts httpclient.Options) (plugin.SourcePlugin, error) {
	client, err := httpclient.New(opts)
	if err != nil {
		return nil, err
	}
	return NewBilibiliWithClient(client), nil
}

func NewBilibiliWithClient(client *httpclient.Client) plugin.SourcePlugin {
	return &apiPlugin{
		sourceID:      "bilibili",
		displayName:   "哔哩哔哩",
		aliases:       []string{"bilibili"},
		hotlistPath:   bilibiliHotlistPath,
		client:        client,
		decodeHotlist: decodeBilibiliHotlist,
		normalizeHot:  normalizeBilibiliEntry,
	}
}

func decodeBilibiliHotlist(body []byte) ([]any, error) {
	var payload struct {
		Data struct {
			List []bilibiliEntry `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]any, 0, len(payload.Data.List))
	for _, item := range payload.Data.List {
		items = append(items, item)
	}
	return items, nil
}

func normalizeBilibiliEntry(raw any) (plugin.HotEntry, error) {
	item, ok := raw.(bilibiliEntry)
	if !ok {
		return plugin.HotEntry{}, fmt.Errorf("unexpected bilibili entry type %T", raw)
	}
	rawJSON, err := marshalRawJSON(item)
	if err != nil {
		return plugin.HotEntry{}, err
	}
	return plugin.HotEntry{
		SourceID:     "bilibili",
		ExternalID:   fmt.Sprintf("%d", item.AID),
		CanonicalURL: fmt.Sprintf("https://www.bilibili.com/video/%s", item.BVID),
		Title:        item.Title,
		Summary:      item.Desc,
		Author:       item.Owner.Name,
		RawJSON:      rawJSON,
		Metadata: map[string]any{
			"aid":  item.AID,
			"bvid": item.BVID,
		},
	}, nil
}
