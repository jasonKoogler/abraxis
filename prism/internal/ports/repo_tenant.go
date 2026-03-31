package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/prism/internal/domain"
)

// TenantRepository defines the interface for tenant operations
type TenantRepository interface {
	// Create creates a new tenant
	Create(ctx context.Context, tenant *domain.Tenant) (*domain.Tenant, error)

	// GetByID retrieves a tenant by ID
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)

	// GetByDomain retrieves a tenant by domain
	GetByDomain(ctx context.Context, domain string) (*domain.Tenant, error)

	// Update updates a tenant
	Update(ctx context.Context, id uuid.UUID, tenant *domain.Tenant) (*domain.Tenant, error)

	// Delete deletes a tenant
	Delete(ctx context.Context, id uuid.UUID) error

	// ListAll lists all tenants with pagination
	ListAll(ctx context.Context, page, pageSize int) ([]*domain.Tenant, error)

	// AddUserToTenant adds a user to a tenant
	AddUserToTenant(ctx context.Context, tenantID, userID uuid.UUID, isAdmin bool) error

	// RemoveUserFromTenant removes a user from a tenant
	RemoveUserFromTenant(ctx context.Context, tenantID, userID uuid.UUID) error

	// GetUserTenants gets all tenants for a user
	GetUserTenants(ctx context.Context, userID uuid.UUID) ([]*domain.Tenant, error)

	// SetDefaultTenant sets a tenant as the default for a user
	SetDefaultTenant(ctx context.Context, userID, tenantID uuid.UUID) error
}
