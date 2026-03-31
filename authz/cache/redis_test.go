package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jasonKoogler/authz/types"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRedisContainer(t *testing.T) (*redis.Client, func()) {
	// Create a new miniredis server
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}

	// Create a new Redis client
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Test the connection
	ctx := context.Background()
	_, err = client.Ping(ctx).Result()
	if err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Return the client and a cleanup function
	return client, func() {
		client.Close()
		mr.Close()
	}
}

func TestNewRedisCache(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	tests := []struct {
		name      string
		client    *redis.Client
		keyPrefix string
	}{
		{
			name:      "default prefix",
			client:    client,
			keyPrefix: "agent:cache:",
		},
		{
			name:      "custom prefix",
			client:    client,
			keyPrefix: "custom:",
		},
		{
			name:      "empty prefix",
			client:    client,
			keyPrefix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewRedisCache(tt.client, WithKeyPrefix(tt.keyPrefix))
			assert.NotNil(t, cache)
			assert.Equal(t, tt.client, cache.client)
			assert.Equal(t, tt.keyPrefix, cache.keyPrefix)
		})
	}
}

func TestRedisCacheGetSet(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	cache := NewRedisCache(client, WithKeyPrefix("test:"))
	require.NotNil(t, cache)

	// Test setting and getting a value
	key := "test-key"
	decision := types.Decision{
		Allowed: true,
		Reason:  "test reason",
	}

	// Set the value
	cache.Set(key, decision, time.Minute)

	// Get the value
	result, found := cache.Get(key)
	assert.True(t, found)
	assert.Equal(t, decision.Allowed, result.Allowed)
	assert.Equal(t, decision.Reason, result.Reason)

	// Get a non-existent key
	_, found = cache.Get("non-existent")
	assert.False(t, found)
}

func TestRedisCacheExpiration(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	cache := NewRedisCache(client, WithKeyPrefix("test:"))
	require.NotNil(t, cache)

	// Test setting and getting a value with expiration
	key := "test-key"
	decision := types.Decision{
		Allowed: true,
		Reason:  "test reason",
	}

	// Set the value with a short TTL
	cache.Set(key, decision, time.Second)

	// Get the value immediately
	result, found := cache.Get(key)
	assert.True(t, found)
	assert.Equal(t, decision.Allowed, result.Allowed)

	// Use miniredis to fast-forward time
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer server.Close()

	// Create a new client connected to the new server
	newClient := redis.NewClient(&redis.Options{
		Addr: server.Addr(),
	})
	defer newClient.Close()

	// Create a new cache with the new client
	newCache := NewRedisCache(newClient, WithKeyPrefix("test:"))

	// Set a key with expiration
	newCache.Set(key, decision, time.Second)

	// Fast-forward time
	server.FastForward(2 * time.Second)

	// Get the value after expiration
	_, found = newCache.Get(key)
	assert.False(t, found, "Key should have expired")
}

func TestRedisCacheDelete(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	cache := NewRedisCache(client, WithKeyPrefix("test:"))
	require.NotNil(t, cache)

	// Test deleting a value
	key := "test-key"
	decision := types.Decision{
		Allowed: true,
		Reason:  "test reason",
	}

	// Set the value
	cache.Set(key, decision, time.Minute)

	// Verify it was set
	_, found := cache.Get(key)
	assert.True(t, found)

	// Delete the value
	cache.Delete(key)

	// Verify it was deleted
	_, found = cache.Get(key)
	assert.False(t, found)
}

func TestRedisCacheClear(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	cache := NewRedisCache(client, WithKeyPrefix("test:"))
	require.NotNil(t, cache)

	// Set multiple values
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("test-key-%d", i)
		decision := types.Decision{
			Allowed: true,
			Reason:  fmt.Sprintf("test reason %d", i),
		}
		cache.Set(key, decision, time.Minute)
	}

	// Create another cache with a different prefix
	otherCache := NewRedisCache(client, WithKeyPrefix("other:"))
	otherKey := "other-key"
	otherDecision := types.Decision{
		Allowed: false,
		Reason:  "other reason",
	}
	otherCache.Set(otherKey, otherDecision, time.Minute)

	// Clear the first cache
	cache.Clear()

	// Verify all keys with the first prefix are gone
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("test-key-%d", i)
		_, found := cache.Get(key)
		assert.False(t, found)
	}

	// Verify the key with the other prefix still exists
	result, found := otherCache.Get(otherKey)
	assert.True(t, found)
	assert.Equal(t, otherDecision.Allowed, result.Allowed)
}

func TestRedisCacheKeyPrefix(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	// Create two caches with different prefixes
	cache1 := NewRedisCache(client, WithKeyPrefix("prefix1:"))
	cache2 := NewRedisCache(client, WithKeyPrefix("prefix2:"))

	// Set the same key in both caches
	key := "same-key"
	decision1 := types.Decision{
		Allowed: true,
		Reason:  "reason 1",
	}
	decision2 := types.Decision{
		Allowed: false,
		Reason:  "reason 2",
	}

	cache1.Set(key, decision1, time.Minute)
	cache2.Set(key, decision2, time.Minute)

	// Get the values from both caches
	result1, found1 := cache1.Get(key)
	result2, found2 := cache2.Get(key)

	// Verify both values were found and are different
	assert.True(t, found1)
	assert.True(t, found2)
	assert.Equal(t, decision1.Allowed, result1.Allowed)
	assert.Equal(t, decision2.Allowed, result2.Allowed)
	assert.NotEqual(t, result1.Allowed, result2.Allowed)
}

func TestRedisCacheErrorHandling(t *testing.T) {
	// Create a miniredis server
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	// Create a Redis client
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	cache := NewRedisCache(client, WithKeyPrefix("test:"))
	require.NotNil(t, cache)

	// Test with a valid key
	key := "test-key"
	decision := types.Decision{
		Allowed: true,
		Reason:  "test reason",
	}
	cache.Set(key, decision, time.Minute)

	// Close the Redis server to simulate a connection error
	mr.Close()

	// Attempt to get the value after the connection is closed
	_, found := cache.Get(key)
	assert.False(t, found)

	// Attempt to set a value after the connection is closed
	cache.Set("new-key", decision, time.Minute)

	// Attempt to delete a value after the connection is closed
	cache.Delete(key)

	// Attempt to clear the cache after the connection is closed
	cache.Clear()
}
