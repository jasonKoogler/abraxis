package app

import "errors"

// Error definitions for the app package
var (
	// Configuration errors
	ErrNilConfig      = errors.New("config cannot be nil")
	ErrConfigRequired = errors.New("config is required")

	// Logger errors
	ErrNilLogger      = errors.New("logger cannot be nil")
	ErrLoggerRequired = errors.New("logger is required")

	// User repository errors
	ErrNilUserRepository      = errors.New("user repository cannot be nil")
	ErrUserRepositoryRequired = errors.New("user repository is required")

	// User service errors
	ErrNilUserService      = errors.New("user service cannot be nil")
	ErrUserServiceRequired = errors.New("user service is required")

	// Auth service errors
	ErrNilAuthService      = errors.New("auth service cannot be nil")
	ErrAuthServiceRequired = errors.New("auth service is required")

	// AuthZ service errors
	ErrNilAuthZService      = errors.New("authorization service cannot be nil")
	ErrAuthZServiceRequired = errors.New("authorization service is required")

	// Rate limiter errors
	ErrNilRateLimiter = errors.New("rate limiter cannot be nil")

	// Redis client errors
	ErrNilRedisClient = errors.New("redis client cannot be nil")

	// Server errors
	ErrNilServer = errors.New("server cannot be nil")

	// Audit service errors
	ErrNilAuditService         = errors.New("audit service cannot be nil")
	ErrAuditServiceRequired    = errors.New("audit service is required")
	ErrNilAuditRepository      = errors.New("audit repository cannot be nil")
	ErrAuditRepositoryRequired = errors.New("audit repository is required")

	// API key service errors
	ErrNilAPIKeyService         = errors.New("API key service cannot be nil")
	ErrAPIKeyServiceRequired    = errors.New("API key service is required")
	ErrNilAPIKeyRepository      = errors.New("API key repository cannot be nil")
	ErrAPIKeyRepositoryRequired = errors.New("API key repository is required")

	// Tenant service errors
	ErrNilTenantService         = errors.New("tenant service cannot be nil")
	ErrTenantServiceRequired    = errors.New("tenant service is required")
	ErrNilTenantRepository      = errors.New("tenant repository cannot be nil")
	ErrTenantRepositoryRequired = errors.New("tenant repository is required")

	// Permission service errors
	ErrNilPermissionService             = errors.New("permission service cannot be nil")
	ErrPermissionServiceRequired        = errors.New("permission service is required")
	ErrNilPermissionRepository          = errors.New("permission repository cannot be nil")
	ErrPermissionRepositoryRequired     = errors.New("permission repository is required")
	ErrNilRolePermissionRepository      = errors.New("role-permission repository cannot be nil")
	ErrRolePermissionRepositoryRequired = errors.New("role-permission repository is required")

	// Initialization errors
	ErrAppInitialization = errors.New("failed to initialize app")
)
