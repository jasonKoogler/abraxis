package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
	"github.com/jasonKoogler/prism/internal/features/gateway/adapters/circuitbreaker"
	"github.com/jasonKoogler/prism/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCircuitBreaker_Execute(t *testing.T) {
	// Setup
	logger := log.NewLogger("debug")
	cfg := &config.CircuitBreakerConfig{
		Threshold:                3,
		Timeout:                  100 * time.Millisecond,
		HalfOpenSuccessThreshold: 2,
		MaxConcurrentRequests:    5,
		RequestTimeout:           50 * time.Millisecond,
	}

	cb := circuitbreaker.NewMemoryCircuitBreaker(cfg, logger)
	require.NotNil(t, cb)

	t.Run("Successful execution", func(t *testing.T) {
		err := cb.Execute(context.Background(), "test-success", func() error {
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, ports.StateClosed, cb.GetState("test-success"))
	})

	t.Run("Failed execution", func(t *testing.T) {
		testErr := errors.New("test error")
		err := cb.Execute(context.Background(), "test-failure", func() error {
			return testErr
		})
		assert.Error(t, err)
		assert.Equal(t, testErr, err)
		assert.Equal(t, ports.StateClosed, cb.GetState("test-failure"))
	})

	t.Run("Circuit opens after threshold failures", func(t *testing.T) {
		circuitName := "test-open-circuit"
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
		circuitName := "test-half-open"
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
		assert.Equal(t, ports.StateHalfOpen, cb.GetState(circuitName))
	})

	t.Run("Circuit closes after success threshold in half-open state", func(t *testing.T) {
		circuitName := "test-close-after-half-open"
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
		}

		// Circuit should now be closed
		assert.Equal(t, ports.StateClosed, cb.GetState(circuitName))
	})

	t.Run("Circuit reopens after failure in half-open state", func(t *testing.T) {
		circuitName := "test-reopen-after-half-open"
		testErr := errors.New("test error")

		// Open the circuit
		for i := 0; i < cfg.Threshold; i++ {
			_ = cb.Execute(context.Background(), circuitName, func() error {
				return testErr
			})
		}

		// Wait for timeout to transition to half-open
		time.Sleep(cfg.Timeout + 10*time.Millisecond)

		// Execute with an error to reopen the circuit
		err := cb.Execute(context.Background(), circuitName, func() error {
			return testErr
		})
		assert.Error(t, err)

		// Circuit should be open again
		assert.Equal(t, ports.StateOpen, cb.GetState(circuitName))
	})

	t.Run("Request times out", func(t *testing.T) {
		err := cb.Execute(context.Background(), "test-timeout", func() error {
			time.Sleep(cfg.RequestTimeout * 2) // Sleep longer than timeout
			return nil
		})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.DeadlineExceeded))
	})

	t.Run("Too many concurrent requests", func(t *testing.T) {
		circuitName := "test-concurrent"

		// Launch concurrent requests
		errChan := make(chan error, cfg.MaxConcurrentRequests+1)
		doneChan := make(chan struct{}, cfg.MaxConcurrentRequests)

		// Helper function that blocks until signaled
		blockingFn := func() error {
			<-doneChan
			return nil
		}

		// Start max concurrent requests
		for i := 0; i < cfg.MaxConcurrentRequests; i++ {
			go func() {
				errChan <- cb.Execute(context.Background(), circuitName, blockingFn)
			}()
		}

		// Wait a bit to ensure all goroutines have started
		time.Sleep(10 * time.Millisecond)

		// This one should be rejected
		err := cb.Execute(context.Background(), circuitName, func() error {
			return nil // Should not be called
		})
		assert.Error(t, err)
		assert.Equal(t, circuitbreaker.ErrTooManyConcurrentRequests, err)

		// Release the blocking goroutines
		for i := 0; i < cfg.MaxConcurrentRequests; i++ {
			doneChan <- struct{}{}
		}

		// Collect results from other goroutines
		for i := 0; i < cfg.MaxConcurrentRequests; i++ {
			err := <-errChan
			assert.NoError(t, err)
		}
	})
}

func TestMemoryCircuitBreaker_ExecuteWithFallback(t *testing.T) {
	logger := log.NewLogger("debug")
	cfg := &config.CircuitBreakerConfig{
		Threshold:                3,
		Timeout:                  100 * time.Millisecond,
		HalfOpenSuccessThreshold: 2,
		MaxConcurrentRequests:    5,
		RequestTimeout:           50 * time.Millisecond,
	}

	cb := circuitbreaker.NewMemoryCircuitBreaker(cfg, logger)
	require.NotNil(t, cb)

	t.Run("Fallback called on error", func(t *testing.T) {
		testErr := errors.New("test error")
		fallbackCalled := false

		err := cb.ExecuteWithFallback(
			context.Background(),
			"test-fallback",
			func() error {
				return testErr
			},
			func(err error) error {
				fallbackCalled = true
				assert.Equal(t, testErr, err)
				return nil
			},
		)

		assert.NoError(t, err)
		assert.True(t, fallbackCalled)
	})

	t.Run("Fallback called when circuit open", func(t *testing.T) {
		circuitName := "test-fallback-open"
		testErr := errors.New("test error")

		// Open the circuit
		for i := 0; i < cfg.Threshold; i++ {
			_ = cb.Execute(context.Background(), circuitName, func() error {
				return testErr
			})
		}

		fallbackCalled := false
		err := cb.ExecuteWithFallback(
			context.Background(),
			circuitName,
			func() error {
				return nil // Should not be called
			},
			func(err error) error {
				fallbackCalled = true
				assert.Equal(t, circuitbreaker.ErrCircuitOpen, err)
				return nil
			},
		)

		assert.NoError(t, err)
		assert.True(t, fallbackCalled)
	})
}

func TestMemoryCircuitBreaker_GetMetrics(t *testing.T) {
	logger := log.NewLogger("debug")
	cfg := &config.CircuitBreakerConfig{
		Threshold:                3,
		Timeout:                  100 * time.Millisecond,
		HalfOpenSuccessThreshold: 2,
		MaxConcurrentRequests:    5,
		RequestTimeout:           50 * time.Millisecond,
	}

	cb := circuitbreaker.NewMemoryCircuitBreaker(cfg, logger)
	require.NotNil(t, cb)

	circuitName := "test-metrics"
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

	// Get metrics
	metrics := cb.GetMetrics(circuitName)
	assert.Equal(t, circuitName, metrics.Name)
	assert.Equal(t, ports.StateOpen, metrics.State)
	assert.Equal(t, int64(3), metrics.Failures)
	assert.Equal(t, int64(2), metrics.Successes)
}

func TestMemoryCircuitBreaker_Reset(t *testing.T) {
	logger := log.NewLogger("debug")
	cfg := &config.CircuitBreakerConfig{
		Threshold:                3,
		Timeout:                  100 * time.Millisecond,
		HalfOpenSuccessThreshold: 2,
		MaxConcurrentRequests:    5,
		RequestTimeout:           50 * time.Millisecond,
	}

	cb := circuitbreaker.NewMemoryCircuitBreaker(cfg, logger)
	require.NotNil(t, cb)

	circuitName := "test-reset"
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

	// Verify circuit is closed
	assert.Equal(t, ports.StateClosed, cb.GetState(circuitName))

	// Verify metrics are reset
	metrics := cb.GetMetrics(circuitName)
	assert.Equal(t, int64(0), metrics.Failures)
	assert.Equal(t, int64(0), metrics.Successes)
}

func TestMemoryCircuitBreaker_Close(t *testing.T) {
	logger := log.NewLogger("debug")
	cfg := &config.CircuitBreakerConfig{
		Threshold:                3,
		Timeout:                  100 * time.Millisecond,
		HalfOpenSuccessThreshold: 2,
		MaxConcurrentRequests:    5,
		RequestTimeout:           50 * time.Millisecond,
	}

	cb := circuitbreaker.NewMemoryCircuitBreaker(cfg, logger)
	require.NotNil(t, cb)

	// Setup some circuits
	_ = cb.Execute(context.Background(), "circuit1", func() error { return nil })
	_ = cb.Execute(context.Background(), "circuit2", func() error { return nil })

	// Close the circuit breaker
	err := cb.Close()
	assert.NoError(t, err)

	// Verify both circuits are closed and reset (we'll validate with a new request)
	err = cb.Execute(context.Background(), "circuit1", func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, ports.StateClosed, cb.GetState("circuit1"))
}
