package circuitbreaker

import (
	"github.com/jasonKoogler/abraxis/prism/internal/common/log"
	"github.com/jasonKoogler/abraxis/prism/internal/config"
	"github.com/jasonKoogler/abraxis/prism/internal/ports"
	"github.com/redis/go-redis/v9"
)

// Option defines a functional option for configuring CircuitBreaker
type Option func(*builder) error

// builder is a builder for creating circuit breakers
type builder struct {
	config      *config.CircuitBreakerConfig
	logger      *log.Logger
	redisConfig *CircuitBreakerRedisConfig
	redisClient *redis.Client
}

// NewBuilder creates a new Builder
func NewBuilder() *builder {
	return &builder{}
}

// WithConfig sets the configuration for the CircuitBreaker
func WithConfig(cfg *config.CircuitBreakerConfig) Option {
	return func(b *builder) error {
		if cfg == nil {
			return ErrNilConfig
		}
		b.config = cfg
		return nil
	}
}

// WithLogger sets the logger for the CircuitBreaker
func WithLogger(logger *log.Logger) Option {
	return func(b *builder) error {
		if logger == nil {
			return ErrNilLogger
		}
		b.logger = logger
		return nil
	}
}

// WithRedisConfig sets the Redis configuration for the CircuitBreaker
func WithRedisConfig(redisConfig *CircuitBreakerRedisConfig) Option {
	return func(b *builder) error {
		if redisConfig == nil {
			return ErrNilRedisConfig
		}
		b.redisConfig = redisConfig
		return nil
	}
}

// WithRedisClient sets a custom Redis client for the CircuitBreaker
func WithRedisClient(client *redis.Client) Option {
	return func(b *builder) error {
		if client == nil {
			return ErrNilRedisClient
		}
		b.redisClient = client
		return nil
	}
}

// Build creates a new CircuitBreaker based on the builder configuration
func (b *builder) Build() (ports.CircuitBreaker, error) {
	// Validate required fields
	if b.config == nil {
		return nil, ErrNilConfig
	}

	if b.logger == nil {
		return nil, ErrNilLogger
	}

	// Create the appropriate circuit breaker based on the provider
	if b.config.Provider == "redis" {
		// Redis-based circuit breaker
		if b.redisConfig == nil {
			// If Redis configuration is not provided, use the one from the main config
			if b.config.Redis == nil {
				return nil, ErrNilRedisConfig
			}

			b.redisConfig = &CircuitBreakerRedisConfig{
				Address:  b.config.Redis.Address,
				Password: b.config.Redis.Password,
				DB:       b.config.Redis.DB,
				CacheTTL: b.config.Redis.CacheTTL,
			}
		}

		return NewRedisCircuitBreaker(b.config, b.redisConfig, b.logger)
	}

	// Default to memory-based circuit breaker
	cb := NewMemoryCircuitBreaker(b.config, b.logger)
	return cb, nil
}

// New creates a new CircuitBreaker with the provided options
func New(opts ...Option) (ports.CircuitBreaker, error) {
	builder := NewBuilder()

	// Apply provided options
	for _, opt := range opts {
		if err := opt(builder); err != nil {
			return nil, err
		}
	}

	return builder.Build()
}
