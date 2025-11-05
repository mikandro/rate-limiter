package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mikandro/rate-limiter"
)

func main() {
	// Create a Gin router
	r := gin.Default()

	// Example 1: Per-IP rate limiting
	// Each IP address gets its own rate limiter
	log.Println("Setting up per-IP rate limiting: 5 requests per second per IP")
	r.Use(ratelimit.GinRateLimiterPerIP(5, time.Second))

	r.GET("/api/data", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "This endpoint is rate limited per IP",
			"data":    []string{"item1", "item2", "item3"},
		})
	})

	// Example 2: Per-user rate limiting with custom configuration
	log.Println("Setting up per-user rate limiting for /api/user routes")
	userGroup := r.Group("/api/user")
	userGroup.Use(ratelimit.GinRateLimiter(ratelimit.MiddlewareConfig{
		RateLimiterFactory: func(key string) (ratelimit.RateLimiter, error) {
			// Each user gets 100 requests per minute
			return ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
				Capacity: 100,
				Rate:     time.Minute / 100, // One token every 600ms
			})
		},
		KeyExtractor: ratelimit.ExtractUserID("X-User-ID"),
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, retryAfter time.Duration) {
			// Custom error response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate_limit_exceeded","message":"You have exceeded your rate limit. Please slow down."}`))
		},
		IncludeHeaders: true,
	}))

	userGroup.GET("/profile", func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		c.JSON(200, gin.H{
			"user_id": userID,
			"message": "User profile data",
		})
	})

	userGroup.POST("/update", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Profile updated successfully",
		})
	})

	// Example 3: Per-API-Key rate limiting
	log.Println("Setting up per-API-key rate limiting for /api/premium routes")
	premiumGroup := r.Group("/api/premium")
	premiumGroup.Use(ratelimit.GinRateLimiterPerAPIKey("X-API-Key", 1000, time.Hour))

	premiumGroup.GET("/data", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Premium data - 1000 requests per hour per API key",
			"data":    "premium content",
		})
	})

	// Example 4: Custom key extraction (combining IP and path)
	log.Println("Setting up per-IP-per-endpoint rate limiting for /api/custom routes")
	customGroup := r.Group("/api/custom")
	customGroup.Use(ratelimit.GinRateLimiter(ratelimit.MiddlewareConfig{
		RateLimiterFactory: func(key string) (ratelimit.RateLimiter, error) {
			// 10 requests per endpoint per IP per minute
			return ratelimit.NewLeakyBucketRateLimiter(ratelimit.Options{
				Capacity: 10,
				Rate:     time.Minute / 10,
			})
		},
		KeyExtractor: ratelimit.CombineExtractors(
			ratelimit.ExtractIPAddress,
			ratelimit.ExtractPath,
		),
		IncludeHeaders: true,
	}))

	customGroup.GET("/search", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"results": []string{"result1", "result2"},
		})
	})

	customGroup.GET("/export", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"file": "export.csv",
		})
	})

	// Example 5: Skip function - don't rate limit health checks
	healthGroup := r.Group("/health")
	healthGroup.Use(ratelimit.GinRateLimiter(ratelimit.MiddlewareConfig{
		RateLimiter: func() ratelimit.RateLimiter {
			rl, _ := ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
				Capacity: 10,
				Rate:     time.Second,
			})
			return rl
		}(),
		SkipFunc: func(r *http.Request) bool {
			// Skip rate limiting for specific user agents (monitoring tools)
			return r.Header.Get("User-Agent") == "HealthCheckBot"
		},
		IncludeHeaders: true,
	}))

	healthGroup.GET("/check", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
		})
	})

	// Public route without rate limiting
	r.GET("/public", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "This route has no rate limiting",
		})
	})

	// Start server
	log.Println("\n============================================================")
	log.Println("Server starting on :8080")
	log.Println("============================================================")
	log.Println("\nTry these examples:")
	log.Println("1. Per-IP rate limiting:")
	log.Println("   curl http://localhost:8080/api/data")
	log.Println("\n2. Per-user rate limiting:")
	log.Println("   curl -H 'X-User-ID: user123' http://localhost:8080/api/user/profile")
	log.Println("\n3. Per-API-key rate limiting:")
	log.Println("   curl -H 'X-API-Key: key123' http://localhost:8080/api/premium/data")
	log.Println("\n4. Per-IP-per-endpoint rate limiting:")
	log.Println("   curl http://localhost:8080/api/custom/search")
	log.Println("\n5. Health check (can skip rate limiting with User-Agent):")
	log.Println("   curl -H 'User-Agent: HealthCheckBot' http://localhost:8080/health/check")
	log.Println("\n6. Public route (no rate limiting):")
	log.Println("   curl http://localhost:8080/public")
	log.Println()

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
