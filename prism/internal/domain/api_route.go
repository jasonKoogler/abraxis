package domain

import (
	"time"

	"github.com/google/uuid"
)

// APIRoute represents a configuration for API gateway routes
type APIRoute struct {
	ID                     uuid.UUID `json:"id"`
	PathPattern            string    `json:"path_pattern"`
	HTTPMethod             string    `json:"http_method"`
	BackendService         string    `json:"backend_service"`
	BackendPath            string    `json:"backend_path,omitempty"`
	RequiresAuthentication bool      `json:"requires_authentication"`
	RequiredScopes         []string  `json:"required_scopes,omitempty"`
	RateLimitPerMinute     int       `json:"rate_limit_per_minute,omitempty"`
	CacheTTLSeconds        int       `json:"cache_ttl_seconds,omitempty"`
	IsActive               bool      `json:"is_active"`
	TenantID               uuid.UUID `json:"tenant_id,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}
