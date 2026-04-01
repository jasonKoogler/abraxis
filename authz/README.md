# Authz - OPA Authorization Framework for Go

Authz is a powerful, flexible authorization framework for Go applications that integrates seamlessly with [Open Policy Agent (OPA)](https://www.openpolicyagent.org/). It provides a clean, idiomatic API for implementing authorization in your services with robust caching, middleware support, dynamic policy updates, and flexible role management.

![Version](https://img.shields.io/badge/version-0.1.0--alpha-blue)
![Go Version](https://img.shields.io/badge/go-%3E%3D1.16-blue)

## Features

- **Flexible Policy Evaluation**

  - Local policy evaluation using embedded OPA
  - External policy evaluation using OPA server
  - Support for custom policy queries and paths

- **Optimized Performance**

  - Multi-tiered caching system
  - In-memory LRU cache with size limiting
  - Redis-based distributed cache
  - Partial input matching for efficient caching
  - Configurable TTL and eviction policies

- **Role-Based Access Control**

  - Pluggable role provider interface
  - Redis role provider included
  - Dynamic role resolution at runtime
  - Context transformers for authorization enrichment

- **HTTP Integration**

  - Middleware for securing HTTP endpoints
  - Custom input extractors
  - Integration with any HTTP router

- **Dynamic Policies**

  - Webhook endpoint for policy updates
  - Hot-reloading of policy rules
  - Signature verification for secure updates

- **Developer Experience**
  - Functional options API
  - Clear error messages
  - Comprehensive logging
  - Detailed examples

## Requirements

- Go 1.16 or later
- [Open Policy Agent](https://www.openpolicyagent.org/) (embedded or external)
- For Redis features:
  - Redis server
  - [github.com/redis/go-redis/v9](https://github.com/redis/go-redis)

## Installation

```bash
go get github.com/jasonKoogler/abraxis/authz
```

## Quick Start

This minimal example shows how to integrate Authz into a Go application:

```go
package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/jasonKoogler/abraxis/authz"
)

func main() {
    // Define a simple policy
    policies := map[string]string{
        "authz.rego": `
            package authz

            default allow = false

            allow {
                input.user.roles[_] == "admin"
            }
        `,
    }

    // Create a new authz instance
    a, err := authz.New(
        authz.WithLocalPolicies(policies),
        authz.WithMemoryCache(time.Minute*5, 1000), // 5 min TTL, max 1000 entries
        authz.WithLogger(log.New(os.Stdout, "[AUTHZ] ", log.LstdFlags)),
    )

    if err != nil {
        log.Fatalf("Failed to create authz: %v", err)
    }

    // Create an HTTP server with a protected endpoint
    mux := http.NewServeMux()

    // Protected endpoint with middleware
    protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Access granted!")
    })

    // Apply the middleware
    mux.Handle("/api/protected", a.Middleware(extractInput)(protectedHandler))

    // Start the server
    http.ListenAndServe(":8080", mux)
}

// extractInput extracts authorization input from an HTTP request
func extractInput(r *http.Request) (interface{}, error) {
    return map[string]interface{}{
        "user": map[string]interface{}{
            "id":    r.Header.Get("X-User-ID"),
            "roles": []string{"admin"},
        },
        "resource": map[string]interface{}{
            "path": r.URL.Path,
        },
        "action": r.Method,
    }, nil
}
```

Test with:

```bash
curl -H "X-User-ID: alice" http://localhost:8080/api/protected
```

## Architecture Overview

Authz is designed with a modular architecture that allows for flexible configuration and extension:

1. **Agent**: The core component that manages policy evaluation, caching, and integration with external systems.
2. **Policy Evaluation**: Supports both local (embedded) and external (OPA server) policy evaluation.
3. **Caching**: Provides multiple caching strategies to optimize performance.
4. **Role Management**: Offers a pluggable role provider interface for dynamic role resolution.
5. **HTTP Integration**: Includes middleware for securing HTTP endpoints.
6. **Dynamic Updates**: Supports webhook-based policy updates for runtime flexibility.

## Configuration Options

Authz uses the functional options pattern for configuration:

### Policy Options

```go
// Local policy evaluation
authz.WithLocalPolicies(map[string]string{
    "policy.rego": `package authz; default allow = false; ...`,
})

// External OPA server
authz.WithExternalOPA("http://opa-server:8181")

// Custom query path
authz.WithDefaultQuery("data.myapp.authz.allow")
```

### Caching Options

```go
// In-memory cache with LRU eviction
authz.WithMemoryCache(time.Minute*5, 1000)

// No caching
authz.WithNoCache()

// External Redis cache
redisCache := cache.NewRedisCache(redisClient,
    cache.WithKeyPrefix("authz:policy:cache:"),
)
authz.WithExternalCache(redisCache, time.Minute*15)

// Selective field caching
authz.WithCacheKeyFields([]string{
    "user.roles",
    "action",
    "resource.type",
})
```

### Role Management

```go
// Create Redis role provider
roleProvider := roles.NewRedisRoleProvider(redisClient,
    roles.WithKeyPrefix("authz:roles:"),
    roles.WithDefaultTTL(time.Hour*24),
    roles.WithDefaultRoles([]string{"viewer"}),
)

// Add to authz configuration
authz.WithRoleProvider(roleProvider)

// Create and add a role transformer
roleTransformer := roles.CreateRoleTransformer(roleProvider)
authz.WithContextTransformer(roleTransformer)
```

### Webhook for Policy Updates

```go
// Enable policy update webhook
authz.WithWebhook("/api/policy-update", "my-secret-key", nil)

// Register the webhook handler
mux := http.NewServeMux()
a.RegisterWebhook(mux)
```

### Logging

```go
// Configure logging
logger := log.New(os.Stdout, "[AUTHZ] ", log.LstdFlags)
authz.WithLogger(logger)
```

## Detailed Usage Guide

### HTTP Middleware

The middleware makes it easy to protect HTTP endpoints:

```go
// Define function to extract input from requests
func extractInput(r *http.Request) (interface{}, error) {
    // Extract user ID from request
    userID := r.Header.Get("X-User-ID")
    if userID == "" {
        return nil, fmt.Errorf("missing user ID")
    }

    // Extract resource type from query parameters
    resourceType := r.URL.Query().Get("type")
    if resourceType == "" {
        resourceType = "default"
    }

    // Map HTTP method to action
    action := "read"
    switch r.Method {
    case http.MethodPost, http.MethodPut, http.MethodPatch:
        action = "write"
    case http.MethodDelete:
        action = "delete"
    }

    // Build input object
    input := map[string]interface{}{
        "user": map[string]interface{}{
            "id": userID,
            // Roles will be added by role provider if configured
        },
        "resource": map[string]interface{}{
            "type": resourceType,
            "path": r.URL.Path,
        },
        "action": action,
    }

    return input, nil
}

// Apply middleware to routes
mux.Handle("/api/protected", a.Middleware(extractInput)(protectedHandler))
```

### Direct Policy Evaluation

You can also evaluate policies directly in your code:

```go
// Create input for evaluation
input := map[string]interface{}{
    "user": map[string]interface{}{
        "id":    "alice",
        "roles": []string{"admin"},
    },
    "resource": map[string]interface{}{
        "type": "financial",
        "id":   "report-123",
    },
    "action": "read",
}

// Direct policy evaluation
decision, err := a.Evaluate(context.Background(), input)
if err != nil {
    log.Printf("Error evaluating policy: %v", err)
    return
}

// Use the decision
if decision.Allowed {
    // Allow access
} else {
    // Deny access with reason
    log.Printf("Access denied: %s", decision.Reason)
}
```

### Custom Query Evaluation

You can evaluate specific queries beyond the default one:

```go
// Evaluate a custom query
decision, err := a.EvaluateQuery(ctx, "data.authz.custom_rule", input)
if err != nil {
    log.Printf("Error evaluating custom query: %v", err)
    return
}

// Use the decision
if decision.Allowed {
    // Allow access
} else {
    // Deny access
    log.Printf("Access denied: %s", decision.Reason)
}
```

### Role Provider Implementation

The Redis role provider stores user roles in Redis:

```go
// Create the Redis role provider
roleProvider := roles.NewRedisRoleProvider(redisClient,
    roles.WithKeyPrefix("authz:roles:"),
    roles.WithDefaultTTL(time.Hour*24),
    roles.WithDefaultRoles([]string{"viewer"}),
)

// Set roles for a user
err := roleProvider.SetRoles(ctx, "alice", []string{"admin", "finance"})
if err != nil {
    log.Printf("Error setting roles: %v", err)
}

// Get roles for a user
userRoles, err := roleProvider.GetRoles(ctx, "alice")
if err != nil {
    log.Printf("Error getting roles: %v", err)
}
fmt.Println("User roles:", userRoles)

// Delete roles for a user
err = roleProvider.DeleteRoles(ctx, "alice")
if err != nil {
    log.Printf("Error deleting roles: %v", err)
}
```

### Context Transformers

Context transformers allow you to modify the input before policy evaluation:

```go
// Create a custom context transformer
myTransformer := func(ctx context.Context, input interface{}) (interface{}, error) {
    // Cast input to map
    inputMap, ok := input.(map[string]interface{})
    if !ok {
        return input, nil
    }

    // Add custom context information
    inputMap["context"] = map[string]interface{}{
        "time": time.Now().Format(time.RFC3339),
        "environment": os.Getenv("APP_ENV"),
    }

    return inputMap, nil
}

// Add the transformer to the agent
authz.WithContextTransformer(myTransformer)
```

### Custom Cache Implementation

You can create your own cache implementation by implementing the `types.Cache` interface:

```go
// Create a custom cache implementation
type MyCustomCache struct {
    // Your fields here
}

// Implement the Cache interface
func (c *MyCustomCache) Get(key string) (types.Decision, bool) {
    // Your implementation
}

func (c *MyCustomCache) Set(key string, decision types.Decision, ttl time.Duration) {
    // Your implementation
}

func (c *MyCustomCache) Delete(key string) {
    // Your implementation
}

func (c *MyCustomCache) Clear() {
    // Your implementation
}

// Use with Authz
authz.WithExternalCache(myCache, time.Minute*10)
```

## Writing Effective OPA Policies

Here are some examples of OPA policies for different authorization patterns:

### Simple RBAC Policy

```rego
package authz

default allow = false

allow {
    # Get roles assigned to user
    role := input.user.roles[_]

    # Check if role has permission for the action and resource
    role_permissions[role][input.action][input.resource.type]
}

# Role definitions
role_permissions = {
    "admin": {
        "read": {"financial": true, "hr": true},
        "write": {"financial": true, "hr": true}
    },
    "viewer": {
        "read": {"financial": true},
        "write": {}
    }
}
```

### ABAC Policy (Attribute-Based Access Control)

```rego
package authz

default allow = false

# Allow access based on various attributes
allow {
    # Department-based access
    input.user.department == "finance"
    input.resource.type == "financial"
}

allow {
    # Owner-based access
    input.resource.owner == input.user.id
}

allow {
    # Time-based access
    time.parse_rfc3339_ns(input.context.time) > time.parse_rfc3339_ns(input.resource.public_after)
}
```

### Policy with Custom Response

```rego
package authz

default response = {"allowed": false, "reason": "Default deny"}

response = {"allowed": true} {
    input.user.roles[_] == "admin"
}

response = {"allowed": true} {
    input.user.roles[_] == "viewer"
    input.action == "read"
}

response = {"allowed": false, "reason": "Insufficient privileges"} {
    input.user.roles[_] == "viewer"
    input.action == "write"
}
```

### Hierarchical RBAC

```rego
package authz

default allow = false

# Define role hierarchy
role_hierarchy = {
    "admin": ["manager", "viewer"],
    "manager": ["viewer"],
    "viewer": []
}

# Check if user has a role or any of its parent roles
has_role(user_roles, role) {
    # Direct role match
    user_roles[_] == role
}

has_role(user_roles, role) {
    # Check parent roles
    parent_role := role_hierarchy[parent_role][_]
    user_roles[_] == parent_role
}

# Allow access based on role hierarchy
allow {
    # Get user roles
    user_roles := input.user.roles

    # Check if user has admin role or any parent role
    has_role(user_roles, "admin")
}

allow {
    # Get user roles
    user_roles := input.user.roles

    # Check if user has viewer role or any parent role
    has_role(user_roles, "viewer")

    # Viewers can only read
    input.action == "read"
}
```

## Webhook for Policy Updates

You can update policies dynamically via the webhook:

```bash
curl -X POST \
  http://localhost:8080/api/policy-update \
  -H "Content-Type: application/json" \
  -d '{
    "policies": {
      "authz.rego": "package authz\n\ndefault allow = false\n\nallow {\n    input.user.roles[_] == \"admin\"\n}"
    },
    "signature": "your-signature-here"
  }'
```

The webhook handler validates the request and updates the policies in real-time, without requiring a service restart.

## Advanced Caching Strategies

### Partial Key Matching

Partial key matching allows you to cache based on specific fields that affect the decision:

```go
// Only cache based on these fields
authz.WithCacheKeyFields([]string{
    "user.roles",
    "action",
    "resource.type"
})
```

This is particularly useful when:

- Some input fields don't affect the decision (e.g., request timestamps)
- You want to maximize cache hits by ignoring irrelevant fields
- You need to handle high-cardinality fields that would otherwise reduce cache efficiency

### Redis Cache Configuration

When using Redis for distributed caching:

```go
// Create Redis cache with custom options
redisCache := cache.NewRedisCache(redisClient,
    cache.WithKeyPrefix("authz:policy:cache:"),
    cache.WithSerializer(cache.JSONSerializer{}),
    cache.WithErrorHandler(func(err error) {
        log.Printf("Redis cache error: %v", err)
    }),
)

// Use with Authz
authz.WithExternalCache(redisCache, time.Minute*15)
```

## Complete Examples

### Basic Example

The basic example demonstrates a simple setup with in-memory caching:

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/jasonKoogler/abraxis/authz"
)

func main() {
    // Create a logger
    logger := log.New(os.Stdout, "[AGENT] ", log.LstdFlags)

    // Define local policies
    policies := map[string]string{
        "authz.rego": `
package authz

default allow = false

# Simple role-based access control
allow {
    input.user.roles[_] == "admin"
}

allow {
    input.user.roles[_] == "viewer"
    input.action == "read"
}
`,
    }

    // Create a new Agent instance with memory cache
    a, err := authz.New(
        authz.WithLocalPolicies(policies),
        authz.WithDefaultQuery("data.authz.allow"),
        authz.WithLogger(logger),
        authz.WithMemoryCache(time.Minute*5, 1000), // 5 minute TTL, max 1000 entries
    )

    if err != nil {
        logger.Fatalf("Failed to create agent: %v", err)
    }

    // Create HTTP server
    mux := http.NewServeMux()

    // Protected endpoint with middleware
    protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"message": "Access granted!"})
    })

    // Apply middleware
    mux.Handle("/api/protected", a.Middleware(extractInput)(protectedHandler))

    // Start server
    logger.Println("Starting server on :8080...")
    if err := http.ListenAndServe(":8080", mux); err != nil {
        logger.Fatalf("Server error: %v", err)
    }
}

// extractInput extracts the input for policy evaluation from the request
func extractInput(r *http.Request) (interface{}, error) {
    // Extract user ID from header
    userID := r.Header.Get("X-User-ID")
    if userID == "" {
        return nil, fmt.Errorf("missing user ID")
    }

    // Parse roles from header
    roles := []string{}
    roleHeader := r.Header.Get("X-User-Roles")
    if roleHeader != "" {
        if err := json.Unmarshal([]byte(roleHeader), &roles); err != nil {
            return nil, fmt.Errorf("invalid roles format")
        }
    }

    // Map HTTP method to action
    action := "read"
    switch r.Method {
    case http.MethodPost, http.MethodPut, http.MethodPatch:
        action = "write"
    case http.MethodDelete:
        action = "delete"
    }

    // Build input object
    input := map[string]interface{}{
        "user": map[string]interface{}{
            "id":    userID,
            "roles": roles,
        },
        "resource": map[string]interface{}{
            "path": r.URL.Path,
        },
        "action": action,
    }

    return input, nil
}
```

### Advanced Example

The advanced example demonstrates a full-featured setup with Redis integration:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/jasonKoogler/abraxis/authz"
    "github.com/jasonKoogler/abraxis/authz/cache"
    "github.com/jasonKoogler/abraxis/authz/roles"
    "github.com/redis/go-redis/v9"
)

func main() {
    // Create a logger
    logger := log.New(os.Stdout, "[AGENT] ", log.LstdFlags)

    // Define local policies
    policies := map[string]string{
        "authz.rego": `
package authz

default allow = false

# RBAC rules
allow {
    # Get roles assigned to user
    role := input.user.roles[_]

    # Check if role has permission for the action and resource
    role_permissions[role][input.action][input.resource.type]
}

# Role definitions
role_permissions = {
    "admin": {
        "read": {"financial": true, "hr": true},
        "write": {"financial": true, "hr": true}
    },
    "viewer": {
        "read": {"financial": true},
        "write": {}
    }
}
`,
    }

    // Set up Redis client
    redisClient := redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "", // no password set
        DB:       0,  // use default DB
    })

    // Create Redis role provider
    roleProvider := roles.NewRedisRoleProvider(redisClient,
        roles.WithKeyPrefix("auth:roles:"),
        roles.WithDefaultTTL(time.Hour*24),
        roles.WithDefaultRoles([]string{"viewer"}),
    )

    // Seed some example roles
    ctx := context.Background()
    roleProvider.SetRoles(ctx, "alice", []string{"admin"})
    roleProvider.SetRoles(ctx, "bob", []string{"viewer"})

    // Create role transformer
    roleTransformer := roles.CreateRoleTransformer(roleProvider)

    // Create Redis cache
    redisCache := cache.NewRedisCache(redisClient,
        cache.WithKeyPrefix("agent:policy:cache:"),
    )

    // Create a new Agent instance with all the bells and whistles
    a, err := authz.New(
        authz.WithLocalPolicies(policies),
        authz.WithDefaultQuery("data.authz.allow"),
        authz.WithLogger(logger),
        authz.WithExternalCache(redisCache, time.Minute*15),                         // Use Redis cache
        authz.WithCacheKeyFields([]string{"user.roles", "action", "resource.type"}), // Partial caching
        authz.WithWebhook("/api/policy-update", "my-secret-key", nil),
        authz.WithRoleProvider(roleProvider),
        authz.WithContextTransformer(roleTransformer),
    )

    if err != nil {
        logger.Fatalf("Failed to create agent: %v", err)
    }

    // Create HTTP server
    mux := http.NewServeMux()

    // Register webhook handler
    a.RegisterWebhook(mux)

    // Protected endpoint with middleware
    protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"message": "Access granted!"})
    })

    // Apply middleware
    mux.Handle("/api/protected", a.Middleware(extractInput)(protectedHandler))

    // Start server
    logger.Println("Starting server on :8080...")
    if err := http.ListenAndServe(":8080", mux); err != nil {
        logger.Fatalf("Server error: %v", err)
    }
}

// extractInput extracts the input for policy evaluation from the request
func extractInput(r *http.Request) (interface{}, error) {
    // Extract user ID from header
    userID := r.Header.Get("X-User-ID")
    if userID == "" {
        return nil, fmt.Errorf("missing user ID")
    }

    // Extract resource type from query parameters
    resourceType := r.URL.Query().Get("type")
    if resourceType == "" {
        resourceType = "default"
    }

    // Map HTTP method to action
    action := "read"
    switch r.Method {
    case http.MethodPost, http.MethodPut, http.MethodPatch:
        action = "write"
    case http.MethodDelete:
        action = "delete"
    }

    // Build input object
    input := map[string]any{
        "user": map[string]any{
            "id": userID,
            // Roles will be added by the role provider
        },
        "resource": map[string]any{
            "type": resourceType,
            "path": r.URL.Path,
        },
        "action": action,
        "context": map[string]any{
            "time": time.Now().Format(time.RFC3339),
        },
    }

    return input, nil
}
```

## Performance Considerations

To get the best performance from Authz:

1. **Use caching effectively**: Configure TTL and max entries based on your workload

   - For high-throughput services, use larger cache sizes
   - For frequently changing policies, use shorter TTLs
   - For distributed systems, use Redis caching

2. **Configure partial key matching**: Include only fields that affect the decision

   - This can dramatically improve cache hit rates
   - Example: `WithCacheKeyFields([]string{"user.roles", "action", "resource.type"})`

3. **Choose the right cache backend**:

   - Use in-memory for speed in single-instance deployments
   - Use Redis for distributed systems
   - Consider custom cache implementations for special requirements

4. **Optimize policy evaluation**:

   - Keep policies simple and efficient
   - Use indexing in Rego policies for better performance
   - Avoid complex computations in policies

5. **Use context transformers wisely**:

   - Heavy transformations can impact performance
   - Consider caching transformed inputs separately

6. **Monitor cache hit rates**:
   - Track the `Cached` field in decisions to measure effectiveness
   - Adjust caching strategy based on hit rates

## Troubleshooting

### Common Issues

1. **Missing Roles**: Check that your role provider is correctly configured

   - Verify Redis connection settings
   - Check key prefixes and TTL settings
   - Ensure roles are being set correctly

2. **Cache Not Working**: Verify TTL and key generation settings

   - Check that cache implementation is correctly initialized
   - Verify that cache keys are being generated consistently
   - For Redis cache, check connection and serialization

3. **OPA Connection Issues**: Check network connectivity to external OPA

   - Verify OPA server URL is correct
   - Check that OPA server is running and accessible
   - Inspect network logs for connection issues

4. **Policy Update Failures**: Check webhook configuration and signature validation

   - Verify webhook endpoint is registered correctly
   - Check secret key configuration
   - Ensure policies are valid Rego syntax

5. **Authorization Failures**: Debug policy evaluation
   - Check input structure matches what policies expect
   - Verify roles are being correctly resolved
   - Test policies directly with OPA playground

### Debugging

Enable detailed logging to help troubleshoot issues:

```go
logger := log.New(os.Stdout, "[AUTHZ] ", log.LstdFlags|log.Lshortfile)
authz.WithLogger(logger)
```

## Security Considerations

1. **Policy Updates**: Use the webhook secret to prevent unauthorized updates

   - Use a strong, unique secret key
   - Consider implementing additional authentication
   - Restrict webhook access by IP if possible

2. **Role Management**: Implement proper authentication for role assignment

   - Secure the Redis instance storing roles
   - Implement audit logging for role changes
   - Consider role expiration for temporary access

3. **Cache Keys**: Be careful with partial key matching to avoid authorization bypasses

   - Always include security-critical fields in cache keys
   - Regularly audit cache key field selection
   - Consider cache poisoning risks in distributed environments

4. **Authorization Context**: Validate and sanitize inputs from clients

   - Never trust client-provided role information
   - Validate all input fields before policy evaluation
   - Consider input normalization for consistent evaluation

5. **External OPA**: Secure communication with external OPA servers
   - Use TLS for all communication
   - Implement mutual TLS for service authentication
   - Consider network-level security (firewalls, VPCs)

## Extending Authz

### Creating Custom Role Providers

Implement the `types.RoleProvider` interface to create custom role providers:

```go
type MyRoleProvider struct {
    // Your fields here
}

func (p *MyRoleProvider) GetRoles(ctx context.Context, userID string) ([]string, error) {
    // Your implementation to fetch roles
    // Could query a database, call an external service, etc.
    return []string{"role1", "role2"}, nil
}

// Use with Authz
authz.WithRoleProvider(myRoleProvider)
```

### Custom Decision Types

You can extend the decision type by processing the OPA response:

```go
// Create a custom context transformer that processes decisions
customDecisionTransformer := func(ctx context.Context, input interface{}) (interface{}, error) {
    // Add a custom field to indicate this is a special evaluation
    inputMap, ok := input.(map[string]interface{})
    if !ok {
        return input, nil
    }

    // Add a marker to tell our policy to return extended information
    inputMap["_internal"] = map[string]interface{}{
        "returnExtendedDecision": true,
    }

    return inputMap, nil
}

// Use with Authz
authz.WithContextTransformer(customDecisionTransformer)
```

Then in your Rego policy:

```rego
package authz

default response = {"allowed": false}

# Return extended decision when requested
response = {
    "allowed": true,
    "permissions": ["read", "write"],
    "expiration": time.add_date(time.now_ns(), 0, 0, 1),
    "resource_types": ["document", "report"]
} {
    input.user.roles[_] == "admin"
    input._internal.returnExtendedDecision == true
}

# Regular decision otherwise
response = {"allowed": true} {
    input.user.roles[_] == "admin"
    not input._internal.returnExtendedDecision == true
}
```

## Acknowledgments

- [Open Policy Agent](https://www.openpolicyagent.org/) for the powerful policy engine
- [Go Redis](https://github.com/redis/go-redis) for the Redis client

## Versioning and Releases

This project follows [Semantic Versioning](https://semver.org/).

- **MAJOR** version for incompatible API changes
- **MINOR** version for new functionality in a backward compatible manner
- **PATCH** version for backward compatible bug fixes

### Release History

See the [CHANGELOG.md](CHANGELOG.md) file for details on all releases.

### Stability Guarantees

- **< 1.0.0**: No stability guarantees. API may change without notice.
- **>= 1.0.0**: Backward compatibility within the same major version.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details on how to contribute to this project.
