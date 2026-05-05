package handlers

import (
	"bytes"
	"content-hub/domain"
	"content-hub/service"
	"content-hub/transport/http/middleware"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIIntakeUploadCreatesPendingSourceDocument(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &stubSourceDocumentRepo{}
	audit := &stubAuditLogRepo{}
	handler := NewAPIIntakeHandler(service.NewWebIntakeService(repo, audit))

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
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "pending", resp["status"])
	require.Equal(t, "upload", resp["source_type"])
	require.Equal(t, "Title", resp["title"])
	require.Len(t, repo.created, 1)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "web_intake.create_from_upload", audit.logs[0].Action)
}

func TestAPIIntakePasteCreatesPendingSourceDocument(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &stubSourceDocumentRepo{}
	audit := &stubAuditLogRepo{}
	handler := NewAPIIntakeHandler(service.NewWebIntakeService(repo, audit))

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/intake/paste", handler.Paste)

	req := httptest.NewRequest(http.MethodPost, "/api/intake/paste", bytes.NewBufferString(`{"title":"Pasted","body":"Body"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp domain.SourceDocument
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "paste", resp.SourceType)
	require.Equal(t, domain.SourceDocumentStatusPending, resp.Status)
	require.Equal(t, "Pasted", resp.Title)
	require.Len(t, repo.created, 1)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "web_intake.create_from_paste", audit.logs[0].Action)
}

func TestAPIIntakePasteReturnsValidationError(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := NewAPIIntakeHandler(service.NewWebIntakeService(&stubSourceDocumentRepo{}, &stubAuditLogRepo{}))

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
