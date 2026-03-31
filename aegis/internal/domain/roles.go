package domain

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// UserRole represents a user's role within a tenant.
type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleEditor UserRole = "editor"
	RoleViewer UserRole = "viewer"
)

// roleHierarchy defines a simple ordering to determine permissions.
var roleHierarchy = map[UserRole]int{
	RoleViewer: 1,
	RoleEditor: 2,
	RoleAdmin:  3,
}

// HasPermission returns true if the current role has at least the permissions
// of requiredRole. For example, an admin (3) can satisfy an editor (2) requirement.
func (r UserRole) HasPermission(requiredRole UserRole) bool {
	return roleHierarchy[r] >= roleHierarchy[requiredRole]
}

// RoleFromString converts a string into a valid UserRole, returning an error if invalid.
func RoleFromString(role string) (UserRole, error) {
	switch strings.ToLower(role) {
	case string(RoleAdmin):
		return RoleAdmin, nil
	case string(RoleEditor):
		return RoleEditor, nil
	case string(RoleViewer):
		return RoleViewer, nil
	}
	return "", fmt.Errorf("invalid role: %s", role)
}

// IsValidRole confirms whether a role string is valid.
func IsValidRole(role string) error {
	_, err := RoleFromString(role)
	return err
}

// RoleMap maps a tenant ID to a single UserRole.
type RoleMap map[uuid.UUID]UserRole

// HasRole checks whether, for the given tenant, the user's role meets the required permissions.
func (rm RoleMap) HasRole(tenantID uuid.UUID, requiredRole UserRole) bool {
	userRole, ok := rm[tenantID]
	if !ok {
		return false
	}
	return userRole.HasPermission(requiredRole)
}

// SetRole assigns a role to a tenant after validating the provided role string.
func (rm RoleMap) SetRole(tenantID uuid.UUID, role string) error {
	parsedRole, err := RoleFromString(role)
	if err != nil {
		return err
	}
	rm[tenantID] = parsedRole
	return nil
}

// RemoveRole removes a user's role for the specified tenant.
func (rm RoleMap) RemoveRole(tenantID uuid.UUID) {
	delete(rm, tenantID)
}

func (rm RoleMap) HasAllRoles(tenantID uuid.UUID, roles []string) bool {
	for _, role := range roles {
		if !rm.HasRole(tenantID, UserRole(role)) {
			return false
		}
	}
	return true
}

func (rm RoleMap) GetTenantIDsAsUUIDs() []uuid.UUID {
	if rm == nil {
		return nil
	}
	tenantIDs := make([]uuid.UUID, 0, len(rm))
	for tenantID := range rm {
		tenantIDs = append(tenantIDs, tenantID)
	}
	return tenantIDs
}

func (rm RoleMap) GetTenantIDsAsStrings() []string {
	if rm == nil {
		return nil
	}
	tenantIDs := make([]string, 0, len(rm))
	for tenantID := range rm {
		tenantIDs = append(tenantIDs, FormatTenantID(tenantID))
	}
	return tenantIDs
}

// todo: move to common
func FormatTenantID(tenantID uuid.UUID) string {
	return fmt.Sprintf("ten_%s", tenantID)
}
