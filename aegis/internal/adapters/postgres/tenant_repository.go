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

// TenantRepository implements the TenantRepository interface
type TenantRepository struct {
	db *db.PostgresPool
}

var _ ports.TenantRepository = &TenantRepository{}

// NewTenantRepository creates a new tenant repository
func NewTenantRepository(db *db.PostgresPool) *TenantRepository {
	return &TenantRepository{db: db}
}

// Create creates a new tenant
func (tr *TenantRepository) Create(ctx context.Context, tenant *domain.Tenant) (*domain.Tenant, error) {
	query := `
		INSERT INTO tenants (
			id, name, domain, status, plan_type, max_users, owner_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		) RETURNING id
	`

	if tenant.ID == uuid.Nil {
		tenant.ID = uuid.New()
	}

	now := time.Now()
	tenant.CreatedAt = now
	tenant.UpdatedAt = now

	_, err := tr.db.Exec(ctx, query,
		tenant.ID, tenant.Name, tenant.Domain, tenant.Status, tenant.PlanType,
		tenant.MaxUsers, tenant.OwnerID, tenant.CreatedAt, tenant.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	return tenant, nil
}

// GetByID retrieves a tenant by ID
func (tr *TenantRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	query := `
		SELECT 
			id, name, domain, status, plan_type, max_users, owner_id, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`

	var tenant domain.Tenant
	err := tr.db.QueryRow(ctx, query, id).Scan(
		&tenant.ID, &tenant.Name, &tenant.Domain, &tenant.Status, &tenant.PlanType,
		&tenant.MaxUsers, &tenant.OwnerID, &tenant.CreatedAt, &tenant.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant by ID %s: %w", id, err)
	}

	return &tenant, nil
}

// GetByDomain retrieves a tenant by domain
func (tr *TenantRepository) GetByDomain(ctx context.Context, d string) (*domain.Tenant, error) {
	query := `
		SELECT 
			id, name, domain, status, plan_type, max_users, owner_id, created_at, updated_at
		FROM tenants
		WHERE domain = $1
	`

	var tenant domain.Tenant
	err := tr.db.QueryRow(ctx, query, d).Scan(
		&tenant.ID, &tenant.Name, &tenant.Domain, &tenant.Status, &tenant.PlanType,
		&tenant.MaxUsers, &tenant.OwnerID, &tenant.CreatedAt, &tenant.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant by domain %s: %w", d, err)
	}

	return &tenant, nil
}

// Update updates a tenant
func (tr *TenantRepository) Update(ctx context.Context, id uuid.UUID, tenant *domain.Tenant) (*domain.Tenant, error) {
	query := `
		UPDATE tenants
		SET name = $2, domain = $3, status = $4, plan_type = $5, max_users = $6, owner_id = $7, updated_at = $8
		WHERE id = $1
	`

	tenant.UpdatedAt = time.Now()

	_, err := tr.db.Exec(ctx, query,
		id, tenant.Name, tenant.Domain, tenant.Status, tenant.PlanType,
		tenant.MaxUsers, tenant.OwnerID, tenant.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update tenant: %w", err)
	}

	return tenant, nil
}

// Delete deletes a tenant
func (tr *TenantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM tenants WHERE id = $1`

	result, err := tr.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete tenant: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("tenant not found")
	}

	return nil
}

// ListAll lists all tenants with pagination
func (tr *TenantRepository) ListAll(ctx context.Context, page, pageSize int) ([]*domain.Tenant, error) {
	query := `
		SELECT 
			id, name, domain, status, plan_type, max_users, owner_id, created_at, updated_at
		FROM tenants
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := tr.db.Query(ctx, query, pageSize, page*pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}
	defer rows.Close()

	tenants := []*domain.Tenant{}
	for rows.Next() {
		var tenant domain.Tenant
		if err := rows.Scan(
			&tenant.ID, &tenant.Name, &tenant.Domain, &tenant.Status, &tenant.PlanType,
			&tenant.MaxUsers, &tenant.OwnerID, &tenant.CreatedAt, &tenant.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan tenant data: %w", err)
		}
		tenants = append(tenants, &tenant)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tenant rows: %w", err)
	}

	return tenants, nil
}

// AddUserToTenant adds a user to a tenant
func (tr *TenantRepository) AddUserToTenant(ctx context.Context, tenantID, userID uuid.UUID, isAdmin bool) error {
	query := `
		INSERT INTO user_tenant_memberships (
			id, user_id, tenant_id, status, is_tenant_admin, joined_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, $2, 'active', $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		) ON CONFLICT (user_id, tenant_id) 
		DO UPDATE SET 
			status = 'active', 
			is_tenant_admin = $3, 
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := tr.db.Exec(ctx, query, userID, tenantID, isAdmin)
	if err != nil {
		return fmt.Errorf("failed to add user to tenant: %w", err)
	}

	return nil
}

// RemoveUserFromTenant removes a user from a tenant
func (tr *TenantRepository) RemoveUserFromTenant(ctx context.Context, tenantID, userID uuid.UUID) error {
	query := `DELETE FROM user_tenant_memberships WHERE tenant_id = $1 AND user_id = $2`

	result, err := tr.db.Exec(ctx, query, tenantID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove user from tenant: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found in tenant")
	}

	return nil
}

// GetTenantUsers gets all users for a tenant
func (tr *TenantRepository) GetTenantUsers(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*domain.User, error) {
	query := `
		SELECT 
			u.id, u.email, u.first_name, u.last_name, u.phone, u.status,
			u.last_login_date, u.avatar_url, u.auth_provider, u.password_hash,
			u.created_at, u.updated_at
		FROM users u
		JOIN user_tenant_memberships utm ON u.id = utm.user_id
		WHERE utm.tenant_id = $1
		ORDER BY utm.joined_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := tr.db.Query(ctx, query, tenantID, pageSize, page*pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant users: %w", err)
	}
	defer rows.Close()

	users := []*domain.User{}
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(
			&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Phone, &user.Status,
			&user.LastLoginDate, &user.AvatarURL, &user.Provider, &user.PasswordHash,
			&user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user data: %w", err)
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user rows: %w", err)
	}

	return users, nil
}

// GetUserTenants gets all tenants for a user
func (tr *TenantRepository) GetUserTenants(ctx context.Context, userID uuid.UUID) ([]*domain.Tenant, error) {
	query := `
		SELECT 
			t.id, t.name, t.domain, t.status, t.plan_type, t.max_users, t.owner_id, t.created_at, t.updated_at
		FROM tenants t
		JOIN user_tenant_memberships utm ON t.id = utm.tenant_id
		WHERE utm.user_id = $1
		ORDER BY utm.is_default_tenant DESC, t.name ASC
	`

	rows, err := tr.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user tenants: %w", err)
	}
	defer rows.Close()

	tenants := []*domain.Tenant{}
	for rows.Next() {
		var tenant domain.Tenant
		if err := rows.Scan(
			&tenant.ID, &tenant.Name, &tenant.Domain, &tenant.Status, &tenant.PlanType,
			&tenant.MaxUsers, &tenant.OwnerID, &tenant.CreatedAt, &tenant.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan tenant data: %w", err)
		}
		tenants = append(tenants, &tenant)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tenant rows: %w", err)
	}

	return tenants, nil
}

// SetDefaultTenant sets a tenant as the default for a user
func (tr *TenantRepository) SetDefaultTenant(ctx context.Context, userID, tenantID uuid.UUID) error {
	query := `
		UPDATE user_tenant_memberships
		SET is_default_tenant = (tenant_id = $2)
		WHERE user_id = $1
	`

	_, err := tr.db.Exec(ctx, query, userID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to set default tenant: %w", err)
	}

	return nil
}
