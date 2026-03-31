package circuitbreaker

import "errors"

// Error definitions for the circuitbreaker package
var (
	// Configuration errors
	ErrNilConfig      = errors.New("config cannot be nil")
	ErrNilLogger      = errors.New("logger cannot be nil")
	ErrNilRedisConfig = errors.New("redis config cannot be nil")
	ErrNilRedisClient = errors.New("redis client cannot be nil")

	// Circuit breaker state errors - already defined in memory.go
	// ErrCircuitOpen and ErrTooManyConcurrentRequests are already defined
	ErrTimeoutExceeded         = errors.New("timeout exceeded")
	ErrFailureThresholdReached = errors.New("failure threshold reached")

	// Redis errors
	ErrRedisConnection = errors.New("failed to connect to redis")
	ErrAcquireLock     = errors.New("failed to acquire lock")
	ErrReleaseLock     = errors.New("failed to release lock")
)
