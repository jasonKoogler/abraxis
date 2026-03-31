package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
)

type UserRepository interface {
	// Create creates a new user.
	Create(ctx context.Context, user *domain.User) (*domain.User, error)
	// GetByID returns a user by id. And also joins the user with users roles and tenants.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	// GetByEmail returns a user by email. And also joins the user with users roles and tenants.
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByOrganizationID(ctx context.Context, organizationID uuid.UUID, page, pageSize int) ([]*domain.User, error)
	ListAll(ctx context.Context, page, pageSize int) ([]*domain.User, error)
	Update(ctx context.Context, id uuid.UUID, user *domain.User) (*domain.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SetLastLoginDate(ctx context.Context, id uuid.UUID) error

	// GetRoles returns the roles for a user.
	GetRoles(ctx context.Context, id uuid.UUID) (*domain.RoleMap, error)
	// AddRoleToTenant adds a role to a tenant.
	AddRoleToTenant(ctx context.Context, id uuid.UUID, tenantID uuid.UUID, role string) error
	// RemoveRoleFromTenant removes a role from a tenant.
	RemoveRoleFromTenant(ctx context.Context, id uuid.UUID, tenantID uuid.UUID, role string) error

	GetUserByProviderUserID(ctx context.Context, provider string, providerUserID string) (*domain.User, error)
}
