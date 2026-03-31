package main

import (
	"context"

	"github.com/jasonKoogler/aegis/internal/adapters/postgres"
	"github.com/jasonKoogler/aegis/internal/adapters/ratelimiter"
	"github.com/jasonKoogler/aegis/internal/app"
	"github.com/jasonKoogler/aegis/internal/common/db"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/jasonKoogler/aegis/internal/common/redis"
	"github.com/jasonKoogler/aegis/internal/config"

	_ "github.com/jasonKoogler/aegis/docs" // swagger docs
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

	// Initialize rate limiter
	rateLimiterParams := ratelimiter.RateLimiterConfigToParams(&cfg.RateLimit)
	rateLimiter, err := ratelimiter.NewRedisRateLimiter(rateLimiterParams)
	if err != nil {
		logger.Fatal("Failed to create redis rate limiter", log.Error(err))
	}

	// Create application with functional options
	application, err := app.NewApp(
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
		app.WithRateLimiter(rateLimiter),
		app.WithRedisClient(redisClient),
		// Initialize all supported services
		app.WithAllDefaultServices(ctx),
	)
	if err != nil {
		logger.Fatal("Failed to create application", log.Error(err))
	}

	// Start the application
	if err := application.Start(); err != nil {
		logger.Fatal("Failed to start application", log.Error(err))
	}
}
