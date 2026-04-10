package integration

import (
	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
	collectorsvc "content-hub/collector/service"
	"content-hub/domain"
	"content-hub/infra/sqlite"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectorDetailFetchMainline(t *testing.T) {
	provider, err := sqlite.NewProvider(filepath.Join(t.TempDir(), "content-hub.db"))
	require.NoError(t, err)
	defer provider.Close()

	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(newIntegrationDetailPlugin(t)))

	registrySvc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, registrySvc.Sync(t.Context()))
	runSvc := collectorsvc.NewRunService(provider.CollectorSourceRepo(), provider.CollectorRunRepo(), provider.CollectorEntryRepo(), registry)
	runSummary, err := runSvc.RunHotlist(t.Context(), "scheduled")
	require.NoError(t, err)

	entries, err := provider.CollectorEntryRepo().ListByRunID(t.Context(), runSummary.RunID)
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	fetchSvc := collectorsvc.NewArticleFetchService(provider.CollectorEntryRepo(), provider.CollectorArticleRepo(), provider.CollectorAttemptRepo(), provider.CollectorRunRepo(), registry)
	article, err := fetchSvc.FetchForEntry(t.Context(), entries[0].ID)
	require.NoError(t, err)

	assert.Equal(t, entries[0].ID, article.EntryID)
	assert.Equal(t, "baidu", article.SourceID)
	assert.Equal(t, domain.CollectorEntryFetchedDetail, integrationEntryByID(t, provider, entries[0].ID).Status)
	attempts := integrationAttemptsForEntry(t, provider, entries[0].RunID, entries[0].SourceID, entries[0].ID)
	require.Len(t, attempts, 1)
	assert.Equal(t, domain.CollectorAttemptSucceeded, attempts[0].Status)
	assert.Equal(t, domain.CollectorStageDetail, attempts[0].Stage)
}

type integrationDetailPlugin struct {
	client *httpclient.Client
}

func newIntegrationDetailPlugin(t *testing.T) plugin.SourcePlugin {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixtureMap := map[string]string{
			"/api/board?platform=wise&tab=realtime":      filepath.Join("..", "testdata", "collector", "fixtures", "baidu-hotlist.json"),
			"/api/article/detail?id=7492239302142358563": filepath.Join("..", "testdata", "collector", "fixtures", "baidu-article.json"),
		}
		fixturePath, ok := fixtureMap[r.URL.RequestURI()]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := os.ReadFile(fixturePath)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	client, err := httpclient.New(httpclient.Options{BaseURL: server.URL})
	require.NoError(t, err)
	return &integrationDetailPlugin{client: client}
}

func (p *integrationDetailPlugin) SourceID() string    { return "baidu" }
func (p *integrationDetailPlugin) DisplayName() string { return "百度热搜" }
func (p *integrationDetailPlugin) Aliases() []string   { return []string{"baidu"} }
func (p *integrationDetailPlugin) HealthCheck(_ context.Context) (plugin.SourceHealth, error) {
	return plugin.SourceHealth{SourceID: "baidu", OK: true, CheckedAt: time.Now().UTC()}, nil
}
func (p *integrationDetailPlugin) Capabilities() plugin.SourceCapabilities {
	return plugin.SourceCapabilities{SupportsHotlist: true, SupportsArticle: true, AuthModes: []string{domain.CollectorAuthModeNone}}
}
func (p *integrationDetailPlugin) NormalizeHotEntry(raw any) (plugin.HotEntry, error) {
	entry, ok := raw.(plugin.HotEntry)
	if !ok {
		return plugin.HotEntry{}, fmt.Errorf("unexpected raw entry %T", raw)
	}
	return entry, nil
}
func (p *integrationDetailPlugin) NormalizeArticle(raw any) (*plugin.NormalizedArticle, error) {
	rawArticle, ok := raw.(*plugin.RawArticle)
	if !ok {
		return nil, fmt.Errorf("unexpected raw article %T", raw)
	}
	var payload struct {
		Data struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			ContentText string `json:"content_text"`
			Summary     string `json:"summary"`
			Author      string `json:"author"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rawArticle.Body, &payload); err != nil {
		return nil, err
	}
	return &plugin.NormalizedArticle{SourceID: "baidu", ExternalID: payload.Data.ID, CanonicalURL: rawArticle.CanonicalURL, Title: payload.Data.Title, Body: payload.Data.ContentText, Summary: payload.Data.Summary, Author: payload.Data.Author, RawJSON: append([]byte(nil), rawArticle.Body...), Metadata: map[string]any{"content_text": payload.Data.ContentText}}, nil
}
func (p *integrationDetailPlugin) FetchHotlist(ctx context.Context, _ plugin.FetchHotlistRequest) ([]plugin.HotEntry, error) {
	resp, err := p.client.Do(ctx, httpclient.Request{Method: http.MethodGet, Path: "/api/board?platform=wise&tab=realtime"})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Cards []struct {
				Content []struct {
					Word   string `json:"word"`
					Query  string `json:"query"`
					URL    string `json:"url"`
					Rank   int    `json:"rank"`
					NewsID string `json:"newsId"`
				} `json:"content"`
			} `json:"cards"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, err
	}
	entries := make([]plugin.HotEntry, 0)
	for _, card := range payload.Data.Cards {
		for _, item := range card.Content {
			rank := item.Rank
			entries = append(entries, plugin.HotEntry{SourceID: "baidu", ExternalID: item.NewsID, CanonicalURL: item.URL, Title: item.Word, Summary: item.Query, Rank: &rank})
		}
	}
	return entries, nil
}
func (p *integrationDetailPlugin) FetchArticle(ctx context.Context, req plugin.FetchArticleRequest) (*plugin.RawArticle, error) {
	resp, err := p.client.Do(ctx, httpclient.Request{Method: http.MethodGet, Path: "/api/article/detail?id=" + req.Entry.ExternalID})
	if err != nil {
		return nil, err
	}
	return &plugin.RawArticle{SourceID: "baidu", ExternalID: req.Entry.ExternalID, CanonicalURL: req.Entry.CanonicalURL, Body: resp.Body, ContentType: "application/json"}, nil
}

func integrationEntryByID(t *testing.T, provider *sqlite.Provider, entryID string) *domain.CollectorEntry {
	t.Helper()
	runs, err := provider.CollectorRunRepo().ListRecent(t.Context(), 10)
	require.NoError(t, err)
	for _, run := range runs {
		entries, listErr := provider.CollectorEntryRepo().ListByRunID(t.Context(), run.ID)
		require.NoError(t, listErr)
		for _, entry := range entries {
			if entry.ID == entryID {
				copyValue := entry
				return &copyValue
			}
		}
	}
	t.Fatalf("entry %s not found", entryID)
	return nil
}

func integrationAttemptsForEntry(t *testing.T, provider *sqlite.Provider, runID, sourceID, entryID string) []domain.CollectorAttempt {
	t.Helper()
	sourceRuns, err := provider.CollectorRunRepo().ListSourceRuns(t.Context(), runID)
	require.NoError(t, err)
	var sourceRunID string
	for _, sourceRun := range sourceRuns {
		if sourceRun.SourceID == sourceID {
			sourceRunID = sourceRun.ID
			break
		}
	}
	require.NotEmpty(t, sourceRunID)
	attempts, err := provider.CollectorAttemptRepo().ListBySourceRunID(t.Context(), sourceRunID)
	require.NoError(t, err)
	filtered := make([]domain.CollectorAttempt, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt.EntryID == entryID {
			filtered = append(filtered, attempt)
		}
	}
	return filtered
}
