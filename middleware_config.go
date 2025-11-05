package ratelimit

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// KeyExtractor is a function that extracts a rate limiting key from an HTTP request.
// The key is used to identify the client/user for rate limiting purposes.
type KeyExtractor func(r *http.Request) string

// RateLimiterFactory creates a new RateLimiter instance for a given key.
// This allows per-key rate limiting where each key (user, IP, etc.) gets its own limiter.
type RateLimiterFactory func(key string) (RateLimiter, error)

// ErrorHandler is a function that handles rate limit errors.
// It receives the http.ResponseWriter, the request, and the limit info.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, retryAfter time.Duration)

// SkipFunc determines whether to skip rate limiting for a request.
type SkipFunc func(r *http.Request) bool

// MiddlewareConfig contains configuration options for rate limiting middleware.
type MiddlewareConfig struct {
	// RateLimiter is a single rate limiter applied to all requests.
	// Either set RateLimiter or RateLimiterFactory, not both.
	RateLimiter RateLimiter

	// RateLimiterFactory creates a rate limiter per key.
	// Either set RateLimiter or RateLimiterFactory, not both.
	RateLimiterFactory RateLimiterFactory

	// KeyExtractor extracts the key from the request.
	// Required when using RateLimiterFactory.
	// Default: ExtractIPAddress
	KeyExtractor KeyExtractor

	// ErrorHandler handles rate limit exceeded responses.
	// Default: DefaultErrorHandler
	ErrorHandler ErrorHandler

	// SkipFunc determines if rate limiting should be skipped for a request.
	// Optional.
	SkipFunc SkipFunc

	// IncludeHeaders determines whether to include rate limit headers in responses.
	// Default: true
	IncludeHeaders bool

	// StatusCode is the HTTP status code returned when rate limit is exceeded.
	// Default: http.StatusTooManyRequests (429)
	StatusCode int

	// Store manages per-key rate limiters (internal use).
	// Automatically created if using RateLimiterFactory.
	Store *LimiterStore
}

// Validate checks if the middleware configuration is valid.
func (c *MiddlewareConfig) Validate() error {
	if c.RateLimiter == nil && c.RateLimiterFactory == nil {
		return fmt.Errorf("either RateLimiter or RateLimiterFactory must be set")
	}

	if c.RateLimiter != nil && c.RateLimiterFactory != nil {
		return fmt.Errorf("cannot set both RateLimiter and RateLimiterFactory")
	}

	if c.RateLimiterFactory != nil && c.KeyExtractor == nil {
		// Set default key extractor
		c.KeyExtractor = ExtractIPAddress
	}

	if c.ErrorHandler == nil {
		c.ErrorHandler = DefaultErrorHandler
	}

	if c.StatusCode == 0 {
		c.StatusCode = http.StatusTooManyRequests
	}

	// Create store if using factory
	if c.RateLimiterFactory != nil && c.Store == nil {
		c.Store = NewLimiterStore(c.RateLimiterFactory)
	}

	return nil
}

// LimiterStore manages a collection of rate limiters keyed by string identifiers.
// It provides thread-safe access and automatic cleanup of expired limiters.
type LimiterStore struct {
	factory    RateLimiterFactory
	limiters   map[string]*limiterEntry
	mu         sync.RWMutex
	cleanupTTL time.Duration
}

type limiterEntry struct {
	limiter    RateLimiter
	lastAccess time.Time
}

// NewLimiterStore creates a new limiter store with the given factory.
func NewLimiterStore(factory RateLimiterFactory) *LimiterStore {
	store := &LimiterStore{
		factory:    factory,
		limiters:   make(map[string]*limiterEntry),
		cleanupTTL: 10 * time.Minute, // Clean up limiters not accessed for 10 minutes
	}

	// Start cleanup goroutine
	go store.cleanupLoop()

	return store
}

// Get retrieves or creates a rate limiter for the given key.
func (s *LimiterStore) Get(key string) (RateLimiter, error) {
	s.mu.RLock()
	entry, exists := s.limiters[key]
	s.mu.RUnlock()

	if exists {
		entry.lastAccess = time.Now()
		return entry.limiter, nil
	}

	// Create new limiter
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, exists := s.limiters[key]; exists {
		entry.lastAccess = time.Now()
		return entry.limiter, nil
	}

	limiter, err := s.factory(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create rate limiter for key %s: %w", key, err)
	}

	s.limiters[key] = &limiterEntry{
		limiter:    limiter,
		lastAccess: time.Now(),
	}

	return limiter, nil
}

// cleanupLoop periodically removes expired limiters.
func (s *LimiterStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

// cleanup removes limiters that haven't been accessed recently.
func (s *LimiterStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, entry := range s.limiters {
		if now.Sub(entry.lastAccess) > s.cleanupTTL {
			delete(s.limiters, key)
		}
	}
}

// Size returns the number of active limiters in the store.
func (s *LimiterStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.limiters)
}

// Pre-built Key Extractors

// ExtractIPAddress extracts the client IP address from the request.
// It checks X-Forwarded-For, X-Real-IP headers, and falls back to RemoteAddr.
func ExtractIPAddress(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, use the first one
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// ExtractHeader creates a key extractor that extracts a value from a specific header.
func ExtractHeader(headerName string) KeyExtractor {
	return func(r *http.Request) string {
		value := r.Header.Get(headerName)
		if value == "" {
			return "anonymous"
		}
		return value
	}
}

// ExtractUserID creates a key extractor that extracts a user ID from a header.
// Commonly used with "X-User-ID" or "Authorization" headers.
func ExtractUserID(headerName string) KeyExtractor {
	return ExtractHeader(headerName)
}

// ExtractAPIKey creates a key extractor that extracts an API key from a header.
// Commonly used with "X-API-Key" header.
func ExtractAPIKey(headerName string) KeyExtractor {
	return ExtractHeader(headerName)
}

// ExtractPath extracts the request path as the rate limiting key.
// Useful for per-endpoint rate limiting.
func ExtractPath(r *http.Request) string {
	return r.URL.Path
}

// ExtractMethod extracts the request method as the rate limiting key.
func ExtractMethod(r *http.Request) string {
	return r.Method
}

// CombineExtractors combines multiple key extractors into a single key.
// The resulting key is a colon-separated string of all extracted values.
func CombineExtractors(extractors ...KeyExtractor) KeyExtractor {
	return func(r *http.Request) string {
		var parts []string
		for _, extractor := range extractors {
			parts = append(parts, extractor(r))
		}
		return strings.Join(parts, ":")
	}
}

// DefaultErrorHandler is the default error handler for rate limit exceeded responses.
func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, retryAfter time.Duration) {
	w.Header().Set("Content-Type", "application/json")
	if retryAfter > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%.0f", retryAfter.Seconds()))
	}
	w.WriteHeader(http.StatusTooManyRequests)
	fmt.Fprintf(w, `{"error":"Too Many Requests","message":"Rate limit exceeded. Please try again later."}`)
}
