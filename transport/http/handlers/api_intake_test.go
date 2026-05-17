package handlers

import (
	"bytes"
	"content-hub/domain"
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
	var resp domain.ArticleWorkspaceRecord
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, domain.ArticleWorkspaceStatusImported, resp.Status)
	require.Equal(t, "upload", resp.Source.SourceType)
	require.Equal(t, "Title", resp.Title)
	require.Equal(t, "# Title\n\nBody", resp.Metadata["source_body"])
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
	var resp domain.ArticleWorkspaceRecord
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "paste", resp.Source.SourceType)
	require.Equal(t, domain.ArticleWorkspaceStatusImported, resp.Status)
	require.Equal(t, "Pasted", resp.Title)
	require.Equal(t, "Body", resp.Metadata["source_body"])
	require.Len(t, workspaceRepo.created, 1)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "web_intake.create_from_paste", audit.logs[0].Action)
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
