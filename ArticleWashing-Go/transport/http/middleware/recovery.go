package middleware

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, err any) {
		traceID := GetTraceIDFromGin(c)
		stack := make([]byte, 4096)
		stack = stack[:runtime.Stack(stack, false)]

		fmt.Printf("[PANIC] trace_id=%s error=%v\n%s\n", traceID, err, stack)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "internal server error",
			"trace_id": traceID,
		})
	})
}
