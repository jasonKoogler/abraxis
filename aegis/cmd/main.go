package main

import (
	"context"

	"github.com/jasonKoogler/abraxis/aegis/internal/adapters/postgres"
	"github.com/jasonKoogler/abraxis/aegis/internal/adapters/ratelimiter"
	"github.com/jasonKoogler/abraxis/aegis/internal/app"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/db"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/redis"
	"github.com/jasonKoogler/abraxis/aegis/internal/config"

	_ "github.com/jasonKoogler/abraxis/aegis/docs" // swagger docs
)

// @title           Aegis Auth Service API
// @version         1.0
// @description     Authentication and user management service with JWT (Ed25519/EdDSA), OAuth, and RBAC.
// @host            localhost:8080
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	// Initialize context
	ctx := context.Background()

	// Initialize the logger
	logger := log.NewLogger("debug")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("Failed to load config", log.Error(err))
	}

	// Initialize Postgres connection pool
	pgpool, err := db.NewPostgresPool(ctx, &cfg.Postgres, logger)
	if err != nil {
		logger.Fatal("Failed to create postgres pool", log.Error(err))
	}

	// Create repositories
	userRepo := postgres.NewUserRepository(pgpool)
	auditRepo := postgres.NewAuditLogRepository(pgpool)
	apiKeyRepo := postgres.NewAPIKeyRepository(pgpool)
	tenantRepo := postgres.NewTenantRepository(pgpool)
	permissionRepo := postgres.NewPermissionRepository(pgpool)
	rolePermissionRepo := postgres.NewRolePermissionRepository(pgpool)

	// Initialize Redis client
	redisClient, err := redis.NewRedisClient(ctx, logger, &cfg.Redis)
	if err != nil {
		logger.Fatal("Failed to create Redis client", log.Error(err))
	}

	// Create application with functional options
	opts := []app.AppOption{
		app.WithConfig(cfg),
		app.WithLogger(logger),
		// Repositories
		app.WithUserRepository(userRepo),
		app.WithAuditRepository(auditRepo),
		app.WithAPIKeyRepository(apiKeyRepo),
		app.WithTenantRepository(tenantRepo),
		app.WithPermissionRepository(permissionRepo),
		app.WithRolePermissionRepository(rolePermissionRepo),
		// Infrastructure
		app.WithRedisClient(redisClient),
		// Initialize all supported services
		app.WithAllDefaultServices(ctx),
	}

	// Conditionally add Redis rate limiter
	if cfg.UseRedisRateLimiter {
		rateLimiterParams := ratelimiter.RateLimiterConfigToParams(&cfg.RateLimit)
		rateLimiterParams.RedisClient = redisClient.Client
		rl, err := ratelimiter.NewRedisRateLimiter(rateLimiterParams)
		if err != nil {
			logger.Fatal("Failed to create redis rate limiter", log.Error(err))
		}
		opts = append(opts, app.WithRateLimiter(rl))
	}

	application, err := app.NewApp(opts...)
	if err != nil {
		logger.Fatal("Failed to create application", log.Error(err))
	}

	// Start the application
	if err := application.Start(); err != nil {
		logger.Fatal("Failed to start application", log.Error(err))
	}
}
