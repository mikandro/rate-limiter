package ratelimit

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type RateLimiter interface {
	Allow() bool
	Wait(ctx context.Context) error
	GetCapacity() int
	GetAvailableTokens() int
}

type Algorithm int

const (
	TokenBucket Algorithm = iota
	LeakyBucket
	SlidingWindowCounter
)

type Options struct {
	Capacity  int
	Rate      time.Duration
}

func (o Options) validate() error {
	if o.Capacity <= 0 {
		return fmt.Errorf("capacity must be greater than 0")
	}

	if o.Rate <= 0 {
		return fmt.Errorf("rate must be greater than 0")
	}

	return nil
}

type TokenBucketRateLimiter struct {
	capacity      int
	tokens        int
	rate          time.Duration
	lastRefreshed time.Time
	mutex         sync.Mutex
}

func NewTokenBucketRateLimiter(opts Options) (*TokenBucketRateLimiter, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	return &TokenBucketRateLimiter{
		capacity:      opts.Capacity,
		tokens:        opts.Capacity,
		rate:          opts.Rate,
		lastRefreshed: time.Now(),
		mutex:         sync.Mutex{},
	}, nil
}

func (rl *TokenBucketRateLimiter) Allow() bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	rl.refill()

	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

func (rl *TokenBucketRateLimiter) refill() {
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

func (rl *TokenBucketRateLimiter) Wait(ctx context.Context) error {
	for {
		rl.mutex.Lock()
		rl.refill()

		if rl.tokens > 0 {
			rl.tokens--
			log.Printf("Wait: Request allowed after waiting. Tokens remaingin: %d", rl.tokens)
			rl.mutex.Unlock()
			return nil
		}
		rl.mutex.Unlock() // Release the lock so other coroutines can access ratelimiter and refill

		// check if the context is done (timeour or cancelled)
		select {
		case <-ctx.Done():
			log.Printf("Wait: Request denied due to context timeout")
			return ctx.Err()
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}


type LeakyBucketRateLimiter struct {
	capacity int
	currentRequests int
	rate time.Duration
	lastRefreshed time.Time
	mutex sync.Mutex
}

func NewLeakyBucketRateLimiter(opts Options) (*LeakyBucketRateLimiter, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	return &LeakyBucketRateLimiter{
		capacity: opts.Capacity,
		currentRequests: opts.Capacity,
		rate: opts.Rate,
		lastRefreshed: time.Now(),
		mutex: sync.Mutex{},
	}, nil
}

func (rl *LeakyBucketRateLimiter) Allow() bool {
	rl.mutex.Lock();
	defer rl.mutex.Unlock();
	rl.leak();

	if rl.currentRequests < rl.capacity {
		rl.currentRequests++
		return true
	}

	return false
	
}

func (rl *LeakyBucketRateLimiter) Wait(ctx context.Context) error {
	for {
		rl.mutex.Lock();
		rl.leak();

		if rl.currentRequests < rl.capacity {
			rl.currentRequests++
			log.Printf("Wait: Request allowed after waiting. Reqeusts remaining: %d", rl.capacity - rl.currentRequests)
			rl.mutex.Unlock()
			return nil
		}

		timeUntilNextRequest := rl.rate - time.Since(rl.lastRefreshed)
		if(timeUntilNextRequest <= 0) {
			rl.mutex.Unlock()
			return nil
		}

		select {
		case <-ctx.Done():
			log.Printf("Wait: Request denied due to context timeout")
			return ctx.Err()
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (rl *LeakyBucketRateLimiter) leak() {
	currentTime := time.Now()
	elapsedTime := currentTime.Sub(rl.lastRefreshed)

	leakedRequests := int(elapsedTime / rl.rate)
	if leakedRequests > 0 {
		rl.currentRequests = max(0, rl.currentRequests - leakedRequests)
		rl.lastRefreshed = currentTime
	}
}

func (rl *LeakyBucketRateLimiter) GetCapacity() int {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	return rl.capacity
}

func (rl *LeakyBucketRateLimiter) GetAvailableTokens() int {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	return rl.capacity - rl.currentRequests
}

// SlidingWindowCounterRateLimiter implements a sliding window counter algorithm
// It provides smooth rate limiting by using a weighted average between the current
// and previous window counts, eliminating the burst problem at window boundaries.
type SlidingWindowCounterRateLimiter struct {
	capacity           int
	windowSize         time.Duration
	currentWindowStart time.Time
	currentCount       int
	previousCount      int
	mutex              sync.Mutex
}

func NewSlidingWindowCounterRateLimiter(opts Options) (*SlidingWindowCounterRateLimiter, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	return &SlidingWindowCounterRateLimiter{
		capacity:           opts.Capacity,
		windowSize:         opts.Rate,
		currentWindowStart: time.Now(),
		currentCount:       0,
		previousCount:      0,
		mutex:              sync.Mutex{},
	}, nil
}

func (rl *SlidingWindowCounterRateLimiter) Allow() bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	rl.slideWindow()

	// Calculate weighted count across sliding window
	now := time.Now()
	elapsedInCurrentWindow := now.Sub(rl.currentWindowStart)
	percentageInCurrentWindow := float64(elapsedInCurrentWindow) / float64(rl.windowSize)

	// Weighted count: previous window contribution + current window
	weightedCount := float64(rl.previousCount)*(1-percentageInCurrentWindow) + float64(rl.currentCount)

	if weightedCount < float64(rl.capacity) {
		rl.currentCount++
		return true
	}

	return false
}

func (rl *SlidingWindowCounterRateLimiter) Wait(ctx context.Context) error {
	for {
		rl.mutex.Lock()
		rl.slideWindow()

		now := time.Now()
		elapsedInCurrentWindow := now.Sub(rl.currentWindowStart)
		percentageInCurrentWindow := float64(elapsedInCurrentWindow) / float64(rl.windowSize)

		weightedCount := float64(rl.previousCount)*(1-percentageInCurrentWindow) + float64(rl.currentCount)

		if weightedCount < float64(rl.capacity) {
			rl.currentCount++
			log.Printf("Wait: Request allowed after waiting. Current count: %d", rl.currentCount)
			rl.mutex.Unlock()
			return nil
		}
		rl.mutex.Unlock()

		select {
		case <-ctx.Done():
			log.Printf("Wait: Request denied due to context timeout")
			return ctx.Err()
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// slideWindow checks if we need to advance to a new window
// and updates the window counters accordingly
func (rl *SlidingWindowCounterRateLimiter) slideWindow() {
	now := time.Now()
	elapsedSinceWindowStart := now.Sub(rl.currentWindowStart)

	// If we've passed one or more full windows
	if elapsedSinceWindowStart >= rl.windowSize {
		windowsPassed := int(elapsedSinceWindowStart / rl.windowSize)

		if windowsPassed == 1 {
			// Move to next window: current becomes previous
			rl.previousCount = rl.currentCount
			rl.currentCount = 0
		} else {
			// Multiple windows passed: both reset to 0
			rl.previousCount = 0
			rl.currentCount = 0
		}

		// Advance the window start time
		rl.currentWindowStart = rl.currentWindowStart.Add(time.Duration(windowsPassed) * rl.windowSize)
	}
}

func (rl *SlidingWindowCounterRateLimiter) GetCapacity() int {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	return rl.capacity
}

func (rl *SlidingWindowCounterRateLimiter) GetAvailableTokens() int {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	rl.slideWindow()

	now := time.Now()
	elapsedInCurrentWindow := now.Sub(rl.currentWindowStart)
	percentageInCurrentWindow := float64(elapsedInCurrentWindow) / float64(rl.windowSize)

	weightedCount := float64(rl.previousCount)*(1-percentageInCurrentWindow) + float64(rl.currentCount)
	available := float64(rl.capacity) - weightedCount

	if available < 0 {
		return 0
	}
	return int(available)
}


