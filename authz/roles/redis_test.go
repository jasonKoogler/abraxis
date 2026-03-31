package roles

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
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

func TestNewRedisRoleProvider(t *testing.T) {
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
			keyPrefix: "roles:",
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
			provider := NewRedisRoleProvider(tt.client, WithKeyPrefix(tt.keyPrefix))
			assert.NotNil(t, provider)
			assert.Equal(t, tt.client, provider.client)
			assert.Equal(t, tt.keyPrefix, provider.keyPrefix)
		})
	}
}

func TestRedisRoleProviderGetRoles(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	provider := NewRedisRoleProvider(client, WithKeyPrefix("roles:"))

	// Set up some test data
	ctx := context.Background()
	key := "roles:user1"
	roles := []string{"admin", "editor"}
	rolesJSON, err := json.Marshal(roles)
	require.NoError(t, err)
	err = client.Set(ctx, key, rolesJSON, 0).Err()
	require.NoError(t, err)

	// Test getting roles for a user with roles
	result, err := provider.GetRoles(ctx, "user1")
	require.NoError(t, err)
	assert.Equal(t, roles, result)

	// Test getting roles for a user without roles
	result, err = provider.GetRoles(ctx, "user2")
	require.NoError(t, err)
	assert.Empty(t, result)

	// Test with default roles
	providerWithDefaults := NewRedisRoleProvider(
		client,
		WithKeyPrefix("roles:"),
		WithDefaultRoles([]string{"user"}),
	)
	result, err = providerWithDefaults.GetRoles(ctx, "user2")
	require.NoError(t, err)
	assert.Equal(t, []string{"user"}, result)

	// Test with invalid JSON in Redis
	err = client.Set(ctx, "roles:invalid", "not-json", 0).Err()
	require.NoError(t, err)
	result, err = provider.GetRoles(ctx, "invalid")
	require.Error(t, err)
	assert.Empty(t, result)
}

func TestRedisRoleProviderSetRoles(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	provider := NewRedisRoleProvider(client, WithKeyPrefix("roles:"))

	// Test setting roles
	ctx := context.Background()
	userID := "user1"
	roles := []string{"admin", "editor"}

	// Set the roles
	err := provider.SetRoles(ctx, userID, roles)
	require.NoError(t, err)

	// Verify they were set correctly
	key := "roles:user1"
	result, err := client.Get(ctx, key).Result()
	require.NoError(t, err)

	var storedRoles []string
	err = json.Unmarshal([]byte(result), &storedRoles)
	require.NoError(t, err)
	assert.Equal(t, roles, storedRoles)

	// Test setting empty roles
	err = provider.SetRoles(ctx, "user2", []string{})
	require.NoError(t, err)

	// Verify they were set correctly
	key = "roles:user2"
	result, err = client.Get(ctx, key).Result()
	require.NoError(t, err)

	err = json.Unmarshal([]byte(result), &storedRoles)
	require.NoError(t, err)
	assert.Empty(t, storedRoles)

	// Test with TTL
	providerWithTTL := NewRedisRoleProvider(
		client,
		WithKeyPrefix("roles:"),
		WithDefaultTTL(time.Second),
	)
	err = providerWithTTL.SetRoles(ctx, "user3", roles)
	require.NoError(t, err)

	// Verify TTL was set
	key = "roles:user3"
	ttl, err := client.TTL(ctx, key).Result()
	require.NoError(t, err)
	assert.True(t, ttl > 0)
}

func TestRedisRoleProviderKeyPrefix(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	// Create two providers with different prefixes
	provider1 := NewRedisRoleProvider(client, WithKeyPrefix("prefix1:"))
	provider2 := NewRedisRoleProvider(client, WithKeyPrefix("prefix2:"))

	// Set roles for the same user ID in both providers
	ctx := context.Background()
	userID := "user1"
	roles1 := []string{"admin", "editor"}
	roles2 := []string{"viewer"}

	err := provider1.SetRoles(ctx, userID, roles1)
	require.NoError(t, err)

	err = provider2.SetRoles(ctx, userID, roles2)
	require.NoError(t, err)

	// Get roles from both providers
	result1, err := provider1.GetRoles(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, roles1, result1)

	result2, err := provider2.GetRoles(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, roles2, result2)

	// Delete roles from one provider
	err = provider1.DeleteRoles(ctx, userID)
	require.NoError(t, err)

	// Verify roles were deleted from provider1 but not provider2
	result1, err = provider1.GetRoles(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, result1)

	result2, err = provider2.GetRoles(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, roles2, result2)
}
