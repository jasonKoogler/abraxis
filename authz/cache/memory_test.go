package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/jasonKoogler/authz/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryCache(t *testing.T) {
	tests := []struct {
		name       string
		maxEntries int
		expected   int
	}{
		{
			name:       "positive max entries",
			maxEntries: 100,
			expected:   100,
		},
		{
			name:       "zero max entries",
			maxEntries: 0,
			expected:   1000, // Default value
		},
		{
			name:       "negative max entries",
			maxEntries: -10,
			expected:   1000, // Default value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewMemoryCache(tt.maxEntries)
			assert.NotNil(t, cache)
			assert.Equal(t, tt.expected, cache.maxEntries)
			assert.NotNil(t, cache.entries)
			assert.NotNil(t, cache.keysByTime)
		})
	}
}

func TestMemoryCacheGetSet(t *testing.T) {
	cache := NewMemoryCache(10)
	require.NotNil(t, cache)

	// Test setting and getting a value
	key := "test-key"
	decision := types.Decision{
		Allowed:   true,
		Reason:    "test reason",
		Timestamp: time.Now(),
	}

	// Initially, the key should not exist
	_, exists := cache.Get(key)
	assert.False(t, exists)

	// Set the value
	cache.Set(key, decision, 0)

	// Now the key should exist
	retrieved, exists := cache.Get(key)
	assert.True(t, exists)
	assert.Equal(t, decision.Allowed, retrieved.Allowed)
	assert.Equal(t, decision.Reason, retrieved.Reason)
}

func TestMemoryCacheExpiration(t *testing.T) {
	cache := NewMemoryCache(10)
	require.NotNil(t, cache)

	// Test with expiration
	key := "expiring-key"
	decision := types.Decision{
		Allowed:   true,
		Reason:    "test reason",
		Timestamp: time.Now(),
	}

	// Set with a short TTL
	cache.Set(key, decision, 50*time.Millisecond)

	// Immediately after setting, the key should exist
	_, exists := cache.Get(key)
	assert.True(t, exists)

	// Wait for the TTL to expire
	time.Sleep(100 * time.Millisecond)

	// After expiration, the key should not exist
	_, exists = cache.Get(key)
	assert.False(t, exists)
}

func TestMemoryCacheDelete(t *testing.T) {
	cache := NewMemoryCache(10)
	require.NotNil(t, cache)

	// Set some values
	cache.Set("key1", types.Decision{Allowed: true}, 0)
	cache.Set("key2", types.Decision{Allowed: false}, 0)

	// Verify they exist
	_, exists1 := cache.Get("key1")
	_, exists2 := cache.Get("key2")
	assert.True(t, exists1)
	assert.True(t, exists2)

	// Delete one key
	cache.Delete("key1")

	// Verify key1 is gone but key2 still exists
	_, exists1 = cache.Get("key1")
	_, exists2 = cache.Get("key2")
	assert.False(t, exists1)
	assert.True(t, exists2)

	// Delete a non-existent key (should not panic)
	cache.Delete("non-existent-key")
}

func TestMemoryCacheClear(t *testing.T) {
	cache := NewMemoryCache(10)
	require.NotNil(t, cache)

	// Set some values
	cache.Set("key1", types.Decision{Allowed: true}, 0)
	cache.Set("key2", types.Decision{Allowed: false}, 0)

	// Verify they exist
	_, exists1 := cache.Get("key1")
	_, exists2 := cache.Get("key2")
	assert.True(t, exists1)
	assert.True(t, exists2)

	// Clear the cache
	cache.Clear()

	// Verify all keys are gone
	_, exists1 = cache.Get("key1")
	_, exists2 = cache.Get("key2")
	assert.False(t, exists1)
	assert.False(t, exists2)
	assert.Empty(t, cache.entries)
	assert.Empty(t, cache.keysByTime)
}

func TestMemoryCacheLRUEviction(t *testing.T) {
	// Create a cache with a small capacity
	cache := NewMemoryCache(3)
	require.NotNil(t, cache)

	// Set more values than the capacity
	cache.Set("key1", types.Decision{Allowed: true, Reason: "1"}, 0)
	cache.Set("key2", types.Decision{Allowed: true, Reason: "2"}, 0)
	cache.Set("key3", types.Decision{Allowed: true, Reason: "3"}, 0)

	// Access key1 to make it the most recently used
	_, _ = cache.Get("key1")

	// Add a new key, which should evict the least recently used (key2)
	cache.Set("key4", types.Decision{Allowed: true, Reason: "4"}, 0)

	// Verify key2 was evicted
	_, exists1 := cache.Get("key1")
	_, exists2 := cache.Get("key2")
	_, exists3 := cache.Get("key3")
	_, exists4 := cache.Get("key4")
	assert.True(t, exists1)
	assert.False(t, exists2) // This should be evicted
	assert.True(t, exists3)
	assert.True(t, exists4)
}

func TestMemoryCacheCleanupExpired(t *testing.T) {
	cache := NewMemoryCache(10)
	require.NotNil(t, cache)

	// Set some values with different expiration times
	cache.Set("non-expiring", types.Decision{Allowed: true, Reason: "non-expiring"}, 0)
	cache.Set("quick-expiring", types.Decision{Allowed: true, Reason: "quick"}, 50*time.Millisecond)
	cache.Set("slow-expiring", types.Decision{Allowed: true, Reason: "slow"}, 150*time.Millisecond)

	// Wait for the quick-expiring to expire
	time.Sleep(100 * time.Millisecond)

	// Run cleanup
	cache.CleanupExpired()

	// Verify the quick-expiring is gone, but others remain
	_, existsNonExpiring := cache.Get("non-expiring")
	_, existsQuickExpiring := cache.Get("quick-expiring")
	_, existsSlowExpiring := cache.Get("slow-expiring")
	assert.True(t, existsNonExpiring)
	assert.False(t, existsQuickExpiring)
	assert.True(t, existsSlowExpiring)

	// Wait for the slow-expiring to expire
	time.Sleep(100 * time.Millisecond)

	// Run cleanup again
	cache.CleanupExpired()

	// Verify the slow-expiring is now gone too
	_, existsNonExpiring = cache.Get("non-expiring")
	_, existsSlowExpiring = cache.Get("slow-expiring")
	assert.True(t, existsNonExpiring)
	assert.False(t, existsSlowExpiring)
}

func TestMemoryCacheStartCleanup(t *testing.T) {
	cache := NewMemoryCache(10)
	require.NotNil(t, cache)

	// Set a value with a short expiration
	cache.Set("expiring", types.Decision{Allowed: true}, 100*time.Millisecond)

	// Start the cleanup routine with a short interval
	cache.StartCleanup(50 * time.Millisecond)

	// Wait for the cleanup to run a couple of times
	time.Sleep(200 * time.Millisecond)

	// Verify the expired entry is gone
	_, exists := cache.Get("expiring")
	assert.False(t, exists)
}

func TestMemoryCacheConcurrency(t *testing.T) {
	cache := NewMemoryCache(100)
	require.NotNil(t, cache)

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				cache.Set(key, types.Decision{Allowed: true}, 0)
				_, _ = cache.Get(key)
				if j%2 == 0 {
					cache.Delete(key)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}

	// The test passes if there are no race conditions or panics
}
