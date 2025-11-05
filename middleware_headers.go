package ratelimit

import (
	"net/http"
	"strconv"
	"time"
)

// RateLimitHeaders contains standard rate limiting header values.
type RateLimitHeaders struct {
	Limit     int           // Total number of requests allowed in the time window
	Remaining int           // Number of requests remaining in the current window
	Reset     time.Time     // Time when the rate limit will reset
	RetryAfter time.Duration // Duration until the next request can be made
}

// SetRateLimitHeaders sets standard rate limiting headers on the response.
// It follows the IETF draft standard for RateLimit headers:
// - X-RateLimit-Limit: Maximum number of requests allowed
// - X-RateLimit-Remaining: Number of requests remaining
// - X-RateLimit-Reset: Unix timestamp when the limit resets
// - Retry-After: Seconds until the limit resets (only when rate limited)
func SetRateLimitHeaders(w http.ResponseWriter, headers RateLimitHeaders) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(headers.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(headers.Remaining))

	if !headers.Reset.IsZero() {
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(headers.Reset.Unix(), 10))
	}

	if headers.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(headers.RetryAfter.Seconds())))
	}
}

// CalculateRetryAfter calculates the duration until the next request can be made.
// For token bucket algorithms, it estimates based on the refill rate.
func CalculateRetryAfter(limiter RateLimiter, rate time.Duration) time.Duration {
	remaining := limiter.GetAvailableTokens()
	if remaining > 0 {
		return 0
	}

	// If no tokens available, return the rate duration
	// This is an approximation; actual implementation may vary by algorithm
	return rate
}

// GetRateLimitHeaders extracts rate limit information from a RateLimiter.
func GetRateLimitHeaders(limiter RateLimiter, rate time.Duration) RateLimitHeaders {
	capacity := limiter.GetCapacity()
	remaining := limiter.GetAvailableTokens()

	// Estimate reset time based on rate
	var reset time.Time
	var retryAfter time.Duration

	if remaining <= 0 {
		// Calculate when the next token will be available
		retryAfter = rate
		reset = time.Now().Add(retryAfter)
	} else {
		// If tokens are available, reset is in the future based on capacity
		reset = time.Now().Add(rate * time.Duration(capacity-remaining))
	}

	return RateLimitHeaders{
		Limit:      capacity,
		Remaining:  remaining,
		Reset:      reset,
		RetryAfter: retryAfter,
	}
}
