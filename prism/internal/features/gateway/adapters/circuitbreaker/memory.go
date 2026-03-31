package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
	"github.com/jasonKoogler/prism/internal/ports"
)

var (
	// ErrCircuitOpen is returned when the circuit breaker is open
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrTooManyConcurrentRequests is returned when too many requests are in flight
	ErrTooManyConcurrentRequests = errors.New("too many concurrent requests")
)

// circuitState tracks the state and metrics of a single circuit breaker
type circuitState struct {
	name               string
	state              ports.CircuitBreakerState
	failures           int64
	successes          int64
	rejected           int64
	timeout            int64
	lastStateChange    time.Time
	openTimeout        time.Duration
	failureThreshold   int
	successThreshold   int
	concurrentRequests int32
	maxConcurrent      int32
}

// MemoryCircuitBreaker implements a circuit breaker pattern using in-memory state
type MemoryCircuitBreaker struct {
	mu       sync.RWMutex
	circuits map[string]*circuitState
	config   *ports.CircuitBreakerConfig
	logger   *log.Logger
}

// NewMemoryCircuitBreaker creates a new in-memory circuit breaker
func NewMemoryCircuitBreaker(config *config.CircuitBreakerConfig, logger *log.Logger) *MemoryCircuitBreaker {
	// Set default values if not provided
	if config.Threshold <= 0 {
		config.Threshold = 5
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	if config.HalfOpenSuccessThreshold <= 0 {
		config.HalfOpenSuccessThreshold = 2
	}
	if config.MaxConcurrentRequests <= 0 {
		config.MaxConcurrentRequests = 100
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 2 * time.Second
	}

	// Convert to ports.CircuitBreakerConfig
	portsCfg := &ports.CircuitBreakerConfig{
		Threshold:                config.Threshold,
		Timeout:                  config.Timeout,
		HalfOpenSuccessThreshold: config.HalfOpenSuccessThreshold,
		MaxConcurrentRequests:    config.MaxConcurrentRequests,
		RequestTimeout:           config.RequestTimeout,
	}

	return &MemoryCircuitBreaker{
		circuits: make(map[string]*circuitState),
		config:   portsCfg,
		logger:   logger,
	}
}

// getCircuit gets or creates a circuit state for a given name
func (cb *MemoryCircuitBreaker) getCircuit(name string) *circuitState {
	cb.mu.RLock()
	circuit, exists := cb.circuits[name]
	cb.mu.RUnlock()

	if exists {
		return circuit
	}

	// Need to create a new circuit
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Double-check in case it was created while we were waiting for the lock
	if circuit, exists = cb.circuits[name]; exists {
		return circuit
	}

	circuit = &circuitState{
		name:             name,
		state:            ports.StateClosed,
		lastStateChange:  time.Now(),
		openTimeout:      cb.config.Timeout,
		failureThreshold: cb.config.Threshold,
		successThreshold: cb.config.HalfOpenSuccessThreshold,
		maxConcurrent:    int32(cb.config.MaxConcurrentRequests),
	}

	cb.circuits[name] = circuit
	return circuit
}

// Execute runs the provided function with circuit breaker protection
func (cb *MemoryCircuitBreaker) Execute(ctx context.Context, name string, fn func() error) error {
	circuit := cb.getCircuit(name)

	// If the circuit is open, check if it's time to try again
	if circuit.state == ports.StateOpen {
		if time.Since(circuit.lastStateChange) > circuit.openTimeout {
			// Transition to half-open state
			cb.mu.Lock()
			if circuit.state == ports.StateOpen {
				circuit.state = ports.StateHalfOpen
				circuit.lastStateChange = time.Now()
				cb.logger.Debug("Circuit transitioned from open to half-open",
					log.String("circuit", name))
			}
			cb.mu.Unlock()
		} else {
			// Circuit is still open, immediately reject
			atomic.AddInt64(&circuit.rejected, 1)
			return ErrCircuitOpen
		}
	}

	// Check if we're at max concurrent requests
	if atomic.LoadInt32(&circuit.concurrentRequests) >= circuit.maxConcurrent {
		atomic.AddInt64(&circuit.rejected, 1)
		return ErrTooManyConcurrentRequests
	}

	// Increment concurrent requests counter
	atomic.AddInt32(&circuit.concurrentRequests, 1)
	defer atomic.AddInt32(&circuit.concurrentRequests, -1)

	// Create a context with timeout if one wasn't provided
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok && cb.config.RequestTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, cb.config.RequestTimeout)
		if cancel != nil {
			defer cancel()
		}
	}

	// Execute with a goroutine and channel to handle timeouts
	errChan := make(chan error, 1)
	go func() {
		errChan <- fn()
	}()

	// Wait for result or timeout
	select {
	case err := <-errChan:
		if err != nil {
			cb.recordFailure(circuit)
			return err
		}
		cb.recordSuccess(circuit)
		return nil

	case <-ctx.Done():
		// Timeout or cancellation
		atomic.AddInt64(&circuit.timeout, 1)
		cb.recordFailure(circuit)
		return ctx.Err()
	}
}

// ExecuteWithFallback executes with a fallback function on failure
func (cb *MemoryCircuitBreaker) ExecuteWithFallback(
	ctx context.Context,
	name string,
	fn func() error,
	fallback func(error) error,
) error {
	err := cb.Execute(ctx, name, fn)
	if err != nil && fallback != nil {
		return fallback(err)
	}
	return err
}

// recordSuccess records a successful execution
func (cb *MemoryCircuitBreaker) recordSuccess(circuit *circuitState) {
	atomic.AddInt64(&circuit.successes, 1)

	// If we're half-open and have reached the success threshold, close the circuit
	if circuit.state == ports.StateHalfOpen {
		// Need a write lock to modify the state
		cb.mu.Lock()
		defer cb.mu.Unlock()

		// Double check the state in case it changed while waiting for the lock
		if circuit.state == ports.StateHalfOpen &&
			atomic.LoadInt64(&circuit.successes) >= int64(circuit.successThreshold) {
			circuit.state = ports.StateClosed
			circuit.lastStateChange = time.Now()

			// Reset counters
			atomic.StoreInt64(&circuit.failures, 0)
			atomic.StoreInt64(&circuit.successes, 0)

			cb.logger.Debug("Circuit closed after success threshold reached",
				log.String("circuit", circuit.name))
		}
	}
}

// recordFailure records a failed execution
func (cb *MemoryCircuitBreaker) recordFailure(circuit *circuitState) {
	failures := atomic.AddInt64(&circuit.failures, 1)

	// If failures exceed threshold, open the circuit
	if (circuit.state == ports.StateClosed && failures >= int64(circuit.failureThreshold)) ||
		(circuit.state == ports.StateHalfOpen) {
		// Need a write lock to modify the state
		cb.mu.Lock()
		defer cb.mu.Unlock()

		// Double check the state and failures in case they changed while waiting for the lock
		currentFailures := atomic.LoadInt64(&circuit.failures)
		if (circuit.state == ports.StateClosed && currentFailures >= int64(circuit.failureThreshold)) ||
			(circuit.state == ports.StateHalfOpen) {
			circuit.state = ports.StateOpen
			circuit.lastStateChange = time.Now()

			cb.logger.Debug("Circuit opened due to failures",
				log.String("circuit", circuit.name),
				log.Int64("failures", currentFailures))
		}
	}
}

// GetState returns the current state of the circuit breaker
func (cb *MemoryCircuitBreaker) GetState(name string) ports.CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if circuit, exists := cb.circuits[name]; exists {
		return circuit.state
	}

	// Default state for a new circuit is closed
	return ports.StateClosed
}

// GetMetrics returns metrics for the circuit breaker
func (cb *MemoryCircuitBreaker) GetMetrics(name string) *ports.CircuitBreakerMetrics {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if circuit, exists := cb.circuits[name]; exists {
		return &ports.CircuitBreakerMetrics{
			Name:            circuit.name,
			State:           circuit.state,
			Failures:        atomic.LoadInt64(&circuit.failures),
			Successes:       atomic.LoadInt64(&circuit.successes),
			Rejected:        atomic.LoadInt64(&circuit.rejected),
			Timeout:         atomic.LoadInt64(&circuit.timeout),
			LastStateChange: circuit.lastStateChange,
		}
	}

	return &ports.CircuitBreakerMetrics{
		Name:  name,
		State: ports.StateClosed,
	}
}

// Reset resets the circuit breaker to closed state
func (cb *MemoryCircuitBreaker) Reset(name string) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if circuit, exists := cb.circuits[name]; exists {
		circuit.state = ports.StateClosed
		circuit.lastStateChange = time.Now()
		atomic.StoreInt64(&circuit.failures, 0)
		atomic.StoreInt64(&circuit.successes, 0)
		atomic.StoreInt64(&circuit.rejected, 0)
		atomic.StoreInt64(&circuit.timeout, 0)

		cb.logger.Debug("Circuit manually reset", log.String("circuit", name))
	}

	return nil
}

// Close performs any necessary cleanup
func (cb *MemoryCircuitBreaker) Close() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Clear circuits
	cb.circuits = make(map[string]*circuitState)

	return nil
}
