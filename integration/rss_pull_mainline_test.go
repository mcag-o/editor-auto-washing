package integration

import (
	"content-hub/domain"
	llminfra "content-hub/infra/llm"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type staticRSSFeedFetcher struct {
	body []byte
}

func (f staticRSSFeedFetcher) Fetch(context.Context, string) ([]byte, error) {
	return append([]byte(nil), f.body...), nil
}

func TestRSSPullMainlineCreatesWorkspaceAndDraft(t *testing.T) {
	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	repos, cleanup, err := service.BuildRuntimeRepos(root)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, cleanup())
	}()

	repos.LLMClient = llminfra.StaticClient{Response: domain.LLMResponse{
		Content:      `{"title":"RSS Rewritten Title","body":"RSS mainline draft body.","template":"daily-intelligence"}`,
		Model:        "static-rss-integration-model",
		FinishReason: "stop",
	}}
	repos.RSSFeedFetcher = staticRSSFeedFetcher{body: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Tech Feed</title>
    <item>
      <guid>rss-guid-1</guid>
      <title>RSS Mainline Title</title>
      <link>https://example.com/articles/rss-mainline</link>
      <description>Original RSS body.</description>
    </item>
  </channel>
</rss>`)}

	rssRuntime, err := service.BuildRSSRuntime(repos)
	require.NoError(t, err)

	_, err = service.BuildRewriteRuntime(repos)
	require.NoError(t, err)

	require.NoError(t, repos.RewritePipelineProfileRepo.Upsert(t.Context(), &domain.RewritePipelineProfile{
		ID:                    "rss-profile-1",
		Name:                  "RSS Mainline",
		TargetType:            "wechat-longform",
		SourceProfile:         "sspai",
		Version:               "v1",
		Description:           "RSS rewrite profile",
		DefaultLLMProfile:     "rewrite-default",
		MaterializationPolicy: "workspace-draft",
		Enabled:               true,
		Stages: []domain.RewriteStageDefinition{{
			Name:      "generate_draft",
			Type:      "generate_draft",
			PromptRef: "generate_draft@v1",
			Enabled:   true,
		}},
	}))

	require.NoError(t, repos.PromptTemplateRepo.Upsert(t.Context(), &domain.PromptTemplate{
		Key:            "generate_draft",
		Version:        "v1",
		SystemTemplate: "You rewrite imported rss articles into drafts.",
		UserTemplate:   "Rewrite {{title}} into a polished draft.",
		Description:    "Integration rss rewrite prompt",
	}))

	require.NoError(t, repos.LLMProfileRepo.Upsert(t.Context(), &domain.LLMProfile{
		Name:        "rewrite-default",
		Provider:    "openai",
		Model:       "static-rss-integration-model",
		Temperature: 0.2,
		MaxTokens:   512,
		TimeoutSec:  30,
	}))

	sub, err := rssRuntime.SubscriptionService.Create(t.Context(), domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai"))
	require.NoError(t, err)
	sub.RewriteProfileVersion = "v1"
	require.NoError(t, repos.RSSSubscriptionRepo.Update(t.Context(), sub))

	result, err := rssRuntime.Scheduler.RunByID(t.Context(), sub.ID)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Run)
	require.Equal(t, domain.RSSPullRunStatusSucceeded, result.Run.Status)
	require.Equal(t, 1, result.FetchedItems)
	require.Equal(t, 1, result.ImportedItems)
	require.Equal(t, 0, result.FailedItems)

	pullRun, err := repos.RSSPullRunRepo.GetByID(t.Context(), result.Run.ID)
	require.NoError(t, err)
	require.Equal(t, domain.RSSPullRunStatusSucceeded, pullRun.Status)

	items, err := repos.RSSItemRepo.List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, domain.RSSItemStatusImported, items[0].Status)
	require.NotEmpty(t, items[0].WorkspaceArticleID)
	require.Equal(t, sub.ID, items[0].SubscriptionID)
	require.Equal(t, "https://example.com/articles/rss-mainline", items[0].Link)

	workspace, err := repos.WorkspaceRepo.GetByID(t.Context(), items[0].WorkspaceArticleID)
	require.NoError(t, err)
	require.Equal(t, domain.ArticleWorkspaceStatusDraft, workspace.Status)
	require.Equal(t, "RSS Mainline Title", workspace.Title)

	draft, err := repos.DraftRepo.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, "daily-intelligence", draft.Template)
	require.Equal(t, "RSS Rewritten Title", draft.Headline["title"])
	require.Equal(t, []string{"RSS mainline draft body."}, domain.DraftParagraphs(draft.Headline["body"]))

	runs, err := repos.RewritePipelineRunRepo.List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, domain.RewriteRunSucceeded, runs[0].Status)
	require.Equal(t, workspace.ID, runs[0].WorkspaceArticleID)

	stageRuns, err := repos.RewriteStageRunRepo.ListByPipelineRunID(t.Context(), runs[0].ID)
	require.NoError(t, err)
	require.Len(t, stageRuns, 1)
	require.Equal(t, domain.RewriteStageSucceeded, stageRuns[0].Status)
	require.Equal(t, "rewrite-default", stageRuns[0].LLMProfileRef)
}
