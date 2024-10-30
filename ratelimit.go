package ratelimit

import (
	"sync"
	"time"
)

type RateLimiter struct {
	capacity      int
	tokens        int
	rate          time.Duration
	lastRefreshed time.Time
	mutex         sync.Mutex
}

func NewRateLimiter(capacity, rate) *RateLimiter {
	return &RateLimiter{
		capacity: capacity,
		tokens: capacity,
		rate: rate,
		lastRefreshed: time.Now(),
		mutex: sync.Mutex{}
	}
}
