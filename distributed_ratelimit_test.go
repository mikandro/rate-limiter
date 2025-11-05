package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRedis creates a miniredis server and returns a redis client
func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err, "Failed to start miniredis")

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, mr
}

func TestNewDistributedTokenBucketRateLimiter(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	tests := []struct {
		name    string
		opts    DistributedOptions
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid options",
			opts: DistributedOptions{
				RedisClient: client,
				Key:         "test:limiter",
				Capacity:    5,
				Rate:        time.Second,
			},
			wantErr: false,
		},
		{
			name: "nil redis client",
			opts: DistributedOptions{
				RedisClient: nil,
				Key:         "test:limiter",
				Capacity:    5,
				Rate:        time.Second,
			},
			wantErr: true,
			errMsg:  "redis client cannot be nil",
		},
		{
			name: "empty key",
			opts: DistributedOptions{
				RedisClient: client,
				Key:         "",
				Capacity:    5,
				Rate:        time.Second,
			},
			wantErr: true,
			errMsg:  "key cannot be empty",
		},
		{
			name: "zero capacity",
			opts: DistributedOptions{
				RedisClient: client,
				Key:         "test:limiter",
				Capacity:    0,
				Rate:        time.Second,
			},
			wantErr: true,
			errMsg:  "capacity must be greater than 0",
		},
		{
			name: "negative capacity",
			opts: DistributedOptions{
				RedisClient: client,
				Key:         "test:limiter",
				Capacity:    -5,
				Rate:        time.Second,
			},
			wantErr: true,
			errMsg:  "capacity must be greater than 0",
		},
		{
			name: "zero rate",
			opts: DistributedOptions{
				RedisClient: client,
				Key:         "test:limiter",
				Capacity:    5,
				Rate:        0,
			},
			wantErr: true,
			errMsg:  "rate must be greater than 0",
		},
		{
			name: "negative rate",
			opts: DistributedOptions{
				RedisClient: client,
				Key:         "test:limiter",
				Capacity:    5,
				Rate:        -time.Second,
			},
			wantErr: true,
			errMsg:  "rate must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter, err := NewDistributedTokenBucketRateLimiter(tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, limiter)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, limiter)
				assert.Equal(t, tt.opts.Capacity, limiter.GetCapacity())
			}
		})
	}
}

func TestDistributedTokenBucketAllow(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limiter, err := NewDistributedTokenBucketRateLimiter(DistributedOptions{
		RedisClient: client,
		Key:         "test:allow",
		Capacity:    2,
		Rate:        time.Second,
	})
	require.NoError(t, err)

	// First two requests should be allowed
	assert.True(t, limiter.Allow(), "1st request should be allowed")
	assert.True(t, limiter.Allow(), "2nd request should be allowed")

	// Third request should be denied (no tokens left)
	assert.False(t, limiter.Allow(), "3rd request should be denied")
}

func TestDistributedTokenBucketRefill(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limiter, err := NewDistributedTokenBucketRateLimiter(DistributedOptions{
		RedisClient: client,
		Key:         "test:refill",
		Capacity:    2,
		Rate:        100 * time.Millisecond, // 1 token every 100ms
	})
	require.NoError(t, err)

	// Consume all tokens
	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())
	assert.False(t, limiter.Allow(), "Should be denied with no tokens")

	// Wait for one token to refill
	time.Sleep(150 * time.Millisecond)

	// Should now be allowed
	assert.True(t, limiter.Allow(), "Should be allowed after token refill")
	assert.False(t, limiter.Allow(), "Should be denied again")
}

func TestDistributedTokenBucketWait(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limiter, err := NewDistributedTokenBucketRateLimiter(DistributedOptions{
		RedisClient: client,
		Key:         "test:wait",
		Capacity:    1,
		Rate:        100 * time.Millisecond,
	})
	require.NoError(t, err)

	// Consume the only token
	assert.True(t, limiter.Allow())

	// Wait should succeed after token refills
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = limiter.Wait(ctx)
	elapsed := time.Since(start)

	assert.NoError(t, err, "Wait should eventually succeed")
	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond, "Wait should take at least 100ms")
}

func TestDistributedTokenBucketWaitTimeout(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limiter, err := NewDistributedTokenBucketRateLimiter(DistributedOptions{
		RedisClient: client,
		Key:         "test:wait-timeout",
		Capacity:    1,
		Rate:        time.Second, // Long refill time
	})
	require.NoError(t, err)

	// Consume the only token
	assert.True(t, limiter.Allow())

	// Wait should timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = limiter.Wait(ctx)
	assert.Error(t, err, "Wait should fail due to timeout")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestDistributedTokenBucketGetAvailableTokens(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limiter, err := NewDistributedTokenBucketRateLimiter(DistributedOptions{
		RedisClient: client,
		Key:         "test:available",
		Capacity:    5,
		Rate:        time.Second,
	})
	require.NoError(t, err)

	// Initially should have full capacity
	assert.Equal(t, 5, limiter.GetAvailableTokens())

	// After consuming some tokens
	limiter.Allow()
	limiter.Allow()

	assert.Equal(t, 3, limiter.GetAvailableTokens())
}

func TestDistributedTokenBucketMultipleInstances(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	// Create two limiters with the same key (simulating distributed instances)
	limiter1, err := NewDistributedTokenBucketRateLimiter(DistributedOptions{
		RedisClient: client,
		Key:         "test:distributed",
		Capacity:    3,
		Rate:        time.Second,
	})
	require.NoError(t, err)

	limiter2, err := NewDistributedTokenBucketRateLimiter(DistributedOptions{
		RedisClient: client,
		Key:         "test:distributed", // Same key
		Capacity:    3,
		Rate:        time.Second,
	})
	require.NoError(t, err)

	// Consume tokens from both limiters
	assert.True(t, limiter1.Allow(), "limiter1: 1st request")
	assert.True(t, limiter2.Allow(), "limiter2: 1st request")
	assert.True(t, limiter1.Allow(), "limiter1: 2nd request")

	// Should be out of tokens for both limiters (they share state)
	assert.False(t, limiter1.Allow(), "limiter1: should be denied")
	assert.False(t, limiter2.Allow(), "limiter2: should be denied")

	// Both should see the same available tokens
	tokens1 := limiter1.GetAvailableTokens()
	tokens2 := limiter2.GetAvailableTokens()
	assert.Equal(t, tokens1, tokens2, "Both limiters should see same token count")
	assert.Equal(t, 0, tokens1, "Should have 0 tokens remaining")
}

func TestDistributedTokenBucketDifferentKeys(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	// Create two limiters with different keys (independent rate limits)
	limiter1, err := NewDistributedTokenBucketRateLimiter(DistributedOptions{
		RedisClient: client,
		Key:         "test:user1",
		Capacity:    2,
		Rate:        time.Second,
	})
	require.NoError(t, err)

	limiter2, err := NewDistributedTokenBucketRateLimiter(DistributedOptions{
		RedisClient: client,
		Key:         "test:user2", // Different key
		Capacity:    2,
		Rate:        time.Second,
	})
	require.NoError(t, err)

	// Consume all tokens from limiter1
	assert.True(t, limiter1.Allow())
	assert.True(t, limiter1.Allow())
	assert.False(t, limiter1.Allow(), "limiter1 should be out of tokens")

	// limiter2 should still have tokens (independent)
	assert.True(t, limiter2.Allow(), "limiter2 should still allow")
	assert.True(t, limiter2.Allow(), "limiter2 should still allow")
	assert.False(t, limiter2.Allow(), "limiter2 should now be out of tokens")
}

func TestDistributedTokenBucketConcurrency(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limiter, err := NewDistributedTokenBucketRateLimiter(DistributedOptions{
		RedisClient: client,
		Key:         "test:concurrent",
		Capacity:    10,
		Rate:        time.Millisecond * 10,
	})
	require.NoError(t, err)

	// Try to consume tokens concurrently
	successCount := 0
	done := make(chan bool)

	for i := 0; i < 20; i++ {
		go func() {
			if limiter.Allow() {
				successCount++
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Should have allowed exactly 10 requests (capacity)
	// Note: This might be flaky due to race conditions, but Redis Lua scripts should ensure atomicity
	assert.LessOrEqual(t, successCount, 10, "Should not exceed capacity")
	assert.Greater(t, successCount, 0, "Should allow some requests")
}

func TestDistributedTokenBucketExpiry(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limiter, err := NewDistributedTokenBucketRateLimiter(DistributedOptions{
		RedisClient: client,
		Key:         "test:expiry",
		Capacity:    5,
		Rate:        time.Second,
	})
	require.NoError(t, err)

	// Consume a token (this will set expiry in Redis)
	assert.True(t, limiter.Allow())

	// Check that the key has a TTL set
	ttl := client.TTL(context.Background(), "test:expiry").Val()
	assert.Greater(t, ttl, time.Duration(0), "Key should have a TTL set")
}

func TestDistributedTokenBucketCapacityLimit(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limiter, err := NewDistributedTokenBucketRateLimiter(DistributedOptions{
		RedisClient: client,
		Key:         "test:capacity-limit",
		Capacity:    3,
		Rate:        50 * time.Millisecond,
	})
	require.NoError(t, err)

	// Consume all tokens
	limiter.Allow()
	limiter.Allow()
	limiter.Allow()

	// Wait for longer than it takes to refill
	time.Sleep(500 * time.Millisecond)

	// Should not have more than capacity
	assert.LessOrEqual(t, limiter.GetAvailableTokens(), 3, "Should not exceed capacity")
}
