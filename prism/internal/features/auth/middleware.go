package auth

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jasonKoogler/prism/internal/domain"
)

// RevocationChecker checks whether a JWT (identified by its JTI) has been
// revoked. Implementations should return (false, nil) when the token is not
// revoked, (true, nil) when it is, and (false, err) when the check itself
// fails.
type RevocationChecker interface {
	IsTokenRevoked(ctx context.Context, jti string) (bool, error)
}

// AuthMiddlewareOption is a functional option for configuring AuthMiddleware.
type AuthMiddlewareOption func(*AuthMiddleware)

// WithRevocationChecker attaches a RevocationChecker to the middleware. When
// set, every authenticated request is checked against the revocation list
// after JWT validation.
func WithRevocationChecker(checker RevocationChecker) AuthMiddlewareOption {
	return func(m *AuthMiddleware) {
		m.revocationChecker = checker
	}
}

// AuthMiddleware provides middleware for authenticating HTTP requests
// using JWT token validation. Token issuance is handled by Aegis.
type AuthMiddleware struct {
	validator         *domain.TokenValidator
	revocationChecker RevocationChecker
}

// NewAuthMiddleware creates a new AuthMiddleware with a token validator and
// optional functional options.
func NewAuthMiddleware(validator *domain.TokenValidator, opts ...AuthMiddlewareOption) *AuthMiddleware {
	m := &AuthMiddleware{validator: validator}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Authenticate validates the JWT token and sets user context data
func (amw *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString, err := domain.ExtractTokenFromHeader(r)
		if err != nil || tokenString == "" {
			http.Error(w, "Unauthorized: no token", http.StatusUnauthorized)
			return
		}

		claims, err := amw.validator.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		// Check revocation if checker is configured
		if amw.revocationChecker != nil && claims.ID != "" {
			revoked, err := amw.revocationChecker.IsTokenRevoked(r.Context(), claims.ID)
			if err == nil && revoked {
				http.Error(w, "Unauthorized: token revoked", http.StatusUnauthorized)
				return
			}
		}

		ctx := domain.ContextWithUserContextData(r.Context(), claims.GetUserContextData())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Authorize returns a middleware that checks if the user has the required roles
func (amw *AuthMiddleware) Authorize(requiredRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleMap := domain.RolesFromContext(r.Context())
			if roleMap == nil {
				http.Error(w, "Unauthorized: missing user roles", http.StatusUnauthorized)
				return
			}

			tenantIDStr, tenantProvided := ExtractTenantIDFromRequest(r)
			if !tenantProvided {
				http.Error(w, "Unauthorized: missing tenant id", http.StatusUnauthorized)
				return
			}

			tenantID, err := uuid.Parse(tenantIDStr)
			if err != nil {
				http.Error(w, "Unauthorized: invalid tenant id", http.StatusUnauthorized)
				return
			}

			for _, roleStr := range requiredRoles {
				requiredRole, err := domain.RoleFromString(roleStr)
				if err != nil {
					http.Error(w, "Server misconfigured: invalid required role", http.StatusInternalServerError)
					return
				}
				if !roleMap.HasRole(tenantID, requiredRole) {
					http.Error(w, "Unauthorized: insufficient roles in tenant", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ExtractTenantIDFromRequest extracts the tenant ID from the request
func ExtractTenantIDFromRequest(r *http.Request) (string, bool) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID != "" {
		return tenantID, true
	}
	tenantID = r.Header.Get("X-Tenant-ID")
	if tenantID != "" {
		return tenantID, true
	}
	return "", false
}
