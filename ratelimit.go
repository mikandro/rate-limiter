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
	waterLevel int
	leakRate time.Duration
	lastLeak time.Time
	mutex sync.Mutex
}

func NewLeakyBucketRateLimiter(opts Options) (*LeakyBucketRateLimiter, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	return &LeakyBucketRateLimiter{
		capacity: opts.Capacity,
		waterLevel: opts.Capacity,
		leakRate: opts.Rate,
		lastLeak: time.Now(),
		mutex: sync.Mutex{},
	}, nil
}

func (rl *LeakyBucketRateLimiter) Allow() bool {
// implementation	
	
}

func (rl *LeakyBucketRateLimiter) Wait(ctx context.Context) error {
	// implementation
}

func (rl *LeakyBucketRateLimiter) leak() {
	// implementation
}

func (rl *LeakyBucketRateLimiter) GetCapacity() int {
	// implementation
}

func (rl *LeakyBucketRateLimiter) GetAvailableTokens() int {
	// implementation
}
