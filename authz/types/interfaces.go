package types

import (
	"context"
	"time"
)

// RoleProvider is an interface for fetching roles from an external source
type RoleProvider interface {
	// GetRoles retrieves roles for a user
	GetRoles(ctx context.Context, userID string) ([]string, error)
}

// Cache is an interface for caching policy evaluation results
type Cache interface {
	// Get retrieves a cached decision for a key
	Get(key string) (Decision, bool)
	// Set stores a decision for a key with optional TTL
	Set(key string, decision Decision, ttl time.Duration)
	// Delete removes an entry from the cache
	Delete(key string)
	// Clear empties the cache
	Clear()
}

type Logger interface {
	Printf(format string, v ...any)
	Println(v ...any)
}
