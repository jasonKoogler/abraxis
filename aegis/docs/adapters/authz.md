# Authorization (AuthZ) Adapter

## Overview

The Authorization (AuthZ) adapter provides policy-based access control for your application through integration with the Open Policy Agent (OPA). It allows you to define and enforce complex authorization rules separate from your application code, enabling fine-grained control over who can access which resources and what actions they can perform.

The adapter supports:

- Policy-based authorization decisions
- Role-based access control (RBAC)
- Attribute-based access control (ABAC)
- Redis-backed caching for performance
- Dynamic policy updates via webhooks
- Flexible context extraction from HTTP requests

## Port Interface

The adapter implements a simple and flexible authorization interface:

```go
// Adapter is the AuthZ adapter that handles policy enforcement
type Adapter struct {
    agent        *authz.Agent
    roleProvider types.RoleProvider
    config       Config
}

// Key methods:
// Evaluate evaluates the authorization policy for the given input
func (a *Adapter) Evaluate(ctx context.Context, input interface{}) (types.Decision, error)

// EvaluateQuery evaluates a custom query with the given input
func (a *Adapter) EvaluateQuery(ctx context.Context, query string, input interface{}) (types.Decision, error)

// Middleware returns an HTTP middleware that enforces authorization
func (a *Adapter) Middleware(extractInput func(*http.Request) (interface{}, error)) func(http.Handler) http.Handler
```

## Authorization Model

The authorization model is based on policy evaluation against input data with a structure like:

```json
{
  "user": {
    "id": "user123",
    "roles": ["admin", "editor"]
  },
  "resource": {
    "type": "document",
    "id": "doc456",
    "owner": "user123"
  },
  "action": "update",
  "context": {
    "ip": "192.168.1.1",
    "time": "2023-05-15T14:30:00Z"
  }
}
```

Policies are written in Rego, OPA's policy language, and return a decision structure:

```json
{
  "allowed": true,
  "reason": "User is the resource owner"
}
```

## Key Components

### Policy Evaluation

The core of the adapter is the policy evaluation engine that processes authorization requests:

- Takes structured input data representing the authorization context
- Evaluates it against defined policies written in Rego
- Returns a decision with allow/deny result and optional reason
- Caches results for improved performance

### Role Provider

The role provider component manages user roles:

- Retrieves roles for users from Redis or in-memory storage
- Supports hierarchical roles and role inheritance
- Injects roles into the authorization context during evaluation
- Provides default roles for users without explicit assignments

### Input Extractors

Built-in extractors convert HTTP requests into authorization input:

- `StandardInputExtractor`: Extracts basic user, resource, and action information
- `APIInputExtractor`: Adds API-specific context like API version

Custom extractors can be created for specialized needs.

### Middleware Integration

HTTP middleware simplifies integration with web applications:

- Seamlessly integrates with Go's standard HTTP handlers
- Extracts authorization context from requests
- Performs policy evaluation before handler execution
- Returns appropriate HTTP status codes for authorization failures

## Configuration Options

The adapter is configured using the `Config` structure:

```go
type Config struct {
    // Redis configuration
    RedisAddr     string
    RedisPassword string
    RedisDB       int

    // Cache configuration
    CacheTTL     time.Duration
    MaxCacheSize int

    // Policy configuration
    Policies map[string]string

    // Webhook configuration
    WebhookPath   string
    WebhookSecret string

    // Logger
    Logger   *log.Logger
    LogLevel string
}
```

## Usage Examples

### Creating the Authz Adapter

```go
// Create a logger
logger := log.NewLogger("debug")

// Create the Authz adapter
adapter, err := authz.New(authz.Config{
    RedisAddr:     "localhost:6379",
    RedisPassword: "",
    RedisDB:       0,
    CacheTTL:      time.Minute * 5,
    MaxCacheSize:  1000,
    Logger:        logger,
    WebhookPath:   "/api/policy-update",
    WebhookSecret: "my-secret-key",
    Policies: map[string]string{
        "authz.rego": `
            package authz

            default allow = false

            allow {
                input.user.roles[_] == "admin"
            }

            allow {
                input.action == "read"
                input.resource.owner == input.user.id
            }
        `,
    },
})
```

### Direct Policy Evaluation

```go
// Create input for evaluation
input := map[string]interface{}{
    "user": map[string]interface{}{
        "id":    "user123",
        "roles": []string{"editor"},
    },
    "resource": map[string]interface{}{
        "type":  "document",
        "id":    "doc456",
        "owner": "user789",
    },
    "action": "update",
}

// Evaluate the policy
decision, err := adapter.Evaluate(ctx, input)
if err != nil {
    log.Errorf("Policy evaluation error: %v", err)
    return
}

// Check the decision
if decision.Allowed {
    fmt.Printf("Access allowed: %s\n", decision.Reason)
} else {
    fmt.Printf("Access denied: %s\n", decision.Reason)
}
```

### Using Middleware for HTTP Handlers

```go
// Create HTTP router
mux := http.NewServeMux()

// Create protected handler
documentHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Handler logic - only executed if authorization succeeds
    fmt.Fprintf(w, "Document content")
})

// Apply authorization middleware with API input extractor
mux.Handle("/api/documents/", adapter.Middleware(authz.APIInputExtractor)(documentHandler))

// Start server
http.ListenAndServe(":8080", mux)
```

### Custom Input Extractor

```go
// Custom extractor for document-specific authorization
func DocumentInputExtractor(r *http.Request) (interface{}, error) {
    // Get user ID from request context
    userID := getUserIDFromContext(r.Context())
    if userID == "" {
        return nil, fmt.Errorf("user not authenticated")
    }

    // Extract document ID from URL path
    documentID := getDocumentIDFromPath(r.URL.Path)

    // Determine action from HTTP method
    action := httpMethodToAction(r.Method)

    // Create input object
    return map[string]interface{}{
        "user": map[string]interface{}{
            "id": userID,
        },
        "resource": map[string]interface{}{
            "type": "document",
            "id":   documentID,
        },
        "action": action,
    }, nil
}

// Apply middleware with custom extractor
mux.Handle("/documents/", adapter.Middleware(DocumentInputExtractor)(documentHandler))
```

### Policy Webhook for Dynamic Updates

```go
// Register webhook handler
adapter.RegisterWebhook(mux)

// Use curl to update policies:
// curl -X POST http://localhost:8080/api/policy-update \
//   -H 'Authorization: Bearer my-secret-key' \
//   -H 'Content-Type: application/json' \
//   -d '{"policies": {"authz.rego": "package authz\n\ndefault allow = false\n\nallow {\n  input.user.roles[_] == \"admin\"\n}"}}'
```

## Writing Policies

Policies are written in Rego, OPA's policy language. Here's an example of a more complex policy:

```rego
package authz

# Default to denying access
default allow = false
default reason = "Access denied by default"

# Define the decision object
decision = {
    "allowed": allow,
    "reason": reason
}

# Admin can do anything
allow {
    input.user.roles[_] == "admin"
}
reason = "User has admin role" {
    input.user.roles[_] == "admin"
}

# Users can read and update their own resources
allow {
    # Only allow read and update actions
    input.action == "read" or input.action == "update"

    # Check if user is the resource owner
    input.resource.owner == input.user.id
}
reason = "User is the resource owner" {
    input.action == "read" or input.action == "update"
    input.resource.owner == input.user.id
}

# Users with editor role can update any resource
allow {
    input.action == "update"
    input.user.roles[_] == "editor"
}
reason = "User has editor role" {
    input.action == "update"
    input.user.roles[_] == "editor"
}

# Anyone can read public resources
allow {
    input.action == "read"
    input.resource.public == true
}
reason = "Resource is public" {
    input.action == "read"
    input.resource.public == true
}
```

## Performance Considerations

### Caching

The adapter uses caching to improve performance:

- In-memory caching for single-instance deployments
- Redis-based caching for distributed environments
- Configurable TTL for cache entries
- Maximum cache size to prevent memory exhaustion

### Redis Role Provider

When using Redis:

- Roles are cached to reduce Redis queries
- Default TTL for role cache can be configured
- Default roles are provided for users without explicit assignments

## Integration with App

The AuthZ adapter integrates with the application through middleware:

```go
// Create HTTP router
router := chi.NewRouter()

// Create authorization middleware
authMiddleware := adapter.Middleware(authz.StandardInputExtractor)

// Apply middleware to specific routes
router.Group(func(r chi.Router) {
    r.Use(authMiddleware)
    r.Get("/api/documents", listDocumentsHandler)
    r.Post("/api/documents", createDocumentHandler)
})

// Or apply to individual routes
router.With(authMiddleware).Get("/api/settings", settingsHandler)
```

## Security Best Practices

1. **Least Privilege**: Design policies with least privilege principle
2. **Defense in Depth**: Use authorization in addition to authentication
3. **Explicit Denies**: Prefer explicit deny rules over implicit allows
4. **Policy Testing**: Test authorization policies thoroughly
5. **Secure Webhook**: Use strong webhook secrets and HTTPS
6. **Audit Logging**: Log authorization decisions for security analysis
7. **Regular Reviews**: Review policies regularly for needed adjustments
