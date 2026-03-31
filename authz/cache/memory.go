package cache

import (
	"sync"
	"time"

	"github.com/jasonKoogler/authz/types"
)

// MemoryCache implements the Cache interface with an in-memory LRU cache
type MemoryCache struct {
	entries    map[string]memoryCacheEntry
	keysByTime []string
	maxEntries int
	mutex      sync.RWMutex
}

type memoryCacheEntry struct {
	decision   types.Decision
	expiration time.Time
	lastAccess time.Time
}

// NewMemoryCache creates a new memory cache with the specified max entries
func NewMemoryCache(maxEntries int) *MemoryCache {
	if maxEntries <= 0 {
		maxEntries = 1000 // Default max entries
	}
	return &MemoryCache{
		entries:    make(map[string]memoryCacheEntry),
		keysByTime: make([]string, 0, maxEntries),
		maxEntries: maxEntries,
	}
}

// Get retrieves a cached decision for a key
func (c *MemoryCache) Get(key string) (types.Decision, bool) {
	c.mutex.RLock()
	entry, exists := c.entries[key]
	c.mutex.RUnlock()

	if !exists {
		return types.Decision{}, false
	}

	// Check if entry is expired
	if !entry.expiration.IsZero() && time.Now().After(entry.expiration) {
		c.mutex.Lock()
		delete(c.entries, key)
		c.pruneKeyByTime(key)
		c.mutex.Unlock()
		return types.Decision{}, false
	}

	// Update last access time
	c.mutex.Lock()
	entry.lastAccess = time.Now()
	c.entries[key] = entry
	c.mutex.Unlock()

	return entry.decision, true
}

// Set stores a decision for a key with optional TTL
func (c *MemoryCache) Set(key string, decision types.Decision, ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Calculate expiration time
	var expiration time.Time
	if ttl > 0 {
		expiration = time.Now().Add(ttl)
	}

	// Check if we need to evict an entry
	if len(c.entries) >= c.maxEntries && c.maxEntries > 0 {
		c.evictLRU()
	}

	// Add new entry
	now := time.Now()
	c.entries[key] = memoryCacheEntry{
		decision:   decision,
		expiration: expiration,
		lastAccess: now,
	}

	// Update keysByTime
	c.keysByTime = append(c.keysByTime, key)
}

// Delete removes an entry from the cache
func (c *MemoryCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.entries, key)
	c.pruneKeyByTime(key)
}

// Clear empties the cache
func (c *MemoryCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.entries = make(map[string]memoryCacheEntry)
	c.keysByTime = make([]string, 0, c.maxEntries)
}

// evictLRU evicts the least recently used entry
func (c *MemoryCache) evictLRU() {
	if len(c.keysByTime) == 0 {
		return
	}

	// Find the least recently used key
	var oldestKey string
	var oldestTime time.Time

	// Initialize with the first key
	oldestKey = c.keysByTime[0]
	if entry, exists := c.entries[oldestKey]; exists {
		oldestTime = entry.lastAccess
	}

	// Check all keys to find the oldest
	for key, entry := range c.entries {
		if oldestTime.IsZero() || entry.lastAccess.Before(oldestTime) {
			oldestTime = entry.lastAccess
			oldestKey = key
		}
	}

	// Evict the oldest entry
	delete(c.entries, oldestKey)
	c.pruneKeyByTime(oldestKey)
}

// pruneKeyByTime removes a key from the keysByTime slice
func (c *MemoryCache) pruneKeyByTime(key string) {
	for i, k := range c.keysByTime {
		if k == key {
			c.keysByTime = append(c.keysByTime[:i], c.keysByTime[i+1:]...)
			break
		}
	}
}

// CleanupExpired removes expired entries from the cache
func (c *MemoryCache) CleanupExpired() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if !entry.expiration.IsZero() && now.After(entry.expiration) {
			delete(c.entries, key)
			c.pruneKeyByTime(key)
		}
	}
}

// StartCleanup starts a background goroutine to periodically clean up expired cache entries
func (c *MemoryCache) StartCleanup(interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute * 5
	}

	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			c.CleanupExpired()
		}
	}()
}
