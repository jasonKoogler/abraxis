package http

import (
	"time"
)

// PasswordLoginRequest is the request body for POST /auth/login.
type PasswordLoginRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"secretpass123"`
}

// UserRegistrationRequest is the request body for POST /auth/register.
type UserRegistrationRequest struct {
	Email     string `json:"email" example:"user@example.com"`
	FirstName string `json:"firstName" example:"John"`
	LastName  string `json:"lastName" example:"Doe"`
	Password  string `json:"password" example:"secretpass123"`
	Phone     string `json:"phone" example:"+1234567890"`
}

// UpdateUserRequest is the request body for POST /users/{userID}.
type UpdateUserRequest struct {
	AuthProvider  *string    `json:"authProvider,omitempty" example:"google"`
	AvatarURL     *string    `json:"avatarURL,omitempty" example:"https://example.com/avatar.jpg"`
	Email         *string    `json:"email,omitempty" example:"updated@example.com"`
	FirstName     *string    `json:"firstName,omitempty" example:"Jane"`
	LastLoginDate *time.Time `json:"lastLoginDate,omitempty"`
	LastName      *string    `json:"lastName,omitempty" example:"Smith"`
	Phone         *string    `json:"phone,omitempty" example:"+1987654321"`
	Status        *string    `json:"status,omitempty" example:"active" enums:"active,inactive,other"`
}