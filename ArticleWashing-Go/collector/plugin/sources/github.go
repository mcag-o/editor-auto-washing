package sources

import (
	"encoding/json"
	"fmt"
	"time"

	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
)

const githubHotlistPath = "/search/repositories?q=stars:%3E1&sort=stars"
const githubArticlePath = "/repos/%s/readme"

type githubEntry struct {
	FullName        string `json:"full_name"`
	HTMLURL         string `json:"html_url"`
	Description     string `json:"description"`
	StargazersCount int    `json:"stargazers_count"`
	CreatedAt       string `json:"created_at"`
	Owner           struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type githubArticle struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Author  string `json:"author"`
	Body    string `json:"body"`
}

func NewGitHub() (plugin.SourcePlugin, error) {
	return NewGitHubWithOptions(httpclient.Options{
		BaseURL: "https://api.github.com",
		DefaultHeaders: map[string]string{
			"Accept": "application/vnd.github+json",
		},
	})
}

func NewGitHubWithOptions(opts httpclient.Options) (plugin.SourcePlugin, error) {
	client, err := httpclient.New(opts)
	if err != nil {
		return nil, err
	}
	return NewGitHubWithClient(client), nil
}

func NewGitHubWithClient(client *httpclient.Client) plugin.SourcePlugin {
	return &apiPlugin{
		sourceID:      "github",
		displayName:   "GitHub Trending",
		aliases:       []string{"github"},
		hotlistPath:   githubHotlistPath,
		articlePath:   githubArticleRequestPath,
		client:        client,
		decodeHotlist: decodeGitHubHotlist,
		normalizeHot:  normalizeGitHubEntry,
		normalizeBody: normalizeGitHubArticle,
	}
}

func decodeGitHubHotlist(body []byte) ([]any, error) {
	var payload struct {
		Items []githubEntry `json:"items"`
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

func normalizeGitHubEntry(raw any) (plugin.HotEntry, error) {
	item, ok := raw.(githubEntry)
	if !ok {
		return plugin.HotEntry{}, fmt.Errorf("unexpected github entry type %T", raw)
	}
	publishedAt, err := time.Parse(time.RFC3339, item.CreatedAt)
	if err != nil {
		return plugin.HotEntry{}, fmt.Errorf("parse github created_at: %w", err)
	}
	rawJSON, err := marshalRawJSON(item)
	if err != nil {
		return plugin.HotEntry{}, err
	}
	return plugin.HotEntry{
		SourceID:     "github",
		ExternalID:   item.FullName,
		CanonicalURL: item.HTMLURL,
		Title:        item.FullName,
		Summary:      item.Description,
		Author:       item.Owner.Login,
		PublishedAt:  &publishedAt,
		RawJSON:      rawJSON,
		Metadata: map[string]any{
			"stars": item.StargazersCount,
		},
	}, nil
}

func githubArticleRequestPath(entry plugin.HotEntry) string {
	return fmt.Sprintf(githubArticlePath, entry.ExternalID)
}

func normalizeGitHubArticle(raw *plugin.RawArticle) (*plugin.NormalizedArticle, error) {
	var payload githubArticle
	if err := json.Unmarshal(raw.Body, &payload); err != nil {
		return nil, fmt.Errorf("decode github article: %w", err)
	}
	return &plugin.NormalizedArticle{
		SourceID:     "github",
		ExternalID:   raw.ExternalID,
		CanonicalURL: raw.CanonicalURL,
		Title:        payload.Title,
		Body:         payload.Body,
		Summary:      payload.Summary,
		Author:       payload.Author,
		RawJSON:      append([]byte(nil), raw.Body...),
		Metadata: map[string]any{
			"request_path": raw.Metadata["request_path"],
		},
	}, nil
}
