package service

import (
	"content-hub/domain"
	llminfra "content-hub/infra/llm"
	"content-hub/infra/memory"
	"context"
	"encoding/json"
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

type multiPromptRegistry struct {
	prompts    map[string]*domain.PromptTemplate
	gotKey     string
	gotVersion string
}

func (r *multiPromptRegistry) Get(_ context.Context, key, version string) (*domain.PromptTemplate, error) {
	r.gotKey = key
	r.gotVersion = version
	if r.prompts == nil {
		return nil, domain.NewNotFoundErr("prompt template", key+"@"+version)
	}
	prompt, ok := r.prompts[key+"@"+version]
	if !ok {
		return nil, domain.NewNotFoundErr("prompt template", key+"@"+version)
	}
	return prompt, nil
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
		Title:              "Source",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "v1",
	})

	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, run.Status)
	require.NotEmpty(t, run.FinalDraftID)
	require.Equal(t, rewriteWorkflowMaterializeNodeID, run.CurrentStage)
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

func TestRewriteOrchestratorRunsPipelineForBrowserFirstWorkspace(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-no-collector", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "upload"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	promptRepo := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "generate_draft",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "Title: {{title}}",
	}}
	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-no-collector",
		TargetType:    "wechat-longform",
		SourceProfile: "web-upload",
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
		Title:              "Source",
		TargetType:         "wechat-longform",
		SourceProfile:      "web-upload",
		Version:            "v1",
	})

	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, run.Status)

	storedRun, err := provider.RewritePipelineRunRepo().GetByID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Equal(t, workspace.ID, storedRun.FinalDraftID)

	storedWorkspace, err := provider.WorkspaceRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ArticleWorkspaceStatusDraft, storedWorkspace.Status)
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

func TestRewriteOrchestratorRunWorkspaceTransitionFailureDoesNotLeaveActiveRun(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-workspace-transition-fails", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-workspace-transition-fails",
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
	failed := false
	workspaceRepo := failingWorkspaceRepoForReview{base: provider.WorkspaceRepo(), failOnStatus: domain.ArticleWorkspaceStatusRewritePending, err: errors.New("workspace transition failed"), failed: &failed}
	orchestrator := NewRewriteOrchestrator(
		NewRewriteProfileRegistry(profileRepo),
		provider.RewritePipelineRunRepo(),
		provider.RewriteStageRunRepo(),
		workspaceRepo,
		NewRewriteStageExecutor(&stubRewritePromptRegistry{}, &recordingLLMClient{}, NewQualityGateEngine()),
		NewDraftMaterializer(provider.DraftRepo(), provider.WorkspaceRepo()),
	)

	run, err := orchestrator.Run(t.Context(), RewriteRunRequest{
		WorkspaceArticleID: workspace.ID,
		Title:              "Source",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "v1",
	})

	require.Nil(t, run)
	require.Error(t, err)
	require.ErrorContains(t, err, "workspace transition failed")

	runs, listErr := provider.RewritePipelineRunRepo().List(t.Context(), 10)
	require.NoError(t, listErr)
	require.Empty(t, runs)

	storedWorkspace, getErr := provider.WorkspaceRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, getErr)
	require.Equal(t, domain.ArticleWorkspaceStatusImported, storedWorkspace.Status)
	require.Equal(t, []string{domain.ArticleWorkspaceStatusImported}, storedWorkspace.StatusHistory)
}

func TestRewriteOrchestratorRunRestoresWorkspaceWhenRewritingTransitionFails(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-workspace-rewriting-fails", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-workspace-rewriting-fails",
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
	failed := false
	workspaceRepo := failingWorkspaceRepoForReview{base: provider.WorkspaceRepo(), failOnStatus: domain.ArticleWorkspaceStatusRewriting, err: errors.New("workspace rewriting transition failed"), failed: &failed}
	orchestrator := NewRewriteOrchestrator(
		NewRewriteProfileRegistry(profileRepo),
		provider.RewritePipelineRunRepo(),
		provider.RewriteStageRunRepo(),
		workspaceRepo,
		NewRewriteStageExecutor(&stubRewritePromptRegistry{}, &recordingLLMClient{}, NewQualityGateEngine()),
		NewDraftMaterializer(provider.DraftRepo(), provider.WorkspaceRepo()),
	)

	run, err := orchestrator.Run(t.Context(), RewriteRunRequest{
		WorkspaceArticleID: workspace.ID,
		Title:              "Source",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "v1",
	})

	require.Nil(t, run)
	require.Error(t, err)
	require.ErrorContains(t, err, "workspace rewriting transition failed")

	runs, listErr := provider.RewritePipelineRunRepo().List(t.Context(), 10)
	require.NoError(t, listErr)
	require.Empty(t, runs)

	storedWorkspace, getErr := provider.WorkspaceRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, getErr)
	require.Equal(t, domain.ArticleWorkspaceStatusImported, storedWorkspace.Status)
	require.Equal(t, []string{domain.ArticleWorkspaceStatusImported}, storedWorkspace.StatusHistory)
	require.Empty(t, storedWorkspace.Notes)
}

func TestRewriteOrchestratorWorkflowMetadataOverridesStagePromptRef(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-workflow-override", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	promptRepo := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "generate_draft_alt",
		Version:        "v2",
		SystemTemplate: "workflow sys {{title}} [{{workflow_template_id}}]",
		UserTemplate:   "workflow user {{title}} [{{workflow_node_generate_draft}}]",
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
	client := &recordingLLMClient{response: &llminfra.GenerateResponse{Response: &domain.LLMResponse{
		Content: `{"title":"Final Title","body":"Paragraph 1","template":"daily-intelligence"}`,
		Model:   "mock-1",
	}}}
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
		Title:              "Source",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "v1",
		Metadata: map[string]any{
			"workflow_template_id":         "workflow-a",
			"workflow_node_generate_draft": "node-generate-draft",
			workflowStageOverridesMetadataKey: map[string]workflowStageOverride{
				"generate_draft": {
					NodeID:    "node-generate-draft",
					PromptRef: "generate_draft_alt@v2",
				},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, run.Status)
	require.Equal(t, "generate_draft_alt", promptRepo.gotKey)
	require.Equal(t, "v2", promptRepo.gotVersion)
	require.Equal(t, "generate_draft_alt@v2", client.gotReq.Metadata["prompt_ref"])
	require.Equal(t, "workflow sys Source [workflow-a]", client.gotReq.Messages[0].Content)
	require.Equal(t, "workflow user Source [node-generate-draft]", client.gotReq.Messages[1].Content)

	stageRuns, err := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Len(t, stageRuns, 1)
	require.Equal(t, "generate_draft_alt@v2", stageRuns[0].PromptRef)
	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(stageRuns[0].InputJSON), &input))
	require.Equal(t, "workflow-a", input["workflow_template_id"])
	require.Equal(t, "node-generate-draft", input["workflow_node_generate_draft"])
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
		Title:              "Source",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "v1",
	})

	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, run.Status)
	require.Equal(t, rewriteWorkflowMaterializeNodeID, run.CurrentStage)

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

func TestRewriteOrchestratorAppliesWorkflowOverridesToRepairStage(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-repair-workflow", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	promptRepo := &multiPromptRegistry{prompts: map[string]*domain.PromptTemplate{
		"rewrite_stage@v1": {
			Key:            "rewrite_stage",
			Version:        "v1",
			SystemTemplate: "sys",
			UserTemplate:   "Title: {{title}}",
		},
		"repair_prompt_alt@v2": {
			Key:            "repair_prompt_alt",
			Version:        "v2",
			SystemTemplate: "repair sys {{workflow_template_id}} {{workflow_marker}}",
			UserTemplate:   "repair user {{title}} {{workflow_node_repair_draft}}",
		},
	}}
	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-repair-workflow",
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
		Title:              "Source",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "v1",
		Metadata: map[string]any{
			"workflow_template_id": "workflow-repair",
			workflowStageOverridesMetadataKey: map[string]workflowStageOverride{
				"repair_draft": {
					NodeID:    "node-repair-draft",
					PromptRef: "repair_prompt_alt@v2",
					Vars: map[string]any{
						"workflow_marker": "repair-path",
					},
				},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, run.Status)
	require.Len(t, client.gotReqs, 2)
	require.Equal(t, "repair_prompt_alt@v2", client.gotReqs[1].Metadata["prompt_ref"])
	require.Equal(t, "repair sys workflow-repair repair-path", client.gotReqs[1].Messages[0].Content)
	require.Equal(t, "repair user Draft Title node-repair-draft", client.gotReqs[1].Messages[1].Content)

	stageRuns, stageErr := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, stageErr)
	require.Len(t, stageRuns, 2)
	require.Equal(t, "repair_prompt_alt@v2", stageRuns[1].PromptRef)
	require.Contains(t, stageRuns[1].InputJSON, "repair-path")
	require.Contains(t, stageRuns[1].InputJSON, "node-repair-draft")
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

	run := domain.NewRewritePipelineRun("profile-resume", "v1", workspace.ID, "wechat-longform", "sspai")
	run.ID = "run-resume"
	run.Status = domain.RewriteRunRunning
	run.CurrentStage = "finalize"
	run.Metadata = map[string]any{
		"title":                       "Source",
		"rewrite_workflow_checkpoint": map[string]any{"node_id": "finalize", "payload": map[string]any{"title": "Draft Title", "body": "Draft body", "template": "daily-intelligence"}},
	}
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
	require.Equal(t, rewriteWorkflowMaterializeNodeID, resumed.CurrentStage)
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

func TestRewriteOrchestratorRunsThroughWorkflowKernelAndCreatesDraft(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-kernel", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	promptRepo := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "generate_draft",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "Title: {{title}}",
	}}
	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-kernel",
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
		Content: `{"title":"Kernel Title","body":"Kernel body","template":"daily-intelligence"}`,
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
		Title:              "Source",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "v1",
	})

	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, run.Status)
	require.Equal(t, rewriteWorkflowMaterializeNodeID, run.CurrentStage)
	require.NotEmpty(t, run.FinalDraftID)

	stageRuns, stageErr := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, stageErr)
	require.Len(t, stageRuns, 1)
	require.Equal(t, "generate_draft", stageRuns[0].StageName)

	draft, draftErr := provider.DraftRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, draftErr)
	require.Equal(t, "Kernel Title", draft.Headline["title"])
	require.Equal(t, []string{"Kernel body"}, draft.Headline["body"])
}

func TestRewriteOrchestratorResumeUsesCheckpointInsteadOfStageHistoryReconstruction(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-resume-checkpoint", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	promptRepo := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "rewrite_stage",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "Title: {{title}} Body: {{body}}",
	}}
	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-resume-checkpoint",
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

	run := domain.NewRewritePipelineRun("profile-resume-checkpoint", "v1", workspace.ID, "wechat-longform", "sspai")
	run.ID = "run-resume-checkpoint"
	run.Status = domain.RewriteRunRunning
	run.CurrentStage = "finalize"
	run.Metadata = map[string]any{
		"title":                       "Source",
		"rewrite_workflow_checkpoint": map[string]any{"node_id": "finalize", "payload": map[string]any{"title": "Draft Title", "body": "Fresh checkpoint body", "template": "daily-intelligence"}},
	}
	require.NoError(t, provider.RewritePipelineRunRepo().Create(t.Context(), run))

	now := time.Now().UTC()
	require.NoError(t, provider.RewriteStageRunRepo().Create(t.Context(), &domain.RewriteStageRun{
		ID:            "stage-stale",
		PipelineRunID: run.ID,
		StageName:     "generate_draft",
		StageType:     "generate_draft",
		Status:        domain.RewriteStageSucceeded,
		Attempt:       1,
		OutputJSON:    `{"title":"Draft Title","body":"Stale stage body","template":"daily-intelligence"}`,
		StartedAt:     now,
		CompletedAt:   &now,
	}))

	resumed, err := orchestrator.Resume(t.Context(), run.ID, "Source")

	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, resumed.Status)
	require.Equal(t, rewriteWorkflowMaterializeNodeID, resumed.CurrentStage)
	require.Equal(t, 1, client.callCount)
	require.Contains(t, client.gotReqs[0].Messages[1].Content, "Fresh checkpoint body")
	require.NotContains(t, client.gotReqs[0].Messages[1].Content, "Stale stage body")

	stageRuns, stageErr := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, stageErr)
	require.Len(t, stageRuns, 2)
	require.Equal(t, []string{"generate_draft", "finalize"}, []string{stageRuns[0].StageName, stageRuns[1].StageName})
}

func TestRewriteOrchestratorResumeRequiresCheckpointInsteadOfStageHistoryReconstruction(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-resume-no-checkpoint", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-resume-no-checkpoint",
		TargetType:    "wechat-longform",
		SourceProfile: "sspai",
		Version:       "v1",
		Enabled:       true,
		Stages: []domain.RewriteStageDefinition{
			{Name: "generate_draft", Type: "generate_draft", PromptRef: "rewrite_stage@v1", Enabled: true},
			{Name: "finalize", Type: "finalize", PromptRef: "rewrite_stage@v1", Enabled: true},
		},
	}}
	orchestrator := NewRewriteOrchestrator(
		NewRewriteProfileRegistry(profileRepo),
		provider.RewritePipelineRunRepo(),
		provider.RewriteStageRunRepo(),
		provider.WorkspaceRepo(),
		NewRewriteStageExecutor(&stubRewritePromptRegistry{}, &recordingLLMClient{}, NewQualityGateEngine()),
		NewDraftMaterializer(provider.DraftRepo(), provider.WorkspaceRepo()),
	)

	run := domain.NewRewritePipelineRun("profile-resume-no-checkpoint", "v1", workspace.ID, "wechat-longform", "sspai")
	run.ID = "run-resume-no-checkpoint"
	run.Status = domain.RewriteRunRunning
	run.CurrentStage = "finalize"
	run.Metadata = map[string]any{"title": "Source"}
	require.NoError(t, provider.RewritePipelineRunRepo().Create(t.Context(), run))

	now := time.Now().UTC()
	require.NoError(t, provider.RewriteStageRunRepo().Create(t.Context(), &domain.RewriteStageRun{
		ID:            "stage-stale-history",
		PipelineRunID: run.ID,
		StageName:     "generate_draft",
		StageType:     "generate_draft",
		Status:        domain.RewriteStageSucceeded,
		Attempt:       1,
		OutputJSON:    `{"title":"Draft Title","body":"Stale stage body","template":"daily-intelligence"}`,
		StartedAt:     now,
		CompletedAt:   &now,
	}))

	resumed, err := orchestrator.Resume(t.Context(), run.ID, "Source")

	require.NotNil(t, resumed)
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrValidation, appErr.Code)
	require.Contains(t, appErr.Message, "resumable checkpoint")
	require.Equal(t, domain.RewriteRunFailed, resumed.Status)
	require.Equal(t, "finalize", resumed.CurrentStage)

	stageRuns, stageErr := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, stageErr)
	require.Len(t, stageRuns, 2)
	require.Equal(t, domain.RewriteStageSucceeded, stageRuns[0].Status)
	require.Equal(t, domain.RewriteStageFailed, stageRuns[1].Status)
	require.Equal(t, "finalize", stageRuns[1].StageName)
}

func TestRewriteOrchestratorResumeDoesNotRematerializeExistingWorkspaceDraft(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-resume-idempotent", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	require.NoError(t, provider.WorkspaceRepo().TransitionStatus(t.Context(), workspace.ID, domain.ArticleWorkspaceStatusRewritePending, rewritePendingNote))
	require.NoError(t, provider.WorkspaceRepo().TransitionStatus(t.Context(), workspace.ID, domain.ArticleWorkspaceStatusRewriting, rewritingNote))
	require.NoError(t, provider.DraftRepo().Create(t.Context(), &domain.ArticleDraft{
		ID:         workspace.ID,
		Template:   "daily-intelligence",
		Headline:   map[string]any{"title": "Existing Draft", "body": []string{"Existing body"}},
		Meta:       map[string]any{"title": "Existing Draft"},
		Sections:   []any{},
		SourceRefs: []any{},
	}))

	promptRepo := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "rewrite_stage",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "Title: {{title}} Body: {{body}}",
	}}
	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:            "profile-resume-idempotent",
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
		Content: `{"title":"Should Not Replace","body":"Should Not Replace","template":"daily-intelligence"}`,
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

	run := domain.NewRewritePipelineRun("profile-resume-idempotent", "v1", workspace.ID, "wechat-longform", "sspai")
	run.ID = "run-resume-idempotent"
	run.Status = domain.RewriteRunRunning
	run.CurrentStage = rewriteWorkflowMaterializeNodeID
	run.Metadata = map[string]any{
		"title":                       "Source",
		"rewrite_workflow_checkpoint": map[string]any{"node_id": rewriteWorkflowMaterializeNodeID, "payload": map[string]any{"title": "Existing Draft", "body": "Existing body", "template": "daily-intelligence", "rewrite_stage_output": map[string]any{"title": "Existing Draft", "body": "Existing body", "template": "daily-intelligence"}}},
	}
	require.NoError(t, provider.RewritePipelineRunRepo().Create(t.Context(), run))

	resumed, err := orchestrator.Resume(t.Context(), run.ID, "Source")

	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, resumed.Status)
	require.Equal(t, 0, client.callCount)
	require.Empty(t, resumed.Metadata[rewriteWorkflowCheckpointMetadataKey])

	draft, draftErr := provider.DraftRepo().GetByID(t.Context(), workspace.ID)
	require.NoError(t, draftErr)
	require.Equal(t, "Existing Draft", draft.Headline["title"])
	require.Equal(t, []string{"Existing body"}, draft.Headline["body"])
}

func TestRewriteOrchestratorRunPersistsKernelFailureStageContextAndInputSnapshot(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("article-failure-context", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "collector"}, nil)
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	promptRepo := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "generate_draft",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "Title: {{title}}",
	}}
	profileRepo := &stubRewritePipelineProfileRepo{profile: &domain.RewritePipelineProfile{
		ID:                "profile-failure-context",
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
		Name:      "rewrite-default",
		Model:     "gpt-4.1-mini-real",
		MaxTokens: 512,
	}}
	client := &recordingLLMClient{err: errors.New("llm unavailable")}
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
		Title:              "Source",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "v1",
	})

	require.Error(t, err)
	require.NotNil(t, run)
	stageRuns, stageErr := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, stageErr)
	require.Len(t, stageRuns, 1)
	require.Equal(t, domain.RewriteStageFailed, stageRuns[0].Status)
	require.Equal(t, "generate_draft", stageRuns[0].StageName)
	require.Equal(t, "generate_draft", stageRuns[0].StageType)
	require.Equal(t, "generate_draft@v1", stageRuns[0].PromptRef)
	require.Equal(t, "rewrite-default", stageRuns[0].LLMProfileRef)
	require.Contains(t, stageRuns[0].InputJSON, "Source")
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
