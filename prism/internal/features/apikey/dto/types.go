package dto

import (
	"time"

	"github.com/google/uuid"
)

// ApiKeyCreateRequest is the request body for POST /apikey.
type ApiKeyCreateRequest struct {
	ExpiresInDays *int       `json:"expires_in_days,omitempty" example:"30"`
	Name          string     `json:"name" example:"my-api-key"`
	Scopes        []string   `json:"scopes" example:"read:users,write:users"`
	TenantID      *uuid.UUID `json:"tenant_id,omitempty"`
	UserID        *uuid.UUID `json:"user_id,omitempty"`
}

// ApiKeyValidateRequest is the request body for POST /apikey/validate.
type ApiKeyValidateRequest struct {
	ApiKey string `json:"api_key" example:"ak_abc123..."`
}

// UpdateApiKeyMetadataRequest is the request body for PUT /apikey/{apikeyID}/metadata.
type UpdateApiKeyMetadataRequest struct {
	ExpiresInDays *int      `json:"expires_in_days,omitempty" example:"90"`
	IsActive      *bool     `json:"is_active,omitempty" example:"true"`
	Name          *string   `json:"name,omitempty" example:"updated-key-name"`
	Scopes        *[]string `json:"scopes,omitempty"`
}

// ApiKeyResponse is the response body for an API key.
type ApiKeyResponse struct {
	ID               uuid.UUID `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name             string    `json:"name" example:"my-api-key"`
	KeyPrefix        string    `json:"key_prefix" example:"ak_abcdefgh"`
	TenantID         uuid.UUID `json:"tenant_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440001"`
	UserID           uuid.UUID `json:"user_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440002"`
	Scopes           []string  `json:"scopes" example:"read:users,write:users"`
	ExpiresAt        time.Time `json:"expires_at,omitempty"`
	LastUsedAt       time.Time `json:"last_used_at,omitempty"`
	CreatedIPAddress string    `json:"created_ip_address,omitempty" example:"192.168.1.1"`
	LastUsedIPAddress string   `json:"last_used_ip_address,omitempty" example:"192.168.1.1"`
	IsActive         bool      `json:"is_active" example:"true"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ApiKeyCreateResponse is the one-time response when creating a new API key.
type ApiKeyCreateResponse struct {
	ID        string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name      string    `json:"name" example:"my-api-key"`
	KeyPrefix string    `json:"key_prefix" example:"ak_abcdefgh"`
	RawAPIKey string    `json:"raw_api_key" example:"ak_abcdefghijklmnopqrstuvwxyz123456"`
	Scopes    []string  `json:"scopes" example:"read:users,write:users"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}