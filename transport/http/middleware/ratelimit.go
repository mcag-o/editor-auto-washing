package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	limiter *rate.Limiter
}

func NewRateLimiter(rps int, burst int) *RateLimiter {
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
	}
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.limiter.Allow() {
			traceID := GetTraceIDFromGin(c)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":    "rate limit exceeded",
				"trace_id": traceID,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func RateLimit(rps, burst int) gin.HandlerFunc {
	rl := NewRateLimiter(rps, burst)
	return rl.Middleware()
}
