package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mikandro/rate-limiter"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// Test Redis connection
	ctx := context.Background()
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	fmt.Println("Connected to Redis successfully")

	// Create a distributed rate limiter
	// Allows 5 requests per second across all instances
	limiter, err := ratelimit.NewDistributedTokenBucketRateLimiter(ratelimit.DistributedOptions{
		RedisClient: redisClient,
		Key:         "my-app:rate-limiter:api",
		Capacity:    5,
		Rate:        time.Second / 5, // One token every 200ms
	})
	if err != nil {
		log.Fatalf("Failed to create rate limiter: %v", err)
	}

	fmt.Println("\nTesting Distributed Token Bucket Rate Limiter:")
	fmt.Println("================================================")

	// Simulate requests from multiple instances
	for i := 1; i <= 10; i++ {
		if limiter.Allow() {
			fmt.Printf("Request %d: ✓ ALLOWED (Tokens remaining: %d)\n", i, limiter.GetAvailableTokens())
		} else {
			fmt.Printf("Request %d: ✗ DENIED (Tokens remaining: %d)\n", i, limiter.GetAvailableTokens())
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Test Wait method
	fmt.Println("\nTesting Wait() method with timeout:")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = limiter.Wait(ctx)
	if err != nil {
		fmt.Printf("Wait failed: %v\n", err)
	} else {
		fmt.Printf("Wait succeeded: Request allowed\n")
	}

	// Clean up
	defer redisClient.Close()
}
