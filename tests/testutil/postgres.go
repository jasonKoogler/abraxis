// Package testutil provides shared Docker container helpers for integration tests.
package testutil

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

// PostgresContainer holds connection details and a pool for a dockertest Postgres instance.
type PostgresContainer struct {
	Pool     *pgxpool.Pool
	Host     string
	Port     string
	User     string
	Password string
	DB       string
	ConnStr  string
}

// SetupPostgres starts a postgres:16-alpine container, waits for readiness,
// runs migrations from migrationsPath (if non-empty), and returns a
// PostgresContainer. The container is automatically purged via t.Cleanup.
//
// migrationsPath should be an absolute path to a directory of migrate-compatible
// SQL files (e.g., "000001_init.up.sql"). Pass "" to skip migrations.
func SetupPostgres(t *testing.T, migrationsPath string) *PostgresContainer {
	t.Helper()
	skipIfNoDocker(t)

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("failed to connect to Docker: %v", err)
	}
	pool.MaxWait = 2 * time.Minute

	const (
		user     = "testuser"
		password = "testpass"
		dbName   = "testdb"
	)

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
		t.Fatalf("failed to start Postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := pool.Purge(resource); err != nil {
			t.Logf("warning: failed to purge Postgres container: %v", err)
		}
	})

	hostPort := resource.GetPort("5432/tcp")
	connStr := fmt.Sprintf("postgres://%s:%s@localhost:%s/%s?sslmode=disable", user, password, hostPort, dbName)

	// Wait for Postgres to accept connections.
	var pgPool *pgxpool.Pool
	if err := pool.Retry(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var retryErr error
		pgPool, retryErr = pgxpool.New(ctx, connStr)
		if retryErr != nil {
			return retryErr
		}
		return pgPool.Ping(ctx)
	}); err != nil {
		t.Fatalf("Postgres container not ready: %v", err)
	}

	t.Cleanup(func() {
		pgPool.Close()
	})

	// Run migrations if a path was provided.
	if migrationsPath != "" {
		runMigrations(t, pool, connStr, migrationsPath)
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

// runMigrations executes database migrations using the migrate/migrate Docker
// image. It uses host networking so the migrate container can reach Postgres on
// localhost:<hostPort>.
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
		t.Fatalf("failed to start migrate container: %v", err)
	}

	// Wait for the migrate container to finish.
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
		t.Fatalf("migration failed: %v", err)
	}
}

// skipIfNoDocker skips the test when Docker is not available.
func skipIfNoDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available — skipping test")
	}
}
