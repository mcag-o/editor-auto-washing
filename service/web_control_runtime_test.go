package service

import (
	"content-hub/domain"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	llminfra "content-hub/infra/llm"

	"github.com/stretchr/testify/require"
)

func TestBuildWebControlRuntimeReturnsReadyServices(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	runtime, err := BuildWebControlRuntime(repos)

	require.NoError(t, err)
	require.NotNil(t, runtime)
	require.NotNil(t, runtime.Config)
	require.NotNil(t, runtime.Control)
	require.NotNil(t, runtime.Audit)
	require.NotNil(t, runtime.Intake)
	require.NotNil(t, runtime.Articles)
}

func TestBuildWebControlRuntimeIncludesWorkflowAndTemplateServices(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	runtime, err := BuildWebControlRuntime(repos)

	require.NoError(t, err)
	require.NotNil(t, runtime)
	require.NotNil(t, runtime.Workflows)
	require.NotNil(t, runtime.Templates)
}

func TestBuildWebControlRuntimeDoesNotRequireCompatibilityInputRepo(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	runtime, err := BuildWebControlRuntime(repos)

	require.NoError(t, err)
	require.NotNil(t, runtime)
	require.NotNil(t, runtime.Control)
	require.NotNil(t, runtime.Articles)
}

func TestBuildWebControlRuntimeRequiresRepos(t *testing.T) {
	runtime, err := BuildWebControlRuntime(nil)

	require.Nil(t, runtime)
	require.Error(t, err)
	require.ErrorContains(t, err, "web control runtime repos are required")
}

type stubWebControlProcessingCycleRunner struct {
	called           bool
	concurrencyLimit int
	err              error
}

func (r *stubWebControlProcessingCycleRunner) ProcessPending(ctx context.Context, concurrencyLimit int) error {
	r.called = true
	r.concurrencyLimit = concurrencyLimit
	_ = ctx
	return r.err
}

func TestWebControlPlaneServiceStartPassesConfiguredConcurrencyToProcessingCycle(t *testing.T) {
	controlRepo := &stubSystemControlStateRepo{}
	auditRepo := &stubAuditLogRepo{}
	runner := &stubWebControlProcessingCycleRunner{}
	svc := NewWebControlPlaneService(
		NewControlStateService(controlRepo),
		NewAuditLogService(auditRepo),
		runner,
	)

	state, err := svc.Start(t.Context(), "local-admin", 3)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, domain.SystemStateStopped, state.State)
	require.True(t, runner.called)
	require.Equal(t, 3, runner.concurrencyLimit)
	require.Len(t, auditRepo.logs, 1)
	require.Equal(t, "control_plane.started", auditRepo.logs[0].Action)
	require.Equal(t, "success", auditRepo.logs[0].Result)
	require.Equal(t, 3, auditRepo.logs[0].Metadata["concurrency_limit"])
	storedState, getErr := controlRepo.Get(t.Context())
	require.NoError(t, getErr)
	require.Equal(t, domain.SystemStateStopped, storedState.State)
}

func TestWebControlPlaneServiceStartReturnsFailureSemanticsWhenProcessingCycleFails(t *testing.T) {
	controlRepo := &stubSystemControlStateRepo{}
	auditRepo := &stubAuditLogRepo{}
	runner := &stubWebControlProcessingCycleRunner{err: errors.New("cycle failed")}
	svc := NewWebControlPlaneService(
		NewControlStateService(controlRepo),
		NewAuditLogService(auditRepo),
		runner,
	)

	state, err := svc.Start(t.Context(), "local-admin", 2)

	require.Nil(t, state)
	require.Error(t, err)
	require.ErrorContains(t, err, "process pending browser intake items")
	require.True(t, runner.called)
	require.Len(t, auditRepo.logs, 1)
	require.Equal(t, "control_plane.started", auditRepo.logs[0].Action)
	require.Equal(t, "failure", auditRepo.logs[0].Result)
	require.Contains(t, auditRepo.logs[0].Message, "cycle failed")
	require.Equal(t, 2, auditRepo.logs[0].Metadata["concurrency_limit"])
	storedState, getErr := controlRepo.Get(t.Context())
	require.NoError(t, getErr)
	require.Equal(t, domain.SystemStateStopped, storedState.State)
}

type stubBrowserWorkspaceContinuationRepo struct {
	items []domain.ArticleWorkspaceRecord
}

func (r *stubBrowserWorkspaceContinuationRepo) Create(context.Context, *domain.ArticleWorkspaceRecord) error {
	return nil
}

func (r *stubBrowserWorkspaceContinuationRepo) Update(_ context.Context, record *domain.ArticleWorkspaceRecord) error {
	for i := range r.items {
		if r.items[i].ID == record.ID {
			r.items[i] = *record
			return nil
		}
	}
	return domain.NewNotFoundErr("workspace_article", record.ID)
}

func (r *stubBrowserWorkspaceContinuationRepo) GetByID(_ context.Context, id string) (*domain.ArticleWorkspaceRecord, error) {
	for i := range r.items {
		if r.items[i].ID == id {
			copyValue := r.items[i]
			return &copyValue, nil
		}
	}
	return nil, domain.NewNotFoundErr("workspace_article", id)
}

func (r *stubBrowserWorkspaceContinuationRepo) List(_ context.Context, status *string) ([]domain.ArticleWorkspaceRecord, error) {
	if status == nil || *status == "" {
		return append([]domain.ArticleWorkspaceRecord(nil), r.items...), nil
	}
	filtered := []domain.ArticleWorkspaceRecord{}
	for _, item := range r.items {
		if item.Status == *status {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (r *stubBrowserWorkspaceContinuationRepo) ListByIngestionID(context.Context, string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (r *stubBrowserWorkspaceContinuationRepo) TransitionStatus(context.Context, string, string, string) error {
	return nil
}

func (r *stubBrowserWorkspaceContinuationRepo) Delete(context.Context, string) error {
	return nil
}

type stubBrowserWorkspaceContinuationIntake struct {
	called             bool
	callCount          int
	workspaceID        string
	article            domain.IntakeArticle
	result             *ArticleIntakeResult
	resumeResult       *SourceProcessingRewriteResult
	resumeRewriteRunID string
	resumeCalled       bool
	err                error
}

func (s *stubBrowserWorkspaceContinuationIntake) IntakeResultIntoWorkspace(_ context.Context, workspaceArticleID string, article domain.IntakeArticle) (*ArticleIntakeResult, error) {
	s.called = true
	s.callCount++
	s.workspaceID = workspaceArticleID
	s.article = article
	return s.result, s.err
}

func (s *stubBrowserWorkspaceContinuationIntake) ResumeResult(_ context.Context, rewriteRunID string, article domain.IntakeArticle) (*SourceProcessingRewriteResult, error) {
	s.resumeCalled = true
	s.callCount++
	s.resumeRewriteRunID = rewriteRunID
	s.article = article
	return s.resumeResult, s.err
}

type stubBrowserWorkspaceContinuationRenderer struct {
	called   bool
	draftID  string
	platform string
	err      error
}

func (s *stubBrowserWorkspaceContinuationRenderer) Render(_ context.Context, draftID, platform, templateName string) (*domain.RenderedAssetRecord, error) {
	s.called = true
	s.draftID = draftID
	s.platform = platform
	_ = templateName
	if s.err != nil {
		return nil, s.err
	}
	return &domain.RenderedAssetRecord{}, nil
}

func TestBrowserWorkspaceContinuationRunnerProcessesImportedBrowserIntake(t *testing.T) {
	workspaceRepo := &stubBrowserWorkspaceContinuationRepo{items: []domain.ArticleWorkspaceRecord{{
		ID:     "workspace-1",
		Title:  "Browser Intake",
		Status: domain.ArticleWorkspaceStatusImported,
		Source: domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/Browser Intake"},
		Metadata: map[string]any{
			"source_body":             "Body",
			"target_type":             "wechat-longform",
			"source_profile":          "web-paste",
			"render_platform":         "wechat",
			"rewrite_profile_version": "v1",
		},
	}}}
	intake := &stubBrowserWorkspaceContinuationIntake{result: &ArticleIntakeResult{WorkspaceArticle: &domain.ArticleWorkspaceRecord{ID: "workspace-1"}, DraftID: "draft-1"}}
	renderer := &stubBrowserWorkspaceContinuationRenderer{}
	runner := newBrowserWorkspaceContinuationRunner(workspaceRepo, intake, renderer)

	err := runner.ProcessPending(t.Context(), 10)

	require.NoError(t, err)
	require.True(t, intake.called)
	require.Equal(t, "workspace-1", intake.workspaceID)
	require.Equal(t, "paste", intake.article.SourceType)
	require.Equal(t, "Browser Intake", intake.article.Title)
	require.Equal(t, "Body", intake.article.Body)
	require.Equal(t, "wechat-longform", intake.article.TargetType)
	require.Equal(t, "web-paste", intake.article.SourceProfile)
	require.True(t, renderer.called)
	require.Equal(t, "draft-1", renderer.draftID)
	require.Equal(t, "wechat", renderer.platform)
}

func TestBrowserWorkspaceContinuationRunnerResumesImportedBrowserIntakeFromSavedRewriteRun(t *testing.T) {
	workspaceRepo := &stubBrowserWorkspaceContinuationRepo{items: []domain.ArticleWorkspaceRecord{{
		ID:     "workspace-1",
		Title:  "Browser Intake",
		Status: domain.ArticleWorkspaceStatusImported,
		Source: domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/Browser Intake"},
		Metadata: map[string]any{
			"source_body":             "Body",
			"target_type":             "wechat-longform",
			"source_profile":          "web-paste",
			"render_platform":         "wechat",
			"rewrite_profile_version": "v1",
			"resume_rewrite_run_id":   "rewrite-1",
		},
	}}}
	intake := &stubBrowserWorkspaceContinuationIntake{resumeResult: &SourceProcessingRewriteResult{WorkspaceArticleID: "workspace-1", DraftID: "draft-1", RewriteRunID: "rewrite-1"}}
	renderer := &stubBrowserWorkspaceContinuationRenderer{}
	runner := newBrowserWorkspaceContinuationRunner(workspaceRepo, intake, renderer)

	err := runner.ProcessPending(t.Context(), 10)

	require.NoError(t, err)
	require.True(t, intake.resumeCalled)
	require.False(t, intake.called)
	require.Equal(t, "rewrite-1", intake.resumeRewriteRunID)
	require.True(t, renderer.called)
	require.Equal(t, "draft-1", renderer.draftID)
	updated, err := workspaceRepo.GetByID(t.Context(), "workspace-1")
	require.NoError(t, err)
	_, exists := updated.Metadata["resume_rewrite_run_id"]
	require.False(t, exists)
}

func TestBrowserWorkspaceContinuationRunnerHonorsConcurrencyLimit(t *testing.T) {
	workspaceRepo := &stubBrowserWorkspaceContinuationRepo{items: []domain.ArticleWorkspaceRecord{
		{ID: "workspace-1", Title: "Browser Intake 1", Status: domain.ArticleWorkspaceStatusImported, Source: domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/1"}, Metadata: map[string]any{"source_body": "Body 1", "target_type": "wechat-longform", "source_profile": "web-paste", "render_platform": "wechat", "rewrite_profile_version": "v1"}},
		{ID: "workspace-2", Title: "Browser Intake 2", Status: domain.ArticleWorkspaceStatusImported, Source: domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/2"}, Metadata: map[string]any{"source_body": "Body 2", "target_type": "wechat-longform", "source_profile": "web-paste", "render_platform": "wechat", "rewrite_profile_version": "v1"}},
	}}
	intake := &stubBrowserWorkspaceContinuationIntake{result: &ArticleIntakeResult{WorkspaceArticle: &domain.ArticleWorkspaceRecord{ID: "workspace-1"}, DraftID: "draft-1"}}
	renderer := &stubBrowserWorkspaceContinuationRenderer{}
	runner := newBrowserWorkspaceContinuationRunner(workspaceRepo, intake, renderer)

	err := runner.ProcessPending(t.Context(), 1)

	require.NoError(t, err)
	require.Equal(t, 1, intake.callCount)
	require.Equal(t, "workspace-1", intake.workspaceID)
}

func TestBuildWebControlRuntimeStartProcessesImportedBrowserWorkspaceIntake(t *testing.T) {
	root := t.TempDir()
	templateDir := filepath.Join(root, "templates")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "daily-intelligence.html"), []byte(`<html><body><h1>{{TITLE}}</h1><div>{{BODY_SECTIONS}}</div><footer>{{CTA}}</footer></body></html>`), 0o644))
	repos, cleanup, err := BuildRuntimeRepos(root)
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	workspace := domain.NewArticleWorkspaceRecord("workspace-browser-1", "Browser Intake", "", domain.ArticleWorkspaceSource{
		SourceType: "paste",
		URL:        "browser://paste/Browser Intake",
	}, map[string]any{
		"source_body":             "Body",
		"target_type":             "wechat-longform",
		"source_profile":          "web-paste",
		"render_platform":         "wechat",
		"rewrite_profile_version": "v1",
	})
	require.NoError(t, repos.WorkspaceRepo.Create(t.Context(), workspace))

	repos.LLMClient = staticLLMClientForWebControlRuntimeTests()
	require.NoError(t, repos.RewritePipelineProfileRepo.Upsert(t.Context(), &domain.RewritePipelineProfile{
		ID:                    "profile-web-mainline",
		Name:                  "Web Control Mainline",
		TargetType:            "wechat-longform",
		SourceProfile:         "web-paste",
		Version:               "v1",
		Description:           "Mainline web control rewrite profile",
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

	runtime, err := BuildWebControlRuntime(repos)
	require.NoError(t, err)

	state, err := runtime.Control.Start(t.Context(), "local-admin", 1)

	require.NoError(t, err)
	require.NotNil(t, state)
	updatedWorkspace, err := repos.WorkspaceRepo.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ArticleWorkspaceStatusRendered, updatedWorkspace.Status)
	draft, err := repos.DraftRepo.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, workspace.ID, draft.ID)
	assets, err := repos.AssetRepo.List(t.Context(), draft.ID, "wechat")
	require.NoError(t, err)
	require.Len(t, assets, 1)
	runs, err := repos.RewritePipelineRunRepo.List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, workspace.ID, runs[0].WorkspaceArticleID)
}

func staticLLMClientForWebControlRuntimeTests() llminfra.StaticClient {
	return llminfra.StaticClient{Response: domain.LLMResponse{
		Content:      `{"title":"Pasted Rewrite Title","body":"Rendered mainline body.","template":"daily-intelligence","meta":{"digest":"Web control digest","author":"Integration Bot"},"sections":[{"cn":"Main Section","blocks":[{"type":"card","title":"Key Point","body":["Control plane detail."],"source":"Web Control"}]}],"conclusion":"End note.","cta":"Read more."}`,
		Model:        "static-integration-model",
		FinishReason: "stop",
	}}
}
