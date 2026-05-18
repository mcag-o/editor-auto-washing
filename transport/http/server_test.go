package http

import (
	"bytes"
	"content-hub/domain"
	"content-hub/infra/config"
	"content-hub/infra/memory"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/fs"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func useTestFrontendFS(t *testing.T, files fstest.MapFS) {
	t.Helper()
	frontendDistFSForTests = func() (fs.FS, bool, error) {
		frontendFS, err := fs.Sub(files, ".")
		if err != nil {
			return nil, false, err
		}
		return frontendFS, true, nil
	}
	t.Cleanup(func() {
		frontendDistFSForTests = nil
	})
}

func disableTestFrontendFS(t *testing.T) {
	t.Helper()
	frontendDistFSForTests = func() (fs.FS, bool, error) {
		return nil, false, nil
	}
	t.Cleanup(func() {
		frontendDistFSForTests = nil
	})
}

func newTestServer(t *testing.T) (*Server, *testWebControlRepos) {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 0

	memProvider := memory.NewProvider()
	workflowDefinitionRepo := &serverWorkflowDefinitionRepo{}
	templateDefinitionRepo := &serverTemplateDefinitionRepo{}
	businessConfigRepo := &stubBusinessConfigRepo{}
	systemControlStateRepo := &stubSystemControlStateRepo{}
	auditLogRepo := &stubAuditLogRepo{}
	webRepos := &testWebControlRepos{
		Workspaces:      memProvider.WorkspaceRepo(),
		AuditLogs:       auditLogRepo,
		Configs:         businessConfigRepo,
		ControlStates:   systemControlStateRepo,
	}

	contentSvc := service.NewContentService(
		memProvider.ArticleRepo(),
		memProvider.PublishRepo(),
	)
	templateSvc := service.NewTemplateService(memProvider.TemplateRepo())
	draftSvc := service.NewDraftService(memProvider.DraftRepo())
	formattingSvc := service.NewFormattingPipelineService(memProvider.DraftRepo(), memProvider.AssetRepo(), memProvider.WorkspaceRepo(), &testFormatter{})
	ingestionSvc := service.NewIngestionPipelineService(memProvider.IngestionRepo(), memProvider.WorkspaceRepo(), memProvider, workspaceinfra.NewLoader())
	workspaceSvc := service.NewWorkspaceArticleService(memProvider.WorkspaceRepo())
	workflowEngine := service.NewWorkflowEngine()
	reviewSvc := service.NewReviewService(memProvider.ReviewRepo(), memProvider.WorkspaceRepo())
	publishSvc := service.NewPublishGateService(memProvider.ReviewRepo(), memProvider.AssetRepo(), memProvider.DraftRepo(), memProvider.PublishRepo(), memProvider.WorkspaceRepo(), map[string]service.PublisherProvider{"wechat": &serverPublishProviderStub{}})
	jobSvc := service.NewJobService(
		memProvider.JobRepo(),
		memProvider.JobEventRepo(),
		&testJobExecutor{engine: workflowEngine},
	)
	workspaceRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspaceRoot, workspaceinfra.WorkspaceConfigFileName), []byte("name: test\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workspaceRoot, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))
	automationSvc := service.NewAutomationService(service.NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator()), ingestionSvc, nil, jobSvc)
	runtimeRepos := &service.RuntimeRepos{
		DraftRepo:              memProvider.DraftRepo(),
		AssetRepo:              memProvider.AssetRepo(),
		WorkspaceRepo:          memProvider.WorkspaceRepo(),
		Formatter:              &testFormatter{},
		WorkflowDefinitionRepo: workflowDefinitionRepo,
		TemplateDefinitionRepo: templateDefinitionRepo,
		WorkflowRunRepo:        memProvider.WorkflowRunRepo(),
		WorkflowCheckpointRepo: memProvider.WorkflowCheckpointRepo(),
		BusinessConfigRepo:     businessConfigRepo,
		SystemControlStateRepo: systemControlStateRepo,
		AuditLogRepo:           auditLogRepo,
	}
	webControlRuntime, err := service.BuildWebControlRuntime(runtimeRepos)
	require.NoError(t, err)
	loader := config.NewLoader("")
	loader.SetCurrent(cfg)

	provider := &Provider{
		ContentSvc:             contentSvc,
		TemplateSvc:            templateSvc,
		DraftSvc:               draftSvc,
		FormattingSvc:          formattingSvc,
		AutomationSvc:          automationSvc,
		WorkspaceSvc:           workspaceSvc,
		JobSvc:                 jobSvc,
		ReviewSvc:              reviewSvc,
		PublishSvc:             publishSvc,
		WebControlRuntime:      webControlRuntime,
		WorkflowEngine:         workflowEngine,
		ConfigLoader:           loader,
		RewriteRunRepo:         memProvider.RewritePipelineRunRepo(),
		RewriteStageRepo:       memProvider.RewriteStageRunRepo(),
		WorkflowRunRepo:        memProvider.WorkflowRunRepo(),
		WorkflowCheckpointRepo: memProvider.WorkflowCheckpointRepo(),
		AuditLogRepo:           auditLogRepo,
		WorkspaceRoot:          workspaceRoot,
	}

	return NewServer(cfg, provider), webRepos
}

func TestServerExposesWorkflowRunAuditRouteWhenReposPresent(t *testing.T) {
	disableTestFrontendFS(t)
	server, _ := newTestServer(t)
	httpTestServer := httptest.NewServer(server.Handler())
	defer httpTestServer.Close()

	resp, err := http.Get(httpTestServer.URL + "/api/workflow-runs/run-1/audit")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.NotEqual(t, http.StatusNotFound, resp.StatusCode)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload []domain.AuditLog
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Empty(t, payload)
}

func TestServerBuildsWithoutCompatibilitySourceRepoExposure(t *testing.T) {
	disableTestFrontendFS(t)
	server, _ := newTestServer(t)
	require.NotNil(t, server)
	require.NotNil(t, server.Handler())
}

func TestServerProviderValidationDoesNotRequireCompatibilitySourceRepo(t *testing.T) {
	disableTestFrontendFS(t)
	server, _ := newTestServer(t)
	require.NotNil(t, server)

	provider := server.provider
	require.NotNil(t, provider)
	require.Nil(t, validateProvider(provider))
}

type testJobExecutor struct {
	engine *service.WorkflowEngine
}

type serverWorkflowDefinitionRepo struct {
	stored map[string]*domain.WorkflowDefinition
}

type serverTemplateDefinitionRepo struct {
	stored map[string]*domain.TemplateDefinition
}

type testFormatter struct{}

type serverPublishProviderStub struct{}

func (testFormatter) Render(_ *domain.ArticleDraft, _ string) (string, error) {
	return "<html><body><h1>ok</h1></body></html>", nil
}

func (testFormatter) ValidateDraft(_ *domain.ArticleDraft, _ string) domain.DraftValidationResult {
	return domain.DraftValidationResult{}
}

func (testFormatter) ValidateRenderedOutput(_ string) []string {
	return nil
}

func (serverPublishProviderStub) Publish(_ context.Context, req domain.PublishRequest) (*domain.PublishResult, error) {
	return &domain.PublishResult{Success: true, Platform: req.Platform, Message: "published", Metadata: map[string]any{"remote_id": "server-remote"}}, nil
}

func (serverPublishProviderStub) Platforms() []string {
	return []string{"wechat"}
}

func (e *testJobExecutor) Execute(ctx context.Context, wf *domain.WorkflowDefinition, wc *domain.WorkflowContext) error {
	return e.engine.Execute(ctx, wf, wc)
}

func (r *serverWorkflowDefinitionRepo) Create(_ context.Context, workflow *domain.WorkflowDefinition) error {
	if workflow == nil {
		return nil
	}
	if r.stored == nil {
		r.stored = map[string]*domain.WorkflowDefinition{}
	}
	copyValue := *workflow
	r.stored[workflow.ID] = &copyValue
	return nil
}

func (r *serverWorkflowDefinitionRepo) Update(ctx context.Context, workflow *domain.WorkflowDefinition) error {
	return r.Create(ctx, workflow)
}

func (r *serverWorkflowDefinitionRepo) Upsert(ctx context.Context, workflow *domain.WorkflowDefinition) error {
	return r.Create(ctx, workflow)
}

func (r *serverWorkflowDefinitionRepo) GetByID(_ context.Context, id string) (*domain.WorkflowDefinition, error) {
	if r.stored == nil || r.stored[id] == nil {
		return nil, domain.NewNotFoundErr("workflow_definition", id)
	}
	copyValue := *r.stored[id]
	return &copyValue, nil
}

func (r *serverWorkflowDefinitionRepo) List(_ context.Context, limit int) ([]domain.WorkflowDefinition, error) {
	items := make([]domain.WorkflowDefinition, 0, len(r.stored))
	for _, workflow := range r.stored {
		if workflow == nil {
			continue
		}
		items = append(items, *workflow)
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (r *serverWorkflowDefinitionRepo) Delete(_ context.Context, id string) error {
	if r.stored != nil {
		delete(r.stored, id)
	}
	return nil
}

func (r *serverTemplateDefinitionRepo) Create(_ context.Context, template *domain.TemplateDefinition) error {
	if template == nil {
		return nil
	}
	if r.stored == nil {
		r.stored = map[string]*domain.TemplateDefinition{}
	}
	copyValue := *template
	r.stored[template.ID] = &copyValue
	return nil
}

func (r *serverTemplateDefinitionRepo) Update(ctx context.Context, template *domain.TemplateDefinition) error {
	return r.Create(ctx, template)
}

func (r *serverTemplateDefinitionRepo) Upsert(ctx context.Context, template *domain.TemplateDefinition) error {
	return r.Create(ctx, template)
}

func (r *serverTemplateDefinitionRepo) GetByID(_ context.Context, id string) (*domain.TemplateDefinition, error) {
	if r.stored == nil || r.stored[id] == nil {
		return nil, domain.NewNotFoundErr("template_definition", id)
	}
	copyValue := *r.stored[id]
	return &copyValue, nil
}

func (r *serverTemplateDefinitionRepo) List(_ context.Context, limit int) ([]domain.TemplateDefinition, error) {
	items := make([]domain.TemplateDefinition, 0, len(r.stored))
	for _, template := range r.stored {
		if template == nil {
			continue
		}
		items = append(items, *template)
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (r *serverTemplateDefinitionRepo) Delete(_ context.Context, id string) error {
	if r.stored != nil {
		delete(r.stored, id)
	}
	return nil
}

func TestHealthEndpoint(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["status"] != "healthy" {
		t.Errorf("expected status=healthy, got %q", resp["status"])
	}
}

func TestReadyEndpoint(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["status"] != "ready" {
		t.Errorf("expected status=ready, got %q", resp["status"])
	}
}

func TestReactFrontendServedFromRoot(t *testing.T) {
	t.Setenv("CONTENT_HUB_WEBAPP_DIST_DIR", "")
	useTestFrontendFS(t, fstest.MapFS{
		"index.html":      &fstest.MapFile{Data: []byte(`<!doctype html><html><head><title>Content Hub Control Plane</title><script type="module" src="/ui/assets/index.js"></script></head><body><div id="root"></div></body></html>`)},
		"assets/index.js": &fstest.MapFile{Data: []byte(`console.log("react-shell")`)},
	})

	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
	require.Contains(t, w.Body.String(), "<title>Content Hub Control Plane</title>")
	require.Contains(t, w.Body.String(), `<div id="root"></div>`)
	require.Contains(t, w.Body.String(), "/ui/assets/index.js")
	require.NotContains(t, w.Body.String(), "图工作流控制台")
}

func TestReactBuildAssetsServedFromRoot(t *testing.T) {
	t.Setenv("CONTENT_HUB_WEBAPP_DIST_DIR", "")
	useTestFrontendFS(t, fstest.MapFS{
		"index.html":       &fstest.MapFile{Data: []byte(`<!doctype html><html><body><div id="root"></div></body></html>`)},
		"assets/index.js":  &fstest.MapFile{Data: []byte(`console.log("asset")`)},
		"assets/index.css": &fstest.MapFile{Data: []byte(`body{margin:0}`)},
	})

	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/ui/assets/index.js", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "javascript")
	require.Contains(t, w.Body.String(), `console.log("asset")`)
}

func TestEmbeddedViteBuildServedFromRoot(t *testing.T) {
	t.Setenv("CONTENT_HUB_WEBAPP_DIST_DIR", "")
	frontendDistFSForTests = nil

	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
	require.Contains(t, w.Body.String(), `<div id="root"></div>`)
	require.Contains(t, w.Body.String(), `/ui/assets/`)
	require.NotContains(t, w.Body.String(), "图工作流控制台")
}

func TestReactFrontendFallbackReturnsShellForClientRoute(t *testing.T) {
	t.Setenv("CONTENT_HUB_WEBAPP_DIST_DIR", "")
	useTestFrontendFS(t, fstest.MapFS{
		"index.html":      &fstest.MapFile{Data: []byte(`<!doctype html><html><head><title>Content Hub Control Plane</title></head><body><div id="root"></div></body></html>`)},
		"assets/index.js": &fstest.MapFile{Data: []byte(`console.log("asset")`)},
	})

	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/workflow-templates/123", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
	require.Contains(t, w.Body.String(), "<title>Content Hub Control Plane</title>")
	require.Contains(t, w.Body.String(), `<div id="root"></div>`)
}

func TestAPIBackendRoutesAreNotSwallowedByFrontendFallback(t *testing.T) {
	t.Setenv("CONTENT_HUB_WEBAPP_DIST_DIR", "")
	useTestFrontendFS(t, fstest.MapFS{
		"index.html":      &fstest.MapFile{Data: []byte(`<!doctype html><html><head><title>Content Hub Control Plane</title></head><body><div id="root"></div></body></html>`)},
		"assets/index.js": &fstest.MapFile{Data: []byte(`console.log("asset")`)},
	})

	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/articles", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/json")
	require.Contains(t, w.Body.String(), `"data"`)
	require.NotContains(t, w.Body.String(), `<div id="root"></div>`)
}

func TestRetiredRSSPathsReturnHardNotFoundEvenWithFrontendFallback(t *testing.T) {
	t.Setenv("CONTENT_HUB_WEBAPP_DIST_DIR", "")
	useTestFrontendFS(t, fstest.MapFS{
		"index.html":      &fstest.MapFile{Data: []byte(`<!doctype html><html><head><title>Content Hub Control Plane</title></head><body><div id="root"></div></body></html>`)},
		"assets/index.js": &fstest.MapFile{Data: []byte(`console.log("asset")`)},
	})

	s, _ := newTestServer(t)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		code   int
	}{
		{name: "root get", method: http.MethodGet, path: "/rss", code: http.StatusNotFound},
		{name: "root head", method: http.MethodHead, path: "/rss", code: http.StatusNotFound},
		{name: "nested get", method: http.MethodGet, path: "/rss/subscriptions", code: http.StatusNotFound},
		{name: "nested head", method: http.MethodHead, path: "/rss/subscriptions", code: http.StatusNotFound},
		{name: "non-get remains missing", method: http.MethodPost, path: "/rss/subscriptions", code: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			s.engine.ServeHTTP(w, req)

			require.Equal(t, tc.code, w.Code)
			require.NotContains(t, w.Header().Get("Content-Type"), "text/html")
			require.NotContains(t, w.Body.String(), "<title>Content Hub Control Plane</title>")
			require.NotContains(t, w.Body.String(), `<div id="root"></div>`)
		})
	}
}

func TestAdminFrontendFallsBackToLegacyStaticShellWhenReactBuildIsUnavailable(t *testing.T) {
	t.Setenv("CONTENT_HUB_WEBAPP_DIST_DIR", filepath.Join(t.TempDir(), "missing-dist"))
	disableTestFrontendFS(t)

	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	require.Contains(t, body, "总览")
	require.Contains(t, body, "文章导入")
	require.Contains(t, body, "文章列表")
	require.Contains(t, body, "工作流控制")
	require.Contains(t, body, "配置管理")
	require.Contains(t, body, "工作流模板")
	require.Contains(t, body, "模板管理")
	require.Contains(t, body, "审计日志")
	require.Contains(t, body, "文件上传（.txt/.md/.json）")
	require.Contains(t, body, "粘贴全文")
	require.NotContains(t, body, "Source URL")
	require.Contains(t, body, "未处理")
	require.Contains(t, body, "处理中")
	require.Contains(t, body, "已处理")
	require.Contains(t, body, "再处理")
	require.Contains(t, body, "删除")
	require.Contains(t, body, "停止")
}

func TestCreateContent(t *testing.T) {
	s, _ := newTestServer(t)

	body := strings.NewReader(`{"title":"Test Article","body":"Hello World","format":"markdown"}`)
	req := httptest.NewRequest(http.MethodPost, "/content", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["title"] != "Test Article" {
		t.Errorf("expected title=Test Article, got %q", resp["title"])
	}
}

func TestCreateContentMissingTitle(t *testing.T) {
	s, _ := newTestServer(t)

	body := strings.NewReader(`{"body":"Hello World"}`)
	req := httptest.NewRequest(http.MethodPost, "/content", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRewriteRunsRouteIsRegistered(t *testing.T) {
	s, _ := newTestServer(t)

	body := strings.NewReader(`{"workspace_article_id":"article-1","title":"Source","target_type":"wechat-longform","source_profile":"sspai","version":"v1"}`)
	req := httptest.NewRequest(http.MethodPost, "/rewrite/runs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
	if !strings.Contains(w.Body.String(), "rewrite orchestrator is not configured") {
		t.Fatalf("expected rewrite route error, got %s", w.Body.String())
	}
}

func TestListContent(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/content", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, ok := resp["data"]; !ok {
		t.Error("expected 'data' key in response")
	}
}

func TestCreateAndGetDraft(t *testing.T) {
	s, _ := newTestServer(t)

	body := strings.NewReader(`{"template":"default"}`)
	req := httptest.NewRequest(http.MethodPost, "/drafts", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var draft map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &draft); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	id, _ := draft["id"].(string)

	req = httptest.NewRequest(http.MethodGet, "/drafts/"+id, nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestSubmitJob(t *testing.T) {
	s, _ := newTestServer(t)

	body := strings.NewReader(`{"topic":"test-topic"}`)
	req := httptest.NewRequest(http.MethodPost, "/jobs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var job map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if job["topic"] != "test-topic" {
		t.Errorf("expected topic=test-topic, got %q", job["topic"])
	}
}

func TestAutomationEndpointsExposeRunOnceAndDaemonOnly(t *testing.T) {
	s, _ := newTestServer(t)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/automation/run-once", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "run-once")

	req = httptest.NewRequest(http.MethodPost, "/automation/daemon", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "daemon")
	assert.Contains(t, w.Body.String(), "running")

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/automation/retry-failed"},
		{method: http.MethodGet, path: "/automation/status"},
		{method: http.MethodGet, path: "/automation/health"},
		{method: http.MethodPost, path: "/automation/stop"},
	} {
		req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		if tc.method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		w = httptest.NewRecorder()
		s.engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code, "%s %s should not be registered", tc.method, tc.path)
	}
}

func TestLegacyRSSRoutesAreNotRegisteredWithoutFrontendFallback(t *testing.T) {
	t.Setenv("CONTENT_HUB_WEBAPP_DIST_DIR", filepath.Join(t.TempDir(), "missing-dist"))
	disableTestFrontendFS(t)

	s, _ := newTestServer(t)

	rssRequests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/rss"},
		{method: http.MethodHead, path: "/rss"},
		{method: http.MethodPost, path: "/rss/subscriptions"},
		{method: http.MethodGet, path: "/rss/subscriptions"},
		{method: http.MethodHead, path: "/rss/subscriptions"},
		{method: http.MethodGet, path: "/rss/subscriptions/sub-1"},
		{method: http.MethodPut, path: "/rss/subscriptions/sub-1"},
		{method: http.MethodDelete, path: "/rss/subscriptions/sub-1"},
		{method: http.MethodPost, path: "/rss/subscriptions/sub-1/run"},
		{method: http.MethodPost, path: "/rss/run-all"},
		{method: http.MethodGet, path: "/rss/runs"},
		{method: http.MethodGet, path: "/rss/runs/run-1"},
		{method: http.MethodGet, path: "/rss/items"},
		{method: http.MethodGet, path: "/rss/items/item-1"},
	}

	for _, tc := range rssRequests {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		s.engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code, "%s %s should not be registered", tc.method, tc.path)
	}
}

func TestLegacyCollectorAndIngestionRoutesAreNotRegisteredWithoutFrontendFallback(t *testing.T) {
	t.Setenv("CONTENT_HUB_WEBAPP_DIST_DIR", filepath.Join(t.TempDir(), "missing-dist"))
	disableTestFrontendFS(t)

	s, _ := newTestServer(t)

	legacyRequests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/collector"},
		{method: http.MethodGet, path: "/collector/sources"},
		{method: http.MethodGet, path: "/ingestion"},
		{method: http.MethodPost, path: "/ingestion/import"},
	}

	for _, tc := range legacyRequests {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		s.engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code, "%s %s should not be registered", tc.method, tc.path)
	}
}

func TestTraceIDHeader(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	traceID := w.Header().Get("X-Trace-ID")
	if traceID == "" {
		t.Error("expected X-Trace-ID header to be set")
	}
}

func TestConfigEndpoint(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, ok := resp["llm"]; !ok {
		t.Error("expected 'llm' key in config response")
	}
	if _, ok := resp["workflow"]; !ok {
		t.Error("expected 'workflow' key in config response")
	}
}

func TestAPIRoutesAreRegistered(t *testing.T) {
	s, repos := newTestServer(t)

	workspace := domain.NewArticleWorkspaceRecord("article-api", "Title", "", domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/title"}, map[string]any{"source_body": "Body", "source_profile": "web-paste", "workflow_template_id": "wf-1", "workflow_template_version": "v1"})
	workspace.Status = domain.ArticleWorkspaceStatusRewriteFailed
	require.NoError(t, repos.Workspaces.Create(t.Context(), workspace))
	run := domain.NewRewritePipelineRun("profile-1", "v1", workspace.ID, "wechat-longform", "web-paste")
	run.ID = "run-api"
	run.Status = domain.RewriteRunFailed
	require.NoError(t, s.provider.RewriteRunRepo.Create(t.Context(), run))

	req := httptest.NewRequest(http.MethodGet, "/api/articles", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "data")

	req = httptest.NewRequest(http.MethodGet, "/api/articles/"+workspace.ID, nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), workspace.ID)

	req = httptest.NewRequest(http.MethodGet, "/api/articles/"+workspace.ID+"/stages", nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "stages")

	req = httptest.NewRequest(http.MethodPost, "/api/articles/"+workspace.ID+"/retry", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	storedWorkspace, err := repos.Workspaces.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	storedWorkspace.Status = domain.ArticleWorkspaceStatusRewriting
	now := time.Now().UTC()
	storedWorkspace.UpdatedAt = now
	require.NoError(t, repos.Workspaces.Update(t.Context(), storedWorkspace))
	workflowRun, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "wf-1", WorkflowVersion: "v1", EntryNodeID: "review", WorkspaceArticleID: workspace.ID})
	require.NoError(t, err)
	workflowRun.ID = "workflow-run-api"
	workflowRun.Status = domain.WorkflowRunRunning
	require.NoError(t, s.provider.WorkflowRunRepo.Create(t.Context(), workflowRun))

	req = httptest.NewRequest(http.MethodPost, "/api/articles/"+workspace.ID+"/stop", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	storedWorkspace, err = repos.Workspaces.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	storedWorkflowRun, err := s.provider.WorkflowRunRepo.GetByID(t.Context(), workflowRun.ID)
	require.NoError(t, err)
	storedWorkflowRun.Status = domain.WorkflowRunPaused
	require.NoError(t, s.provider.WorkflowRunRepo.Update(t.Context(), storedWorkflowRun))
	storedWorkspace.Status = domain.WorkflowRunPaused
	require.NoError(t, repos.Workspaces.Update(t.Context(), storedWorkspace))

	req = httptest.NewRequest(http.MethodPost, "/api/articles/"+workspace.ID+"/resume", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	storedWorkflowRun, err = s.provider.WorkflowRunRepo.GetByID(t.Context(), workflowRun.ID)
	require.NoError(t, err)
	storedWorkflowRun.Status = domain.WorkflowRunPaused
	require.NoError(t, s.provider.WorkflowRunRepo.Update(t.Context(), storedWorkflowRun))
	storedWorkspace, err = repos.Workspaces.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	storedWorkspace.Status = domain.WorkflowRunPaused
	require.NoError(t, repos.Workspaces.Update(t.Context(), storedWorkspace))

	req = httptest.NewRequest(http.MethodPost, "/api/articles/"+workspace.ID+"/resume", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	storedWorkspace, err = repos.Workspaces.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	storedWorkspace.Status = domain.ArticleWorkspaceStatusRendered
	require.NoError(t, repos.Workspaces.Update(t.Context(), storedWorkspace))

	req = httptest.NewRequest(http.MethodDelete, "/api/articles/"+workspace.ID, nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"default_target_type":"wechat-longform"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest(http.MethodPost, "/api/system/start", strings.NewReader(`{"concurrency_limit":2}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	require.NoError(t, repos.ControlStates.Upsert(t.Context(), domain.NewSystemControlState("local-admin")))
	state, err := repos.ControlStates.Get(t.Context())
	require.NoError(t, err)
	state.State = domain.SystemStateRunning
	state.Reason = "started"
	state.Metadata = map[string]any{"concurrency_limit": 2}
	require.NoError(t, repos.ControlStates.Upsert(t.Context(), state))

	req = httptest.NewRequest(http.MethodPost, "/api/system/pause", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest(http.MethodPost, "/api/system/resume", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "state")

	req = httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "data")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "audit.md")
	require.NoError(t, err)
	_, err = part.Write([]byte("# Audit\n\nBody"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req = httptest.NewRequest(http.MethodPost, "/api/intake/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	auditLog, err := repos.AuditLogs.List(t.Context(), 10)
	require.NoError(t, err)
	require.NotEmpty(t, auditLog)

	req = httptest.NewRequest(http.MethodGet, "/api/audit/"+auditLog[0].ID, nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), auditLog[0].ID)
}

func TestNewServerWithWebControlDependenciesSucceeds(t *testing.T) {
	s, _ := newTestServer(t)
	require.NotNil(t, s)
	require.NotNil(t, s.engine)
}

func TestDeleteRoutesAreRegisteredForWorkflowsAndTemplates(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/workflows/nonexistent", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.NotEqual(t, http.StatusNotFound, w.Code)
	require.NotEqual(t, http.StatusMethodNotAllowed, w.Code)

	req = httptest.NewRequest(http.MethodDelete, "/api/templates/nonexistent", nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.NotEqual(t, http.StatusNotFound, w.Code)
	require.NotEqual(t, http.StatusMethodNotAllowed, w.Code)
}

func TestNewServerMissingWebControlDependenciesFailsExplicitly(t *testing.T) {
	cfg := config.DefaultConfig()
	provider := &Provider{}

	require.PanicsWithValue(t, "http server provider validation failed: missing ConfigLoader, ContentSvc, TemplateSvc, DraftSvc, FormattingSvc, AutomationSvc, WorkspaceSvc, JobSvc, ReviewSvc, PublishSvc, WorkflowEngine, WebControlRuntime, RewriteRunRepo, RewriteStageRepo, AuditLogRepo", func() {
		NewServer(cfg, provider)
	})
}

type failingListener struct{}

func (failingListener) Accept() (net.Conn, error) {
	return nil, errors.New("listen failed")
}

func (failingListener) Close() error { return nil }

func (failingListener) Addr() net.Addr { return &net.TCPAddr{} }

func TestServeReturnsImmediateListenerFailure(t *testing.T) {
	s, _ := newTestServer(t)
	err := s.serveWithListener(failingListener{}, make(chan os.Signal, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listen failed")
}

func TestServeShutsDownWhenSignalReceived(t *testing.T) {
	s, _ := newTestServer(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	quit := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() { done <- s.serveWithListener(listener, quit) }()
	time.Sleep(50 * time.Millisecond)
	quit <- os.Interrupt
	require.NoError(t, <-done)
}

func TestConfigEndpointDoesNotMutateLoaderState(t *testing.T) {
	s, _ := newTestServer(t)
	original := s.provider.ConfigLoader.Current()

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	current := s.provider.ConfigLoader.Current()
	assert.Equal(t, original.HTTP.Port, current.HTTP.Port)
	assert.Equal(t, original.LLM.Provider, current.LLM.Provider)
}

func TestCreateTemplate(t *testing.T) {
	s, _ := newTestServer(t)

	body := strings.NewReader(`{"category":"test","name":"tpl1","content":"Hello {{.Name}}"}`)
	req := httptest.NewRequest(http.MethodPost, "/templates", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["category"] != "test" {
		t.Errorf("expected category=test, got %q", resp["category"])
	}
}

func TestListTemplateCategories(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/templates/categories", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestDeleteContent(t *testing.T) {
	s, _ := newTestServer(t)

	createBody := strings.NewReader(`{"title":"To Delete","body":"content","format":"markdown"}`)
	req := httptest.NewRequest(http.MethodPost, "/content", createBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	id, _ := created["id"].(string)

	req = httptest.NewRequest(http.MethodDelete, "/content?id="+id, nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp["deleted"] {
		t.Error("expected deleted=true")
	}
}

func TestNotFoundContent(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/content/detail?id=nonexistent", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}
