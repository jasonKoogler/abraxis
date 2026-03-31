package app

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/jasonKoogler/abraxis/prism/internal/common/log"
	"github.com/jasonKoogler/abraxis/prism/internal/common/redis"
	"github.com/jasonKoogler/abraxis/prism/internal/config"
	"github.com/jasonKoogler/abraxis/prism/internal/domain"
	"github.com/jasonKoogler/abraxis/prism/internal/features/auth"
	"github.com/jasonKoogler/abraxis/prism/internal/features/auth/adapters/authz"
	"github.com/jasonKoogler/abraxis/prism/internal/features/gateway"
	aegisclient "github.com/jasonKoogler/abraxis/prism/internal/features/gateway/adapters/aegis"
	"github.com/jasonKoogler/abraxis/prism/internal/ports"
)

// ReadinessChecker reports whether a subsystem is ready to serve traffic.
type ReadinessChecker interface {
	IsReady() bool
}

// App encapsulates the application and its dependencies
type App struct {
	ctx                context.Context
	cfg                *config.Config
	logger             *log.Logger
	tokenValidator     *domain.TokenValidator
	jwksFetcher        *domain.JWKSFetcher
	readinessCheckers  []ReadinessChecker
	authzService       *authz.Adapter
	rateLimiter        ports.RateLimiter
	srv                *Server
	redisClient        *redis.RedisClient
	circuitBreaker     ports.CircuitBreaker
	serviceProxy       *gateway.ServiceProxy
	serviceDiscovery   ports.ServiceDiscoverer
	auditService       ports.AuditService
	auditRepo          ports.AuditLogRepository
	apiKeyService      ports.ApiKeyService
	apiKeyRepo         ports.ApiKeyRepository
	tenantRepo         ports.TenantRepository
	permissionRepo     ports.PermissionRepository
	rolePermissionRepo ports.RolePermissionRepository
	aegisClient        *aegisclient.Client
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
		app.srv, err = NewServer(app.cfg, app.logger)
		if err != nil {
			return nil, err
		}
	}

	// Create default service proxy if not provided and services are configured
	if app.serviceProxy == nil && app.cfg.Services != nil && len(app.cfg.Services) > 0 {
		// Apply the WithDefaultServiceProxy option
		err := WithDefaultServiceProxy()(app)
		if err != nil {
			app.logger.Warn("Failed to create default service proxy", log.Error(err))
			// Continue without service proxy
		}
	}

	return app, nil
}

// Start creates the server instance, registers routes via public/protected handlers,
// and starts the server using the built-in signal handling and graceful shutdown.
func (a *App) Start() error {
	// Start JWKS fetcher before creating the token validator. The first fetch
	// blocks so that we don't serve traffic before keys are available.
	if a.jwksFetcher != nil {
		if err := a.jwksFetcher.Start(context.Background()); err != nil {
			a.logger.Error("JWKS fetch failed", log.Error(err))
		}
	}

	// Create token validator from JWKS fetcher (EdDSA) if not already set.
	if a.tokenValidator == nil && a.jwksFetcher != nil {
		a.tokenValidator = domain.NewTokenValidator(a.jwksFetcher)
	}

	// Register readiness checkers for the /ready endpoint.
	if a.jwksFetcher != nil {
		a.readinessCheckers = append(a.readinessCheckers, a.jwksFetcher)
	}
	if a.aegisClient != nil {
		a.readinessCheckers = append(a.readinessCheckers, a.aegisClient)
	}

	// Pass readiness checkers to the server so the /ready endpoint can use them.
	a.srv.SetReadinessCheckers(a.readinessCheckers)

	// Register global middlewares.
	a.srv.GetRouter().Use(createCorsMiddleware(a.cfg))
	a.srv.GetRouter().Use(chimiddleware.RequestID)
	a.srv.GetRouter().Use(chimiddleware.RealIP)
	a.srv.GetRouter().Use(chimiddleware.Recoverer)
	a.srv.GetRouter().Use(chimiddleware.Logger)

	// Start Aegis gRPC sync if enabled
	if a.cfg.Aegis.SyncEnabled && a.aegisClient != nil {
		if err := a.aegisClient.Start(context.Background()); err != nil {
			a.logger.Warn("failed to start Aegis gRPC sync", log.Error(err))
		} else {
			a.logger.Info("Aegis gRPC sync started", log.String("address", a.cfg.Aegis.GRPCAddress))
		}
	}

	// Initialize and register the service proxy if needed
	if a.serviceProxy == nil && a.cfg.Services != nil && len(a.cfg.Services) > 0 {
		// Apply the WithDefaultServiceProxy option
		if err := WithDefaultServiceProxy()(a); err != nil {
			a.logger.Warn("Failed to initialize service proxy", log.Error(err))
		}
	}

	// Register the proxy handler if available
	if a.serviceProxy != nil {
		// Register service and route management API endpoints (chi-based)
		serviceAPIHandler := gateway.NewServiceAPIHandler(a.serviceProxy, a.logger)
		serviceAPIHandler.RegisterRoutes(a.srv.GetRouter())

		// Build auth options — attach revocation checker when Aegis client is available
		var authOpts []auth.AuthMiddlewareOption
		if a.aegisClient != nil {
			authOpts = append(authOpts, auth.WithRevocationChecker(a.aegisClient))
		}

		// Create an auth middleware instance for the service proxy
		authMiddlewareInstance := auth.NewAuthMiddleware(a.tokenValidator, authOpts...)

		// Register the service proxy handler with conditional authentication
		// This leverages our routing table to determine auth requirements per route
		proxyHandler := a.serviceProxy.AuthenticatedHandler(authMiddlewareInstance.Authenticate)
		a.srv.GetRouter().Handle("/*", proxyHandler)

		// Register the health check endpoint
		a.srv.GetRouter().Handle("/api/services-health", a.serviceProxy.HealthHandler())

		a.logger.Info("Service proxy and API handlers initialized and registered")
	}

	// Start watching for service discovery events if available
	if a.serviceDiscovery != nil && a.serviceProxy != nil {
		// Start a goroutine to watch for service updates
		go a.startServiceWatcher(context.Background())
		a.logger.Info("Service discovery watcher started")
	}

	// Register feature API routes
	if a.auditService != nil {
		RegisterAuditRoutes(a.srv.GetRouter(), a.auditService, a.cfg, a.logger)
	}
	if a.apiKeyService != nil {
		RegisterApiKeyRoutes(a.srv.GetRouter(), a.apiKeyService, a.cfg, a.logger)
	}

	// Swagger UI
	a.srv.GetRouter().Get("/swagger/*", httpSwagger.WrapHandler)

	// Define maps for public and protected endpoints.
	// Public endpoints don't require authentication
	publicHandlers := map[string]http.Handler{
		// Public health check endpoint
		"/health": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}),
	}

	// Protected endpoints require authentication (none by default for gateway-only mode)
	protectedHandlers := map[string]http.Handler{}

	// Start the server with the defined public and protected endpoints.
	ctx := context.Background()
	if err := a.srv.Start(ctx, publicHandlers, protectedHandlers); err != nil {
		return err
	}

	// Wait here until the server shuts down gracefully (via signals or returned context cancellation).
	a.srv.Wait()
	return nil
}

// startServiceWatcher starts a goroutine to watch for service changes
func (a *App) startServiceWatcher(ctx context.Context) {
	// Create a cancel context for the watcher
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create a watcher for all services
	instanceCh, err := a.serviceDiscovery.WatchServices(watchCtx)
	if err != nil {
		a.logger.Error("Failed to start service watcher", log.Error(err))
		return
	}

	// Get initial list of services
	instances, err := a.serviceDiscovery.ListInstances(ctx)
	if err != nil {
		a.logger.Error("Failed to list service instances", log.Error(err))
	} else {
		// Register all existing services
		for _, instance := range instances {
			a.registerServiceInstance(instance)
		}
	}

	// Watch for service changes
	for {
		select {
		case <-ctx.Done():
			return
		case instance, ok := <-instanceCh:
			if !ok {
				a.logger.Warn("Service discovery channel closed, reconnecting")
				// Try to reconnect after a delay
				time.Sleep(5 * time.Second)
				instanceCh, err = a.serviceDiscovery.WatchServices(watchCtx)
				if err != nil {
					a.logger.Error("Failed to restart service watcher", log.Error(err))
					return
				}
				continue
			}

			// Check if the instance is still active
			if instance.Status == "active" {
				a.registerServiceInstance(instance)
			} else {
				a.deregisterServiceInstance(instance)
			}
		}
	}
}

// registerServiceInstance adds or updates a service in the proxy
func (a *App) registerServiceInstance(instance *ports.ServiceInstance) {
	if a.serviceProxy == nil {
		return
	}

	a.logger.Info("Registering service instance",
		log.String("service", instance.ServiceName),
		log.String("id", instance.ID),
		log.String("address", instance.Address))

	// Create a service config from the instance
	svcConfig := config.ServiceConfig{
		Name:            instance.ServiceName,
		URL:             fmt.Sprintf("%s:%d", instance.Address, instance.Port),
		HealthCheckPath: instance.Metadata["health_check_path"],
		RequiresAuth:    instance.Metadata["requires_auth"] == "true",
	}

	// If allowed methods are specified, parse them
	if allowedMethods, ok := instance.Metadata["allowed_methods"]; ok && allowedMethods != "" {
		svcConfig.AllowedMethods = strings.Split(allowedMethods, ",")
	}

	// If timeout is specified, parse it
	if timeout, ok := instance.Metadata["timeout"]; ok && timeout != "" {
		if duration, err := time.ParseDuration(timeout); err == nil {
			svcConfig.Timeout = duration
		}
	}

	// Parse retry count if specified
	if retryCount, ok := instance.Metadata["retry_count"]; ok && retryCount != "" {
		if count, err := strconv.Atoi(retryCount); err == nil {
			svcConfig.RetryCount = count
		}
	}

	// Register or update the service
	if err := a.serviceProxy.RegisterService(svcConfig); err != nil {
		a.logger.Error("Failed to register service with proxy",
			log.String("name", instance.ServiceName),
			log.Error(err))
	} else {
		a.logger.Info("Service registered with proxy", log.String("name", instance.ServiceName))
	}
}

// deregisterServiceInstance removes a service from the proxy
func (a *App) deregisterServiceInstance(instance *ports.ServiceInstance) {
	if a.serviceProxy == nil {
		return
	}

	a.logger.Info("Deregistering service instance",
		log.String("service", instance.ServiceName),
		log.String("id", instance.ID))

	// Remove the service from the proxy
	if err := a.serviceProxy.DeregisterService(instance.ServiceName); err != nil {
		a.logger.Error("Failed to deregister service from proxy",
			log.String("name", instance.ServiceName),
			log.Error(err))
	} else {
		a.logger.Info("Service deregistered from proxy", log.String("name", instance.ServiceName))
	}
}

// Shutdown gracefully shuts down the application.
func (a *App) Shutdown(ctx context.Context) error {
	// Stop JWKS fetcher
	if a.jwksFetcher != nil {
		a.jwksFetcher.Stop()
	}

	// Stop Aegis gRPC client
	if a.aegisClient != nil {
		if err := a.aegisClient.Stop(); err != nil {
			a.logger.Error("Failed to stop Aegis client", log.Error(err))
		}
	}

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
func createCorsMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	corsOrigins := cfg.HTTPServer.CORS.AllowedOrigins
	if corsOrigins == "" {
		return nil
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
			userHandler := auth.NewAuthMiddleware(a.tokenValidator).Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

// RegisterServiceRoute adds a new route for a service in the proxy
func (a *App) RegisterServiceRoute(serviceName string, path, method string, isPublic bool, requiredScopes []string) error {
	if a.serviceProxy == nil {
		return ErrNilServiceProxy
	}

	// Create a route config
	routeConfig := config.RouteConfig{
		Path:           path,
		Method:         method,
		Public:         isPublic,
		RequiredScopes: requiredScopes,
		Priority:       10, // Default priority
	}

	// Register the route
	return a.serviceProxy.RegisterServiceRoute(serviceName, routeConfig)
}

// RegisterPublicRoute registers a public route for a service
func (a *App) RegisterPublicRoute(serviceName, path, method string) error {
	return a.RegisterServiceRoute(serviceName, path, method, true, nil)
}

// RegisterProtectedRoute registers a protected route for a service with required scopes
func (a *App) RegisterProtectedRoute(serviceName, path, method string, requiredScopes []string) error {
	return a.RegisterServiceRoute(serviceName, path, method, false, requiredScopes)
}

// GetServiceRoutes returns all routes for a specific service
func (a *App) GetServiceRoutes(serviceName string) ([]gateway.RouteMetadata, error) {
	if a.serviceProxy == nil {
		return nil, ErrNilServiceProxy
	}

	return a.serviceProxy.ListServiceRoutes(serviceName)
}

// GetAllRoutes returns all registered routes
func (a *App) GetAllRoutes() []gateway.RouteMetadata {
	if a.serviceProxy == nil {
		return nil
	}

	return a.serviceProxy.ListRoutes()
}

// GetPublicRoutes returns all public routes
func (a *App) GetPublicRoutes() []gateway.RouteMetadata {
	if a.serviceProxy == nil {
		return nil
	}

	return a.serviceProxy.ListPublicRoutes()
}

// GetProtectedRoutes returns all protected routes
func (a *App) GetProtectedRoutes() []gateway.RouteMetadata {
	if a.serviceProxy == nil {
		return nil
	}

	return a.serviceProxy.ListProtectedRoutes()
}
