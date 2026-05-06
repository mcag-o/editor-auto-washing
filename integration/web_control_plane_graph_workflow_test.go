package integration

import (
	"content-hub/domain"
	"content-hub/infra/config"
	llminfra "content-hub/infra/llm"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	httpserver "content-hub/transport/http"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebControlPlaneUploadToRenderedResultWithWorkflowTemplate(t *testing.T) {
	root, repos, serverURL := newWebControlPlaneIntegrationServer(t)
	require.DirExists(t, root)

	templateResp := postJSON(t, serverURL+"/api/templates", map[string]any{
		"id":             "template-definition-mainline",
		"name":           "Generate Draft Mainline",
		"type":           "prompt",
		"version":        "v1",
		"enabled":        true,
		"content":        "Rewrite {{title}} into a polished draft.",
		"variables_json": map[string]any{"title": "string"},
		"updated_by":     "integration-test",
	})
	defer templateResp.Body.Close()
	require.Equal(t, http.StatusCreated, templateResp.StatusCode)

	workflowResp := postJSON(t, serverURL+"/api/workflows", map[string]any{
		"id":            "workflow-template-mainline",
		"name":          "Graph Workflow Mainline",
		"description":   "Mainline integration workflow",
		"version":       "v1",
		"enabled":       true,
		"entry_node_id": "generate_draft",
		"nodes": []map[string]any{{
			"id":          "generate_draft",
			"type":        "rewrite_stage",
			"name":        "generate_draft",
			"config_json": `{"stage_name":"generate_draft","prompt_ref":"generate_draft_alt@v2","vars":{"workflow_marker":"graph-mainline"}}`,
		}},
		"edges":      []map[string]any{},
		"updated_by": "integration-test",
	})
	defer workflowResp.Body.Close()
	require.Equal(t, http.StatusCreated, workflowResp.StatusCode)

	templatesListResp, err := http.Get(serverURL + "/api/templates")
	require.NoError(t, err)
	defer templatesListResp.Body.Close()
	require.Equal(t, http.StatusOK, templatesListResp.StatusCode)

	var templatesList struct {
		Data []domain.TemplateDefinition `json:"data"`
	}
	require.NoError(t, json.NewDecoder(templatesListResp.Body).Decode(&templatesList))
	require.Len(t, templatesList.Data, 1)
	require.Equal(t, "template-definition-mainline", templatesList.Data[0].ID)

	workflowsListResp, err := http.Get(serverURL + "/api/workflows")
	require.NoError(t, err)
	defer workflowsListResp.Body.Close()
	require.Equal(t, http.StatusOK, workflowsListResp.StatusCode)

	var workflowsList struct {
		Data []domain.WorkflowDefinition `json:"data"`
	}
	require.NoError(t, json.NewDecoder(workflowsListResp.Body).Decode(&workflowsList))
	require.Len(t, workflowsList.Data, 1)
	require.Equal(t, "workflow-template-mainline", workflowsList.Data[0].ID)

	pasteResp := postJSON(t, serverURL+"/api/intake/paste", map[string]any{
		"title": "Graph Control Plane Source",
		"body":  "Body pasted for graph workflow mainline.",
	})
	defer pasteResp.Body.Close()
	require.Equal(t, http.StatusCreated, pasteResp.StatusCode)

	var createdDoc domain.SourceDocument
	require.NoError(t, json.NewDecoder(pasteResp.Body).Decode(&createdDoc))

	assignResp := postJSON(t, serverURL+"/api/articles/"+createdDoc.ID+"/workflow-template", map[string]any{
		"workflow_template_id": "workflow-template-mainline",
	})
	defer assignResp.Body.Close()
	require.Equal(t, http.StatusOK, assignResp.StatusCode)

	startResp := postJSON(t, serverURL+"/api/system/start", map[string]any{
		"concurrency_limit": 1,
	})
	defer startResp.Body.Close()
	if startResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(startResp.Body)
		t.Fatalf("unexpected start status %d: %s", startResp.StatusCode, string(body))
	}
	require.Equal(t, http.StatusOK, startResp.StatusCode)

	articleResp, err := http.Get(serverURL + "/api/articles/" + createdDoc.ID)
	require.NoError(t, err)
	defer articleResp.Body.Close()
	require.Equal(t, http.StatusOK, articleResp.StatusCode)

	var article domain.SourceDocument
	require.NoError(t, json.NewDecoder(articleResp.Body).Decode(&article))
	require.Equal(t, domain.SourceDocumentStatusCompleted, article.Status)
	require.Equal(t, "workflow-template-mainline", article.Metadata["workflow_template_id"])
	require.NotEmpty(t, article.WorkspaceArticleID)
	require.NotEmpty(t, article.RewriteRunID)

	stagesResp, err := http.Get(serverURL + "/api/articles/" + createdDoc.ID + "/stages")
	require.NoError(t, err)
	defer stagesResp.Body.Close()
	require.Equal(t, http.StatusOK, stagesResp.StatusCode)

	var stagesPayload struct {
		Article domain.SourceDocument      `json:"article"`
		Run     *domain.RewritePipelineRun `json:"run"`
		Stages  []domain.RewriteStageRun   `json:"stages"`
	}
	require.NoError(t, json.NewDecoder(stagesResp.Body).Decode(&stagesPayload))
	require.NotNil(t, stagesPayload.Run)
	require.Equal(t, domain.RewriteRunSucceeded, stagesPayload.Run.Status)
	require.Equal(t, "workflow-template-mainline", stagesPayload.Run.Metadata["workflow_template_id"])
	require.Equal(t, "generate_draft_alt@v2", stagesPayload.Run.Metadata["workflow_prompt_ref"])
	require.Len(t, stagesPayload.Stages, 1)
	require.Equal(t, domain.RewriteStageSucceeded, stagesPayload.Stages[0].Status)
	require.Equal(t, "generate_draft_alt@v2", stagesPayload.Stages[0].PromptRef)
	require.Contains(t, stagesPayload.Stages[0].InputJSON, "graph-mainline")

	workspace, err := repos.WorkspaceRepo.GetByID(t.Context(), article.WorkspaceArticleID)
	require.NoError(t, err)
	require.Equal(t, domain.ArticleWorkspaceStatusRendered, workspace.Status)
	require.Equal(t, "workflow-template-mainline", workspace.Metadata["workflow_template_id"])

	draft, err := repos.DraftRepo.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, "daily-intelligence-alt", draft.Template)
	require.Equal(t, "Graph Mainline Title", draft.Headline["title"])

	assets, err := repos.AssetRepo.List(t.Context(), draft.ID, "wechat")
	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.Contains(t, assets[0].Content, "Graph Mainline Title")

	auditResp, err := http.Get(serverURL + "/api/audit")
	require.NoError(t, err)
	defer auditResp.Body.Close()
	require.Equal(t, http.StatusOK, auditResp.StatusCode)

	var auditList struct {
		Data []domain.AuditLog `json:"data"`
	}
	require.NoError(t, json.NewDecoder(auditResp.Body).Decode(&auditList))

	actions := map[string]domain.AuditLog{}
	for _, entry := range auditList.Data {
		actions[entry.Action] = entry
	}
	require.Contains(t, actions, "web_intake.create_from_paste")
	require.Contains(t, actions, "control_plane.started")
	require.Contains(t, actions, "web_control.article.workflow_template_assigned")
	require.Equal(t, createdDoc.ID, actions["web_control.article.workflow_template_assigned"].ResourceID)
	require.Equal(t, "workflow-template-mainline", actions["web_control.article.workflow_template_assigned"].Metadata["workflow_template_id"])
}

func newWebControlPlaneIntegrationServer(t *testing.T) (string, *service.RuntimeRepos, string) {
	t.Helper()

	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	templateDir := filepath.Join(root, "templates")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "daily-intelligence.html"), []byte(`<html><body><h1>{{TITLE}}</h1><div>{{BODY_SECTIONS}}</div><footer>{{CTA}}</footer></body></html>`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "daily-intelligence-alt.html"), []byte(`<html><body><article><header>{{TITLE}}</header><section>{{BODY_SECTIONS}}</section><aside>{{CTA}}</aside></article></body></html>`), 0o644))

	repos, cleanup, err := service.BuildRuntimeRepos(root)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, cleanup())
	})

	repos.LLMClient = llminfra.StaticClient{Response: domain.LLMResponse{
		Content:      `{"title":"Graph Mainline Title","body":"Rendered graph workflow body.","template":"daily-intelligence-alt","meta":{"digest":"Graph digest","author":"Integration Bot"},"sections":[{"cn":"Main Section","blocks":[{"type":"card","title":"Key Point","body":["Workflow-selected control plane detail."],"source":"Web Control"}]}],"conclusion":"Graph end note.","cta":"Read more."}`,
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

	require.NoError(t, repos.PromptTemplateRepo.Upsert(t.Context(), &domain.PromptTemplate{
		Key:            "generate_draft_alt",
		Version:        "v2",
		SystemTemplate: "You rewrite imported articles into drafts using {{workflow_marker}}.",
		UserTemplate:   "Rewrite {{title}} into a polished draft with {{workflow_marker}}.",
		Description:    "Workflow-selected integration rewrite prompt",
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
	t.Cleanup(httpTestServer.Close)

	return root, repos, httpTestServer.URL
}
