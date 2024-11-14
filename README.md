# RateLimiter Package

A simple, thread-safe **Token Bucket Rate Limiter** implemented in Go. It limits the rate of requests to ensure your system is protected against overload and maintains consistent request rates.

## Features

- **Token Bucket Strategy** for controlling request rates.
- Configurable **rate** and **capacity**.
- Supports both **blocking (`Wait()`)** and **non-blocking (`Allow()`)** modes.
- Designed to be **thread-safe** for concurrent usage.

## Installation

Requires **Go 1.18** or above.

```bash
go get github.com/mikandro/ratelimiter
```

## Usage

### Basic Example

```go
package main

import (
    "fmt"
    "time"
    "github.com/mikandro/ratelimiter"
)

func main() {
    // Create a new rate limiter: 5 tokens capacity, 1 token added per second
    rl := ratelimiter.NewRateLimiter(5, time.Second)

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

## API Overview

- **`NewRateLimiter(capacity int, rate time.Duration) *RateLimiter`**:
  - Creates a new rate limiter with specified `capacity` and `rate`.
- **`Allow() bool`**:
  - Non-blocking. Returns `true` if a token is available; otherwise, `false`.
- **`Wait(ctx context.Context) error`**:
  - Blocking. Waits for a token to become available or until the context expires.

## Versioning

This project follows **semantic versioning** conventions:

- **`MAJOR.MINOR.PATCH`** format.
- **Major versions** (`v1`, `v2`, etc.) indicate breaking changes.
- **Minor versions** add functionality in a backward-compatible manner.
- **Patch versions** are for backward-compatible bug fixes.

You can find released versions [here](https://github.com/yourusername/ratelimiter/releases).

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
- **Additional Rate Limiting Strategies**: Implement other strategies such as **Leaky Bucket** and **Sliding Window**.
- **Middleware Support**: Provide easy integration with popular Go HTTP frameworks such as Gin, Echo, and Chi.
