package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/aegis/internal/common/db"
	"github.com/jasonKoogler/aegis/internal/domain"
	"github.com/jasonKoogler/aegis/internal/ports"
)

// RolePermissionRepository implements the RolePermissionRepository interface
type RolePermissionRepository struct {
	db *db.PostgresPool
}

var _ ports.RolePermissionRepository = &RolePermissionRepository{}

// NewRolePermissionRepository creates a new role-permission repository
func NewRolePermissionRepository(db *db.PostgresPool) *RolePermissionRepository {
	return &RolePermissionRepository{db: db}
}

// AddPermissionToRole adds a permission to a role
func (rpr *RolePermissionRepository) AddPermissionToRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	query := `
		INSERT INTO role_permissions (
			id, role_id, permission_id, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4
		) ON CONFLICT (role_id, permission_id) DO NOTHING
	`

	now := time.Now()
	_, err := rpr.db.Exec(ctx, query, roleID, permissionID, now, now)
	if err != nil {
		return fmt.Errorf("failed to add permission %s to role %s: %w", permissionID, roleID, err)
	}

	return nil
}

// RemovePermissionFromRole removes a permission from a role
func (rpr *RolePermissionRepository) RemovePermissionFromRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	query := `DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2`

	result, err := rpr.db.Exec(ctx, query, roleID, permissionID)
	if err != nil {
		return fmt.Errorf("failed to remove permission %s from role %s: %w", permissionID, roleID, err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("permission %s not found in role %s", permissionID, roleID)
	}

	return nil
}

// GetRolePermissions gets all permissions for a role
func (rpr *RolePermissionRepository) GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]*domain.Permission, error) {
	query := `
		SELECT 
			p.id, p.name, p.description, p.action, p.resource, p.is_system_permission, p.created_at, p.updated_at
		FROM permissions p
		JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.name ASC
	`

	rows, err := rpr.db.Query(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions for role %s: %w", roleID, err)
	}
	defer rows.Close()

	permissions := []*domain.Permission{}
	for rows.Next() {
		var permission domain.Permission
		if err := rows.Scan(
			&permission.ID, &permission.Name, &permission.Description, &permission.Action,
			&permission.Resource, &permission.IsSystemPermission, &permission.CreatedAt, &permission.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan permission data: %w", err)
		}
		permissions = append(permissions, &permission)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating permission rows: %w", err)
	}

	return permissions, nil
}

// GetPermissionRoles gets all roles that have a permission
func (rpr *RolePermissionRepository) GetPermissionRoles(ctx context.Context, permissionID uuid.UUID) ([]*domain.Role, error) {
	query := `
		SELECT 
			r.id, r.name, r.description, r.is_system_role, r.tenant_id, r.created_at, r.updated_at
		FROM roles r
		JOIN role_permissions rp ON r.id = rp.role_id
		WHERE rp.permission_id = $1
		ORDER BY r.name ASC
	`

	rows, err := rpr.db.Query(ctx, query, permissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles for permission %s: %w", permissionID, err)
	}
	defer rows.Close()

	roles := []*domain.Role{}
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(
			&role.ID, &role.Name, &role.Description, &role.IsSystemRole, &role.TenantID,
			&role.CreatedAt, &role.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan role data: %w", err)
		}
		roles = append(roles, &role)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating role rows: %w", err)
	}

	return roles, nil
}
