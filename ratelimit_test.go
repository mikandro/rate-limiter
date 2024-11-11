package ratelimit

import (
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
