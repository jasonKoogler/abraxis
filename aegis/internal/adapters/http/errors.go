package http

import "errors"

// Error definitions for the http package
var (
	// Authentication and authorization errors
	ErrInvalidToken       = errors.New("invalid token")
	ErrExpiredToken       = errors.New("token has expired")
	ErrInsufficientScopes = errors.New("insufficient scopes")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")

	// Request errors
	ErrInvalidRequest   = errors.New("invalid request")
	ErrMethodNotAllowed = errors.New("method not allowed")
	ErrRateLimited      = errors.New("rate limit exceeded")

	// Server errors
	ErrInternalServer     = errors.New("internal server error")
	ErrServiceUnavailable = errors.New("service unavailable")
)
