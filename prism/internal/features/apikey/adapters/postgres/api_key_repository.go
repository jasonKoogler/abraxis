package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/prism/internal/common/db"
	"github.com/jasonKoogler/abraxis/prism/internal/domain"
	"github.com/jasonKoogler/abraxis/prism/internal/ports"
)

// Recommended indexes for api_keys table:
// CREATE UNIQUE INDEX idx_api_keys_id ON api_keys(id);
// CREATE UNIQUE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);
// CREATE INDEX idx_api_keys_tenant_id ON api_keys(tenant_id);
// CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
// CREATE INDEX idx_api_keys_expires_at ON api_keys(expires_at);

var (
	// ErrAPIKeyNotFound is returned when an API key is not found
	ErrAPIKeyNotFound = errors.New("API key not found")

	// ErrAPIKeyInactive is returned when an API key is inactive
	ErrAPIKeyInactive = errors.New("API key is inactive")

	// ErrAPIKeyExpired is returned when an API key has expired
	ErrAPIKeyExpired = errors.New("API key has expired")
)

// APIKeyRepository implements the ApiKeyRepository interface for PostgreSQL
type APIKeyRepository struct {
	db *db.PostgresPool
}

// Ensure APIKeyRepository implements ApiKeyRepository interface
var _ ports.ApiKeyRepository = &APIKeyRepository{}

// NewAPIKeyRepository creates a new API key repository
func NewAPIKeyRepository(db *db.PostgresPool) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

// Create creates a new API key and returns the created entity
func (ar *APIKeyRepository) Create(ctx context.Context, apiKey *domain.APIKey) (*domain.APIKey, error) {
	query := `
		INSERT INTO api_keys (
			id, name, key_prefix, key_hash, tenant_id, user_id, scopes, 
			expires_at, created_ip_address, is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		) RETURNING 
			id, name, key_prefix, key_hash, tenant_id, user_id, scopes,
			expires_at, last_used_at, created_ip_address, last_used_ip_address,
			is_active, created_at, updated_at
	`

	// Generate UUID if not provided
	if apiKey.ID == uuid.Nil {
		apiKey.ID = uuid.New()
	}

	// Set timestamps
	now := time.Now().UTC()
	apiKey.CreatedAt = now
	apiKey.UpdatedAt = now

	// Clear RawAPIKey for security before storage
	// This ensures the raw key is never stored in the database
	apiKey.RawAPIKey = ""

	// Execute query and scan result
	var result domain.APIKey
	var scopes []string
	var expiresAt, lastUsedAt *time.Time
	var createdIP, lastUsedIP net.IP

	err := ar.db.QueryRow(ctx, query,
		apiKey.ID, apiKey.Name, apiKey.KeyPrefix, apiKey.KeyHash,
		apiKey.TenantID, apiKey.UserID, apiKey.Scopes, apiKey.ExpiresAt,
		apiKey.CreatedIPAddress, apiKey.IsActive, apiKey.CreatedAt, apiKey.UpdatedAt,
	).Scan(
		&result.ID, &result.Name, &result.KeyPrefix, &result.KeyHash,
		&result.TenantID, &result.UserID, &scopes, &expiresAt,
		&lastUsedAt, &createdIP, &lastUsedIP,
		&result.IsActive, &result.CreatedAt, &result.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	// Process nullable fields
	result.Scopes = scopes
	if expiresAt != nil {
		result.ExpiresAt = *expiresAt
	}
	if lastUsedAt != nil {
		result.LastUsedAt = *lastUsedAt
	}
	result.CreatedIPAddress = createdIP
	result.LastUsedIPAddress = lastUsedIP

	return &result, nil
}

// GetByID retrieves an API key by ID
func (ar *APIKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	query := `
		SELECT 
			id, name, key_prefix, key_hash, tenant_id, user_id, scopes, 
			expires_at, last_used_at, created_ip_address, last_used_ip_address, 
			is_active, created_at, updated_at
		FROM api_keys
		WHERE id = $1
	`

	apiKey, err := ar.scanSingleAPIKey(ctx, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to get API key by ID %s: %w", id, err)
	}

	return apiKey, nil
}

// GetByPrefix retrieves an API key by prefix
func (ar *APIKeyRepository) GetByPrefix(ctx context.Context, prefix string) (*domain.APIKey, error) {
	query := `
		SELECT 
			id, name, key_prefix, key_hash, tenant_id, user_id, scopes, 
			expires_at, last_used_at, created_ip_address, last_used_ip_address, 
			is_active, created_at, updated_at
		FROM api_keys
		WHERE key_prefix = $1
	`

	apiKey, err := ar.scanSingleAPIKey(ctx, query, prefix)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to get API key by prefix %s: %w", prefix, err)
	}

	return apiKey, nil
}

// GetActiveByPrefix retrieves an active and non-expired API key by prefix
// This combines database lookup with domain validation in a single efficient query
func (ar *APIKeyRepository) GetActiveByPrefix(ctx context.Context, prefix string) (*domain.APIKey, error) {
	query := `
		SELECT 
			id, name, key_prefix, key_hash, tenant_id, user_id, scopes, 
			expires_at, last_used_at, created_ip_address, last_used_ip_address, 
			is_active, created_at, updated_at
		FROM api_keys
		WHERE key_prefix = $1
		  AND is_active = true
		  AND (expires_at IS NULL OR expires_at > $2)
	`

	apiKey, err := ar.scanSingleAPIKey(ctx, query, prefix, time.Now().UTC())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to get active API key by prefix %s: %w", prefix, err)
	}

	return apiKey, nil
}

// Update updates an API key's metadata
func (ar *APIKeyRepository) Update(ctx context.Context, id uuid.UUID, apiKey *domain.APIKey) (*domain.APIKey, error) {
	query := `
		UPDATE api_keys
		SET name = $2, 
		    scopes = $3, 
		    expires_at = $4, 
		    is_active = $5, 
		    updated_at = $6
		WHERE id = $1
		RETURNING 
			id, name, key_prefix, key_hash, tenant_id, user_id, scopes,
			expires_at, last_used_at, created_ip_address, last_used_ip_address,
			is_active, created_at, updated_at
	`

	apiKey.UpdatedAt = time.Now().UTC()

	updatedApiKey, err := ar.scanSingleAPIKey(ctx, query,
		id, apiKey.Name, apiKey.Scopes, apiKey.ExpiresAt,
		apiKey.IsActive, apiKey.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to update API key: %w", err)
	}

	return updatedApiKey, nil
}

// Delete deletes an API key
func (ar *APIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM api_keys WHERE id = $1`

	result, err := ar.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrAPIKeyNotFound
	}

	return nil
}

// ListByTenant lists all API keys for a tenant
func (ar *APIKeyRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*domain.APIKey, error) {
	query := `
		SELECT 
			id, name, key_prefix, key_hash, tenant_id, user_id, scopes, 
			expires_at, last_used_at, created_ip_address, last_used_ip_address, 
			is_active, created_at, updated_at
		FROM api_keys
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	offset := (page - 1) * pageSize // Adjust page to be 1-indexed
	return ar.scanMultipleAPIKeys(ctx, query, tenantID, pageSize, offset)
}

// ListByUser lists all API keys for a user
func (ar *APIKeyRepository) ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.APIKey, error) {
	query := `
		SELECT 
			id, name, key_prefix, key_hash, tenant_id, user_id, scopes, 
			expires_at, last_used_at, created_ip_address, last_used_ip_address, 
			is_active, created_at, updated_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	offset := (page - 1) * pageSize // Adjust page to be 1-indexed
	return ar.scanMultipleAPIKeys(ctx, query, userID, pageSize, offset)
}

// UpdateLastUsed updates the last used timestamp and IP address
func (ar *APIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID, ipAddress string) error {
	query := `
		UPDATE api_keys
		SET last_used_at = $2, 
		    last_used_ip_address = $3, 
		    updated_at = $4
		WHERE id = $1
	`

	now := time.Now().UTC()

	result, err := ar.db.Exec(ctx, query, id, now, net.ParseIP(ipAddress), now)
	if err != nil {
		return fmt.Errorf("failed to update last used for API key %s: %w", id, err)
	}

	if result.RowsAffected() == 0 {
		return ErrAPIKeyNotFound
	}

	return nil
}

// scanSingleAPIKey is a helper method to scan a single API key from a query result
func (ar *APIKeyRepository) scanSingleAPIKey(ctx context.Context, query string, args ...interface{}) (*domain.APIKey, error) {
	var apiKey domain.APIKey
	var scopes []string
	var expiresAt, lastUsedAt *time.Time
	var createdIP, lastUsedIP net.IP

	err := ar.db.QueryRow(ctx, query, args...).Scan(
		&apiKey.ID, &apiKey.Name, &apiKey.KeyPrefix, &apiKey.KeyHash,
		&apiKey.TenantID, &apiKey.UserID, &scopes, &expiresAt,
		&lastUsedAt, &createdIP, &lastUsedIP,
		&apiKey.IsActive, &apiKey.CreatedAt, &apiKey.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	apiKey.Scopes = scopes
	if expiresAt != nil {
		apiKey.ExpiresAt = *expiresAt
	}
	if lastUsedAt != nil {
		apiKey.LastUsedAt = *lastUsedAt
	}
	apiKey.CreatedIPAddress = createdIP
	apiKey.LastUsedIPAddress = lastUsedIP

	return &apiKey, nil
}

// scanMultipleAPIKeys is a helper method to scan multiple API keys from a query result
func (ar *APIKeyRepository) scanMultipleAPIKeys(ctx context.Context, query string, args ...interface{}) ([]*domain.APIKey, error) {
	rows, err := ar.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	apiKeys := []*domain.APIKey{}
	for rows.Next() {
		var apiKey domain.APIKey
		var scopes []string
		var expiresAt, lastUsedAt *time.Time
		var createdIP, lastUsedIP net.IP

		if err := rows.Scan(
			&apiKey.ID, &apiKey.Name, &apiKey.KeyPrefix, &apiKey.KeyHash,
			&apiKey.TenantID, &apiKey.UserID, &scopes, &expiresAt,
			&lastUsedAt, &createdIP, &lastUsedIP,
			&apiKey.IsActive, &apiKey.CreatedAt, &apiKey.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan API key data: %w", err)
		}

		apiKey.Scopes = scopes
		if expiresAt != nil {
			apiKey.ExpiresAt = *expiresAt
		}
		if lastUsedAt != nil {
			apiKey.LastUsedAt = *lastUsedAt
		}
		apiKey.CreatedIPAddress = createdIP
		apiKey.LastUsedIPAddress = lastUsedIP

		apiKeys = append(apiKeys, &apiKey)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API key rows: %w", err)
	}

	return apiKeys, nil
}
