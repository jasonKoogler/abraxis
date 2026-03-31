package ports

import (
	"net/http"
	"time"
)

// RateLimiter is an interface for rate limiting implementations
type RateLimiter interface {
	Allow(key string) (bool, RateLimitInfo)
	LimitMiddleware(next http.Handler) http.Handler
	Close() error
}

// RateLimitInfo contains information about the rate limit status
type RateLimitInfo struct {
	Limit     int
	Remaining int
	Reset     time.Time
	RetryAt   time.Time
}
