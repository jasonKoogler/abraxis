# Rate Limiter Adapter

## Overview

The Rate Limiter adapter provides mechanisms to control the rate of incoming requests to your application. It helps protect your APIs from abuse, prevents resource exhaustion, and ensures fair usage by limiting the number of requests that clients can make within a specified time period.

The adapter supports both in-memory and Redis-based rate limiting, with the latter providing distributed rate limiting suitable for clustered environments.

## Port Interface

The adapter implements the `RateLimiter` interface from the `ports` package:

```go
type RateLimiter interface {
    // Allow checks if a request should be allowed based on the rate limit
    // Returns whether the request is allowed and information about the rate limit
    Allow(key string) (bool, RateLimitInfo)

    // LimitMiddleware returns an HTTP middleware that applies rate limiting
    LimitMiddleware(next http.Handler) http.Handler

    // Close releases any resources used by the rate limiter
    Close() error
}

// RateLimitInfo contains information about the rate limit status
type RateLimitInfo struct {
    Limit     int       // Maximum number of requests allowed
    Remaining int       // Number of requests remaining in the current window
    Reset     time.Time // When the rate limit window resets
    RetryAt   time.Time // When the client should retry if rate limited
}
```

## Rate Limiting Strategies

The adapter supports multiple strategies for identifying and grouping requests:

1. **IP-based** (`StrategyIP`) - Limits requests by client IP address
2. **Route-based** (`StrategyRoute`) - Limits requests to specific API routes/endpoints
3. **Token-based** (`StrategyToken`) - Limits requests by authentication token
4. **User-based** (`StrategyAuth`) - Limits requests by authenticated user ID
5. **Custom** (`StrategyCustom`) - Custom key generation for specialized rate limiting needs

## Key Components

### MemoryRateLimiter

An in-memory implementation suitable for single-instance deployments:

- Thread-safe with a synchronized map
- Efficient token bucket algorithm implementation
- No external dependencies
- Fast local operation
- Perfect for development and small to medium deployments

### RedisRateLimiter

A distributed implementation using Redis for state persistence:

- Suitable for clustered environments
- Ensures consistent rate limiting across instances
- Uses Redis Lua scripts for atomic operations
- Graceful fallback in case of Redis failures
- Ideal for production and high-scale applications

### Middleware Integration

HTTP middleware for easy integration with web handlers:

- Seamlessly integrates with Go's standard HTTP handlers
- Automatically sets rate limit headers for client awareness
- Configurable path exclusions for bypass routes
- Returns standard 429 (Too Many Requests) status with Retry-After header

## Configuration Parameters

The adapter is configured using the `RateLimiterParams` structure:

```go
type RateLimiterParams struct {
    // Limit defines the maximum average number of requests allowed per second
    Limit rate.Limit

    // Burst specifies the maximum number of requests that can be processed in a short burst
    Burst int

    // TTL represents the duration for which the rate limiter's state is preserved
    TTL time.Duration

    // Strategy indicates the strategy used to distinguish between clients or endpoints
    Strategy RateLimitStrategy

    // RedisClient is a pointer to a Redis client used for persisting rate limiting state
    RedisClient *redis.Client

    // KeyGen is a function used to generate a unique key from an HTTP request
    KeyGen func(*http.Request) (string, error)

    // ExcludedPaths lists the URL paths that should bypass the rate limiter
    ExcludedPaths []string

    // Headers determines whether to include rate limiting information in response headers
    Headers bool
}
```

## Usage Examples

### Creating a Memory-Based Rate Limiter

```go
// Create an in-memory rate limiter with 10 requests per second, burst of 20
limiter, err := ratelimiter.NewMemoryRateLimiter(&ratelimiter.RateLimiterParams{
    Limit:    rate.Limit(10),  // 10 requests per second
    Burst:    20,              // Allow bursts of up to 20 requests
    TTL:      time.Minute,     // State preservation time
    Strategy: ratelimiter.StrategyIP,
    Headers:  true,            // Include rate limit headers in responses
})
```

### Creating a Redis-Based Rate Limiter

```go
// Create a Redis-based rate limiter
limiter, err := ratelimiter.NewRedisRateLimiter(&ratelimiter.RateLimiterParams{
    Limit:       rate.Limit(5),    // 5 requests per second
    Burst:       10,               // Allow bursts of up to 10 requests
    TTL:         time.Minute,      // State preservation time
    Strategy:    ratelimiter.StrategyToken,
    Headers:     true,
    RedisClient: redisClient,      // Redis client instance
})
```

### Using Custom Key Generation

```go
// Create a rate limiter with custom key generation
limiter, err := ratelimiter.NewMemoryRateLimiter(&ratelimiter.RateLimiterParams{
    Limit:    rate.Limit(10),
    Burst:    20,
    TTL:      time.Minute,
    Strategy: ratelimiter.StrategyCustom,
    KeyGen: func(r *http.Request) (string, error) {
        // Custom logic to generate a key
        apiKey := r.Header.Get("X-API-Key")
        if apiKey == "" {
            return "", fmt.Errorf("missing API key")
        }
        // Rate limit by API key prefix (first 8 chars)
        return fmt.Sprintf("apikey:%s", apiKey[:8]), nil
    },
})
```

### Applying Rate Limiting Middleware

```go
// Create a router
r := chi.NewRouter()

// Apply rate limiting middleware to all routes
r.Use(limiter.LimitMiddleware)

// Or apply to specific routes
r.Group(func(r chi.Router) {
    r.Use(limiter.LimitMiddleware)
    r.Get("/api/users", userHandler)
    r.Post("/api/users", createUserHandler)
})
```

### Manual Rate Limiting

```go
// For non-HTTP use cases, use Allow directly
allowed, info := limiter.Allow("custom-key")
if !allowed {
    fmt.Printf("Rate limit exceeded. Try again in %v\n",
        time.Until(info.RetryAt))
    return
}

// Continue with the operation
processRequest()
```

## Rate Limit Headers

When the `Headers` option is enabled, the middleware adds the following headers to responses:

- `X-RateLimit-Limit` - Maximum number of requests allowed in the current window
- `X-RateLimit-Remaining` - Number of requests remaining in the current window
- `X-RateLimit-Reset` - Unix timestamp when the rate limit window resets

When a request is rate-limited, an additional header is included:

- `Retry-After` - Seconds to wait before retrying

## Error Handling

The rate limiter handles errors gracefully:

- Redis connection issues result in allowing requests rather than blocking them
- Invalid configuration is caught during initialization
- Missing tokens or user IDs in context result in appropriate error responses

## Metrics

The rate limiter includes Prometheus metrics for monitoring:

- `rate_limit_requests_total` - Total number of requests handled by rate limiter
- `rate_limit_blocked_total` - Total number of requests blocked by rate limiter
- `rate_limit_latency_seconds` - Rate limiter operation latency in seconds

These metrics include labels for the strategy and outcome (allowed/blocked).

## Path Exclusions

The `ExcludedPaths` parameter allows specific paths to bypass rate limiting:

```go
limiter, err := ratelimiter.NewMemoryRateLimiter(&ratelimiter.RateLimiterParams{
    // ... other parameters
    ExcludedPaths: []string{
        "/health",
        "/metrics",
        "/static/*",
    },
})
```

Requests to these paths will not be rate-limited, ensuring that monitoring and static assets remain accessible even during heavy traffic.

## Performance Considerations

### Memory Implementation

- Very low overhead, suitable for high-throughput systems
- No network calls
- Uses Go's efficient token bucket algorithm implementation
- Scales with number of unique keys (IP addresses, users, etc.)

### Redis Implementation

- Higher latency due to network calls
- Uses Redis Lua scripting for atomic operations
- Suitable for distributed systems where state must be shared
- Includes fallback mechanism for Redis failures

## Integration with App

The rate limiter can be integrated with the application's HTTP handlers:

```go
// Create a rate limiter
limiter, err := ratelimiter.NewMemoryRateLimiter(&ratelimiterParams)
if err != nil {
    log.Fatal(err)
}

// Apply middleware to your HTTP server
http.Handle("/api/", limiter.LimitMiddleware(apiHandler))
```

## Security Considerations

- Rate limiting is an essential security measure to protect against brute force attacks
- IP-based limiting can be bypassed with proxies; consider combining strategies
- Token-based limiting is more reliable but requires authentication
- For high-security applications, combine rate limiting with request validation, CAPTCHA, and other security measures
