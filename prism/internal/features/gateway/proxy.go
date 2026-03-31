package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
	"github.com/jasonKoogler/prism/internal/domain"
	"github.com/jasonKoogler/prism/internal/features/gateway/adapters/circuitbreaker"
	"github.com/jasonKoogler/prism/internal/ports"
)

// ServiceProxy handles proxying requests to backend services
type ServiceProxy struct {
	config         *config.Config
	registry       ports.ServiceRegistry
	logger         *log.Logger
	circuitBreaker ports.CircuitBreaker
	serviceRepo    ports.ServiceRepository
	configDir      string        // Field to store the configuration directory
	routingTable   *RoutingTable // New field for the routing table
}

// NewServiceProxy creates a new service proxy using functional options pattern
func NewServiceProxy(serviceRegistry ports.ServiceRegistry, opts ...ServiceProxyOption) (*ServiceProxy, error) {
	// Initialize with empty values
	sp := &ServiceProxy{
		configDir:    "./config/services", // Default config directory
		routingTable: NewRoutingTable(),   // Initialize routing table
	}

	// Apply all provided options
	for _, opt := range opts {
		if err := opt(sp); err != nil {
			return nil, err
		}
	}

	// Validate required fields
	if sp.config == nil {
		return nil, ErrNilConfig
	}
	if sp.logger == nil {
		return nil, ErrNilLogger
	}

	// Initialize the service repository if not already done
	if sp.registry == nil {
		sp.logger.Warn("Service registry is nil, cannot create service proxy, please pass a concrete implementation of ServiceRegistry")
		return nil, ErrNilServiceRegistry
	} else {

		if err := sp.registry.LoadFromRepository(); err != nil {
			sp.logger.Warn("Failed to load services from repository, falling back to configuration file services", log.Error(err))

			// Fall back to loading from config
			if err := sp.registry.LoadFromConfig(sp.config); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrLoadFromConfig, err)
			}
		}
	}

	// Register all services in the routing table
	services := sp.registry.List()
	for _, svc := range services {
		if err := sp.registerServiceRoutes(svc); err != nil {
			sp.logger.Warn("Failed to register routes for service",
				log.String("service", svc.Name),
				log.Error(err))
		}
	}

	return sp, nil
}

// Create a convenience constructor that uses the traditional parameters
func NewServiceProxyWithDefaults(cfg *config.Config, configDir string, serviceRegistry ports.ServiceRegistry, logger *log.Logger) (*ServiceProxy, error) {
	if configDir == "" {
		configDir = "./config/services" // Default config directory
	}

	if serviceRegistry == nil {
		return nil, ErrNilServiceRegistry
	}

	return NewServiceProxy(
		serviceRegistry,
		WithProxyConfig(cfg),
		WithProxyLogger(logger),
		WithConfigDir(configDir),
	)
}

type proxyContextKey string

const (
	ServiceNameKey proxyContextKey = "service_name"
	PathParamsKey  proxyContextKey = "path_params"
)

// Handler returns an http.Handler that proxies requests to the appropriate service
func (sp *ServiceProxy) Handler() http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First, try to match the route using the routing table
		route, params, found := sp.routingTable.LookupRoute(r.Method, r.URL.Path)

		var serviceName string
		var serviceEntry *ports.ServiceEntry
		var exists bool

		if found {
			// Use the matched service from the route
			serviceName = route.ServiceName
			serviceEntry, exists = sp.registry.Get(serviceName)

			// Store path params in request context for later use
			if len(params) > 0 {
				ctx := context.WithValue(r.Context(), PathParamsKey, params)
				r = r.WithContext(ctx)
			}

			sp.logger.Debug("Route matched",
				log.String("path", r.URL.Path),
				log.String("method", r.Method),
				log.String("service", serviceName),
				log.String("public", fmt.Sprintf("%t", route.Public)))
		} else {
			// Fall back to the legacy path-based routing if no route match
			pathParts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
			if len(pathParts) == 0 {
				http.Error(w, "Invalid service path", http.StatusBadRequest)
				return
			}

			serviceName = pathParts[0]
			serviceEntry, exists = sp.registry.Get(serviceName)
		}

		// If service doesn't exist, return 404
		if !exists {
			http.Error(w, "Service not found", http.StatusNotFound)
			return
		}

		// Check if the method is allowed for this service
		if !isMethodAllowed(r.Method, serviceEntry.Config) {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Add request context for circuit breaker and metrics
		ctx := context.WithValue(r.Context(), ServiceNameKey, serviceName)

		// Create a custom director function for this request
		proxy := serviceEntry.Proxy
		originalDirector := proxy.Director

		// Create a new director for this specific request
		director := func(req *http.Request) {
			originalDirector(req)

			// For matched routes with path params, use the original path when forwarding
			_, hasParams := req.Context().Value("path_params").(map[string]string)

			// Strip the service prefix from the path if needed
			if found && hasParams {
				// For parameterized routes, use original path without service prefix
				req.URL.Path = strings.TrimPrefix(req.URL.Path, "/"+serviceName)
				if req.URL.Path == "" {
					req.URL.Path = "/"
				}
			} else {
				// Legacy routing: strip the service name from the path
				req.URL.Path = strings.TrimPrefix(req.URL.Path, "/"+serviceName)
				if req.URL.Path == "" {
					req.URL.Path = "/"
				}
			}

			// Pass user context to downstream service if available
			if userData, ok := domain.UserContextDataFromContext(req.Context()); ok {
				req.Header.Set("X-User-ID", userData.UserID)
				req.Header.Set("X-User-Email", req.Header.Get("X-Email")) // Get from request header if available

				// Convert roles to a comma-separated list of role strings
				if userData.Roles != nil {
					roleStrings := userData.Roles.GetTenantIDsAsStrings()
					req.Header.Set("X-User-Roles", strings.Join(roleStrings, ","))
				}
			}

			// Add tracing headers
			req.Header.Set("X-Request-ID", req.Header.Get("X-Request-ID"))
			req.Header.Set("X-Forwarded-For", req.RemoteAddr)
		}

		// Create a new proxy with our custom director
		customProxy := &httputil.ReverseProxy{
			Director:       director,
			Transport:      proxy.Transport,
			FlushInterval:  proxy.FlushInterval,
			ErrorLog:       proxy.ErrorLog,
			BufferPool:     proxy.BufferPool,
			ModifyResponse: proxy.ModifyResponse,
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				sp.logger.Error("Error proxying request",
					log.String("service", serviceName),
					log.Error(err))
				w.WriteHeader(http.StatusBadGateway)
				w.Write([]byte("Service unavailable"))
			},
		}

		// Forward the request to the service
		customProxy.ServeHTTP(w, r.WithContext(ctx))
	})

	// Apply circuit breaker middleware if enabled
	if sp.circuitBreaker != nil {
		cbMiddleware := circuitbreaker.NewMiddleware(sp.circuitBreaker, sp.logger)
		return cbMiddleware.Handler(handler)
	}

	return handler
}

// isMethodAllowed checks if the HTTP method is allowed for the service
func isMethodAllowed(method string, svcConfig config.ServiceConfig) bool {
	if len(svcConfig.AllowedMethods) == 0 {
		// If no methods are specified, allow all
		return true
	}

	for _, allowed := range svcConfig.AllowedMethods {
		if method == allowed {
			return true
		}
	}
	return false
}

// HealthHandler implements a handler for proxy health checks
func (sp *ServiceProxy) HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check health of dependent services if requested
		if r.URL.Query().Get("check") == "services" {
			results := make(map[string]string)

			services := sp.registry.List()
			for _, svc := range services {
				status := "ok"

				// Skip health check if no path is defined
				if svc.HealthCheckPath == "" {
					results[svc.Name] = "unknown"
					continue
				}

				// Create a context with timeout
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				// Create request to service health endpoint
				req, err := http.NewRequestWithContext(ctx, "GET", svc.URL+svc.HealthCheckPath, nil)
				if err != nil {
					results[svc.Name] = "error"
					continue
				}

				// Make the request
				resp, err := http.DefaultClient.Do(req)
				if err != nil || resp.StatusCode != http.StatusOK {
					status = "unhealthy"
				}

				if resp != nil && resp.Body != nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}

				results[svc.Name] = status
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(results)
			return
		}

		// Simple health check for the gateway itself
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"status":"ok"}`)
	})
}

// RegisterService adds a new service to the proxy
func (sp *ServiceProxy) RegisterService(svcConfig config.ServiceConfig) error {
	// First register with the registry
	if err := sp.registry.Register(svcConfig); err != nil {
		return err
	}

	// Then register routes for the service
	return sp.registerServiceRoutes(svcConfig)
}

// registerServiceRoutes registers routes for a service in the routing table
func (sp *ServiceProxy) registerServiceRoutes(svcConfig config.ServiceConfig) error {
	// First register any explicitly defined custom routes with higher priority
	if len(svcConfig.Routes) > 0 {
		for _, routeCfg := range svcConfig.Routes {
			// Ensure the path starts with the service name for proper isolation
			routePath := routeCfg.Path
			if !strings.HasPrefix(routePath, "/"+svcConfig.Name+"/") && !strings.HasPrefix(routePath, "/"+svcConfig.Name) {
				routePath = "/" + svcConfig.Name + routePath
			}

			route := RouteMetadata{
				Path:           routePath,
				Method:         routeCfg.Method,
				ServiceID:      svcConfig.Name,
				ServiceName:    svcConfig.Name,
				ServiceURL:     svcConfig.URL,
				Public:         routeCfg.Public,
				RequiredScopes: routeCfg.RequiredScopes,
				Priority:       routeCfg.Priority + 10, // Give custom routes higher base priority
			}

			if err := sp.routingTable.AddRoute(route); err != nil {
				sp.logger.Warn("Failed to register custom route",
					log.String("service", svcConfig.Name),
					log.String("path", routePath),
					log.Error(err))
			}
		}
	}

	// Then register the default catch-all route with lower priority
	defaultRoute := RouteMetadata{
		Path:           "/" + svcConfig.Name + "/*",
		Method:         "*", // Match any method
		ServiceID:      svcConfig.Name,
		ServiceName:    svcConfig.Name,
		ServiceURL:     svcConfig.URL,
		Public:         !svcConfig.RequiresAuth,
		RequiredScopes: svcConfig.AllowedRoles,
		Priority:       0, // Lowest priority
	}

	if err := sp.routingTable.AddRoute(defaultRoute); err != nil {
		return fmt.Errorf("failed to register default route for service %s: %w", svcConfig.Name, err)
	}

	// Also register the specific service root route
	rootRoute := RouteMetadata{
		Path:           "/" + svcConfig.Name,
		Method:         "*", // Match any method
		ServiceID:      svcConfig.Name,
		ServiceName:    svcConfig.Name,
		ServiceURL:     svcConfig.URL,
		Public:         !svcConfig.RequiresAuth,
		RequiredScopes: svcConfig.AllowedRoles,
		Priority:       1, // Slightly higher priority than wildcard
	}

	if err := sp.routingTable.AddRoute(rootRoute); err != nil {
		sp.logger.Warn("Failed to register root route",
			log.String("service", svcConfig.Name),
			log.Error(err))
	}

	sp.logger.Debug("Registered routes for service",
		log.String("service", svcConfig.Name),
		log.String("route_count", fmt.Sprintf("%d", len(svcConfig.Routes)+2))) // +2 for default and root routes

	return nil
}

// DeregisterService removes a service from the proxy
func (sp *ServiceProxy) DeregisterService(serviceName string) error {
	// First deregister from the registry
	if err := sp.registry.Deregister(serviceName); err != nil {
		return err
	}

	// Then remove routes for the service
	sp.routingTable.RemoveServiceRoutes(serviceName)

	return nil
}

// ListServices returns a list of all registered services
func (sp *ServiceProxy) ListServices() []config.ServiceConfig {
	return sp.registry.List()
}

// UpdateService updates a service configuration
func (sp *ServiceProxy) UpdateService(svcConfig config.ServiceConfig) error {
	// First update in the registry
	if err := sp.registry.Update(svcConfig); err != nil {
		return err
	}

	// Remove existing routes for this service
	sp.routingTable.RemoveServiceRoutes(svcConfig.Name)

	// Then re-register routes for the service with the updated config
	return sp.registerServiceRoutes(svcConfig)
}

// AuthenticatedHandler creates a handler that applies authentication middleware
// only to routes that require it according to the routing table.
func (sp *ServiceProxy) AuthenticatedHandler(authMiddleware func(http.Handler) http.Handler) http.Handler {
	baseHandler := sp.Handler()

	// Define routeParamsKey if not defined elsewhere
	type contextKey string
	var routeParamsKey contextKey = "routeParams"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if we have a routing table
		if sp.routingTable == nil {
			// No routing table, apply auth to all requests
			authMiddleware(baseHandler).ServeHTTP(w, r)
			return
		}

		// Get the path and method from request
		path := r.URL.Path
		method := r.Method

		// Check if this route is defined in our routing table
		route, params, found := sp.routingTable.LookupRoute(method, path)
		if !found {
			// Route not found in routing table
			sp.logger.Debug("Route not found in routing table",
				log.String("path", path),
				log.String("method", method))

			// Default to authenticated access if route not found
			authMiddleware(baseHandler).ServeHTTP(w, r)
			return
		}

		// Check if the route is public
		if route.Public {
			// Public route - no authentication needed
			sp.logger.Debug("Public route accessed",
				log.String("path", path),
				log.String("method", method),
				log.String("service", route.ServiceName))

			// Add route params to request context if any
			if len(params) > 0 {
				ctx := context.WithValue(r.Context(), routeParamsKey, params)
				r = r.WithContext(ctx)
			}

			baseHandler.ServeHTTP(w, r)
			return
		}

		// Apply authentication for protected routes
		sp.logger.Debug("Protected route accessed",
			log.String("path", path),
			log.String("method", method),
			log.String("service", route.ServiceName))

		// Create a middleware chain that first authenticates the user
		// and then checks for required scopes
		authHandler := authMiddleware(baseHandler)

		// If the route has required scopes, we should check them after authentication
		if len(route.RequiredScopes) > 0 {
			// Wrap the existing auth handler with scope checking
			originalHandler := authHandler
			authHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Get user data from context after authentication
				userData, ok := domain.UserContextDataFromContext(r.Context())
				if !ok {
					sp.logger.Warn("User data not found in context after authentication",
						log.String("path", path),
						log.String("method", method))
					http.Error(w, "Unauthorized: Invalid or expired token", http.StatusUnauthorized)
					return
				}

				// Check if user has any of the required scopes
				// Scopes can be in format:
				// 1. Simple scope: "read:users"
				// 2. Tenant-specific scope: "tenant:read:users"
				hasScope := false

				// Get the user's roles and tenant IDs
				tenantIDs := userData.Roles.GetTenantIDsAsStrings()

				// For each required scope
				for _, requiredScope := range route.RequiredScopes {
					// Simple scope check - if format is not tenant-specific
					if !strings.Contains(requiredScope, "tenant:") {
						// For simple scope, check if any role has this permission
						for _, role := range userData.Roles {
							if role == domain.UserRole(requiredScope) {
								hasScope = true
								break
							}
						}
					} else {
						// Tenant-specific scope
						// Strip "tenant:" prefix
						scopeWithoutPrefix := strings.TrimPrefix(requiredScope, "tenant:")

						// Check if the user has this scope in any tenant
						for _, tenantID := range tenantIDs {
							scopeToCheck := tenantID + ":" + scopeWithoutPrefix
							if scopeToCheck == requiredScope {
								hasScope = true
								break
							}
						}
					}

					if hasScope {
						break
					}
				}

				if !hasScope {
					// Convert string slice to comma-separated string for logging
					scopesStr := strings.Join(route.RequiredScopes, ",")

					sp.logger.Warn("User lacks required scopes",
						log.String("user_id", userData.UserID),
						log.String("required_scopes", scopesStr))
					http.Error(w, "Forbidden: Insufficient privileges", http.StatusForbidden)
					return
				}

				// Add route params to request context if any
				if len(params) > 0 {
					ctx := context.WithValue(r.Context(), routeParamsKey, params)
					r = r.WithContext(ctx)
				}

				// User has the required scopes, continue
				originalHandler.ServeHTTP(w, r)
			})
		}

		// Apply the complete auth middleware chain
		authHandler.ServeHTTP(w, r)
	})
}

// RegisterServiceRoute adds a custom route for an existing service
func (sp *ServiceProxy) RegisterServiceRoute(serviceName string, routeConfig config.RouteConfig) error {
	// Check if service exists
	serviceEntry, exists := sp.registry.Get(serviceName)
	if !exists {
		return fmt.Errorf("%w: %s", ErrServiceNotFound, serviceName)
	}

	// Ensure the path starts with the service name
	routePath := routeConfig.Path
	if !strings.HasPrefix(routePath, "/"+serviceName+"/") && !strings.HasPrefix(routePath, "/"+serviceName) {
		routePath = "/" + serviceName + routePath
	}

	// Create and add the route
	route := RouteMetadata{
		Path:           routePath,
		Method:         routeConfig.Method,
		ServiceID:      serviceName,
		ServiceName:    serviceName,
		ServiceURL:     serviceEntry.Config.URL,
		Public:         routeConfig.Public,
		RequiredScopes: routeConfig.RequiredScopes,
		Priority:       routeConfig.Priority + 10, // Give custom routes higher base priority
	}

	if err := sp.routingTable.AddRoute(route); err != nil {
		return fmt.Errorf("failed to register route: %w", err)
	}

	sp.logger.Debug("Registered custom route",
		log.String("service", serviceName),
		log.String("path", routePath),
		log.String("method", routeConfig.Method),
		log.String("public", fmt.Sprintf("%t", routeConfig.Public)))

	return nil
}

// ListRoutes returns all registered routes
func (sp *ServiceProxy) ListRoutes() []RouteMetadata {
	return sp.routingTable.GetRoutes()
}

// ListPublicRoutes returns all public routes
func (sp *ServiceProxy) ListPublicRoutes() []RouteMetadata {
	return sp.routingTable.GetPublicRoutes()
}

// ListProtectedRoutes returns all protected routes
func (sp *ServiceProxy) ListProtectedRoutes() []RouteMetadata {
	return sp.routingTable.GetProtectedRoutes()
}

// ListServiceRoutes returns all routes for a specific service
func (sp *ServiceProxy) ListServiceRoutes(serviceName string) ([]RouteMetadata, error) {
	// Check if service exists
	_, exists := sp.registry.Get(serviceName)
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrServiceNotFound, serviceName)
	}

	return sp.routingTable.GetServiceRoutes(serviceName), nil
}
