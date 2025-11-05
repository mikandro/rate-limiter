package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mikandro/rate-limiter"
)

func main() {
	// Create a Gin router
	r := gin.Default()

	// Example 1: Simple global rate limiting
	// All requests share the same rate limiter
	log.Println("Example 1: Simple global rate limiting")
	rl, err := ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
		Capacity: 10,  // 10 requests
		Rate:     time.Second, // per second
	})
	if err != nil {
		log.Fatal(err)
	}

	// Apply middleware to all routes
	r.Use(ratelimit.GinRateLimiterSimple(rl))

	// Define routes
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.GET("/hello", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello, World!",
		})
	})

	// Start server
	log.Println("Starting server on :8080")
	log.Println("Try: curl http://localhost:8080/ping")
	log.Println("Rate limit: 10 requests per second (global)")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
