package main

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mikandro/rate-limiter"
)

func main() {
	// Create a Chi router
	r := chi.NewRouter()

	// Use Chi's built-in middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Example: Simple global rate limiting
	// All requests share the same rate limiter
	log.Println("Setting up global rate limiting: 10 requests per second")
	rl, err := ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
		Capacity: 10,  // 10 requests
		Rate:     time.Second, // per second
	})
	if err != nil {
		log.Fatal(err)
	}

	// Apply rate limiting middleware to all routes
	r.Use(ratelimit.ChiRateLimiterSimple(rl))

	// Define routes
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"pong"}`))
	})

	r.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"Hello, World!"}`))
	})

	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","uptime":"1h23m"}`))
	})

	// Start server
	log.Println("\n============================================================")
	log.Println("Server starting on :8080")
	log.Println("============================================================")
	log.Println("\nTry these commands:")
	log.Println("  curl http://localhost:8080/ping")
	log.Println("  curl http://localhost:8080/hello")
	log.Println("  curl http://localhost:8080/status")
	log.Println("\nRate limit: 10 requests per second (global)")
	log.Println("Headers will show: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset")
	log.Println()

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
