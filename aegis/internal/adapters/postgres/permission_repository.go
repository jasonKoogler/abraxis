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

// PermissionRepository implements the PermissionRepository interface
type PermissionRepository struct {
	db *db.PostgresPool
}

var _ ports.PermissionRepository = &PermissionRepository{}

// NewPermissionRepository creates a new permission repository
func NewPermissionRepository(db *db.PostgresPool) *PermissionRepository {
	return &PermissionRepository{db: db}
}

// Create creates a new permission
func (pr *PermissionRepository) Create(ctx context.Context, permission *domain.Permission) (*domain.Permission, error) {
	query := `
		INSERT INTO permissions (
			id, name, description, action, resource, is_system_permission, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		) RETURNING id
	`

	if permission.ID == uuid.Nil {
		permission.ID = uuid.New()
	}

	now := time.Now()
	permission.CreatedAt = now
	permission.UpdatedAt = now

	_, err := pr.db.Exec(ctx, query,
		permission.ID, permission.Name, permission.Description, permission.Action,
		permission.Resource, permission.IsSystemPermission, permission.CreatedAt, permission.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create permission: %w", err)
	}

	return permission, nil
}

// GetByID retrieves a permission by ID
func (pr *PermissionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	query := `
		SELECT 
			id, name, description, action, resource, is_system_permission, created_at, updated_at
		FROM permissions
		WHERE id = $1
	`

	var permission domain.Permission
	err := pr.db.QueryRow(ctx, query, id).Scan(
		&permission.ID, &permission.Name, &permission.Description, &permission.Action,
		&permission.Resource, &permission.IsSystemPermission, &permission.CreatedAt, &permission.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get permission by ID %s: %w", id, err)
	}

	return &permission, nil
}

// GetByName retrieves a permission by name
func (pr *PermissionRepository) GetByName(ctx context.Context, name string) (*domain.Permission, error) {
	query := `
		SELECT 
			id, name, description, action, resource, is_system_permission, created_at, updated_at
		FROM permissions
		WHERE name = $1
	`

	var permission domain.Permission
	err := pr.db.QueryRow(ctx, query, name).Scan(
		&permission.ID, &permission.Name, &permission.Description, &permission.Action,
		&permission.Resource, &permission.IsSystemPermission, &permission.CreatedAt, &permission.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get permission by name %s: %w", name, err)
	}

	return &permission, nil
}

// GetByActionAndResource retrieves a permission by action and resource
func (pr *PermissionRepository) GetByActionAndResource(ctx context.Context, action, resource string) (*domain.Permission, error) {
	query := `
		SELECT 
			id, name, description, action, resource, is_system_permission, created_at, updated_at
		FROM permissions
		WHERE action = $1 AND resource = $2
	`

	var permission domain.Permission
	err := pr.db.QueryRow(ctx, query, action, resource).Scan(
		&permission.ID, &permission.Name, &permission.Description, &permission.Action,
		&permission.Resource, &permission.IsSystemPermission, &permission.CreatedAt, &permission.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get permission by action %s and resource %s: %w", action, resource, err)
	}

	return &permission, nil
}

// Update updates a permission
func (pr *PermissionRepository) Update(ctx context.Context, id uuid.UUID, permission *domain.Permission) (*domain.Permission, error) {
	query := `
		UPDATE permissions
		SET name = $2, description = $3, action = $4, resource = $5, is_system_permission = $6, updated_at = $7
		WHERE id = $1
	`

	permission.UpdatedAt = time.Now()

	_, err := pr.db.Exec(ctx, query,
		id, permission.Name, permission.Description, permission.Action,
		permission.Resource, permission.IsSystemPermission, permission.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update permission: %w", err)
	}

	return permission, nil
}

// Delete deletes a permission
func (pr *PermissionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM permissions WHERE id = $1`

	result, err := pr.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete permission: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("permission not found")
	}

	return nil
}

// ListAll lists all permissions with pagination
func (pr *PermissionRepository) ListAll(ctx context.Context, page, pageSize int) ([]*domain.Permission, error) {
	query := `
		SELECT 
			id, name, description, action, resource, is_system_permission, created_at, updated_at
		FROM permissions
		ORDER BY name ASC
		LIMIT $1 OFFSET $2
	`

	rows, err := pr.db.Query(ctx, query, pageSize, page*pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list permissions: %w", err)
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

// ListByAction lists all permissions for an action
func (pr *PermissionRepository) ListByAction(ctx context.Context, action string) ([]*domain.Permission, error) {
	query := `
		SELECT 
			id, name, description, action, resource, is_system_permission, created_at, updated_at
		FROM permissions
		WHERE action = $1
		ORDER BY name ASC
	`

	rows, err := pr.db.Query(ctx, query, action)
	if err != nil {
		return nil, fmt.Errorf("failed to list permissions by action %s: %w", action, err)
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

// ListByResource lists all permissions for a resource
func (pr *PermissionRepository) ListByResource(ctx context.Context, resource string) ([]*domain.Permission, error) {
	query := `
		SELECT 
			id, name, description, action, resource, is_system_permission, created_at, updated_at
		FROM permissions
		WHERE resource = $1
		ORDER BY name ASC
	`

	rows, err := pr.db.Query(ctx, query, resource)
	if err != nil {
		return nil, fmt.Errorf("failed to list permissions by resource %s: %w", resource, err)
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

// ListSystemPermissions lists all system permissions
func (pr *PermissionRepository) ListSystemPermissions(ctx context.Context) ([]*domain.Permission, error) {
	query := `
		SELECT 
			id, name, description, action, resource, is_system_permission, created_at, updated_at
		FROM permissions
		WHERE is_system_permission = true
		ORDER BY name ASC
	`

	rows, err := pr.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list system permissions: %w", err)
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
