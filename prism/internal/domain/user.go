package domain

// AuthenticatedUser represents a user as known to the gateway from JWT claims.
// The full user model lives in Aegis — Prism only sees what's in the token.
type AuthenticatedUser struct {
	ID       string  `json:"id"`
	TenantID string  `json:"tenant_id,omitempty"`
	Roles    RoleMap `json:"roles"`
}
