package domain

import "errors"

var (
	ErrSessionNotFound   = errors.New("session not found")
	ErrInvalidSession    = errors.New("invalid session")
	ErrTokenExpired      = errors.New("token expired")
	ErrSessionExpired    = errors.New("session expired")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")

	ErrInvalidToken        = errors.New("invalid token")
	ErrInvalidTokenBinding = errors.New("invalid token binding")
	ErrNoToken             = errors.New("no token provided")
	ErrInvalidCredentials  = errors.New("invalid credentials")

	ErrSessionCreationFailed     = errors.New("session creation failed")
	ErrTokenRefreshFailed        = errors.New("token refresh failed")
	ErrSessionInvalidationFailed = errors.New("session invalidation failed")
	ErrTokenCreationFailed       = errors.New("token creation failed")

	ErrInvalidTenantID   = errors.New("invalid tenant ID")
	ErrInvalidUserID     = errors.New("invalid user ID")
	ErrInvalidSessionID  = errors.New("invalid session ID")
	ErrInvalidSessionKey = errors.New("invalid session key")

	ErrTokenRevoked = errors.New("token revoked")

	ErrTooManyRequests = errors.New("too many requests")
)
