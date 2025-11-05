package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
)

// Test Key Extractors
func TestExtractIPAddress(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expected   string
	}{
		{
			name:       "X-Forwarded-For single IP",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.1"},
			remoteAddr: "192.168.1.1:1234",
			expected:   "203.0.113.1",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.1, 198.51.100.1, 192.0.2.1"},
			remoteAddr: "192.168.1.1:1234",
			expected:   "203.0.113.1",
		},
		{
			name:       "X-Real-IP header",
			headers:    map[string]string{"X-Real-IP": "203.0.113.2"},
			remoteAddr: "192.168.1.1:1234",
			expected:   "203.0.113.2",
		},
		{
			name:       "RemoteAddr fallback",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1:1234",
			expected:   "192.168.1.1",
		},
		{
			name:       "RemoteAddr without port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1",
			expected:   "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := ExtractIPAddress(req)
			if result != tt.expected {
				t.Errorf("ExtractIPAddress() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractHeader(t *testing.T) {
	extractor := ExtractHeader("X-Custom-Header")

	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "Header present",
			header:   "custom-value",
			expected: "custom-value",
		},
		{
			name:     "Header missing",
			header:   "",
			expected: "anonymous",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("X-Custom-Header", tt.header)
			}

			result := extractor(req)
			if result != tt.expected {
				t.Errorf("ExtractHeader() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Simple path",
			path:     "/api/users",
			expected: "/api/users",
		},
		{
			name:     "Path with query",
			path:     "/api/users?id=123",
			expected: "/api/users",
		},
		{
			name:     "Root path",
			path:     "/",
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			result := ExtractPath(req)
			if result != tt.expected {
				t.Errorf("ExtractPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCombineExtractors(t *testing.T) {
	extractor := CombineExtractors(ExtractIPAddress, ExtractPath, ExtractMethod)

	req := httptest.NewRequest("POST", "/api/users", nil)
	req.RemoteAddr = "192.168.1.1:1234"

	result := extractor(req)
	expected := "192.168.1.1:/api/users:POST"

	if result != expected {
		t.Errorf("CombineExtractors() = %v, want %v", result, expected)
	}
}

// Test LimiterStore
func TestLimiterStore(t *testing.T) {
	factory := func(key string) (RateLimiter, error) {
		return NewTokenBucketRateLimiter(Options{
			Capacity: 10,
			Rate:     time.Second,
		})
	}

	store := NewLimiterStore(factory)

	// Test getting a limiter for the first time
	limiter1, err := store.Get("key1")
	if err != nil {
		t.Fatalf("Failed to get limiter: %v", err)
	}
	if limiter1 == nil {
		t.Fatal("Expected non-nil limiter")
	}

	// Test getting the same limiter again (should be cached)
	limiter2, err := store.Get("key1")
	if err != nil {
		t.Fatalf("Failed to get limiter: %v", err)
	}
	if limiter1 != limiter2 {
		t.Error("Expected same limiter instance for same key")
	}

	// Test getting a different limiter
	limiter3, err := store.Get("key2")
	if err != nil {
		t.Fatalf("Failed to get limiter: %v", err)
	}
	if limiter1 == limiter3 {
		t.Error("Expected different limiter instance for different key")
	}

	// Test store size
	if store.Size() != 2 {
		t.Errorf("Expected store size 2, got %d", store.Size())
	}
}

// Test Chi Middleware
func TestChiRateLimiterSimple(t *testing.T) {
	// Create a rate limiter with 2 requests capacity
	rl, err := NewTokenBucketRateLimiter(Options{
		Capacity: 2,
		Rate:     time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}

	// Create Chi router with middleware
	r := chi.NewRouter()
	r.Use(ChiRateLimiterSimple(rl))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// First request should succeed
	req1 := httptest.NewRequest("GET", "/test", nil)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("First request failed with status %d", rr1.Code)
	}

	// Second request should succeed
	req2 := httptest.NewRequest("GET", "/test", nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("Second request failed with status %d", rr2.Code)
	}

	// Third request should be rate limited
	req3 := httptest.NewRequest("GET", "/test", nil)
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusTooManyRequests {
		t.Errorf("Third request should be rate limited, got status %d", rr3.Code)
	}

	// Check for rate limit headers
	if rr3.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("Expected X-RateLimit-Limit header")
	}
}

func TestChiRateLimiterPerIP(t *testing.T) {
	// Create Chi router with per-IP rate limiting
	r := chi.NewRouter()
	r.Use(ChiRateLimiterPerIP(2, time.Second))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Requests from IP1
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("First request from IP1 failed with status %d", rr1.Code)
	}

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:1234"
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("Second request from IP1 failed with status %d", rr2.Code)
	}

	// Third request from IP1 should be rate limited
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "192.168.1.1:1234"
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusTooManyRequests {
		t.Errorf("Third request from IP1 should be rate limited, got status %d", rr3.Code)
	}

	// Request from IP2 should succeed (different limiter)
	req4 := httptest.NewRequest("GET", "/test", nil)
	req4.RemoteAddr = "192.168.1.2:1234"
	rr4 := httptest.NewRecorder()
	r.ServeHTTP(rr4, req4)

	if rr4.Code != http.StatusOK {
		t.Errorf("Request from IP2 should succeed, got status %d", rr4.Code)
	}
}

// Test Gin Middleware
func TestGinRateLimiterSimple(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a rate limiter with 2 requests capacity
	rl, err := NewTokenBucketRateLimiter(Options{
		Capacity: 2,
		Rate:     time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}

	// Create Gin router with middleware
	r := gin.New()
	r.Use(GinRateLimiterSimple(rl))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// First request should succeed
	req1 := httptest.NewRequest("GET", "/test", nil)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("First request failed with status %d", rr1.Code)
	}

	// Second request should succeed
	req2 := httptest.NewRequest("GET", "/test", nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("Second request failed with status %d", rr2.Code)
	}

	// Third request should be rate limited
	req3 := httptest.NewRequest("GET", "/test", nil)
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusTooManyRequests {
		t.Errorf("Third request should be rate limited, got status %d", rr3.Code)
	}

	// Check for rate limit headers
	if rr3.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("Expected X-RateLimit-Limit header")
	}
}

func TestGinRateLimiterPerIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create Gin router with per-IP rate limiting
	r := gin.New()
	r.Use(GinRateLimiterPerIP(2, time.Second))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Requests from IP1
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("First request from IP1 failed with status %d", rr1.Code)
	}

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:1234"
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("Second request from IP1 failed with status %d", rr2.Code)
	}

	// Third request from IP1 should be rate limited
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "192.168.1.1:1234"
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusTooManyRequests {
		t.Errorf("Third request from IP1 should be rate limited, got status %d", rr3.Code)
	}

	// Request from IP2 should succeed (different limiter)
	req4 := httptest.NewRequest("GET", "/test", nil)
	req4.RemoteAddr = "192.168.1.2:1234"
	rr4 := httptest.NewRecorder()
	r.ServeHTTP(rr4, req4)

	if rr4.Code != http.StatusOK {
		t.Errorf("Request from IP2 should succeed, got status %d", rr4.Code)
	}
}

func TestGinRateLimiterWithSkipFunc(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rl, _ := NewTokenBucketRateLimiter(Options{
		Capacity: 1,
		Rate:     time.Second,
	})

	// Create Gin router with skip function
	r := gin.New()
	r.Use(GinRateLimiter(MiddlewareConfig{
		RateLimiter: rl,
		SkipFunc: func(r *http.Request) bool {
			// Skip rate limiting for health checks
			return r.URL.Path == "/health"
		},
		IncludeHeaders: true,
	}))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})
	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "Healthy")
	})

	// First request to /test should succeed
	req1 := httptest.NewRequest("GET", "/test", nil)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("First request to /test failed with status %d", rr1.Code)
	}

	// Second request to /test should be rate limited
	req2 := httptest.NewRequest("GET", "/test", nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request to /test should be rate limited, got status %d", rr2.Code)
	}

	// Requests to /health should always succeed (skipped)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Health check request %d failed with status %d", i+1, rr.Code)
		}
	}
}

// Test Headers
func TestSetRateLimitHeaders(t *testing.T) {
	w := httptest.NewRecorder()

	headers := RateLimitHeaders{
		Limit:      100,
		Remaining:  75,
		Reset:      time.Unix(1234567890, 0),
		RetryAfter: 5 * time.Second,
	}

	SetRateLimitHeaders(w, headers)

	if w.Header().Get("X-RateLimit-Limit") != "100" {
		t.Errorf("Expected X-RateLimit-Limit=100, got %s", w.Header().Get("X-RateLimit-Limit"))
	}

	if w.Header().Get("X-RateLimit-Remaining") != "75" {
		t.Errorf("Expected X-RateLimit-Remaining=75, got %s", w.Header().Get("X-RateLimit-Remaining"))
	}

	if w.Header().Get("X-RateLimit-Reset") != "1234567890" {
		t.Errorf("Expected X-RateLimit-Reset=1234567890, got %s", w.Header().Get("X-RateLimit-Reset"))
	}

	if w.Header().Get("Retry-After") != "5" {
		t.Errorf("Expected Retry-After=5, got %s", w.Header().Get("Retry-After"))
	}
}

func TestGetRateLimitHeaders(t *testing.T) {
	rl, err := NewTokenBucketRateLimiter(Options{
		Capacity: 10,
		Rate:     time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}

	headers := GetRateLimitHeaders(rl, time.Second)

	if headers.Limit != 10 {
		t.Errorf("Expected Limit=10, got %d", headers.Limit)
	}

	if headers.Remaining != 10 {
		t.Errorf("Expected Remaining=10, got %d", headers.Remaining)
	}
}

func TestMiddlewareConfigValidation(t *testing.T) {
	// Test missing both RateLimiter and RateLimiterFactory
	config := MiddlewareConfig{}
	err := config.Validate()
	if err == nil {
		t.Error("Expected validation error when both RateLimiter and RateLimiterFactory are nil")
	}

	// Test setting both RateLimiter and RateLimiterFactory
	rl, _ := NewTokenBucketRateLimiter(Options{Capacity: 10, Rate: time.Second})
	config = MiddlewareConfig{
		RateLimiter: rl,
		RateLimiterFactory: func(key string) (RateLimiter, error) {
			return rl, nil
		},
	}
	err = config.Validate()
	if err == nil {
		t.Error("Expected validation error when both RateLimiter and RateLimiterFactory are set")
	}

	// Test valid config with RateLimiter
	config = MiddlewareConfig{
		RateLimiter: rl,
	}
	err = config.Validate()
	if err != nil {
		t.Errorf("Expected no validation error for valid config, got %v", err)
	}

	// Test valid config with RateLimiterFactory
	config = MiddlewareConfig{
		RateLimiterFactory: func(key string) (RateLimiter, error) {
			return NewTokenBucketRateLimiter(Options{Capacity: 10, Rate: time.Second})
		},
	}
	err = config.Validate()
	if err != nil {
		t.Errorf("Expected no validation error for valid config, got %v", err)
	}

	// Verify defaults are set
	if config.KeyExtractor == nil {
		t.Error("Expected default KeyExtractor to be set")
	}
	if config.ErrorHandler == nil {
		t.Error("Expected default ErrorHandler to be set")
	}
	if config.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected default StatusCode=%d, got %d", http.StatusTooManyRequests, config.StatusCode)
	}
}
