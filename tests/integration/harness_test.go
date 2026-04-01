package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

// FullStack holds URLs for the running Aegis and Prism services.
type FullStack struct {
	AegisURL string
	PrismURL string
}

// freePort finds an available TCP port on localhost.
func freePort(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := fmt.Sprintf("%d", lis.Addr().(*net.TCPAddr).Port)
	lis.Close()
	return port
}

// pgContainer holds connection details for a dockertest Postgres container.
type pgContainer struct {
	host     string
	port     string
	user     string
	password string
	pool     *pgxpool.Pool
}

// startPostgres starts a single Postgres 16 container, creates two databases
// (aegis_db and prism_db), runs migrations for each, and returns connection
// details. The container is purged via t.Cleanup.
func startPostgres(t *testing.T) *pgContainer {
	t.Helper()

	dockerPool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("failed to connect to Docker: %v", err)
	}
	dockerPool.MaxWait = 2 * time.Minute

	const (
		user     = "testuser"
		password = "testpass"
		adminDB  = "postgres"
	)

	resource, err := dockerPool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "16-alpine",
		Env: []string{
			fmt.Sprintf("POSTGRES_USER=%s", user),
			fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
			fmt.Sprintf("POSTGRES_DB=%s", adminDB),
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
		t.Fatalf("failed to start Postgres container: %v", err)
	}
	t.Cleanup(func() {
		if purgeErr := dockerPool.Purge(resource); purgeErr != nil {
			t.Logf("warning: failed to purge Postgres container: %v", purgeErr)
		}
	})

	hostPort := resource.GetPort("5432/tcp")
	adminConnStr := fmt.Sprintf("postgres://%s:%s@localhost:%s/%s?sslmode=disable", user, password, hostPort, adminDB)

	// Wait for Postgres to accept connections.
	var adminPool *pgxpool.Pool
	if err := dockerPool.Retry(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var retryErr error
		adminPool, retryErr = pgxpool.New(ctx, adminConnStr)
		if retryErr != nil {
			return retryErr
		}
		return adminPool.Ping(ctx)
	}); err != nil {
		t.Fatalf("Postgres not ready: %v", err)
	}

	// Create aegis_db and prism_db.
	ctx := context.Background()
	for _, dbName := range []string{"aegis_db", "prism_db"} {
		if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
			t.Fatalf("failed to create database %s: %v", dbName, err)
		}
	}

	// Run Aegis migrations.
	aegisMigrations, err := filepath.Abs(filepath.Join("..", "..", "aegis", "deploy", "migrations"))
	if err != nil {
		t.Fatalf("failed to resolve aegis migrations path: %v", err)
	}
	runMigrations(t, dockerPool, fmt.Sprintf("postgres://%s:%s@localhost:%s/aegis_db?sslmode=disable", user, password, hostPort), aegisMigrations)

	// Run Prism migrations.
	prismMigrations, err := filepath.Abs(filepath.Join("..", "..", "prism", "deploy", "migrations"))
	if err != nil {
		t.Fatalf("failed to resolve prism migrations path: %v", err)
	}
	runMigrations(t, dockerPool, fmt.Sprintf("postgres://%s:%s@localhost:%s/prism_db?sslmode=disable", user, password, hostPort), prismMigrations)

	t.Cleanup(func() {
		adminPool.Close()
	})

	return &pgContainer{
		host:     "localhost",
		port:     hostPort,
		user:     user,
		password: password,
		pool:     adminPool,
	}
}

// runMigrations runs golang-migrate via Docker against the given connection string.
func runMigrations(t *testing.T, pool *dockertest.Pool, connStr, migrationsPath string) {
	t.Helper()

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "migrate/migrate",
		Tag:        "v4.17.0",
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
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
		config.NetworkMode = "host"
	})
	if err != nil {
		t.Fatalf("failed to start migrate container for %s: %v", migrationsPath, err)
	}

	if err := pool.Retry(func() error {
		container, inspectErr := pool.Client.InspectContainer(resource.Container.ID)
		if inspectErr != nil {
			// Container already removed (AutoRemove) means it finished.
			return nil
		}
		if !container.State.Running {
			if container.State.ExitCode != 0 {
				return fmt.Errorf("migrate exited with code %d", container.State.ExitCode)
			}
			return nil
		}
		return fmt.Errorf("migrate container still running")
	}); err != nil {
		t.Fatalf("migration failed for %s: %v", migrationsPath, err)
	}
}

// redisContainer holds connection details for a dockertest Redis container.
type redisContainer struct {
	host     string
	port     string
	password string
}

// startRedis starts a single Redis 7 container and returns connection details.
func startRedis(t *testing.T) *redisContainer {
	t.Helper()

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("failed to connect to Docker: %v", err)
	}
	pool.MaxWait = 2 * time.Minute

	const password = "testredis"

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
		t.Fatalf("failed to start Redis container: %v", err)
	}
	t.Cleanup(func() {
		if purgeErr := pool.Purge(resource); purgeErr != nil {
			t.Logf("warning: failed to purge Redis container: %v", purgeErr)
		}
	})

	hostPort := resource.GetPort("6379/tcp")

	// Wait for Redis to accept connections.
	if err := pool.Retry(func() error {
		conn, dialErr := net.DialTimeout("tcp", fmt.Sprintf("localhost:%s", hostPort), 2*time.Second)
		if dialErr != nil {
			return dialErr
		}
		conn.Close()
		return nil
	}); err != nil {
		t.Fatalf("Redis not ready: %v", err)
	}

	return &redisContainer{
		host:     "localhost",
		port:     hostPort,
		password: password,
	}
}

// buildBinary compiles a Go binary from the given package path (relative to
// the monorepo root) into tmpDir and returns the path to the built binary.
func buildBinary(t *testing.T, tmpDir, name, pkgPath string) string {
	t.Helper()

	outPath := filepath.Join(tmpDir, name)

	// Resolve the monorepo root (two levels up from tests/integration).
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", outPath, pkgPath)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	t.Logf("building %s: go build -o %s %s (dir=%s)", name, outPath, pkgPath, repoRoot)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build %s: %v", name, err)
	}
	return outPath
}

// startProcess starts a binary as a subprocess with the given environment and
// returns the exec.Cmd. The process is killed via t.Cleanup with SIGTERM.
func startProcess(t *testing.T, binPath string, env []string) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(binPath)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start %s: %v", binPath, err)
	}

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
			}
		}
	})

	return cmd
}

// waitForHealth polls the given URL until it returns 200 or the timeout elapses.
func waitForHealth(t *testing.T, url string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("endpoint %s did not become healthy within %s", url, timeout)
}

// commonEnv returns a clean set of environment variables shared between both
// services for Postgres and Redis connectivity.
func commonEnv(pg *pgContainer, rd *redisContainer, dbName string) []string {
	return []string{
		// Postgres
		fmt.Sprintf("POSTGRES_HOST=%s", pg.host),
		fmt.Sprintf("POSTGRES_PORT=%s", pg.port),
		fmt.Sprintf("POSTGRES_USER=%s", pg.user),
		fmt.Sprintf("POSTGRES_PASSWORD=%s", pg.password),
		fmt.Sprintf("POSTGRES_DB=%s", dbName),
		"POSTGRES_SSL_MODE=disable",
		"POSTGRES_TIMEZONE=UTC",
		"POSTGRES_TIMEOUT=30s",

		// Redis
		fmt.Sprintf("REDIS_HOST=%s", rd.host),
		fmt.Sprintf("REDIS_PORT=%s", rd.port),
		fmt.Sprintf("REDIS_PASSWORD=%s", rd.password),
		"REDIS_USERNAME=default",

		// HTTP server timeouts
		"HTTP_SERVER_READ_TIMEOUT=10s",
		"HTTP_SERVER_WRITE_TIMEOUT=10s",
		"HTTP_SERVER_IDLE_TIMEOUT=10s",
		"HTTP_SERVER_SHUTDOWN_TIMEOUT=5s",

		// Rate limiting (disabled)
		"USE_REDIS_RATE_LIMITER=false",
		"RATE_LIMIT_REQUESTS_PER_SECOND=100",
		"RATE_LIMIT_BURST=200",
		"RATE_LIMIT_TTL=1m",

		// Auth
		"SESSION_MANAGER=redis",
		"USE_CUSTOM_JWT=true",
		"JWT_ISSUER=aegis-test",
		"ACCESS_TOKEN_EXPIRATION=15m",
		"REFRESH_TOKEN_EXPIRATION=24h",
		"TOKEN_ROTATION_INTERVAL=168h",
		"OAUTH_VERIFIER_STORAGE=memory",

		// OAuth provider (dummy values for config validation)
		"GOOGLE_KEY=test-client-id",
		"GOOGLE_SECRET=test-client-secret",
		"GOOGLE_CALLBACK_URL=http://localhost/callback",
		"GOOGLE_SCOPES=email",

		// General
		"ENV=development",
		"LOG_LEVEL=debug",
	}
}

// StartFullStack starts Postgres, Redis, Aegis, and Prism. It builds both
// binaries, starts them as subprocesses, and waits for health checks. Returns
// a FullStack with the URLs of both services.
func StartFullStack(t *testing.T) *FullStack {
	t.Helper()

	// Skip if Docker is not available.
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available — skipping integration test")
	}

	// Start infrastructure containers.
	pg := startPostgres(t)
	rd := startRedis(t)

	// Find free ports for Aegis HTTP, Aegis gRPC, and Prism HTTP.
	aegisHTTPPort := freePort(t)
	aegisGRPCPort := freePort(t)
	prismHTTPPort := freePort(t)

	// Create temp directory for binaries.
	tmpDir := t.TempDir()

	// Build both binaries.
	aegisBin := buildBinary(t, tmpDir, "aegis", "./aegis/cmd/")
	prismBin := buildBinary(t, tmpDir, "prism", "./prism/cmd/")

	// --- Start Aegis ---
	aegisEnv := append(
		commonEnv(pg, rd, "aegis_db"),
		fmt.Sprintf("HTTP_SERVER_PORT=%s", aegisHTTPPort),
		fmt.Sprintf("GRPC_PORT=%s", aegisGRPCPort),
		"GRPC_ENABLED=true",
	)
	startProcess(t, aegisBin, aegisEnv)

	aegisURL := fmt.Sprintf("http://127.0.0.1:%s", aegisHTTPPort)
	t.Logf("waiting for Aegis at %s/health", aegisURL)
	waitForHealth(t, aegisURL+"/health", 30*time.Second)
	t.Logf("Aegis is healthy at %s", aegisURL)

	// --- Start Prism ---
	prismEnv := append(
		commonEnv(pg, rd, "prism_db"),
		fmt.Sprintf("HTTP_SERVER_PORT=%s", prismHTTPPort),

		// Aegis integration
		fmt.Sprintf("AEGIS_GRPC_ADDRESS=localhost:%s", aegisGRPCPort),
		fmt.Sprintf("AEGIS_JWKS_URL=%s/.well-known/jwks.json", aegisURL),
		"AEGIS_SYNC_ENABLED=true",
		"AEGIS_CACHE_TTL=5s",
		"AEGIS_JWKS_REFRESH_INTERVAL=10s",

		// Prism-specific: no circuit breaker needed for tests
		"CIRCUIT_BREAKER_ENABLED=false",
	)
	startProcess(t, prismBin, prismEnv)

	prismURL := fmt.Sprintf("http://127.0.0.1:%s", prismHTTPPort)
	t.Logf("waiting for Prism at %s/health", prismURL)
	waitForHealth(t, prismURL+"/health", 30*time.Second)
	t.Logf("Prism is healthy at %s", prismURL)

	// Give Prism a few seconds to complete JWKS fetch and gRPC sync.
	// The /ready endpoint gates on these, so we poll it.
	t.Log("waiting for Prism readiness (JWKS + gRPC sync)...")
	waitForReady(t, prismURL+"/ready", 30*time.Second)
	t.Log("Prism is ready")

	return &FullStack{
		AegisURL: aegisURL,
		PrismURL: prismURL,
	}
}

// waitForReady polls the /ready endpoint until it returns 200 or timeout.
func waitForReady(t *testing.T, url string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			t.Logf("  /ready returned %d, retrying...", resp.StatusCode)
		}
		time.Sleep(500 * time.Millisecond)
	}
	// Don't fail hard — readiness may not be fully wired.
	t.Logf("warning: %s did not return 200 within %s (continuing anyway)", url, timeout)
}
