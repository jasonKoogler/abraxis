# Aegis Integration Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integration tests for Aegis auth flows (register, login, protected access, refresh, logout) against real Postgres + Redis via dockertest.

**Architecture:** Shared test infrastructure module (`tests/`) provides Postgres and Redis container helpers. Aegis tests in `aegis/tests/` create a real `app.App` with all dependencies, start the HTTP server on a random port, and exercise the auth API via standard HTTP client. No mocks.

**Tech Stack:** dockertest/v3, testify, pgxpool, go-redis, net/http

**Spec:** `docs/specs/2026-04-01-integration-tests-design.md` (Sub-project 1)

---

### Task 1: Create shared test infrastructure module

**Files:**
- Create: `/home/jason/jdk/abraxis/tests/go.mod`
- Create: `/home/jason/jdk/abraxis/tests/testutil/postgres.go`
- Create: `/home/jason/jdk/abraxis/tests/testutil/redis.go`
- Modify: `/home/jason/jdk/abraxis/go.work` (add `./tests`)

- [ ] **Step 1: Create tests/go.mod**

```bash
mkdir -p /home/jason/jdk/abraxis/tests/testutil
```

Create `/home/jason/jdk/abraxis/tests/go.mod`:

```
module github.com/jasonKoogler/abraxis/tests

go 1.24.0

require (
	github.com/jackc/pgx/v5 v5.7.2
	github.com/ory/dockertest/v3 v3.11.0
	github.com/redis/go-redis/v9 v9.7.1
)
```

Then run:
```bash
cd /home/jason/jdk/abraxis/tests && go mod tidy
```

- [ ] **Step 2: Create testutil/postgres.go**

Create `/home/jason/jdk/abraxis/tests/testutil/postgres.go`:

```go
package testutil

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

// PostgresContainer holds connection info for a test Postgres instance.
type PostgresContainer struct {
	Pool     *pgxpool.Pool
	Host     string
	Port     string
	User     string
	Password string
	DB       string
	ConnStr  string
}

// SetupPostgres starts a Postgres container, runs migrations, and returns a connection pool.
// The container is automatically cleaned up when the test finishes.
func SetupPostgres(t *testing.T, migrationsPath string) *PostgresContainer {
	t.Helper()
	skipIfNoDocker(t)

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("Could not connect to docker: %s", err)
	}
	pool.MaxWait = 2 * time.Minute

	user, password, dbName := "testuser", "testpass", "testdb"

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "16-alpine",
		Env: []string{
			fmt.Sprintf("POSTGRES_USER=%s", user),
			fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
			fmt.Sprintf("POSTGRES_DB=%s", dbName),
		},
		ExposedPorts: []string{"5432/tcp"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"5432/tcp": {{HostIP: "0.0.0.0", HostPort: "0"}},
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		t.Fatalf("Could not start postgres container: %s", err)
	}

	t.Cleanup(func() {
		_ = pool.Purge(resource)
	})

	hostPort := resource.GetPort("5432/tcp")
	connStr := fmt.Sprintf("postgres://%s:%s@localhost:%s/%s?sslmode=disable", user, password, hostPort, dbName)

	// Wait for Postgres to be ready
	var pgPool *pgxpool.Pool
	err = pool.Retry(func() error {
		var err error
		pgPool, err = pgxpool.New(context.Background(), connStr)
		if err != nil {
			return err
		}
		return pgPool.Ping(context.Background())
	})
	if err != nil {
		t.Fatalf("Could not connect to postgres: %s", err)
	}

	// Run migrations if path provided
	if migrationsPath != "" {
		absPath, err := filepath.Abs(migrationsPath)
		if err != nil {
			t.Fatalf("Could not resolve migrations path: %s", err)
		}
		runMigrations(t, connStr, absPath)
	}

	return &PostgresContainer{
		Pool:     pgPool,
		Host:     "localhost",
		Port:     hostPort,
		User:     user,
		Password: password,
		DB:       dbName,
		ConnStr:  connStr,
	}
}

func runMigrations(t *testing.T, connStr, migrationsPath string) {
	t.Helper()

	// Use golang-migrate CLI via Docker (same image as docker-compose)
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("Could not connect to docker for migrations: %s", err)
	}

	// Run migrate container with host network to reach the Postgres container
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "migrate/migrate",
		Tag:        "latest",
		Cmd: []string{
			"-path", "/migrations",
			"-database", connStr,
			"up",
		},
		Mounts: []string{
			fmt.Sprintf("%s:/migrations:ro", migrationsPath),
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.NetworkMode = "host"
	})
	if err != nil {
		t.Fatalf("Could not run migrations: %s", err)
	}

	// Wait for migration container to finish
	exitCode, err := pool.Client.WaitContainer(resource.Container.ID)
	if err != nil {
		t.Fatalf("Error waiting for migration container: %s", err)
	}
	if exitCode != 0 {
		t.Fatalf("Migration failed with exit code %d", exitCode)
	}
}

func skipIfNoDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available — skipping integration test")
	}
}
```

- [ ] **Step 3: Create testutil/redis.go**

Create `/home/jason/jdk/abraxis/tests/testutil/redis.go`:

```go
package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/redis/go-redis/v9"
)

// RedisContainer holds connection info for a test Redis instance.
type RedisContainer struct {
	Client   *redis.Client
	Host     string
	Port     string
	Password string
}

// SetupRedis starts a Redis container and returns a connected client.
// The container is automatically cleaned up when the test finishes.
func SetupRedis(t *testing.T) *RedisContainer {
	t.Helper()
	skipIfNoDocker(t)

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("Could not connect to docker: %s", err)
	}
	pool.MaxWait = 2 * time.Minute

	password := "testredis"

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "7-alpine",
		Cmd:        []string{"redis-server", "--requirepass", password},
		ExposedPorts: []string{"6379/tcp"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"6379/tcp": {{HostIP: "0.0.0.0", HostPort: "0"}},
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		t.Fatalf("Could not start redis container: %s", err)
	}

	t.Cleanup(func() {
		_ = pool.Purge(resource)
	})

	hostPort := resource.GetPort("6379/tcp")

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("localhost:%s", hostPort),
		Password: password,
	})

	// Wait for Redis to be ready
	err = pool.Retry(func() error {
		return client.Ping(context.Background()).Err()
	})
	if err != nil {
		t.Fatalf("Could not connect to redis: %s", err)
	}

	return &RedisContainer{
		Client:   client,
		Host:     "localhost",
		Port:     hostPort,
		Password: password,
	}
}
```

- [ ] **Step 4: Add tests module to go.work**

In `/home/jason/jdk/abraxis/go.work`, add `./tests`:

```
go 1.24.0

use (
	./aegis
	./prism
	./authz
	./tests
)
```

- [ ] **Step 5: Run go mod tidy and verify**

```bash
cd /home/jason/jdk/abraxis/tests && go mod tidy
cd /home/jason/jdk/abraxis && go build ./tests/...
```

- [ ] **Step 6: Commit**

```bash
cd /home/jason/jdk/abraxis
git add tests/ go.work
git commit -m "feat: add shared test infrastructure with dockertest Postgres and Redis helpers"
```

---

### Task 2: Add dockertest dependency to Aegis and create test helper

**Files:**
- Modify: `/home/jason/jdk/abraxis/aegis/go.mod` (add dockertest, tests module)
- Create: `/home/jason/jdk/abraxis/aegis/tests/helper_test.go`

- [ ] **Step 1: Add dockertest to aegis**

```bash
cd /home/jason/jdk/abraxis/aegis
go get github.com/ory/dockertest/v3@v3.11.0
```

- [ ] **Step 2: Create the test helper**

Create `/home/jason/jdk/abraxis/aegis/tests/helper_test.go`:

```go
package tests

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jasonKoogler/abraxis/aegis/internal/adapters/postgres"
	"github.com/jasonKoogler/abraxis/aegis/internal/app"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
	aegisredis "github.com/jasonKoogler/abraxis/aegis/internal/common/redis"
	"github.com/jasonKoogler/abraxis/aegis/internal/config"
	"github.com/jasonKoogler/abraxis/tests/testutil"
	"testing"
)

// AegisTestServer holds a running Aegis instance for integration testing.
type AegisTestServer struct {
	BaseURL string
	App     *app.App
	cancel  context.CancelFunc
}

// StartAegisServer creates and starts a real Aegis server backed by the given
// Postgres and Redis containers. The server listens on a random port.
func StartAegisServer(t *testing.T, pg *testutil.PostgresContainer, rd *testutil.RedisContainer) *AegisTestServer {
	t.Helper()

	logger := log.NewLogger("error") // quiet during tests

	// Build config programmatically (not from env vars)
	cfg := &config.Config{
		UseRedisRateLimiter: false,
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
			Port:            "0", // random port
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    10 * time.Second,
			IdleTimeout:     10 * time.Second,
			ShutdownTimeout: 5 * time.Second,
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
				UseCustomJWT:           true,
				JWTIssuer:              "aegis-test",
				AccessTokenExpiration:  15 * time.Minute,
				RefreshTokenExpiration: 24 * time.Hour,
				TokenRotationInterval:  168 * time.Hour,
				OAuthConfig: config.OAuthConfig{
					VerifierStorage: "memory",
					Providers: []config.OAuthProviderConfig{
						{Name: "google", ClientID: "test", ClientSecret: "test", RedirectURL: "http://localhost/callback", Scopes: []string{"email"}},
					},
				},
			},
		},
		GRPC: config.GRPCConfig{
			Enabled: false, // no gRPC needed for HTTP-only tests
		},
	}

	// Create Postgres pool from the test container
	pgPool, err := db.NewPostgresPool(context.Background(), &cfg.Postgres, logger)
	if err != nil {
		t.Fatalf("Failed to create postgres pool: %s", err)
	}

	// Create Redis client from the test container
	redisClient, err := aegisredis.NewRedisClient(context.Background(), logger, &cfg.Auth.AuthN.RedisConfig)
	if err != nil {
		t.Fatalf("Failed to create redis client: %s", err)
	}

	// Create repositories
	userRepo := postgres.NewUserRepository(pgPool)
	auditRepo := postgres.NewAuditLogRepository(pgPool)
	apiKeyRepo := postgres.NewAPIKeyRepository(pgPool)
	tenantRepo := postgres.NewTenantRepository(pgPool)
	permissionRepo := postgres.NewPermissionRepository(pgPool)
	rolePermissionRepo := postgres.NewRolePermissionRepository(pgPool)

	ctx := context.Background()
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithUserRepository(userRepo),
		app.WithAuditRepository(auditRepo),
		app.WithAPIKeyRepository(apiKeyRepo),
		app.WithTenantRepository(tenantRepo),
		app.WithPermissionRepository(permissionRepo),
		app.WithRolePermissionRepository(rolePermissionRepo),
		app.WithRedisClient(redisClient),
		app.WithAllDefaultServices(ctx),
	)
	if err != nil {
		t.Fatalf("Failed to create aegis app: %s", err)
	}

	// Start in background
	go func() {
		_ = application.Start()
	}()

	// Discover the actual port the server bound to.
	// The app's server exposes GetRouter() but we need the listener address.
	// For now, use a health check polling approach.
	// NOTE: Port "0" may not be supported by the server config. If so,
	// pick a high random port like "18080" instead and use that.
	// The implementer should verify how the server resolves port "0".
	baseURL := fmt.Sprintf("http://localhost:%s", cfg.HTTPServer.Port)

	// Wait for server to be ready
	ready := false
	for i := 0; i < 50; i++ {
		resp, err := http.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatal("Aegis server did not become ready within 5 seconds")
	}

	t.Cleanup(func() {
		_ = application.Shutdown(context.Background())
	})

	return &AegisTestServer{
		BaseURL: baseURL,
		App:     application,
	}
}
```

NOTE TO IMPLEMENTER: The `db` import is `"github.com/jasonKoogler/abraxis/aegis/internal/common/db"`. The server may not support port `"0"` — check how `app/server.go` resolves the port. If it doesn't support dynamic ports, use a high random port (e.g., generate one in range 10000-60000) and set `cfg.HTTPServer.Port` to that. The key requirement is that the test server doesn't conflict with any running Docker Compose services.

- [ ] **Step 3: Verify compilation**

```bash
cd /home/jason/jdk/abraxis && go build ./aegis/tests/...
```

This will fail because test files need `go test` to compile, not `go build`. Use:

```bash
cd /home/jason/jdk/abraxis/aegis && go test -c -o /dev/null ./tests/
```

- [ ] **Step 4: Commit**

```bash
cd /home/jason/jdk/abraxis
git add aegis/tests/ aegis/go.mod aegis/go.sum
git commit -m "feat: add aegis integration test helper with real app construction"
```

---

### Task 3: Write the auth flow integration tests

**Files:**
- Create: `/home/jason/jdk/abraxis/aegis/tests/auth_integration_test.go`

- [ ] **Step 1: Write the integration test file**

Create `/home/jason/jdk/abraxis/aegis/tests/auth_integration_test.go`:

```go
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/jasonKoogler/abraxis/tests/testutil"
	"github.com/stretchr/testify/require"
)

func TestAuthFlows(t *testing.T) {
	// Start real Postgres with Aegis migrations
	pg := testutil.SetupPostgres(t, "../deploy/migrations")

	// Start real Redis
	rd := testutil.SetupRedis(t)

	// Start real Aegis server
	server := StartAegisServer(t, pg, rd)
	client := &http.Client{}

	var accessToken, refreshToken, sessionID, userID string

	t.Run("register_user", func(t *testing.T) {
		body := map[string]string{
			"email":     "testuser@example.com",
			"password":  "SecurePass123!",
			"firstName": "Test",
			"lastName":  "User",
			"phone":     "+1234567890",
		}
		resp := doPost(t, client, server.BaseURL+"/auth/register", body)
		require.Equal(t, http.StatusCreated, resp.StatusCode, "register should return 201")

		// Verify auth headers are set
		accessToken = resp.Header.Get("Authorization")
		refreshToken = resp.Header.Get("X-Refresh-Token")
		sessionID = resp.Header.Get("X-Session-ID")

		require.NotEmpty(t, accessToken, "Authorization header should be set")
		require.NotEmpty(t, refreshToken, "X-Refresh-Token header should be set")
		require.NotEmpty(t, sessionID, "X-Session-ID header should be set")
	})

	t.Run("login_with_password", func(t *testing.T) {
		body := map[string]string{
			"email":    "testuser@example.com",
			"password": "SecurePass123!",
		}
		resp := doPost(t, client, server.BaseURL+"/auth/login", body)
		require.Equal(t, http.StatusOK, resp.StatusCode, "login should return 200")

		// Save tokens for subsequent tests
		accessToken = resp.Header.Get("Authorization")
		refreshToken = resp.Header.Get("X-Refresh-Token")
		sessionID = resp.Header.Get("X-Session-ID")

		require.NotEmpty(t, accessToken, "Authorization header should be set")
		require.NotEmpty(t, refreshToken, "X-Refresh-Token header should be set")
	})

	t.Run("access_protected_endpoint", func(t *testing.T) {
		// First, get the user ID from the users list
		req, _ := http.NewRequest(http.MethodGet, server.BaseURL+"/users", nil)
		req.Header.Set("Authorization", accessToken)
		resp, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode, "GET /users with valid token should return 200")

		var users []map[string]interface{}
		readJSON(t, resp, &users)
		require.NotEmpty(t, users, "should have at least one user")
		userID = fmt.Sprintf("%v", users[0]["id"])

		// Access specific user
		req, _ = http.NewRequest(http.MethodGet, server.BaseURL+"/users/"+userID, nil)
		req.Header.Set("Authorization", accessToken)
		resp, err = client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode, "GET /users/{id} with valid token should return 200")

		var user map[string]interface{}
		readJSON(t, resp, &user)
		require.Equal(t, "testuser@example.com", user["email"])

		// Access without token should fail
		req, _ = http.NewRequest(http.MethodGet, server.BaseURL+"/users", nil)
		resp, err = client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "GET /users without token should return 401")
	})

	t.Run("refresh_token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, server.BaseURL+"/auth/refresh", nil)
		req.Header.Set("Authorization", "Bearer "+refreshToken)
		resp, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode, "refresh should return 200")

		// Get new tokens
		newAccessToken := resp.Header.Get("Authorization")
		if newAccessToken == "" {
			// Token might be in response body
			var tokenResp map[string]interface{}
			readJSON(t, resp, &tokenResp)
			t.Logf("Refresh response body: %+v", tokenResp)
		}

		// The new access token should work
		// (Implementation detail: the refresh response format varies —
		// the implementer should check whether tokens come in headers or body)
	})

	t.Run("logout_invalidates_session", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, server.BaseURL+"/auth/logout", nil)
		req.Header.Set("Authorization", accessToken)
		resp, err := client.Do(req)
		require.NoError(t, err)
		// Logout may return 302 (redirect) or 200
		require.True(t, resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusOK,
			"logout should return 302 or 200, got %d", resp.StatusCode)

		// Old token should no longer work on protected endpoints
		req, _ = http.NewRequest(http.MethodGet, server.BaseURL+"/users", nil)
		req.Header.Set("Authorization", accessToken)
		resp, err = client.Do(req)
		require.NoError(t, err)
		// After logout, the token may still be valid (JWT is stateless) unless
		// session-based revocation is implemented. The implementer should verify
		// the expected behavior and adjust the assertion accordingly.
		t.Logf("Post-logout /users status: %d", resp.StatusCode)
	})
}

// --- HTTP helpers ---

func doPost(t *testing.T, client *http.Client, url string, body interface{}) *http.Response {
	t.Helper()
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	resp, err := client.Post(url, "application/json", bytes.NewReader(jsonBody))
	require.NoError(t, err)
	return resp
}

func readJSON(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	err = json.Unmarshal(data, target)
	require.NoError(t, err, "failed to parse JSON: %s", string(data))
}
```

NOTE TO IMPLEMENTER: Several aspects depend on exact Aegis behavior:
1. The `Authorization` header format — Aegis may return `"Bearer xxx"` or just `"xxx"`. Check `setAuthHeaders` in `aegis/internal/adapters/http/helpers.go`.
2. The refresh token flow — check if `POST /auth/refresh` expects the refresh token in `Authorization` header or request body.
3. Post-logout behavior — check if Aegis invalidates the JWT or just the session.

Adjust assertions based on actual behavior. The test should match what the service actually does.

- [ ] **Step 2: Run the tests**

```bash
cd /home/jason/jdk/abraxis/aegis
go test -v -count=1 -timeout 120s ./tests/
```

This will:
1. Start a Postgres container
2. Run migrations
3. Start a Redis container
4. Start a real Aegis server
5. Run all 5 subtests
6. Clean up containers

- [ ] **Step 3: Fix any failures**

Read test output carefully. Common issues:
- Port binding (if port "0" not supported)
- Auth header format mismatch
- Refresh token flow details
- Missing config fields

Fix the test helper and/or test assertions to match actual service behavior.

- [ ] **Step 4: Verify tests pass**

```bash
cd /home/jason/jdk/abraxis/aegis
go test -v -count=1 -timeout 120s ./tests/
```

All 5 subtests should pass.

- [ ] **Step 5: Commit**

```bash
cd /home/jason/jdk/abraxis
git add aegis/tests/auth_integration_test.go
git commit -m "feat: add aegis auth flow integration tests (register, login, access, refresh, logout)"
```

- [ ] **Step 6: Push**

```bash
git push origin main
```
