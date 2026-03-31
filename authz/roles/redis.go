package roles

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRoleProvider implements the agent.RoleProvider interface
// using Redis as a backend storage
type RedisRoleProvider struct {
	client       *redis.Client
	keyPrefix    string
	defaultTTL   time.Duration
	defaultRoles []string
}

// RedisRoleProviderOption is a functional option for configuring RedisRoleProvider
type RedisRoleProviderOption func(*RedisRoleProvider)

// WithKeyPrefix sets the prefix for Redis keys
func WithKeyPrefix(prefix string) RedisRoleProviderOption {
	return func(p *RedisRoleProvider) {
		p.keyPrefix = prefix
	}
}

// WithDefaultTTL sets the default TTL for cached roles
func WithDefaultTTL(ttl time.Duration) RedisRoleProviderOption {
	return func(p *RedisRoleProvider) {
		p.defaultTTL = ttl
	}
}

// WithDefaultRoles sets the default roles to return if none are found
func WithDefaultRoles(roles []string) RedisRoleProviderOption {
	return func(p *RedisRoleProvider) {
		p.defaultRoles = roles
	}
}

// NewRedisRoleProvider creates a new Redis-based role provider
func NewRedisRoleProvider(client *redis.Client, options ...RedisRoleProviderOption) *RedisRoleProvider {
	provider := &RedisRoleProvider{
		client:       client,
		keyPrefix:    "roles:",
		defaultTTL:   time.Hour * 24,
		defaultRoles: []string{},
	}

	// Apply options
	for _, option := range options {
		option(provider)
	}

	return provider
}

// GetRoles retrieves roles for a user from Redis
func (p *RedisRoleProvider) GetRoles(ctx context.Context, userID string) ([]string, error) {
	// Construct the key
	key := fmt.Sprintf("%s%s", p.keyPrefix, userID)

	// Get roles from Redis
	val, err := p.client.Get(ctx, key).Result()
	if err == redis.Nil {
		// Key does not exist, return default roles
		return p.defaultRoles, nil
	} else if err != nil {
		return nil, fmt.Errorf("error getting roles from Redis: %w", err)
	}

	// Parse roles from JSON
	var roles []string
	if err := json.Unmarshal([]byte(val), &roles); err != nil {
		return nil, fmt.Errorf("error parsing roles: %w", err)
	}

	return roles, nil
}

// SetRoles sets roles for a user in Redis
func (p *RedisRoleProvider) SetRoles(ctx context.Context, userID string, roles []string) error {
	// Construct the key
	key := fmt.Sprintf("%s%s", p.keyPrefix, userID)

	// Convert roles to JSON
	rolesJSON, err := json.Marshal(roles)
	if err != nil {
		return fmt.Errorf("error marshaling roles: %w", err)
	}

	// Set roles in Redis with TTL
	err = p.client.Set(ctx, key, rolesJSON, p.defaultTTL).Err()
	if err != nil {
		return fmt.Errorf("error setting roles in Redis: %w", err)
	}

	return nil
}

// DeleteRoles deletes roles for a user from Redis
func (p *RedisRoleProvider) DeleteRoles(ctx context.Context, userID string) error {
	// Construct the key
	key := fmt.Sprintf("%s%s", p.keyPrefix, userID)

	// Delete roles from Redis
	err := p.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("error deleting roles from Redis: %w", err)
	}

	return nil
}
