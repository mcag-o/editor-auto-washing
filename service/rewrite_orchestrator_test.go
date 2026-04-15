package service

import (
	"content-hub/domain"
	llminfra "content-hub/infra/llm"
	"content-hub/infra/memory"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRewriteOrchestratorRunsPipelineAndCreatesDraft(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-1", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	promptRepo := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "generate_draft",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "Title: {{title}}",
	}}
	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-1",
		TargetType:    "wechat-longform",
		SourceProfile: "sspai",
		Version:       "v1",
		Enabled:       true,
		Stages: []domain.RewriteStageDefinition{{
			Name:      "generate_draft",
			Type:      "generate_draft",
			PromptRef: "generate_draft@v1",
			Enabled:   true,
		}},
	}}
	executor := NewRewriteStageExecutor(promptRepo, &recordingLLMClient{response: &llminfra.GenerateResponse{Response: &domain.LLMResponse{
		Content: `{"title":"Final Title","body":["Paragraph 1"],"template":"daily-intelligence"}`,
		Model:   "mock-1",
	}}}, NewQualityGateEngine())
	materializer := NewDraftMaterializer(provider.DraftRepo(), provider.WorkspaceRepo())
	orchestrator := NewRewriteOrchestrator(
		NewRewriteProfileRegistry(profileRepo),
		provider.RewritePipelineRunRepo(),
		provider.RewriteStageRunRepo(),
		provider.WorkspaceRepo(),
		executor,
		materializer,
	)

	run, err := orchestrator.Run(t.Context(), RewriteRunRequest{
		WorkspaceArticleID: workspace.ID,
		CollectorArticleID: "collector-1",
		Title:              "Source",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "v1",
	})

	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, run.Status)
	require.NotEmpty(t, run.FinalDraftID)
	require.Equal(t, "generate_draft", run.CurrentStage)
	require.NotNil(t, run.CompletedAt)

	storedRun, err := provider.RewritePipelineRunRepo().GetByID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, storedRun.Status)
	require.Equal(t, workspace.ID, storedRun.FinalDraftID)

	stageRuns, err := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Len(t, stageRuns, 1)
	require.Equal(t, domain.RewriteStageSucceeded, stageRuns[0].Status)
	require.Equal(t, "generate_draft", stageRuns[0].StageName)
	require.Contains(t, stageRuns[0].InputJSON, "Source")
	require.Contains(t, stageRuns[0].OutputJSON, "Final Title")
	require.NotNil(t, stageRuns[0].CompletedAt)

	draft, err := provider.DraftRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, "daily-intelligence", draft.Template)
	require.Equal(t, "Final Title", draft.Headline["title"])
	require.Equal(t, []string{"Paragraph 1"}, draft.Headline["body"])

	storedWorkspace, err := provider.WorkspaceRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ArticleWorkspaceStatusDraft, storedWorkspace.Status)
	require.Equal(t, []string{
		domain.ArticleWorkspaceStatusImported,
		domain.ArticleWorkspaceStatusRewritePending,
		domain.ArticleWorkspaceStatusRewriting,
		domain.ArticleWorkspaceStatusDraft,
	}, storedWorkspace.StatusHistory)
	require.Equal(t, draftMaterializedNote, storedWorkspace.Notes)
}

func TestRewriteOrchestratorRejectsDisabledProfile(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-disabled", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-disabled",
		TargetType:    "wechat-longform",
		SourceProfile: "sspai",
		Version:       "v1",
		Enabled:       false,
	}}
	orchestrator := NewRewriteOrchestrator(
		NewRewriteProfileRegistry(profileRepo),
		provider.RewritePipelineRunRepo(),
		provider.RewriteStageRunRepo(),
		provider.WorkspaceRepo(),
		NewRewriteStageExecutor(&stubRewritePromptRegistry{}, &recordingLLMClient{}, NewQualityGateEngine()),
		NewDraftMaterializer(provider.DraftRepo(), provider.WorkspaceRepo()),
	)

	run, err := orchestrator.Run(t.Context(), RewriteRunRequest{
		WorkspaceArticleID: workspace.ID,
		CollectorArticleID: "collector-1",
		Title:              "Source",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "v1",
	})

	require.Nil(t, run)
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrValidation, appErr.Code)
	require.Contains(t, appErr.Message, "disabled")

	runs, listErr := provider.RewritePipelineRunRepo().List(t.Context(), 10)
	require.NoError(t, listErr)
	require.Empty(t, runs)

	storedWorkspace, getErr := provider.WorkspaceRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, getErr)
	require.Equal(t, domain.ArticleWorkspaceStatusImported, storedWorkspace.Status)
	require.Equal(t, []string{domain.ArticleWorkspaceStatusImported}, storedWorkspace.StatusHistory)
}

func TestRewriteOrchestratorUsesProfileDefaultLLMProfileWhenStageIsUnset(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-default-llm", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	promptRepo := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "generate_draft",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "Title: {{title}}",
	}}
	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:                "profile-default-llm",
		TargetType:        "wechat-longform",
		SourceProfile:     "sspai",
		Version:           "v1",
		Enabled:           true,
		DefaultLLMProfile: "rewrite-default",
		Stages: []domain.RewriteStageDefinition{{
			Name:      "generate_draft",
			Type:      "generate_draft",
			PromptRef: "generate_draft@v1",
			Enabled:   true,
		}},
	}}
	profiles := &stubLLMProfileResolver{profile: &domain.LLMProfile{
		Name:        "rewrite-default",
		Model:       "gpt-4.1-mini-real",
		Temperature: 0.4,
		MaxTokens:   512,
	}}
	client := &recordingLLMClient{response: &llminfra.GenerateResponse{Response: &domain.LLMResponse{
		Content: `{"title":"Final Title","body":["Paragraph 1"],"template":"daily-intelligence"}`,
		Model:   "gpt-4.1-mini-real",
	}}}
	executor := NewRewriteStageExecutorWithProfileResolver(promptRepo, profiles, client, NewQualityGateEngine())
	orchestrator := NewRewriteOrchestrator(
		NewRewriteProfileRegistry(profileRepo),
		provider.RewritePipelineRunRepo(),
		provider.RewriteStageRunRepo(),
		provider.WorkspaceRepo(),
		executor,
		NewDraftMaterializer(provider.DraftRepo(), provider.WorkspaceRepo()),
	)

	run, err := orchestrator.Run(t.Context(), RewriteRunRequest{
		WorkspaceArticleID: workspace.ID,
		CollectorArticleID: "collector-1",
		Title:              "Source",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "v1",
	})

	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, run.Status)
	require.Equal(t, "rewrite-default", profiles.gotName)
	require.Equal(t, "gpt-4.1-mini-real", client.gotReq.Options.Model)

	stageRuns, stageErr := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, stageErr)
	require.Len(t, stageRuns, 1)
	require.Equal(t, "rewrite-default", stageRuns[0].LLMProfileRef)
}
