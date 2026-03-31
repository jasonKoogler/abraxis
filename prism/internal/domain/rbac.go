package domain

import (
	"time"

	"github.com/google/uuid"
)

// Role represents a role in the RBAC system
type Role struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	IsSystemRole bool      `json:"is_system_role"`
	TenantID     uuid.UUID `json:"tenant_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Permission represents a permission in the RBAC system
type Permission struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description,omitempty"`
	Action             string    `json:"action"`
	Resource           string    `json:"resource"`
	IsSystemPermission bool      `json:"is_system_permission"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// ResourceType represents a resource type in the ABAC system
type ResourceType struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Policy represents an access control policy in the ABAC system
type Policy struct {
	ID             uuid.UUID         `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	ResourceTypeID uuid.UUID         `json:"resource_type_id"`
	Action         string            `json:"action"`
	Effect         PolicyEffect      `json:"effect"`
	Priority       int               `json:"priority"`
	TenantID       uuid.UUID         `json:"tenant_id,omitempty"`
	IsSystemPolicy bool              `json:"is_system_policy"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Conditions     []PolicyCondition `json:"conditions,omitempty"`
}

// PolicyEffect represents the effect of a policy (allow or deny)
type PolicyEffect string

const (
	PolicyEffectAllow PolicyEffect = "allow"
	PolicyEffectDeny  PolicyEffect = "deny"
)

// PolicyCondition represents a condition for a policy
type PolicyCondition struct {
	ID        uuid.UUID `json:"id"`
	PolicyID  uuid.UUID `json:"policy_id"`
	Attribute string    `json:"attribute"`
	Operator  string    `json:"operator"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserAttribute represents an attribute for a user in the ABAC system
type UserAttribute struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ResourceAttribute represents an attribute for a resource in the ABAC system
type ResourceAttribute struct {
	ID             uuid.UUID `json:"id"`
	ResourceTypeID uuid.UUID `json:"resource_type_id"`
	ResourceID     uuid.UUID `json:"resource_id"`
	Key            string    `json:"key"`
	Value          string    `json:"value"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
