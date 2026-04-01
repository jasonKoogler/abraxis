package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/db"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
	"github.com/jasonKoogler/abraxis/aegis/internal/ports"
)

type UserRepository struct {
	db *db.PostgresPool
}

var _ ports.UserRepository = &UserRepository{}

func NewUserRepository(db *db.PostgresPool) *UserRepository {
	return &UserRepository{db: db}
}

func (ur *UserRepository) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	query := `
        INSERT INTO users (
            id, email, first_name, last_name, phone, status, 
            last_login_date, avatar_url, auth_provider, password_hash,
            created_at, updated_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
    `

	_, err := ur.db.Exec(ctx, query,
		user.ID, user.Email, user.FirstName, user.LastName, user.Phone, user.Status,
		user.LastLoginDate, user.AvatarURL, user.Provider, user.PasswordHash,
		user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetByID returns a user by id, along with their tenant/role data
func (ur *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `
        SELECT 
            u.id, u.email, u.first_name, u.last_name, u.phone, u.status,
            u.last_login_date, u.avatar_url, u.auth_provider, u.password_hash,
            u.created_at, u.updated_at
        FROM users u
        WHERE u.id = $1
    `

	var user domain.User
	err := ur.db.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Phone, &user.Status,
		&user.LastLoginDate, &user.AvatarURL, &user.Provider, &user.PasswordHash,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query user with ID %s: %w", id, err)
	}

	// Get user roles
	roleMap, err := ur.GetRoles(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles for user %s: %w", id, err)
	}
	user.Roles = *roleMap

	return &user, nil
}

// GetByEmail returns a user by email, along with their tenant/role data
func (ur *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
        SELECT 
            u.id, u.email, u.first_name, u.last_name, u.phone, u.status,
            u.last_login_date, u.avatar_url, u.auth_provider, u.password_hash,
            u.created_at, u.updated_at
        FROM users u
        WHERE u.email = $1
    `

	var user domain.User
	err := ur.db.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Phone, &user.Status,
		&user.LastLoginDate, &user.AvatarURL, &user.Provider, &user.PasswordHash,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query user with email %s: %w", email, err)
	}

	// Get user roles
	roleMap, err := ur.GetRoles(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles for user %s: %w", user.ID, err)
	}
	user.Roles = *roleMap

	return &user, nil
}

func (ur *UserRepository) Update(ctx context.Context, id uuid.UUID, user *domain.User) (*domain.User, error) {
	// Begin a transaction
	tx, err := ur.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Ensure we either commit at the end or rollback on error/panic
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			_ = tx.Rollback(ctx)
		} else {
			err = tx.Commit(ctx)
		}
	}()

	// 1) Update the fields in the users table
	updateQuery := `
        UPDATE users
        SET email = $2, first_name = $3, last_name = $4, phone = $5,
            status = $6, last_login_date = $7, avatar_url = $8,
            auth_provider = $9, password_hash = $10, updated_at = $11
        WHERE id = $1
    `
	_, err = tx.Exec(ctx, updateQuery,
		id,
		user.Email,
		user.FirstName,
		user.LastName,
		user.Phone,
		user.Status,
		user.LastLoginDate,
		user.AvatarURL,
		user.Provider,
		user.PasswordHash,
		user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update user record: %w", err)
	}

	// 2) Remove all roles for this user
	deleteRolesQuery := `DELETE FROM user_roles WHERE user_id = $1`
	_, err = tx.Exec(ctx, deleteRolesQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to clear user_roles for user %s: %w", id, err)
	}

	// 3) Reinsert roles from user.Roles (RoleMap of tenantID -> Roles).
	//    We need the role IDs from the roles table, so we'll fetch them by name.
	insertUserRoleQuery := `
        INSERT INTO user_roles (id, role_id, user_id, tenant_id)
        VALUES (gen_random_uuid(), $1, $2, $3)
    `
	for tenantID, rolesSlice := range user.Roles {
		for _, role := range rolesSlice {
			var roleID uuid.UUID
			err = tx.QueryRow(ctx, `SELECT id FROM roles WHERE name = $1`, role).
				Scan(&roleID)
			if err != nil {
				return nil, fmt.Errorf("failed to find role %q in roles table: %w", role, err)
			}

			_, err = tx.Exec(ctx, insertUserRoleQuery, roleID, id, tenantID)
			if err != nil {
				return nil, fmt.Errorf("failed to insert user_roles row for role %q: %w", role, err)
			}
		}
	}

	// If we reach here, everything succeeded in the transaction.
	// Defer block above will commit.
	return user, nil
}

func (ur *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`
	result, err := ur.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// GetByOrganizationID returns a list of users in an organization, complete with tenant/role data.
func (ur *UserRepository) GetByOrganizationID(ctx context.Context, organizationID uuid.UUID, page, pageSize int) ([]*domain.User, error) {
	// In the new schema, we use tenants instead of organizations
	query := `
        SELECT
            u.id, u.email, u.first_name, u.last_name, u.phone, u.status,
            u.last_login_date, u.avatar_url, u.auth_provider, u.password_hash,
            u.created_at, u.updated_at
        FROM users u
        JOIN user_tenant_memberships utm ON u.id = utm.user_id
        WHERE utm.tenant_id = $1
        ORDER BY u.created_at DESC
        LIMIT $2 OFFSET $3
    `

	rows, err := ur.db.Query(ctx, query, organizationID, pageSize, page*pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query users by tenant %s: %w", organizationID, err)
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
			return nil, fmt.Errorf("failed to scan user data in tenant %s: %w", organizationID, err)
		}

		// Get user roles
		roleMap, err := ur.GetRoles(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get roles for user %s: %w", user.ID, err)
		}
		user.Roles = *roleMap

		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows for tenant %s: %w", organizationID, err)
	}

	return users, nil
}

// ListAll returns a list of all users, complete with tenant/role data.
func (ur *UserRepository) ListAll(ctx context.Context, page, pageSize int) ([]*domain.User, error) {
	query := `
        SELECT
            u.id, u.email, u.first_name, u.last_name, u.phone, u.status,
            u.last_login_date, u.avatar_url, u.auth_provider, u.password_hash,
            u.created_at, u.updated_at
        FROM users u
        ORDER BY u.created_at DESC
        LIMIT $1 OFFSET $2
    `

	rows, err := ur.db.Query(ctx, query, pageSize, page*pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query all users: %w", err)
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

		// Get user roles
		roleMap, err := ur.GetRoles(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get roles for user %s: %w", user.ID, err)
		}
		user.Roles = *roleMap

		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows when listing all users: %w", err)
	}

	return users, nil
}

func (ur *UserRepository) SetLastLoginDate(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET last_login_date = NOW() WHERE id = $1`
	_, err := ur.db.Exec(ctx, query, id)
	return err
}

// GetRoles returns the roles for a user.
func (ur *UserRepository) GetRoles(ctx context.Context, id uuid.UUID) (*domain.RoleMap, error) {
	query := `
		SELECT ur.tenant_id, r.name
		FROM user_roles ur
		JOIN roles r ON ur.role_id = r.id
		WHERE ur.user_id = $1
	`
	rows, err := ur.db.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles for user %s: %w", id, err)
	}
	defer rows.Close()

	roleMap := domain.RoleMap{}
	for rows.Next() {
		var tenantID uuid.UUID
		var roleName string
		if err := rows.Scan(&tenantID, &roleName); err != nil {
			return nil, fmt.Errorf("failed to scan role for user %s: %w", id, err)
		}

		roleObj, err := domain.RoleFromString(roleName)
		if err != nil {
			return nil, fmt.Errorf("invalid role name %s for user %s: %w", roleName, id, err)
		}

		roleMap[tenantID] = roleObj
	}

	return &roleMap, nil
}

// AddRoleToTenant adds a role to a tenant.
func (ur *UserRepository) AddRoleToTenant(ctx context.Context, id uuid.UUID, tenantID uuid.UUID, role string) error {
	// First, get the role ID from the roles table
	var roleID uuid.UUID
	roleQuery := `SELECT id FROM roles WHERE name = $1`
	err := ur.db.QueryRow(ctx, roleQuery, role).Scan(&roleID)
	if err != nil {
		return fmt.Errorf("failed to find role %s: %w", role, err)
	}

	// Then insert the user role
	query := `
		INSERT INTO user_roles (id, user_id, role_id, tenant_id, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id, role_id, tenant_id) DO NOTHING
	`
	_, err = ur.db.Exec(ctx, query, id, roleID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to add role %s to tenant %s for user %s: %w", role, tenantID, id, err)
	}
	return nil
}

// RemoveRoleFromTenant removes a role from a tenant.
func (ur *UserRepository) RemoveRoleFromTenant(ctx context.Context, id uuid.UUID, tenantID uuid.UUID, role string) error {
	// First, get the role ID from the roles table
	var roleID uuid.UUID
	roleQuery := `SELECT id FROM roles WHERE name = $1`
	err := ur.db.QueryRow(ctx, roleQuery, role).Scan(&roleID)
	if err != nil {
		return fmt.Errorf("failed to find role %s: %w", role, err)
	}

	// Then delete the user role
	query := `DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2 AND tenant_id = $3`
	_, err = ur.db.Exec(ctx, query, id, roleID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to remove role %s from tenant %s for user %s: %w", role, tenantID, id, err)
	}
	return nil
}

// GetUserByProviderUserID returns a user by provider user ID.
func (ur *UserRepository) GetUserByProviderUserID(ctx context.Context, provider string, providerUserID string) (*domain.User, error) {
	query := `
		SELECT 
			id, email, first_name, last_name, phone, status, 
			last_login_date, avatar_url, auth_provider, password_hash,
			created_at, updated_at
		FROM users 
		WHERE auth_provider = $1 AND provider_user_id = $2
	`

	var user domain.User
	err := ur.db.QueryRow(ctx, query, provider, providerUserID).Scan(
		&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Phone, &user.Status,
		&user.LastLoginDate, &user.AvatarURL, &user.Provider, &user.PasswordHash,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by provider user ID %s: %w", providerUserID, err)
	}

	// Get user roles
	roleMap, err := ur.GetRoles(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles for user %s: %w", user.ID, err)
	}
	user.Roles = *roleMap

	return &user, nil
}
