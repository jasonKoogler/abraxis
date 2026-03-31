package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jasonKoogler/abraxis/prism/internal/common/log"
	"github.com/jasonKoogler/abraxis/prism/internal/common/redis"
	"github.com/jasonKoogler/abraxis/prism/internal/config"
	"github.com/jasonKoogler/abraxis/prism/internal/domain"
	"github.com/jasonKoogler/abraxis/prism/internal/features/auth/adapters/authz"
	"github.com/jasonKoogler/abraxis/prism/internal/features/discovery/adapters/provider"
	"github.com/jasonKoogler/abraxis/prism/internal/features/gateway"
	"github.com/jasonKoogler/abraxis/prism/internal/features/gateway/adapters/circuitbreaker"
	aegisclient "github.com/jasonKoogler/abraxis/prism/internal/features/gateway/adapters/aegis"
	"github.com/jasonKoogler/abraxis/prism/internal/ports"
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

// WithTokenValidator sets the token validator for the App
func WithTokenValidator(v *domain.TokenValidator) AppOption {
	return func(a *App) error {
		if v == nil {
			return ErrNilTokenValidator
		}
		a.tokenValidator = v
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

// WithDefaultServer creates a default server based on config and logger
func WithDefaultServer() AppOption {
	return func(a *App) error {
		if a.cfg == nil {
			return ErrConfigRequired
		}
		if a.logger == nil {
			return ErrLoggerRequired
		}

		srv, err := NewServer(a.cfg, a.logger)
		if err != nil {
			return err
		}
		a.srv = srv
		return nil
	}
}

// WithCircuitBreaker sets a custom circuit breaker for the App
func WithCircuitBreaker(cb ports.CircuitBreaker) AppOption {
	return func(a *App) error {
		if cb == nil {
			return ErrNilCircuitBreaker
		}
		a.circuitBreaker = cb
		return nil
	}
}

// WithServiceProxy sets a custom service proxy for the App
func WithServiceProxy(proxy *gateway.ServiceProxy) AppOption {
	return func(a *App) error {
		if proxy == nil {
			return ErrNilServiceProxy
		}
		a.serviceProxy = proxy
		return nil
	}
}

// WithDefaultCircuitBreaker creates a default circuit breaker based on config
func WithDefaultCircuitBreaker() AppOption {
	return func(a *App) error {
		if a.cfg == nil {
			return ErrConfigRequired
		}
		if a.logger == nil {
			return ErrLoggerRequired
		}

		// Skip if circuit breaker is disabled in the config
		if a.cfg.CircuitBreaker == nil || !a.cfg.CircuitBreaker.Enabled {
			return nil
		}

		// Create circuit breaker options
		opts := []circuitbreaker.Option{
			circuitbreaker.WithConfig(a.cfg.CircuitBreaker),
			circuitbreaker.WithLogger(a.logger),
		}

		// If Redis is configured and Redis provider is selected, add Redis options
		if a.cfg.CircuitBreaker.Provider == "redis" && a.cfg.CircuitBreaker.Redis != nil {
			opts = append(opts, circuitbreaker.WithRedisConfig(&circuitbreaker.CircuitBreakerRedisConfig{
				Address:  a.cfg.CircuitBreaker.Redis.Address,
				Password: a.cfg.CircuitBreaker.Redis.Password,
				DB:       a.cfg.CircuitBreaker.Redis.DB,
				CacheTTL: a.cfg.CircuitBreaker.Redis.CacheTTL,
			}))
		}

		// Create the circuit breaker
		cb, err := circuitbreaker.New(opts...)
		if err != nil {
			return err
		}

		a.circuitBreaker = cb
		return nil
	}
}

// WithServiceDiscovery sets a custom service discovery for the App
func WithServiceDiscovery(discovery ports.ServiceDiscoverer) AppOption {
	return func(a *App) error {
		if discovery == nil {
			return ErrNilServiceDiscovery
		}
		a.serviceDiscovery = discovery
		return nil
	}
}

// WithDefaultServiceDiscovery creates a default service discovery based on config
func WithDefaultServiceDiscovery(ctx context.Context) AppOption {
	return func(a *App) error {
		if a.cfg == nil {
			return ErrConfigRequired
		}
		if a.logger == nil {
			return ErrLoggerRequired
		}

		// Load discovery config from environment
		discoveryConfig := provider.LoadDiscoveryConfig()

		// Create the service discovery
		serviceDiscovery, err := provider.NewServiceDiscovery(ctx, discoveryConfig)
		if err != nil {
			return err
		}

		a.serviceDiscovery = serviceDiscovery
		return nil
	}
}

// WithAuditService sets a custom audit service for the App
func WithAuditService(auditService ports.AuditService) AppOption {
	return func(a *App) error {
		if auditService == nil {
			return ErrNilAuditService
		}
		a.auditService = auditService
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

// WithAPIKeyService sets a custom API key service for the App
func WithAPIKeyService(apiKeyService ports.ApiKeyService) AppOption {
	return func(a *App) error {
		if apiKeyService == nil {
			return ErrNilAPIKeyService
		}
		a.apiKeyService = apiKeyService
		return nil
	}
}

// WithAPIKeyRepository sets the API key repository for the App
func WithAPIKeyRepository(repo ports.ApiKeyRepository) AppOption {
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

// WithDefaultJWKSFetcher creates a JWKSFetcher from Aegis config. The actual
// Start() call happens in App.Start() so that we can block until keys are
// loaded before serving traffic.
func WithDefaultJWKSFetcher() AppOption {
	return func(a *App) error {
		if a.cfg == nil {
			return ErrConfigRequired
		}
		if a.cfg.Aegis.JWKSURL == "" {
			return nil
		}
		a.jwksFetcher = domain.NewJWKSFetcher(a.cfg.Aegis.JWKSURL, a.cfg.Aegis.JWKSRefreshInterval)
		return nil
	}
}

// WithAegisClient sets a pre-constructed Aegis gRPC client on the App.
func WithAegisClient(client *aegisclient.Client) AppOption {
	return func(a *App) error {
		a.aegisClient = client
		return nil
	}
}

// WithDefaultAegisClient creates an Aegis gRPC client from config when
// AEGIS_SYNC_ENABLED is true. If the client cannot be created, a warning is
// logged and startup continues without Aegis integration.
func WithDefaultAegisClient() AppOption {
	return func(a *App) error {
		if a.cfg == nil {
			return ErrConfigRequired
		}
		if !a.cfg.Aegis.SyncEnabled {
			return nil
		}
		if a.logger == nil {
			return ErrLoggerRequired
		}

		client := aegisclient.NewClient(
			aegisclient.Config{
				Address:    a.cfg.Aegis.GRPCAddress,
				CacheTTL:   a.cfg.Aegis.CacheTTL,
				MaxBackoff: a.cfg.Aegis.MaxBackoff,
			},
			a.logger,
			a.redisClient,
			a.authzService,
		)
		a.aegisClient = client
		return nil
	}
}

// WithAllDefaultServices creates all default services based on available repositories
func WithAllDefaultServices(ctx context.Context) AppOption {
	return func(a *App) error {
		// Apply individual service options in order
		options := []AppOption{
			WithDefaultAuthZService(),
			WithDefaultServiceDiscovery(ctx),
			WithDefaultServer(),
			WithDefaultCircuitBreaker(),
			WithDefaultJWKSFetcher(),
			WithDefaultAegisClient(),
		}

		for _, opt := range options {
			// Skip options that fail due to missing dependencies
			// This allows creating services when only some repositories are available
			_ = opt(a)
		}

		return nil
	}
}

// WithDefaultServiceProxy creates a default service proxy if one is not provided.
func WithDefaultServiceProxy() AppOption {
	return func(a *App) error {
		if a.serviceProxy == nil {
			if a.cfg == nil {
				return ErrNilConfig
			}
			if a.logger == nil {
				return ErrNilLogger
			}

			// Create proxy with default options
			proxyOptions := []gateway.ServiceProxyOption{
				gateway.WithProxyConfig(a.cfg),
				gateway.WithProxyLogger(a.logger),
			}

			// Add circuit breaker if available
			if a.circuitBreaker != nil {
				proxyOptions = append(proxyOptions, gateway.WithCircuitBreaker(a.circuitBreaker))
			}

			// Note: ServiceProxy now requires a ServiceRegistry as first argument.
			// When no registry is available, we skip proxy creation.
			a.logger.Warn("Service proxy creation requires a ServiceRegistry; skipping default proxy creation")
			return nil
		}
		return nil
	}
}
