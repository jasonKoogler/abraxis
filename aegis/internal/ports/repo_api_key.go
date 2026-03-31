package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/jasonKoogler/aegis/internal/domain"
)

// APIKeyRepository defines the interface for API key operations
type APIKeyRepository interface {
	// Create creates a new API key
	Create(ctx context.Context, apiKey *domain.APIKey) (*domain.APIKey, error)

	// GetByID retrieves an API key by ID
	GetByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error)

	// GetByPrefix retrieves an API key by prefix
	GetByPrefix(ctx context.Context, prefix string) (*domain.APIKey, error)

	// Update updates an API key
	Update(ctx context.Context, id uuid.UUID, apiKey *domain.APIKey) (*domain.APIKey, error)

	// Delete deletes an API key
	Delete(ctx context.Context, id uuid.UUID) error

	// ListByTenant lists all API keys for a tenant
	ListByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*domain.APIKey, error)

	// ListByUser lists all API keys for a user
	ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.APIKey, error)

	// UpdateLastUsed updates the last used timestamp and IP address
	UpdateLastUsed(ctx context.Context, id uuid.UUID, ipAddress string) error
}
