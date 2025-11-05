# RateLimiter Package

A simple, thread-safe rate limiting library implemented in Go. It limits the rate of requests to ensure your system is protected against overload and maintains consistent request rates.

## Features

- **Multiple Rate Limiting Strategies**:
  - **Token Bucket**: Allows burst traffic up to capacity with token refills at a fixed rate.
  - **Leaky Bucket**: Provides uniform rate limiting by "leaking" requests at a constant rate.
- Configurable **rate** and **capacity**.
- Supports both **blocking (`Wait()`)** and **non-blocking (`Allow()`)** modes.
- Designed to be **thread-safe** for concurrent usage.
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

- **`NewTokenBucketRateLimiter(opts Options) (*TokenBucketRateLimiter, error)`**:
  - Creates a Token Bucket rate limiter with specified options.
- **`NewLeakyBucketRateLimiter(opts Options) (*LeakyBucketRateLimiter, error)`**:
  - Creates a Leaky Bucket rate limiter with specified options.


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

- **Distributed Rate Limiting**: Add support for distributed rate limiting using Redis.
- **Additional Rate Limiting Strategies**: Implement other strategies such as **Sliding Window**, **Fixed Window Counter**, and **Sliding Log**.
- **Enhanced Middleware Support**: Expand middleware integration with popular Go HTTP frameworks such as Echo and Chi.
