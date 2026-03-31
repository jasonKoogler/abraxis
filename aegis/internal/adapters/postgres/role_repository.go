package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/db"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
	"github.com/jasonKoogler/abraxis/aegis/internal/ports"
)

// RoleRepository implements the RoleRepository interface
type RoleRepository struct {
	db *db.PostgresPool
}

var _ ports.RoleRepository = &RoleRepository{}

// NewRoleRepository creates a new role repository
func NewRoleRepository(db *db.PostgresPool) *RoleRepository {
	return &RoleRepository{db: db}
}

// Create creates a new role
func (rr *RoleRepository) Create(ctx context.Context, role *domain.Role) (*domain.Role, error) {
	query := `
		INSERT INTO roles (
			id, name, description, is_system_role, tenant_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		) RETURNING id
	`

	if role.ID == uuid.Nil {
		role.ID = uuid.New()
	}

	now := time.Now()
	role.CreatedAt = now
	role.UpdatedAt = now

	_, err := rr.db.Exec(ctx, query,
		role.ID, role.Name, role.Description, role.IsSystemRole, role.TenantID,
		role.CreatedAt, role.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	return role, nil
}

// GetByID retrieves a role by ID
func (rr *RoleRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	query := `
		SELECT 
			id, name, description, is_system_role, tenant_id, created_at, updated_at
		FROM roles
		WHERE id = $1
	`

	var role domain.Role
	err := rr.db.QueryRow(ctx, query, id).Scan(
		&role.ID, &role.Name, &role.Description, &role.IsSystemRole, &role.TenantID,
		&role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get role by ID %s: %w", id, err)
	}

	return &role, nil
}

// GetByName retrieves a role by name
func (rr *RoleRepository) GetByName(ctx context.Context, name string) (*domain.Role, error) {
	query := `
		SELECT 
			id, name, description, is_system_role, tenant_id, created_at, updated_at
		FROM roles
		WHERE name = $1
	`

	var role domain.Role
	err := rr.db.QueryRow(ctx, query, name).Scan(
		&role.ID, &role.Name, &role.Description, &role.IsSystemRole, &role.TenantID,
		&role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get role by name %s: %w", name, err)
	}

	return &role, nil
}

// Update updates a role
func (rr *RoleRepository) Update(ctx context.Context, id uuid.UUID, role *domain.Role) (*domain.Role, error) {
	query := `
		UPDATE roles
		SET name = $2, description = $3, is_system_role = $4, tenant_id = $5, updated_at = $6
		WHERE id = $1
	`

	role.UpdatedAt = time.Now()

	_, err := rr.db.Exec(ctx, query,
		id, role.Name, role.Description, role.IsSystemRole, role.TenantID, role.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	return role, nil
}

// Delete deletes a role
func (rr *RoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM roles WHERE id = $1`

	result, err := rr.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("role not found")
	}

	return nil
}

// ListAll lists all roles with pagination
func (rr *RoleRepository) ListAll(ctx context.Context, page, pageSize int) ([]*domain.Role, error) {
	query := `
		SELECT 
			id, name, description, is_system_role, tenant_id, created_at, updated_at
		FROM roles
		ORDER BY name ASC
		LIMIT $1 OFFSET $2
	`

	rows, err := rr.db.Query(ctx, query, pageSize, page*pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
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

// ListByTenant lists all roles for a tenant
func (rr *RoleRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*domain.Role, error) {
	query := `
		SELECT 
			id, name, description, is_system_role, tenant_id, created_at, updated_at
		FROM roles
		WHERE tenant_id = $1 OR is_system_role = true
		ORDER BY name ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := rr.db.Query(ctx, query, tenantID, pageSize, page*pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles for tenant %s: %w", tenantID, err)
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

// ListSystemRoles lists all system roles
func (rr *RoleRepository) ListSystemRoles(ctx context.Context) ([]*domain.Role, error) {
	query := `
		SELECT 
			id, name, description, is_system_role, tenant_id, created_at, updated_at
		FROM roles
		WHERE is_system_role = true
		ORDER BY name ASC
	`

	rows, err := rr.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list system roles: %w", err)
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
