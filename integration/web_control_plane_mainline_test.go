package integration

import (
	"bytes"
	"context"
	"content-hub/domain"
	"content-hub/infra/config"
	llminfra "content-hub/infra/llm"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	httpserver "content-hub/transport/http"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebControlPlanePasteToRenderedResult(t *testing.T) {
	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	templateDir := filepath.Join(root, "templates")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "daily-intelligence.html"), []byte(`<html><body><h1>{{TITLE}}</h1><div>{{BODY_SECTIONS}}</div><footer>{{CTA}}</footer></body></html>`), 0o644))

	repos, cleanup, err := service.BuildRuntimeRepos(root)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, cleanup())
	}()

	repos.LLMClient = llminfra.StaticClient{Response: domain.LLMResponse{
		Content:      `{"title":"Pasted Rewrite Title","body":"Rendered mainline body.","template":"daily-intelligence","meta":{"digest":"Web control digest","author":"Integration Bot"},"sections":[{"cn":"Main Section","blocks":[{"type":"card","title":"Key Point","body":["Control plane detail."],"source":"Web Control"}]}],"conclusion":"End note.","cta":"Read more."}`,
		Model:        "static-integration-model",
		FinishReason: "stop",
	}}

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

	webControlRuntime, err := service.BuildWebControlRuntime(repos)
	require.NoError(t, err)
	rewriteRuntime, err := service.BuildRewriteRuntime(repos)
	require.NoError(t, err)
	formattingSvc := service.NewFormattingPipelineService(repos.DraftRepo, repos.AssetRepo, repos.WorkspaceRepo, repos.Formatter).WithRenderedDir(repos.RenderedDir)

	cfg := config.DefaultConfig()
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 0
	configLoader := config.NewLoader("")
	configLoader.SetCurrent(cfg)

	provider := &httpserver.Provider{
		ContentSvc:         service.NewContentService(repos.ArticleRepo, repos.PublishRepo),
		TemplateSvc:        service.NewTemplateService(repos.TemplateRepo),
		DraftSvc:           service.NewDraftService(repos.DraftRepo),
		FormattingSvc:      formattingSvc,
		AutomationSvc:      service.NewAutomationService(service.NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator()), service.NewIngestionPipelineService(repos.IngestionRepo, repos.WorkspaceRepo, repos.BundleImportTxStarter, workspaceinfra.NewLoader()), nil, nil),
		WorkspaceSvc:       service.NewWorkspaceArticleService(repos.WorkspaceRepo),
		JobSvc:             service.NewJobService(repos.JobRepo, repos.JobEventRepo, noopJobExecutor{}),
		ReviewSvc:          service.NewReviewService(repos.ReviewRepo, repos.WorkspaceRepo),
		PublishSvc:         service.NewPublishGateService(repos.ReviewRepo, repos.AssetRepo, repos.DraftRepo, repos.PublishRepo, repos.WorkspaceRepo, map[string]service.PublisherProvider{"wechat": integrationPublishProviderStub{}}),
		RewriteRuntime:     rewriteRuntime,
		WebControlRuntime:  webControlRuntime,
		WorkflowEngine:     service.NewWorkflowEngine(),
		ConfigLoader:       configLoader,
		SourceDocumentRepo: repos.SourceDocumentRepo,
		RewriteRunRepo:     repos.RewritePipelineRunRepo,
		RewriteStageRepo:   repos.RewriteStageRunRepo,
		AuditLogRepo:       repos.AuditLogRepo,
		WorkspaceRoot:      root,
	}

	server := httpserver.NewServer(cfg, provider)
	httpTestServer := httptest.NewServer(server.Handler())
	defer httpTestServer.Close()

	pasteResp := postJSON(t, httpTestServer.URL+"/api/intake/paste", map[string]any{
		"title": "Control Plane Source",
		"body":  "Body pasted from the web control plane.",
	})
	defer pasteResp.Body.Close()
	require.Equal(t, http.StatusCreated, pasteResp.StatusCode)

	var createdDoc domain.SourceDocument
	require.NoError(t, json.NewDecoder(pasteResp.Body).Decode(&createdDoc))
	require.Equal(t, domain.SourceDocumentStatusPending, createdDoc.Status)

	startResp := postJSON(t, httpTestServer.URL+"/api/system/start", map[string]any{
		"concurrency_limit": 1,
	})
	defer startResp.Body.Close()
	require.Equal(t, http.StatusOK, startResp.StatusCode)

	var startedState domain.SystemControlState
	require.NoError(t, json.NewDecoder(startResp.Body).Decode(&startedState))
	require.Equal(t, domain.SystemStateRunning, startedState.State)

	statusResp, err := http.Get(httpTestServer.URL + "/api/system/status")
	require.NoError(t, err)
	defer statusResp.Body.Close()
	require.Equal(t, http.StatusOK, statusResp.StatusCode)

	var systemStatus domain.SystemControlState
	require.NoError(t, json.NewDecoder(statusResp.Body).Decode(&systemStatus))
	require.Equal(t, domain.SystemStateRunning, systemStatus.State)

	articleListResp, err := http.Get(httpTestServer.URL + "/api/articles")
	require.NoError(t, err)
	defer articleListResp.Body.Close()
	require.Equal(t, http.StatusOK, articleListResp.StatusCode)

	var articleList struct {
		Data []domain.SourceDocument `json:"data"`
	}
	require.NoError(t, json.NewDecoder(articleListResp.Body).Decode(&articleList))
	require.Len(t, articleList.Data, 1)
	require.Equal(t, createdDoc.ID, articleList.Data[0].ID)
	require.Equal(t, domain.SourceDocumentStatusCompleted, articleList.Data[0].Status)
	require.NotEmpty(t, articleList.Data[0].WorkspaceArticleID)
	require.NotEmpty(t, articleList.Data[0].RewriteRunID)
	require.NotNil(t, articleList.Data[0].CompletedAt)

	articleResp, err := http.Get(httpTestServer.URL + "/api/articles/" + createdDoc.ID)
	require.NoError(t, err)
	defer articleResp.Body.Close()
	require.Equal(t, http.StatusOK, articleResp.StatusCode)

	var article domain.SourceDocument
	require.NoError(t, json.NewDecoder(articleResp.Body).Decode(&article))
	require.Equal(t, domain.SourceDocumentStatusCompleted, article.Status)
	require.Equal(t, articleList.Data[0].WorkspaceArticleID, article.WorkspaceArticleID)

	stagesResp, err := http.Get(httpTestServer.URL + "/api/articles/" + createdDoc.ID + "/stages")
	require.NoError(t, err)
	defer stagesResp.Body.Close()
	require.Equal(t, http.StatusOK, stagesResp.StatusCode)

	var stagesPayload struct {
		Article domain.SourceDocument     `json:"article"`
		Run     *domain.RewritePipelineRun `json:"run"`
		Stages  []domain.RewriteStageRun  `json:"stages"`
	}
	require.NoError(t, json.NewDecoder(stagesResp.Body).Decode(&stagesPayload))
	require.Equal(t, domain.SourceDocumentStatusCompleted, stagesPayload.Article.Status)
	require.NotNil(t, stagesPayload.Run)
	require.Equal(t, domain.RewriteRunSucceeded, stagesPayload.Run.Status)
	require.Len(t, stagesPayload.Stages, 1)
	require.Equal(t, domain.RewriteStageSucceeded, stagesPayload.Stages[0].Status)

	workspace, err := repos.WorkspaceRepo.GetByID(t.Context(), article.WorkspaceArticleID)
	require.NoError(t, err)
	require.Equal(t, domain.ArticleWorkspaceStatusRendered, workspace.Status)
	require.Equal(t, "Control Plane Source", workspace.Title)

	draft, err := repos.DraftRepo.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, "daily-intelligence", draft.Template)
	require.Equal(t, "Pasted Rewrite Title", draft.Headline["title"])

	assets, err := repos.AssetRepo.List(t.Context(), draft.ID, "wechat")
	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.FileExists(t, assets[0].ArtifactPath)
	require.Contains(t, assets[0].Content, "Pasted Rewrite Title")

	auditResp, err := http.Get(httpTestServer.URL + "/api/audit")
	require.NoError(t, err)
	defer auditResp.Body.Close()
	require.Equal(t, http.StatusOK, auditResp.StatusCode)

	var auditList struct {
		Data []domain.AuditLog `json:"data"`
	}
	require.NoError(t, json.NewDecoder(auditResp.Body).Decode(&auditList))
	require.Len(t, auditList.Data, 2)
	require.Contains(t, []string{auditList.Data[0].Action, auditList.Data[1].Action}, "web_intake.create_from_paste")
	require.Contains(t, []string{auditList.Data[0].Action, auditList.Data[1].Action}, "control_plane.started")

	auditByAction := map[string]domain.AuditLog{}
	for _, entry := range auditList.Data {
		auditByAction[entry.Action] = entry
	}
	require.Equal(t, createdDoc.ID, auditByAction["web_intake.create_from_paste"].ResourceID)
	require.Equal(t, "success", auditByAction["web_intake.create_from_paste"].Result)
	require.Equal(t, "success", auditByAction["control_plane.started"].Result)

	auditEntryResp, err := http.Get(httpTestServer.URL + "/api/audit/" + auditByAction["control_plane.started"].ID)
	require.NoError(t, err)
	defer auditEntryResp.Body.Close()
	require.Equal(t, http.StatusOK, auditEntryResp.StatusCode)

	var auditEntry domain.AuditLog
	require.NoError(t, json.NewDecoder(auditEntryResp.Body).Decode(&auditEntry))
	require.Equal(t, "control_plane.started", auditEntry.Action)
	if limit, ok := auditEntry.Metadata["concurrency_limit"].(float64); ok {
		require.Equal(t, 1.0, limit)
	}
}

func postJSON(t *testing.T, url string, payload any) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	return resp
}

type noopJobExecutor struct{}

func (noopJobExecutor) Execute(_ context.Context, _ *domain.WorkflowDefinition, _ *domain.WorkflowContext) error {
	return nil
}

type integrationPublishProviderStub struct{}

func (integrationPublishProviderStub) Publish(_ context.Context, req domain.PublishRequest) (*domain.PublishResult, error) {
	return &domain.PublishResult{Success: true, Platform: req.Platform, Message: "published", Metadata: map[string]any{"provider": "integration-test"}}, nil
}

func (integrationPublishProviderStub) Platforms() []string {
	return []string{"wechat"}
}
