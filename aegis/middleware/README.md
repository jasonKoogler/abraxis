# Combined Authentication and Authorization Middleware

This package provides a unified middleware solution that combines authentication and authorization for your Go applications. It integrates the built-in authentication middleware with the powerful Authz library for fine-grained authorization.

## Features

- **Combined Authentication and Authorization**: Apply both authentication and authorization in a single middleware
- **Role-Based Access Control**: Protect routes based on user roles
- **Policy-Based Authorization**: Use OPA policies for fine-grained access control
- **Conditional Protection**: Skip authentication for public endpoints
- **Resource-Specific Authorization**: Apply authorization based on resource types
- **Ownership-Based Authorization**: Check if users own the resources they're accessing
- **Flexible Input Extractors**: Extract authorization inputs from various sources

## Installation

```bash
# The middleware is part of your project, no additional installation required
```

## Usage

### Basic Usage

```go
// Create the Authz adapter
authzAdapter, err := authz.New(authz.Config{
    RedisAddr:     "localhost:6379",
    RedisPassword: "",
    RedisDB:       0,
    CacheTTL:      time.Minute * 5,
    MaxCacheSize:  1000,
    Logger:        logger,
    LogLevel:      "debug",
})
if err != nil {
    logger.Fatal("Failed to create authz adapter", log.Error(err))
}

// Create the combined middleware
combinedMiddleware := middleware.NewCombinedMiddleware(authManager, authzAdapter)

// Apply the middleware to a route
router.With(combinedMiddleware.Protect(middleware.APIInputExtractor)).
    Get("/api/resources", resourceHandler)
```

### Conditional Protection

Skip authentication for public endpoints:

```go
// Define public paths that don't require authentication
publicPaths := []string{
    "/auth/login",
    "/auth/register",
    "/auth/google",
    "/auth/google/callback",
}

// Apply conditional protection to all routes
router.Use(combinedMiddleware.ConditionalProtect(
    middleware.APIInputExtractor,
    publicPaths,
))
```

### Role-Based Authorization

Protect routes based on user roles:

```go
// Protect a route with role-based authorization
router.With(combinedMiddleware.ProtectWithRoles("admin")).
    Get("/admin/dashboard", adminDashboardHandler)
```

### Policy-Based Authorization

Use OPA policies for fine-grained access control:

```go
// Protect a route with policy-based authorization
router.With(combinedMiddleware.ProtectWithPolicy(middleware.APIInputExtractor)).
    Get("/api/resources", resourcesHandler)
```

### Resource-Specific Authorization

Apply authorization based on resource types:

```go
// Define resource attributes
resourceAttrs := map[string]interface{}{
    "public": true,
}

// Protect a route with resource-specific authorization
router.With(combinedMiddleware.Protect(middleware.ResourceInputExtractor("document", resourceAttrs))).
    Get("/api/documents", documentsHandler)
```

### Ownership-Based Authorization

Check if users own the resources they're accessing:

```go
// Protect a route with ownership-based authorization
router.With(combinedMiddleware.Protect(middleware.OwnershipExtractor("userID"))).
    Get("/api/users/{userID}/documents", userDocumentsHandler)
```

## Input Extractors

The middleware provides several input extractors for different use cases:

- **StandardInputExtractor**: Extracts standard authorization input from an HTTP request
- **APIInputExtractor**: Extracts API-specific authorization input from an HTTP request
- **ResourceInputExtractor**: Creates an input extractor for a specific resource type
- **PathParamExtractor**: Creates an input extractor that includes URL path parameters
- **OwnershipExtractor**: Creates an input extractor that checks resource ownership
- **UserContextExtractor**: Creates an input extractor that pulls data from the authenticated user context

## Integration with App

```go
// In your app.go file:

// Add a field for the combined middleware
type App struct {
    // ... existing fields
    combinedMiddleware *middleware.CombinedMiddleware
}

// In your app constructor:
func NewApp(cfg *config.Config, userRepo domain.UserRepository) *App {
    // ... existing code

    // Create the Authz adapter
    authzAdapter, err := authz.New(authz.Config{
        RedisAddr:     cfg.Auth.AuthZ.RedisConfig.Host + ":" + cfg.Auth.AuthZ.RedisConfig.Port,
        RedisPassword: cfg.Auth.AuthZ.RedisConfig.Password,
        RedisDB:       0,
        CacheTTL:      time.Minute * 5,
        MaxCacheSize:  1000,
        Logger:        logger,
        LogLevel:      cfg.LogLevel.String(),
        Policies:      loadPolicies(cfg.Auth.AuthZ.PolicyFilePath),
        WebhookPath:   cfg.Auth.AuthZ.WebhookPath,
        WebhookSecret: cfg.Auth.AuthZ.GitHubToken,
    })
    if err != nil {
        logger.Fatal("Failed to create authz adapter", log.Error(err))
    }

    // Create the combined middleware
    combinedMiddleware := middleware.NewCombinedMiddleware(authService, authzAdapter)

    return &App{
        // ... existing fields
        combinedMiddleware: combinedMiddleware,
    }
}

// Add methods to apply the middleware
func (a *App) ApplyCombinedMiddleware(next http.Handler) http.Handler {
    return a.combinedMiddleware.Protect(middleware.APIInputExtractor)(next)
}

func (a *App) ApplyConditionalMiddleware(next http.Handler, publicPaths []string) http.Handler {
    return a.combinedMiddleware.ConditionalProtect(middleware.APIInputExtractor, publicPaths)(next)
}
```

## Integration with Routes

```go
// In your routes.go file:

func RegisterRoutes(router chi.Router, app *App) {
    // Public routes (no authentication required)
    router.Group(func(r chi.Router) {
        r.Post("/auth/login", app.authHandler.Login)
        r.Post("/auth/register", app.authHandler.Register)
    })

    // Protected routes (authentication required)
    router.Group(func(r chi.Router) {
        // Apply the combined middleware to all routes in this group
        r.Use(app.combinedMiddleware.Protect(middleware.APIInputExtractor))

        r.Get("/api/users", app.userHandler.ListUsers)
        r.Get("/api/users/{id}", app.userHandler.GetUser)
        r.Put("/api/users/{id}", app.userHandler.UpdateUser)
        r.Delete("/api/users/{id}", app.userHandler.DeleteUser)
    })

    // Admin-only routes
    router.Group(func(r chi.Router) {
        // Apply role-based protection
        r.Use(app.combinedMiddleware.ProtectWithRoles("admin"))

        r.Get("/admin/dashboard", app.adminHandler.Dashboard)
        r.Get("/admin/users", app.adminHandler.ListUsers)
    })

    // Resource-specific routes with ownership checks
    router.Group(func(r chi.Router) {
        // Apply ownership-based protection
        r.Use(app.combinedMiddleware.Protect(middleware.OwnershipExtractor("userID")))

        r.Get("/api/users/{userID}/documents", app.documentHandler.ListUserDocuments)
        r.Post("/api/users/{userID}/documents", app.documentHandler.CreateUserDocument)
    })
}
```

## Integration with Server

```go
// In your server.go file:

// Add a field for the combined middleware
type Server struct {
    // ... existing fields
    combinedMiddleware *middleware.CombinedMiddleware
}

// Add a server option for the combined middleware
func WithCombinedMiddleware(authManager *service.AuthManager, authzAdapter *authz.Adapter) ServerOption {
    return func(s *Server) error {
        s.combinedMiddleware = middleware.NewCombinedMiddleware(authManager, authzAdapter)
        return nil
    }
}

// Add methods to register protected handlers
func (s *Server) RegisterProtectedHandler(path string, handler http.Handler) {
    // Apply the combined middleware
    protectedHandler := s.combinedMiddleware.Protect(middleware.APIInputExtractor)(handler)
    s.router.Mount(path, protectedHandler)
}

func (s *Server) RegisterRoleProtectedHandler(path string, handler http.Handler, roles ...string) {
    // Apply the role-based middleware
    protectedHandler := s.combinedMiddleware.ProtectWithRoles(roles...)(handler)
    s.router.Mount(path, protectedHandler)
}

func (s *Server) RegisterPolicyProtectedHandler(path string, handler http.Handler) {
    // Apply the policy-based middleware
    protectedHandler := s.combinedMiddleware.ProtectWithPolicy(middleware.APIInputExtractor)(handler)
    s.router.Mount(path, protectedHandler)
}
```
