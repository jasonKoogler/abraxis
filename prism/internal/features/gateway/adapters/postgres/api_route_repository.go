package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/prism/internal/common/db"
	"github.com/jasonKoogler/prism/internal/domain"
	"github.com/jasonKoogler/prism/internal/ports"
)

// APIRouteRepository implements the APIRouteRepository interface
type APIRouteRepository struct {
	db *db.PostgresPool
}

var _ ports.APIRouteRepository = &APIRouteRepository{}

// NewAPIRouteRepository creates a new API route repository
func NewAPIRouteRepository(db *db.PostgresPool) *APIRouteRepository {
	return &APIRouteRepository{db: db}
}

// Create creates a new API route
func (a *APIRouteRepository) Create(ctx context.Context, route *domain.APIRoute) (*domain.APIRoute, error) {
	query := `
		INSERT INTO api_routes (
			id, path_pattern, http_method, backend_service, backend_path, 
			requires_authentication, required_scopes, rate_limit_per_minute, 
			cache_ttl_seconds, is_active, tenant_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		) RETURNING id
	`

	if route.ID == uuid.Nil {
		route.ID = uuid.New()
	}

	now := time.Now()
	route.CreatedAt = now
	route.UpdatedAt = now

	_, err := a.db.Exec(ctx, query,
		route.ID, route.PathPattern, route.HTTPMethod, route.BackendService, route.BackendPath,
		route.RequiresAuthentication, route.RequiredScopes, route.RateLimitPerMinute,
		route.CacheTTLSeconds, route.IsActive, route.TenantID, route.CreatedAt, route.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create API route: %w", err)
	}

	return route, nil
}

// GetByID retrieves an API route by ID
func (a *APIRouteRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.APIRoute, error) {
	query := `
		SELECT 
			id, path_pattern, http_method, backend_service, backend_path, 
			requires_authentication, required_scopes, rate_limit_per_minute, 
			cache_ttl_seconds, is_active, tenant_id, created_at, updated_at
		FROM api_routes
		WHERE id = $1
	`

	var route domain.APIRoute
	var requiredScopes []string

	err := a.db.QueryRow(ctx, query, id).Scan(
		&route.ID, &route.PathPattern, &route.HTTPMethod, &route.BackendService, &route.BackendPath,
		&route.RequiresAuthentication, &requiredScopes, &route.RateLimitPerMinute,
		&route.CacheTTLSeconds, &route.IsActive, &route.TenantID, &route.CreatedAt, &route.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get API route by ID %s: %w", id, err)
	}

	route.RequiredScopes = requiredScopes

	return &route, nil
}

// GetByPathAndMethod retrieves an API route by path pattern and HTTP method
func (a *APIRouteRepository) GetByPathAndMethod(ctx context.Context, pathPattern, httpMethod string, tenantID *uuid.UUID) (*domain.APIRoute, error) {
	var query string
	var args []interface{}

	if tenantID != nil {
		query = `
			SELECT 
				id, path_pattern, http_method, backend_service, backend_path, 
				requires_authentication, required_scopes, rate_limit_per_minute, 
				cache_ttl_seconds, is_active, tenant_id, created_at, updated_at
			FROM api_routes
			WHERE path_pattern = $1 AND http_method = $2 AND (tenant_id = $3 OR tenant_id IS NULL)
			ORDER BY tenant_id NULLS LAST
			LIMIT 1
		`
		args = []interface{}{pathPattern, httpMethod, tenantID}
	} else {
		query = `
			SELECT 
				id, path_pattern, http_method, backend_service, backend_path, 
				requires_authentication, required_scopes, rate_limit_per_minute, 
				cache_ttl_seconds, is_active, tenant_id, created_at, updated_at
			FROM api_routes
			WHERE path_pattern = $1 AND http_method = $2 AND tenant_id IS NULL
			LIMIT 1
		`
		args = []interface{}{pathPattern, httpMethod}
	}

	var route domain.APIRoute
	var requiredScopes []string

	err := a.db.QueryRow(ctx, query, args...).Scan(
		&route.ID, &route.PathPattern, &route.HTTPMethod, &route.BackendService, &route.BackendPath,
		&route.RequiresAuthentication, &requiredScopes, &route.RateLimitPerMinute,
		&route.CacheTTLSeconds, &route.IsActive, &route.TenantID, &route.CreatedAt, &route.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get API route by path and method: %w", err)
	}

	route.RequiredScopes = requiredScopes

	return &route, nil
}

// Update updates an API route
func (a *APIRouteRepository) Update(ctx context.Context, id uuid.UUID, route *domain.APIRoute) (*domain.APIRoute, error) {
	query := `
		UPDATE api_routes
		SET path_pattern = $2, http_method = $3, backend_service = $4, backend_path = $5, 
			requires_authentication = $6, required_scopes = $7, rate_limit_per_minute = $8, 
			cache_ttl_seconds = $9, is_active = $10, tenant_id = $11, updated_at = $12
		WHERE id = $1
	`

	route.UpdatedAt = time.Now()

	_, err := a.db.Exec(ctx, query,
		id, route.PathPattern, route.HTTPMethod, route.BackendService, route.BackendPath,
		route.RequiresAuthentication, route.RequiredScopes, route.RateLimitPerMinute,
		route.CacheTTLSeconds, route.IsActive, route.TenantID, route.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update API route: %w", err)
	}

	return route, nil
}

// Delete deletes an API route
func (a *APIRouteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM api_routes WHERE id = $1`

	result, err := a.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete API route: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("API route not found")
	}

	return nil
}

// ListByTenant lists all API routes for a tenant
func (a *APIRouteRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*domain.APIRoute, error) {
	query := `
		SELECT 
			id, path_pattern, http_method, backend_service, backend_path, 
			requires_authentication, required_scopes, rate_limit_per_minute, 
			cache_ttl_seconds, is_active, tenant_id, created_at, updated_at
		FROM api_routes
		WHERE tenant_id = $1 OR tenant_id IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return a.queryAPIRoutes(ctx, query, tenantID, pageSize, page*pageSize)
}

// ListByBackendService lists all API routes for a backend service
func (a *APIRouteRepository) ListByBackendService(ctx context.Context, backendService string, page, pageSize int) ([]*domain.APIRoute, error) {
	query := `
		SELECT 
			id, path_pattern, http_method, backend_service, backend_path, 
			requires_authentication, required_scopes, rate_limit_per_minute, 
			cache_ttl_seconds, is_active, tenant_id, created_at, updated_at
		FROM api_routes
		WHERE backend_service = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return a.queryAPIRoutes(ctx, query, backendService, pageSize, page*pageSize)
}

// queryAPIRoutes is a helper function to query API routes
func (a *APIRouteRepository) queryAPIRoutes(ctx context.Context, query string, args ...interface{}) ([]*domain.APIRoute, error) {
	rows, err := a.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query API routes: %w", err)
	}
	defer rows.Close()

	routes := []*domain.APIRoute{}
	for rows.Next() {
		var route domain.APIRoute
		var requiredScopes []string

		if err := rows.Scan(
			&route.ID, &route.PathPattern, &route.HTTPMethod, &route.BackendService, &route.BackendPath,
			&route.RequiresAuthentication, &requiredScopes, &route.RateLimitPerMinute,
			&route.CacheTTLSeconds, &route.IsActive, &route.TenantID, &route.CreatedAt, &route.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan API route data: %w", err)
		}

		route.RequiredScopes = requiredScopes
		routes = append(routes, &route)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API route rows: %w", err)
	}

	return routes, nil
}
