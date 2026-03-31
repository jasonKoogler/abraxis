package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
)

// RoleRepository defines the interface for role operations
type RoleRepository interface {
	// Create creates a new role
	Create(ctx context.Context, role *domain.Role) (*domain.Role, error)

	// GetByID retrieves a role by ID
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Role, error)

	// GetByName retrieves a role by name
	GetByName(ctx context.Context, name string) (*domain.Role, error)

	// Update updates a role
	Update(ctx context.Context, id uuid.UUID, role *domain.Role) (*domain.Role, error)

	// Delete deletes a role
	Delete(ctx context.Context, id uuid.UUID) error

	// ListAll lists all roles with pagination
	ListAll(ctx context.Context, page, pageSize int) ([]*domain.Role, error)

	// ListByTenant lists all roles for a tenant
	ListByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*domain.Role, error)

	// ListSystemRoles lists all system roles
	ListSystemRoles(ctx context.Context) ([]*domain.Role, error)
}

// PermissionRepository defines the interface for permission operations
type PermissionRepository interface {
	// Create creates a new permission
	Create(ctx context.Context, permission *domain.Permission) (*domain.Permission, error)

	// GetByID retrieves a permission by ID
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error)

	// GetByName retrieves a permission by name
	GetByName(ctx context.Context, name string) (*domain.Permission, error)

	// GetByActionAndResource retrieves a permission by action and resource
	GetByActionAndResource(ctx context.Context, action, resource string) (*domain.Permission, error)

	// Update updates a permission
	Update(ctx context.Context, id uuid.UUID, permission *domain.Permission) (*domain.Permission, error)

	// Delete deletes a permission
	Delete(ctx context.Context, id uuid.UUID) error

	// ListAll lists all permissions with pagination
	ListAll(ctx context.Context, page, pageSize int) ([]*domain.Permission, error)

	// ListByAction lists all permissions for an action
	ListByAction(ctx context.Context, action string) ([]*domain.Permission, error)

	// ListByResource lists all permissions for a resource
	ListByResource(ctx context.Context, resource string) ([]*domain.Permission, error)

	// ListSystemPermissions lists all system permissions
	ListSystemPermissions(ctx context.Context) ([]*domain.Permission, error)
}

// RolePermissionRepository defines the interface for role-permission mapping operations
type RolePermissionRepository interface {
	// AddPermissionToRole adds a permission to a role
	AddPermissionToRole(ctx context.Context, roleID, permissionID uuid.UUID) error

	// RemovePermissionFromRole removes a permission from a role
	RemovePermissionFromRole(ctx context.Context, roleID, permissionID uuid.UUID) error

	// GetRolePermissions gets all permissions for a role
	GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]*domain.Permission, error)

	// GetPermissionRoles gets all roles that have a permission
	GetPermissionRoles(ctx context.Context, permissionID uuid.UUID) ([]*domain.Role, error)
}
