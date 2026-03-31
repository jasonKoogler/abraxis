package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jasonKoogler/authz/types"
	"github.com/redis/go-redis/v9"
)

// RedisCache implements the agent.Cache interface using Redis as a backend
type RedisCache struct {
	client    *redis.Client
	keyPrefix string
}

// RedisCacheOption is a functional option for configuring RedisCache
type RedisCacheOption func(*RedisCache)

// WithKeyPrefix sets the prefix for Redis keys
func WithKeyPrefix(prefix string) RedisCacheOption {
	return func(c *RedisCache) {
		c.keyPrefix = prefix
	}
}

// NewRedisCache creates a new Redis-based cache provider
func NewRedisCache(client *redis.Client, options ...RedisCacheOption) *RedisCache {
	cache := &RedisCache{
		client:    client,
		keyPrefix: "agent:cache:",
	}

	// Apply options
	for _, option := range options {
		option(cache)
	}

	return cache
}

// Get retrieves a cached decision for a key
func (c *RedisCache) Get(key string) (types.Decision, bool) {
	// Construct the Redis key
	redisKey := fmt.Sprintf("%s%s", c.keyPrefix, key)

	// Get from Redis
	ctx := context.Background()
	val, err := c.client.Get(ctx, redisKey).Result()
	if err == redis.Nil {
		// Key does not exist
		return types.Decision{}, false
	} else if err != nil {
		// Error getting from Redis
		return types.Decision{}, false
	}

	// Parse the decision
	var decision types.Decision
	if err := json.Unmarshal([]byte(val), &decision); err != nil {
		// Error parsing decision
		return types.Decision{}, false
	}

	return decision, true
}

// Set stores a decision for a key with optional TTL
func (c *RedisCache) Set(key string, decision types.Decision, ttl time.Duration) {
	// Construct the Redis key
	redisKey := fmt.Sprintf("%s%s", c.keyPrefix, key)

	// Convert decision to JSON
	decisionJSON, err := json.Marshal(decision)
	if err != nil {
		// Error marshaling decision
		return
	}

	// Set in Redis with TTL
	ctx := context.Background()
	c.client.Set(ctx, redisKey, decisionJSON, ttl)
}

// Delete removes an entry from the cache
func (c *RedisCache) Delete(key string) {
	// Construct the Redis key
	redisKey := fmt.Sprintf("%s%s", c.keyPrefix, key)

	// Delete from Redis
	ctx := context.Background()
	c.client.Del(ctx, redisKey)
}

// Clear empties the cache
func (c *RedisCache) Clear() {
	// Find all keys with our prefix
	ctx := context.Background()
	pattern := fmt.Sprintf("%s*", c.keyPrefix)

	// Scan for matching keys
	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()

	// Delete all matching keys
	keysToDelete := []string{}
	for iter.Next(ctx) {
		keysToDelete = append(keysToDelete, iter.Val())

		// Delete in batches of 1000 keys
		if len(keysToDelete) >= 1000 {
			c.client.Del(ctx, keysToDelete...)
			keysToDelete = []string{}
		}
	}

	// Delete any remaining keys
	if len(keysToDelete) > 0 {
		c.client.Del(ctx, keysToDelete...)
	}

	if err := iter.Err(); err != nil {
		// Error scanning keys
		return
	}
}
