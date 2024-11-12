package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(5, time.Second)
	assert.Equal(t, rl.capacity, 5)
	assert.Equal(t, rl.tokens, 5)
	assert.Equal(t, rl.rate, time.Second)
}

func TestAllow(t *testing.T) {
	rl := NewRateLimiter(2, time.Second)
	assert.True(t, rl.Allow(), "1'st allowed")
	assert.True(t, rl.Allow(), "2'nd allowed")
	assert.False(t, rl.Allow(), "3'rd blocked")
}

func TestRefill(t *testing.T) {
	rl := NewRateLimiter(2, 500*time.Millisecond)

	assert.True(t, rl.Allow(), "1'st allowed")
	assert.True(t, rl.Allow(), "2'nd allowed")
	assert.False(t, rl.Allow(), "3'rd blocked")

	// refill 1 token
	time.Sleep(600 * time.Millisecond)

	rl.mutex.Lock()
	rl.refill()
	rl.mutex.Unlock()

	assert.True(t, rl.Allow(), "1'st allowed")
	assert.False(t, rl.Allow(), "2'nd blocked")
}

func TestWait(t *testing.T) {
	rl := NewRateLimiter(1, 500*time.Millisecond)

	assert.True(t, rl.Allow())

	// Start a context with a timeout of 1 second
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := rl.Wait(ctx)
	assert.NoError(t, err, "Wait should eventually succeed")

	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = rl.Wait(ctx)
	assert.Error(t, err, "Wait should fail due to context timeout")
}

func Test0Capacity(t *testing.T) {
	assert.Panics(t, func() { NewRateLimiter(0, 1*time.Second) }, "Creating limiter with 0 capacity should panic")
}

func Test0Rate(t *testing.T) {
	assert.Panics(t, func() { NewRateLimiter(1, 0) }, "Creating limiter with 0 capacity should panic")
}
