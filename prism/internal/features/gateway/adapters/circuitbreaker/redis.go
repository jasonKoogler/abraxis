package circuitbreaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
	"github.com/jasonKoogler/prism/internal/ports"
	"github.com/redis/go-redis/v9"
)

const (
	// Redis key prefixes
	cbStatePrefix      = "cb:state:"
	cbMetricsPrefix    = "cb:metrics:"
	cbConcurrentPrefix = "cb:concurrent:"
	cbLockPrefix       = "cb:lock:"

	// Default lock timeout
	lockTimeout = 5 * time.Second
)

// RedisCircuitBreaker implements a circuit breaker pattern using Redis for distributed state
type RedisCircuitBreaker struct {
	client *redis.Client
	config *ports.CircuitBreakerConfig
	logger *log.Logger

	// Local cache to reduce Redis calls
	mu         sync.RWMutex
	localCache map[string]circuitBreakerCache
	cacheTTL   time.Duration
}

// circuitBreakerCache holds cached circuit breaker data
type circuitBreakerCache struct {
	state    ports.CircuitBreakerState
	cachedAt time.Time
}

// CircuitBreakerRedisConfig holds configuration for the Redis circuit breaker
type CircuitBreakerRedisConfig struct {
	// Redis connection options
	Address  string
	Password string
	DB       int

	// Circuit breaker options
	CacheTTL time.Duration
}

// NewRedisCircuitBreaker creates a new Redis-based circuit breaker
func NewRedisCircuitBreaker(cfg *config.CircuitBreakerConfig, redisCfg *CircuitBreakerRedisConfig, logger *log.Logger) (*RedisCircuitBreaker, error) {
	// Set default values if not provided
	if cfg.Threshold <= 0 {
		cfg.Threshold = 5
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.HalfOpenSuccessThreshold <= 0 {
		cfg.HalfOpenSuccessThreshold = 2
	}
	if cfg.MaxConcurrentRequests <= 0 {
		cfg.MaxConcurrentRequests = 100
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 2 * time.Second
	}

	if redisCfg.CacheTTL <= 0 {
		redisCfg.CacheTTL = 1 * time.Second
	}

	// Convert to ports.CircuitBreakerConfig
	portsCfg := &ports.CircuitBreakerConfig{
		Threshold:                cfg.Threshold,
		Timeout:                  cfg.Timeout,
		HalfOpenSuccessThreshold: cfg.HalfOpenSuccessThreshold,
		MaxConcurrentRequests:    cfg.MaxConcurrentRequests,
		RequestTimeout:           cfg.RequestTimeout,
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     redisCfg.Address,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})

	// Verify connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCircuitBreaker{
		client:     client,
		config:     portsCfg,
		logger:     logger,
		localCache: make(map[string]circuitBreakerCache),
		cacheTTL:   redisCfg.CacheTTL,
	}, nil
}

// Execute runs the provided function with circuit breaker protection
func (cb *RedisCircuitBreaker) Execute(ctx context.Context, name string, fn func() error) error {
	// Get current state (check cache first)
	state, err := cb.getState(ctx, name)
	if err != nil {
		cb.logger.Warn("Error getting circuit state, assuming closed", log.Error(err))
		state = ports.StateClosed
	}

	// If the circuit is open, check if it's time to try again
	if state == ports.StateOpen {
		// Get when the circuit was opened
		lastChange, err := cb.getLastStateChange(ctx, name)
		if err != nil {
			cb.logger.Warn("Error getting last state change, using current time", log.Error(err))
			lastChange = time.Now()
		}

		if time.Since(lastChange) > cb.config.Timeout {
			// Transition to half-open state
			if err := cb.trySetState(ctx, name, ports.StateHalfOpen); err != nil {
				cb.logger.Warn("Failed to set half-open state", log.Error(err))
				// Treat as still open
				return ErrCircuitOpen
			}

			cb.logger.Debug("Circuit transitioned from open to half-open", log.String("circuit", name))
			state = ports.StateHalfOpen
		} else {
			// Circuit is still open, immediately reject
			cb.incrementRejected(ctx, name)
			return ErrCircuitOpen
		}
	}

	// Check concurrent requests
	if !cb.tryAcquireExecution(ctx, name) {
		cb.incrementRejected(ctx, name)
		return ErrTooManyConcurrentRequests
	}

	// Ensure we release the concurrent request counter
	defer cb.releaseExecution(ctx, name)

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
			cb.recordFailure(ctx, name, state)
			return err
		}
		cb.recordSuccess(ctx, name, state)
		return nil

	case <-ctx.Done():
		// Timeout or cancellation
		cb.incrementTimeout(ctx, name)
		cb.recordFailure(ctx, name, state)
		return ctx.Err()
	}
}

// ExecuteWithFallback executes with a fallback function on failure
func (cb *RedisCircuitBreaker) ExecuteWithFallback(
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

// getState retrieves the current state from cache or Redis
func (cb *RedisCircuitBreaker) getState(ctx context.Context, name string) (ports.CircuitBreakerState, error) {
	// Check local cache first
	cb.mu.RLock()
	cached, exists := cb.localCache[name]
	cb.mu.RUnlock()

	if exists && time.Since(cached.cachedAt) < cb.cacheTTL {
		return cached.state, nil
	}

	// Not in cache or expired, get from Redis
	stateKey := cbStatePrefix + name
	stateStr, err := cb.client.Get(ctx, stateKey).Result()
	if err == redis.Nil {
		// Key does not exist, initialize as closed
		state := ports.StateClosed
		err := cb.client.Set(ctx, stateKey, string(state), 0).Err()
		if err != nil {
			return state, err
		}

		// Update local cache
		cb.mu.Lock()
		cb.localCache[name] = circuitBreakerCache{
			state:    state,
			cachedAt: time.Now(),
		}
		cb.mu.Unlock()

		return state, nil
	} else if err != nil {
		return "", err
	}

	state := ports.CircuitBreakerState(stateStr)

	// Update local cache
	cb.mu.Lock()
	cb.localCache[name] = circuitBreakerCache{
		state:    state,
		cachedAt: time.Now(),
	}
	cb.mu.Unlock()

	return state, nil
}

// getLastStateChange gets the timestamp of the last state change
func (cb *RedisCircuitBreaker) getLastStateChange(ctx context.Context, name string) (time.Time, error) {
	key := cbMetricsPrefix + name + ":lastchange"
	timestamp, err := cb.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		// Key does not exist, use current time
		now := time.Now()
		if err := cb.client.Set(ctx, key, now.UnixNano(), 0).Err(); err != nil {
			return now, err
		}
		return now, nil
	} else if err != nil {
		return time.Time{}, err
	}

	return time.Unix(0, timestamp), nil
}

// trySetState attempts to set the circuit state with distributed locking
func (cb *RedisCircuitBreaker) trySetState(ctx context.Context, name string, state ports.CircuitBreakerState) error {
	lockKey := cbLockPrefix + name
	stateKey := cbStatePrefix + name

	// Try to acquire lock
	locked, err := cb.client.SetNX(ctx, lockKey, "1", lockTimeout).Result()
	if err != nil {
		return err
	}

	if !locked {
		return fmt.Errorf("failed to acquire lock for circuit %s", name)
	}

	// We have the lock, set the state
	defer cb.client.Del(ctx, lockKey)

	// Set the state
	if err := cb.client.Set(ctx, stateKey, string(state), 0).Err(); err != nil {
		return err
	}

	// Record the timestamp
	now := time.Now().UnixNano()
	if err := cb.client.Set(ctx, cbMetricsPrefix+name+":lastchange", now, 0).Err(); err != nil {
		return err
	}

	// Update local cache
	cb.mu.Lock()
	cb.localCache[name] = circuitBreakerCache{
		state:    state,
		cachedAt: time.Now(),
	}
	cb.mu.Unlock()

	return nil
}

// tryAcquireExecution tries to acquire a slot for execution, respecting max concurrent requests
func (cb *RedisCircuitBreaker) tryAcquireExecution(ctx context.Context, name string) bool {
	key := cbConcurrentPrefix + name

	// Increment counter
	count, err := cb.client.Incr(ctx, key).Result()
	if err != nil {
		cb.logger.Warn("Failed to increment concurrent counter", log.Error(err))
		return false
	}

	// Set expiration if it's a new key
	if count == 1 {
		cb.client.Expire(ctx, key, 30*time.Second)
	}

	// Check if we've exceeded max concurrent requests
	if count > int64(cb.config.MaxConcurrentRequests) {
		// Decrement since we're not going to execute
		cb.client.Decr(ctx, key)
		return false
	}

	return true
}

// releaseExecution decrements the concurrent execution counter
func (cb *RedisCircuitBreaker) releaseExecution(ctx context.Context, name string) {
	key := cbConcurrentPrefix + name
	cb.client.Decr(ctx, key)
}

// incrementRejected increments the rejected requests counter
func (cb *RedisCircuitBreaker) incrementRejected(ctx context.Context, name string) {
	key := cbMetricsPrefix + name + ":rejected"
	cb.client.Incr(ctx, key)
}

// incrementTimeout increments the timeout counter
func (cb *RedisCircuitBreaker) incrementTimeout(ctx context.Context, name string) {
	key := cbMetricsPrefix + name + ":timeout"
	cb.client.Incr(ctx, key)
}

// recordFailure records a failed execution
func (cb *RedisCircuitBreaker) recordFailure(ctx context.Context, name string, currentState ports.CircuitBreakerState) {
	// Increment failure counter
	failureKey := cbMetricsPrefix + name + ":failures"
	failures, err := cb.client.Incr(ctx, failureKey).Result()
	if err != nil {
		cb.logger.Warn("Failed to record failure", log.Error(err))
		return
	}

	// If failures exceed threshold, open the circuit
	if (currentState == ports.StateClosed && failures >= int64(cb.config.Threshold)) ||
		(currentState == ports.StateHalfOpen) {
		if err := cb.trySetState(ctx, name, ports.StateOpen); err != nil {
			cb.logger.Warn("Failed to open circuit", log.Error(err))
			return
		}

		cb.logger.Debug("Circuit opened due to failures",
			log.String("circuit", name),
			log.Int64("failures", failures))
	}
}

// recordSuccess records a successful execution
func (cb *RedisCircuitBreaker) recordSuccess(ctx context.Context, name string, currentState ports.CircuitBreakerState) {
	// Increment success counter
	successKey := cbMetricsPrefix + name + ":successes"
	successes, err := cb.client.Incr(ctx, successKey).Result()
	if err != nil {
		cb.logger.Warn("Failed to record success", log.Error(err))
		return
	}

	// If we're half-open and have reached the success threshold, close the circuit
	if currentState == ports.StateHalfOpen && successes >= int64(cb.config.HalfOpenSuccessThreshold) {
		if err := cb.trySetState(ctx, name, ports.StateClosed); err != nil {
			cb.logger.Warn("Failed to close circuit", log.Error(err))
			return
		}

		// Reset counters
		pipeline := cb.client.Pipeline()
		pipeline.Set(ctx, cbMetricsPrefix+name+":failures", 0, 0)
		pipeline.Set(ctx, cbMetricsPrefix+name+":successes", 0, 0)
		_, err := pipeline.Exec(ctx)
		if err != nil {
			cb.logger.Warn("Failed to reset counters", log.Error(err))
		}

		cb.logger.Debug("Circuit closed after success threshold reached",
			log.String("circuit", name))
	}
}

// GetState returns the current state of the circuit breaker
func (cb *RedisCircuitBreaker) GetState(name string) ports.CircuitBreakerState {
	state, err := cb.getState(context.Background(), name)
	if err != nil {
		cb.logger.Warn("Error getting state", log.Error(err))
		return ports.StateClosed
	}
	return state
}

// GetMetrics returns metrics for the circuit breaker
func (cb *RedisCircuitBreaker) GetMetrics(name string) *ports.CircuitBreakerMetrics {
	ctx := context.Background()

	// Get state
	state, err := cb.getState(ctx, name)
	if err != nil {
		cb.logger.Warn("Error getting state for metrics", log.Error(err))
		state = ports.StateClosed
	}

	// Get last state change
	lastChange, err := cb.getLastStateChange(ctx, name)
	if err != nil {
		cb.logger.Warn("Error getting last state change for metrics", log.Error(err))
		lastChange = time.Now()
	}

	// Get all metrics in a pipeline
	pipe := cb.client.Pipeline()
	failureCmd := pipe.Get(ctx, cbMetricsPrefix+name+":failures")
	successCmd := pipe.Get(ctx, cbMetricsPrefix+name+":successes")
	rejectedCmd := pipe.Get(ctx, cbMetricsPrefix+name+":rejected")
	timeoutCmd := pipe.Get(ctx, cbMetricsPrefix+name+":timeout")

	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		cb.logger.Warn("Error getting metrics", log.Error(err))
	}

	// Parse metrics (default to 0 if not found)
	getInt64 := func(cmd *redis.StringCmd) int64 {
		val, err := cmd.Int64()
		if err != nil {
			return 0
		}
		return val
	}

	return &ports.CircuitBreakerMetrics{
		Name:            name,
		State:           state,
		Failures:        getInt64(failureCmd),
		Successes:       getInt64(successCmd),
		Rejected:        getInt64(rejectedCmd),
		Timeout:         getInt64(timeoutCmd),
		LastStateChange: lastChange,
	}
}

// Reset resets the circuit breaker to closed state
func (cb *RedisCircuitBreaker) Reset(name string) error {
	ctx := context.Background()

	// Set state to closed
	if err := cb.trySetState(ctx, name, ports.StateClosed); err != nil {
		return err
	}

	// Reset all counters
	pipe := cb.client.Pipeline()
	pipe.Set(ctx, cbMetricsPrefix+name+":failures", 0, 0)
	pipe.Set(ctx, cbMetricsPrefix+name+":successes", 0, 0)
	pipe.Set(ctx, cbMetricsPrefix+name+":rejected", 0, 0)
	pipe.Set(ctx, cbMetricsPrefix+name+":timeout", 0, 0)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	cb.logger.Debug("Circuit manually reset", log.String("circuit", name))
	return nil
}

// Close performs any necessary cleanup
func (cb *RedisCircuitBreaker) Close() error {
	return cb.client.Close()
}
