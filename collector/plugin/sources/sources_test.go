package sources_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
	"content-hub/collector/plugin/sources"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaiduPlugin_FetchHotlistNormalizesEntries(t *testing.T) {
	entries := fetchHotlistEntries(t, "baidu-hotlist.json", sources.NewBaiduWithClient)

	require.Len(t, entries, 2)
	assert.Equal(t, "baidu", entries[0].SourceID)
	assert.Equal(t, "7492239302142358563", entries[0].ExternalID)
	assert.Equal(t, "SpaceX completes third orbital refueling test", entries[0].Title)
	assert.Equal(t, "https://top.baidu.com/board?tab=realtime", entries[0].CanonicalURL)
	if assert.NotNil(t, entries[0].Rank) {
		assert.Equal(t, 1, *entries[0].Rank)
	}
}

func TestBilibiliPlugin_FetchHotlistNormalizesEntries(t *testing.T) {
	entries := fetchHotlistEntries(t, "bilibili-hotlist.json", sources.NewBilibiliWithClient)

	require.Len(t, entries, 2)
	assert.Equal(t, "bilibili", entries[0].SourceID)
	assert.Equal(t, "987654", entries[0].ExternalID)
	assert.Equal(t, "Annual animation festival premieres surprise short", entries[0].Title)
	assert.Equal(t, "https://www.bilibili.com/video/BV1xx411c7mD", entries[0].CanonicalURL)
	if assert.NotNil(t, entries[0].Rank) {
		assert.Equal(t, 1, *entries[0].Rank)
	}
}

func TestGitHubPlugin_FetchHotlistNormalizesEntries(t *testing.T) {
	entries := fetchHotlistEntries(t, "github-hotlist.json", sources.NewGitHubWithClient)

	require.Len(t, entries, 2)
	assert.Equal(t, "github", entries[0].SourceID)
	assert.Equal(t, "openai/gpt-oss", entries[0].ExternalID)
	assert.Equal(t, "openai/gpt-oss", entries[0].Title)
	assert.Equal(t, "https://github.com/openai/gpt-oss", entries[0].CanonicalURL)
	assert.Equal(t, "Open foundation model release", entries[0].Summary)
	assert.Equal(t, "openai", entries[0].Author)
	if assert.NotNil(t, entries[0].Rank) {
		assert.Equal(t, 1, *entries[0].Rank)
	}
	if assert.NotNil(t, entries[0].PublishedAt) {
		assert.Equal(t, time.Date(2026, 4, 9, 7, 0, 0, 0, time.UTC), *entries[0].PublishedAt)
	}
}

func TestStackOverflowPlugin_FetchHotlistNormalizesEntries(t *testing.T) {
	entries := fetchHotlistEntries(t, "stackoverflow-hotlist.json", sources.NewStackOverflowWithClient)

	require.Len(t, entries, 2)
	assert.Equal(t, "stackoverflow", entries[0].SourceID)
	assert.Equal(t, "81234567", entries[0].ExternalID)
	assert.Equal(t, "Why does Go range capture the wrong loop variable?", entries[0].Title)
	assert.Equal(t, "https://stackoverflow.com/questions/81234567", entries[0].CanonicalURL)
	assert.Equal(t, "Ava", entries[0].Author)
	if assert.NotNil(t, entries[0].Rank) {
		assert.Equal(t, 1, *entries[0].Rank)
	}
}

func TestPluginHealthCheckUsesHotlistEndpoint(t *testing.T) {
	pluginUnderTest := newPluginWithFixture(t, "github-hotlist.json", sources.NewGitHubWithClient)

	health, err := pluginUnderTest.HealthCheck(t.Context())

	require.NoError(t, err)
	assert.True(t, health.OK)
	assert.Equal(t, "github", health.SourceID)
	assert.NotZero(t, health.CheckedAt)
	assert.Empty(t, health.Message)
}

func TestBaiduPlugin_FetchArticleNormalizesDetail(t *testing.T) {
	pluginUnderTest := newPluginWithFixtures(t, map[string]string{
		"/api/board?platform=wise&tab=realtime":      "baidu-hotlist.json",
		"/api/article/detail?id=7492239302142358563": "baidu-article.json",
	}, sources.NewBaiduWithClient)

	rawArticle, err := pluginUnderTest.FetchArticle(t.Context(), plugin.FetchArticleRequest{Entry: plugin.HotEntry{
		SourceID:     "baidu",
		ExternalID:   "7492239302142358563",
		CanonicalURL: "https://top.baidu.com/board?tab=realtime",
		Title:        "SpaceX completes third orbital refueling test",
	}})
	require.NoError(t, err)

	normalized, err := pluginUnderTest.NormalizeArticle(rawArticle)
	require.NoError(t, err)
	assert.Equal(t, "baidu", normalized.SourceID)
	assert.Equal(t, "7492239302142358563", normalized.ExternalID)
	assert.Equal(t, "SpaceX completes third orbital refueling test", normalized.Title)
	assert.Equal(t, "Mission milestone summary", normalized.Summary)
	assert.Equal(t, "Baidu News", normalized.Author)
	assert.Contains(t, normalized.Body, "third orbital refueling test")
	assert.NotEmpty(t, normalized.RawJSON)
	assert.True(t, pluginUnderTest.Capabilities().SupportsArticle)
}

func TestGitHubPlugin_FetchArticleNormalizesDetail(t *testing.T) {
	pluginUnderTest := newPluginWithFixtures(t, map[string]string{
		"/search/repositories?q=stars:%3E1&sort=stars": "github-hotlist.json",
		"/repos/openai/gpt-oss/readme":                 "github-article.json",
	}, sources.NewGitHubWithClient)

	rawArticle, err := pluginUnderTest.FetchArticle(t.Context(), plugin.FetchArticleRequest{Entry: plugin.HotEntry{
		SourceID:     "github",
		ExternalID:   "openai/gpt-oss",
		CanonicalURL: "https://github.com/openai/gpt-oss",
		Title:        "openai/gpt-oss",
		Summary:      "Open foundation model release",
		Author:       "openai",
	}})
	require.NoError(t, err)

	normalized, err := pluginUnderTest.NormalizeArticle(rawArticle)
	require.NoError(t, err)
	assert.Equal(t, "github", normalized.SourceID)
	assert.Equal(t, "openai/gpt-oss", normalized.ExternalID)
	assert.Equal(t, "gpt-oss README", normalized.Title)
	assert.Equal(t, "Open foundation model release", normalized.Summary)
	assert.Equal(t, "openai", normalized.Author)
	assert.Contains(t, normalized.Body, "open-weight model")
	assert.NotEmpty(t, normalized.RawJSON)
	assert.True(t, pluginUnderTest.Capabilities().SupportsArticle)
}

func fetchHotlistEntries(t *testing.T, fixture string, builder func(*httpclient.Client) plugin.SourcePlugin) []plugin.HotEntry {
	t.Helper()
	pluginUnderTest := newPluginWithFixture(t, fixture, builder)

	entries, err := pluginUnderTest.FetchHotlist(t.Context(), plugin.FetchHotlistRequest{})
	require.NoError(t, err)

	return entries
}

func newPluginWithFixture(t *testing.T, fixture string, builder func(*httpclient.Client) plugin.SourcePlugin) plugin.SourcePlugin {
	t.Helper()
	client := fakeHTTPClientFromFixture(t, fixture)
	return builder(client)
}

func newPluginWithFixtures(t *testing.T, fixtures map[string]string, builder func(*httpclient.Client) plugin.SourcePlugin) plugin.SourcePlugin {
	t.Helper()
	client := fakeHTTPClientFromFixtures(t, fixtures)
	return builder(client)
}

func fakeHTTPClientFromFixture(t *testing.T, fixture string) *httpclient.Client {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "collector", "fixtures", fixture))
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.New(httpclient.Options{BaseURL: server.URL})
	require.NoError(t, err)
	return client
}

func fakeHTTPClientFromFixtures(t *testing.T, fixtures map[string]string) *httpclient.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture, ok := fixtures[r.URL.RequestURI()]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "collector", "fixtures", fixture))
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.New(httpclient.Options{BaseURL: server.URL})
	require.NoError(t, err)
	return client
}

func TestUnsupportedArticleFetch(t *testing.T) {
	pluginUnderTest := newPluginWithFixture(t, "bilibili-hotlist.json", sources.NewBilibiliWithClient)

	_, err := pluginUnderTest.FetchArticle(context.Background(), plugin.FetchArticleRequest{})

	require.Error(t, err)
	assert.ErrorContains(t, err, "not supported")
}

func TestSourceConstructors_ReturnErrorForInvalidConfig(t *testing.T) {
	pluginUnderTest, err := sources.NewGitHubWithOptions(httpclient.Options{BaseURL: "://bad-url"})

	require.Error(t, err)
	assert.Nil(t, pluginUnderTest)
	assert.ErrorContains(t, err, "base url")
}

func TestHTMLSource_FetchHotlistParsesFixture(t *testing.T) {
	pluginUnderTest := newPluginWithHTMLFixtures(t, map[string]string{
		"/?tab=hot": "v2ex-hotlist.html",
		"/t/12345":  "v2ex-article.html",
	}, sources.NewV2EXWithClient)

	entries, err := pluginUnderTest.FetchHotlist(t.Context(), plugin.FetchHotlistRequest{})

	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "v2ex", entries[0].SourceID)
	assert.Equal(t, "12345", entries[0].ExternalID)
	assert.Equal(t, "Why Go 1.25 loop semantics matter in collectors", entries[0].Title)
	assert.Equal(t, "https://www.v2ex.com/t/12345", entries[0].CanonicalURL)
	assert.Contains(t, entries[0].Summary, "Go")
	assert.True(t, pluginUnderTest.Capabilities().SupportsArticle)
}

func TestHTMLSource_FetchArticleNormalizesFixture(t *testing.T) {
	pluginUnderTest := newPluginWithHTMLFixtures(t, map[string]string{
		"/?tab=hot": "v2ex-hotlist.html",
		"/t/12345":  "v2ex-article.html",
	}, sources.NewV2EXWithClient)

	rawArticle, err := pluginUnderTest.FetchArticle(t.Context(), plugin.FetchArticleRequest{Entry: plugin.HotEntry{
		SourceID:     "v2ex",
		ExternalID:   "12345",
		CanonicalURL: "https://www.v2ex.com/t/12345",
		Title:        "Why Go 1.25 loop semantics matter in collectors",
	}})
	require.NoError(t, err)

	normalized, err := pluginUnderTest.NormalizeArticle(rawArticle)
	require.NoError(t, err)
	assert.Equal(t, "v2ex", normalized.SourceID)
	assert.Equal(t, "12345", normalized.ExternalID)
	assert.Equal(t, "Why Go 1.25 loop semantics matter in collectors", normalized.Title)
	assert.Equal(t, "alex", normalized.Author)
	assert.Contains(t, normalized.Body, "worker pool")
	assert.NotEmpty(t, normalized.RawJSON)
}

func TestCookieSource_HealthCheckDoesNotAssumePluginLocalAuthState(t *testing.T) {
	pluginUnderTest := newCookieSourceWithFixture(t, http.StatusOK, "weibo-auth-failure.json", "SUB=collector-cookie", "")

	health, err := pluginUnderTest.HealthCheck(t.Context())

	require.Error(t, err)
	assert.False(t, health.OK)
	assert.Equal(t, "unavailable", health.Code)
}

func TestCookieSource_FetchHotlistDoesNotInjectCookieWithoutRuntimeAuth(t *testing.T) {
	pluginUnderTest := newCookieSourceWithFixture(t, http.StatusOK, "weibo-hotlist.json", "SUB=collector-cookie", "")

	entries, err := pluginUnderTest.FetchHotlist(t.Context(), plugin.FetchHotlistRequest{})

	require.Error(t, err)
	assert.Nil(t, entries)
	assert.ErrorContains(t, err, "fetch hotlist for weibo")
}

func TestCookieSource_FetchHotlistUsesRuntimeProvidedCookieHeader(t *testing.T) {
	pluginUnderTest := newCookieSourceWithFixture(t, http.StatusOK, "weibo-hotlist.json", "SUB=collector-cookie", "")

	entries, err := pluginUnderTest.FetchHotlist(t.Context(), plugin.FetchHotlistRequest{Headers: map[string]string{"Cookie": "SUB=collector-cookie"}})

	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "weibo", entries[0].SourceID)
	assert.Equal(t, "Mars colony launch date set", entries[0].Title)
	assert.Equal(t, "https://s.weibo.com/weibo?q=%23Mars+colony+launch+date+set%23", entries[0].CanonicalURL)
}

func TestCookieSource_HealthCheckUsesRuntimeProvidedCookieHeader(t *testing.T) {
	pluginUnderTest := newCookieSourceWithFixture(t, http.StatusOK, "weibo-hotlist.json", "SUB=collector-cookie", "SUB=collector-cookie")

	health, err := pluginUnderTest.HealthCheck(t.Context())

	require.NoError(t, err)
	assert.True(t, health.OK)
	assert.Equal(t, "healthy", health.Code)
}

func newPluginWithHTMLFixtures(t *testing.T, fixtures map[string]string, builder func(*httpclient.Client) plugin.SourcePlugin) plugin.SourcePlugin {
	t.Helper()
	client := fakeHTTPClientFromMixedFixtures(t, fixtures)
	return builder(client)
}

func newCookieSourceWithFixture(t *testing.T, statusCode int, fixture string, requiredCookie string, runtimeCookie string) plugin.SourcePlugin {
	t.Helper()
	client := fakeCookieHTTPClientFromFixture(t, statusCode, fixture, requiredCookie, runtimeCookie)
	return sources.NewWeiboWithClient(client)
}

func fakeCookieHTTPClientFromFixture(t *testing.T, statusCode int, fixture string, requiredCookie string, runtimeCookie string) *httpclient.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requiredCookie != "" && r.Header.Get("Cookie") != requiredCookie {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, err := readCollectorFixture(t, fixture)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	options := httpclient.Options{BaseURL: server.URL}
	if runtimeCookie != "" {
		options.AuthInjector = httpclient.HeaderAuthInjector(map[string]string{"Cookie": runtimeCookie})
	}
	client, err := httpclient.New(options)
	require.NoError(t, err)
	return client
}

func fakeHTTPClientFromMixedFixtures(t *testing.T, fixtures map[string]string) *httpclient.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture, ok := fixtures[r.URL.RequestURI()]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := readCollectorFixture(t, fixture)
		require.NoError(t, err)
		if filepath.Ext(fixture) == ".html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.New(httpclient.Options{BaseURL: server.URL})
	require.NoError(t, err)
	return client
}

func readCollectorFixture(t *testing.T, fixture string) ([]byte, error) {
	t.Helper()
	baseDir := filepath.Join("..", "..", "..", "testdata", "collector")
	if filepath.Ext(fixture) == ".html" {
		return os.ReadFile(filepath.Join(baseDir, "html", fixture))
	}
	return os.ReadFile(filepath.Join(baseDir, "fixtures", fixture))
}
