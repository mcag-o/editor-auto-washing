package handlers

import (
	"bytes"
	"content-hub/domain"
	"content-hub/infra/memory"
	"content-hub/service"
	"content-hub/transport/http/middleware"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type browserIntakeResponse struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Body     string `json:"body"`
	Metadata struct {
		SourceType            string `json:"source_type"`
		OriginalURL           string `json:"original_url"`
		TargetType            string `json:"target_type"`
		SourceProfile         string `json:"source_profile"`
		RenderPlatform        string `json:"render_platform"`
		RewriteProfileVersion string `json:"rewrite_profile_version"`
	} `json:"metadata"`
}

func TestAPIIntakeUploadCreatesWorkspaceBackedIntakeItem(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &serviceStubWebIntakeWorkspaceRepo{}
	audit := &stubAuditLogRepo{}
	handler := NewAPIIntakeHandler(service.NewWebIntakeService(workspaceRepo, audit))

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/intake/upload", handler.Upload)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "article.md")
	require.NoError(t, err)
	_, err = part.Write([]byte("# Title\n\nBody"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/intake/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp browserIntakeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.ID)
	require.Equal(t, domain.ArticleWorkspaceStatusImported, resp.Status)
	require.Equal(t, "Title", resp.Title)
	require.Equal(t, "# Title\n\nBody", resp.Body)
	require.Equal(t, "upload", resp.Metadata.SourceType)
	require.Equal(t, "browser://upload/article.md", resp.Metadata.OriginalURL)
	require.Equal(t, "wechat-longform", resp.Metadata.TargetType)
	require.Equal(t, "web-upload", resp.Metadata.SourceProfile)
	require.Equal(t, "wechat", resp.Metadata.RenderPlatform)
	require.Equal(t, "v1", resp.Metadata.RewriteProfileVersion)
	assertIntakeResponseDoesNotExposeLegacyFields(t, w.Body.Bytes())
	require.Len(t, workspaceRepo.created, 1)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "web_intake.create_from_upload", audit.logs[0].Action)
}

func TestAPIIntakePasteCreatesWorkspaceBackedIntakeItem(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &serviceStubWebIntakeWorkspaceRepo{}
	audit := &stubAuditLogRepo{}
	handler := NewAPIIntakeHandler(service.NewWebIntakeService(workspaceRepo, audit))

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/intake/paste", handler.Paste)

	req := httptest.NewRequest(http.MethodPost, "/api/intake/paste", bytes.NewBufferString(`{"title":"Pasted","body":"Body"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp browserIntakeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.ID)
	require.Equal(t, domain.ArticleWorkspaceStatusImported, resp.Status)
	require.Equal(t, "Pasted", resp.Title)
	require.Equal(t, "Body", resp.Body)
	require.Equal(t, "paste", resp.Metadata.SourceType)
	require.Equal(t, "browser://paste/Pasted", resp.Metadata.OriginalURL)
	require.Equal(t, "wechat-longform", resp.Metadata.TargetType)
	require.Equal(t, "web-paste", resp.Metadata.SourceProfile)
	require.Equal(t, "wechat", resp.Metadata.RenderPlatform)
	require.Equal(t, "v1", resp.Metadata.RewriteProfileVersion)
	assertIntakeResponseDoesNotExposeLegacyFields(t, w.Body.Bytes())
	require.Len(t, workspaceRepo.created, 1)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "web_intake.create_from_paste", audit.logs[0].Action)
}

func TestAPIIntakeArticleCreatesWorkspaceAndQueuesJob(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &serviceStubWebIntakeWorkspaceRepo{}
	audit := &stubAuditLogRepo{}
	provider := memory.NewProvider()
	jobSvc := service.NewJobService(provider.JobRepo(), provider.JobEventRepo(), nil)
	handler := NewAPIIntakeHandler(service.NewWebIntakeService(workspaceRepo, audit), jobSvc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/intake/articles", handler.Article)

	req := httptest.NewRequest(http.MethodPost, "/api/intake/articles", bytes.NewBufferString(`{"source_type":"xiaohongshu","title":"Article","body":"Body","original_url":"https://example.com/a/1"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp struct {
		WorkspaceArticle browserIntakeResponse `json:"workspace_article"`
		Job              domain.JobRun         `json:"job"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.WorkspaceArticle.ID)
	require.NotEmpty(t, resp.Job.ID)
	require.Equal(t, service.ExternalIntakeProcessTopic, resp.Job.Topic)
	require.NotNil(t, resp.Job.ArtifactPath)
	require.Equal(t, resp.WorkspaceArticle.ID, *resp.Job.ArtifactPath)
	require.Len(t, workspaceRepo.created, 1)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "api_intake.create_external_article", audit.logs[0].Action)
}

func TestAPIIntakeFileCreatesWorkspaceAndQueuesJob(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &serviceStubWebIntakeWorkspaceRepo{}
	audit := &stubAuditLogRepo{}
	provider := memory.NewProvider()
	jobSvc := service.NewJobService(provider.JobRepo(), provider.JobEventRepo(), nil)
	handler := NewAPIIntakeHandler(service.NewWebIntakeService(workspaceRepo, audit), jobSvc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/intake/files", handler.File)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "article.md")
	require.NoError(t, err)
	_, err = part.Write([]byte("# Title\n\nBody"))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("metadata", `{"external_id":"file-1"}`))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/intake/files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp struct {
		WorkspaceArticle browserIntakeResponse `json:"workspace_article"`
		Job              domain.JobRun         `json:"job"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "Title", resp.WorkspaceArticle.Title)
	require.Equal(t, service.ExternalIntakeProcessTopic, resp.Job.Topic)
	require.NotNil(t, resp.Job.ArtifactPath)
	require.Equal(t, resp.WorkspaceArticle.ID, *resp.Job.ArtifactPath)
	require.Equal(t, "file-1", workspaceRepo.created[0].Metadata["external_id"])
}

func assertIntakeResponseDoesNotExposeLegacyFields(t *testing.T, body []byte) {
	t.Helper()

	var raw map[string]any
	require.NoError(t, json.Unmarshal(body, &raw))
	require.NotContains(t, raw, "source")
	require.NotContains(t, raw, "source_body")
	require.NotContains(t, raw, "original_path")
	require.NotContains(t, raw, "original_filename")
	require.NotContains(t, raw, "file_type")
	require.NotContains(t, raw, "status_history")
	require.NotContains(t, raw, "lifecycle_history")
	require.NotContains(t, raw, "created_at")
	require.NotContains(t, raw, "updated_at")
	require.NotContains(t, raw, "published_at")
	require.NotContains(t, raw, "notes")
	metadata, ok := raw["metadata"].(map[string]any)
	require.True(t, ok)
	require.NotContains(t, metadata, "source_body")
}

func TestAPIIntakePasteReturnsValidationError(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := NewAPIIntakeHandler(service.NewWebIntakeService(&serviceStubWebIntakeWorkspaceRepo{}, &stubAuditLogRepo{}))

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/intake/paste", handler.Paste)

	req := httptest.NewRequest(http.MethodPost, "/api/intake/paste", bytes.NewBufferString(`{"title":"","body":"Body"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "title is required")
}

type serviceStubWebIntakeWorkspaceRepo struct {
	created []*domain.ArticleWorkspaceRecord
}

func (r *serviceStubWebIntakeWorkspaceRepo) Create(_ context.Context, record *domain.ArticleWorkspaceRecord) error {
	copyValue := *record
	r.created = append(r.created, &copyValue)
	return nil
}

func (r *serviceStubWebIntakeWorkspaceRepo) Update(_ context.Context, record *domain.ArticleWorkspaceRecord) error {
	for i, item := range r.created {
		if item.ID == record.ID {
			copyValue := *record
			r.created[i] = &copyValue
			return nil
		}
	}
	return domain.NewNotFoundErr("workspace_article", record.ID)
}

func (r *serviceStubWebIntakeWorkspaceRepo) GetByID(_ context.Context, id string) (*domain.ArticleWorkspaceRecord, error) {
	for _, item := range r.created {
		if item.ID == id {
			copyValue := *item
			return &copyValue, nil
		}
	}
	return nil, domain.NewNotFoundErr("workspace_article", id)
}

func (r *serviceStubWebIntakeWorkspaceRepo) List(context.Context, *string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (r *serviceStubWebIntakeWorkspaceRepo) ListByIngestionID(context.Context, string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (r *serviceStubWebIntakeWorkspaceRepo) TransitionStatus(context.Context, string, string, string) error {
	return nil
}

func (r *serviceStubWebIntakeWorkspaceRepo) Delete(context.Context, string) error {
	return nil
}
