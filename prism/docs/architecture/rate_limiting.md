# Rate Limiting Integration

This document explains the rate limiting system and its integration with the service proxy and other components in the system architecture.

## Architecture Overview

Rate limiting is a critical mechanism for controlling the flow of incoming requests to protect system resources, maintain performance, and prevent abuse. The rate limiting system consists of several key components:

1. **Rate Limiter Interface**: Defines common operations for all rate limiter implementations
2. **Memory Implementation**: In-memory rate limiter for single-instance deployments
3. **Redis Implementation**: Distributed rate limiter for multi-instance deployments
4. **HTTP Middleware**: Applies rate limiting to incoming requests
5. **Service Proxy Integration**: Protects downstream service calls
6. **Metrics Collection**: Monitors rate limiting activity for analysis

## Rate Limiter Interface

The rate limiter interface is defined in `internal/ports/ratelimiter.go`:

```go
type RateLimiter interface {
    // Allow checks if a request with the given key is allowed based on the rate limit
    Allow(key string) (bool, RateLimitInfo)

    // LimitMiddleware provides HTTP middleware for rate limiting
    LimitMiddleware(next http.Handler) http.Handler

    // Close cleans up the rate limiter resources
    Close() error
}

// RateLimitInfo contains information about the rate limit status
type RateLimitInfo struct {
    Limit     int
    Remaining int
    Reset     time.Time
    RetryAt   time.Time
}
```

## Rate Limiting Strategies

The system supports multiple rate limiting strategies to provide flexible control over different traffic patterns:

```go
type RateLimitStrategy string

const (
    StrategyIP     RateLimitStrategy = "ip"      // Limit by client IP address
    StrategyRoute  RateLimitStrategy = "route"   // Limit by route/endpoint
    StrategyToken  RateLimitStrategy = "token"   // Limit by authentication token
    StrategyCustom RateLimitStrategy = "custom"  // Limit by custom key generator
    StrategyAuth   RateLimitStrategy = "auth"    // Limit by authenticated user
)
```

## Configuration

Rate limiter configuration is managed through the `RateLimitConfig` struct and can be loaded from environment variables:

```go
type RateLimitConfig struct {
    // Number of requests allowed per second
    RequestsPerSecond int64 `yaml:"requests_per_second"`

    // Maximum burst size allowed
    Burst int `yaml:"burst"`

    // Duration for which rate limit records are kept
    TTL time.Duration `yaml:"ttl"`
}
```

Environment variable configuration:

```
USE_REDIS_RATE_LIMITER=true
RATE_LIMIT_REQUESTS_PER_SECOND=100
RATE_LIMIT_BURST=150
RATE_LIMIT_TTL=1h
```

## Memory Rate Limiter

The `MemoryRateLimiter` provides an in-memory implementation using Go's standard rate limiting package:

```go
type MemoryRateLimiter struct {
    config   *RateLimiterParams
    limiters sync.Map
    metrics  *rateLimitMetrics
}
```

Key features:

- Thread-safe operation with sync.Map
- Token bucket algorithm implementation
- Dynamic creation of limiters per key
- Configurable limits and burst sizes
- Local memory storage (not shared across instances)

## Redis Rate Limiter

The `RedisRateLimiter` provides a distributed implementation using Redis for state storage:

```go
type RedisRateLimiter struct {
    config  *RateLimiterParams
    client  *redis.Client
    metrics *rateLimitMetrics
}
```

Key features:

- Distributed state management using Redis
- Atomic operations via Lua scripts
- Consistent rate limiting across multiple service instances
- Automatic TTL expiration for rate limit records
- Resilience to Redis connection issues

## HTTP Middleware Integration

The rate limiter is implemented as HTTP middleware that can be inserted into the request processing pipeline:

```go
func rateLimitMiddleware(config *RateLimiterParams, limiter ports.RateLimiter, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Check if path should be excluded
        for _, path := range config.ExcludedPaths {
            if strings.HasPrefix(r.URL.Path, path) {
                next.ServeHTTP(w, r)
                return
            }
        }

        // Generate key based on strategy
        key, err := generateKey(config, r)
        if err != nil {
            http.Error(w, "Rate limit key generation failed", http.StatusInternalServerError)
            return
        }

        // Check if request is allowed
        allowed, info := limiter.Allow(key)

        // Add rate limit headers if configured
        if config.Headers {
            setRateLimitHeaders(w, info)
        }

        if !allowed {
            w.WriteHeader(http.StatusTooManyRequests)
            w.Write([]byte("Rate limit exceeded"))
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

## Key Generation

The system generates rate limit keys based on the selected strategy:

```go
func generateKey(config *RateLimiterParams, r *http.Request) (string, error) {
    prefix := "ratelimit:"

    switch config.Strategy {
    case StrategyIP:
        return prefix + "ip:" + getClientIP(r), nil

    case StrategyRoute:
        return prefix + "route:" + chi.RouteContext(r.Context()).RoutePattern(), nil

    case StrategyToken:
        token := r.Header.Get("Authorization")
        if token == "" {
            return prefix + "ip:" + getClientIP(r), nil
        }
        return prefix + "token:" + token, nil

    case StrategyAuth:
        userID := getUserIDFromContext(r.Context())
        if userID == "" {
            return prefix + "ip:" + getClientIP(r), nil
        }
        return prefix + "user:" + userID, nil

    case StrategyCustom:
        if config.KeyGen == nil {
            return "", ErrInvalidStrategy
        }
        key, err := config.KeyGen(r)
        if err != nil {
            return "", err
        }
        return prefix + "custom:" + key, nil

    default:
        return "", ErrInvalidStrategy
    }
}
```

## Response Headers

The rate limiter can add informational headers to responses:

```go
func setRateLimitHeaders(w http.ResponseWriter, info ports.RateLimitInfo) {
    w.Header().Set(RateLimitHeader, fmt.Sprintf("%d", info.Limit))
    w.Header().Set(RateLimitRemaining, fmt.Sprintf("%d", info.Remaining))
    w.Header().Set(RateLimitReset, fmt.Sprintf("%d", info.Reset.Unix()))
}
```

Headers include:

- `X-RateLimit-Limit`: The maximum number of requests allowed
- `X-RateLimit-Remaining`: The number of requests remaining in the current window
- `X-RateLimit-Reset`: The time when the rate limit window resets (Unix timestamp)

## Metrics and Monitoring

The rate limiter collects metrics for monitoring and analysis:

```go
type rateLimitMetrics struct {
    requests *prometheus.CounterVec
    blocked  *prometheus.CounterVec
    latency  *prometheus.HistogramVec
}
```

Metrics include:

- Total number of requests processed
- Number of requests blocked by rate limiting
- Response latency for rate limiting operations
- Breakdown by strategy and status

## Service Registration Integration

Individual service routes can have their own rate limits:

```go
type APIRoute struct {
    // ... other fields ...
    RateLimitPerMinute int `json:"rate_limit_per_minute,omitempty"`
}
```

This allows for:

- Service-specific rate limiting
- Different limits for different endpoints
- Dynamic configuration through the API

## Application Integration

The rate limiter is integrated at the application level through the `App` struct:

```go
type App struct {
    // Other fields...
    rateLimiter ports.RateLimiter
    // Other fields...
}

// WithRateLimiter sets a custom rate limiter for the App
func WithRateLimiter(rateLimiter ports.RateLimiter) AppOption {
    return func(a *App) error {
        if rateLimiter == nil {
            return ErrNilRateLimiter
        }
        a.rateLimiter = rateLimiter
        return nil
    }
}
```

## Initialization

The rate limiter is initialized in the application's main function:

```go
// Initialize rate limiter
rateLimiterParams := ratelimiter.RateLimiterConfigToParams(&cfg.RateLimit)
rateLimiterParams.RedisClient = redisClient
rateLimiterParams.Strategy = ratelimiter.StrategyIP
rateLimiterParams.Headers = true
rateLimiterParams.ExcludedPaths = []string{"/health", "/metrics"}

rateLimiter, err := ratelimiter.NewRedisRateLimiter(rateLimiterParams)
if err != nil {
    logger.Fatal("Failed to create redis rate limiter", log.Error(err))
}

// Create app with rate limiter
app, err := internal.NewApp(
    // ... other options ...
    app.WithRateLimiter(rateLimiter),
)
```

## Sequence Diagram

```
┌─────┐                  ┌─────────────┐               ┌──────────────┐          ┌────────────┐
│ App │                  │ Rate        │               │ HTTP         │          │ Service    │
│     │                  │ Limiter     │               │ Server       │          │ Handler    │
└──┬──┘                  └──────┬──────┘               └──────┬───────┘          └─────┬──────┘
   │                            │                             │                        │
   │ Initialize                 │                             │                        │
   │ Rate Limiter               │                             │                        │
   │ ─────────────────────────►│                             │                        │
   │                            │                             │                        │
   │ Register Rate Limiter      │                             │                        │
   │ Middleware                 │                             │                        │
   │ ────────────────────────────────────────────────────────►│                        │
   │                            │                             │                        │
   │ Client Request             │                             │                        │
   │ ────────────────────────────────────────────────────────►│                        │
   │                            │                             │                        │
   │                            │      Allow Request?         │                        │
   │                            │◄────────────────────────────│                        │
   │                            │                             │                        │
   │                            │ Generate Key                │                        │
   │                            │ (IP/Route/Token)            │                        │
   │                            │                             │                        │
   │                            │ Check Rate Limit            │                        │
   │                            │ for Key                     │                        │
   │                            │                             │                        │
   │                            │ Add Rate Limit              │                        │
   │                            │ Headers                     │                        │
   │                            │                             │                        │
   │                            │ Allow/Reject                │                        │
   │                            │ ───────────────────────────►│                        │
   │                            │                             │                        │
   │                            │ If Allowed:                 │                        │
   │                            │ Forward Request             │                        │
   │                            │ ──────────────────────────────────────────────────────►
   │                            │                             │                        │
   │                            │                             │       Process          │
   │                            │                             │       Request          │
   │                            │                             │                        │
   │                            │                             │       Response         │
   │                            │                             │◄─────────────────────────
   │                            │                             │                        │
   │                            │                             │ Return Response        │
   │                            │                             │ with Rate Limit        │
   │                            │                             │ Headers                │
   │                            │                             │                        │
   │ Response                   │                             │                        │
   │◄────────────────────────────────────────────────────────────────────────────────────
   │                            │                             │                        │
   │                            │                             │                        │
   │                            │ If Rate Limit Exceeded:     │                        │
   │                            │ Return 429 Too Many Requests│                        │
   │                            │                             │                        │
   │ 429 Too Many Requests      │                             │                        │
   │◄────────────────────────────────────────────────────────────────────────────────────
   │                            │                             │                        │
   │                            │                             │                        │
└─────┘                  └──────┴──────┘               └──────┴───────┘          └─────┴──────┘
```

## Integration with Other Components

The rate limiting system works in conjunction with:

1. **Service Proxy**: Adds rate limiting to service requests
2. **Authentication**: Used for user-based rate limiting
3. **Circuit Breaker**: Provides complementary protection
4. **Metrics**: Rate limiting metrics are exported for monitoring
5. **API Management**: Routes can have individual rate limits

## Best Practices

1. **Use appropriate strategies**: Choose rate limiting strategies based on your security and fairness requirements
2. **Distributed rate limiting**: Use Redis implementation in multi-instance deployments
3. **Tiered rate limits**: Apply different limits for authenticated vs. unauthenticated users
4. **Exclude health checks**: Always exclude health and metrics endpoints
5. **Informative responses**: Provide retry-after headers when possible
6. **Monitor rate limits**: Alert on unusual patterns of rate limit hits
7. **Emergency controls**: Have mechanisms to adjust rate limits dynamically

## Configuration Guidelines

| Parameter         | Recommended Value                           | Description                       |
| ----------------- | ------------------------------------------- | --------------------------------- |
| RequestsPerSecond | 10-1000 (varies)                            | Base requests per second limit    |
| Burst             | 1.5-3x RequestsPerSecond                    | Allowed burst capacity            |
| TTL               | 10m-1h                                      | How long rate limit data persists |
| Strategy          | IP for public APIs, Token for authenticated | Limiting strategy                 |
| Headers           | true                                        | Include rate limit headers        |

## API Route Rate Limiting

Individual API routes can have their own rate limit settings:

```sql
-- API Routes Table
CREATE TABLE IF NOT EXISTS api_routes (
    -- ... other fields ...
    rate_limit_per_minute INTEGER,
    -- ... other fields ...
);
```

This allows for fine-grained control at the route level, which can be used to:

- Apply stricter limits to expensive operations
- Provide higher limits for critical endpoints
- Implement tiered access based on subscription levels
- Protect vulnerable endpoints from abuse

## Conclusion

The rate limiting system provides a robust defense against traffic spikes, abusive behavior, and resource exhaustion. By integrating at multiple levels and supporting different strategies, it offers flexible protection while maintaining service availability for legitimate users.
