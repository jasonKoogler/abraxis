package app

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/jasonKoogler/abraxis/aegis/internal/adapters/authz"
	adaptersHTTP "github.com/jasonKoogler/abraxis/aegis/internal/adapters/http"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/redis"
	"github.com/jasonKoogler/abraxis/aegis/internal/config"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
	aegisgrpc "github.com/jasonKoogler/abraxis/aegis/internal/grpc"
	"github.com/jasonKoogler/abraxis/aegis/internal/ports"
	"github.com/jasonKoogler/abraxis/aegis/internal/service"
)

// App encapsulates the application and its dependencies
type App struct {
	ctx                context.Context
	cfg                *config.Config
	logger             *log.Logger
	keyManager         *domain.KeyManager
	userRepo           ports.UserRepository
	userService        *service.UserService
	authService        *service.AuthManager
	authzService       *authz.Adapter
	rateLimiter        ports.RateLimiter
	srv                *Server
	redisClient        *redis.RedisClient
	auditService       *service.AuditService
	auditRepo          ports.AuditLogRepository
	apiKeyService      *service.APIKeyService
	apiKeyRepo         ports.APIKeyRepository
	tenantService      *service.TenantDomainService
	tenantRepo         ports.TenantRepository
	permissionService  *service.PermissionService
	permissionRepo     ports.PermissionRepository
	rolePermissionRepo ports.RolePermissionRepository
	grpcServer         *aegisgrpc.AegisAuthServer
}

// NewApp creates a new App instance with the provided options
func NewApp(opts ...AppOption) (*App, error) {
	app := &App{}

	app.ctx = context.Background()

	// Apply provided options
	for _, opt := range opts {
		if err := opt(app); err != nil {
			return nil, err
		}
	}

	// Validate required dependencies
	if app.cfg == nil {
		return nil, ErrConfigRequired
	}
	if app.logger == nil {
		// Create default logger if not provided
		app.logger = log.NewLogger(app.cfg.LogLevel.String())
	}
	if app.userRepo == nil {
		return nil, ErrUserRepositoryRequired
	}
	if app.keyManager == nil {
		km, err := domain.NewKeyManager()
		if err != nil {
			return nil, fmt.Errorf("failed to create key manager: %w", err)
		}
		app.keyManager = km
	}
	if app.userService == nil {
		// Create default user service if not provided
		app.userService = service.NewUserService(app.userRepo)
	}
	if app.authService == nil {
		// Create default auth service if not provided
		var err error
		app.authService, err = service.NewAuthManager(&app.cfg.Auth, app.logger, app.userService, app.keyManager)
		if err != nil {
			return nil, err
		}
	}
	if app.redisClient == nil {
		// Create default Redis client if not provided
		redisClient, err := redis.NewRedisClient(app.ctx, app.logger, &app.cfg.Auth.AuthN.RedisConfig)
		if err != nil {
			return nil, err
		}
		app.redisClient = redisClient
	}
	if app.srv == nil {
		// Create default server if not provided
		var err error
		app.srv, err = NewServer(app.cfg, app.logger, WithAuthService(app.authService))
		if err != nil {
			return nil, err
		}
	}

	return app, nil
}

// Start creates the server instance, registers routes via public/protected handlers,
// and starts the server using the built-in signal handling and graceful shutdown.
func (a *App) Start() error {
	// Register global middlewares.
	a.srv.GetRouter().Use(createCorsMiddleware(a.cfg))
	a.srv.GetRouter().Use(chimiddleware.RequestID)
	a.srv.GetRouter().Use(chimiddleware.RealIP)
	a.srv.GetRouter().Use(chimiddleware.Recoverer)
	a.srv.GetRouter().Use(log.NewHTTPLogger(a.logger))

	// Register auth + user API routes directly on chi with auth middleware grouping.
	httpServer := adaptersHTTP.NewServer(a.authService, a.userService, a.cfg, a.logger)
	authMiddleware := adaptersHTTP.NewAuthMiddleware(a.authService).Authenticate
	httpServer.RegisterRoutes(a.srv.GetRouter(), authMiddleware)

	// Swagger UI
	a.srv.GetRouter().Get("/swagger/*", httpSwagger.WrapHandler)

	// Public infrastructure endpoints.
	publicHandlers := map[string]http.Handler{
		"/.well-known/jwks.json": a.keyManager.JWKSHandler(),
	}

	// No more protected handler map — auth middleware is applied via chi route groups above.
	protectedHandlers := map[string]http.Handler{}

	// Start the gRPC server on its own port if enabled.
	if a.cfg.GRPC.Enabled && a.grpcServer != nil {
		go func() {
			if err := a.grpcServer.Start(a.cfg.GRPC.Port); err != nil {
				a.logger.Error("gRPC server failed", log.Error(err))
			}
		}()
		a.logger.Info("gRPC server started", log.String("port", a.cfg.GRPC.Port))
	}

	// Start the server with the defined public and protected endpoints.
	ctx := context.Background()
	if err := a.srv.Start(ctx, publicHandlers, protectedHandlers); err != nil {
		return err
	}

	// Wait here until the server shuts down gracefully (via signals or returned context cancellation).
	a.srv.Wait()
	return nil
}

// Shutdown gracefully shuts down the application.
func (a *App) Shutdown(ctx context.Context) error {
	// Close Redis client
	if a.redisClient != nil {
		if err := a.redisClient.Close(); err != nil {
			a.logger.Error("Failed to close Redis client", log.Error(err))
		}
	}

	// Shutdown the server
	if a.srv != nil {
		return a.srv.Stop(ctx)
	}

	return nil
}

// createCorsMiddleware creates and returns a CORS middleware handler if CORS is configured.
// When no allowed origins are set it returns a pass-through middleware so that
// chi.Use never receives a nil handler.
func createCorsMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	corsOrigins := cfg.HTTPServer.CORS.AllowedOrigins
	if corsOrigins == "" {
		return func(next http.Handler) http.Handler { return next }
	}
	allowedOrigins := strings.Split(corsOrigins, ";")
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	})
	return corsHandler.Handler
}

// CreateAuthMiddleware creates a middleware that combines authentication and authorization
func (a *App) CreateAuthMiddleware(requiredPermissions ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// First apply authentication to get the user
			userHandler := adaptersHTTP.NewAuthMiddleware(a.authService).Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Authentication successful, user is in context
				// Now apply authorization if required and available
				if len(requiredPermissions) > 0 && a.authzService != nil {
					// Extract authorization input from request and user context
					input, err := extractAuthZInput(r, requiredPermissions)
					if err != nil {
						a.logger.Error("Failed to extract authorization input", log.Error(err))
						http.Error(w, "Unauthorized", http.StatusUnauthorized)
						return
					}

					// Evaluate authorization
					decision, err := a.authzService.Evaluate(r.Context(), input)
					if err != nil {
						a.logger.Error("Authorization evaluation failed", log.Error(err))
						http.Error(w, "Forbidden", http.StatusForbidden)
						return
					}

					// Log the decision for debugging
					a.logger.Debug("Authorization decision",
						log.String("decision", fmt.Sprintf("%+v", decision)))

					// Most common implementations have an Allowed field
					// but let's check the actual properties
					allowedField := reflect.ValueOf(decision).FieldByName("Allowed")
					if allowedField.IsValid() && allowedField.Kind() == reflect.Bool && allowedField.Bool() {
						// Permission granted
						next.ServeHTTP(w, r)
						return
					}

					// Get reason for denial if available
					var reason string = "access denied"
					reasonField := reflect.ValueOf(decision).FieldByName("Reason")
					if reasonField.IsValid() && reasonField.Kind() == reflect.String {
						reason = reasonField.String()
					}

					a.logger.Warn("Authorization denied",
						log.String("reason", reason),
						log.String("user_id", input.(map[string]interface{})["user"].(map[string]interface{})["id"].(string)))
					http.Error(w, "Forbidden: "+reason, http.StatusForbidden)
				}

				// Both authentication and authorization passed, continue to the handler
				next.ServeHTTP(w, r)
			}))

			userHandler.ServeHTTP(w, r)
		})
	}
}

// extractAuthZInput extracts authorization input from the request and user context
func extractAuthZInput(r *http.Request, requiredPermissions []string) (interface{}, error) {
	// Get user data from context
	userData, ok := domain.UserContextDataFromContext(r.Context())
	if !ok {
		return nil, fmt.Errorf("user data not found in context")
	}

	// Extract path, method, and any path parameters
	path := r.URL.Path
	method := r.Method

	// Extract resource information (e.g., from path parameters or query parameters)
	// This is a simplified example and should be customized based on your API design
	resourceType := "unknown"
	resourceID := ""

	// Try to extract resource type and ID from path
	// Example patterns: /api/users/:id, /api/tenants/:id/users
	pathParts := strings.Split(path, "/")
	if len(pathParts) > 2 {
		resourceType = pathParts[2] // e.g., "users" from "/api/users/123"
		if len(pathParts) > 3 {
			resourceID = pathParts[3] // e.g., "123" from "/api/users/123"
		}
	}

	// Build the input object for OPA
	input := map[string]interface{}{
		"user": map[string]interface{}{
			"id":    userData.UserID,
			"roles": userData.Roles.GetTenantIDsAsStrings(),
		},
		"request": map[string]interface{}{
			"path":   path,
			"method": method,
		},
		"resource": map[string]interface{}{
			"type": resourceType,
			"id":   resourceID,
		},
		"permissions": requiredPermissions,
	}

	return input, nil
}

// ExportAuthMiddleware exports the auth middleware for use by services
func (a *App) ExportAuthMiddleware() func(requiredPermissions ...string) func(http.Handler) http.Handler {
	return a.CreateAuthMiddleware
}
