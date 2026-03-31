package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jasonKoogler/abraxis/aegis/internal/adapters/authz"
	adapterHTTP "github.com/jasonKoogler/abraxis/aegis/internal/adapters/http"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
	"github.com/jasonKoogler/abraxis/aegis/internal/service"
)

// CombinedMiddleware provides a unified middleware that handles both authentication and authorization.
type CombinedMiddleware struct {
	authMiddleware *adapterHTTP.AuthMiddleware
	authzAdapter   *authz.Adapter
}

// NewCombinedMiddleware creates a new combined middleware.
func NewCombinedMiddleware(authManager *service.AuthManager, authzAdapter *authz.Adapter) *CombinedMiddleware {
	return &CombinedMiddleware{
		authMiddleware: adapterHTTP.NewAuthMiddleware(authManager),
		authzAdapter:   authzAdapter,
	}
}

// Protect applies both authentication and authorization middleware.
// It first authenticates the request, then authorizes it using the Authz library.
func (cm *CombinedMiddleware) Protect(extractInput func(*http.Request) (interface{}, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Create a middleware chain: first authenticate, then authorize
		return cm.authMiddleware.Authenticate(
			cm.authzAdapter.Middleware(extractInput)(next),
		)
	}
}

// ProtectWithRoles applies authentication and role-based authorization.
// This uses the built-in role-based authorization from the auth middleware.
func (cm *CombinedMiddleware) ProtectWithRoles(requiredRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Create a middleware chain: first authenticate, then check roles
		return cm.authMiddleware.Authenticate(
			cm.authMiddleware.Authorize(requiredRoles...)(next),
		)
	}
}

// ProtectWithPolicy applies authentication and policy-based authorization.
// This uses the Authz library for fine-grained policy evaluation.
func (cm *CombinedMiddleware) ProtectWithPolicy(extractInput func(*http.Request) (interface{}, error)) func(http.Handler) http.Handler {
	// Create an input extractor that enhances the input with user context data
	enhancedExtractor := func(r *http.Request) (interface{}, error) {
		// Get the base input from the provided extractor
		input, err := extractInput(r)
		if err != nil {
			return nil, err
		}

		// Cast to map to add user context
		inputMap, ok := input.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("input must be a map[string]interface{}")
		}

		// Get user context data from the request context
		userCtxData, ok := domain.UserContextDataFromContext(r.Context())
		if !ok {
			return nil, fmt.Errorf("user context data not found in request context")
		}

		// Enhance the user section with data from the authenticated user
		userMap, ok := inputMap["user"].(map[string]interface{})
		if !ok {
			userMap = make(map[string]interface{})
			inputMap["user"] = userMap
		}

		// Add user ID if not already present
		if _, exists := userMap["id"]; !exists {
			userMap["id"] = userCtxData.UserID
		}

		// Add roles if not already present
		if _, exists := userMap["roles"]; !exists {
			// Convert RoleMap to a simple slice of role strings for the policy engine
			roles := make([]string, 0)
			for _, tenantRoles := range userCtxData.Roles {
				for _, role := range tenantRoles {
					roles = append(roles, string(role))
				}
			}
			userMap["roles"] = roles
		}

		return inputMap, nil
	}

	return func(next http.Handler) http.Handler {
		// Create a middleware chain: first authenticate, then authorize with policy
		return cm.authMiddleware.Authenticate(
			cm.authzAdapter.Middleware(enhancedExtractor)(next),
		)
	}
}

// ConditionalProtect applies authentication and authorization only for paths that are not in the excludedPaths list.
// This is useful for APIs where some endpoints (like login/register) should be public.
func (cm *CombinedMiddleware) ConditionalProtect(
	extractInput func(*http.Request) (interface{}, error),
	excludedPaths []string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the path should be excluded from auth
			for _, path := range excludedPaths {
				if strings.HasSuffix(r.URL.Path, path) {
					// Skip auth for excluded paths
					next.ServeHTTP(w, r)
					return
				}
			}

			// Apply the full protection for non-excluded paths
			cm.Protect(extractInput)(next).ServeHTTP(w, r)
		})
	}
}

// UserContextExtractor creates an input extractor that pulls data from the authenticated user context
// This is useful when you want to base authorization decisions on the authenticated user's data
func (cm *CombinedMiddleware) UserContextExtractor() func(*http.Request) (interface{}, error) {
	return func(r *http.Request) (interface{}, error) {
		// Get user context data from the request context
		userCtxData, ok := domain.UserContextDataFromContext(r.Context())
		if !ok {
			return nil, fmt.Errorf("user context data not found in request context")
		}

		// Convert RoleMap to a simple slice of role strings for the policy engine
		roles := make([]string, 0)
		for _, tenantRoles := range userCtxData.Roles {
			for _, role := range tenantRoles {
				roles = append(roles, string(role))
			}
		}

		// Build the input object for policy evaluation
		input := map[string]interface{}{
			"user": map[string]interface{}{
				"id":    userCtxData.UserID,
				"roles": roles,
			},
			"resource": map[string]interface{}{
				"path": r.URL.Path,
			},
			"action": mapMethodToAction(r.Method),
		}

		return input, nil
	}
}

// mapMethodToAction maps HTTP methods to action strings for policy evaluation
func mapMethodToAction(method string) string {
	switch method {
	case http.MethodGet:
		return "read"
	case http.MethodPost:
		return "create"
	case http.MethodPut, http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	default:
		return "read"
	}
}
