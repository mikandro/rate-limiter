package ratelimit

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GinRateLimiter creates a Gin middleware with the given configuration.
// It supports both single rate limiter and per-key rate limiting.
//
// Example with single rate limiter:
//
//	rl, _ := ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
//	    Capacity: 100,
//	    Rate:     time.Second,
//	})
//	r.Use(ratelimit.GinRateLimiter(ratelimit.MiddlewareConfig{
//	    RateLimiter: rl,
//	}))
//
// Example with per-IP rate limiting:
//
//	r.Use(ratelimit.GinRateLimiter(ratelimit.MiddlewareConfig{
//	    RateLimiterFactory: func(key string) (ratelimit.RateLimiter, error) {
//	        return ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
//	            Capacity: 10,
//	            Rate:     time.Second,
//	        })
//	    },
//	    KeyExtractor: ratelimit.ExtractIPAddress,
//	}))
func GinRateLimiter(config MiddlewareConfig) gin.HandlerFunc {
	// Validate configuration
	if err := config.Validate(); err != nil {
		panic(err)
	}

	return func(c *gin.Context) {
		// Check if we should skip rate limiting for this request
		if config.SkipFunc != nil && config.SkipFunc(c.Request) {
			c.Next()
			return
		}

		var limiter RateLimiter
		var err error

		// Get the appropriate rate limiter
		if config.RateLimiter != nil {
			limiter = config.RateLimiter
		} else {
			// Extract key and get per-key limiter
			key := config.KeyExtractor(c.Request)
			limiter, err = config.Store.Get(key)
			if err != nil {
				// On error, return internal server error
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal Server Error",
				})
				c.Abort()
				return
			}
		}

		// Check rate limit
		allowed := limiter.Allow()

		// Set rate limit headers if enabled
		if config.IncludeHeaders || (!allowed) {
			headers := GetRateLimitHeaders(limiter, time.Second)
			SetRateLimitHeaders(c.Writer, headers)
		}

		if !allowed {
			// Rate limit exceeded - use custom error handler
			retryAfter := CalculateRetryAfter(limiter, time.Second)

			// Create a response writer wrapper to capture the status code
			config.ErrorHandler(c.Writer, c.Request, retryAfter)
			c.Abort()
			return
		}

		// Request allowed, proceed
		c.Next()
	}
}

// GinRateLimiterSimple creates a simple Gin middleware using a single rate limiter.
// This is a convenience function for basic use cases.
//
// Example:
//
//	rl, _ := ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
//	    Capacity: 100,
//	    Rate:     time.Second,
//	})
//	r.Use(ratelimit.GinRateLimiterSimple(rl))
func GinRateLimiterSimple(limiter RateLimiter) gin.HandlerFunc {
	return GinRateLimiter(MiddlewareConfig{
		RateLimiter:    limiter,
		IncludeHeaders: true,
	})
}

// GinRateLimiterPerIP creates a Gin middleware that rate limits per IP address.
// Each IP address gets its own rate limiter with the specified capacity and rate.
//
// Example:
//
//	r.Use(ratelimit.GinRateLimiterPerIP(10, time.Second))
func GinRateLimiterPerIP(capacity int, rate time.Duration) gin.HandlerFunc {
	return GinRateLimiter(MiddlewareConfig{
		RateLimiterFactory: func(key string) (RateLimiter, error) {
			return NewTokenBucketRateLimiter(Options{
				Capacity: capacity,
				Rate:     rate,
			})
		},
		KeyExtractor:   ExtractIPAddress,
		IncludeHeaders: true,
	})
}

// GinRateLimiterPerUser creates a Gin middleware that rate limits per user.
// It extracts the user identifier from the specified header.
//
// Example:
//
//	r.Use(ratelimit.GinRateLimiterPerUser("X-User-ID", 100, time.Minute))
func GinRateLimiterPerUser(headerName string, capacity int, rate time.Duration) gin.HandlerFunc {
	return GinRateLimiter(MiddlewareConfig{
		RateLimiterFactory: func(key string) (RateLimiter, error) {
			return NewTokenBucketRateLimiter(Options{
				Capacity: capacity,
				Rate:     rate,
			})
		},
		KeyExtractor:   ExtractUserID(headerName),
		IncludeHeaders: true,
	})
}

// GinRateLimiterPerAPIKey creates a Gin middleware that rate limits per API key.
// It extracts the API key from the specified header.
//
// Example:
//
//	r.Use(ratelimit.GinRateLimiterPerAPIKey("X-API-Key", 1000, time.Hour))
func GinRateLimiterPerAPIKey(headerName string, capacity int, rate time.Duration) gin.HandlerFunc {
	return GinRateLimiter(MiddlewareConfig{
		RateLimiterFactory: func(key string) (RateLimiter, error) {
			return NewTokenBucketRateLimiter(Options{
				Capacity: capacity,
				Rate:     rate,
			})
		},
		KeyExtractor:   ExtractAPIKey(headerName),
		IncludeHeaders: true,
	})
}

// GinRateLimiterMiddleware is the legacy middleware function for backward compatibility.
// It's recommended to use GinRateLimiterSimple or GinRateLimiter for new code.
//
// Deprecated: Use GinRateLimiterSimple instead.
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
