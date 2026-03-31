package gateway

import "errors"

// Error definitions for the http package
var (
	// ServiceProxy errors
	ErrNilConfig            = errors.New("config cannot be nil")
	ErrNilLogger            = errors.New("logger cannot be nil")
	ErrNilRegistry          = errors.New("registry cannot be nil")
	ErrEmptyConfigDir       = errors.New("config directory cannot be empty")
	ErrNilCircuitBreaker    = errors.New("circuit breaker cannot be nil")
	ErrNilRoutingTable      = errors.New("routing table cannot be nil")
	ErrRepositoryCreation   = errors.New("failed to create service repository")
	ErrRegistryCreation     = errors.New("failed to create service registry")
	ErrLoadFromRepository   = errors.New("failed to load services from repository")
	ErrLoadFromConfig       = errors.New("failed to load services from config")
	ErrServiceAlreadyExists = errors.New("service already exists")
	ErrServiceNotFound      = errors.New("service not found")
	ErrNilServiceRegistry   = errors.New("service registry cannot be nil")
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
	ErrGatewayTimeout     = errors.New("gateway timeout")
)
