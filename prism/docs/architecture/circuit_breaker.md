# Circuit Breaker Integration

This document explains the circuit breaker pattern implementation and its integration with the service proxy and other components in the system architecture.

## Architecture Overview

The circuit breaker pattern is a design pattern used to detect failures and prevent cascading failures in distributed systems. It works on the principle of "fail fast" to maintain system stability and prevent resource exhaustion.

Key components in our circuit breaker implementation:

1. **Circuit Breaker Interface**: Defines common operations for all circuit breaker implementations
2. **Memory Implementation**: In-memory circuit breaker for single-instance deployments
3. **Redis Implementation**: Distributed circuit breaker for multi-instance deployments
4. **Circuit Breaker Middleware**: HTTP middleware that applies circuit breaking to incoming requests
5. **Service Proxy Integration**: Protects downstream service calls

## Circuit Breaker Interface

The circuit breaker interface is defined in `internal/ports/circuitbreaker.go`:

```go
type CircuitBreaker interface {
    // Execute executes the given function with circuit breaker protection
    Execute(ctx context.Context, name string, fn func() error) error

    // ExecuteWithFallback executes the given function with circuit breaker protection
    // and runs fallback if the execution fails
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

Circuit breakers operate in three distinct states:

1. **Closed**: Normal operation, requests are allowed through
2. **Open**: Failure threshold exceeded, all requests are rejected immediately
3. **Half-Open**: Probationary state after the timeout period, limited requests allowed to test recovery

```go
type CircuitBreakerState string

const (
    // StateClosed represents a circuit that is closed and operating normally
    StateClosed CircuitBreakerState = "closed"

    // StateOpen represents a circuit that is open (tripped) and fast-failing requests
    StateOpen CircuitBreakerState = "open"

    // StateHalfOpen represents a circuit that is allowing limited test requests through
    StateHalfOpen CircuitBreakerState = "half-open"
)
```

## Memory Implementation

The `MemoryCircuitBreaker` implementation (in `internal/adapters/circuitbreaker/memory.go`) provides an in-memory circuit breaker suitable for single-instance applications:

```go
type MemoryCircuitBreaker struct {
    mu       sync.RWMutex
    circuits map[string]*circuitState
    config   *ports.CircuitBreakerConfig
    logger   *log.Logger
}
```

Key features:

- Thread-safe operation with mutex protection
- Configurable failure thresholds and timeout periods
- Metrics tracking for success, failure, and rejection counts
- Automatic state transitions based on success/failure patterns

## Redis Implementation

The `RedisCircuitBreaker` implementation (in `internal/adapters/circuitbreaker/redis.go`) provides a distributed circuit breaker for multi-instance deployments:

```go
type RedisCircuitBreaker struct {
    client     *redis.Client
    config     *ports.CircuitBreakerConfig
    logger     *log.Logger
    mu         sync.RWMutex
    localCache map[string]circuitBreakerCache
    cacheTTL   time.Duration
}
```

Key features:

- Distributed state management using Redis
- Local caching to reduce Redis calls
- Atomic operations for concurrent safety
- Consistent circuit behavior across multiple service instances
- Support for handling Redis connection failures

## Configuration

Circuit breaker configuration is managed through the `CircuitBreakerConfig` struct:

```go
type CircuitBreakerConfig struct {
    // Whether circuit breaker is enabled
    Enabled bool `yaml:"enabled"`

    // Provider type: "memory" or "redis"
    Provider string `yaml:"provider"`

    // Number of consecutive failures before opening the circuit
    Threshold int `yaml:"threshold"`

    // Duration the circuit stays open before trying half-open state
    Timeout time.Duration `yaml:"timeout"`

    // Number of successful requests needed in half-open state to close the circuit
    HalfOpenSuccessThreshold int `yaml:"half_open_success_threshold"`

    // Maximum number of concurrent requests allowed
    MaxConcurrentRequests int `yaml:"max_concurrent_requests"`

    // Maximum time allowed for a request before timing out
    RequestTimeout time.Duration `yaml:"request_timeout"`

    // Redis-specific configuration for distributed circuit breaker
    Redis *CircuitBreakerRedisConfig `yaml:"redis,omitempty"`
}
```

Configuration can be loaded from environment variables:

```
CIRCUIT_BREAKER_ENABLED=true
CIRCUIT_BREAKER_PROVIDER=redis
CIRCUIT_BREAKER_THRESHOLD=5
CIRCUIT_BREAKER_TIMEOUT=5s
CIRCUIT_BREAKER_HALF_OPEN_SUCCESS_THRESHOLD=3
CIRCUIT_BREAKER_MAX_CONCURRENT_REQUESTS=10
CIRCUIT_BREAKER_REQUEST_TIMEOUT=1s
```

## Builder Pattern

The circuit breaker system uses a builder pattern with functional options to create and configure instances:

```go
// Create circuit breaker options
opts := []circuitbreaker.CircuitBreakerOption{
    circuitbreaker.WithConfig(cfg.CircuitBreaker),
    circuitbreaker.WithLogger(logger),
}

// Add Redis configuration if using Redis provider
if cfg.CircuitBreaker.Provider == "redis" && cfg.CircuitBreaker.Redis != nil {
    opts = append(opts, circuitbreaker.WithRedisConfig(&circuitbreaker.CircuitBreakerRedisConfig{
        Address:  cfg.CircuitBreaker.Redis.Address,
        Password: cfg.CircuitBreaker.Redis.Password,
        DB:       cfg.CircuitBreaker.Redis.DB,
        CacheTTL: cfg.CircuitBreaker.Redis.CacheTTL,
    }))
}

// Create the circuit breaker
cb, err := circuitbreaker.NewCircuitBreaker(opts...)
```

## HTTP Middleware Integration

The circuit breaker can be used as HTTP middleware to protect endpoints:

```go
type Middleware struct {
    cb     ports.CircuitBreaker
    logger *log.Logger
}

// Handler wraps an HTTP handler with circuit breaker functionality
func (m *Middleware) Handler(circuitName string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        err := m.cb.Execute(r.Context(), circuitName, func() error {
            recorder := &statusRecorder{
                ResponseWriter: w,
                Status:         http.StatusOK,
            }

            next.ServeHTTP(recorder, r)

            if recorder.Status >= 500 {
                return fmt.Errorf("server error: %d", recorder.Status)
            }

            return nil
        })

        if err != nil {
            if err == ErrCircuitOpen {
                w.WriteHeader(http.StatusServiceUnavailable)
                w.Write([]byte("Service unavailable: circuit breaker is open"))
            } else {
                w.WriteHeader(http.StatusInternalServerError)
                w.Write([]byte("Internal server error occurred"))
            }
        }
    })
}
```

## Integration with Service Proxy

The circuit breaker is integrated with the Service Proxy to protect downstream service calls:

```go
// Apply circuit breaker middleware if enabled
if sp.circuitBreaker != nil {
    cbMiddleware := circuitbreaker.NewMiddleware(sp.circuitBreaker, sp.logger)
    handler = cbMiddleware.Handler(serviceName, handler)
}
```

This integration provides:

- Automatic protection for all service calls
- Service-specific circuit breaker instances
- Fast failure for unavailable services
- Gradual recovery testing with half-open state

## Application Integration

The circuit breaker is integrated at the application level through the `App` struct:

```go
type App struct {
    // Other fields...
    circuitBreaker ports.CircuitBreaker
    // Other fields...
}
```

During application startup (`App.Start()`), the circuit breaker is added to the service proxy:

```go
// Add circuit breaker if available
if a.circuitBreaker != nil {
    proxyOptions = append(proxyOptions, adaptersHTTP.WithCircuitBreaker(a.circuitBreaker))
}
```

## Metrics and Monitoring

Circuit breaker metrics are tracked for monitoring and diagnostic purposes:

```go
type CircuitBreakerMetrics struct {
    // Current state of the circuit breaker
    State CircuitBreakerState

    // Count of successful requests
    Successes int64

    // Count of failed requests
    Failures int64

    // Count of rejected requests due to open circuit
    Rejected int64

    // Count of timed out requests
    Timeouts int64

    // Timestamp of last state change
    LastStateChange time.Time
}
```

These metrics can be accessed through the `GetMetrics()` method and used for:

- Monitoring dashboards
- Alerting on circuit tripping
- Performance analysis
- Capacity planning

## Sequence Diagram

```
┌─────┐                  ┌─────────────┐               ┌──────────────┐          ┌────────────┐
│ App │                  │ Circuit     │               │ Service      │          │ Downstream │
│     │                  │ Breaker     │               │ Proxy        │          │ Service    │
└──┬──┘                  └──────┬──────┘               └──────┬───────┘          └─────┬──────┘
   │                            │                             │                        │
   │ Initialize                 │                             │                        │
   │ Circuit Breaker            │                             │                        │
   │ ─────────────────────────►│                             │                        │
   │                            │                             │                        │
   │ Add Circuit Breaker        │                             │                        │
   │ to Service Proxy           │                             │                        │
   │ ─────────────────────────────────────────────────────────►                        │
   │                            │                             │                        │
   │ Client Request             │                             │                        │
   │ ─────────────────────────────────────────────────────────►                        │
   │                            │                             │                        │
   │                            │      Execute With           │                        │
   │                            │◄────────────────────────────│                        │
   │                            │      Circuit Breaker        │                        │
   │                            │                             │                        │
   │                            │ Check Circuit State         │                        │
   │                            │ ───────────────────────────►│                        │
   │                            │                             │                        │
   │                            │ If Closed:                  │                        │
   │                            │ Forward Request             │                        │
   │                            │ ──────────────────────────────────────────────────────►
   │                            │                             │                        │
   │                            │                             │       Process          │
   │                            │                             │       Request          │
   │                            │                             │                        │
   │                            │                             │       Response         │
   │                            │                             │◄─────────────────────────
   │                            │                             │                        │
   │                            │ Record Success/Failure      │                        │
   │                            │ ───────────────────────────►│                        │
   │                            │                             │                        │
   │                            │ If Success:                 │                        │
   │                            │ Return Response             │                        │
   │                            │ ──────────────────────────────────────────────────────►
   │                            │                             │                        │
   │ Response                   │                             │                        │
   │◄────────────────────────────────────────────────────────────────────────────────────
   │                            │                             │                        │
   │                            │                             │                        │
   │                            │ If Failures > Threshold:    │                        │
   │                            │ Open Circuit                │                        │
   │                            │                             │                        │
   │ New Request                │                             │                        │
   │ ─────────────────────────────────────────────────────────►                        │
   │                            │                             │                        │
   │                            │ Check Circuit (Open)        │                        │
   │                            │◄────────────────────────────│                        │
   │                            │                             │                        │
   │                            │ Reject Request              │                        │
   │                            │ ───────────────────────────►│                        │
   │                            │                             │                        │
   │ 503 Service Unavailable    │                             │                        │
   │◄────────────────────────────────────────────────────────────────────────────────────
   │                            │                             │                        │
   │                            │                             │                        │
   │                            │ After Timeout Period:       │                        │
   │                            │ Half-Open Circuit           │                        │
   │                            │                             │                        │
   │ Probe Request              │                             │                        │
   │ ─────────────────────────────────────────────────────────►                        │
   │                            │                             │                        │
   │                            │ Allow Limited Requests      │                        │
   │                            │ ──────────────────────────────────────────────────────►
   │                            │                             │                        │
   │                            │                             │                        │
   │                            │ If Success Threshold Met:   │                        │
   │                            │ Close Circuit               │                        │
   │                            │                             │                        │
└─────┘                  └──────┴──────┘               └──────┴───────┘          └─────┴──────┘
```

## Best Practices

1. **Use appropriate thresholds**: Set failure thresholds based on normal error rates
2. **Configure suitable timeouts**: Balance between quick recovery and preventing premature recovery
3. **Service-specific circuits**: Use different circuit breakers for different services
4. **Fallback mechanisms**: Implement fallbacks for critical operations
5. **Monitoring**: Track circuit breaker metrics and set up alerts for circuit tripping
6. **Testing**: Include circuit breaker testing in chaos engineering practices
7. **Distributed state**: Use Redis implementation in multi-instance deployments

## Configuration Guidelines

| Parameter                | Recommended Value           | Description                       |
| ------------------------ | --------------------------- | --------------------------------- |
| Threshold                | 5-10                        | Failures before opening circuit   |
| Timeout                  | 5s-30s                      | How long circuit stays open       |
| HalfOpenSuccessThreshold | 2-5                         | Successes needed to close circuit |
| MaxConcurrentRequests    | Depends on service capacity | Prevents resource exhaustion      |
| RequestTimeout           | 1s-5s                       | Maximum time for a request        |

## Integration with Other Components

The circuit breaker works in conjunction with:

1. **Service Proxy**: Protects downstream service calls
2. **Service Discovery**: Circuits are named after services
3. **Logging**: Circuit state changes are logged
4. **Metrics**: Circuit metrics can be exported for monitoring
5. **Rate Limiting**: Complementary protection mechanism

This multi-layered approach provides comprehensive resilience against failures in distributed microservice architectures.
