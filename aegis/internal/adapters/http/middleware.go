package http

import (
	"net/http"

	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
	"github.com/jasonKoogler/abraxis/aegis/internal/service"
)

// AuthMiddleware provides middleware for authenticating HTTP requests.
type AuthMiddleware struct {
	authManager *service.AuthManager
}

// NewAuthMiddleware creates a new AuthMiddleware.
func NewAuthMiddleware(am *service.AuthManager) *AuthMiddleware {
	return &AuthMiddleware{am}
}

// func NewAuthMiddlewareZ(cfg *config.Config, redis *redis.RedisClient, logger *log.Logger, userRepo domain.UserRepository) *AuthMiddleware {
// 	userService := service.NewUserService(userRepo)
// 	s := service.NewAuthManager(cfg, redis, logger, userService)
// 	return &AuthMiddleware{s}
// }

// todo: add support for social providers
func (amw *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString, err := domain.ExtractTokenFromHeader(r)
		if err != nil {
			http.Error(w, "Unauthorized: no token", http.StatusUnauthorized)
			return
		}

		userContextData, err := amw.authManager.ValidateAccessToken(r.Context(), tokenString)
		if err != nil {
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		// At this point, we have user ID & roles from claims
		ctx := domain.ContextWithUserContextData(r.Context(), userContextData)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Authorize returns a middleware function that checks if the user has the required roles.
func (amw *AuthMiddleware) Authorize(requiredRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract the user's RoleMap (maps tenantID to a single UserRole) from the context.
			roleMap := domain.RolesFromContext(r.Context())
			if roleMap == nil {
				http.Error(w, "Unauthorized: missing user roles", http.StatusUnauthorized)
				return
			}

			// Require that a tenant ID is provided in the request.
			tenantID, tenantProvided := ExtractTenantIDFromRequest(r)
			if !tenantProvided {
				http.Error(w, "Unauthorized: missing tenant id", http.StatusUnauthorized)
				return
			}

			// For each required role, ensure that the user has sufficient permissions for the provided tenant.
			for _, roleStr := range requiredRoles {
				requiredRole, err := domain.RoleFromString(roleStr)
				if err != nil {
					// Misconfiguration: the required role passed is invalid.
					http.Error(w, "Server misconfigured: invalid required role", http.StatusInternalServerError)
					return
				}
				if !roleMap.HasRole(tenantID, requiredRole) {
					http.Error(w, "Unauthorized: insufficient roles in tenant", http.StatusForbidden)
					return
				}
			}

			// Proceed to the next handler if authorization is successful.
			next.ServeHTTP(w, r)
		})
	}
}
