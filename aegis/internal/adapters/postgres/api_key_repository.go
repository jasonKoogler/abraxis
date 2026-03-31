package postgres

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/aegis/internal/common/db"
	"github.com/jasonKoogler/aegis/internal/domain"
	"github.com/jasonKoogler/aegis/internal/ports"
)

// APIKeyRepository implements the APIKeyRepository interface
type APIKeyRepository struct {
	db *db.PostgresPool
}

var _ ports.APIKeyRepository = &APIKeyRepository{}

// NewAPIKeyRepository creates a new API key repository
func NewAPIKeyRepository(db *db.PostgresPool) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

// Create creates a new API key
func (ar *APIKeyRepository) Create(ctx context.Context, apiKey *domain.APIKey) (*domain.APIKey, error) {
	query := `
		INSERT INTO api_keys (
			id, name, key_prefix, key_hash, tenant_id, user_id, scopes, 
			expires_at, created_ip_address, is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		) RETURNING id
	`

	if apiKey.ID == uuid.Nil {
		apiKey.ID = uuid.New()
	}

	now := time.Now()
	apiKey.CreatedAt = now
	apiKey.UpdatedAt = now

	var createdIPStr *string
	if apiKey.CreatedIPAddress != nil {
		ipStr := apiKey.CreatedIPAddress.String()
		createdIPStr = &ipStr
	}

	_, err := ar.db.Exec(ctx, query,
		apiKey.ID, apiKey.Name, apiKey.KeyPrefix, apiKey.KeyHash,
		apiKey.TenantID, apiKey.UserID, apiKey.Scopes, apiKey.ExpiresAt,
		createdIPStr, apiKey.IsActive, apiKey.CreatedAt, apiKey.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return apiKey, nil
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

	var apiKey domain.APIKey
	var createdIPStr, lastUsedIPStr *string
	var scopes []string

	err := ar.db.QueryRow(ctx, query, id).Scan(
		&apiKey.ID, &apiKey.Name, &apiKey.KeyPrefix, &apiKey.KeyHash,
		&apiKey.TenantID, &apiKey.UserID, &scopes, &apiKey.ExpiresAt,
		&apiKey.LastUsedAt, &createdIPStr, &lastUsedIPStr,
		&apiKey.IsActive, &apiKey.CreatedAt, &apiKey.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key by ID %s: %w", id, err)
	}

	apiKey.Scopes = scopes

	if createdIPStr != nil {
		apiKey.CreatedIPAddress = net.ParseIP(*createdIPStr)
	}

	if lastUsedIPStr != nil {
		apiKey.LastUsedIPAddress = net.ParseIP(*lastUsedIPStr)
	}

	return &apiKey, nil
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

	var apiKey domain.APIKey
	var createdIPStr, lastUsedIPStr *string
	var scopes []string

	err := ar.db.QueryRow(ctx, query, prefix).Scan(
		&apiKey.ID, &apiKey.Name, &apiKey.KeyPrefix, &apiKey.KeyHash,
		&apiKey.TenantID, &apiKey.UserID, &scopes, &apiKey.ExpiresAt,
		&apiKey.LastUsedAt, &createdIPStr, &lastUsedIPStr,
		&apiKey.IsActive, &apiKey.CreatedAt, &apiKey.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key by prefix %s: %w", prefix, err)
	}

	apiKey.Scopes = scopes

	if createdIPStr != nil {
		apiKey.CreatedIPAddress = net.ParseIP(*createdIPStr)
	}

	if lastUsedIPStr != nil {
		apiKey.LastUsedIPAddress = net.ParseIP(*lastUsedIPStr)
	}

	return &apiKey, nil
}

// Update updates an API key
func (ar *APIKeyRepository) Update(ctx context.Context, id uuid.UUID, apiKey *domain.APIKey) (*domain.APIKey, error) {
	query := `
		UPDATE api_keys
		SET name = $2, scopes = $3, expires_at = $4, is_active = $5, updated_at = $6
		WHERE id = $1
	`

	apiKey.UpdatedAt = time.Now()

	_, err := ar.db.Exec(ctx, query,
		id, apiKey.Name, apiKey.Scopes, apiKey.ExpiresAt,
		apiKey.IsActive, apiKey.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update API key: %w", err)
	}

	return apiKey, nil
}

// Delete deletes an API key
func (ar *APIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM api_keys WHERE id = $1`

	result, err := ar.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("API key not found")
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

	rows, err := ar.db.Query(ctx, query, tenantID, pageSize, page*pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	apiKeys := []*domain.APIKey{}
	for rows.Next() {
		var apiKey domain.APIKey
		var createdIPStr, lastUsedIPStr *string
		var scopes []string

		if err := rows.Scan(
			&apiKey.ID, &apiKey.Name, &apiKey.KeyPrefix, &apiKey.KeyHash,
			&apiKey.TenantID, &apiKey.UserID, &scopes, &apiKey.ExpiresAt,
			&apiKey.LastUsedAt, &createdIPStr, &lastUsedIPStr,
			&apiKey.IsActive, &apiKey.CreatedAt, &apiKey.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan API key data: %w", err)
		}

		apiKey.Scopes = scopes

		if createdIPStr != nil {
			apiKey.CreatedIPAddress = net.ParseIP(*createdIPStr)
		}

		if lastUsedIPStr != nil {
			apiKey.LastUsedIPAddress = net.ParseIP(*lastUsedIPStr)
		}

		apiKeys = append(apiKeys, &apiKey)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API key rows: %w", err)
	}

	return apiKeys, nil
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

	rows, err := ar.db.Query(ctx, query, userID, pageSize, page*pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys for user %s: %w", userID, err)
	}
	defer rows.Close()

	apiKeys := []*domain.APIKey{}
	for rows.Next() {
		var apiKey domain.APIKey
		var createdIPStr, lastUsedIPStr *string
		var scopes []string

		if err := rows.Scan(
			&apiKey.ID, &apiKey.Name, &apiKey.KeyPrefix, &apiKey.KeyHash,
			&apiKey.TenantID, &apiKey.UserID, &scopes, &apiKey.ExpiresAt,
			&apiKey.LastUsedAt, &createdIPStr, &lastUsedIPStr,
			&apiKey.IsActive, &apiKey.CreatedAt, &apiKey.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan API key data: %w", err)
		}

		apiKey.Scopes = scopes

		if createdIPStr != nil {
			apiKey.CreatedIPAddress = net.ParseIP(*createdIPStr)
		}

		if lastUsedIPStr != nil {
			apiKey.LastUsedIPAddress = net.ParseIP(*lastUsedIPStr)
		}

		apiKeys = append(apiKeys, &apiKey)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API key rows: %w", err)
	}

	return apiKeys, nil
}

// UpdateLastUsed updates the last used timestamp and IP address
func (ar *APIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID, ipAddress string) error {
	query := `
		UPDATE api_keys
		SET last_used_at = $2, last_used_ip_address = $3, updated_at = $4
		WHERE id = $1
	`

	now := time.Now()

	_, err := ar.db.Exec(ctx, query, id, now, ipAddress, now)
	if err != nil {
		return fmt.Errorf("failed to update last used for API key %s: %w", id, err)
	}

	return nil
}
