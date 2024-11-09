# RateLimiter Package

A simple, thread-safe **Token Bucket Rate Limiter** implemented in Go. It limits the rate of requests to ensure your system is protected against overload and maintains consistent request rates.

## Features

- **Token Bucket Strategy** for controlling request rates.
- Configurable **rate** and **capacity**.
- Supports both **blocking (`Wait()`)** and **non-blocking (`Allow()`)** modes.
- Designed to be **thread-safe** for concurrent usage.

## Installation

Requires **Go 1.18** or above.

