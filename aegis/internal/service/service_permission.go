package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/jasonKoogler/aegis/internal/domain"
	"github.com/jasonKoogler/aegis/internal/ports"
)

// Common permission actions
const (
	ActionCreate = "create"
	ActionRead   = "read"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionList   = "list"
	ActionManage = "manage" // Implies all actions
)

// Common resource types
const (
	ResourceUser       = "user"
	ResourceRole       = "role"
	ResourcePermission = "permission"
	ResourceTenant     = "tenant"
	ResourceAPIKey     = "apikey"
	ResourceSession    = "session"
	ResourceAuditLog   = "auditlog"
	ResourceAPIRoute   = "apiroute"
)

// PermissionService handles role-based access control
type PermissionService struct {
	permissionRepo     ports.PermissionRepository
	rolePermissionRepo ports.RolePermissionRepository
	logger             *log.Logger
	cache              map[string]bool // Simple in-memory cache for permission checks
}

// NewPermissionService creates a new permission service
func NewPermissionService(
	permissionRepo ports.PermissionRepository,
	rolePermissionRepo ports.RolePermissionRepository,
	logger *log.Logger,
) *PermissionService {
	return &PermissionService{
		permissionRepo:     permissionRepo,
		rolePermissionRepo: rolePermissionRepo,
		logger:             logger,
		cache:              make(map[string]bool),
	}
}

// CreatePermission creates a new permission
func (s *PermissionService) CreatePermission(ctx context.Context, name, description, action, resource string, isSystem bool) (*domain.Permission, error) {
	// Validate inputs
	if name == "" {
		return nil, fmt.Errorf("permission name is required")
	}
	if action == "" {
		return nil, fmt.Errorf("permission action is required")
	}
	if resource == "" {
		return nil, fmt.Errorf("permission resource is required")
	}

	// Check if permission already exists
	existingPerm, err := s.permissionRepo.GetByName(ctx, name)
	if err == nil && existingPerm != nil {
		return nil, fmt.Errorf("permission with name '%s' already exists", name)
	}

	// Check if action:resource combination already exists
	existingPerm, err = s.permissionRepo.GetByActionAndResource(ctx, action, resource)
	if err == nil && existingPerm != nil {
		return nil, fmt.Errorf("permission for action '%s' on resource '%s' already exists", action, resource)
	}

	// Create the permission
	permission := &domain.Permission{
		Name:        name,
		Description: description,
		Action:      action,
		Resource:    resource,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	createdPerm, err := s.permissionRepo.Create(ctx, permission)
	if err != nil {
		return nil, fmt.Errorf("failed to create permission: %w", err)
	}

	s.logger.Info("Permission created",
		log.String("name", name),
		log.String("action", action),
		log.String("resource", resource))

	// Clear cache
	s.clearCache()

	return createdPerm, nil
}

// GetPermission retrieves a permission by ID
func (s *PermissionService) GetPermission(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	return s.permissionRepo.GetByID(ctx, id)
}

// GetPermissionByName retrieves a permission by name
func (s *PermissionService) GetPermissionByName(ctx context.Context, name string) (*domain.Permission, error) {
	return s.permissionRepo.GetByName(ctx, name)
}

// UpdatePermission updates a permission
func (s *PermissionService) UpdatePermission(ctx context.Context, id uuid.UUID, description string) (*domain.Permission, error) {
	// Get the existing permission
	permission, err := s.permissionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get permission: %w", err)
	}

	// Update fields
	permission.Description = description
	permission.UpdatedAt = time.Now()

	// Save changes
	updatedPerm, err := s.permissionRepo.Update(ctx, id, permission)
	if err != nil {
		return nil, fmt.Errorf("failed to update permission: %w", err)
	}

	s.logger.Info("Permission updated",
		log.String("id", id.String()),
		log.String("name", permission.Name))

	// Clear cache
	s.clearCache()

	return updatedPerm, nil
}

// DeletePermission deletes a permission
func (s *PermissionService) DeletePermission(ctx context.Context, id uuid.UUID) error {
	// Get the permission first for logging
	permission, err := s.permissionRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get permission: %w", err)
	}

	// Check if it's a system permission - using a naming convention instead of a field
	if strings.HasPrefix(permission.Name, "system:") {
		return fmt.Errorf("cannot delete system permission")
	}

	// Delete the permission
	err = s.permissionRepo.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete permission: %w", err)
	}

	s.logger.Info("Permission deleted",
		log.String("id", id.String()),
		log.String("name", permission.Name))

	// Clear cache
	s.clearCache()

	return nil
}

// ListAllPermissions lists all permissions
func (s *PermissionService) ListAllPermissions(ctx context.Context, page, pageSize int) ([]*domain.Permission, error) {
	return s.permissionRepo.ListAll(ctx, page, pageSize)
}

// ListPermissionsByAction lists all permissions for a specific action
func (s *PermissionService) ListPermissionsByAction(ctx context.Context, action string) ([]*domain.Permission, error) {
	return s.permissionRepo.ListByAction(ctx, action)
}

// ListPermissionsByResource lists all permissions for a specific resource
func (s *PermissionService) ListPermissionsByResource(ctx context.Context, resource string) ([]*domain.Permission, error) {
	return s.permissionRepo.ListByResource(ctx, resource)
}

// ListSystemPermissions lists all system permissions
func (s *PermissionService) ListSystemPermissions(ctx context.Context) ([]*domain.Permission, error) {
	return s.permissionRepo.ListSystemPermissions(ctx)
}

// AddPermissionToRole adds a permission to a role
func (s *PermissionService) AddPermissionToRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	err := s.rolePermissionRepo.AddPermissionToRole(ctx, roleID, permissionID)
	if err != nil {
		return fmt.Errorf("failed to add permission to role: %w", err)
	}

	// Clear cache
	s.clearCache()

	return nil
}

// RemovePermissionFromRole removes a permission from a role
func (s *PermissionService) RemovePermissionFromRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	err := s.rolePermissionRepo.RemovePermissionFromRole(ctx, roleID, permissionID)
	if err != nil {
		return fmt.Errorf("failed to remove permission from role: %w", err)
	}

	// Clear cache
	s.clearCache()

	return nil
}

// GetRolePermissions gets all permissions for a role
func (s *PermissionService) GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]*domain.Permission, error) {
	return s.rolePermissionRepo.GetRolePermissions(ctx, roleID)
}

// HasPermission checks if a user with the given roles has a specific permission
func (s *PermissionService) HasPermission(ctx context.Context, roles []string, action, resource string) (bool, error) {
	// Check cache first
	cacheKey := s.makeCacheKey(roles, action, resource)
	if result, ok := s.cache[cacheKey]; ok {
		return result, nil
	}

	// Special case: empty roles
	if len(roles) == 0 {
		return false, nil
	}

	// Convert role names to UUIDs
	// This is a simplified example - in a real implementation, you would need to
	// look up the role IDs from the role names
	roleIDs := make([]uuid.UUID, 0, len(roles))
	for _, role := range roles {
		// This is a placeholder - you would need to implement role name to ID lookup
		roleID, err := uuid.Parse(role)
		if err != nil {
			// If the role is not a valid UUID, skip it
			continue
		}
		roleIDs = append(roleIDs, roleID)
	}

	// Check each role for the permission
	for _, roleID := range roleIDs {
		permissions, err := s.GetRolePermissions(ctx, roleID)
		if err != nil {
			return false, fmt.Errorf("failed to get role permissions: %w", err)
		}

		for _, perm := range permissions {
			// Check for exact match
			if perm.Action == action && perm.Resource == resource {
				s.cache[cacheKey] = true
				return true, nil
			}

			// Check for wildcard action
			if perm.Action == "*" && perm.Resource == resource {
				s.cache[cacheKey] = true
				return true, nil
			}

			// Check for wildcard resource
			if perm.Action == action && perm.Resource == "*" {
				s.cache[cacheKey] = true
				return true, nil
			}

			// Check for full wildcard
			if perm.Action == "*" && perm.Resource == "*" {
				s.cache[cacheKey] = true
				return true, nil
			}

			// Check for manage action (implies all actions)
			if perm.Action == ActionManage && perm.Resource == resource {
				s.cache[cacheKey] = true
				return true, nil
			}

			// Check for resource hierarchy (e.g., "user:profile" matches "user")
			if strings.HasPrefix(resource, perm.Resource+":") &&
				(perm.Action == action || perm.Action == "*" || perm.Action == ActionManage) {
				s.cache[cacheKey] = true
				return true, nil
			}
		}
	}

	// Cache the negative result
	s.cache[cacheKey] = false
	return false, nil
}

// ClearCache clears the permission cache
func (s *PermissionService) ClearCache() {
	s.clearCache()
	s.logger.Info("Permission cache cleared")
}

// Helper methods

// clearCache clears the permission cache
func (s *PermissionService) clearCache() {
	s.cache = make(map[string]bool)
}

// makeCacheKey creates a cache key for a permission check
func (s *PermissionService) makeCacheKey(roles []string, action, resource string) string {
	return fmt.Sprintf("%s:%s:%s", strings.Join(roles, ","), action, resource)
}

// CreateDefaultPermissions creates the default system permissions
func (s *PermissionService) CreateDefaultPermissions(ctx context.Context) error {
	// Define default permissions
	defaultPermissions := []struct {
		name        string
		description string
		action      string
		resource    string
	}{
		// User permissions
		{
			name:        "system:user:create",
			description: "Create users",
			action:      ActionCreate,
			resource:    ResourceUser,
		},
		{
			name:        "system:user:read",
			description: "Read user details",
			action:      ActionRead,
			resource:    ResourceUser,
		},
		{
			name:        "system:user:update",
			description: "Update user details",
			action:      ActionUpdate,
			resource:    ResourceUser,
		},
		{
			name:        "system:user:delete",
			description: "Delete users",
			action:      ActionDelete,
			resource:    ResourceUser,
		},
		{
			name:        "system:user:list",
			description: "List users",
			action:      ActionList,
			resource:    ResourceUser,
		},

		// Role permissions
		{
			name:        "system:role:create",
			description: "Create roles",
			action:      ActionCreate,
			resource:    ResourceRole,
		},
		{
			name:        "system:role:read",
			description: "Read role details",
			action:      ActionRead,
			resource:    ResourceRole,
		},
		{
			name:        "system:role:update",
			description: "Update role details",
			action:      ActionUpdate,
			resource:    ResourceRole,
		},
		{
			name:        "system:role:delete",
			description: "Delete roles",
			action:      ActionDelete,
			resource:    ResourceRole,
		},
		{
			name:        "system:role:list",
			description: "List roles",
			action:      ActionList,
			resource:    ResourceRole,
		},

		// Permission permissions
		{
			name:        "system:permission:create",
			description: "Create permissions",
			action:      ActionCreate,
			resource:    ResourcePermission,
		},
		{
			name:        "system:permission:read",
			description: "Read permission details",
			action:      ActionRead,
			resource:    ResourcePermission,
		},
		{
			name:        "system:permission:update",
			description: "Update permission details",
			action:      ActionUpdate,
			resource:    ResourcePermission,
		},
		{
			name:        "system:permission:delete",
			description: "Delete permissions",
			action:      ActionDelete,
			resource:    ResourcePermission,
		},
		{
			name:        "system:permission:list",
			description: "List permissions",
			action:      ActionList,
			resource:    ResourcePermission,
		},

		// API key permissions
		{
			name:        "system:apikey:create",
			description: "Create API keys",
			action:      ActionCreate,
			resource:    ResourceAPIKey,
		},
		{
			name:        "system:apikey:read",
			description: "Read API key details",
			action:      ActionRead,
			resource:    ResourceAPIKey,
		},
		{
			name:        "system:apikey:update",
			description: "Update API key details",
			action:      ActionUpdate,
			resource:    ResourceAPIKey,
		},
		{
			name:        "system:apikey:delete",
			description: "Delete API keys",
			action:      ActionDelete,
			resource:    ResourceAPIKey,
		},
		{
			name:        "system:apikey:list",
			description: "List API keys",
			action:      ActionList,
			resource:    ResourceAPIKey,
		},

		// Session permissions
		{
			name:        "system:session:read",
			description: "Read session details",
			action:      ActionRead,
			resource:    ResourceSession,
		},
		{
			name:        "system:session:delete",
			description: "Delete sessions",
			action:      ActionDelete,
			resource:    ResourceSession,
		},
		{
			name:        "system:session:list",
			description: "List sessions",
			action:      ActionList,
			resource:    ResourceSession,
		},

		// Audit log permissions
		{
			name:        "system:auditlog:read",
			description: "Read audit log entries",
			action:      ActionRead,
			resource:    ResourceAuditLog,
		},
		{
			name:        "system:auditlog:list",
			description: "List audit log entries",
			action:      ActionList,
			resource:    ResourceAuditLog,
		},

		// API route permissions
		{
			name:        "system:apiroute:create",
			description: "Create API routes",
			action:      ActionCreate,
			resource:    ResourceAPIRoute,
		},
		{
			name:        "system:apiroute:read",
			description: "Read API route details",
			action:      ActionRead,
			resource:    ResourceAPIRoute,
		},
		{
			name:        "system:apiroute:update",
			description: "Update API route details",
			action:      ActionUpdate,
			resource:    ResourceAPIRoute,
		},
		{
			name:        "system:apiroute:delete",
			description: "Delete API routes",
			action:      ActionDelete,
			resource:    ResourceAPIRoute,
		},
		{
			name:        "system:apiroute:list",
			description: "List API routes",
			action:      ActionList,
			resource:    ResourceAPIRoute,
		},

		// Super admin permission
		{
			name:        "system:admin:all",
			description: "Full administrative access",
			action:      "*",
			resource:    "*",
		},
	}

	// Create each permission
	for _, p := range defaultPermissions {
		// Check if permission already exists
		_, err := s.permissionRepo.GetByName(ctx, p.name)
		if err == nil {
			// Permission already exists, skip
			continue
		}

		// Create the permission
		_, err = s.CreatePermission(ctx, p.name, p.description, p.action, p.resource, false)
		if err != nil {
			return fmt.Errorf("failed to create default permission '%s': %w", p.name, err)
		}
	}

	return nil
}
