package integration

import (
	"content-hub/domain"
	"content-hub/infra/config"
	llminfra "content-hub/infra/llm"
	"content-hub/pkg/repo"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	httpserver "content-hub/transport/http"
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newWebControlPlaneIntegrationServer(t *testing.T) (string, *service.RuntimeRepos, repo.WorkflowRunRepo, repo.WorkflowCheckpointRepo, string) {
	t.Helper()

	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	templateDir := filepath.Join(root, "templates")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "daily-intelligence.html"), []byte(`<html><body><h1>{{TITLE}}</h1><div>{{BODY_SECTIONS}}</div><footer>{{CTA}}</footer></body></html>`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "daily-intelligence-alt.html"), []byte(`<html><body><main><h1>{{TITLE}}</h1><section>{{BODY_SECTIONS}}</section><aside>{{CTA}}</aside></main></body></html>`), 0o644))

	repos, cleanup, err := service.BuildRuntimeRepos(root)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, cleanup())
	})

	repos.LLMClient = llminfra.StaticClient{Response: domain.LLMResponse{
		Content:      `{"title":"Graph Mainline Title","body":"Rendered graph mainline body.","template":"daily-intelligence-alt","meta":{"digest":"Graph digest","author":"Integration Bot"},"sections":[{"cn":"Main Section","blocks":[{"type":"card","title":"Key Point","body":["Graph control detail."],"source":"Web Control"}]}],"conclusion":"End note.","cta":"Read more."}`,
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
		SystemTemplate: "You rewrite imported articles into graph-aware drafts.",
		UserTemplate:   "Rewrite {{title}} with workflow marker {{workflow_marker}}.",
		Description:    "Integration alternate rewrite prompt",
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
		WorkflowRunRepo:    repos.WorkflowRunRepo,
		WorkflowCheckpointRepo: repos.WorkflowCheckpointRepo,
		ConfigLoader:       configLoader,
		RewriteRunRepo:     repos.RewritePipelineRunRepo,
		RewriteStageRepo:   repos.RewriteStageRunRepo,
		AuditLogRepo:       repos.AuditLogRepo,
		WorkspaceRoot:      root,
	}

	server := httpserver.NewServer(cfg, provider)
	httpTestServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpTestServer.Close)

	return root, repos, repos.WorkflowRunRepo, repos.WorkflowCheckpointRepo, httpTestServer.URL
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
