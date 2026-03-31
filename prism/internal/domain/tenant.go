package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/prism/internal/common/validator"
	"github.com/jasonKoogler/prism/internal/common/validator/is"
)

// TenantStatus represents the status of a tenant
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusInactive  TenantStatus = "inactive"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusDeleted   TenantStatus = "deleted"
)

// Tenant represents a tenant/organization in the system
type Tenant struct {
	ID        uuid.UUID    `json:"id"`
	Name      string       `json:"name"`
	Domain    string       `json:"domain,omitempty"`
	Status    TenantStatus `json:"status"`
	PlanType  string       `json:"plan_type,omitempty"`
	MaxUsers  int          `json:"max_users,omitempty"`
	OwnerID   uuid.UUID    `json:"owner_id,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// NewTenant creates a new tenant with the provided details
func NewTenant(params *TenantCreateParams) (*Tenant, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	return &Tenant{
		Name:     params.Name,
		Domain:   params.Domain,
		Status:   TenantStatusActive,
		PlanType: params.PlanType,
		MaxUsers: params.MaxUsers,
		OwnerID:  params.OwnerID,
	}, nil
}

// FormatID returns a formatted tenant ID
func (t *Tenant) FormatID() string {
	return fmt.Sprintf("ten_%s", t.ID.String())
}

// TenantCreateParams contains parameters for creating a new tenant
type TenantCreateParams struct {
	Name     string
	Domain   string
	PlanType string
	MaxUsers int
	OwnerID  uuid.UUID
}

// Validate validates the tenant creation parameters
func (p *TenantCreateParams) Validate() error {
	v := validator.New()

	v.CheckString("name", p.Name, is.Length(1, 255))

	if p.Domain != "" {
		v.CheckString("domain", p.Domain, is.Length(1, 255))
	}

	return v.Errors()
}

// TenantUpdateParams contains parameters for updating a tenant
type TenantUpdateParams struct {
	Name     *string
	Domain   *string
	Status   *string
	PlanType *string
	MaxUsers *int
	OwnerID  *uuid.UUID
}

// Validate validates the tenant update parameters
func (p *TenantUpdateParams) Validate() error {
	v := validator.New()

	if p.Name != nil {
		v.CheckString("name", *p.Name, is.Length(1, 255))
	}

	if p.Domain != nil {
		v.CheckString("domain", *p.Domain, is.Length(1, 255))
	}

	if p.Status != nil {
		valid := false
		for _, s := range []TenantStatus{
			TenantStatusActive,
			TenantStatusInactive,
			TenantStatusSuspended,
			TenantStatusDeleted,
		} {
			if TenantStatus(*p.Status) == s {
				valid = true
				break
			}
		}
		if !valid {
			v.AddError("status", "must be one of: active, inactive, suspended, deleted")
		}
	}

	return v.Errors()
}

// Update updates the tenant with the given parameters
func (t *Tenant) Update(params *TenantUpdateParams) error {
	if params == nil {
		return ErrNilUpdateParams
	}

	if err := params.Validate(); err != nil {
		return err
	}

	if !t.applyUpdates(params) {
		return ErrNoChangesProvided
	}

	t.UpdatedAt = time.Now()
	return nil
}

// applyUpdates applies the updates to the tenant
func (t *Tenant) applyUpdates(params *TenantUpdateParams) bool {
	var updated bool

	if params.Name != nil {
		t.Name = *params.Name
		updated = true
	}

	if params.Domain != nil {
		t.Domain = *params.Domain
		updated = true
	}

	if params.Status != nil {
		t.Status = TenantStatus(*params.Status)
		updated = true
	}

	if params.PlanType != nil {
		t.PlanType = *params.PlanType
		updated = true
	}

	if params.MaxUsers != nil {
		t.MaxUsers = *params.MaxUsers
		updated = true
	}

	if params.OwnerID != nil {
		t.OwnerID = *params.OwnerID
		updated = true
	}

	return updated
}
