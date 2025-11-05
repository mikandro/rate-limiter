package ratelimit

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// DistributedTokenBucketRateLimiter implements a distributed token bucket algorithm using Redis
// It allows rate limiting across multiple servers/instances by storing state in Redis.
type DistributedTokenBucketRateLimiter struct {
	client    *redis.Client
	key       string
	capacity  int
	rate      time.Duration
	ctx       context.Context
}

// DistributedOptions contains configuration for distributed rate limiter
type DistributedOptions struct {
	RedisClient *redis.Client
	Key         string        // Redis key prefix for this rate limiter
	Capacity    int           // Maximum tokens
	Rate        time.Duration // Time to add one token
}

func (o DistributedOptions) validate() error {
	if o.RedisClient == nil {
		return fmt.Errorf("redis client cannot be nil")
	}
	if o.Key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if o.Capacity <= 0 {
		return fmt.Errorf("capacity must be greater than 0")
	}
	if o.Rate <= 0 {
		return fmt.Errorf("rate must be greater than 0")
	}
	return nil
}

// Lua script for atomic token bucket operations
// This ensures thread-safety across distributed instances
const tokenBucketScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])  -- nanoseconds per token
local now = tonumber(ARGV[3])

-- Get current state
local tokens = redis.call('HGET', key, 'tokens')
local last_refreshed = redis.call('HGET', key, 'last_refreshed')

-- Initialize if not exists
if not tokens then
    tokens = capacity
    last_refreshed = now
else
    tokens = tonumber(tokens)
    last_refreshed = tonumber(last_refreshed)

    -- Calculate tokens to add based on elapsed time
    local elapsed = now - last_refreshed
    if elapsed > 0 then
        local tokens_to_add = math.floor(elapsed / rate)
        if tokens_to_add > 0 then
            tokens = math.min(capacity, tokens + tokens_to_add)
            last_refreshed = last_refreshed + (tokens_to_add * rate)
        end
    end
end

-- Try to consume a token
if tokens > 0 then
    tokens = tokens - 1
    redis.call('HSET', key, 'tokens', tokens)
    redis.call('HSET', key, 'last_refreshed', last_refreshed)
    redis.call('EXPIRE', key, math.ceil(capacity * rate / 1000000000) + 60)  -- TTL in seconds with buffer
    return {1, tokens}  -- Success, return remaining tokens
else
    redis.call('HSET', key, 'tokens', tokens)
    redis.call('HSET', key, 'last_refreshed', last_refreshed)
    return {0, tokens}  -- Failure, no tokens available
end
`

// Lua script for checking available tokens without consuming
const checkTokensScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local tokens = redis.call('HGET', key, 'tokens')
local last_refreshed = redis.call('HGET', key, 'last_refreshed')

if not tokens then
    return capacity
else
    tokens = tonumber(tokens)
    last_refreshed = tonumber(last_refreshed)

    local elapsed = now - last_refreshed
    if elapsed > 0 then
        local tokens_to_add = math.floor(elapsed / rate)
        tokens = math.min(capacity, tokens + tokens_to_add)
    end

    return tokens
end
`

func NewDistributedTokenBucketRateLimiter(opts DistributedOptions) (*DistributedTokenBucketRateLimiter, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	return &DistributedTokenBucketRateLimiter{
		client:   opts.RedisClient,
		key:      opts.Key,
		capacity: opts.Capacity,
		rate:     opts.Rate,
		ctx:      context.Background(),
	}, nil
}

func (rl *DistributedTokenBucketRateLimiter) Allow() bool {
	now := time.Now().UnixNano()

	result, err := rl.client.Eval(rl.ctx, tokenBucketScript, []string{rl.key},
		rl.capacity,
		rl.rate.Nanoseconds(),
		now).Result()

	if err != nil {
		log.Printf("Error executing rate limit script: %v", err)
		return false
	}

	// Parse result
	resultSlice, ok := result.([]interface{})
	if !ok || len(resultSlice) < 2 {
		log.Printf("Unexpected result format from Redis")
		return false
	}

	allowed, ok := resultSlice[0].(int64)
	if !ok {
		log.Printf("Failed to parse allowed status")
		return false
	}

	return allowed == 1
}

func (rl *DistributedTokenBucketRateLimiter) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			log.Printf("Wait: Request allowed after waiting")
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

func (rl *DistributedTokenBucketRateLimiter) GetCapacity() int {
	return rl.capacity
}

func (rl *DistributedTokenBucketRateLimiter) GetAvailableTokens() int {
	now := time.Now().UnixNano()

	result, err := rl.client.Eval(rl.ctx, checkTokensScript, []string{rl.key},
		rl.capacity,
		rl.rate.Nanoseconds(),
		now).Result()

	if err != nil {
		log.Printf("Error checking available tokens: %v", err)
		return 0
	}

	// Convert result to int
	switch v := result.(type) {
	case int64:
		return int(v)
	case string:
		tokens, err := strconv.Atoi(v)
		if err != nil {
			log.Printf("Failed to parse token count: %v", err)
			return 0
		}
		return tokens
	default:
		log.Printf("Unexpected token count type: %T", result)
		return 0
	}
}

// Close cleans up resources (optional, can be called when done with the limiter)
func (rl *DistributedTokenBucketRateLimiter) Close() error {
	return nil // Redis client should be managed by the caller
}
