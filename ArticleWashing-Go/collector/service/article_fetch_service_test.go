package service_test

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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArticleFetchService_FetchesDetailAndPersistsArticle(t *testing.T) {
	provider := newArticleFetchProvider(t)
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(newDetailPluginFromFixtures(t, "baidu", map[string]string{
		"/api/board?platform=wise&tab=realtime":      "/testdata/collector/fixtures/baidu-hotlist.json",
		"/api/article/detail?id=7492239302142358563": "/testdata/collector/fixtures/baidu-article.json",
	})))

	registrySvc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, registrySvc.Sync(t.Context()))
	runSvc := collectorsvc.NewRunService(provider.CollectorSourceRepo(), provider.CollectorRunRepo(), provider.CollectorEntryRepo(), registry)
	run, err := runSvc.RunHotlist(t.Context(), "manual")
	require.NoError(t, err)

	entries, err := provider.CollectorEntryRepo().ListByRunID(t.Context(), run.RunID)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	svc := collectorsvc.NewArticleFetchService(provider.CollectorEntryRepo(), provider.CollectorArticleRepo(), provider.CollectorAttemptRepo(), provider.CollectorRunRepo(), registry)
	article, err := svc.FetchForEntry(t.Context(), entries[0].ID)

	require.NoError(t, err)
	assert.Equal(t, entries[0].ID, article.EntryID)
	assert.Equal(t, "baidu", article.SourceID)
	assert.NotEmpty(t, article.Body)
	assert.Equal(t, domain.CollectorEntryFetchedDetail, mustEntryByID(t, provider, entries[0].ID).Status)
	assert.Len(t, mustAttemptsForEntry(t, provider, entries[0].RunID, entries[0].SourceID, entries[0].ID), 1)
	assert.NotEmpty(t, mustArticleByID(t, provider, article.ID).RawJSON)
}

func TestArticleFetchService_RecordsFailedAttemptAndMarksEntryFailed(t *testing.T) {
	provider := newArticleFetchProvider(t)
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(newDetailPluginFromFixtures(t, "github", map[string]string{
		"/search/repositories?q=stars:%3E1&sort=stars": "/testdata/collector/fixtures/github-hotlist.json",
		"/repos/openai/gpt-oss/readme":                 "/testdata/collector/fixtures/github-article.json",
	})))

	registrySvc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, registrySvc.Sync(t.Context()))
	runSvc := collectorsvc.NewRunService(provider.CollectorSourceRepo(), provider.CollectorRunRepo(), provider.CollectorEntryRepo(), registry)
	run, err := runSvc.RunHotlist(t.Context(), "manual")
	require.NoError(t, err)

	entries, err := provider.CollectorEntryRepo().ListByRunID(t.Context(), run.RunID)
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	svc := collectorsvc.NewArticleFetchService(provider.CollectorEntryRepo(), provider.CollectorArticleRepo(), provider.CollectorAttemptRepo(), provider.CollectorRunRepo(), plugin.NewRegistry())
	_, err = svc.FetchForEntry(t.Context(), entries[0].ID)

	require.Error(t, err)
	assert.Equal(t, domain.CollectorEntryDetailFailed, mustEntryByID(t, provider, entries[0].ID).Status)
	attempts := mustAttemptsForEntry(t, provider, entries[0].RunID, entries[0].SourceID, entries[0].ID)
	require.Len(t, attempts, 1)
	assert.Equal(t, domain.CollectorAttemptFailed, attempts[0].Status)
	assert.Equal(t, domain.CollectorErrorSchemaChanged, attempts[0].ErrorCode)
}

func TestArticleFetchService_RepeatedFetchReturnsExistingArticle(t *testing.T) {
	provider := newArticleFetchProvider(t)
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(newDetailPluginFromFixtures(t, "baidu", map[string]string{
		"/api/board?platform=wise&tab=realtime":      "/testdata/collector/fixtures/baidu-hotlist.json",
		"/api/article/detail?id=7492239302142358563": "/testdata/collector/fixtures/baidu-article.json",
	})))
	registrySvc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, registrySvc.Sync(t.Context()))
	runSvc := collectorsvc.NewRunService(provider.CollectorSourceRepo(), provider.CollectorRunRepo(), provider.CollectorEntryRepo(), registry)
	run, err := runSvc.RunHotlist(t.Context(), "manual")
	require.NoError(t, err)
	entries, err := provider.CollectorEntryRepo().ListByRunID(t.Context(), run.RunID)
	require.NoError(t, err)

	svc := collectorsvc.NewArticleFetchService(provider.CollectorEntryRepo(), provider.CollectorArticleRepo(), provider.CollectorAttemptRepo(), provider.CollectorRunRepo(), registry)
	first, err := svc.FetchForEntry(t.Context(), entries[0].ID)
	require.NoError(t, err)
	second, err := svc.FetchForEntry(t.Context(), entries[0].ID)
	require.NoError(t, err)

	assert.Equal(t, first.ID, second.ID)
	attempts := mustAttemptsForEntry(t, provider, entries[0].RunID, entries[0].SourceID, entries[0].ID)
	assert.Len(t, attempts, 1)
}

func TestArticleFetchService_CompensatesWhenEntryUpdateFailsAfterArticleCreate(t *testing.T) {
	entry := domain.NewCollectorEntry("run-1", "baidu", "external-1", "Entry One", "https://example.com/entry-1")
	entryRepo := &stubCollectorEntryRepo{entry: cloneCollectorEntry(entry), updateErr: fmt.Errorf("entry update failed")}
	articleRepo := &stubCollectorArticleRepo{}
	attemptRepo := &stubCollectorAttemptRepo{}
	runRepo := &stubSourceRunReader{sourceRunID: "source-run-1"}
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(&stubFetchPlugin{normalized: &plugin.NormalizedArticle{SourceID: "baidu", ExternalID: "external-1", CanonicalURL: entry.CanonicalURL, Title: "Entry One", Body: "body"}}))

	svc := collectorsvc.NewArticleFetchService(entryRepo, articleRepo, attemptRepo, runRepo, registry)
	article, err := svc.FetchForEntry(t.Context(), entry.ID)

	require.Error(t, err)
	assert.Nil(t, article)
	assert.Len(t, articleRepo.created, 1)
	assert.Len(t, articleRepo.deletedIDs, 1)
	assert.Equal(t, articleRepo.created[0].ID, articleRepo.deletedIDs[0])
	assert.Empty(t, attemptRepo.created)
	assert.Equal(t, domain.CollectorEntryPendingDetail, entryRepo.entry.Status)
}

func TestArticleFetchService_CompensatesWhenAttemptCreateFails(t *testing.T) {
	entry := domain.NewCollectorEntry("run-1", "baidu", "external-1", "Entry One", "https://example.com/entry-1")
	entryRepo := &stubCollectorEntryRepo{entry: cloneCollectorEntry(entry)}
	articleRepo := &stubCollectorArticleRepo{}
	attemptRepo := &stubCollectorAttemptRepo{createErr: fmt.Errorf("attempt create failed")}
	runRepo := &stubSourceRunReader{sourceRunID: "source-run-1"}
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(&stubFetchPlugin{normalized: &plugin.NormalizedArticle{SourceID: "baidu", ExternalID: "external-1", CanonicalURL: entry.CanonicalURL, Title: "Entry One", Body: "body"}}))

	svc := collectorsvc.NewArticleFetchService(entryRepo, articleRepo, attemptRepo, runRepo, registry)
	article, err := svc.FetchForEntry(t.Context(), entry.ID)

	require.Error(t, err)
	assert.Nil(t, article)
	assert.Len(t, articleRepo.deletedIDs, 1)
	assert.Equal(t, domain.CollectorEntryPendingDetail, entryRepo.entry.Status)
}

func TestArticleFetchService_SurfacesCleanupFailureAlongsideOriginalFailure(t *testing.T) {
	entry := domain.NewCollectorEntry("run-1", "baidu", "external-1", "Entry One", "https://example.com/entry-1")
	entryRepo := &stubCollectorEntryRepo{entry: cloneCollectorEntry(entry), updateErr: fmt.Errorf("entry update failed")}
	articleRepo := &stubCollectorArticleRepo{deleteErr: fmt.Errorf("article rollback failed")}
	attemptRepo := &stubCollectorAttemptRepo{}
	runRepo := &stubSourceRunReader{sourceRunID: "source-run-1"}
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(&stubFetchPlugin{normalized: &plugin.NormalizedArticle{SourceID: "baidu", ExternalID: "external-1", CanonicalURL: entry.CanonicalURL, Title: "Entry One", Body: "body"}}))

	svc := collectorsvc.NewArticleFetchService(entryRepo, articleRepo, attemptRepo, runRepo, registry)
	article, err := svc.FetchForEntry(t.Context(), entry.ID)

	require.Error(t, err)
	assert.Nil(t, article)
	assert.ErrorContains(t, err, "entry update failed")
	assert.ErrorContains(t, err, "article rollback failed")
}

func newArticleFetchProvider(t *testing.T) *sqlite.Provider {
	t.Helper()
	dbPath := fmt.Sprintf("%s/test_article_fetch_%d.db", t.TempDir(), os.Getpid())
	provider, err := sqlite.NewProvider(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	return provider
}

func newDetailPluginFromFixtures(t *testing.T, sourceID string, routes map[string]string) plugin.SourcePlugin {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture, ok := routes[r.URL.RequestURI()]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := os.ReadFile(filepath.Join("..", "..", strings.TrimPrefix(fixture, "/")))
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	client, err := httpclient.New(httpclient.Options{BaseURL: server.URL})
	require.NoError(t, err)
	return &detailFixturePlugin{sourceID: sourceID, client: client}
}

func mustEntryByID(t *testing.T, provider *sqlite.Provider, entryID string) *domain.CollectorEntry {
	t.Helper()
	runs, err := provider.CollectorRunRepo().ListRecent(context.Background(), 20)
	require.NoError(t, err)
	for _, run := range runs {
		entries, listErr := provider.CollectorEntryRepo().ListByRunID(context.Background(), run.ID)
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

func mustArticleByID(t *testing.T, provider *sqlite.Provider, articleID string) *domain.CollectorArticle {
	t.Helper()
	article, err := provider.CollectorArticleRepo().GetByID(context.Background(), articleID)
	require.NoError(t, err)
	return article
}

func mustAttemptsForEntry(t *testing.T, provider *sqlite.Provider, runID, sourceID, entryID string) []domain.CollectorAttempt {
	t.Helper()
	sourceRuns, err := provider.CollectorRunRepo().ListSourceRuns(context.Background(), runID)
	require.NoError(t, err)
	var sourceRunID string
	for _, sourceRun := range sourceRuns {
		if sourceRun.SourceID == sourceID {
			sourceRunID = sourceRun.ID
			break
		}
	}
	require.NotEmpty(t, sourceRunID)
	attempts, err := provider.CollectorAttemptRepo().ListBySourceRunID(context.Background(), sourceRunID)
	require.NoError(t, err)
	filtered := make([]domain.CollectorAttempt, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt.EntryID == entryID {
			filtered = append(filtered, attempt)
		}
	}
	return filtered
}

type stubCollectorEntryRepo struct {
	entry      *domain.CollectorEntry
	updateErr  error
	updateHits int
}

func (r *stubCollectorEntryRepo) Create(context.Context, *domain.CollectorEntry) error { return nil }
func (r *stubCollectorEntryRepo) GetByID(context.Context, string) (*domain.CollectorEntry, error) {
	if r.entry == nil {
		return nil, domain.NewNotFoundErr("collector_entry", "missing")
	}
	return cloneCollectorEntry(r.entry), nil
}
func (r *stubCollectorEntryRepo) Update(_ context.Context, entry *domain.CollectorEntry) error {
	r.updateHits++
	if r.updateErr != nil {
		return r.updateErr
	}
	r.entry = cloneCollectorEntry(entry)
	return nil
}
func (r *stubCollectorEntryRepo) ListByRunID(context.Context, string) ([]domain.CollectorEntry, error) {
	return nil, nil
}

type stubCollectorArticleRepo struct {
	created    []*domain.CollectorArticle
	stored     map[string]*domain.CollectorArticle
	deletedIDs []string
	updateErr  error
	deleteErr  error
}

func (r *stubCollectorArticleRepo) Create(_ context.Context, article *domain.CollectorArticle) error {
	if r.stored == nil {
		r.stored = map[string]*domain.CollectorArticle{}
	}
	copyValue := *article
	r.created = append(r.created, &copyValue)
	r.stored[article.ID] = &copyValue
	return nil
}
func (r *stubCollectorArticleRepo) GetByID(_ context.Context, id string) (*domain.CollectorArticle, error) {
	if article, ok := r.stored[id]; ok {
		copyValue := *article
		return &copyValue, nil
	}
	return nil, domain.NewNotFoundErr("collector_article", id)
}
func (r *stubCollectorArticleRepo) GetByEntryID(_ context.Context, entryID string) (*domain.CollectorArticle, error) {
	for _, article := range r.stored {
		if article.EntryID == entryID {
			copyValue := *article
			return &copyValue, nil
		}
	}
	return nil, domain.NewNotFoundErr("collector_article", entryID)
}
func (r *stubCollectorArticleRepo) Update(_ context.Context, article *domain.CollectorArticle) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	if r.stored == nil {
		r.stored = map[string]*domain.CollectorArticle{}
	}
	copyValue := *article
	r.stored[article.ID] = &copyValue
	return nil
}
func (r *stubCollectorArticleRepo) Delete(_ context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletedIDs = append(r.deletedIDs, id)
	delete(r.stored, id)
	return nil
}

type stubCollectorAttemptRepo struct {
	created   []*domain.CollectorAttempt
	createErr error
}

func (r *stubCollectorAttemptRepo) Create(_ context.Context, attempt *domain.CollectorAttempt) error {
	if r.createErr != nil {
		return r.createErr
	}
	copyValue := *attempt
	r.created = append(r.created, &copyValue)
	return nil
}
func (r *stubCollectorAttemptRepo) ListBySourceRunID(context.Context, string) ([]domain.CollectorAttempt, error) {
	return nil, nil
}

type stubSourceRunReader struct{ sourceRunID string }

func (r *stubSourceRunReader) ListSourceRuns(context.Context, string) ([]domain.CollectorSourceRun, error) {
	return []domain.CollectorSourceRun{{ID: r.sourceRunID, SourceID: "baidu"}}, nil
}

type stubFetchPlugin struct{ normalized *plugin.NormalizedArticle }

func (p *stubFetchPlugin) SourceID() string    { return "baidu" }
func (p *stubFetchPlugin) DisplayName() string { return "baidu" }
func (p *stubFetchPlugin) Aliases() []string   { return nil }
func (p *stubFetchPlugin) FetchHotlist(context.Context, plugin.FetchHotlistRequest) ([]plugin.HotEntry, error) {
	return nil, nil
}
func (p *stubFetchPlugin) FetchArticle(_ context.Context, req plugin.FetchArticleRequest) (*plugin.RawArticle, error) {
	return &plugin.RawArticle{SourceID: "baidu", ExternalID: req.Entry.ExternalID, CanonicalURL: req.Entry.CanonicalURL, Body: []byte(`{"ok":true}`)}, nil
}
func (p *stubFetchPlugin) NormalizeHotEntry(any) (plugin.HotEntry, error) {
	return plugin.HotEntry{}, nil
}
func (p *stubFetchPlugin) NormalizeArticle(any) (*plugin.NormalizedArticle, error) {
	return p.normalized, nil
}
func (p *stubFetchPlugin) HealthCheck(context.Context) (plugin.SourceHealth, error) {
	return plugin.SourceHealth{SourceID: "baidu", OK: true}, nil
}
func (p *stubFetchPlugin) Capabilities() plugin.SourceCapabilities {
	return plugin.SourceCapabilities{SupportsHotlist: true, SupportsArticle: true}
}

func cloneCollectorEntry(entry *domain.CollectorEntry) *domain.CollectorEntry {
	copyValue := *entry
	copyValue.RawJSON = append([]byte(nil), entry.RawJSON...)
	copyValue.NormalizedJSON = append([]byte(nil), entry.NormalizedJSON...)
	copyValue.MetadataJSON = append([]byte(nil), entry.MetadataJSON...)
	return &copyValue
}

type detailFixturePlugin struct {
	sourceID string
	client   *httpclient.Client
}

func (p *detailFixturePlugin) SourceID() string { return p.sourceID }

func (p *detailFixturePlugin) DisplayName() string { return p.sourceID }

func (p *detailFixturePlugin) Aliases() []string { return []string{p.sourceID} }

func (p *detailFixturePlugin) FetchHotlist(ctx context.Context, _ plugin.FetchHotlistRequest) ([]plugin.HotEntry, error) {
	var path string
	switch p.sourceID {
	case "baidu":
		path = "/api/board?platform=wise&tab=realtime"
	case "github":
		path = "/search/repositories?q=stars:%3E1&sort=stars"
	default:
		return nil, fmt.Errorf("unsupported source %s", p.sourceID)
	}
	resp, err := p.client.Do(ctx, httpclient.Request{Method: http.MethodGet, Path: path})
	if err != nil {
		return nil, err
	}
	if p.sourceID == "baidu" {
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
				entries = append(entries, plugin.HotEntry{SourceID: p.sourceID, ExternalID: item.NewsID, CanonicalURL: item.URL, Title: item.Word, Summary: item.Query, Rank: &rank})
			}
		}
		return entries, nil
	}
	var payload struct {
		Items []struct {
			FullName    string `json:"full_name"`
			HTMLURL     string `json:"html_url"`
			Description string `json:"description"`
			CreatedAt   string `json:"created_at"`
			Owner       struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"items"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, err
	}
	entries := make([]plugin.HotEntry, 0, len(payload.Items))
	for idx, item := range payload.Items {
		publishedAt, err := time.Parse(time.RFC3339, item.CreatedAt)
		if err != nil {
			return nil, err
		}
		rank := idx + 1
		entries = append(entries, plugin.HotEntry{SourceID: p.sourceID, ExternalID: item.FullName, CanonicalURL: item.HTMLURL, Title: item.FullName, Summary: item.Description, Author: item.Owner.Login, PublishedAt: &publishedAt, Rank: &rank})
	}
	return entries, nil
}

func (p *detailFixturePlugin) FetchArticle(ctx context.Context, req plugin.FetchArticleRequest) (*plugin.RawArticle, error) {
	var path string
	switch p.sourceID {
	case "baidu":
		path = "/api/article/detail?id=" + req.Entry.ExternalID
	case "github":
		path = "/repos/" + req.Entry.ExternalID + "/readme"
	default:
		return nil, fmt.Errorf("unsupported source %s", p.sourceID)
	}
	resp, err := p.client.Do(ctx, httpclient.Request{Method: http.MethodGet, Path: path})
	if err != nil {
		return nil, err
	}
	return &plugin.RawArticle{SourceID: p.sourceID, ExternalID: req.Entry.ExternalID, CanonicalURL: req.Entry.CanonicalURL, Body: resp.Body, ContentType: "application/json", Metadata: map[string]any{"request_path": path}}, nil
}

func (p *detailFixturePlugin) NormalizeHotEntry(raw any) (plugin.HotEntry, error) {
	entry, ok := raw.(plugin.HotEntry)
	if !ok {
		return plugin.HotEntry{}, fmt.Errorf("unexpected raw entry %T", raw)
	}
	return entry, nil
}

func (p *detailFixturePlugin) NormalizeArticle(raw any) (*plugin.NormalizedArticle, error) {
	rawArticle, ok := raw.(*plugin.RawArticle)
	if !ok {
		return nil, fmt.Errorf("unexpected raw article %T", raw)
	}
	if p.sourceID == "baidu" {
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
		return &plugin.NormalizedArticle{SourceID: p.sourceID, ExternalID: payload.Data.ID, CanonicalURL: rawArticle.CanonicalURL, Title: payload.Data.Title, Body: payload.Data.ContentText, Summary: payload.Data.Summary, Author: payload.Data.Author, RawJSON: append([]byte(nil), rawArticle.Body...), Metadata: map[string]any{"content_text": payload.Data.ContentText}}, nil
	}
	var payload struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(rawArticle.Body, &payload); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("schema changed: missing normalized fields in github article response: %s", payload.Content)
}

func (p *detailFixturePlugin) HealthCheck(_ context.Context) (plugin.SourceHealth, error) {
	return plugin.SourceHealth{SourceID: p.sourceID, OK: true, CheckedAt: time.Now().UTC()}, nil
}

func (p *detailFixturePlugin) Capabilities() plugin.SourceCapabilities {
	return plugin.SourceCapabilities{SupportsHotlist: true, SupportsArticle: true, AuthModes: []string{domain.CollectorAuthModeNone}}
}
