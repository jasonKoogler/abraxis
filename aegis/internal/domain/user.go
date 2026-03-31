package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/aegis/internal/common/passwordhasher"
	"github.com/jasonKoogler/aegis/internal/common/validator"
	"github.com/jasonKoogler/aegis/internal/common/validator/is"
)

// User represents a user in the system.
//
// !! IMPORTANT !!
//
// Do Not Attempt to use this type directly.
// Use the user.NewUser() function to create a new user.
type User struct {
	ID            uuid.UUID  `json:"id"`
	Email         string     `json:"email"`
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	Phone         string     `json:"phone"`
	AvatarURL     string     `json:"avatar_url"`
	Status        UserStatus `json:"status"`
	LastLoginDate time.Time  `json:"last_login_date"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	Roles RoleMap `json:"roles"`

	Provider       AuthProvider `json:"provider"`
	ProviderUserID *string      `json:"provider_user_id,omitempty"` // only used for social login providers
	// ProviderAccessToken string       `json:"provider_access_token"` // only used for social login providers
	// ProviderRefreshToken string       `json:"provider_refresh_token"` // only used for social login providers
	PasswordHash string `json:"-"`

	// RawData is used to store the raw data from the provider
	// This is useful for debugging and for storing additional user information
	// This is not serialized to JSON
	ProviderRawData map[string]interface{} `json:"-"`
}

// NewUser creates a new user with provided details.
//
// IMPORTANT: Must pass a valid email, first name, last name, and phone number.
//
// Roles can be added later using the AddRoleToTenant() method
// Returns an error if the email is invalid, or if any of the required fields are not provided.
func NewUser(params *UserCreateParams) (*User, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	var passwordHash string

	if params.Password != nil && *params.Password != "" && params.Provider == AuthProviderPassword {
		hasher := passwordhasher.NewArgon2PasswordHasher()
		ph, err := hasher.Hash(*params.Password)
		if err != nil {
			return nil, err
		}

		passwordHash = ph
	}

	return &User{
		Email:           params.Email,
		FirstName:       params.FirstName,
		LastName:        params.LastName,
		Phone:           params.Phone,
		AvatarURL:       params.AvatarURL,
		Status:          UserStatusActive,
		PasswordHash:    passwordHash,
		Provider:        params.Provider,
		ProviderUserID:  params.ProviderUserID,
		ProviderRawData: params.ProviderRawData,
	}, nil
}

func (u *User) GetRoles() RoleMap {
	return u.Roles
}

func (u *User) FormatID() string {
	return fmt.Sprintf("usr_%s", u.ID.String())
}

func (u *User) SetLastLoginDate() {
	u.LastLoginDate = time.Now()
	u.UpdatedAt = time.Now()
}

// SetPassword hashes and sets the user's password.
func (u *User) SetPassword(password string) error {
	v := validator.New()
	v.CheckString("password", password, is.ValidPassword)
	if err := v.Errors(); err != nil {
		return err
	}

	hasher := passwordhasher.NewArgon2PasswordHasher()
	hashedPassword, err := hasher.Hash(password)
	if err != nil {
		return err
	}

	u.PasswordHash = hashedPassword
	u.UpdatedAt = time.Now()
	return nil
}

func (u *User) ValidatePasswordHash(password string) (bool, error) {
	hasher := passwordhasher.NewArgon2PasswordHasher()
	return hasher.Verify(password, u.PasswordHash)
}

// Update updates the user with the given parameters
// returns an error if the parameters are invalid
// returns ErrNoChangesProvided if no changes are provided
func (u *User) Update(params *UpdateUserParams) error {
	if params == nil {
		return ErrNilUpdateParams
	}

	if err := params.Validate(); err != nil {
		return err
	}

	if !u.applyUpdates(params) {
		return ErrNoChangesProvided
	}

	u.UpdatedAt = time.Now()
	return nil
}

// applyUpdates applies the updates to the user
// returns true if any updates were applied
// returns false if no updates were applied
func (u *User) applyUpdates(params *UpdateUserParams) bool {
	var updated bool

	if params.Email != nil {
		u.Email = *params.Email
		updated = true
	}
	if params.FirstName != nil {
		u.FirstName = *params.FirstName
		updated = true
	}
	if params.LastName != nil {
		u.LastName = *params.LastName
		updated = true
	}
	if params.Phone != nil {
		u.Phone = *params.Phone
		updated = true
	}
	if params.Status != nil {
		status, err := StatusFromString(*params.Status)
		if err != nil {
			return false
		}
		u.Status = status
		updated = true
	}

	return updated
}

// UserStatus is the status of a user
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
	UserStatusLocked   UserStatus = "locked"
	UserStatusDeleted  UserStatus = "deleted"
)

func IsValidStatus() func(p string) error {
	return func(value string) error {
		_, err := StatusFromString(value)
		return err
	}
}

func StatusFromString(status string) (UserStatus, error) {
	switch strings.ToLower(status) {
	case string(UserStatusActive):
		return UserStatusActive, nil
	case string(UserStatusInactive):
		return UserStatusInactive, nil
	case string(UserStatusLocked):
		return UserStatusLocked, nil
	case string(UserStatusDeleted):
		return UserStatusDeleted, nil
	default:
		return "", ErrInvalidUserStatus
	}
}

// UserCreateParams is the objecct for creating a new user
// IMPORTANT: Must call Validate() before using the params
type UserCreateParams struct {
	Email     string
	FirstName string
	LastName  string
	Phone     string
	AvatarURL string

	Password        *string // plaintext password must be hashed
	Provider        AuthProvider
	ProviderUserID  *string // only used for social login providers, represents the user's ID on the provider (e.g. google, github, etc.)
	ProviderRawData map[string]interface{}
}

// Validate validates the create user params
// returns an error if the params are invalid
func (u *UserCreateParams) Validate() error {
	v := validator.New()

	v.CheckString("email", u.Email, is.Email)
	v.CheckString("first_name", u.FirstName, is.Length(1, 50))
	v.CheckString("last_name", u.LastName, is.Length(1, 50))
	v.CheckString("phone", u.Phone, is.ValidPhoneNumber)
	v.CheckOptionalString("password", u.Password, is.ValidPassword)

	return v.Errors()
}

func NewCreateUserParams(email, firstName, lastName, phone string, password *string, provider AuthProvider, providerUserID *string) (*UserCreateParams, error) {

	if provider == AuthProviderPassword && (password == nil || *password == "") {
		return nil, errors.New("password is required")
	}

	if provider != AuthProviderPassword && (password != nil && *password != "") {
		return nil, errors.New("password is not allowed for non-password auth providers")
	}

	params := &UserCreateParams{
		Email:          email,
		FirstName:      firstName,
		LastName:       lastName,
		Phone:          phone,
		Password:       password,
		Provider:       provider,
		ProviderUserID: providerUserID,
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	return params, nil
}

// UpdateUserParams is a struct that holds the parameters that can be updated for a user
// IMPORTANT: the fields are pointers so that we can differentiate between a field that is not updated and a field that is updated to an empty value
//
// IMPORTANT: Must call Validate() before using the params
type UpdateUserParams struct {
	Email           *string
	FirstName       *string
	LastName        *string
	Phone           *string
	Status          *string
	AvatarURL       *string
	Provider        *string
	ProviderUserID  *string
	ProviderRawData *map[string]interface{}
	LastLoginDate   *string
	Roles           *RoleMap
}

// Validate validates the update user params
// returns an error if the params are invalid
func (u *UpdateUserParams) Validate() error {
	v := validator.New()

	v.CheckOptionalString("email", u.Email, is.Email)
	v.CheckOptionalString("first_name", u.FirstName, is.Length(1, 255))
	v.CheckOptionalString("last_name", u.LastName, is.Length(1, 255))
	v.CheckOptionalString("phone", u.Phone, is.ValidPhoneNumber)
	v.CheckOptionalString("status", u.Status, IsValidStatus())
	v.CheckOptionalString("avatar_url", u.AvatarURL, is.URL)
	v.CheckOptionalString("provider", u.Provider)
	v.CheckOptionalString("provider_user_id", u.ProviderUserID)
	v.CheckOptionalString("last_login_date", u.LastLoginDate, is.RFC3339Time)

	return v.Errors()
}

// NewUpdateUserParams creates a new UpdateUserParams instance
//
// IMPORTANT: This is the preferred way to create a new UpdateUserParams instance
func NewUpdateUserParams(
	email,
	firstName,
	lastName,
	phone,
	status,
	avatarURL,
	authProvider,
	lastLoginDate *string,
	roles *RoleMap,
) (*UpdateUserParams, error) {

	params := &UpdateUserParams{
		Email:         email,
		FirstName:     firstName,
		LastName:      lastName,
		Phone:         phone,
		Status:        status,
		AvatarURL:     avatarURL,
		Provider:      authProvider,
		LastLoginDate: lastLoginDate,
		Roles:         roles,
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	return params, nil
}

// todo: fix everything below
func (u *User) TenantIDsAsStrings() []string {
	return u.Roles.GetTenantIDsAsStrings()
}

func (u *User) TenantIDsAsUUIDs() []uuid.UUID {
	return u.Roles.GetTenantIDsAsUUIDs()
}

// func (u *User) AddRoleToTenant(tenantID uuid.UUID, role string) error {
// 	return u.Roles.AddRolesToTenant(tenantID, role)
// }

// func (u *User) RemoveRoleFromTenant(tenantID uuid.UUID, role string) error {
// 	return u.Roles.RemoveRolesFromTenant(tenantID, role)
// }

// func (u *User) HasRole(tenantID uuid.UUID, role string) bool {
// 	return u.Roles.HasRole(tenantID, role)
// }
