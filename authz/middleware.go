package authz

import (
	"errors"
	"net/http"

	"github.com/jasonKoogler/authz/types"
)

// MiddlewareOptions contains options for the middleware
type MiddlewareOptions struct {
	// InputTransformer transforms an HTTP request into an input object for policy evaluation
	InputTransformer func(r *http.Request) map[string]interface{}

	// UnauthorizedHandler is called when a request is not authorized
	UnauthorizedHandler func(w http.ResponseWriter, r *http.Request)

	// CacheKeyFn generates a cache key for a request
	CacheKeyFn func(r *http.Request) string
}

// MiddlewareOption is a function that configures MiddlewareOptions
type MiddlewareOption func(*MiddlewareOptions)

// WithInputTransformer sets a custom input transformer for the middleware.
//
// The input transformer converts an HTTP request into a map that will be used
// as input for policy evaluation. This allows you to customize what data from
// the request is considered during authorization decisions.
//
// Example:
//
//	WithInputTransformer(func(r *http.Request) map[string]interface{} {
//	    return map[string]interface{}{
//	        "method": r.Method,
//	        "path": r.URL.Path,
//	        "user_id": r.Header.Get("X-User-ID"),
//	        "tenant": r.Header.Get("X-Tenant"),
//	    }
//	})
func WithInputTransformer(fn func(r *http.Request) map[string]interface{}) MiddlewareOption {
	return func(opts *MiddlewareOptions) {
		opts.InputTransformer = fn
	}
}

// WithUnauthorizedHandler sets a custom handler for unauthorized requests.
//
// This handler is called when a request is not authorized according to the policy.
// It allows you to customize the response sent back to the client, such as
// returning a JSON error response instead of a plain text error.
//
// Example:
//
//	WithUnauthorizedHandler(func(w http.ResponseWriter, r *http.Request) {
//	    w.Header().Set("Content-Type", "application/json")
//	    w.WriteHeader(http.StatusForbidden)
//	    json.NewEncoder(w).Encode(map[string]string{
//	        "error": "Forbidden",
//	        "message": "You don't have permission to access this resource",
//	        "path": r.URL.Path,
//	    })
//	})
func WithUnauthorizedHandler(fn func(w http.ResponseWriter, r *http.Request)) MiddlewareOption {
	return func(opts *MiddlewareOptions) {
		opts.UnauthorizedHandler = fn
	}
}

// WithCacheKey sets a custom cache key function for the middleware.
//
// The cache key function generates a string key that will be used to cache
// authorization decisions. This allows you to control the caching behavior
// based on request attributes.
//
// Example:
//
//	WithCacheKey(func(r *http.Request) string {
//	    return fmt.Sprintf("auth:%s:%s:%s",
//	        r.Method,
//	        r.URL.Path,
//	        r.Header.Get("X-User-ID"))
//	})
func WithCacheKey(fn func(r *http.Request) string) MiddlewareOption {
	return func(opts *MiddlewareOptions) {
		opts.CacheKeyFn = fn
	}
}

// defaultInputTransformer is the default function to transform an HTTP request into an input object.
//
// It extracts the following information from the request:
// - HTTP method
// - URL path
// - User roles from the X-User-Roles header (if present)
//
// This provides a basic input structure that works with simple authorization policies.
func defaultInputTransformer(r *http.Request) map[string]interface{} {
	input := map[string]interface{}{
		"method":  r.Method,
		"path":    r.URL.Path,
		"headers": map[string]interface{}{},
	}

	// Add user roles if present in the X-User-Roles header
	if roles := r.Header.Get("X-User-Roles"); roles != "" {
		input["user"] = map[string]interface{}{
			"roles": []string{roles},
		}
	}

	return input
}

// defaultUnauthorizedHandler is the default handler for unauthorized requests.
//
// It returns a simple "Forbidden" error with HTTP status code 403.
func defaultUnauthorizedHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Forbidden", http.StatusForbidden)
}

// RequireAuth creates a middleware that requires authorization for requests.
//
// This is a high-level, user-friendly function designed for most common use cases.
// It provides sensible defaults and configuration options through functional options.
//
// Use RequireAuth when:
// - You want a simple, declarative way to add authorization to your HTTP handlers
// - You need customizable input transformation from HTTP requests to policy inputs
// - You want to customize how unauthorized requests are handled
// - You need to integrate with caching through custom cache keys
//
// Example:
//
//	// Basic usage with defaults
//	router.Use(RequireAuth(agent))
//
//	// With custom input transformer
//	router.Use(RequireAuth(agent, WithInputTransformer(myTransformer)))
//
//	// With custom unauthorized handler
//	router.Use(RequireAuth(agent, WithUnauthorizedHandler(myHandler)))
func RequireAuth(agent *Agent, options ...MiddlewareOption) func(http.Handler) http.Handler {
	// Initialize options with defaults
	opts := &MiddlewareOptions{
		InputTransformer:    defaultInputTransformer,
		UnauthorizedHandler: defaultUnauthorizedHandler,
	}

	// Apply options
	for _, option := range options {
		option(opts)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Transform request to input
			input := opts.InputTransformer(r)

			// Evaluate policy
			var decision types.Decision
			var err error

			// For now, we'll just use the agent's Evaluate method directly
			// The agent handles caching internally based on its configuration
			decision, err = agent.Evaluate(r.Context(), input)

			if err != nil {
				agent.config.Logger.Printf("Error evaluating policy: %v", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !decision.Allowed {
				opts.UnauthorizedHandler(w, r)
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// Middleware returns an http.Handler middleware for authorization.
//
// This is a lower-level method that provides more flexibility but requires more
// knowledge of the system. Unlike RequireAuth, it doesn't provide defaults or
// configuration options, but allows for complete customization of input extraction.
//
// Use Middleware when:
// - You need complete control over how inputs are extracted from HTTP requests
// - You want to leverage the Agent's role provider for automatic role enrichment
// - You need detailed error handling with specific error messages
// - You're building a custom middleware on top of the core authorization functionality
//
// Example:
//
//	// Custom input extractor
//	extractInput := func(r *http.Request) (interface{}, error) {
//	    // Custom logic to extract input from request
//	    return input, nil
//	}
//
//	// Use the middleware
//	router.Use(agent.Middleware(extractInput))
func (a *Agent) Middleware(extractInput func(r *http.Request) (interface{}, error)) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract input from request
			input, err := extractInput(r)
			if err != nil {
				a.config.Logger.Printf("Error extracting input: %v", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// If we have a role provider, enrich the input with roles
			if a.config.RoleProvider != nil {
				// Extract user ID from input
				userID, err := extractUserID(input)
				if err == nil {
					roles, err := a.config.RoleProvider.GetRoles(r.Context(), userID)
					if err == nil {
						// Enrich input with roles
						input = enrichInputWithRoles(input, roles)
					} else {
						a.config.Logger.Printf("Error getting roles: %v", err)
					}
				}
			}

			// Evaluate policy
			decision, err := a.Evaluate(r.Context(), input)
			if err != nil {
				a.config.Logger.Printf("Error evaluating policy: %v", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !decision.Allowed {
				if decision.Reason != "" {
					http.Error(w, decision.Reason, http.StatusForbidden)
				} else {
					http.Error(w, "Forbidden", http.StatusForbidden)
				}
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// extractUserID is a helper function to extract user ID from the input.
//
// It attempts to find a user ID in the input using common patterns:
// - input.user.id
// - input.subject
//
// This is used by the Middleware method when a role provider is configured
// to automatically enrich the input with roles.
func extractUserID(input interface{}) (string, error) {
	// Try to extract user ID based on common input patterns
	if inputMap, ok := input.(map[string]interface{}); ok {
		if user, ok := inputMap["user"].(map[string]interface{}); ok {
			if id, ok := user["id"].(string); ok {
				return id, nil
			}
		}
		if subject, ok := inputMap["subject"].(string); ok {
			return subject, nil
		}
	}
	return "", errors.New("user ID not found in input")
}

// enrichInputWithRoles enriches the input with roles.
//
// It adds the provided roles to the input in a format that can be used by
// authorization policies. If the input already has a user object, it adds
// the roles to that object. Otherwise, it creates a new user object.
//
// This is used by the Middleware method when a role provider is configured
// to automatically enrich the input with roles.
func enrichInputWithRoles(input interface{}, roles []string) interface{} {
	if inputMap, ok := input.(map[string]interface{}); ok {
		if user, ok := inputMap["user"].(map[string]interface{}); ok {
			user["roles"] = roles
			return inputMap
		}
		// If there's no user object, create one
		inputMap["user"] = map[string]interface{}{
			"roles": roles,
		}
		return inputMap
	}
	// If input is not a map, return it unchanged
	return input
}
