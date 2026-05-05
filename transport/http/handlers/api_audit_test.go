package handlers

import (
	"content-hub/service"
	"content-hub/transport/http/middleware"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIAuditListReturnsLogs(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &stubAuditLogRepo{}
	svc := service.NewAuditLogService(repo)
	first, err := svc.Create(t.Context(), service.AuditLogCreateInput{Actor: "local-admin", Action: "upload_article", Result: "success"})
	require.NoError(t, err)
	_, err = svc.Create(t.Context(), service.AuditLogCreateInput{Actor: "local-admin", Action: "pause_system", Result: "success"})
	require.NoError(t, err)

	handler := NewAPIAuditHandler(svc, repo)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/audit", handler.List)
	router.GET("/api/audit/:id", handler.Get)

	listReq := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)

	require.Equal(t, http.StatusOK, listW.Code)
	var listResp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &listResp))
	require.Len(t, listResp.Data, 2)

	getReq := httptest.NewRequest(http.MethodGet, "/api/audit/"+first.ID, nil)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)
	require.Contains(t, getW.Body.String(), first.ID)
}
