package ports

import (
	"context"
	"time"
)

// CircuitBreakerState represents the current state of a circuit breaker
type CircuitBreakerState string

const (
	// Closed means the circuit is closed and requests flow normally
	StateClosed CircuitBreakerState = "closed"
	// Open means the circuit is open and requests are immediately rejected
	StateOpen CircuitBreakerState = "open"
	// HalfOpen means the circuit is allowing a limited number of test requests
	StateHalfOpen CircuitBreakerState = "half-open"
)

// CircuitBreakerMetrics contains metrics about circuit breaker operations
type CircuitBreakerMetrics struct {
	Name            string
	State           CircuitBreakerState
	Failures        int64
	Successes       int64
	Rejected        int64
	Timeout         int64
	LastStateChange time.Time
}

// CircuitBreaker defines the interface for circuit breaker operations
type CircuitBreaker interface {
	// Execute executes the given function with circuit breaker protection
	// Returns error if the circuit is open or if the function fails
	Execute(ctx context.Context, name string, fn func() error) error

	// ExecuteWithFallback executes the given function with circuit breaker protection
	// If the circuit is open or the function fails, executes the fallback function
	ExecuteWithFallback(ctx context.Context, name string, fn func() error, fallback func(error) error) error

	// GetState returns the current state of the circuit breaker for a service
	GetState(name string) CircuitBreakerState

	// GetMetrics returns metrics for the specified circuit breaker
	GetMetrics(name string) *CircuitBreakerMetrics

	// Reset resets the circuit breaker to closed state
	Reset(name string) error

	// Close gracefully closes the circuit breaker and cleans up resources
	Close() error
}

// CircuitBreakerConfig holds configuration for circuit breakers
type CircuitBreakerConfig struct {
	// Threshold is the number of failures needed to trip the circuit
	Threshold int

	// Timeout is how long to wait before trying again once open
	Timeout time.Duration

	// HalfOpenSuccessThreshold is the number of successful requests in half-open state to close the circuit
	HalfOpenSuccessThreshold int

	// MaxConcurrentRequests is the maximum number of concurrent requests allowed
	MaxConcurrentRequests int

	// RequestTimeout is the timeout for a single request
	RequestTimeout time.Duration
}
