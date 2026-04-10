package handlers

import (
	"content-hub/domain"
	"content-hub/infra/memory"
	"content-hub/service"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobCancelCancelsPendingJob(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := memory.NewProvider()
	jobSvc := service.NewJobService(provider.JobRepo(), provider.JobEventRepo(), &jobHandlerExecutorStub{})
	job, err := jobSvc.Submit(t.Context(), "cancel-test")
	require.NoError(t, err)
	handler := NewJobHandler(jobSvc)
	router := gin.New()
	router.POST("/jobs/:id/cancel", handler.Cancel)

	req := httptest.NewRequest(http.MethodPost, "/jobs/"+job.ID+"/cancel", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), `"status":"cancelled"`)
	assert.Contains(t, resp.Body.String(), "job cancelled")

	events, err := jobSvc.GetEvents(t.Context(), job.ID)
	require.NoError(t, err)
	require.NotEmpty(t, events)
	assert.Equal(t, "cancelled", events[len(events)-1].Status)
}

type jobHandlerExecutorStub struct{}

func (jobHandlerExecutorStub) Execute(ctx context.Context, wf *domain.WorkflowDefinition, wc *domain.WorkflowContext) error {
	return nil
}
