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
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) (*Server, *testWebControlRepos) {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 0

	memProvider := memory.NewProvider()
	sourceDocumentRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	businessConfigRepo := &stubBusinessConfigRepo{}
	systemControlStateRepo := &stubSystemControlStateRepo{}
	auditLogRepo := &stubAuditLogRepo{}
	webRepos := &testWebControlRepos{
		SourceDocuments: sourceDocumentRepo,
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
		SourceDocumentRepo:     sourceDocumentRepo,
		BusinessConfigRepo:     businessConfigRepo,
		SystemControlStateRepo: systemControlStateRepo,
		AuditLogRepo:           auditLogRepo,
	}
	webControlRuntime, err := service.BuildWebControlRuntime(runtimeRepos)
	require.NoError(t, err)
	loader := config.NewLoader("")
	loader.SetCurrent(cfg)

	provider := &Provider{
		ContentSvc:         contentSvc,
		TemplateSvc:        templateSvc,
		DraftSvc:           draftSvc,
		FormattingSvc:      formattingSvc,
		AutomationSvc:      automationSvc,
		WorkspaceSvc:       workspaceSvc,
		JobSvc:             jobSvc,
		ReviewSvc:          reviewSvc,
		PublishSvc:         publishSvc,
		WebControlRuntime:  webControlRuntime,
		WorkflowEngine:     workflowEngine,
		ConfigLoader:       loader,
		SourceDocumentRepo: sourceDocumentRepo,
		RewriteRunRepo:     memProvider.RewritePipelineRunRepo(),
		RewriteStageRepo:   memProvider.RewriteStageRunRepo(),
		AuditLogRepo:       auditLogRepo,
		WorkspaceRoot:      workspaceRoot,
	}

	return NewServer(cfg, provider), webRepos
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

func TestAdminFrontendServedFromRoot(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
	require.Contains(t, w.Body.String(), "<title>Content Hub Admin</title>")
	require.Contains(t, w.Body.String(), "图工作流控制台")
	require.Contains(t, w.Body.String(), "总览")
	require.Contains(t, w.Body.String(), "/app.js")
	require.Contains(t, w.Body.String(), "/styles.css")
}

func TestAdminFrontendContainsChineseWorkflowAndTemplateSections(t *testing.T) {
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

func TestAPIRoutesAreRegistered(t *testing.T) {
	s, repos := newTestServer(t)

	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-api")
	doc.Status = domain.SourceDocumentStatusFailed
	require.NoError(t, repos.SourceDocuments.Create(t.Context(), doc))

	req := httptest.NewRequest(http.MethodGet, "/api/articles", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "data")

	req = httptest.NewRequest(http.MethodGet, "/api/articles/"+doc.ID, nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), doc.ID)

	req = httptest.NewRequest(http.MethodGet, "/api/articles/"+doc.ID+"/stages", nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "stages")

	req = httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/retry", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	doc.Status = domain.SourceDocumentStatusProcessing
	now := time.Now().UTC()
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.ProcessingStartedAt = &now
	require.NoError(t, repos.SourceDocuments.Update(t.Context(), doc))

	req = httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/stop", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	storedDoc, err := repos.SourceDocuments.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	storedDoc.Status = "paused"
	storedDoc.ClaimedBy = ""
	storedDoc.ClaimedAt = nil
	storedDoc.ProcessingStartedAt = nil
	require.NoError(t, repos.SourceDocuments.Update(t.Context(), storedDoc))

	req = httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/resume", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	storedDoc, err = repos.SourceDocuments.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	storedDoc.Status = "paused"
	activeNow := time.Now().UTC()
	storedDoc.ClaimedBy = "worker-1"
	storedDoc.ClaimedAt = &activeNow
	storedDoc.ProcessingStartedAt = &activeNow
	require.NoError(t, repos.SourceDocuments.Update(t.Context(), storedDoc))

	req = httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/resume", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	storedDoc, err = repos.SourceDocuments.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	storedDoc.Status = domain.SourceDocumentStatusCompleted
	storedDoc.ClaimedBy = ""
	storedDoc.ClaimedAt = nil
	storedDoc.ProcessingStartedAt = nil
	require.NoError(t, repos.SourceDocuments.Update(t.Context(), storedDoc))

	req = httptest.NewRequest(http.MethodDelete, "/api/articles/"+doc.ID, nil)
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

func TestAPIDeleteRoutesAreRegisteredForWorkflowsAndTemplates(t *testing.T) {
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

	require.PanicsWithValue(t, "http server provider validation failed: missing ConfigLoader, ContentSvc, TemplateSvc, DraftSvc, FormattingSvc, AutomationSvc, WorkspaceSvc, JobSvc, ReviewSvc, PublishSvc, WorkflowEngine, WebControlRuntime, SourceDocumentRepo, RewriteRunRepo, RewriteStageRepo, AuditLogRepo", func() {
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
