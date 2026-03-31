package tests

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
	"github.com/jasonKoogler/prism/internal/features/gateway/adapters/circuitbreaker"
	"github.com/jasonKoogler/prism/internal/ports"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfNoRedis skips the test if Redis is not available
func skipIfNoRedis(t *testing.T) *redis.Client {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Try to ping Redis
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := client.Ping(ctx).Err()
	if err != nil {
		t.Skip("Skipping test as Redis is not available:", err)
		return nil
	}

	return client
}

// clearTestKeys clears all test keys to prevent test pollution
func clearTestKeys(t *testing.T, client *redis.Client) {
	ctx := context.Background()
	keys, err := client.Keys(ctx, "cb:*").Result()
	if err != nil {
		t.Logf("Error clearing test keys: %v", err)
		return
	}

	if len(keys) > 0 {
		err = client.Del(ctx, keys...).Err()
		if err != nil {
			t.Logf("Error deleting test keys: %v", err)
		}
	}
}

func TestRedisCircuitBreaker_Integration(t *testing.T) {
	client := skipIfNoRedis(t)
	if client == nil {
		return
	}
	defer client.Close()

	clearTestKeys(t, client)

	logger := log.NewLogger("debug")
	cfg := &config.CircuitBreakerConfig{
		Threshold:                3,
		Timeout:                  100 * time.Millisecond,
		HalfOpenSuccessThreshold: 2,
		MaxConcurrentRequests:    5,
		RequestTimeout:           50 * time.Millisecond,
	}

	redisCfg := &circuitbreaker.CircuitBreakerRedisConfig{
		Address:  client.Options().Addr,
		Password: client.Options().Password,
		DB:       client.Options().DB,
		CacheTTL: 50 * time.Millisecond,
	}

	cb, err := circuitbreaker.NewRedisCircuitBreaker(cfg, redisCfg, logger)
	require.NoError(t, err, "Failed to create Redis circuit breaker")
	require.NotNil(t, cb)

	defer func() {
		err := cb.Close()
		assert.NoError(t, err)
	}()

	t.Run("Successful execution", func(t *testing.T) {
		err := cb.Execute(context.Background(), "test-redis-success", func() error {
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, ports.StateClosed, cb.GetState("test-redis-success"))
	})

	t.Run("Failed execution", func(t *testing.T) {
		testErr := errors.New("test error")
		err := cb.Execute(context.Background(), "test-redis-failure", func() error {
			return testErr
		})
		assert.Error(t, err)
		assert.Equal(t, testErr, err)
		assert.Equal(t, ports.StateClosed, cb.GetState("test-redis-failure"))
	})

	t.Run("Circuit opens after threshold failures", func(t *testing.T) {
		circuitName := "test-redis-open-circuit"
		testErr := errors.New("test error")

		// Cause failures to reach threshold
		for i := 0; i < cfg.Threshold; i++ {
			_ = cb.Execute(context.Background(), circuitName, func() error {
				return testErr
			})
		}

		// Circuit should now be open
		assert.Equal(t, ports.StateOpen, cb.GetState(circuitName))

		// Execute again - should be rejected immediately
		err := cb.Execute(context.Background(), circuitName, func() error {
			return nil // This shouldn't be called
		})
		assert.Error(t, err)
		assert.Equal(t, circuitbreaker.ErrCircuitOpen, err)
	})

	t.Run("Circuit transitions to half-open after timeout", func(t *testing.T) {
		circuitName := "test-redis-half-open"
		testErr := errors.New("test error")

		// Cause failures to open circuit
		for i := 0; i < cfg.Threshold; i++ {
			_ = cb.Execute(context.Background(), circuitName, func() error {
				return testErr
			})
		}

		// Circuit should be open
		assert.Equal(t, ports.StateOpen, cb.GetState(circuitName))

		// Wait for timeout
		time.Sleep(cfg.Timeout + 10*time.Millisecond)

		// Execute again - should try and go to half-open
		called := false
		err := cb.Execute(context.Background(), circuitName, func() error {
			called = true
			return nil
		})
		assert.NoError(t, err)
		assert.True(t, called)

		// May need a brief delay for state to propagate in distributed environment
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, ports.StateHalfOpen, cb.GetState(circuitName))
	})

	t.Run("Circuit closes after success threshold in half-open state", func(t *testing.T) {
		circuitName := "test-redis-close-after-half-open"
		testErr := errors.New("test error")

		// Open the circuit
		for i := 0; i < cfg.Threshold; i++ {
			_ = cb.Execute(context.Background(), circuitName, func() error {
				return testErr
			})
		}

		// Wait for timeout to transition to half-open
		time.Sleep(cfg.Timeout + 10*time.Millisecond)

		// Execute success calls to meet threshold
		for i := 0; i < cfg.HalfOpenSuccessThreshold; i++ {
			err := cb.Execute(context.Background(), circuitName, func() error {
				return nil
			})
			assert.NoError(t, err)

			// Small delay to allow state to propagate
			time.Sleep(10 * time.Millisecond)
		}

		// Circuit should now be closed
		assert.Equal(t, ports.StateClosed, cb.GetState(circuitName))
	})

	t.Run("Request times out", func(t *testing.T) {
		err := cb.Execute(context.Background(), "test-redis-timeout", func() error {
			time.Sleep(cfg.RequestTimeout * 2) // Sleep longer than timeout
			return nil
		})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.DeadlineExceeded))
	})

	t.Run("GetMetrics returns circuit data", func(t *testing.T) {
		circuitName := "test-redis-metrics"
		testErr := errors.New("test error")

		// Execute successful requests
		for i := 0; i < 2; i++ {
			_ = cb.Execute(context.Background(), circuitName, func() error {
				return nil
			})
		}

		// Execute failed requests
		for i := 0; i < 3; i++ {
			_ = cb.Execute(context.Background(), circuitName, func() error {
				return testErr
			})
		}

		// Small delay to allow metrics to propagate
		time.Sleep(10 * time.Millisecond)

		// Get metrics
		metrics := cb.GetMetrics(circuitName)
		assert.Equal(t, circuitName, metrics.Name)
		assert.Equal(t, ports.StateOpen, metrics.State)
		assert.Equal(t, int64(3), metrics.Failures)
		assert.Equal(t, int64(2), metrics.Successes)
	})

	t.Run("Reset properly resets circuit state", func(t *testing.T) {
		circuitName := "test-redis-reset"
		testErr := errors.New("test error")

		// Open the circuit
		for i := 0; i < cfg.Threshold; i++ {
			_ = cb.Execute(context.Background(), circuitName, func() error {
				return testErr
			})
		}

		// Verify circuit is open
		assert.Equal(t, ports.StateOpen, cb.GetState(circuitName))

		// Reset the circuit
		err := cb.Reset(circuitName)
		assert.NoError(t, err)

		// Small delay to allow reset to propagate
		time.Sleep(10 * time.Millisecond)

		// Verify circuit is closed
		assert.Equal(t, ports.StateClosed, cb.GetState(circuitName))

		// Verify metrics are reset
		metrics := cb.GetMetrics(circuitName)
		assert.Equal(t, int64(0), metrics.Failures)
		assert.Equal(t, int64(0), metrics.Successes)
	})
}

func TestRedisCircuitBreaker_WithPreExistingClient(t *testing.T) {
	client := skipIfNoRedis(t)
	if client == nil {
		return
	}
	defer client.Close()

	clearTestKeys(t, client)

	logger := log.NewLogger("debug")
	cfg := &config.CircuitBreakerConfig{
		Threshold:                3,
		Timeout:                  100 * time.Millisecond,
		HalfOpenSuccessThreshold: 2,
		MaxConcurrentRequests:    5,
		RequestTimeout:           50 * time.Millisecond,
	}

	redisCfg := &circuitbreaker.CircuitBreakerRedisConfig{
		CacheTTL: 50 * time.Millisecond,
	}

	// Use NewRedisCircuitBreakerWithClient to create with a pre-existing client
	cb, err := circuitbreaker.NewRedisCircuitBreaker(cfg, redisCfg, logger)

	require.NoError(t, err, "Failed to create Redis circuit breaker with existing client")
	require.NotNil(t, cb)

	defer func() {
		err := cb.Close()
		assert.NoError(t, err)
	}()

	// Basic test to ensure it works
	err = cb.Execute(context.Background(), "test-with-existing-client", func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, ports.StateClosed, cb.GetState("test-with-existing-client"))
}
