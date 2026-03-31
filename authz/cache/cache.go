// Package cache provides cache implementations for the Agent library
package cache

import (
	"time"

	"github.com/jasonKoogler/abraxis/authz/types"
)

// CacheConfig contains configuration for the policy evaluation cache
type CacheConfig struct {
	// Type defines the type of cache to use
	Type CacheType

	// TTL for cached entries (0 means no expiration)
	TTL time.Duration

	// MaxEntries limits the number of entries in the cache (0 means no limit)
	MaxEntries int

	// CacheKeyFields specifies which fields in the input should be used for cache key generation
	// If empty, all fields are used
	CacheKeyFields []string

	// ExternalCache is an external cache implementation (used when Type is CacheTypeExternal)
	ExternalCache types.Cache
	// ExternalCache interface {
	// 	// Get retrieves a cached decision for a key
	// 	// Get(key string) (interface{}, bool)
	// 	Get(key string) (types.Decision, bool)
	// 	// Set stores a decision for a key with optional TTL
	// 	Set(key string, decision interface{}, ttl time.Duration)
	// 	// Delete removes an entry from the cache
	// 	Delete(key string)
	// 	// Clear empties the cache
	// 	Clear()
	// }
}

// CacheType defines the type of cache to use
type CacheType int

const (
	// CacheTypeNone disables caching
	CacheTypeNone CacheType = iota
	// CacheTypeMemory uses an in-memory cache
	CacheTypeMemory
	// CacheTypeExternal uses an external cache implementation
	CacheTypeExternal
)
