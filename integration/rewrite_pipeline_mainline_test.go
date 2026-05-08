package integration

import (
	"content-hub/domain"
	llminfra "content-hub/infra/llm"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRewritePipelineMainlineMaterializesDraft(t *testing.T) {
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
		Content:      `{"title":"Rewritten Title","body":"Mainline draft body.","template":"daily-intelligence"}`,
		Model:        "static-integration-model",
		FinishReason: "stop",
	}}

	workspace := domain.NewArticleWorkspaceRecord(
		"article-1",
		"Source Title",
		"Source Summary",
		domain.ArticleWorkspaceSource{
			SourceType: "collector",
			Platform:   "sspai",
			URL:        "https://example.com/articles/source-title",
		},
		nil,
	)
	require.NoError(t, repos.WorkspaceRepo.Create(t.Context(), workspace))

	require.NoError(t, repos.RewritePipelineProfileRepo.Upsert(t.Context(), &domain.RewritePipelineProfile{
		ID:                    "profile-1",
		Name:                  "Rewrite Mainline",
		TargetType:            "wechat-longform",
		SourceProfile:         "sspai",
		Version:               "v1",
		Description:           "Mainline rewrite profile",
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
		SystemTemplate: "You rewrite imported articles into drafts.",
		UserTemplate:   "Rewrite {{title}} into a polished draft.",
		Description:    "Integration rewrite prompt",
	}))

	require.NoError(t, repos.LLMProfileRepo.Upsert(t.Context(), &domain.LLMProfile{
		Name:        "rewrite-default",
		Provider:    "openai",
		Model:       "static-integration-model",
		Temperature: 0.2,
		MaxTokens:   512,
		TimeoutSec:  30,
	}))

	runtime, err := service.BuildRewriteRuntime(repos)
	require.NoError(t, err)

	run, err := runtime.Orchestrator().Run(t.Context(), service.RewriteRunRequest{
		WorkspaceArticleID: workspace.ID,
		CollectorArticleID: "collector-1",
		Title:              workspace.Title,
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "v1",
	})

	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, run.Status)
	require.Equal(t, workspace.ID, run.FinalDraftID)
	require.Equal(t, serviceRewriteMaterializeNodeIDForTest(), run.CurrentStage)
	require.NotContains(t, run.Metadata, "active_token_set")
	require.NotContains(t, run.Metadata, "rewrite_workflow_checkpoint")
	routeSummary, ok := run.Metadata["workflow_route_latest"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, serviceRewriteMaterializeNodeIDForTest(), routeSummary["node_id"])
	require.Equal(t, "no_match", routeSummary["outcome"])
	require.Empty(t, routeSummary["evaluation_trace"])

	storedWorkspace, err := repos.WorkspaceRepo.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ArticleWorkspaceStatusDraft, storedWorkspace.Status)

	storedRun, err := repos.RewritePipelineRunRepo.GetByID(t.Context(), run.ID)
	require.NoError(t, err)
	require.NotContains(t, storedRun.Metadata, "active_token_set")
	require.NotContains(t, storedRun.Metadata, "rewrite_workflow_checkpoint")
	storedRouteSummary, ok := storedRun.Metadata["workflow_route_latest"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, serviceRewriteMaterializeNodeIDForTest(), storedRouteSummary["node_id"])
	require.Equal(t, "no_match", storedRouteSummary["outcome"])
	require.Empty(t, storedRouteSummary["evaluation_trace"])

	draft, err := repos.DraftRepo.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, "daily-intelligence", draft.Template)
	require.Equal(t, "Rewritten Title", draft.Headline["title"])
	require.Equal(t, []string{"Mainline draft body."}, domain.DraftParagraphs(draft.Headline["body"]))

	stageRuns, err := repos.RewriteStageRunRepo.ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Len(t, stageRuns, 1)
	require.Equal(t, domain.RewriteStageSucceeded, stageRuns[0].Status)
	require.Equal(t, "rewrite-default", stageRuns[0].LLMProfileRef)
}

func serviceRewriteMaterializeNodeIDForTest() string {
	return "materialize_draft"
}
