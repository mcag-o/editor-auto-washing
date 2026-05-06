package service

import (
	"content-hub/domain"
	llminfra "content-hub/infra/llm"
	"content-hub/infra/memory"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type sequentialLLMClient struct {
	responses []*llminfra.GenerateResponse
	errs      []error
	gotReqs   []llminfra.GenerateRequest
	callCount int
}

func (c *sequentialLLMClient) Generate(_ context.Context, req llminfra.GenerateRequest) (*llminfra.GenerateResponse, error) {
	c.gotReqs = append(c.gotReqs, req)
	idx := c.callCount
	c.callCount++

	if idx < len(c.errs) && c.errs[idx] != nil {
		return nil, c.errs[idx]
	}
	if idx < len(c.responses) {
		return c.responses[idx], nil
	}
	return nil, errors.New("unexpected llm call")
}

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
		Content: `{"title":"Final Title","body":"Paragraph 1","template":"daily-intelligence"}`,
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
		Content: `{"title":"Final Title","body":"Paragraph 1","template":"daily-intelligence"}`,
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

func TestRewriteOrchestratorMarksRunFailedAndWorkspaceRewriteFailed(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-failed", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-failed",
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

	require.Error(t, err)
	require.NotNil(t, run)
	require.Equal(t, domain.RewriteRunFailed, run.Status)
	require.Equal(t, "generate_draft", run.CurrentStage)
	require.NotEmpty(t, run.ErrorSummary)
	require.NotNil(t, run.CompletedAt)

	storedRun, getRunErr := provider.RewritePipelineRunRepo().GetByID(t.Context(), run.ID)
	require.NoError(t, getRunErr)
	require.Equal(t, domain.RewriteRunFailed, storedRun.Status)
	require.Equal(t, "generate_draft", storedRun.CurrentStage)
	require.NotEmpty(t, storedRun.ErrorSummary)
	require.NotNil(t, storedRun.CompletedAt)

	stageRuns, stageErr := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, stageErr)
	require.Len(t, stageRuns, 1)
	require.Equal(t, domain.RewriteStageFailed, stageRuns[0].Status)
	require.Equal(t, "generate_draft", stageRuns[0].StageName)
	require.NotEmpty(t, stageRuns[0].ErrorSummary)
	require.NotNil(t, stageRuns[0].CompletedAt)

	storedWorkspace, getWorkspaceErr := provider.WorkspaceRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, getWorkspaceErr)
	require.Equal(t, domain.ArticleWorkspaceStatusRewriteFailed, storedWorkspace.Status)
	require.Equal(t, []string{
		domain.ArticleWorkspaceStatusImported,
		domain.ArticleWorkspaceStatusRewritePending,
		domain.ArticleWorkspaceStatusRewriting,
		domain.ArticleWorkspaceStatusRewriteFailed,
	}, storedWorkspace.StatusHistory)
}

func TestRewriteOrchestratorRoutesToRepairStageAndContinues(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-repair", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	promptRepo := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "rewrite_stage",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "Title: {{title}}",
	}}
	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-repair",
		TargetType:    "wechat-longform",
		SourceProfile: "sspai",
		Version:       "v1",
		Enabled:       true,
		Stages: []domain.RewriteStageDefinition{
			{
				Name:      "generate_draft",
				Type:      "generate_draft",
				PromptRef: "rewrite_stage@v1",
				Enabled:   true,
				OnFailure: domain.RewriteFailurePolicy{Action: QualityDecisionRepair, RepairStage: "repair_draft"},
			},
			{
				Name:      "repair_draft",
				Type:      "repair_draft",
				PromptRef: "rewrite_stage@v1",
				Enabled:   true,
			},
		},
	}}
	client := &sequentialLLMClient{responses: []*llminfra.GenerateResponse{
		{Response: &domain.LLMResponse{Content: `{"title":"Draft Title","body":["needs repair"],"template":"daily-intelligence"}`, Model: "mock-1"}},
		{Response: &domain.LLMResponse{Content: `{"title":"Repaired Title","body":"This repaired body is long enough.","template":"daily-intelligence"}`, Model: "mock-1"}},
	}}
	executor := NewRewriteStageExecutor(promptRepo, client, NewQualityGateEngine())
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
	require.Equal(t, "repair_draft", run.CurrentStage)

	stageRuns, stageErr := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, stageErr)
	require.Len(t, stageRuns, 2)
	require.Equal(t, []string{"generate_draft", "repair_draft"}, []string{stageRuns[0].StageName, stageRuns[1].StageName})
	require.Equal(t, domain.RewriteStageSucceeded, stageRuns[0].Status)
	require.Equal(t, domain.RewriteStageSucceeded, stageRuns[1].Status)

	draft, draftErr := provider.DraftRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, draftErr)
	require.Equal(t, "Repaired Title", draft.Headline["title"])
	require.Equal(t, []string{"This repaired body is long enough."}, draft.Headline["body"])
}

func TestRewriteOrchestratorResumeReusesExistingRunAndContinuesFromSavedState(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-resume", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	promptRepo := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "rewrite_stage",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "Title: {{title}}",
	}}
	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-resume",
		TargetType:    "wechat-longform",
		SourceProfile: "sspai",
		Version:       "v1",
		Enabled:       true,
		Stages: []domain.RewriteStageDefinition{
			{Name: "generate_draft", Type: "generate_draft", PromptRef: "rewrite_stage@v1", Enabled: true},
			{Name: "finalize", Type: "finalize", PromptRef: "rewrite_stage@v1", Enabled: true},
		},
	}}
	client := &sequentialLLMClient{responses: []*llminfra.GenerateResponse{{Response: &domain.LLMResponse{
		Content: `{"title":"Final Title","body":"Final body","template":"daily-intelligence"}`,
		Model:   "mock-1",
	}}}}
	orchestrator := NewRewriteOrchestrator(
		NewRewriteProfileRegistry(profileRepo),
		provider.RewritePipelineRunRepo(),
		provider.RewriteStageRunRepo(),
		provider.WorkspaceRepo(),
		NewRewriteStageExecutor(promptRepo, client, NewQualityGateEngine()),
		NewDraftMaterializer(provider.DraftRepo(), provider.WorkspaceRepo()),
	)

	run := domain.NewRewritePipelineRun("profile-resume", "v1", workspace.ID, "collector-1", "wechat-longform", "sspai")
	run.ID = "run-resume"
	run.Status = domain.RewriteRunRunning
	run.CurrentStage = "finalize"
	run.Metadata = map[string]any{"title": "Source"}
	require.NoError(t, provider.RewritePipelineRunRepo().Create(t.Context(), run))

	now := time.Now().UTC()
	require.NoError(t, provider.RewriteStageRunRepo().Create(t.Context(), &domain.RewriteStageRun{
		ID:            "stage-1",
		PipelineRunID: run.ID,
		StageName:     "generate_draft",
		StageType:     "generate_draft",
		Status:        domain.RewriteStageSucceeded,
		Attempt:       1,
		OutputJSON:    `{"title":"Draft Title","body":"Draft body","template":"daily-intelligence"}`,
		StartedAt:     now,
		CompletedAt:   &now,
	}))

	resumed, err := orchestrator.Resume(t.Context(), run.ID, "Source")

	require.NoError(t, err)
	require.Equal(t, run.ID, resumed.ID)
	require.Equal(t, domain.RewriteRunSucceeded, resumed.Status)
	require.Equal(t, "finalize", resumed.CurrentStage)
	require.NotEmpty(t, resumed.FinalDraftID)
	require.Equal(t, 1, client.callCount)

	runs, listErr := provider.RewritePipelineRunRepo().List(t.Context(), 10)
	require.NoError(t, listErr)
	require.Len(t, runs, 1)
	require.Equal(t, run.ID, runs[0].ID)

	stageRuns, stageErr := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, stageErr)
	require.Len(t, stageRuns, 2)
	require.Equal(t, []string{"generate_draft", "finalize"}, []string{stageRuns[0].StageName, stageRuns[1].StageName})

	draft, draftErr := provider.DraftRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, draftErr)
	require.Equal(t, "Final Title", draft.Headline["title"])
	require.Equal(t, []string{"Final body"}, draft.Headline["body"])
}

func TestRewriteOrchestratorDoesNotRepairWithoutRepairPolicyAction(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-no-repair-policy", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	promptRepo := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "rewrite_stage",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "Title: {{title}}",
	}}
	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-no-repair-policy",
		TargetType:    "wechat-longform",
		SourceProfile: "sspai",
		Version:       "v1",
		Enabled:       true,
		Stages: []domain.RewriteStageDefinition{
			{
				Name:      "generate_draft",
				Type:      "generate_draft",
				PromptRef: "rewrite_stage@v1",
				Enabled:   true,
				OnFailure: domain.RewriteFailurePolicy{Action: QualityDecisionFail, RepairStage: "repair_draft"},
			},
			{
				Name:      "repair_draft",
				Type:      "repair_draft",
				PromptRef: "rewrite_stage@v1",
				Enabled:   true,
			},
		},
	}}
	client := &sequentialLLMClient{responses: []*llminfra.GenerateResponse{
		{Response: &domain.LLMResponse{Content: `{"title":"Draft Title","body":["needs repair"],"template":"daily-intelligence"}`, Model: "mock-1"}},
	}}
	orchestrator := NewRewriteOrchestrator(
		NewRewriteProfileRegistry(profileRepo),
		provider.RewritePipelineRunRepo(),
		provider.RewriteStageRunRepo(),
		provider.WorkspaceRepo(),
		NewRewriteStageExecutor(promptRepo, client, NewQualityGateEngine()),
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

	require.Error(t, err)
	require.NotNil(t, run)
	require.Equal(t, domain.RewriteRunFailed, run.Status)
	require.Equal(t, "generate_draft", run.CurrentStage)
	require.Len(t, client.gotReqs, 1)

	stageRuns, stageErr := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, stageErr)
	require.Len(t, stageRuns, 2)
	require.Equal(t, "generate_draft", stageRuns[0].StageName)
	require.Equal(t, domain.RewriteStageSucceeded, stageRuns[0].Status)
	require.Equal(t, "generate_draft", stageRuns[1].StageName)
	require.Equal(t, domain.RewriteStageFailed, stageRuns[1].Status)

	storedWorkspace, getWorkspaceErr := provider.WorkspaceRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, getWorkspaceErr)
	require.Equal(t, domain.ArticleWorkspaceStatusRewriteFailed, storedWorkspace.Status)
}

func TestRewriteOrchestratorFailsWhenRepairResultStillRequestsRepair(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-repair-fails", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	promptRepo := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "rewrite_stage",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "Title: {{title}}",
	}}
	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-repair-fails",
		TargetType:    "wechat-longform",
		SourceProfile: "sspai",
		Version:       "v1",
		Enabled:       true,
		Stages: []domain.RewriteStageDefinition{
			{
				Name:      "generate_draft",
				Type:      "generate_draft",
				PromptRef: "rewrite_stage@v1",
				Enabled:   true,
				OnFailure: domain.RewriteFailurePolicy{Action: QualityDecisionRepair, RepairStage: "repair_draft"},
			},
			{
				Name:      "repair_draft",
				Type:      "repair_draft",
				PromptRef: "rewrite_stage@v1",
				Enabled:   true,
			},
		},
	}}
	client := &sequentialLLMClient{responses: []*llminfra.GenerateResponse{
		{Response: &domain.LLMResponse{Content: `{"title":"Draft Title","body":["needs repair"],"template":"daily-intelligence"}`, Model: "mock-1"}},
		{Response: &domain.LLMResponse{Content: `{"title":"Still Broken","body":["still broken"],"template":"daily-intelligence"}`, Model: "mock-1"}},
	}}
	orchestrator := NewRewriteOrchestrator(
		NewRewriteProfileRegistry(profileRepo),
		provider.RewritePipelineRunRepo(),
		provider.RewriteStageRunRepo(),
		provider.WorkspaceRepo(),
		NewRewriteStageExecutor(promptRepo, client, NewQualityGateEngine()),
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

	require.Error(t, err)
	require.NotNil(t, run)
	require.Equal(t, domain.RewriteRunFailed, run.Status)
	require.Equal(t, "repair_draft", run.CurrentStage)
	require.Len(t, client.gotReqs, 2)

	stageRuns, stageErr := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, stageErr)
	require.Len(t, stageRuns, 3)
	require.Equal(t, []string{"generate_draft", "repair_draft", "repair_draft"}, []string{stageRuns[0].StageName, stageRuns[1].StageName, stageRuns[2].StageName})
	require.Equal(t, domain.RewriteStageSucceeded, stageRuns[0].Status)
	require.Equal(t, domain.RewriteStageSucceeded, stageRuns[1].Status)
	require.Equal(t, domain.RewriteStageFailed, stageRuns[2].Status)

	storedWorkspace, getWorkspaceErr := provider.WorkspaceRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, getWorkspaceErr)
	require.Equal(t, domain.ArticleWorkspaceStatusRewriteFailed, storedWorkspace.Status)
}
