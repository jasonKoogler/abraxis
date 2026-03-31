# HTTP Adapter

## Overview

The HTTP adapter provides comprehensive HTTP functionality for the application, including:

1. **Service Proxy** - A dynamic reverse proxy for routing requests to microservices
2. **Authentication Middleware** - JWT and OAuth-based authentication
3. **Authorization** - Role and scope-based access control
4. **API Route Management** - Dynamic routing configuration
5. **Service Registry** - Management of backend services
6. **Health Monitoring** - Endpoint for checking system health

The adapter serves as the gateway for all HTTP traffic in the system, providing a single point of entry while dynamically routing to the appropriate backend services.

## Components

### Service Proxy

The `ServiceProxy` is a sophisticated reverse proxy that:

- Routes incoming requests to appropriate backend services
- Applies conditional authentication based on route configuration
- Enforces scope-based authorization
- Handles circuit breaking for fault tolerance
- Monitors health of backend services
- Supports dynamic route registration and service discovery

#### Key Features

- **Dynamic Routing Table** - Pattern-based routing with path parameter extraction
- **Conditional Authentication** - Public vs. protected routes
- **Scope-Based Authorization** - Tenant-aware permission checks
- **Circuit Breaking** - Integration with circuit breaker for fault tolerance
- **Health Monitoring** - Aggregated health status of all services

### Authentication Middleware

The `AuthMiddleware` component provides:

- JWT-based authentication using Bearer tokens
- Integration with the authentication manager
- User context population with role and tenant information
- Support for OAuth-based authentication flows

### Service Registry

The `ServiceRegistry` manages:

- Backend service registration and deregistration
- Service configuration persistence
- Load balancing configuration
- Health check configuration

### API Route Management

The `RoutingTable` and related components provide:

- Pattern-based URL matching (static segments, parameters, wildcards)
- Priority-based route resolution
- Route conflict detection
- Support for public and protected routes

## Port Interfaces

The HTTP adapter implements several port interfaces:

### ServerInterface (OpenAPI)

Generated interface from the OpenAPI specification for handling API endpoints:

```go
type ServerInterface interface {
    LoginUserWithPassword(w http.ResponseWriter, r *http.Request)
    LogoutUser(w http.ResponseWriter, r *http.Request)
    RefreshToken(w http.ResponseWriter, r *http.Request)
    RegisterUser(w http.ResponseWriter, r *http.Request)
    InitiateSocialLogin(w http.ResponseWriter, r *http.Request, provider InitiateSocialLoginParamsProvider)
    HandleSocialLoginCallback(w http.ResponseWriter, r *http.Request, provider HandleSocialLoginCallbackParamsProvider, params HandleSocialLoginCallbackParams)
    // ...other API endpoints
}
```

## Configuration Options

The HTTP adapter uses the functional options pattern for configuration:

### ServiceProxy Options

```go
// WithProxyConfig sets the configuration for ServiceProxy
func WithProxyConfig(cfg *config.Config) ServiceProxyOption

// WithProxyLogger sets the logger for ServiceProxy
func WithProxyLogger(logger *log.Logger) ServiceProxyOption

// WithServiceRegistry sets a custom service registry for ServiceProxy
func WithServiceRegistry(registry *ServiceRegistry) ServiceProxyOption

// WithConfigDir sets the configuration directory for ServiceProxy
func WithConfigDir(configDir string) ServiceProxyOption

// WithCircuitBreaker sets a circuit breaker for ServiceProxy
func WithCircuitBreaker(cb ports.CircuitBreaker) ServiceProxyOption

// WithRoutingTable sets a custom routing table for ServiceProxy
func WithRoutingTable(routingTable *RoutingTable) ServiceProxyOption
```

## Service Configuration

Services are configured using the `ServiceConfig` struct:

```go
type ServiceConfig struct {
    Name            string          // Service name/identifier
    URL             string          // Base URL of the service
    HealthCheckPath string          // Path to health check endpoint
    RequiresAuth    bool            // Whether authentication is required by default
    AllowedMethods  []string        // HTTP methods allowed for this service
    AllowedRoles    []string        // Roles allowed to access this service
    Timeout         time.Duration   // Request timeout
    RetryCount      int             // Number of retries for failed requests
    Routes          []RouteConfig   // Custom route definitions
}
```

## Route Configuration

Routes are configured using the `RouteConfig` struct:

```go
type RouteConfig struct {
    Path           string   // URL path pattern
    Method         string   // HTTP method (GET, POST, etc.)
    Public         bool     // Whether authentication is required
    RequiredScopes []string // Scopes required for access
    Priority       int      // Route priority (higher values = higher priority)
}
```

## Usage Examples

### Creating a Service Proxy

```go
// Create a service proxy with default options
serviceProxy, err := http.NewServiceProxy(
    http.WithProxyLogger(logger),
    http.WithCircuitBreaker(circuitBreaker),
    http.WithConfigDir("./config/services"),
)
```

### Registering a Service

```go
// Register a new service
err := serviceProxy.RegisterService(config.ServiceConfig{
    Name:            "user-service",
    URL:             "http://user-service:8080",
    HealthCheckPath: "/health",
    RequiresAuth:    true,
    AllowedMethods:  []string{"GET", "POST", "PUT", "DELETE"},
    Timeout:         5 * time.Second,
    RetryCount:      3,
})
```

### Registering a Custom Route

```go
// Register a custom route for a service
err := serviceProxy.RegisterServiceRoute("user-service", config.RouteConfig{
    Path:           "/users/public",
    Method:         "GET",
    Public:         true,
    RequiredScopes: nil,
    Priority:       10,
})
```

### Using Authentication Middleware

```go
// Create an authentication middleware
authMiddleware := http.NewAuthMiddleware(authManager)

// Apply to a handler
protectedHandler := authMiddleware.Authenticate(myHandler)

// Apply with specific authorization requirements
restrictedHandler := authMiddleware.Authenticate(
    authMiddleware.Authorize("admin", "user:write")(myHandler)
)
```

### Registering Handlers with Authentication

```go
// Create an authenticated proxy handler
authMiddlewareInstance := http.NewAuthMiddleware(authManager)
proxyHandler := serviceProxy.AuthenticatedHandler(authMiddlewareInstance.Authenticate)

// Register with router
router.Handle("/*", proxyHandler)
```

## Error Handling

The HTTP adapter defines and handles several categories of errors:

- **Service Proxy Errors** - Configuration, registry, and routing errors
- **Authentication/Authorization Errors** - Token validation, scope verification
- **Request Errors** - Invalid requests, rate limiting
- **Server Errors** - Internal errors, service unavailability

## Integration with App

The App struct integrates with the HTTP adapter through:

```go
// WithServiceProxy sets a custom service proxy for the App
func WithServiceProxy(proxy *adaptersHTTP.ServiceProxy) AppOption

// WithDefaultServiceProxy creates a default service proxy
func WithDefaultServiceProxy() AppOption
```

## Health Monitoring

The adapter provides a health check endpoint for monitoring the status of all services:

```
GET /api/services-health?check=services
```

This returns a JSON object with the health status of each service:

```json
{
  "user-service": "ok",
  "auth-service": "ok",
  "payment-service": "unhealthy"
}
```
