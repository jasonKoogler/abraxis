package domain

import (
	"net"
	"time"

	"github.com/google/uuid"
)

// APIKey represents an API key for service-to-service authentication
type APIKey struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	KeyPrefix         string    `json:"key_prefix"`
	KeyHash           string    `json:"-"`
	TenantID          uuid.UUID `json:"tenant_id,omitempty"`
	UserID            uuid.UUID `json:"user_id,omitempty"`
	Scopes            []string  `json:"scopes"`
	ExpiresAt         time.Time `json:"expires_at,omitempty"`
	LastUsedAt        time.Time `json:"last_used_at,omitempty"`
	CreatedIPAddress  net.IP    `json:"created_ip_address,omitempty"`
	LastUsedIPAddress net.IP    `json:"last_used_ip_address,omitempty"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
