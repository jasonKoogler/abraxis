# Circuit Breaker Adapter

## Overview

The Circuit Breaker adapter implements the circuit breaker pattern for fault tolerance in distributed systems. It prevents cascading failures by temporarily stopping operations to a failing service and providing graceful degradation when external dependencies fail.

The adapter supports both in-memory and Redis-based circuit breakers, with the latter providing distributed state management suitable for clustered environments.

## Port Interface

The adapter implements the `CircuitBreaker` interface from the `ports` package:

```go
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
```

## Circuit Breaker States

The circuit breaker can be in one of three states:

1. **Closed** - Normal operation; requests flow through.
2. **Open** - Circuit is broken; requests are immediately rejected.
3. **Half-Open** - Testing state after timeout; allows a limited number of test requests.

## Key Components

### MemoryCircuitBreaker

An in-memory implementation suitable for single-instance deployments:

- Thread-safe with fine-grained locking
- Per-circuit state tracking
- Configurable thresholds and timeouts
- Fast local operation

### RedisCircuitBreaker

A distributed implementation using Redis for state persistence:

- Suitable for clustered environments
- Ensures consistent circuit state across instances
- Local caching to reduce Redis calls
- Atomic operations for counters and state changes

### Middleware

HTTP middleware that integrates the circuit breaker with web handlers:

- Wraps HTTP handlers with circuit breaker protection
- Automatically derives circuit names from requests
- Provides fallback responses when circuit is open
- Handles error responses and status codes

## Configuration Options

The adapter uses the functional options pattern for configuration:

```go
// WithConfig sets the configuration for the CircuitBreaker
func WithConfig(cfg *config.CircuitBreakerConfig) CircuitBreakerOption

// WithLogger sets the logger for the CircuitBreaker
func WithLogger(logger *log.Logger) CircuitBreakerOption

// WithRedisConfig sets the Redis configuration for the CircuitBreaker
func WithRedisConfig(redisConfig *CircuitBreakerRedisConfig) CircuitBreakerOption

// WithRedisClient sets a custom Redis client for the CircuitBreaker
func WithRedisClient(client *redis.Client) CircuitBreakerOption
```

## Configuration Parameters

The following parameters are configurable:

| Parameter                  | Description                                  | Default                    |
| -------------------------- | -------------------------------------------- | -------------------------- |
| `Threshold`                | Number of failures before opening circuit    | 5                          |
| `Timeout`                  | Duration circuit stays open before half-open | 10s                        |
| `HalfOpenSuccessThreshold` | Successes needed to close circuit            | 2                          |
| `MaxConcurrentRequests`    | Maximum concurrent requests allowed          | 100                        |
| `RequestTimeout`           | Timeout for individual requests              | 2s                         |
| `Provider`                 | Provider type ("memory" or "redis")          | "memory"                   |
| `Redis.Address`            | Redis server address                         | required if Redis provider |
| `Redis.Password`           | Redis password                               | ""                         |
| `Redis.DB`                 | Redis database number                        | 0                          |
| `Redis.CacheTTL`           | Duration to cache state locally              | 1s                         |

## Usage Examples

### Creating a Circuit Breaker

```go
// Create with functional options
circuitBreaker, err := circuitbreaker.NewCircuitBreaker(
    circuitbreaker.WithConfig(config),
    circuitbreaker.WithLogger(logger),
)

// For Redis-based circuit breaker
circuitBreaker, err := circuitbreaker.NewCircuitBreaker(
    circuitbreaker.WithConfig(config),
    circuitbreaker.WithLogger(logger),
    circuitbreaker.WithRedisConfig(&circuitbreaker.CircuitBreakerRedisConfig{
        Address:  "localhost:6379",
        Password: "",
        DB:       0,
        CacheTTL: 500 * time.Millisecond,
    }),
)
```

### Executing with Circuit Breaker Protection

```go
// Basic execution
err := circuitBreaker.Execute(ctx, "user-service", func() error {
    return userService.GetUser(ctx, userID)
})

// Execution with fallback
err := circuitBreaker.ExecuteWithFallback(
    ctx,
    "payment-service",
    func() error {
        return paymentService.ProcessPayment(ctx, payment)
    },
    func(err error) error {
        // Fallback logic
        metrics.RecordFailure("payment")
        return errors.New("payment service unavailable, try again later")
    },
)
```

### Using the HTTP Middleware

```go
// Create middleware
cbMiddleware := circuitbreaker.NewMiddleware(circuitBreaker, logger)

// Apply to a handler
protectedHandler := cbMiddleware.Handler(myHandler)

// Register with router
router.Handle("/api/users", protectedHandler)
```

### Monitoring Circuit State

```go
// Get current state
state := circuitBreaker.GetState("user-service")
if state == ports.StateOpen {
    log.Warn("User service circuit is open")
}

// Get detailed metrics
metrics := circuitBreaker.GetMetrics("user-service")
log.Info("Circuit metrics",
    "service", metrics.Name,
    "state", metrics.State,
    "failures", metrics.Failures,
    "successes", metrics.Successes,
    "rejected", metrics.Rejected,
    "since", time.Since(metrics.LastStateChange),
)
```

### Manual Circuit Control

```go
// Reset circuit to closed state
err := circuitBreaker.Reset("user-service")
```

## Error Handling

The circuit breaker provides specific error types to indicate different failure modes:

- `ErrCircuitOpen` - Returned when circuit is open and request is rejected
- `ErrTooManyConcurrentRequests` - Returned when concurrent request limit is exceeded
- `ErrTimeoutExceeded` - Returned when request timeout is exceeded
- `ErrFailureThresholdReached` - Returned when failure threshold is reached

## Integration with App

The App struct integrates with the circuit breaker adapter through:

```go
// WithCircuitBreaker sets a custom circuit breaker for the App
func WithCircuitBreaker(cb ports.CircuitBreaker) AppOption

// WithDefaultCircuitBreaker creates a default circuit breaker
func WithDefaultCircuitBreaker() AppOption
```

## Performance Considerations

### Memory Implementation

- Very low overhead, suitable for high-throughput systems
- No network calls
- Thread-safe with minimal contention
- Scales with number of circuits (services)

### Redis Implementation

- Higher latency due to network calls
- Local caching reduces Redis load
- Suitable for distributed systems where state must be shared
- Uses Redis pipelining to reduce round trips

## Health and Monitoring

The circuit breaker provides metrics that can be used for monitoring:

- Circuit state changes (closed → open, open → half-open, half-open → closed)
- Success and failure counts
- Rejected request counts
- Timeout counts

These metrics can be exported to monitoring systems for alerting and visualization.
