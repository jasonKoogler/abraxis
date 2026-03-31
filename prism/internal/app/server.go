package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/jasonKoogler/abraxis/prism/internal/common/log"
	"github.com/jasonKoogler/abraxis/prism/internal/config"
	"github.com/jasonKoogler/abraxis/prism/internal/features/ratelimit"
	"github.com/jasonKoogler/abraxis/prism/internal/ports"
)

// HTTPServer defines the core server operations
type HTTPServer interface {
	// Start initializes and starts the HTTP server
	Start(context.Context, map[string]http.Handler, map[string]http.Handler) error
	// Stop gracefully shuts down the server
	Stop(context.Context) error
	// Wait blocks until the server has fully stopped
	Wait()
}

// Server implements HTTPServer and manages the HTTP server lifecycle
type Server struct {
	cfg                *config.Config
	logger             *log.Logger
	httpServer         *http.Server
	router             *chi.Mux
	rateLimiter        ports.RateLimiter
	readinessCheckers  []ReadinessChecker
	wg                 sync.WaitGroup
	shutdownCh         chan struct{}
}

// ServerOption defines functional options for configuring the server
type ServerOption func(*Server) error

// WithRedisRateLimiter configures the server to use Redis-based rate limiting
func WithRedisRateLimiter(client *redis.Client, cfg *ratelimiter.RateLimiterParams) ServerOption {
	return func(s *Server) error {
		rateLimiter, err := ratelimiter.NewRedisRateLimiter(cfg)
		if err != nil {
			return err
		}
		s.rateLimiter = rateLimiter
		return nil
	}
}

// WithMemoryRateLimiter configures the server to use in-memory rate limiting
func WithMemoryRateLimiter(cfg *ratelimiter.RateLimiterParams) ServerOption {
	return func(s *Server) error {
		rateLimiter, err := ratelimiter.NewMemoryRateLimiter(cfg)
		if err != nil {
			return err
		}
		s.rateLimiter = rateLimiter
		return nil
	}
}

// NewServer creates a new server instance with the provided options
func NewServer(cfg *config.Config, logger *log.Logger, opts ...ServerOption) (*Server, error) {
	s := &Server{
		cfg:        cfg,
		logger:     logger,
		router:     chi.NewRouter(),
		shutdownCh: make(chan struct{}),
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// GetRouter returns the server's router for adding routes
func (s *Server) GetRouter() *chi.Mux {
	return s.router
}

// RegisterPublicHandler mounts a handler at the given path without authentication.
func (s *Server) RegisterPublicHandler(path string, handler http.Handler) {
	s.router.Mount(path, handler)
}

// RegisterProtectedHandler mounts a handler at the given path.
// Note: Authentication middleware should be applied by the caller.
func (s *Server) RegisterProtectedHandler(path string, handler http.Handler) {
	s.router.Mount(path, handler)
}

// Start initializes routes and starts the HTTP server.
// It accepts two maps: one for public endpoints and one for protected endpoints.
func (s *Server) Start(ctx context.Context, publicHandlers map[string]http.Handler, protectedHandlers map[string]http.Handler) error {
	// Register non-auth middleware on the main router.
	if s.rateLimiter != nil {
		s.RegisterMiddleware(s.rateLimiter.LimitMiddleware)
	}

	// Setup health checks.
	s.setupHealthChecks()

	// Register public API endpoints.
	for path, handler := range publicHandlers {
		s.RegisterPublicHandler(path, handler)
	}

	// Register protected API endpoints.
	for path, handler := range protectedHandlers {
		s.RegisterProtectedHandler(path, handler)
	}

	// Configure the metrics endpoint.
	s.router.Handle("/metrics", promhttp.Handler())

	// Configure the HTTP server.
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%s", s.cfg.HTTPServer.Port),
		Handler:      s.router,
		ReadTimeout:  s.cfg.HTTPServer.ReadTimeout,
		WriteTimeout: s.cfg.HTTPServer.WriteTimeout,
		IdleTimeout:  s.cfg.HTTPServer.IdleTimeout,
	}

	// Start the signal handler.
	s.handleSignals(ctx)

	s.logger.Info(fmt.Sprintf("Starting server on port %s", s.cfg.HTTPServer.Port))

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Server error", log.Error(err))
		}
	}()

	s.PrintRoutes()

	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Shutting down server...")

	// Create a shutdown context with timeout.
	shutdownCtx, cancel := context.WithTimeout(ctx, s.cfg.HTTPServer.ShutdownTimeout)
	defer cancel()

	// Shutdown the HTTP server.
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	// Close rate limiter if necessary.
	if s.rateLimiter != nil {
		if err := s.rateLimiter.Close(); err != nil {
			s.logger.Error("Error closing rate limiter", log.Error(err))
		}
	}

	close(s.shutdownCh)
	s.logger.Info("Server shutdown complete")
	return nil
}

// Wait blocks until the server has fully stopped.
func (s *Server) Wait() {
	<-s.shutdownCh
	s.wg.Wait()
}

// handleSignals sets up signal handling for graceful shutdown.
func (s *Server) handleSignals(ctx context.Context) {
	s.wg.Add(1)
	go func(ctx context.Context) {
		defer s.wg.Done()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		select {
		case sig := <-sigCh:
			s.logger.Info(fmt.Sprintf("Received signal: %s", sig))
			if err := s.Stop(ctx); err != nil {
				s.logger.Error("Error during shutdown", log.Error(err))
			}
		case <-s.shutdownCh:
			// Shutdown initiated elsewhere.
		case <-ctx.Done():
			s.logger.Info("Context done, shutting down server")
			if err := s.Stop(ctx); err != nil {
				s.logger.Error("Error during shutdown", log.Error(err))
			}
		}
	}(ctx)
}

// SetReadinessCheckers configures the readiness checkers used by the /ready
// endpoint. Must be called before Start.
func (s *Server) SetReadinessCheckers(checkers []ReadinessChecker) {
	s.readinessCheckers = checkers
}

// setupHealthChecks configures health check endpoints.
func (s *Server) setupHealthChecks() {
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	s.router.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		for _, checker := range s.readinessCheckers {
			if !checker.IsReady() {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"not ready"}`))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
}

// RegisterMiddleware adds a new middleware to the server.
func (s *Server) RegisterMiddleware(middlewareFunc func(http.Handler) http.Handler) {
	s.router.Use(middlewareFunc)
}

// PrintRoutes prints all registered routes in a formatted way.
func (s *Server) PrintRoutes() {
	var printRoute func(router chi.Routes, path string, indent string)

	printRoute = func(router chi.Routes, path string, indent string) {
		routes := router.Routes()
		for _, route := range routes {
			if route.SubRoutes != nil {
				s.logger.Info(fmt.Sprintf("%s%s -> [SubRoutes]", indent, path+route.Pattern))
				printRoute(route.SubRoutes, path+route.Pattern, indent+"  ")
			} else {
				methods := make([]string, 0, len(route.Handlers))
				for method := range route.Handlers {
					methods = append(methods, method)
				}
				s.logger.Info(fmt.Sprintf("%s%s -> [%s]", indent, path+route.Pattern, strings.Join(methods, ",")))
			}
		}
	}

	s.logger.Info("Registered Routes:")
	s.logger.Info("=================")
	printRoute(s.router, "", "")
	s.logger.Info("=================")
}
