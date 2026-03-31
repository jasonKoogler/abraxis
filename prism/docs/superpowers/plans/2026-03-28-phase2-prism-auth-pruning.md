# Phase 2: Prism Auth Business Logic Pruning

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all auth business logic from Prism so it is purely an API gateway with lightweight JWT validation.

**Architecture:** Prism currently contains a full auth stack (OAuth, session management, JWT issuance, user CRUD, password hashing) that belongs in Aegis. Additionally, the app wiring layer (`app.go`, `options.go`, `main.go`) references old flat import paths (`internal/adapters/*`, `internal/service`) that don't exist — Prism was refactored to feature-based organization (`internal/features/*/`) but the wiring was never updated. This plan removes auth business logic AND rewrites the wiring to use correct feature-based imports, producing a compiling gateway.

**Tech Stack:** Go 1.24, chi/v5, pgx/v5, go-redis/v9

**Spec:** `/home/jason/jdk/aegis/docs/superpowers/specs/2026-03-28-aegis-prism-separation-design.md`

**Phase Context:** Phase 2 of 4. Phase 1 (Aegis pruning) is complete. Phases 3-4 (gRPC contract, integration) follow.

**Important baseline note:** Prism does NOT compile in its current state — the app wiring references non-existent import paths. This plan fixes that as part of the wiring rewrite.

---

### Task 1: Delete auth business logic files

**Files to delete:**

Auth feature — remove everything except middleware.go and authz adapter:
- Delete: `internal/features/auth/auth_handlers.go`
- Delete: `internal/features/auth/auth_manager.go`
- Delete: `internal/features/auth/server.go`
- Delete: `internal/features/auth/helpers.go`
- Delete: `internal/features/auth/logout.go`
- Delete: `internal/features/auth/permission_service.go`
- Delete: `internal/features/auth/dto/` (entire directory)
- Delete: `internal/features/auth/gen/` (entire directory)
- Delete: `internal/features/auth/adapters/oauth/` (entire directory, including tests)
- Delete: `internal/features/auth/adapters/session/` (entire directory)
- Delete: `internal/features/auth/adapters/postgres/` (entire directory)
- Delete: `internal/features/auth/adapters/authz/example.go`

User feature — remove entirely:
- Delete: `internal/features/user/` (entire directory)

Auth domain types:
- Delete: `internal/domain/session.go`
- Delete: `internal/domain/auth_provider.go`
- Delete: `internal/domain/user_errors.go`

Auth ports:
- Delete: `internal/ports/sessionManager.go`
- Delete: `internal/ports/repo_user.go`
- Delete: `internal/ports/user_service_interface.go`
- Delete: `internal/ports/oauthProvider.go`

Password hasher:
- Delete: `internal/common/passwordhasher/` (entire directory)

- [ ] **Step 1: Delete auth feature files (keep middleware.go and authz adapter)**

```bash
cd /home/jason/jdk/prism
rm -f internal/features/auth/auth_handlers.go
rm -f internal/features/auth/auth_manager.go
rm -f internal/features/auth/server.go
rm -f internal/features/auth/helpers.go
rm -f internal/features/auth/logout.go
rm -f internal/features/auth/permission_service.go
rm -rf internal/features/auth/dto/
rm -rf internal/features/auth/gen/
rm -rf internal/features/auth/adapters/oauth/
rm -rf internal/features/auth/adapters/session/
rm -rf internal/features/auth/adapters/postgres/
rm -f internal/features/auth/adapters/authz/example.go
```

- [ ] **Step 2: Delete user feature**

```bash
cd /home/jason/jdk/prism
rm -rf internal/features/user/
```

- [ ] **Step 3: Delete auth domain types and ports**

```bash
cd /home/jason/jdk/prism
rm -f internal/domain/session.go
rm -f internal/domain/auth_provider.go
rm -f internal/domain/user_errors.go
rm -f internal/ports/sessionManager.go
rm -f internal/ports/repo_user.go
rm -f internal/ports/user_service_interface.go
rm -f internal/ports/oauthProvider.go
rm -rf internal/common/passwordhasher/
```

Do not commit yet — wait until Task 4 restores compilation.

---

### Task 2: Slim domain types

**Files:**
- Modify: `internal/domain/user.go` — replace full User model with lightweight AuthenticatedUser
- Modify: `internal/domain/jwt.go` — remove token issuance, keep validation only

- [ ] **Step 1: Replace user.go with AuthenticatedUser**

The full User model (with PasswordHash, Provider, SetPassword, etc.) is an Aegis concern. Prism only needs the data it gets from JWT claims.

Replace `internal/domain/user.go` entirely with:

```go
package domain

// AuthenticatedUser represents a user as known to the gateway from JWT claims.
// The full user model lives in Aegis — Prism only sees what's in the token.
type AuthenticatedUser struct {
	ID       string  `json:"id"`
	TenantID string  `json:"tenant_id,omitempty"`
	Roles    RoleMap `json:"roles"`
}
```

- [ ] **Step 2: Slim jwt.go — remove issuance, keep validation**

Remove from `internal/domain/jwt.go`:
- `GenerateTokenPair` method
- `RefreshToken` method
- `makeRefreshToken` method
- `ParseRefreshToken` method
- `OAuthToken` struct
- `TokenPair` struct
- `AccessToken` type alias
- `RefreshToken` type alias (the type, not the method)

Keep:
- `TokenType` type and constants (`TypeAccessToken`, `TypeRefreshToken`)
- `CustomClaims` struct and all its methods (`GetUserID`, `GetRoles`, `GetAuthProvider`, `GetSessionID`, `GetAudience`, `GetUserContextData`)
- `TokenManager` struct
- `NewTokenManager` constructor
- `ValidateToken` method
- `ExtractTokenFromHeader` function

The resulting `jwt.go` should contain only validation logic:

```go
package domain

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	TypeAccessToken  TokenType = "access_token"
	TypeRefreshToken TokenType = "refresh_token"
)

type CustomClaims struct {
	jwt.RegisteredClaims
	TokenType    TokenType `json:"token_type"`
	Roles        RoleMap   `json:"roles"`
	SessionID    string    `json:"session_id"`
	AuthProvider string    `json:"auth_provider"`
}

func (c CustomClaims) GetUserID() string {
	return c.Subject
}

func (c CustomClaims) GetRoles() RoleMap {
	return c.Roles
}

func (c CustomClaims) GetAuthProvider() string {
	return c.AuthProvider
}

func (c CustomClaims) GetSessionID() string {
	return c.SessionID
}

func (c CustomClaims) GetAudience() (jwt.ClaimStrings, error) {
	return c.Audience, nil
}

func (c CustomClaims) GetUserContextData() *UserContextData {
	return &UserContextData{
		UserID:       c.Subject,
		Roles:        c.Roles,
		AuthProvider: c.AuthProvider,
		SessionID:    c.SessionID,
	}
}

// TokenValidator handles JWT validation for the gateway.
// Token issuance is handled by Aegis.
type TokenValidator struct {
	secretKey []byte
	issuer    string
}

// NewTokenValidator creates a new token validator
func NewTokenValidator(secretKey []byte, issuer string) *TokenValidator {
	return &TokenValidator{
		secretKey: secretKey,
		issuer:    issuer,
	}
}

// ValidateToken validates a JWT token and returns the claims
func (v *TokenValidator) ValidateToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return v.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		if claims.ExpiresAt.Before(time.Now()) {
			return nil, ErrTokenExpired
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// ExtractTokenFromHeader retrieves the access token from the Authorization header.
func ExtractTokenFromHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", nil
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1], nil
	}
	return "", nil
}
```

Note: Renamed `TokenManager` → `TokenValidator` to reflect its reduced role. Removed `accessExpiration` and `refreshExpiration` fields since the gateway doesn't issue tokens.

- [ ] **Step 3: Check that domain/errors.go has ErrTokenExpired**

Read `internal/domain/errors.go` and verify `ErrTokenExpired` is defined there. If not, it may have been in another file — add it if missing.

Do not commit yet — wait until Task 3 restores compilation.

---

### Task 3: Rewrite auth middleware to use TokenValidator

**Files:**
- Modify: `internal/features/auth/middleware.go`

The current middleware depends on `AuthManager` which is deleted. Replace with direct use of `TokenValidator`.

- [ ] **Step 1: Rewrite middleware.go**

Replace `internal/features/auth/middleware.go` with:

```go
package auth

import (
	"net/http"

	"github.com/jasonKoogler/prism/internal/domain"
)

// AuthMiddleware provides middleware for authenticating HTTP requests
// using JWT token validation. Token issuance is handled by Aegis.
type AuthMiddleware struct {
	validator *domain.TokenValidator
}

// NewAuthMiddleware creates a new AuthMiddleware with a token validator
func NewAuthMiddleware(validator *domain.TokenValidator) *AuthMiddleware {
	return &AuthMiddleware{validator: validator}
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

			tenantID, tenantProvided := ExtractTenantIDFromRequest(r)
			if !tenantProvided {
				http.Error(w, "Unauthorized: missing tenant id", http.StatusUnauthorized)
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
```

Note: `ExtractTenantIDFromRequest` is referenced but may have been in `helpers.go` (now deleted). If so, it needs to be moved here or to a shared location. Check if it's defined in `middleware/extractors.go` or the auth helpers. If it was in auth helpers, add it to middleware.go:

```go
// ExtractTenantIDFromRequest extracts the tenant ID from the request URL params or header
func ExtractTenantIDFromRequest(r *http.Request) (string, bool) {
	// Check URL query parameter
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID != "" {
		return tenantID, true
	}

	// Check header
	tenantID = r.Header.Get("X-Tenant-ID")
	if tenantID != "" {
		return tenantID, true
	}

	return "", false
}
```

Do not commit yet — wait until Task 4 restores compilation.

---

### Task 4: Update middleware/combined.go

**Files:**
- Modify: `middleware/combined.go`
- Delete: `middleware/example.go` (references deleted types)

- [ ] **Step 1: Update NewCombinedMiddleware signature**

Change `NewCombinedMiddleware` to accept `*domain.TokenValidator` instead of `*auth.AuthManager`:

```go
func NewCombinedMiddleware(tokenValidator *domain.TokenValidator, authzAdapter *authz.Adapter) *CombinedMiddleware {
	return &CombinedMiddleware{
		authMiddleware: auth.NewAuthMiddleware(tokenValidator),
		authzAdapter:   authzAdapter,
	}
}
```

The import for `auth` stays the same (`internal/features/auth`) since `NewAuthMiddleware` now takes `*domain.TokenValidator`.

- [ ] **Step 2: Delete example.go**

```bash
rm -f /home/jason/jdk/prism/middleware/example.go
```

It references `auth.AuthManager` and `service.AuthManager` — both deleted.

Do not commit yet — wait until Task 5.

---

### Task 5: Rewrite app wiring with correct feature-based imports

**Files:**
- Rewrite: `internal/app/app.go`
- Rewrite: `internal/app/options.go`
- Modify: `internal/app/errors.go`
- Modify: `internal/app/server.go`
- Rewrite: `cmd/main.go`
- Modify: `internal/config/config.go`

This is the critical task. The current wiring references non-existent flat import paths AND deleted auth code. We rewrite it to use correct feature-based imports without auth business logic.

- [ ] **Step 1: Read all current wiring files**

Read these files to understand the current state:
- `internal/app/app.go` (already read — has broken imports)
- `internal/app/options.go` (already read — has broken imports)
- `internal/app/server.go`
- `internal/app/errors.go`
- `internal/app/routes.go` (if it exists)
- `internal/app/health.go` (if it exists)
- `internal/config/config.go`

- [ ] **Step 2: Rewrite `internal/app/app.go`**

Key changes:
1. Replace `service.AuthManager` with `domain.TokenValidator`
2. Remove `userRepo`, `userService`, `authService` fields
3. Add `tokenValidator *domain.TokenValidator` field
4. Fix imports to use feature-based paths
5. Remove `ConditionalAuthMiddleware` (no auth endpoints in Prism)
6. Remove the auth handler registration block from `Start()` (no `ports.HandlerFromMux`, no `adaptersHTTP.NewServer`)
7. Update `CreateAuthMiddleware` to use `tokenValidator` instead of `authService`
8. The proxy handler authentication should use the new auth middleware with `tokenValidator`

The App struct should become:
```go
type App struct {
	ctx                context.Context
	cfg                *config.Config
	logger             *log.Logger
	tokenValidator     *domain.TokenValidator
	authzService       *authz.Adapter
	rateLimiter        ports.RateLimiter
	srv                *Server
	redisClient        *redis.RedisClient
	circuitBreaker     ports.CircuitBreaker
	serviceProxy       *gateway.ServiceProxy // updated import path
	serviceDiscovery   ports.ServiceDiscoverer
	auditService       *audit.AuditService // updated import path
	auditRepo          ports.AuditLogRepository
	apiKeyService      *apikey.APIKeyService // updated import path
	apiKeyRepo         ports.ApiKeyRepository
	tenantService      *tenant.TenantDomainService // updated import path
	tenantRepo         ports.TenantRepository
	permissionService  *auth.PermissionService // if it still exists, otherwise remove
	permissionRepo     ports.PermissionRepository
	rolePermissionRepo ports.RolePermissionRepository
}
```

Note: The exact import paths depend on what packages exist in the feature directories. The implementer should check actual package names and adjust. Key pattern: services moved from `internal/service` to `internal/features/<feature>/`.

In `NewApp()`:
- Remove user repo/service validation
- Remove auth service creation — instead create `TokenValidator` from JWT config:
```go
if app.tokenValidator == nil {
	app.tokenValidator = domain.NewTokenValidator(
		[]byte(app.cfg.Auth.AuthN.JWTSecret),
		app.cfg.Auth.AuthN.JWTIssuer,
	)
}
```

In `Start()`:
- Remove the auth handler registration block (no `ports.HandlerFromMux`, no `ConditionalAuthMiddleware`)
- Keep the proxy handler block but update it to use `auth.NewAuthMiddleware(a.tokenValidator)` from the features/auth package
- Keep service discovery watcher
- Keep health/docs endpoints

- [ ] **Step 3: Rewrite `internal/app/options.go`**

Remove:
- `WithUserRepository`
- `WithUserService`
- `WithDefaultUserService`
- `WithAppAuthService`
- `WithDefaultAuthService`
- `WithServerAuthService`
- `WithDefaultServices` (the one that creates user + auth services)

Add:
- `WithTokenValidator(v *domain.TokenValidator)` option

Update `WithAllDefaultServices` to remove user/auth service creation.

Fix all imports to use feature-based paths.

- [ ] **Step 4: Update `internal/app/errors.go`**

Remove:
- `ErrNilUserRepository` / `ErrUserRepositoryRequired`
- `ErrNilUserService` / `ErrUserServiceRequired`
- `ErrNilAuthService` / `ErrAuthServiceRequired`

- [ ] **Step 5: Update `internal/app/server.go`**

Remove any references to `authService` or `service.AuthManager`. The server may have an `authService` field and `WithAuthService` option — remove them. The server should not hold auth state; the app layer handles auth middleware.

- [ ] **Step 6: Rewrite `cmd/main.go`**

Remove:
- `userRepo` creation
- User repository import

Update:
- Fix import paths to feature-based
- Create `TokenValidator` from config and pass via `app.WithTokenValidator()`

The new main.go should look approximately like:

```go
package main

import (
	"context"

	"github.com/jasonKoogler/prism/internal/app"
	"github.com/jasonKoogler/prism/internal/common/db"
	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/common/redis"
	"github.com/jasonKoogler/prism/internal/config"
	"github.com/jasonKoogler/prism/internal/domain"
	// Feature-based repo imports
	auditPG "github.com/jasonKoogler/prism/internal/features/audit/adapters/postgres"
	apikeyPG "github.com/jasonKoogler/prism/internal/features/apikey/adapters/postgres"
	// ... other feature imports as needed
)

func main() {
	ctx := context.Background()
	logger := log.NewLogger("debug")

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("Failed to load config", log.Error(err))
	}

	pgpool, err := db.NewPostgresPool(ctx, &cfg.Postgres, logger)
	if err != nil {
		logger.Fatal("Failed to create postgres pool", log.Error(err))
	}

	// Create token validator (JWT validation only — issuance is in Aegis)
	tokenValidator := domain.NewTokenValidator(
		[]byte(cfg.Auth.AuthN.JWTSecret),
		cfg.Auth.AuthN.JWTIssuer,
	)

	// Create repositories (gateway-domain only)
	auditRepo := auditPG.NewAuditLogRepository(pgpool)
	apiKeyRepo := apikeyPG.NewAPIKeyRepository(pgpool)
	// ... other repos as needed

	redisClient, err := redis.NewRedisClient(ctx, logger, &cfg.Redis)
	if err != nil {
		logger.Fatal("Failed to create Redis client", log.Error(err))
	}

	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithTokenValidator(tokenValidator),
		app.WithAuditRepository(auditRepo),
		app.WithAPIKeyRepository(apiKeyRepo),
		app.WithRedisClient(redisClient),
		app.WithAllDefaultServices(ctx),
	)
	if err != nil {
		logger.Fatal("Failed to create application", log.Error(err))
	}

	if err := application.Start(); err != nil {
		logger.Fatal("Failed to start application", log.Error(err))
	}
}
```

Note: The exact imports depend on what postgres repositories exist in each feature directory. The implementer must check which repos are actually defined.

- [ ] **Step 7: Update config.go — remove OAuth config requirements**

In `internal/config/config.go`, find the `validateConfig` function and remove validation for:
- OAuth verifier storage
- OAuth providers (at least one provider required)
- Any session manager validation that would block startup without OAuth

Keep JWT validation config (JWTSecret, JWTIssuer) since the gateway still needs those.

- [ ] **Step 8: Verify compilation**

```bash
cd /home/jason/jdk/prism && go build ./...
```

If compilation fails, fix remaining references to deleted types or broken imports.

- [ ] **Step 9: Commit**

```bash
cd /home/jason/jdk/prism
git add -A
git commit -m "refactor: remove auth business logic, fix feature-based wiring

Remove OAuth providers, session management, JWT issuance, user CRUD,
password hashing, and related domain types from Prism.

Rewrite app wiring to use correct feature-based import paths.
Prism now validates JWTs locally but delegates all auth operations
to Aegis. Gateway functionality (routing, discovery, circuit breaker,
rate limiting) is preserved."
```

---

### Task 6: go mod tidy and cleanup

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Run go mod tidy**

```bash
cd /home/jason/jdk/prism && go mod tidy
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/jason/jdk/prism && go build ./...
```

- [ ] **Step 3: Commit**

```bash
cd /home/jason/jdk/prism
git add go.mod go.sum
git commit -m "chore: go mod tidy after auth pruning"
```

---

### Task 7: Clean up stale auth documentation

**Files:**
- Delete: `docs/adapters/session.md`
- Delete: `docs/adapters/oauth.md`
- Delete: `docs/adapters/crypto.md` (if it only documents password hashing/JWE for auth)
- Delete: `internal/features/auth/adapters/oauth/tests/README.md` (if directory still exists — should be gone)

- [ ] **Step 1: Delete auth-specific documentation**

```bash
cd /home/jason/jdk/prism
rm -f docs/adapters/session.md
rm -f docs/adapters/oauth.md
rm -f docs/adapters/crypto.md
```

Read other docs files to check if they reference deleted auth components. Clean up references.

- [ ] **Step 2: Commit**

```bash
cd /home/jason/jdk/prism
git add -A
git commit -m "docs: remove auth-specific documentation"
```

---

### Task 8: Final verification

**Files:** None modified

- [ ] **Step 1: Full compilation check**

```bash
cd /home/jason/jdk/prism && go build ./...
```

- [ ] **Step 2: Run all tests**

```bash
cd /home/jason/jdk/prism && go test ./... -count=1 -short 2>&1 | tail -30
```

- [ ] **Step 3: Check for remaining references to pruned auth code**

```bash
cd /home/jason/jdk/prism && grep -rn "AuthManager\|UserService\|UserRepository\|SessionManager\|OAuthProvider\|PasswordHash\|passwordhasher\|auth_manager\|auth_handlers" --include="*.go" internal/ 2>/dev/null | grep -v "_test.go" | head -20
```

Expected: No matches.

- [ ] **Step 4: go vet**

```bash
cd /home/jason/jdk/prism && go vet ./...
```

- [ ] **Step 5: Verify file tree**

```bash
cd /home/jason/jdk/prism && find internal/features/auth/ -type f | sort
```

Expected: Only `middleware.go`, `adapters/authz/authz.go`, `adapters/authz/extractors.go`.

- [ ] **Step 6: Commit any fixes**

```bash
cd /home/jason/jdk/prism
git add -A
git commit -m "fix: clean up remaining auth references"
```
