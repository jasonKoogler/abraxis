package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/prism/internal/domain"
)

// APIRouteRepository defines the interface for API route operations
type APIRouteRepository interface {
	// Create creates a new API route
	Create(ctx context.Context, route *domain.APIRoute) (*domain.APIRoute, error)

	// GetByID retrieves an API route by ID
	GetByID(ctx context.Context, id uuid.UUID) (*domain.APIRoute, error)

	// GetByPathAndMethod retrieves an API route by path pattern and HTTP method
	GetByPathAndMethod(ctx context.Context, pathPattern, httpMethod string, tenantID *uuid.UUID) (*domain.APIRoute, error)

	// Update updates an API route
	Update(ctx context.Context, id uuid.UUID, route *domain.APIRoute) (*domain.APIRoute, error)

	// Delete deletes an API route
	Delete(ctx context.Context, id uuid.UUID) error

	// ListByTenant lists all API routes for a tenant
	ListByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*domain.APIRoute, error)

	// ListByBackendService lists all API routes for a backend service
	ListByBackendService(ctx context.Context, backendService string, page, pageSize int) ([]*domain.APIRoute, error)
}
