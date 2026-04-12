package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTraceIDGeneratesID(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(TraceID())
	engine.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	traceID := w.Header().Get("X-Trace-ID")
	if traceID == "" {
		t.Error("expected X-Trace-ID header to be set")
	}
}

func TestTraceIDPreservesProvided(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(TraceID())
	engine.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Trace-ID", "my-custom-id")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	traceID := w.Header().Get("X-Trace-ID")
	if traceID != "my-custom-id" {
		t.Errorf("expected X-Trace-ID=my-custom-id, got %q", traceID)
	}
}

func TestGetTraceIDFromGin(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	var capturedID string
	engine.Use(TraceID())
	engine.GET("/test", func(c *gin.Context) {
		capturedID = GetTraceIDFromGin(c)
		c.String(200, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if capturedID == "" {
		t.Error("expected trace ID to be captured")
	}
}
