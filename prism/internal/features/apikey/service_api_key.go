package apikey

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/prism/internal/common/log"
	commonuuid "github.com/jasonKoogler/abraxis/prism/internal/common/uuid"
	"github.com/jasonKoogler/abraxis/prism/internal/domain"
	"github.com/jasonKoogler/abraxis/prism/internal/domain/prefixid"
	"github.com/jasonKoogler/abraxis/prism/internal/ports"
)

// APIKeyService handles API key management operations
type APIKeyService struct {
	apiKeyRepo ports.ApiKeyRepository
	logger     *log.Logger
}

var _ ports.ApiKeyService = &APIKeyService{}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(apiKeyRepo ports.ApiKeyRepository, logger *log.Logger) *APIKeyService {
	return &APIKeyService{
		apiKeyRepo: apiKeyRepo,
		logger:     logger,
	}
}

// Create creates a new API key
func (s *APIKeyService) Create(ctx context.Context, params *domain.APIKey_CreateParams) (*domain.APIKey_CreateResponse, error) {
	apiKey, err := domain.APIKey_New(params)
	if err != nil {
		return nil, err
	}

	// Preserve the raw key before the repository clears it for security.
	rawKey := apiKey.RawAPIKey

	// Save to repository
	createdKey, err := s.apiKeyRepo.Create(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	commonID, err := googleToCommonUUID(createdKey.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	apiKeyID, err := prefixid.ApiKeyIDFromUUID(commonID)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	s.logger.Info("API key created",
		log.String("key_prefix", apiKey.KeyPrefix),
		log.String("key_id", apiKeyID.String()),
		log.String("tenant_id", params.TenantID.String()),
		log.String("user_id", params.UserID.String()))

	return &domain.APIKey_CreateResponse{
		ID:        apiKeyID.String(),
		Name:      apiKey.Name,
		KeyPrefix: apiKey.KeyPrefix,
		RawAPIKey: rawKey,
		Scopes:    apiKey.Scopes,
		ExpiresAt: apiKey.ExpiresAt,
		CreatedAt: apiKey.CreatedAt,
	}, nil
}

// Validate validates an API key and returns the associated API key entity
func (s *APIKeyService) Validate(ctx context.Context, rawKey, ipAddress string) (*domain.APIKey, error) {
	// Extract prefix (using domain helper)
	keyPrefix, err := domain.ExtractKeyPrefixForLookup(rawKey)
	if err != nil {
		return nil, err
	}

	// Look up the key by prefix and validate it's active and not expired in a single query
	apiKey, err := s.apiKeyRepo.GetActiveByPrefix(ctx, keyPrefix)
	if err != nil {
		// Use a standardized error message for security
		return nil, errors.New("invalid or expired API key")
	}

	// Verify the key hash
	if !domain.APIKey_Verify(rawKey, apiKey.KeyHash) {
		return nil, errors.New("invalid API key")
	}

	// Update last used information (if IP address provided)
	if ipAddress != "" {
		err = s.apiKeyRepo.UpdateLastUsed(ctx, apiKey.ID, ipAddress)
		if err != nil {
			s.logger.Warn("Failed to update API key last used information",
				log.String("key_prefix", keyPrefix),
				log.Error(err))
			// Continue anyway, this is not critical
		}
	}

	return apiKey, nil
}

// Get retrieves an API key by ID
func (s *APIKeyService) Get(ctx context.Context, id string) (*domain.APIKey, error) {
	apiKeyID, err := prefixid.ParseApiKeyID(id)
	if err != nil {
		return nil, fmt.Errorf("invalid API key ID: %w", err)
	}
	googleID, err := uuid.Parse(apiKeyID.Raw().String())
	if err != nil {
		return nil, fmt.Errorf("invalid API key ID: %w", err)
	}
	return s.apiKeyRepo.GetByID(ctx, googleID)
}

// List lists API keys based on the provided parameters
func (s *APIKeyService) List(ctx context.Context, params *domain.APIKey_ListParams) ([]*domain.APIKey, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	// Determine which repository method to call based on the params
	if params.TenantID != nil {
		return s.apiKeyRepo.ListByTenant(ctx, *params.TenantID, params.Page, params.PageSize)
	} else if params.UserID != nil {
		return s.apiKeyRepo.ListByUser(ctx, *params.UserID, params.Page, params.PageSize)
	}

	// This should never happen due to validation, but just in case
	return nil, fmt.Errorf("must provide either tenant_id or user_id")
}

// UpdateMetadata updates an API key's metadata
func (s *APIKeyService) UpdateMetadata(ctx context.Context, params *domain.APIKey_UpdateMetadataParams) (*domain.APIKey, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	// Get the existing API key
	apiKey, err := s.apiKeyRepo.GetByID(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	// Update fields
	if err := apiKey.UpdateMetadata(params); err != nil {
		return nil, err
	}

	// Save changes
	updatedKey, err := s.apiKeyRepo.Update(ctx, params.ID, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to update API key: %w", err)
	}

	s.logger.Info("API key updated",
		log.String("key_id", params.ID.String()),
		log.String("key_prefix", apiKey.KeyPrefix))

	return updatedKey, nil
}

// Revoke revokes (deletes) an API key
func (s *APIKeyService) Revoke(ctx context.Context, id string) error {
	apiKeyID, err := prefixid.ParseApiKeyID(id)
	if err != nil {
		return fmt.Errorf("invalid API key ID: %w", err)
	}

	googleID, err := uuid.Parse(apiKeyID.Raw().String())
	if err != nil {
		return fmt.Errorf("invalid API key ID: %w", err)
	}

	// Get the key first for logging
	apiKey, err := s.apiKeyRepo.GetByID(ctx, googleID)
	if err != nil {
		return fmt.Errorf("failed to get API key: %w", err)
	}

	// Delete the key
	err = s.apiKeyRepo.Delete(ctx, googleID)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	s.logger.Info("API key revoked",
		log.String("key_id", apiKeyID.String()),
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

// googleToCommonUUID converts a google/uuid.UUID to common/uuid.UUID via string parsing
func googleToCommonUUID(id uuid.UUID) (commonuuid.UUID, error) {
	return commonuuid.Parse(id.String())
}
