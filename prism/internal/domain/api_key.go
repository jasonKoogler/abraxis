package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/prism/internal/common/validator"
	"github.com/jasonKoogler/abraxis/prism/internal/common/validator/is"
)

const (
	// APIKeyPrefix is the prefix for all API keys
	APIKeyPrefix = "ak_"

	// APIKeyLength is the length of the API key in bytes (before encoding)
	APIKeyLength = 32

	// APIKeyPrefixDisplayLength is the number of characters of the key to use as a prefix for display/lookup
	APIKeyPrefixDisplayLength = 8

	// DefaultAPIKeyExpirationDays is the default number of days until an API key expires
	DefaultAPIKeyExpirationDays = 90
)

// APIKey represents an API key for service-to-service authentication
type APIKey struct {
	// ID is the unique identifier for the API key
	ID uuid.UUID `json:"id"`

	// Name is a user-friendly label for the API key
	Name string `json:"name"`

	// KeyPrefix is the visible portion of the API key used for display and lookup
	// Format: ak_XXXXXXXX where X are the first few characters of the encoded key
	KeyPrefix string `json:"key_prefix"`

	// KeyHash is the SHA-256 hash of the complete API key for secure storage and verification
	// Not included in JSON serialization for security
	KeyHash string `json:"-"`

	// RawAPIKey contains the complete API key value
	// Only available immediately after creation and never stored in the database
	RawAPIKey string `json:"raw_api_key"`

	// TenantID identifies the tenant that owns this API key (optional)
	TenantID uuid.UUID `json:"tenant_id,omitempty"`

	// UserID identifies the user that created or owns this API key (optional)
	UserID uuid.UUID `json:"user_id,omitempty"`

	// Scopes defines the permissions granted to this API key
	// Format: resource:action (e.g., "users:read", "documents:*")
	Scopes []string `json:"scopes"`

	// ExpiresAt defines when the API key will no longer be valid
	// By default, keys expire after DefaultAPIKeyExpirationDays
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// LastUsedAt tracks the most recent time the API key was used for authentication
	LastUsedAt time.Time `json:"last_used_at,omitempty"`

	// CreatedIPAddress captures the IP address from which the API key was created
	CreatedIPAddress net.IP `json:"created_ip_address,omitempty"`

	// LastUsedIPAddress tracks the most recent IP address that used this API key
	LastUsedIPAddress net.IP `json:"last_used_ip_address,omitempty"`

	// IsActive indicates whether the API key is currently enabled
	// Can be set to false to temporarily disable the key without deleting it
	IsActive bool `json:"is_active"`

	// CreatedAt records when the API key was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt records when the API key was last modified
	UpdatedAt time.Time `json:"updated_at"`
}

// APIKey_CreateResponse represents the one-time response when creating a new API key
type APIKey_CreateResponse struct {
	// ID is the unique identifier for the API key
	ID string `json:"id"`

	// Name is the user-friendly label for the API key
	Name string `json:"name"`

	// KeyPrefix is the visible portion used for display
	KeyPrefix string `json:"key_prefix"`

	// RawAPIKey is the full API key value that should be shown to the user only once
	// This is the only time the raw key is available
	RawAPIKey string `json:"raw_api_key"`

	// Scopes define the permissions granted to this API key
	Scopes []string `json:"scopes"`

	// ExpiresAt defines when this API key will expire
	ExpiresAt time.Time `json:"expires_at"`

	// Additional fields you might want to include
	CreatedAt time.Time `json:"created_at"`
}

func APIKey_New(params *APIKey_CreateParams) (*APIKey, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	rawKey, keyPrefix, keyHash, err := generateAPIKey()
	if err != nil {
		return nil, err
	}

	var expiresAt time.Time
	if params.ExpiresInDays > 0 {
		expiresAt = time.Now().AddDate(0, 0, params.ExpiresInDays)
	} else {
		expiresAt = time.Now().AddDate(0, 0, DefaultAPIKeyExpirationDays)
	}

	return &APIKey{
		Name:             params.Name,
		TenantID:         params.TenantID,
		UserID:           params.UserID,
		Scopes:           params.Scopes,
		ExpiresAt:        expiresAt,
		IsActive:         true,
		KeyPrefix:        keyPrefix,
		KeyHash:          keyHash,
		RawAPIKey:        rawKey,
		CreatedIPAddress: net.ParseIP(params.IPAddress),
	}, nil
}

// generateAPIKey generates a new random API key, its prefix for lookup, and its hash for storage
func generateAPIKey() (string, string, string, error) {
	// Generate random bytes
	keyBytes := make([]byte, APIKeyLength)
	_, err := rand.Read(keyBytes)
	if err != nil {
		return "", "", "", err
	}

	// Encode as base64
	keyStr := base64.URLEncoding.EncodeToString(keyBytes)

	// Format with prefix
	rawKey := fmt.Sprintf("%s%s", APIKeyPrefix, keyStr)

	// Get the prefix for lookup
	keyPrefix := APIKeyPrefix + keyStr[:APIKeyPrefixDisplayLength]

	// Hash the key for storage
	keyHash := hashAPIKey(rawKey)

	return rawKey, keyPrefix, keyHash, nil
}

// hashAPIKey creates a SHA-256 hash of an API key
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// APIKey_Verify verifies if a raw API key matches a stored hash
func APIKey_Verify(rawKey, storedHash string) bool {
	keyHash := hashAPIKey(rawKey)
	return keyHash == storedHash
}

type APIKey_CreateParams struct {
	Name          string
	TenantID      uuid.UUID
	UserID        uuid.UUID
	Scopes        []string
	IPAddress     string
	ExpiresInDays int
}

func (p *APIKey_CreateParams) Validate() error {
	v := validator.New()

	v.CheckString("name", p.Name, is.String)
	v.CheckString("ip_address", p.IPAddress, is.String)

	return v.Errors()
}

type APIKey_UpdateMetadataParams struct {
	ID            uuid.UUID
	Name          *string
	Scopes        *[]string
	IsActive      *bool
	ExpiresInDays *int

	updateScopes bool // if true, the scopes will be updated, not exported because its set by the validator
}

func (p *APIKey_UpdateMetadataParams) Validate() error {
	v := validator.New()

	// ID is required
	if p.ID == uuid.Nil {
		v.AddError("id", "must be provided")
	}

	// Name validation if provided
	if p.Name != nil {
		v.CheckString("name", *p.Name, is.String, is.Length(1, 255))
	}

	// Scopes validation if provided
	if p.Scopes != nil {
		// Cannot provide empty slice
		if len(*p.Scopes) == 0 {
			v.AddError("scopes", "cannot be empty if provided")
		}

		// Validate individual scopes
		uniqueScopes := make(map[string]bool)
		for _, scope := range *p.Scopes {
			v.CheckString("scope", scope, is.String, is.Pattern(`^[a-z0-9_\-]+:[a-z0-9_\-*]+$`))

			// Check for duplicates
			if uniqueScopes[scope] {
				v.AddError("scopes", fmt.Sprintf("contains duplicate scope: %s", scope))
			}
			uniqueScopes[scope] = true
		}

		p.updateScopes = true
	}

	// ExpiresInDays validation if provided
	if p.ExpiresInDays != nil {
		v.CheckInt("expires_in_days", *p.ExpiresInDays, is.NonNegativeInt, is.Max(3650))
	}

	return v.Errors()
}

func (a *APIKey) UpdateMetadata(params *APIKey_UpdateMetadataParams) error {
	if err := params.Validate(); err != nil {
		return err
	}

	if params.Name != nil {
		a.Name = *params.Name
	}

	if params.Scopes != nil && params.updateScopes {
		a.Scopes = *params.Scopes
	}

	if params.IsActive != nil {
		a.IsActive = *params.IsActive
	}

	if params.ExpiresInDays != nil {
		a.ExpiresAt = time.Now().AddDate(0, 0, *params.ExpiresInDays)
	}

	return nil
}

type APIKey_ListParams struct {
	TenantID *uuid.UUID
	UserID   *uuid.UUID
	Page     int
	PageSize int
}

func (p *APIKey_ListParams) Validate() error {
	v := validator.New()

	if p.TenantID != nil && p.UserID != nil {
		v.AddError("tenant_id", "cannot provide both tenant_id and user_id")
	}

	if p.TenantID == nil && p.UserID == nil {
		v.AddError("tenant_id", "must provide either tenant_id or user_id")
	}

	if p.Page < 1 {
		v.AddError("page", "must be greater than 0")
	}

	if p.PageSize < 1 {
		v.AddError("page_size", "must be greater than 0")
	}

	return v.Errors()
}

type APIKey_ValidateParams struct {
	RawAPIKey string
	IPAddress string
}

func (p *APIKey_ValidateParams) Validate() error {
	v := validator.New()

	if p.RawAPIKey == "" {
		v.AddError("raw_api_key", "must be provided")
	}

	if p.IPAddress != "" {
		v.CheckString("ip_address", p.IPAddress, is.String)
	}

	if net.ParseIP(p.IPAddress) == nil {
		v.AddError("ip_address", "must be a valid IP address")
	}

	return v.Errors()
}

// Validate checks if a raw API key is valid by verifying its format, extracting prefix for lookup,
// and validating against the retrieved API key entity
func (a *APIKey) Validate(rawKey string, currentTime time.Time) error {
	// 1. Check active status
	if !a.IsActive {
		return errors.New("API key is inactive")
	}

	// 2. Check expiration
	if !a.ExpiresAt.IsZero() && a.ExpiresAt.Before(currentTime) {
		return errors.New("API key has expired")
	}

	// 3. Verify hash
	if !APIKey_Verify(rawKey, a.KeyHash) {
		return errors.New("invalid API key")
	}

	return nil
}

// ExtractKeyPrefixForLookup extracts the lookup prefix from a raw API key
// This is a domain helper that can be used by infrastructure/service layers
func ExtractKeyPrefixForLookup(rawKey string) (string, error) {
	// Check prefix
	if !strings.HasPrefix(rawKey, APIKeyPrefix) {
		return "", errors.New("invalid API key format: missing prefix")
	}

	// Split to get the parts
	keyParts := strings.Split(rawKey, "_")
	if len(keyParts) != 2 {
		return "", errors.New("invalid API key format: incorrect format")
	}

	// Get the key value and validate minimum length
	keyValue := keyParts[1]
	if len(keyValue) <= APIKeyPrefixDisplayLength {
		return "", errors.New("invalid API key format: key too short")
	}

	// Return the prefix used for lookup
	return APIKeyPrefix + keyValue[:APIKeyPrefixDisplayLength], nil
}
