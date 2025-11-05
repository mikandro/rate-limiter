package ratelimit

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func GinRateLimiterMiddleware(rl RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too Many Requests"})
			c.Abort()
			return
		}
		c.Next()
	}
}
