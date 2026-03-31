package tests

import (
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

func TestCircuitBreakerBuilder(t *testing.T) {
	logger := log.NewLogger("debug")
	cfg := &config.CircuitBreakerConfig{
		Threshold:                5,
		Timeout:                  10 * time.Second,
		HalfOpenSuccessThreshold: 2,
		MaxConcurrentRequests:    100,
		RequestTimeout:           2 * time.Second,
		Provider:                 "memory",
	}

	t.Run("Builder with valid config", func(t *testing.T) {
		builder := circuitbreaker.NewBuilder()
		require.NotNil(t, builder)

		// Apply valid options
		err := circuitbreaker.WithConfig(cfg)(builder)
		require.NoError(t, err)

		err = circuitbreaker.WithLogger(logger)(builder)
		require.NoError(t, err)

		// Build should succeed
		cb, err := builder.Build()
		require.NoError(t, err)
		require.NotNil(t, cb)

		// Should be a MemoryCircuitBreaker by default
		state := cb.GetState("test")
		assert.Equal(t, ports.StateClosed, state)
	})

	t.Run("Builder with nil config", func(t *testing.T) {
		builder := circuitbreaker.NewBuilder()
		require.NotNil(t, builder)

		// Apply nil config
		err := circuitbreaker.WithConfig(nil)(builder)
		assert.Error(t, err)
		assert.Equal(t, circuitbreaker.ErrNilConfig, err)

		// Build should fail
		cb, err := builder.Build()
		assert.Error(t, err)
		assert.Equal(t, circuitbreaker.ErrNilConfig, err)
		assert.Nil(t, cb)
	})

	t.Run("Builder with nil logger", func(t *testing.T) {
		builder := circuitbreaker.NewBuilder()
		require.NotNil(t, builder)

		// Apply config
		err := circuitbreaker.WithConfig(cfg)(builder)
		require.NoError(t, err)

		// Apply nil logger
		err = circuitbreaker.WithLogger(nil)(builder)
		assert.Error(t, err)
		assert.Equal(t, circuitbreaker.ErrNilLogger, err)

		// Build should fail
		cb, err := builder.Build()
		assert.Error(t, err)
		assert.Equal(t, circuitbreaker.ErrNilLogger, err)
		assert.Nil(t, cb)
	})

	t.Run("Redis provider without Redis config", func(t *testing.T) {
		redisCfg := &config.CircuitBreakerConfig{
			Threshold:                5,
			Timeout:                  10 * time.Second,
			HalfOpenSuccessThreshold: 2,
			MaxConcurrentRequests:    100,
			RequestTimeout:           2 * time.Second,
			Provider:                 "redis",
			// Redis config missing
		}

		builder := circuitbreaker.NewBuilder()
		require.NotNil(t, builder)

		// Apply config with Redis provider but no Redis config
		err := circuitbreaker.WithConfig(redisCfg)(builder)
		require.NoError(t, err)

		err = circuitbreaker.WithLogger(logger)(builder)
		require.NoError(t, err)

		// Build should fail
		cb, err := builder.Build()
		assert.Error(t, err)
		assert.Equal(t, circuitbreaker.ErrNilRedisConfig, err)
		assert.Nil(t, cb)
	})

	t.Run("Builder with Redis config", func(t *testing.T) {
		redisCfg := &config.CircuitBreakerConfig{
			Threshold:                5,
			Timeout:                  10 * time.Second,
			HalfOpenSuccessThreshold: 2,
			MaxConcurrentRequests:    100,
			RequestTimeout:           2 * time.Second,
			Provider:                 "redis",
			Redis: &config.CircuitBreakerRedisConfig{
				Address:  "localhost:6379",
				Password: "",
				DB:       0,
				CacheTTL: 1 * time.Second,
			},
		}

		builder := circuitbreaker.NewBuilder()
		require.NotNil(t, builder)

		// Apply config with Redis provider
		err := circuitbreaker.WithConfig(redisCfg)(builder)
		require.NoError(t, err)

		err = circuitbreaker.WithLogger(logger)(builder)
		require.NoError(t, err)

		// In a real test, we would need Redis running, so we'll skip the build step
		// or mock it. For this example we'll just test that the config was applied.
		// cb, err := builder.Build()
		// require.NoError(t, err)
		// require.NotNil(t, cb)
	})

	t.Run("Builder with custom Redis config", func(t *testing.T) {
		redisConfig := &circuitbreaker.CircuitBreakerRedisConfig{
			Address:  "localhost:6379",
			Password: "",
			DB:       0,
			CacheTTL: 1 * time.Second,
		}

		builder := circuitbreaker.NewBuilder()
		require.NotNil(t, builder)

		// Apply config
		err := circuitbreaker.WithConfig(cfg)(builder)
		require.NoError(t, err)

		err = circuitbreaker.WithLogger(logger)(builder)
		require.NoError(t, err)

		// Apply custom Redis config
		err = circuitbreaker.WithRedisConfig(redisConfig)(builder)
		require.NoError(t, err)

		// In a real test, we would need Redis running, so we'll skip the build step
	})

	t.Run("Builder with nil Redis config", func(t *testing.T) {
		builder := circuitbreaker.NewBuilder()
		require.NotNil(t, builder)

		// Apply nil Redis config
		err := circuitbreaker.WithRedisConfig(nil)(builder)
		assert.Error(t, err)
		assert.Equal(t, circuitbreaker.ErrNilRedisConfig, err)
	})

	t.Run("Builder with custom Redis client", func(t *testing.T) {
		// Create a Redis client (we won't actually connect in tests)
		client := redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
		defer client.Close()

		builder := circuitbreaker.NewBuilder()
		require.NotNil(t, builder)

		// Apply config
		err := circuitbreaker.WithConfig(cfg)(builder)
		require.NoError(t, err)

		err = circuitbreaker.WithLogger(logger)(builder)
		require.NoError(t, err)

		// Apply custom Redis client
		err = circuitbreaker.WithRedisClient(client)(builder)
		require.NoError(t, err)

		// In a real test, we would need Redis running, so we'll skip the build step
	})

	t.Run("Builder with nil Redis client", func(t *testing.T) {
		builder := circuitbreaker.NewBuilder()
		require.NotNil(t, builder)

		// Apply nil Redis client
		err := circuitbreaker.WithRedisClient(nil)(builder)
		assert.Error(t, err)
		assert.Equal(t, circuitbreaker.ErrNilRedisClient, err)
	})
}

func TestNewCircuitBreaker(t *testing.T) {
	logger := log.NewLogger("debug")
	cfg := &config.CircuitBreakerConfig{
		Threshold:                5,
		Timeout:                  10 * time.Second,
		HalfOpenSuccessThreshold: 2,
		MaxConcurrentRequests:    100,
		RequestTimeout:           2 * time.Second,
		Provider:                 "memory",
	}

	t.Run("Create circuit breaker with valid options", func(t *testing.T) {
		cb, err := circuitbreaker.New(
			circuitbreaker.WithConfig(cfg),
			circuitbreaker.WithLogger(logger),
		)
		require.NoError(t, err)
		require.NotNil(t, cb)

		// Verify it works
		state := cb.GetState("test")
		assert.Equal(t, ports.StateClosed, state)
	})

	t.Run("Create circuit breaker with invalid option", func(t *testing.T) {
		cb, err := circuitbreaker.New(
			circuitbreaker.WithConfig(nil), // Invalid
			circuitbreaker.WithLogger(logger),
		)
		assert.Error(t, err)
		assert.Equal(t, circuitbreaker.ErrNilConfig, err)
		assert.Nil(t, cb)
	})

	t.Run("Create circuit breaker with multiple invalid options", func(t *testing.T) {
		cb, err := circuitbreaker.New(
			circuitbreaker.WithConfig(nil),      // Invalid
			circuitbreaker.WithLogger(nil),      // Invalid
			circuitbreaker.WithRedisConfig(nil), // Invalid
		)
		assert.Error(t, err)
		assert.Equal(t, circuitbreaker.ErrNilConfig, err) // First error is returned
		assert.Nil(t, cb)
	})
}
