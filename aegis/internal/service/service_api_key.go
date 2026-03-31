package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
	"github.com/jasonKoogler/abraxis/aegis/internal/ports"
)

const (
	// APIKeyPrefix is the prefix for all API keys
	APIKeyPrefix = "ak_"

	// APIKeyLength is the length of the API key in bytes (before encoding)
	APIKeyLength = 32

	// APIKeyPrefixDisplayLength is the number of characters of the key to use as a prefix for display/lookup
	APIKeyPrefixDisplayLength = 8

	// DefaultAPIKeyExpirationDays is the default number of days until an API key expires
	DefaultAPIKeyExpirationDays = 90
)

// APIKeyService handles API key management operations
type APIKeyService struct {
	apiKeyRepo ports.APIKeyRepository
	logger     *log.Logger
}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(apiKeyRepo ports.APIKeyRepository, logger *log.Logger) *APIKeyService {
	return &APIKeyService{
		apiKeyRepo: apiKeyRepo,
		logger:     logger,
	}
}

// CreateAPIKeyResult contains the result of creating a new API key
type CreateAPIKeyResult struct {
	APIKey    *domain.APIKey `json:"api_key"`
	RawAPIKey string         `json:"raw_api_key"`
}

// CreateAPIKey creates a new API key
func (s *APIKeyService) CreateAPIKey(ctx context.Context, name string, tenantID, userID uuid.UUID, scopes []string, ipAddress string, expiresInDays int) (*CreateAPIKeyResult, error) {
	// Generate a random API key
	rawKey, keyPrefix, keyHash, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Set expiration date
	var expiresAt time.Time
	if expiresInDays > 0 {
		expiresAt = time.Now().AddDate(0, 0, expiresInDays)
	} else {
		expiresAt = time.Now().AddDate(0, 0, DefaultAPIKeyExpirationDays)
	}

	// Create the API key entity
	apiKey := &domain.APIKey{
		Name:             name,
		KeyPrefix:        keyPrefix,
		KeyHash:          keyHash,
		TenantID:         tenantID,
		UserID:           userID,
		Scopes:           scopes,
		ExpiresAt:        expiresAt,
		CreatedIPAddress: net.ParseIP(ipAddress),
		IsActive:         true,
	}

	// Save to repository
	createdKey, err := s.apiKeyRepo.Create(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	s.logger.Info("API key created",
		log.String("key_prefix", keyPrefix),
		log.String("tenant_id", tenantID.String()),
		log.String("user_id", userID.String()))

	return &CreateAPIKeyResult{
		APIKey:    createdKey,
		RawAPIKey: rawKey,
	}, nil
}

// ValidateAPIKey validates an API key and returns the associated API key entity
func (s *APIKeyService) ValidateAPIKey(ctx context.Context, rawKey, ipAddress string) (*domain.APIKey, error) {
	// Extract the prefix from the raw key
	if !strings.HasPrefix(rawKey, APIKeyPrefix) {
		return nil, fmt.Errorf("invalid API key format")
	}

	// Get the key prefix for lookup
	keyParts := strings.Split(rawKey, "_")
	if len(keyParts) != 2 {
		return nil, fmt.Errorf("invalid API key format")
	}

	// Get the display prefix (first few characters after the prefix)
	keyValue := keyParts[1]
	if len(keyValue) <= APIKeyPrefixDisplayLength {
		return nil, fmt.Errorf("invalid API key format")
	}

	keyPrefix := APIKeyPrefix + keyValue[:APIKeyPrefixDisplayLength]

	// Look up the key by prefix
	apiKey, err := s.apiKeyRepo.GetByPrefix(ctx, keyPrefix)
	if err != nil {
		return nil, fmt.Errorf("API key not found")
	}

	// Check if the key is active
	if !apiKey.IsActive {
		return nil, fmt.Errorf("API key is inactive")
	}

	// Check if the key has expired
	if !apiKey.ExpiresAt.IsZero() && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("API key has expired")
	}

	// Verify the key hash
	if !verifyAPIKey(rawKey, apiKey.KeyHash) {
		return nil, fmt.Errorf("invalid API key")
	}

	// Update last used information
	err = s.apiKeyRepo.UpdateLastUsed(ctx, apiKey.ID, ipAddress)
	if err != nil {
		s.logger.Warn("Failed to update API key last used information",
			log.String("key_prefix", keyPrefix),
			log.Error(err))
		// Continue anyway, this is not critical
	}

	return apiKey, nil
}

// GetAPIKey retrieves an API key by ID
func (s *APIKeyService) GetAPIKey(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	return s.apiKeyRepo.GetByID(ctx, id)
}

// ListAPIKeysByTenant lists all API keys for a tenant
func (s *APIKeyService) ListAPIKeysByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*domain.APIKey, error) {
	return s.apiKeyRepo.ListByTenant(ctx, tenantID, page, pageSize)
}

// ListAPIKeysByUser lists all API keys for a user
func (s *APIKeyService) ListAPIKeysByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.APIKey, error) {
	return s.apiKeyRepo.ListByUser(ctx, userID, page, pageSize)
}

// UpdateAPIKey updates an API key
func (s *APIKeyService) UpdateAPIKey(ctx context.Context, id uuid.UUID, name string, scopes []string, isActive bool, expiresInDays int) (*domain.APIKey, error) {
	// Get the existing API key
	apiKey, err := s.apiKeyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	// Update fields
	apiKey.Name = name
	apiKey.Scopes = scopes
	apiKey.IsActive = isActive

	// Update expiration if specified
	if expiresInDays > 0 {
		apiKey.ExpiresAt = time.Now().AddDate(0, 0, expiresInDays)
	}

	// Save changes
	updatedKey, err := s.apiKeyRepo.Update(ctx, id, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to update API key: %w", err)
	}

	s.logger.Info("API key updated",
		log.String("key_id", id.String()),
		log.String("key_prefix", apiKey.KeyPrefix))

	return updatedKey, nil
}

// RevokeAPIKey revokes (deletes) an API key
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, id uuid.UUID) error {
	// Get the key first for logging
	apiKey, err := s.apiKeyRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get API key: %w", err)
	}

	// Delete the key
	err = s.apiKeyRepo.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	s.logger.Info("API key revoked",
		log.String("key_id", id.String()),
		log.String("key_prefix", apiKey.KeyPrefix))

	return nil
}

// HasScope checks if an API key has a specific scope
func (s *APIKeyService) HasScope(apiKey *domain.APIKey, requiredScope string) bool {
	for _, scope := range apiKey.Scopes {
		// Check for exact match
		if scope == requiredScope {
			return true
		}

		// Check for wildcard match (e.g., "users:*" matches "users:read")
		if strings.HasSuffix(scope, ":*") {
			prefix := strings.TrimSuffix(scope, ":*")
			if strings.HasPrefix(requiredScope, prefix+":") {
				return true
			}
		}
	}
	return false
}

// Helper functions

// generateAPIKey generates a new random API key, its prefix for lookup, and its hash for storage
func generateAPIKey() (string, string, string, error) {
	// Generate random bytes
	keyBytes := make([]byte, APIKeyLength)
	_, err := rand.Read(keyBytes)
	if err != nil {
		return "", "", "", err
	}

	// Encode as base64
	keyStr := base64.URLEncoding.EncodeToString(keyBytes)

	// Format with prefix
	rawKey := fmt.Sprintf("%s%s", APIKeyPrefix, keyStr)

	// Get the prefix for lookup
	keyPrefix := APIKeyPrefix + keyStr[:APIKeyPrefixDisplayLength]

	// Hash the key for storage
	keyHash := hashAPIKey(rawKey)

	return rawKey, keyPrefix, keyHash, nil
}

// hashAPIKey creates a SHA-256 hash of an API key
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// verifyAPIKey verifies if a raw API key matches a stored hash
func verifyAPIKey(rawKey, storedHash string) bool {
	keyHash := hashAPIKey(rawKey)
	return keyHash == storedHash
}
