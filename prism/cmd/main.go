package main

import (
	"context"

	_ "github.com/jasonKoogler/abraxis/prism/docs" // swagger docs

	auditpg "github.com/jasonKoogler/abraxis/prism/internal/features/audit/adapters/postgres"
	apikeypg "github.com/jasonKoogler/abraxis/prism/internal/features/apikey/adapters/postgres"
	tenantpg "github.com/jasonKoogler/abraxis/prism/internal/features/tenant/adapters"
	"github.com/jasonKoogler/abraxis/prism/internal/features/ratelimit"
	"github.com/jasonKoogler/abraxis/prism/internal/app"
	"github.com/jasonKoogler/abraxis/prism/internal/common/db"
	"github.com/jasonKoogler/abraxis/prism/internal/common/log"
	"github.com/jasonKoogler/abraxis/prism/internal/common/redis"
	"github.com/jasonKoogler/abraxis/prism/internal/config"
)

// @title           Prism API Gateway
// @version         1.0
// @description     API gateway with service routing, audit logging, API key management, and service discovery.
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
	auditRepo := auditpg.NewAuditLogRepository(pgpool)
	apiKeyRepo := apikeypg.NewAPIKeyRepository(pgpool)
	tenantRepo := tenantpg.NewTenantRepository(pgpool)

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
		app.WithAuditRepository(auditRepo),
		app.WithAPIKeyRepository(apiKeyRepo),
		app.WithTenantRepository(tenantRepo),
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
