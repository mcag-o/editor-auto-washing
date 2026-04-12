package middleware

import (
	"content-hub/domain"
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const TraceIDHeader = "X-Trace-ID"
const TraceIDContextKey = "trace_id"

func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(TraceIDHeader)
		if traceID == "" {
			traceID = uuid.New().String()
		}
		c.Set(TraceIDContextKey, traceID)
		c.Header(TraceIDHeader, traceID)
		c.Request = c.Request.WithContext(withTraceID(c.Request.Context(), traceID))
		c.Next()
	}
}

func withTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDContextKey, traceID)
}

func GetTraceID(ctx context.Context) string {
	if id, ok := ctx.Value(TraceIDContextKey).(string); ok {
		return id
	}
	return ""
}

func GetTraceIDFromGin(c *gin.Context) string {
	if id, exists := c.Get(TraceIDContextKey); exists {
		return id.(string)
	}
	return ""
}

func TraceIDError(err error, c *gin.Context) error {
	if appErr, ok := err.(*domain.AppError); ok {
		appErr.TraceID = GetTraceIDFromGin(c)
		return appErr
	}
	return err
}
