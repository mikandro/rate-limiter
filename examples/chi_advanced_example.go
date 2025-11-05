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

	// Example 1: Per-IP rate limiting for API routes
	log.Println("Setting up per-IP rate limiting: 5 requests per second per IP")
	r.Route("/api", func(r chi.Router) {
		r.Use(ratelimit.ChiRateLimiterPerIP(5, time.Second))

		r.Get("/data", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"message":"This endpoint is rate limited per IP","data":["item1","item2","item3"]}`))
		})

		r.Post("/submit", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"message":"Data submitted successfully"}`))
		})
	})

	// Example 2: Per-user rate limiting with custom configuration
	log.Println("Setting up per-user rate limiting for /users routes")
	r.Route("/users", func(r chi.Router) {
		r.Use(ratelimit.ChiRateLimiter(ratelimit.MiddlewareConfig{
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

		r.Get("/{userID}/profile", func(w http.ResponseWriter, r *http.Request) {
			userID := chi.URLParam(r, "userID")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"user_id":"` + userID + `","message":"User profile data"}`))
		})

		r.Put("/{userID}/update", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"message":"Profile updated successfully"}`))
		})
	})

	// Example 3: Per-API-Key rate limiting for premium routes
	log.Println("Setting up per-API-key rate limiting for /premium routes")
	r.Route("/premium", func(r chi.Router) {
		r.Use(ratelimit.ChiRateLimiterPerAPIKey("X-API-Key", 1000, time.Hour))

		r.Get("/data", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"message":"Premium data - 1000 requests per hour per API key","data":"premium content"}`))
		})

		r.Get("/analytics", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"analytics":"detailed analytics data"}`))
		})
	})

	// Example 4: Custom key extraction with Leaky Bucket algorithm
	log.Println("Setting up per-IP-per-endpoint rate limiting for /custom routes")
	r.Route("/custom", func(r chi.Router) {
		r.Use(ratelimit.ChiRateLimiter(ratelimit.MiddlewareConfig{
			RateLimiterFactory: func(key string) (ratelimit.RateLimiter, error) {
				// 10 requests per endpoint per IP per minute using Leaky Bucket
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

		r.Get("/search", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"results":["result1","result2"]}`))
		})

		r.Get("/export", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"file":"export.csv"}`))
		})
	})

	// Example 5: Skip function - don't rate limit health checks
	log.Println("Setting up health check routes with conditional rate limiting")
	r.Route("/health", func(r chi.Router) {
		// Create a rate limiter
		rl, _ := ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
			Capacity: 10,
			Rate:     time.Second,
		})

		r.Use(ratelimit.ChiRateLimiter(ratelimit.MiddlewareConfig{
			RateLimiter: rl,
			SkipFunc: func(r *http.Request) bool {
				// Skip rate limiting for specific user agents (monitoring tools)
				return r.Header.Get("User-Agent") == "HealthCheckBot"
			},
			IncludeHeaders: true,
		}))

		r.Get("/check", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
		})

		r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ready":true}`))
		})
	})

	// Example 6: Different rate limiters for different algorithms
	log.Println("Setting up routes with different rate limiting algorithms")
	r.Route("/algorithms", func(r chi.Router) {
		// Sliding Window Counter algorithm
		r.Route("/sliding", func(r chi.Router) {
			r.Use(ratelimit.ChiRateLimiter(ratelimit.MiddlewareConfig{
				RateLimiterFactory: func(key string) (ratelimit.RateLimiter, error) {
					return ratelimit.NewSlidingWindowCounterRateLimiter(ratelimit.Options{
						Capacity: 20,
						Rate:     time.Minute,
					})
				},
				KeyExtractor:   ratelimit.ExtractIPAddress,
				IncludeHeaders: true,
			}))

			r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"algorithm":"Sliding Window Counter","limit":"20 requests per minute"}`))
			})
		})

		// Fixed Window Counter algorithm
		r.Route("/fixed", func(r chi.Router) {
			r.Use(ratelimit.ChiRateLimiter(ratelimit.MiddlewareConfig{
				RateLimiterFactory: func(key string) (ratelimit.RateLimiter, error) {
					return ratelimit.NewFixedWindowCounterRateLimiter(ratelimit.Options{
						Capacity: 15,
						Rate:     time.Minute,
					})
				},
				KeyExtractor:   ratelimit.ExtractIPAddress,
				IncludeHeaders: true,
			}))

			r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"algorithm":"Fixed Window Counter","limit":"15 requests per minute"}`))
			})
		})
	})

	// Public route without rate limiting
	r.Get("/public", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"This route has no rate limiting"}`))
	})

	// Start server
	log.Println("\n============================================================")
	log.Println("Server starting on :8080")
	log.Println("============================================================")
	log.Println("\nTry these examples:")
	log.Println("\n1. Per-IP rate limiting:")
	log.Println("   curl http://localhost:8080/api/data")
	log.Println("\n2. Per-user rate limiting:")
	log.Println("   curl -H 'X-User-ID: user123' http://localhost:8080/users/123/profile")
	log.Println("\n3. Per-API-key rate limiting:")
	log.Println("   curl -H 'X-API-Key: key123' http://localhost:8080/premium/data")
	log.Println("\n4. Per-IP-per-endpoint rate limiting:")
	log.Println("   curl http://localhost:8080/custom/search")
	log.Println("\n5. Health check (can skip rate limiting with User-Agent):")
	log.Println("   curl -H 'User-Agent: HealthCheckBot' http://localhost:8080/health/check")
	log.Println("\n6. Different algorithms:")
	log.Println("   curl http://localhost:8080/algorithms/sliding/test")
	log.Println("   curl http://localhost:8080/algorithms/fixed/test")
	log.Println("\n7. Public route (no rate limiting):")
	log.Println("   curl http://localhost:8080/public")
	log.Println()

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
