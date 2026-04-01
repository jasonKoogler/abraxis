package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jasonKoogler/abraxis/aegis/internal/adapters/authz"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/redis"
	"github.com/jasonKoogler/abraxis/aegis/internal/config"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
	aegisgrpc "github.com/jasonKoogler/abraxis/aegis/internal/grpc"
	"github.com/jasonKoogler/abraxis/aegis/internal/ports"
	"github.com/jasonKoogler/abraxis/aegis/internal/service"
)

// AppOption is a functional option for configuring the App
type AppOption func(*App) error

// WithConfig sets the configuration for the App
func WithConfig(cfg *config.Config) AppOption {
	return func(a *App) error {
		if cfg == nil {
			return ErrNilConfig
		}
		a.cfg = cfg
		return nil
	}
}

// WithLogger sets a custom logger for the App
func WithLogger(logger *log.Logger) AppOption {
	return func(a *App) error {
		if logger == nil {
			return ErrNilLogger
		}
		a.logger = logger
		return nil
	}
}

// WithDefaultLogger creates a default logger based on the config
func WithDefaultLogger() AppOption {
	return func(a *App) error {
		if a.cfg == nil {
			return ErrConfigRequired
		}
		a.logger = log.NewLogger(a.cfg.LogLevel.String())
		return nil
	}
}

// WithUserRepository sets the user repository for the App
func WithUserRepository(repo ports.UserRepository) AppOption {
	return func(a *App) error {
		if repo == nil {
			return ErrNilUserRepository
		}
		a.userRepo = repo
		return nil
	}
}

// WithUserService sets a custom user service for the App
func WithUserService(userService *service.UserService) AppOption {
	return func(a *App) error {
		if userService == nil {
			return ErrNilUserService
		}
		a.userService = userService
		return nil
	}
}

// WithDefaultUserService creates a default user service based on the user repository
func WithDefaultUserService() AppOption {
	return func(a *App) error {
		if a.userRepo == nil {
			return ErrUserRepositoryRequired
		}
		a.userService = service.NewUserService(a.userRepo)
		return nil
	}
}

// WithAppAuthService sets a custom auth service for the App
func WithAppAuthService(authService *service.AuthManager) AppOption {
	return func(a *App) error {
		if authService == nil {
			return ErrNilAuthService
		}
		a.authService = authService
		return nil
	}
}

// WithDefaultAuthService creates a default auth service based on config and user service
func WithDefaultAuthService() AppOption {
	return func(a *App) error {
		if a.cfg == nil {
			return ErrConfigRequired
		}
		if a.userService == nil {
			return ErrUserServiceRequired
		}
		if a.logger == nil {
			return ErrLoggerRequired
		}
		if a.keyManager == nil {
			km, err := domain.NewKeyManager()
			if err != nil {
				return fmt.Errorf("failed to create key manager: %w", err)
			}
			a.keyManager = km
		}

		authService, err := service.NewAuthManager(&a.cfg.Auth, a.logger, a.userService, a.keyManager)
		if err != nil {
			return err
		}
		a.authService = authService
		return nil
	}
}

// WithRateLimiter sets a custom rate limiter for the App
func WithRateLimiter(rateLimiter ports.RateLimiter) AppOption {
	return func(a *App) error {
		if rateLimiter == nil {
			return ErrNilRateLimiter
		}
		a.rateLimiter = rateLimiter
		return nil
	}
}

// WithRedisClient sets a custom Redis client for the App
func WithRedisClient(client *redis.RedisClient) AppOption {
	return func(a *App) error {
		if client == nil {
			return ErrNilRedisClient
		}
		a.redisClient = client
		return nil
	}
}

// WithDefaultRedisClient creates a default Redis client based on config
func WithDefaultRedisClient(ctx context.Context) AppOption {
	return func(a *App) error {
		if a.cfg == nil {
			return ErrConfigRequired
		}

		redisConfig := a.cfg.Auth.AuthN.RedisConfig
		redisClient, err := redis.NewRedisClient(ctx, a.logger, &redisConfig)
		if err != nil {
			return err
		}
		a.redisClient = redisClient
		return nil
	}
}

// WithServer sets a custom server for the App
func WithServer(srv *Server) AppOption {
	return func(a *App) error {
		if srv == nil {
			return ErrNilServer
		}
		a.srv = srv
		return nil
	}
}

// WithServerAuthService sets a custom server for the App with a specific auth service
func WithServerAuthService(authService *service.AuthManager) AppOption {
	return func(a *App) error {
		if authService == nil {
			return ErrNilAuthService
		}
		a.authService = authService
		return nil
	}
}

// WithDefaultServer creates a default server based on config, logger, and auth service
func WithDefaultServer() AppOption {
	return func(a *App) error {
		if a.cfg == nil {
			return ErrConfigRequired
		}
		if a.logger == nil {
			return ErrLoggerRequired
		}
		if a.authService == nil {
			return ErrAuthServiceRequired
		}

		// Use the server package's WithAuthService function directly
		srv, err := NewServer(a.cfg, a.logger, WithAuthService(a.authService))
		if err != nil {
			return err
		}
		a.srv = srv
		return nil
	}
}

// WithDefaultServices creates default user and auth services
func WithDefaultServices() AppOption {
	return func(a *App) error {
		// Create default user service if needed
		if a.userService == nil {
			if a.userRepo == nil {
				return ErrUserRepositoryRequired
			}
			a.userService = service.NewUserService(a.userRepo)
		}

		// Create key manager if needed
		if a.keyManager == nil {
			km, err := domain.NewKeyManager()
			if err != nil {
				return fmt.Errorf("failed to create key manager: %w", err)
			}
			a.keyManager = km
		}

		// Create default auth service if needed
		if a.authService == nil {
			if a.cfg == nil {
				return ErrConfigRequired
			}
			if a.logger == nil {
				return ErrLoggerRequired
			}
			if a.userService == nil {
				return ErrUserServiceRequired
			}

			authService, err := service.NewAuthManager(&a.cfg.Auth, a.logger, a.userService, a.keyManager)
			if err != nil {
				return err
			}
			a.authService = authService
		}

		return nil
	}
}

// WithAuditService sets a custom audit service for the App
func WithAuditService(auditService *service.AuditService) AppOption {
	return func(a *App) error {
		if auditService == nil {
			return ErrNilAuditService
		}
		a.auditService = auditService
		return nil
	}
}

// WithDefaultAuditService creates a default audit service based on config
func WithDefaultAuditService() AppOption {
	return func(a *App) error {
		if a.logger == nil {
			return ErrLoggerRequired
		}
		if a.auditRepo == nil {
			return ErrAuditRepositoryRequired
		}

		// Create audit service
		auditService := service.NewAuditService(a.auditRepo, a.logger)
		a.auditService = auditService
		return nil
	}
}

// WithAPIKeyService sets a custom API key service for the App
func WithAPIKeyService(apiKeyService *service.APIKeyService) AppOption {
	return func(a *App) error {
		if apiKeyService == nil {
			return ErrNilAPIKeyService
		}
		a.apiKeyService = apiKeyService
		return nil
	}
}

// WithDefaultAPIKeyService creates a default API key service based on config
func WithDefaultAPIKeyService() AppOption {
	return func(a *App) error {
		if a.logger == nil {
			return ErrLoggerRequired
		}
		if a.apiKeyRepo == nil {
			return ErrAPIKeyRepositoryRequired
		}

		// Create API key service
		apiKeyService := service.NewAPIKeyService(a.apiKeyRepo, a.logger)
		a.apiKeyService = apiKeyService
		return nil
	}
}

// WithTenantService sets a custom tenant service for the App
func WithTenantService(tenantService *service.TenantDomainService) AppOption {
	return func(a *App) error {
		if tenantService == nil {
			return ErrNilTenantService
		}
		a.tenantService = tenantService
		return nil
	}
}

// WithDefaultTenantService creates a default tenant service based on config
func WithDefaultTenantService() AppOption {
	return func(a *App) error {
		if a.logger == nil {
			return ErrLoggerRequired
		}
		if a.tenantRepo == nil {
			return ErrTenantRepositoryRequired
		}
		if a.auditService == nil {
			return ErrAuditServiceRequired
		}

		// Create tenant service
		tenantService := service.NewTenantDomainService(a.tenantRepo, a.auditService, a.logger)
		a.tenantService = tenantService
		return nil
	}
}

// WithPermissionService sets a custom permission service for the App
func WithPermissionService(permissionService *service.PermissionService) AppOption {
	return func(a *App) error {
		if permissionService == nil {
			return ErrNilPermissionService
		}
		a.permissionService = permissionService
		return nil
	}
}

// WithDefaultPermissionService creates a default permission service based on config
func WithDefaultPermissionService() AppOption {
	return func(a *App) error {
		if a.logger == nil {
			return ErrLoggerRequired
		}
		if a.permissionRepo == nil {
			return ErrPermissionRepositoryRequired
		}
		if a.rolePermissionRepo == nil {
			return ErrRolePermissionRepositoryRequired
		}

		// Create permission service
		permissionService := service.NewPermissionService(a.permissionRepo, a.rolePermissionRepo, a.logger)
		a.permissionService = permissionService
		return nil
	}
}

// WithAuditRepository sets the audit repository for the App
func WithAuditRepository(repo ports.AuditLogRepository) AppOption {
	return func(a *App) error {
		if repo == nil {
			return ErrNilAuditRepository
		}
		a.auditRepo = repo
		return nil
	}
}

// WithAPIKeyRepository sets the API key repository for the App
func WithAPIKeyRepository(repo ports.APIKeyRepository) AppOption {
	return func(a *App) error {
		if repo == nil {
			return ErrNilAPIKeyRepository
		}
		a.apiKeyRepo = repo
		return nil
	}
}

// WithTenantRepository sets the tenant repository for the App
func WithTenantRepository(repo ports.TenantRepository) AppOption {
	return func(a *App) error {
		if repo == nil {
			return ErrNilTenantRepository
		}
		a.tenantRepo = repo
		return nil
	}
}

// WithPermissionRepository sets the permission repository for the App
func WithPermissionRepository(repo ports.PermissionRepository) AppOption {
	return func(a *App) error {
		if repo == nil {
			return ErrNilPermissionRepository
		}
		a.permissionRepo = repo
		return nil
	}
}

// WithRolePermissionRepository sets the role-permission repository for the App
func WithRolePermissionRepository(repo ports.RolePermissionRepository) AppOption {
	return func(a *App) error {
		if repo == nil {
			return ErrNilRolePermissionRepository
		}
		a.rolePermissionRepo = repo
		return nil
	}
}

// WithAuthZService sets a custom authorization service for the App
func WithAuthZService(authzService *authz.Adapter) AppOption {
	return func(a *App) error {
		if authzService == nil {
			return ErrNilAuthZService
		}
		a.authzService = authzService
		return nil
	}
}

// WithDefaultAuthZService creates a default authorization service based on config
func WithDefaultAuthZService() AppOption {
	return func(a *App) error {
		if a.cfg == nil {
			return ErrConfigRequired
		}
		if a.logger == nil {
			return ErrLoggerRequired
		}

		// Skip if authz is not configured
		if a.cfg.Auth.AuthZ.PolicyFilePath == "" {
			return nil
		}

		// Create authz config
		authzConfig := authz.Config{
			RedisAddr:     a.cfg.Redis.Host + ":" + a.cfg.Redis.Port,
			RedisPassword: a.cfg.Redis.Password,
			RedisDB:       0,
			CacheTTL:      10 * time.Minute,
			MaxCacheSize:  1000,
			WebhookPath:   a.cfg.Auth.AuthZ.WebhookPath,
			WebhookSecret: a.cfg.Auth.AuthZ.GitHubToken,
			Logger:        a.logger,
			LogLevel:      a.cfg.LogLevel.String(),
		}

		// Load policies from file
		policyContent, err := os.ReadFile(a.cfg.Auth.AuthZ.PolicyFilePath)
		if err != nil {
			return fmt.Errorf("failed to read policy file: %w", err)
		}

		// Set the policy
		authzConfig.Policies = map[string]string{
			"authz.rego": string(policyContent),
		}

		// Create the authz adapter
		authzService, err := authz.New(authzConfig)
		if err != nil {
			return fmt.Errorf("failed to create authz adapter: %w", err)
		}

		a.authzService = authzService
		return nil
	}
}

// WithGRPCServer sets the gRPC server. When an AuthManager is already
// configured, the server is automatically registered as its TokenRevoker.
func WithGRPCServer(server *aegisgrpc.AegisAuthServer) AppOption {
	return func(a *App) error {
		a.grpcServer = server
		if a.authService != nil && server != nil {
			a.authService.SetTokenRevoker(server)
		}
		return nil
	}
}

// WithDefaultGRPCServer creates a default gRPC server using the app's configured dependencies.
// It is a no-op when GRPC is not enabled in config. When an AuthManager is available, the
// gRPC server is automatically registered as its TokenRevoker so that logout events are
// broadcast to downstream consumers.
func WithDefaultGRPCServer() AppOption {
	return func(a *App) error {
		if !a.cfg.GRPC.Enabled {
			return nil
		}
		a.grpcServer = aegisgrpc.NewAegisAuthServer(aegisgrpc.ServerConfig{
			Logger:         a.logger,
			UserRepo:       a.userRepo,
			APIKeyService:  a.apiKeyService,
			AuthzAdapter:   a.authzService,
			PolicyFilePath: a.cfg.Auth.AuthZ.PolicyFilePath,
		})

		// Wire the gRPC server as the token revoker for the AuthManager so
		// that Logout publishes revocation events to the gRPC event bus.
		if a.authService != nil {
			a.authService.SetTokenRevoker(a.grpcServer)
		}

		return nil
	}
}

// WithAllDefaultServices creates all default services based on available repositories
func WithAllDefaultServices(ctx context.Context) AppOption {
	return func(a *App) error {
		// Apply individual service options in order
		options := []AppOption{
			WithDefaultUserService(),
			WithDefaultAuthService(),
			WithDefaultAuthZService(),
			WithDefaultAuditService(),
			WithDefaultAPIKeyService(),
			WithDefaultTenantService(),
			WithDefaultPermissionService(),
			WithDefaultServer(),
			WithDefaultGRPCServer(),
		}

		for _, opt := range options {
			// Skip options that fail due to missing dependencies
			// This allows creating services when only some repositories are available
			_ = opt(a)
		}

		return nil
	}
}
