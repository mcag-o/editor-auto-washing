package http

import (
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
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) (*Server, *memory.Provider) {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 0

	memProvider := memory.NewProvider()

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
	loader := config.NewLoader("")
	loader.SetCurrent(cfg)

	provider := &Provider{
		ContentSvc:     contentSvc,
		TemplateSvc:    templateSvc,
		DraftSvc:       draftSvc,
		FormattingSvc:  formattingSvc,
		IngestionSvc:   ingestionSvc,
		AutomationSvc:  automationSvc,
		WorkspaceSvc:   workspaceSvc,
		JobSvc:         jobSvc,
		ReviewSvc:      reviewSvc,
		PublishSvc:     publishSvc,
		WorkflowEngine: workflowEngine,
		ConfigLoader:   loader,
		WorkspaceRoot:  workspaceRoot,
	}

	return NewServer(cfg, provider), memProvider
}

type testJobExecutor struct {
	engine *service.WorkflowEngine
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

	body := strings.NewReader(`{"workspace_article_id":"article-1","collector_article_id":"collector-1","title":"Source","target_type":"wechat-longform","source_profile":"sspai","version":"v1"}`)
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

func TestAutomationEndpointsExposeRunOnceDaemonStatusHealthRetryFailedAndStop(t *testing.T) {
	s, _ := newTestServer(t)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/automation/run-once", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "run-once")

	req = httptest.NewRequest(http.MethodPost, "/automation/retry-failed", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest(http.MethodPost, "/automation/daemon", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "daemon")
	assert.Contains(t, w.Body.String(), "running")

	req = httptest.NewRequest(http.MethodGet, "/automation/status", nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "state")

	req = httptest.NewRequest(http.MethodGet, "/automation/health", nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "status")

	req = httptest.NewRequest(http.MethodPost, "/automation/stop", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "stopped")
}

func TestLegacyRSSRoutesRemovedFromActiveRuntime(t *testing.T) {
	s, _ := newTestServer(t)

	rssRequests := []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/rss/subscriptions"},
		{method: http.MethodGet, path: "/rss/subscriptions"},
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

	legacyRequests := []struct {
		method string
		path   string
	}{
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
