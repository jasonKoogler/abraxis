package domain

import (
	"crypto/ed25519"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	TypeAccessToken  TokenType = "access_token"
	TypeRefreshToken TokenType = "refresh_token"
)

type CustomClaims struct {
	jwt.RegisteredClaims
	TokenType    TokenType `json:"token_type"`
	Roles        RoleMap   `json:"roles"`
	SessionID    string    `json:"session_id"`
	AuthProvider string    `json:"auth_provider"`
}

func (c CustomClaims) GetUserID() string {
	return c.Subject
}

func (c CustomClaims) GetRoles() RoleMap {
	return c.Roles
}

func (c CustomClaims) GetAuthProvider() string {
	return c.AuthProvider
}

func (c CustomClaims) GetSessionID() string {
	return c.SessionID
}

func (c CustomClaims) GetAudience() (jwt.ClaimStrings, error) {
	return c.Audience, nil
}

func (c CustomClaims) GetUserContextData() *UserContextData {
	return &UserContextData{
		UserID:       c.Subject,
		Roles:        c.Roles,
		AuthProvider: c.AuthProvider,
		SessionID:    c.SessionID,
	}
}

// PublicKeyProvider resolves an Ed25519 public key by key ID (kid).
type PublicKeyProvider interface {
	FindPublicKey(kid string) ed25519.PublicKey
}

// TokenValidator handles JWT validation for the gateway.
// Token issuance is handled by Aegis.
type TokenValidator struct {
	keyProvider PublicKeyProvider
}

// NewTokenValidator creates a new token validator that verifies EdDSA
// signatures using public keys from the given provider.
func NewTokenValidator(keyProvider PublicKeyProvider) *TokenValidator {
	return &TokenValidator{
		keyProvider: keyProvider,
	}
}

// ValidateToken validates a JWT token and returns the claims
func (v *TokenValidator) ValidateToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != "EdDSA" {
			return nil, errors.New("unexpected signing method")
		}
		kid, _ := token.Header["kid"].(string)
		key := v.keyProvider.FindPublicKey(kid)
		if key == nil {
			return nil, errors.New("unknown key ID")
		}
		return key, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		if claims.ExpiresAt.Before(time.Now()) {
			return nil, ErrTokenExpired
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// ExtractTokenFromHeader retrieves the access token from the Authorization header.
func ExtractTokenFromHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", nil
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1], nil
	}
	return "", nil
}
