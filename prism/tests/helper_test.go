package tests

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	apikeypg "github.com/jasonKoogler/abraxis/prism/internal/features/apikey/adapters/postgres"
	auditpg "github.com/jasonKoogler/abraxis/prism/internal/features/audit/adapters/postgres"
	tenantpg "github.com/jasonKoogler/abraxis/prism/internal/features/tenant/adapters"

	"github.com/jasonKoogler/abraxis/prism/internal/app"
	"github.com/jasonKoogler/abraxis/prism/internal/common/db"
	"github.com/jasonKoogler/abraxis/prism/internal/common/log"
	aegisredis "github.com/jasonKoogler/abraxis/prism/internal/common/redis"
	"github.com/jasonKoogler/abraxis/prism/internal/config"
	"github.com/jasonKoogler/abraxis/tests/testutil"
)

// PrismTestServer holds the base URL and Postgres pool for a running Prism
// server started for integration tests.
type PrismTestServer struct {
	BaseURL string
	PGPool  *db.PostgresPool
}

// StartPrismServer spins up a full Prism application against the given Postgres
// and Redis containers. It waits for the /health endpoint to respond, registers
// cleanup via t.Cleanup, and returns a PrismTestServer with the base URL.
func StartPrismServer(t *testing.T, pg *testutil.PostgresContainer, rd *testutil.RedisContainer) *PrismTestServer {
	t.Helper()

	// Pick a random free port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
	listener.Close()

	cfg := &config.Config{
		Environment:         config.Development,
		LogLevel:            config.LogLevelDebug,
		UseRedisRateLimiter: false,
		APIKeys:             map[string]string{},
		Timeouts: config.Timeouts{
			DatabaseQuery: 30 * time.Second,
			HTTPRequest:   30 * time.Second,
			CacheExpiry:   5 * time.Minute,
		},
		Postgres: config.PostgresConfig{
			Host:     pg.Host,
			Port:     pg.Port,
			User:     pg.User,
			Password: pg.Password,
			DB:       pg.DB,
			SSLMode:  "disable",
			Timezone: "UTC",
			Timeout:  "30s",
		},
		Redis: config.RedisConfig{
			Host:     rd.Host,
			Port:     rd.Port,
			Password: rd.Password,
			Username: "default",
		},
		HTTPServer: config.HTTPServerConfig{
			Port:            port,
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    10 * time.Second,
			IdleTimeout:     10 * time.Second,
			ShutdownTimeout: 5 * time.Second,
		},
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 100,
			Burst:             200,
			TTL:               time.Minute,
		},
		Auth: config.AuthConfig{
			AuthN: config.AuthNConfig{
				RedisConfig: config.RedisConfig{
					Host:     rd.Host,
					Port:     rd.Port,
					Password: rd.Password,
					Username: "default",
				},
				SessionManager:         "redis",
				AccessTokenExpiration:  15 * time.Minute,
				RefreshTokenExpiration: 24 * time.Hour,
				TokenRotationInterval:  168 * time.Hour,
			},
		},
		Aegis: config.AegisConfig{
			SyncEnabled: false,
		},
	}

	ctx := context.Background()
	logger := log.NewLogger(cfg.LogLevel.String())

	// Create Postgres pool.
	pgPool, err := db.NewPostgresPool(ctx, &cfg.Postgres, logger)
	if err != nil {
		t.Fatalf("failed to create Postgres pool: %v", err)
	}
	t.Cleanup(func() { pgPool.Close() })

	// Create Redis client.
	redisClient, err := aegisredis.NewRedisClient(ctx, logger, &cfg.Redis)
	if err != nil {
		t.Fatalf("failed to create Redis client: %v", err)
	}
	t.Cleanup(func() { _ = redisClient.Close() })

	// Create repositories.
	auditRepo := auditpg.NewAuditLogRepository(pgPool)
	apiKeyRepo := apikeypg.NewAPIKeyRepository(pgPool)
	tenantRepo := tenantpg.NewTenantRepository(pgPool)

	// Build the App with all services.
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithAuditRepository(auditRepo),
		app.WithAPIKeyRepository(apiKeyRepo),
		app.WithTenantRepository(tenantRepo),
		app.WithRedisClient(redisClient),
		app.WithAllDefaultServices(ctx),
	)
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}

	// Start the app in a background goroutine.
	errCh := make(chan error, 1)
	go func() {
		errCh <- application.Start()
	}()

	// Register cleanup to shut down the app.
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := application.Shutdown(shutdownCtx); shutdownErr != nil {
			t.Logf("warning: app shutdown error: %v", shutdownErr)
		}
	})

	// Wait for /health to respond.
	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)
	healthURL := baseURL + "/health"

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, httpErr := http.Get(healthURL)
		if httpErr == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return &PrismTestServer{BaseURL: baseURL, PGPool: pgPool}
			}
		}
		// Check if the app goroutine failed.
		select {
		case startErr := <-errCh:
			t.Fatalf("app.Start() returned early with error: %v", startErr)
		default:
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("server did not become healthy within 15s at %s", healthURL)
	return nil
}
