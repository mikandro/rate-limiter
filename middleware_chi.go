package ratelimit

import (
	"net/http"
	"time"
)

// ChiRateLimiter creates a Chi-compatible middleware with the given configuration.
// It supports both single rate limiter and per-key rate limiting.
//
// Example with single rate limiter:
//
//	rl, _ := ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
//	    Capacity: 100,
//	    Rate:     time.Second,
//	})
//	r.Use(ratelimit.ChiRateLimiter(ratelimit.MiddlewareConfig{
//	    RateLimiter: rl,
//	}))
//
// Example with per-IP rate limiting:
//
//	r.Use(ratelimit.ChiRateLimiter(ratelimit.MiddlewareConfig{
//	    RateLimiterFactory: func(key string) (ratelimit.RateLimiter, error) {
//	        return ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
//	            Capacity: 10,
//	            Rate:     time.Second,
//	        })
//	    },
//	    KeyExtractor: ratelimit.ExtractIPAddress,
//	}))
func ChiRateLimiter(config MiddlewareConfig) func(next http.Handler) http.Handler {
	// Validate configuration
	if err := config.Validate(); err != nil {
		panic(err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if we should skip rate limiting for this request
			if config.SkipFunc != nil && config.SkipFunc(r) {
				next.ServeHTTP(w, r)
				return
			}

			var limiter RateLimiter
			var err error

			// Get the appropriate rate limiter
			if config.RateLimiter != nil {
				limiter = config.RateLimiter
			} else {
				// Extract key and get per-key limiter
				key := config.KeyExtractor(r)
				limiter, err = config.Store.Get(key)
				if err != nil {
					// On error, allow the request through (fail open)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
			}

			// Check rate limit
			allowed := limiter.Allow()

			// Set rate limit headers if enabled
			if config.IncludeHeaders || (!allowed) {
				headers := GetRateLimitHeaders(limiter, time.Second)
				SetRateLimitHeaders(w, headers)
			}

			if !allowed {
				// Rate limit exceeded
				retryAfter := CalculateRetryAfter(limiter, time.Second)
				config.ErrorHandler(w, r, retryAfter)
				return
			}

			// Request allowed, proceed
			next.ServeHTTP(w, r)
		})
	}
}

// ChiRateLimiterSimple creates a simple Chi middleware using a single rate limiter.
// This is a convenience function for basic use cases.
//
// Example:
//
//	rl, _ := ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
//	    Capacity: 100,
//	    Rate:     time.Second,
//	})
//	r.Use(ratelimit.ChiRateLimiterSimple(rl))
func ChiRateLimiterSimple(limiter RateLimiter) func(next http.Handler) http.Handler {
	return ChiRateLimiter(MiddlewareConfig{
		RateLimiter:    limiter,
		IncludeHeaders: true,
	})
}

// ChiRateLimiterPerIP creates a Chi middleware that rate limits per IP address.
// Each IP address gets its own rate limiter with the specified capacity and rate.
//
// Example:
//
//	r.Use(ratelimit.ChiRateLimiterPerIP(10, time.Second))
func ChiRateLimiterPerIP(capacity int, rate time.Duration) func(next http.Handler) http.Handler {
	return ChiRateLimiter(MiddlewareConfig{
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

// ChiRateLimiterPerUser creates a Chi middleware that rate limits per user.
// It extracts the user identifier from the specified header.
//
// Example:
//
//	r.Use(ratelimit.ChiRateLimiterPerUser("X-User-ID", 100, time.Minute))
func ChiRateLimiterPerUser(headerName string, capacity int, rate time.Duration) func(next http.Handler) http.Handler {
	return ChiRateLimiter(MiddlewareConfig{
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

// ChiRateLimiterPerAPIKey creates a Chi middleware that rate limits per API key.
// It extracts the API key from the specified header.
//
// Example:
//
//	r.Use(ratelimit.ChiRateLimiterPerAPIKey("X-API-Key", 1000, time.Hour))
func ChiRateLimiterPerAPIKey(headerName string, capacity int, rate time.Duration) func(next http.Handler) http.Handler {
	return ChiRateLimiter(MiddlewareConfig{
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
