# RateLimiter Package

A simple, thread-safe rate limiting library implemented in Go. It limits the rate of requests to ensure your system is protected against overload and maintains consistent request rates.

## Features

- **Multiple Rate Limiting Strategies**:
  - **Token Bucket**: Allows burst traffic up to capacity with token refills at a fixed rate.
  - **Leaky Bucket**: Provides uniform rate limiting by "leaking" requests at a constant rate.
  - **Sliding Window Counter**: Smooth rate limiting using weighted averages between windows.
  - **Fixed Window Counter**: Simple, memory-efficient rate limiting with fixed time windows.
  - **Distributed Token Bucket**: Distributed rate limiting across multiple servers using Redis.
- **HTTP Middleware Support**:
  - **Gin** and **Chi** middleware implementations
  - Per-IP, per-user, per-API-key rate limiting
  - Custom key extraction strategies
  - Rate limit headers (RFC 6585 compliant)
  - Flexible configuration options
- Configurable **rate** and **capacity**.
- Supports both **blocking (`Wait()`)** and **non-blocking (`Allow()`)** modes.
- Designed to be **thread-safe** for concurrent usage.
- **Distributed rate limiting** with Redis for cross-instance synchronization.
- Common interface for all rate limiting strategies.

## Installation

Requires **Go 1.18** or above.

```bash
go get github.com/mikandro/ratelimiter
```

## Usage

### Token Bucket Example

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/mikandro/ratelimiter"
)

func main() {
    // Create a Token Bucket rate limiter: 5 tokens capacity, 1 token added per second
    opts := ratelimiter.Options{
        Capacity: 5,
        Rate:     time.Second,
    }
    rl, err := ratelimiter.NewTokenBucketRateLimiter(opts)
    if err != nil {
        panic(err)
    }

    for i := 0; i < 10; i++ {
        if rl.Allow() {
            fmt.Printf("Request %d allowed\n", i+1)
        } else {
            fmt.Printf("Request %d denied\n", i+1)
        }
        time.Sleep(200 * time.Millisecond)
    }
}
```

### Leaky Bucket Example

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/mikandro/ratelimiter"
)

func main() {
    // Create a Leaky Bucket rate limiter: 5 requests capacity, 1 request leaked per second
    opts := ratelimiter.Options{
        Capacity: 5,
        Rate:     time.Second,
    }
    rl, err := ratelimiter.NewLeakyBucketRateLimiter(opts)
    if err != nil {
        panic(err)
    }

    // Using Wait() for blocking behavior
    ctx := context.Background()
    for i := 0; i < 10; i++ {
        if err := rl.Wait(ctx); err == nil {
            fmt.Printf("Request %d allowed\n", i+1)
        }
    }
}
```

## HTTP Middleware

The library provides easy integration with popular Go HTTP frameworks including **Gin** and **Chi**. The middleware supports both simple global rate limiting and advanced per-key rate limiting strategies.

### Gin Middleware

#### Basic Usage

```go
package main

import (
    "time"
    "github.com/gin-gonic/gin"
    "github.com/mikandro/rate-limiter"
)

func main() {
    r := gin.Default()

    // Create a rate limiter: 100 requests per second
    rl, _ := ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
        Capacity: 100,
        Rate:     time.Second,
    })

    // Apply to all routes
    r.Use(ratelimit.GinRateLimiterSimple(rl))

    r.GET("/api/data", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "Success"})
    })

    r.Run(":8080")
}
```

#### Per-IP Rate Limiting

```go
// Each IP address gets 10 requests per second
r.Use(ratelimit.GinRateLimiterPerIP(10, time.Second))
```

#### Per-User Rate Limiting

```go
// Rate limit by user ID from header (100 requests per minute per user)
r.Use(ratelimit.GinRateLimiterPerUser("X-User-ID", 100, time.Minute))
```

#### Per-API-Key Rate Limiting

```go
// Rate limit by API key (1000 requests per hour per key)
r.Use(ratelimit.GinRateLimiterPerAPIKey("X-API-Key", 1000, time.Hour))
```

#### Advanced Configuration

```go
r.Use(ratelimit.GinRateLimiter(ratelimit.MiddlewareConfig{
    RateLimiterFactory: func(key string) (ratelimit.RateLimiter, error) {
        return ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
            Capacity: 50,
            Rate:     time.Minute,
        })
    },
    KeyExtractor: ratelimit.CombineExtractors(
        ratelimit.ExtractIPAddress,
        ratelimit.ExtractPath,
    ),
    ErrorHandler: func(w http.ResponseWriter, r *http.Request, retryAfter time.Duration) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusTooManyRequests)
        w.Write([]byte(`{"error":"rate_limit_exceeded"}`))
    },
    SkipFunc: func(r *http.Request) bool {
        // Skip rate limiting for health checks
        return r.URL.Path == "/health"
    },
    IncludeHeaders: true,
}))
```

### Chi Middleware

#### Basic Usage

```go
package main

import (
    "net/http"
    "time"
    "github.com/go-chi/chi/v5"
    "github.com/mikandro/rate-limiter"
)

func main() {
    r := chi.NewRouter()

    // Create a rate limiter: 100 requests per second
    rl, _ := ratelimit.NewTokenBucketRateLimiter(ratelimit.Options{
        Capacity: 100,
        Rate:     time.Second,
    })

    // Apply to all routes
    r.Use(ratelimit.ChiRateLimiterSimple(rl))

    r.Get("/api/data", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Success"))
    })

    http.ListenAndServe(":8080", r)
}
```

#### Per-IP Rate Limiting

```go
// Each IP address gets 10 requests per second
r.Use(ratelimit.ChiRateLimiterPerIP(10, time.Second))
```

#### Per-User Rate Limiting

```go
// Rate limit by user ID from header (100 requests per minute per user)
r.Use(ratelimit.ChiRateLimiterPerUser("X-User-ID", 100, time.Minute))
```

#### Per-API-Key Rate Limiting

```go
// Rate limit by API key (1000 requests per hour per key)
r.Use(ratelimit.ChiRateLimiterPerAPIKey("X-API-Key", 1000, time.Hour))
```

#### Advanced Configuration

```go
r.Use(ratelimit.ChiRateLimiter(ratelimit.MiddlewareConfig{
    RateLimiterFactory: func(key string) (ratelimit.RateLimiter, error) {
        return ratelimit.NewLeakyBucketRateLimiter(ratelimit.Options{
            Capacity: 50,
            Rate:     time.Minute,
        })
    },
    KeyExtractor: ratelimit.CombineExtractors(
        ratelimit.ExtractIPAddress,
        ratelimit.ExtractPath,
    ),
    ErrorHandler: func(w http.ResponseWriter, r *http.Request, retryAfter time.Duration) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusTooManyRequests)
        w.Write([]byte(`{"error":"rate_limit_exceeded"}`))
    },
    SkipFunc: func(r *http.Request) bool {
        // Skip rate limiting for monitoring bots
        return r.Header.Get("User-Agent") == "HealthCheckBot"
    },
    IncludeHeaders: true,
}))
```

### Rate Limit Headers

When `IncludeHeaders` is enabled, the middleware automatically adds standard rate limiting headers to responses:

- **X-RateLimit-Limit**: Maximum number of requests allowed
- **X-RateLimit-Remaining**: Number of requests remaining in the current window
- **X-RateLimit-Reset**: Unix timestamp when the rate limit resets
- **Retry-After**: Seconds until the next request can be made (when rate limited)

Example response headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1699564800
```

### Key Extraction Strategies

The library provides several built-in key extractors:

- **`ExtractIPAddress`**: Extract client IP (checks X-Forwarded-For, X-Real-IP, RemoteAddr)
- **`ExtractUserID(headerName)`**: Extract user ID from a header
- **`ExtractAPIKey(headerName)`**: Extract API key from a header
- **`ExtractPath`**: Use the request path as the key
- **`ExtractMethod`**: Use the HTTP method as the key
- **`CombineExtractors(...)`**: Combine multiple extractors (e.g., IP + Path)

### Examples

Complete examples are available in the `examples/` directory:

- `gin_basic_example.go` - Basic Gin middleware usage
- `gin_advanced_example.go` - Advanced Gin features (per-user, custom config)
- `chi_basic_example.go` - Basic Chi middleware usage
- `chi_advanced_example.go` - Advanced Chi features (different algorithms, custom extractors)

## API Overview

### RateLimiter Interface

All rate limiters implement the `RateLimiter` interface:

- **`Allow() bool`**:
  - Non-blocking. Returns `true` if a token/slot is available; otherwise, `false`.
- **`Wait(ctx context.Context) error`**:
  - Blocking. Waits for a token/slot to become available or until the context expires.
- **`GetCapacity() int`**:
  - Returns the maximum capacity of the rate limiter.
- **`GetAvailableTokens() int`**:
  - Returns the number of currently available tokens/slots.

### Creating Rate Limiters

#### Local (Single-Instance) Rate Limiters

- **`NewTokenBucketRateLimiter(opts Options) (*TokenBucketRateLimiter, error)`**:
  - Creates a Token Bucket rate limiter with specified options.
- **`NewLeakyBucketRateLimiter(opts Options) (*LeakyBucketRateLimiter, error)`**:
  - Creates a Leaky Bucket rate limiter with specified options.
- **`NewSlidingWindowCounterRateLimiter(opts Options) (*SlidingWindowCounterRateLimiter, error)`**:
  - Creates a Sliding Window Counter rate limiter with specified options.
- **`NewFixedWindowCounterRateLimiter(opts Options) (*FixedWindowCounterRateLimiter, error)`**:
  - Creates a Fixed Window Counter rate limiter with specified options.
- **`NewDistributedTokenBucketRateLimiter(opts DistributedOptions) (*DistributedTokenBucketRateLimiter, error)`**:
  - Creates a Distributed Token Bucket rate limiter using Redis for distributed rate limiting.


### Tagging Releases

To create a release:

```bash
git tag -a v1.0.0 -m "Initial stable release of the RateLimiter package"
git push origin v1.0.0
```

## License

This project is licensed under the **MIT License**. See the [LICENSE](LICENSE) file for details.

## Author

Developed by [Your Name](https://github.com/mikandro).

## Contributing

Contributions are welcome! Please open an issue or submit a pull request to suggest improvements or add new features.

### Steps to Contribute

1. **Fork** the repository.
2. Create a new **branch** for your feature or bug fix.
3. **Commit** your changes and push them to your fork.
4. Submit a **pull request** to the `main` branch.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for the detailed history of changes.

## Future Improvements

- **Additional HTTP Frameworks**: Add middleware support for Echo, Fiber, and other popular Go frameworks.
- **Metrics & Observability**: Add built-in metrics collection (Prometheus, StatsD) for rate limiter performance.
- **Dynamic Rate Limits**: Support for dynamic rate limit adjustment based on system load.
- **Sliding Log Algorithm**: Implement the Sliding Log rate limiting strategy.
- **Rate Limit Quotas**: Add support for quota-based rate limiting (e.g., monthly API quotas).
