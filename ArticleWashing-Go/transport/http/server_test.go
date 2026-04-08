package http

import (
	"content-hub/domain"
	"content-hub/infra/config"
	"content-hub/infra/memory"
	"content-hub/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	workflowEngine := service.NewWorkflowEngine()
	jobSvc := service.NewJobService(
		memProvider.JobRepo(),
		memProvider.JobEventRepo(),
		&testJobExecutor{engine: workflowEngine},
	)

	loader := config.NewLoader("")
	_ = loader.Save(cfg)

	provider := &Provider{
		ContentSvc:     contentSvc,
		TemplateSvc:    templateSvc,
		DraftSvc:       draftSvc,
		JobSvc:         jobSvc,
		WorkflowEngine: workflowEngine,
		ConfigLoader:   loader,
	}

	return NewServer(cfg, provider), memProvider
}

type testJobExecutor struct {
	engine *service.WorkflowEngine
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
