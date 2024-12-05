package ratelimiter

import (
	"net/http",
	"github.com/gin-gonic/gin",
	"context"
)

func GinRateLimiterMiddleware(rl *Ratelimiter)  gin.HandlerFunc{
	return func(c *gin.context) {
		if !rl.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too Many Requests"})
            c.Abort()
            return	
		}
		c.next()
	}
}
