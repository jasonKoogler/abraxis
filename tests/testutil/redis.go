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

// RedisContainer holds connection details and a client for a dockertest Redis instance.
type RedisContainer struct {
	Client   *redis.Client
	Host     string
	Port     string
	Password string
}

// SetupRedis starts a redis:7-alpine container with password authentication,
// waits for readiness, and returns a RedisContainer. The container is
// automatically purged via t.Cleanup.
func SetupRedis(t *testing.T) *RedisContainer {
	t.Helper()
	skipIfNoDocker(t)

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
		if err := pool.Purge(resource); err != nil {
			t.Logf("warning: failed to purge Redis container: %v", err)
		}
	})

	hostPort := resource.GetPort("6379/tcp")

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("localhost:%s", hostPort),
		Password: password,
	})

	t.Cleanup(func() {
		_ = client.Close()
	})

	// Wait for Redis to accept connections.
	if err := pool.Retry(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return client.Ping(ctx).Err()
	}); err != nil {
		t.Fatalf("Redis container not ready: %v", err)
	}

	return &RedisContainer{
		Client:   client,
		Host:     "localhost",
		Port:     hostPort,
		Password: password,
	}
}
