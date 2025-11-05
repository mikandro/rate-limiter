package ratelimit

import (
	"context"
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



