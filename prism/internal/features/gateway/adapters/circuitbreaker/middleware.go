package circuitbreaker

import (
	"fmt"
	"net/http"

	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/ports"
)

// Middleware wraps an HTTP handler with circuit breaker functionality
type Middleware struct {
	cb     ports.CircuitBreaker
	logger *log.Logger
}

// NewMiddleware creates a new circuit breaker middleware
func NewMiddleware(cb ports.CircuitBreaker, logger *log.Logger) *Middleware {
	return &Middleware{
		cb:     cb,
		logger: logger,
	}
}

// Handler wraps an HTTP handler with circuit breaker functionality
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract service name from context, or derive from URL path
		serviceName := serviceNameFromRequest(r)

		// Create a response wrapper to catch status code
		rw := &responseWriter{ResponseWriter: w}

		// Execute the request through the circuit breaker
		err := m.cb.ExecuteWithFallback(
			r.Context(),
			serviceName,
			func() error {
				// Call the next handler in the chain
				next.ServeHTTP(rw, r)

				// Consider non-2xx status codes as errors
				if rw.statusCode >= 500 {
					return fmt.Errorf("service returned status %d", rw.statusCode)
				}
				return nil
			},
			func(err error) error {
				// Fallback handler when circuit is open or error occurs
				code := http.StatusServiceUnavailable
				if err == ErrCircuitOpen {
					m.logger.Warn("Circuit open, rejecting request",
						log.String("service", serviceName),
						log.String("path", r.URL.Path))
				} else if err == ErrTooManyConcurrentRequests {
					code = http.StatusTooManyRequests
					m.logger.Warn("Too many concurrent requests",
						log.String("service", serviceName),
						log.String("path", r.URL.Path))
				} else {
					m.logger.Error("Service error",
						log.String("service", serviceName),
						log.String("path", r.URL.Path),
						log.Error(err))
				}

				w.WriteHeader(code)
				w.Write([]byte(`{"error":"Service temporarily unavailable"}`))
				return nil
			},
		)

		if err != nil {
			m.logger.Error("Unexpected error in circuit breaker execution", log.Error(err))
		}
	})
}

// HandlerFunc wraps an HTTP handler function with circuit breaker functionality
func (m *Middleware) HandlerFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.Handler(next).ServeHTTP(w, r)
	}
}

// responseWriter is a wrapper around http.ResponseWriter that captures the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code and passes it to the wrapped ResponseWriter
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures a default status code of 200 if not explicitly set
func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// serviceNameFromRequest extracts the service name from the request
func serviceNameFromRequest(r *http.Request) string {
	// First try to get from context using the defined key
	if contextService, ok := r.Context().Value(ServiceNameContextKey).(string); ok && contextService != "" {
		return contextService
	}

	// Otherwise, derive from URL path
	path := r.URL.Path
	if len(path) <= 1 {
		return "default"
	}

	// Assuming the first segment is the service name
	// e.g., /users/123 => "users"
	segments := splitPath(path)
	if len(segments) > 0 {
		return segments[0]
	}

	return "default"
}

// splitPath splits a URL path into segments, removing empty segments
func splitPath(path string) []string {
	var segments []string
	current := ""

	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if current != "" {
				segments = append(segments, current)
				current = ""
			}
		} else {
			current += string(path[i])
		}
	}

	if current != "" {
		segments = append(segments, current)
	}

	return segments
}
