package ratelimiter

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jasonKoogler/aegis/internal/config"
	"github.com/jasonKoogler/aegis/internal/ports"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

var (
	ErrRateLimitExceeded  = fmt.Errorf("rate limit exceeded")
	ErrInvalidStrategy    = fmt.Errorf("invalid rate limit strategy")
	ErrMissingRedisClient = fmt.Errorf("redis client is required for Redis rate limiter")
	ErrInvalidConfig      = fmt.Errorf("invalid rate limiter configuration")
)

// RateLimitStrategy defines the strategy for rate limiting
type RateLimitStrategy string

const (
	StrategyIP     RateLimitStrategy = "ip"
	StrategyRoute  RateLimitStrategy = "route"
	StrategyToken  RateLimitStrategy = "token"
	StrategyCustom RateLimitStrategy = "custom"
	StrategyAuth   RateLimitStrategy = "auth"
)

const (
	RateLimitHeader    = "X-RateLimit-Limit"
	RateLimitRemaining = "X-RateLimit-Remaining"
	RateLimitReset     = "X-RateLimit-Reset"
)

func RateLimiterConfigToParams(cfg *config.RateLimitConfig) *RateLimiterParams {
	return &RateLimiterParams{
		Limit: rate.Limit(cfg.RequestsPerSecond),
		Burst: cfg.Burst,
		TTL:   cfg.TTL,
	}
}

// RateLimiterParams holds the configuration for the rate limiter
// RateLimiterParams encapsulates all configuration parameters required to initialize and manage a rate limiter.
type RateLimiterParams struct {
	// Limit defines the maximum average number of requests allowed per second.
	// It uses the rate.Limit type from the rate package for precise control over the allowed rate.
	Limit rate.Limit

	// Burst specifies the maximum number of requests that can be processed in a short burst.
	// This allows temporary exceeding of the average limit defined by Limit, accommodating sudden spikes in traffic.
	Burst int

	// TTL (Time-To-Live) represents the duration for which the rate limiter's state is preserved.
	// This is especially useful in distributed environments where state needs to persist for a period before expiring.
	TTL time.Duration

	// Strategy indicates the strategy used to distinguish between different clients or endpoints.
	// For example, it might limit based on the client's IP address, the requested route, an authentication token, or a custom criteria.
	Strategy RateLimitStrategy

	// RedisClient is a pointer to a Redis client used for persisting rate limiting state.
	// It is required when using a distributed rate limiting strategy that leverages Redis for storage.
	RedisClient *redis.Client

	// KeyGen is a function used to generate a unique key from an HTTP request.
	// This key is used to identify and track the rate limiting counters for custom strategies.
	// It should return a string key and an error if the key generation fails.
	KeyGen func(*http.Request) (string, error)

	// ExcludedPaths lists the URL paths that should bypass the rate limiter.
	// Requests matching any of these paths will not be subjected to rate limiting checks.
	ExcludedPaths []string

	// Headers determines whether to include rate limiting information in the response headers.
	// When enabled, headers such as rate limit, remaining quota, and reset time are added to responses.
	Headers bool
}

func (c *RateLimiterParams) Validate() error {
	if c.Limit <= 0 {
		return fmt.Errorf("%w: limit must be positive", ErrInvalidConfig)
	}
	if c.Burst <= 0 {
		return fmt.Errorf("%w: burst must be positive", ErrInvalidConfig)
	}
	if c.Strategy == StrategyCustom && c.KeyGen == nil {
		return fmt.Errorf("%w: KeyGen function required for custom strategy", ErrInvalidConfig)
	}
	if c.Strategy == "" {
		c.Strategy = StrategyIP // Set default strategy
	}
	return nil
}

// rateLimitMetrics contains Prometheus metrics for the rate limiter
type rateLimitMetrics struct {
	requests *prometheus.CounterVec
	blocked  *prometheus.CounterVec
	latency  *prometheus.HistogramVec
}

func newRateLimitMetrics() *rateLimitMetrics {
	m := &rateLimitMetrics{
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rate_limit_requests_total",
				Help: "Total number of requests handled by rate limiter",
			},
			[]string{"strategy", "status"},
		),
		blocked: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rate_limit_blocked_total",
				Help: "Total number of requests blocked by rate limiter",
			},
			[]string{"strategy"},
		),
		latency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rate_limit_latency_seconds",
				Help:    "Rate limiter operation latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"strategy"},
		),
	}
	prometheus.MustRegister(m.requests, m.blocked, m.latency)
	return m
}

// MemoryRateLimiter implements in-memory rate limiting
type MemoryRateLimiter struct {
	config   *RateLimiterParams
	limiters sync.Map
	metrics  *rateLimitMetrics
}

// NewMemoryRateLimiter creates a new in-memory rate limiter
func NewMemoryRateLimiter(params *RateLimiterParams) (*MemoryRateLimiter, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	return &MemoryRateLimiter{
		config:  params,
		metrics: newRateLimitMetrics(),
	}, nil
}

// Allow checks if a request is allowed based on the rate limit
func (m *MemoryRateLimiter) Allow(key string) (bool, ports.RateLimitInfo) {
	start := time.Now()
	defer func() {
		m.metrics.latency.WithLabelValues("memory").Observe(time.Since(start).Seconds())
	}()

	limiterInterface, _ := m.limiters.LoadOrStore(key, rate.NewLimiter(m.config.Limit, m.config.Burst))
	limiter := limiterInterface.(*rate.Limiter)

	allowed := limiter.Allow()
	info := ports.RateLimitInfo{
		Limit:     m.config.Burst,
		Remaining: int(limiter.Tokens()),
		Reset:     time.Now().Add(time.Second),
	}

	if !allowed {
		info.RetryAt = time.Now().Add(time.Second / time.Duration(m.config.Limit))
		m.metrics.blocked.WithLabelValues("memory").Inc()
	}

	m.metrics.requests.WithLabelValues("memory", fmt.Sprintf("%t", allowed)).Inc()
	return allowed, info
}

// LimitMiddleware provides the middleware implementation
func (m *MemoryRateLimiter) LimitMiddleware(next http.Handler) http.Handler {
	return rateLimitMiddleware(m.config, m, next)
}

// Close cleans up the rate limiter resources
func (m *MemoryRateLimiter) Close() error {
	m.limiters.Range(func(key, value interface{}) bool {
		m.limiters.Delete(key)
		return true
	})
	return nil
}

// RedisRateLimiter implements Redis-based rate limiting
type RedisRateLimiter struct {
	config  *RateLimiterParams
	client  *redis.Client
	metrics *rateLimitMetrics
}

// NewRedisRateLimiter creates a new Redis-based rate limiter
func NewRedisRateLimiter(config *RateLimiterParams) (*RedisRateLimiter, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if config.RedisClient == nil {
		return nil, ErrMissingRedisClient
	}

	return &RedisRateLimiter{
		config:  config,
		client:  config.RedisClient,
		metrics: newRateLimitMetrics(),
	}, nil
}

// Allow checks if a request is allowed based on the rate limit using Redis
func (r *RedisRateLimiter) Allow(key string) (bool, ports.RateLimitInfo) {
	start := time.Now()
	defer func() {
		r.metrics.latency.WithLabelValues("redis").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	script := redis.NewScript(`
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local burst = tonumber(ARGV[2])
		local ttl = tonumber(ARGV[3])

		local current = tonumber(redis.call("GET", key) or "0")
		if current + 1 > burst then
			return {0, current}
		else
			current = redis.call("INCR", key)
			if current == 1 then
				redis.call("EXPIRE", key, ttl)
			end
			return {1, current}
		end
	`)

	result, err := script.Run(ctx, r.client, []string{key}, r.config.Limit, r.config.Burst, int(r.config.TTL.Seconds())).Result()
	if err != nil {
		r.metrics.requests.WithLabelValues("redis", "error").Inc()
		// Allow the request in case of Redis errors
		return true, ports.RateLimitInfo{
			Limit:     r.config.Burst,
			Remaining: r.config.Burst,
			Reset:     time.Now().Add(r.config.TTL),
		}
	}

	values, ok := result.([]interface{})
	if !ok || len(values) < 2 {
		r.metrics.requests.WithLabelValues("redis", "error").Inc()
		return true, ports.RateLimitInfo{
			Limit:     r.config.Burst,
			Remaining: r.config.Burst,
			Reset:     time.Now().Add(r.config.TTL),
		}
	}

	allowed := values[0].(int64) == 1
	current := values[1].(int64)

	info := ports.RateLimitInfo{
		Limit:     r.config.Burst,
		Remaining: r.config.Burst - int(current),
		Reset:     time.Now().Add(r.config.TTL),
	}

	if !allowed {
		info.RetryAt = time.Now().Add(time.Duration(1/float64(r.config.Limit)) * time.Second)
		r.metrics.blocked.WithLabelValues("redis").Inc()
	}

	r.metrics.requests.WithLabelValues("redis", fmt.Sprintf("%t", allowed)).Inc()
	return allowed, info
}

// LimitMiddleware provides the middleware implementation
func (r *RedisRateLimiter) LimitMiddleware(next http.Handler) http.Handler {
	return rateLimitMiddleware(r.config, r, next)
}

// Close cleans up the rate limiter resources
func (r *RedisRateLimiter) Close() error {
	// Redis client should be closed by the owner if necessary
	return nil
}

// rateLimitMiddleware is a helper function to create rate limiting middleware
func rateLimitMiddleware(config *RateLimiterParams, limiter ports.RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for excluded paths
		for _, path := range config.ExcludedPaths {
			if r.URL.Path == path {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Generate key based on strategy
		key, err := generateKey(config, r)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Check rate limit
		allowed, info := limiter.Allow(key)

		// Set rate limit headers if enabled
		if config.Headers {
			setRateLimitHeaders(w, info)
		}

		if allowed {
			next.ServeHTTP(w, r)
		} else {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(info.RetryAt.Sub(time.Now()).Seconds())))
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		}
	})
}

// generateKey generates a key for rate limiting based on the strategy
func generateKey(config *RateLimiterParams, r *http.Request) (string, error) {
	switch config.Strategy {
	case StrategyIP:
		return getClientIP(r), nil
	case StrategyRoute:
		return chi.RouteContext(r.Context()).RoutePattern(), nil
	case StrategyToken:
		token := r.Header.Get("Authorization")
		if token == "" {
			return "", fmt.Errorf("missing authorization header")
		}
		return token, nil
	case StrategyCustom:
		if config.KeyGen == nil {
			return "", fmt.Errorf("missing key generation function")
		}
		return config.KeyGen(r)
	case StrategyAuth:
		userID, ok := r.Context().Value("userID").(string)
		if !ok {
			return "", fmt.Errorf("missing userID in context")
		}
		return fmt.Sprintf("auth:%s", userID), nil
	default:
		return "", ErrInvalidStrategy
	}
}

// setRateLimitHeaders sets rate limit headers in the response
func setRateLimitHeaders(w http.ResponseWriter, info ports.RateLimitInfo) {
	w.Header().Set(RateLimitHeader, fmt.Sprintf("%d", info.Limit))
	w.Header().Set(RateLimitRemaining, fmt.Sprintf("%d", info.Remaining))
	w.Header().Set(RateLimitReset, fmt.Sprintf("%d", info.Reset.Unix()))
}

// getClientIP retrieves the client's IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		return strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
	}
	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}
	// Fall back to RemoteAddr
	ipPort := strings.Split(r.RemoteAddr, ":")
	if len(ipPort) > 0 {
		return ipPort[0]
	}
	return ""
}
