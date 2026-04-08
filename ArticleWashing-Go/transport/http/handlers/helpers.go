package handlers

import (
	"content-hub/domain"
	"errors"

	"github.com/gin-gonic/gin"
)

func HandleError(c *gin.Context, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.HTTPStatus(), gin.H{
			"error":    appErr.Message,
			"code":     appErr.Code,
			"trace_id": appErr.TraceID,
		})
		return
	}
	c.JSON(500, gin.H{"error": err.Error()})
}
