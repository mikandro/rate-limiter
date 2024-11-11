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

func NewRateLimiter(capacity int, rate time.Duration) *RateLimiter {
	if capacity <= 0 {
		panic("Capacity must be greater than 0")
	}

	if rate <= 0 {
		panic("Rate must be greater than 0")
	}

	return &RateLimiter{
		capacity:      capacity,
		tokens:        capacity,
		rate:          rate,
		lastRefreshed: time.Now(),
		mutex:         sync.Mutex{},
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.refill()

	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) refill() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefreshed)

	if elapsed > 0 {
		tokensToAdd := int(elapsed / rl.rate)
		if tokensToAdd > 0 {
			// Add new tokens or max capacity
			rl.tokens = min(rl.capacity, rl.tokens+tokensToAdd)
			rl.lastRefreshed = rl.lastRefreshed.Add(time.Duration(tokensToAdd) * rl.rate)
		}
	}
}
