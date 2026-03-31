# Swaggo Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace oapi-codegen (spec-first) with swaggo (code-first) in both Aegis and Prism.

**Architecture:** Remove generated ServerInterface and types. Handlers register routes directly on chi.Router. swaggo annotations on handler methods generate the OpenAPI spec via `swag init`. httpSwagger serves Swagger UI.

**Tech Stack:** `github.com/swaggo/swag` v1.16.6, `github.com/swaggo/http-swagger/v2` v2.0.2, `go-chi/chi/v5` (existing)

**Spec:** `docs/superpowers/specs/2026-03-29-swaggo-migration-design.md`

---

## Phase 1: Aegis

### Task 1: Aegis — Add swaggo dependencies and create hand-written types

**Files:**
- Modify: `/home/jason/jdk/aegis/go.mod`
- Create: `/home/jason/jdk/aegis/internal/adapters/http/types.go`

- [ ] **Step 1: Add swaggo dependencies**

```bash
cd /home/jason/jdk/aegis
go get github.com/swaggo/swag@v1.16.6
go get github.com/swaggo/http-swagger/v2@v2.0.2
```

- [ ] **Step 2: Install swag CLI**

```bash
go install github.com/swaggo/swag/cmd/swag@v1.16.6
```

- [ ] **Step 3: Create hand-written types replacing generated ones**

Create `/home/jason/jdk/aegis/internal/adapters/http/types.go` — these replace the types from `internal/ports/openapi-types.gen.go` that are actually referenced by handler code:

```go
package http

import (
	"time"
)

// PasswordLoginRequest is the request body for POST /auth/login.
type PasswordLoginRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"secretpass123"`
}

// UserRegistrationRequest is the request body for POST /auth/register.
type UserRegistrationRequest struct {
	Email     string `json:"email" example:"user@example.com"`
	FirstName string `json:"firstName" example:"John"`
	LastName  string `json:"lastName" example:"Doe"`
	Password  string `json:"password" example:"secretpass123"`
	Phone     string `json:"phone" example:"+1234567890"`
}

// UpdateUserRequest is the request body for POST /users/{userID}.
type UpdateUserRequest struct {
	AuthProvider  *string    `json:"authProvider,omitempty" example:"google"`
	AvatarURL     *string    `json:"avatarURL,omitempty" example:"https://example.com/avatar.jpg"`
	Email         *string    `json:"email,omitempty" example:"updated@example.com"`
	FirstName     *string    `json:"firstName,omitempty" example:"Jane"`
	LastLoginDate *time.Time `json:"lastLoginDate,omitempty"`
	LastName      *string    `json:"lastName,omitempty" example:"Smith"`
	Phone         *string    `json:"phone,omitempty" example:"+1987654321"`
	Status        *string    `json:"status,omitempty" example:"active" enums:"active,inactive,other"`
}

// ErrorResponse is the standard error response body.
type ErrorResponse struct {
	Slug    string `json:"slug" example:"invalid-json"`
	Message string `json:"message" example:"Invalid JSON request"`
}
```

- [ ] **Step 4: Verify compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

Expected: Compiles (types.go doesn't conflict with generated code yet — different package).

- [ ] **Step 5: Commit**

```bash
cd /home/jason/jdk/aegis
git add internal/adapters/http/types.go go.mod go.sum
git commit -m "feat: add swaggo dependencies and hand-written HTTP types"
```

---

### Task 2: Aegis — Refactor server for direct chi routing with swaggo annotations

**Files:**
- Modify: `/home/jason/jdk/aegis/internal/adapters/http/server.go`
- Modify: `/home/jason/jdk/aegis/internal/adapters/http/auth_handlers.go`
- Modify: `/home/jason/jdk/aegis/internal/adapters/http/user_handler.go`
- Modify: `/home/jason/jdk/aegis/internal/adapters/http/user_types.go`

This is the core refactor. The Server struct drops its `ports.ServerInterface` implementation. Handler methods become `api.APIFunc` (func(w, r) error) with swaggo annotations. A new `RegisterRoutes` method wires them on chi with proper auth middleware grouping.

- [ ] **Step 1: Rewrite server.go**

Replace the entire contents of `/home/jason/jdk/aegis/internal/adapters/http/server.go`:

```go
package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jasonKoogler/aegis/internal/common/api"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/jasonKoogler/aegis/internal/config"
	"github.com/jasonKoogler/aegis/internal/service"
)

type Server struct {
	authHandler *authHandler
	userHandler *userHandler

	config *config.Config
	logger *log.Logger
}

func NewServer(authManager *service.AuthManager, userService *service.UserService, config *config.Config, logger *log.Logger) *Server {
	return &Server{
		authHandler: NewAuthHandler(authManager, config, logger),
		userHandler: NewUserHandler(userService),
		config:      config,
		logger:      logger,
	}
}

// RegisterRoutes wires all HTTP endpoints on the given chi.Router.
// publicMiddleware is applied to the public auth group; protectedMiddleware to protected routes.
func (s *Server) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	// Public auth routes — no authentication required
	r.Group(func(r chi.Router) {
		r.Post("/auth/login", api.Make(s.authHandler.loginUserWithPassword, s.logger))
		r.Post("/auth/register", api.Make(s.authHandler.registerUser, s.logger))
		r.Post("/auth/refresh", api.Make(s.authHandler.refreshToken, s.logger))
		r.Get("/auth/{provider}", api.Make(s.initiateSocialLogin, s.logger))
		r.Get("/auth/{provider}/callback", api.Make(s.handleSocialLoginCallback, s.logger))
	})

	// Protected routes — authentication required
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Post("/auth/logout", api.Make(s.authHandler.logoutUser, s.logger))
		r.Get("/users", api.Make(s.getUsers, s.logger))
		r.Get("/users/{userID}", api.Make(s.getUserByID, s.logger))
		r.Post("/users/{userID}", api.Make(s.updateUserByID, s.logger))
	})
}

// --- Wrapper methods that extract URL/query params and delegate to private handlers ---

// initiateSocialLogin godoc
// @Summary Initiate social login
// @Description Redirects to the OAuth provider's login page
// @Tags auth
// @Produce json
// @Param provider path string true "OAuth provider" Enums(google, facebook, twitter)
// @Success 200 {object} object{authURL=string,state=string}
// @Failure 500 {object} ErrorResponse
// @Router /auth/{provider} [get]
func (s *Server) initiateSocialLogin(w http.ResponseWriter, r *http.Request) error {
	provider := chi.URLParam(r, "provider")
	return s.authHandler.initiateSocialLogin(w, r, provider)
}

// handleSocialLoginCallback godoc
// @Summary Handle OAuth provider callback
// @Description Processes the callback from the OAuth provider after user authentication
// @Tags auth
// @Produce json
// @Param provider path string true "OAuth provider" Enums(google, facebook, twitter)
// @Param state query string true "OAuth state parameter"
// @Param code query string true "OAuth authorization code"
// @Success 200 "Sets auth headers (Authorization, X-Refresh-Token, X-Session-ID)"
// @Failure 500 {object} ErrorResponse
// @Router /auth/{provider}/callback [get]
func (s *Server) handleSocialLoginCallback(w http.ResponseWriter, r *http.Request) error {
	provider := chi.URLParam(r, "provider")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	return s.authHandler.ProviderCallback(w, r, provider, code, state)
}

// getUsers godoc
// @Summary List all users
// @Description Get paginated list of users
// @Tags users
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(10)
// @Success 200 {array} domain.User
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /users [get]
func (s *Server) getUsers(w http.ResponseWriter, r *http.Request) error {
	page := intQueryParam(r, "page", 1)
	pageSize := intQueryParam(r, "pageSize", 10)
	return s.userHandler.getUsers(w, r, &page, &pageSize)
}

// getUserByID godoc
// @Summary Get user by ID
// @Description Get a single user by their UUID
// @Tags users
// @Produce json
// @Param userID path string true "User ID (UUID)"
// @Success 200 {object} domain.User
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /users/{userID} [get]
func (s *Server) getUserByID(w http.ResponseWriter, r *http.Request) error {
	userID := chi.URLParam(r, "userID")
	return s.userHandler.getUserByID(w, r, userID)
}

// updateUserByID godoc
// @Summary Update user by ID
// @Description Update a user's profile fields
// @Tags users
// @Accept json
// @Produce json
// @Param userID path string true "User ID (UUID)"
// @Param body body UpdateUserRequest true "Fields to update"
// @Success 200 {object} domain.User
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /users/{userID} [post]
func (s *Server) updateUserByID(w http.ResponseWriter, r *http.Request) error {
	userID := chi.URLParam(r, "userID")
	return s.userHandler.updateUserByID(w, r, userID)
}

// intQueryParam extracts an integer query parameter with a default fallback.
func intQueryParam(r *http.Request, key string, defaultVal int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(v)
	if err != nil || i < 1 {
		return defaultVal
	}
	return i
}
```

- [ ] **Step 2: Add swaggo annotations to auth_handlers.go**

Add annotations above each public handler method. The file imports change: replace `ports` with the local `http` package types.

In `/home/jason/jdk/aegis/internal/adapters/http/auth_handlers.go`:

Replace the import of `"github.com/jasonKoogler/aegis/internal/ports"` — it's no longer needed since `PasswordLoginRequest` is now in the local `http` package.

Update `loginUserWithPassword` to use the local type:
```go
// Change: var loginRequest ports.PasswordLoginRequest
// To:     var loginRequest PasswordLoginRequest
```

Add swaggo annotations above each method:

Above `loginUserWithPassword`:
```go
// loginUserWithPassword godoc
// @Summary Login with password
// @Description Authenticate a user with email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param body body PasswordLoginRequest true "Login credentials"
// @Success 200 "Sets auth headers (Authorization, X-Refresh-Token, X-Session-ID)"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/login [post]
```

Above `logoutUser`:
```go
// logoutUser godoc
// @Summary Logout user
// @Description Invalidate the current session
// @Tags auth
// @Success 302 "Redirects to /"
// @Failure 401 {object} ErrorResponse
// @Security BearerAuth
// @Router /auth/logout [post]
```

Above `refreshToken`:
```go
// refreshToken godoc
// @Summary Refresh authentication token
// @Description Exchange a refresh token for a new token pair
// @Tags auth
// @Produce json
// @Success 200 {object} domain.TokenPair
// @Failure 401 {object} ErrorResponse
// @Router /auth/refresh [post]
```

Above `registerUser`:
```go
// registerUser godoc
// @Summary Register a new user
// @Description Create a new user account with email and password
// @Tags auth
// @Accept json
// @Param body body UserRegistrationRequest true "Registration details"
// @Success 201 "Sets auth headers (Authorization, X-Refresh-Token, X-Session-ID)"
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /auth/register [post]
```

- [ ] **Step 3: Update user_types.go to use local types**

In `/home/jason/jdk/aegis/internal/adapters/http/user_types.go`, replace all references to `ports.UserRegistrationRequest` and `ports.UpdateUserRequest` with the local types:

```go
package http

import (
	"errors"
	"net/http"

	"github.com/jasonKoogler/aegis/internal/common/api"
	"github.com/jasonKoogler/aegis/internal/domain"
)

func ReqistrationRequestToUserParams(r *http.Request) (*domain.UserCreateParams, error) {
	req := &UserRegistrationRequest{}
	if err := api.BindRequest(r, req); err != nil {
		return nil, err
	}

	if req.Password == "" {
		return nil, errors.New("password is required")
	}

	return domain.NewCreateUserParams(
		req.Email,
		req.FirstName,
		req.LastName,
		req.Phone,
		&req.Password,
		domain.AuthProviderPassword,
		nil,
	)
}

func UpdateUserRequestToParams(r *http.Request) (*domain.UpdateUserParams, error) {
	req := &UpdateUserRequest{}
	if err := api.BindRequest(r, req); err != nil {
		return nil, err
	}

	params := domain.UpdateUserParams{
		Provider:  req.AuthProvider,
		AvatarURL: req.AvatarURL,
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		Status:    req.Status,
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	return &params, nil
}
```

Note: The `Status` field changes from `*UpdateUserRequestStatus` (generated enum) to `*string` (our local type). The domain's `UpdateUserParams.Status` is already `*string`, so this simplifies the conversion — no more `string(*req.Status)` cast needed.

- [ ] **Step 4: Remove ports import from auth_handlers.go**

The `ports` import is no longer needed. Remove it and change the `PasswordLoginRequest` reference:

In `auth_handlers.go`, change:
```go
import (
	...
	"github.com/jasonKoogler/aegis/internal/ports"
	...
)
```
Remove the `ports` import line.

Change `var loginRequest ports.PasswordLoginRequest` to `var loginRequest PasswordLoginRequest`.

- [ ] **Step 5: Remove ports import from user_handler.go**

In `user_handler.go`, replace:
```go
import (
	"net/http"

	"github.com/jasonKoogler/aegis/internal/common/api"
	"github.com/jasonKoogler/aegis/internal/ports"
)
```

With:
```go
import (
	"net/http"

	"github.com/jasonKoogler/aegis/internal/common/api"
)
```

And update the `userHandler` struct and constructor:
```go
type userHandler struct {
	userService UserService
}
```

Note: `ports.UserService` interface needs to either stay as a `ports` import or be defined locally. Since other things still reference `ports`, keep the `ports.UserService` import for now — it's a separate interface, not oapi-codegen generated. Check whether `ports.UserService` is in the generated file or a separate interface file.

- [ ] **Step 6: Verify compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

Expected: Will NOT compile yet because `app.go` still references `ports.HandlerFromMux`. That's the next task. But `internal/adapters/http/` package should compile on its own if we haven't deleted the generated files yet. Verify:

```bash
cd /home/jason/jdk/aegis && go build ./internal/adapters/http/
```

- [ ] **Step 7: Commit**

```bash
cd /home/jason/jdk/aegis
git add internal/adapters/http/
git commit -m "refactor: rewrite aegis handlers for direct chi routing with swaggo annotations"
```

---

### Task 3: Aegis — Update app wiring, add swagger UI, and main annotations

**Files:**
- Modify: `/home/jason/jdk/aegis/internal/app/app.go`
- Modify: `/home/jason/jdk/aegis/cmd/main.go`

- [ ] **Step 1: Add swaggo main annotation block to cmd/main.go**

Add these comments above the `main` function in `/home/jason/jdk/aegis/cmd/main.go`:

```go
// @title           Aegis Auth Service API
// @version         1.0
// @description     Authentication and user management service with JWT (Ed25519/EdDSA), OAuth, and RBAC.

// @host            localhost:8080
// @BasePath        /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
```

Add the blank import for the generated docs package at the top of `cmd/main.go`:

```go
import (
	// ... existing imports ...
	_ "github.com/jasonKoogler/aegis/docs" // swagger docs
)
```

Note: The `docs` package won't exist yet (created by `swag init`). This import will fail until Task 4.

- [ ] **Step 2: Rewrite the route registration in app.go**

In `/home/jason/jdk/aegis/internal/app/app.go`, replace the `Start()` method's route registration block.

Replace this section (lines ~121-148):
```go
	// Generate the auth handler using the generated code.
	authHandler := ports.HandlerFromMux(adaptersHTTP.NewServer(a.authService, a.userService, a.cfg, a.logger), a.srv.GetRouter())
	wrappedAuthHandler := ConditionalAuthMiddleware(a.authService, authHandler)
	publicHandlers := map[string]http.Handler{
		"/health": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}),
		"/docs": http.FileServer(http.Dir("./docs")),
		"/.well-known/jwks.json": a.keyManager.JWKSHandler(),
	}
	protectedHandlers := map[string]http.Handler{
		"/auth": wrappedAuthHandler,
	}
```

With:
```go
	// Register auth + user API routes directly on chi with auth middleware grouping.
	httpServer := adaptersHTTP.NewServer(a.authService, a.userService, a.cfg, a.logger)
	authMiddleware := adaptersHTTP.NewAuthMiddleware(a.authService).Authenticate
	httpServer.RegisterRoutes(a.srv.GetRouter(), authMiddleware)

	// Public infrastructure endpoints.
	publicHandlers := map[string]http.Handler{
		"/.well-known/jwks.json": a.keyManager.JWKSHandler(),
	}

	// No more protected handler map — auth middleware is applied via chi route groups above.
	protectedHandlers := map[string]http.Handler{}
```

- [ ] **Step 3: Add swagger UI route in app.go**

Add the httpSwagger import to `app.go`:
```go
import (
	// ... existing imports ...
	httpSwagger "github.com/swaggo/http-swagger/v2"
)
```

In the `Start()` method, after registering routes but before `a.srv.Start()`, add:
```go
	// Swagger UI
	a.srv.GetRouter().Get("/swagger/*", httpSwagger.WrapHandler)
```

- [ ] **Step 4: Remove the ConditionalAuthMiddleware function**

Delete the `ConditionalAuthMiddleware` function from `app.go` (lines ~188-215). It's replaced by chi route groups in `RegisterRoutes`.

- [ ] **Step 5: Clean up imports in app.go**

Remove the `"github.com/jasonKoogler/aegis/internal/ports"` import from `app.go` — it's no longer needed.

Remove `"strings"` import if it was only used by `ConditionalAuthMiddleware`.

- [ ] **Step 6: Commit**

```bash
cd /home/jason/jdk/aegis
git add internal/app/app.go cmd/main.go
git commit -m "refactor: wire aegis routes via RegisterRoutes, add swagger UI"
```

---

### Task 4: Aegis — Delete oapi-codegen artifacts, run swag init, verify

**Files:**
- Delete: `/home/jason/jdk/aegis/internal/ports/openapi-server.gen.go`
- Delete: `/home/jason/jdk/aegis/internal/ports/openapi-types.gen.go`
- Delete: `/home/jason/jdk/aegis/api/openapi/` (entire directory)
- Delete: `/home/jason/jdk/aegis/redocly.yaml`
- Delete: `/home/jason/jdk/aegis/internal/common/client/` (if exists and unused)
- Modify: `/home/jason/jdk/aegis/Makefile`
- Modify: `/home/jason/jdk/aegis/go.mod`

- [ ] **Step 1: Check what else imports the generated ports types**

```bash
cd /home/jason/jdk/aegis && grep -r "ports\." --include="*.go" internal/ cmd/ | grep -v "_test.go" | grep -v "openapi-" | grep -v "\.gen\.go"
```

Any file still importing generated types from `ports` (like `ports.ServerInterface`, `ports.HandlerFromMux`, `ports.PasswordLoginRequest`) needs to be updated. The `ports` package likely also contains non-generated interfaces like `UserService`, `UserRepository`, etc. — those stay.

- [ ] **Step 2: Delete generated OpenAPI files**

```bash
cd /home/jason/jdk/aegis
rm internal/ports/openapi-server.gen.go
rm internal/ports/openapi-types.gen.go
```

- [ ] **Step 3: Delete hand-written OpenAPI spec directory**

```bash
cd /home/jason/jdk/aegis
rm -rf api/openapi/
rm -f redocly.yaml
```

- [ ] **Step 4: Check and delete generated client if unused**

```bash
cd /home/jason/jdk/aegis
# Check if anything imports the client package
grep -r "common/client" --include="*.go" internal/ cmd/
```

If nothing imports it:
```bash
rm -rf internal/common/client/
```

- [ ] **Step 5: Remove oapi-codegen runtime from go.mod**

```bash
cd /home/jason/jdk/aegis
go mod tidy
```

This will remove `github.com/oapi-codegen/runtime` and any other unused dependencies.

- [ ] **Step 6: Verify compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

Fix any remaining import or type reference errors. Common issues:
- `ports.UserService` interface — should be in a non-generated file in `ports/`, verify it still exists
- Any lingering references to generated types

- [ ] **Step 7: Update Makefile**

Add swagger target and remove old openapi references. Add to `/home/jason/jdk/aegis/Makefile`:

```makefile
## Swagger
swagger: ## Generate swagger docs
	swag init -g cmd/main.go --parseDependency --parseInternal
```

- [ ] **Step 8: Run swag init**

```bash
cd /home/jason/jdk/aegis
swag init -g cmd/main.go --parseDependency --parseInternal
```

Expected: Creates `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`.

- [ ] **Step 9: Verify full compilation with generated docs**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

Expected: Clean compilation including the `docs` package import in `cmd/main.go`.

- [ ] **Step 10: Commit**

```bash
cd /home/jason/jdk/aegis
git add -A
git commit -m "feat: complete aegis swaggo migration — remove oapi-codegen, generate swagger docs"
```

---

## Phase 2: Prism

### Task 5: Prism — Add swaggo dependencies and create hand-written types

**Files:**
- Modify: `/home/jason/jdk/prism/go.mod`
- Create: `/home/jason/jdk/prism/internal/features/audit/dto/types.go`
- Create: `/home/jason/jdk/prism/internal/features/apikey/dto/types.go`

- [ ] **Step 1: Add swaggo dependencies**

```bash
cd /home/jason/jdk/prism
go get github.com/swaggo/swag@v1.16.6
go get github.com/swaggo/http-swagger/v2@v2.0.2
```

- [ ] **Step 2: Create audit DTO types**

Create `/home/jason/jdk/prism/internal/features/audit/dto/types.go` — hand-written replacements for the generated audit param types:

```go
package dto

import "time"

// ListAuditLogsParams holds query parameters for listing audit logs.
type ListAuditLogsParams struct {
	TenantID     *string `json:"tenant_id,omitempty"`
	UserID       *string `json:"user_id,omitempty"`
	EventType    *string `json:"event_type,omitempty"`
	ResourceType *string `json:"resource_type,omitempty"`
	ResourceID   *string `json:"resource_id,omitempty"`
	Page         *int    `json:"page,omitempty"`
	PageSize     *int    `json:"pageSize,omitempty"`
	StartDate    *string `json:"start_date,omitempty"`
	EndDate      *string `json:"end_date,omitempty"`
}

// AggregateAuditLogsParams holds query parameters for audit log aggregation.
type AggregateAuditLogsParams struct {
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
}

// ExportAuditLogsParams holds query parameters for exporting audit logs.
type ExportAuditLogsParams struct {
	TenantID     *string    `json:"tenant_id,omitempty"`
	UserID       *string    `json:"user_id,omitempty"`
	EventType    *string    `json:"event_type,omitempty"`
	ResourceType *string    `json:"resource_type,omitempty"`
	ResourceID   *string    `json:"resource_id,omitempty"`
	StartDate    *time.Time `json:"start_date,omitempty"`
	EndDate      *time.Time `json:"end_date,omitempty"`
	Page         *int       `json:"page,omitempty"`
	PageSize     *int       `json:"pageSize,omitempty"`
}
```

- [ ] **Step 3: Create apikey DTO types**

Create `/home/jason/jdk/prism/internal/features/apikey/dto/types.go` — hand-written replacements for generated apikey types:

```go
package dto

import "github.com/google/uuid"

// ApiKeyCreateRequest is the request body for POST /apikey.
type ApiKeyCreateRequest struct {
	ExpiresInDays *int      `json:"expires_in_days,omitempty" example:"30"`
	Name          string    `json:"name" example:"my-api-key"`
	Scopes        []string  `json:"scopes" example:"read:users,write:users"`
	TenantID      *uuid.UUID `json:"tenant_id,omitempty"`
	UserID        *uuid.UUID `json:"user_id,omitempty"`
}

// ApiKeyValidateRequest is the request body for POST /apikey/validate.
type ApiKeyValidateRequest struct {
	ApiKey string `json:"api_key" example:"ak_abc123..."`
}

// UpdateApiKeyMetadataRequest is the request body for PUT /apikey/{apikeyID}/metadata.
type UpdateApiKeyMetadataRequest struct {
	ExpiresInDays *int      `json:"expires_in_days,omitempty" example:"90"`
	IsActive      *bool     `json:"is_active,omitempty" example:"true"`
	Name          *string   `json:"name,omitempty" example:"updated-key-name"`
	Scopes        *[]string `json:"scopes,omitempty"`
}

// ListApiKeysParams holds query parameters for listing API keys.
type ListApiKeysParams struct {
	UserID   *string `json:"user_id,omitempty"`
	TenantID *string `json:"tenant_id,omitempty"`
	Page     *int    `json:"page,omitempty"`
	PageSize *int    `json:"pageSize,omitempty"`
}

// ErrorResponse is the standard error response body.
type ErrorResponse struct {
	Slug    string `json:"slug" example:"internal_error"`
	Message string `json:"message" example:"An unexpected error occurred"`
}
```

- [ ] **Step 4: Verify compilation**

```bash
cd /home/jason/jdk/prism && go build ./...
```

Expected: Compiles (new types don't conflict with gen types yet).

- [ ] **Step 5: Commit**

```bash
cd /home/jason/jdk/prism
git add internal/features/audit/dto/types.go internal/features/apikey/dto/types.go go.mod go.sum
git commit -m "feat: add swaggo dependencies and hand-written DTO types for prism"
```

---

### Task 6: Prism — Refactor audit feature for direct routing with swaggo annotations

**Files:**
- Modify: `/home/jason/jdk/prism/internal/features/audit/server.go`
- Modify: `/home/jason/jdk/prism/internal/features/audit/handler.go`
- Modify: `/home/jason/jdk/prism/internal/features/audit/dto/converter.go`

- [ ] **Step 1: Rewrite audit dto/converter.go to use local types instead of gen types**

Replace `/home/jason/jdk/prism/internal/features/audit/dto/converter.go`:

```go
package dto

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jasonKoogler/prism/internal/domain"
)

// ListAuditLogsParamsFromRequest extracts list params from the HTTP request query string.
func ListAuditLogsParamsFromRequest(r *http.Request) (*domain.ListAuditLogsReq, error) {
	q := r.URL.Query()

	req := &domain.ListAuditLogsReq{
		TenantID:     optionalString(q.Get("tenant_id")),
		UserID:       optionalString(q.Get("user_id")),
		EventType:    optionalString(q.Get("event_type")),
		ResourceType: optionalString(q.Get("resource_type")),
		ResourceID:   optionalString(q.Get("resource_id")),
		Page:         optionalInt(q.Get("page")),
		PageSize:     optionalInt(q.Get("pageSize")),
		StartDate:    optionalString(q.Get("start_date")),
		EndDate:      optionalString(q.Get("end_date")),
	}

	return req, nil
}

// AggregateParamsFromRequest extracts aggregation date range from the HTTP request.
func AggregateParamsFromRequest(r *http.Request) (startDate, endDate *time.Time) {
	q := r.URL.Query()
	if v := q.Get("start_date"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			startDate = &t
		}
	}
	if v := q.Get("end_date"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			endDate = &t
		}
	}
	return
}

// ExportAuditLogsParamsFromRequest extracts export params from the HTTP request.
func ExportAuditLogsParamsFromRequest(r *http.Request) (*domain.ExportAuditLogsReq, error) {
	q := r.URL.Query()
	return &domain.ExportAuditLogsReq{
		TenantID: optionalString(q.Get("tenant_id")),
	}, nil
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optionalInt(s string) *int {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}
```

- [ ] **Step 2: Rewrite audit handler.go to remove gen type dependencies**

Replace `/home/jason/jdk/prism/internal/features/audit/handler.go`:

```go
package audit

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jasonKoogler/prism/internal/common/api"
	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
	"github.com/jasonKoogler/prism/internal/domain"
	"github.com/jasonKoogler/prism/internal/domain/prefixid"
	"github.com/jasonKoogler/prism/internal/features/audit/dto"
	"github.com/jasonKoogler/prism/internal/ports"
)

type auditHandler struct {
	auditService ports.AuditService
	config       *config.Config
	logger       *log.Logger
}

func NewAuditHandler(auditService ports.AuditService, config *config.Config, logger *log.Logger) *auditHandler {
	return &auditHandler{
		auditService: auditService,
		config:       config,
		logger:       logger,
	}
}

func (h *auditHandler) listAuditLogs(w http.ResponseWriter, r *http.Request) error {
	domainReq, err := dto.ListAuditLogsParamsFromRequest(r)
	if err != nil {
		return api.InvalidJSONError()
	}

	logs, err := h.auditService.ListAuditLogs(r.Context(), domainReq)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidRequest) {
			return api.NewError(
				"invalid-request",
				"Invalid request parameters - at least one filter must be specified",
				http.StatusBadRequest,
				api.ErrorTypeBadRequest,
				err,
			)
		}
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, logs)
}

func (h *auditHandler) getAuditLog(w http.ResponseWriter, r *http.Request) error {
	auditID := chi.URLParam(r, "auditID")
	auditLog, err := h.auditService.GetAuditLog(r.Context(), auditID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return api.NotFound(err)
		}
		if errors.Is(err, prefixid.ErrInvalidID) {
			return api.InvalidQueryParamError("audit_id")
		}
		return api.InternalError(err)
	}
	return api.Respond(w, http.StatusOK, auditLog)
}

func (h *auditHandler) aggregateAuditLogs(w http.ResponseWriter, r *http.Request) error {
	groupBy := chi.URLParam(r, "groupBy")
	startDate, endDate := dto.AggregateParamsFromRequest(r)

	var start, end time.Time
	if startDate != nil {
		start = *startDate
	}
	if endDate != nil {
		end = *endDate
	}

	aggregation, err := h.auditService.AggregateAuditLogs(r.Context(), groupBy, start, end)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, aggregation)
}

func (h *auditHandler) exportAuditLogs(w http.ResponseWriter, r *http.Request) error {
	domainReq, err := dto.ExportAuditLogsParamsFromRequest(r)
	if err != nil {
		return api.InvalidJSONError()
	}

	csvData, err := h.auditService.ExportAuditLogs(r.Context(), domainReq)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidRequest) {
			return api.NewError(
				"invalid-request",
				"Invalid request parameters",
				http.StatusBadRequest,
				api.ErrorTypeBadRequest,
				err,
			)
		}
		return api.InternalError(err)
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=audit_logs.csv")
	_, err = w.Write(csvData)
	if err != nil {
		return api.InternalError(err)
	}

	return nil
}
```

Note: add `"time"` to handler.go imports since `aggregateAuditLogs` uses `time.Time`.

- [ ] **Step 3: Rewrite audit server.go with swaggo annotations and RegisterRoutes**

Replace `/home/jason/jdk/prism/internal/features/audit/server.go`:

```go
package audit

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jasonKoogler/prism/internal/common/api"
	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
)

type Server struct {
	auditHandler *auditHandler
	config       *config.Config
	logger       *log.Logger
}

func NewServer(auditHandler *auditHandler, config *config.Config, logger *log.Logger) *Server {
	return &Server{
		auditHandler: auditHandler,
		config:       config,
		logger:       logger,
	}
}

// RegisterRoutes registers all audit endpoints on the given chi.Router.
func (s *Server) RegisterRoutes(r chi.Router) {
	r.Route("/audit", func(r chi.Router) {
		r.Get("/", api.Make(s.auditHandler.listAuditLogs, s.logger))
		r.Get("/aggregate/{groupBy}", api.Make(s.auditHandler.aggregateAuditLogs, s.logger))
		r.Get("/export", api.Make(s.auditHandler.exportAuditLogs, s.logger))
		r.Get("/{auditID}", api.Make(s.auditHandler.getAuditLog, s.logger))
	})
}
```

Note: swaggo annotations go on the handler methods (in handler.go) since those are the functions that contain the logic. Add these annotations to the handler methods in handler.go:

Above `listAuditLogs`:
```go
// listAuditLogs godoc
// @Summary List audit logs
// @Description Get paginated, filtered list of audit logs
// @Tags audit
// @Produce json
// @Param tenant_id query string false "Filter by tenant ID"
// @Param user_id query string false "Filter by user ID"
// @Param event_type query string false "Filter by event type"
// @Param resource_type query string false "Filter by resource type"
// @Param resource_id query string false "Filter by resource ID"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Param start_date query string false "Start date (RFC3339)"
// @Param end_date query string false "End date (RFC3339)"
// @Success 200 {object} domain.AuditLogListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /audit [get]
```

Above `getAuditLog`:
```go
// getAuditLog godoc
// @Summary Get audit log by ID
// @Description Retrieve a single audit log entry
// @Tags audit
// @Produce json
// @Param auditID path string true "Audit log ID"
// @Success 200 {object} domain.AuditLog
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /audit/{auditID} [get]
```

Above `aggregateAuditLogs`:
```go
// aggregateAuditLogs godoc
// @Summary Aggregate audit logs
// @Description Group and count audit logs by a field
// @Tags audit
// @Produce json
// @Param groupBy path string true "Group by field" Enums(event_type, actor_type, tenant)
// @Param start_date query string false "Start date (RFC3339)"
// @Param end_date query string false "End date (RFC3339)"
// @Success 200 {object} domain.AuditLogAggregateResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /audit/aggregate/{groupBy} [get]
```

Above `exportAuditLogs`:
```go
// exportAuditLogs godoc
// @Summary Export audit logs
// @Description Export audit logs as CSV
// @Tags audit
// @Produce text/csv
// @Param tenant_id query string false "Filter by tenant ID"
// @Success 200 {file} file "CSV file download"
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /audit/export [get]
```

- [ ] **Step 4: Verify audit package compiles**

```bash
cd /home/jason/jdk/prism && go build ./internal/features/audit/...
```

Expected: May fail because `gen` package is still imported elsewhere. Check and fix.

- [ ] **Step 5: Commit**

```bash
cd /home/jason/jdk/prism
git add internal/features/audit/
git commit -m "refactor: rewrite prism audit feature for direct chi routing with swaggo"
```

---

### Task 7: Prism — Refactor apikey feature for direct routing with swaggo annotations

**Files:**
- Modify: `/home/jason/jdk/prism/internal/features/apikey/server.go`
- Modify: `/home/jason/jdk/prism/internal/features/apikey/handler.go`
- Modify: `/home/jason/jdk/prism/internal/features/apikey/dto/converter.go`

- [ ] **Step 1: Rewrite apikey dto/converter.go to use local types**

Replace `/home/jason/jdk/prism/internal/features/apikey/dto/converter.go`:

```go
package dto

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jasonKoogler/prism/internal/common/api"
	"github.com/jasonKoogler/prism/internal/domain"
	"github.com/jasonKoogler/prism/internal/domain/prefixid"
)

func CreateApiKeyRequestToParams(r *http.Request) (*domain.APIKey_CreateParams, error) {
	req := &ApiKeyCreateRequest{}
	if err := api.BindRequest(r, req); err != nil {
		return nil, err
	}

	var tenantUUID, userUUID uuid.UUID
	if req.TenantID != nil {
		tenantUUID = *req.TenantID
	}
	if req.UserID != nil {
		userUUID = *req.UserID
	}

	params := &domain.APIKey_CreateParams{
		Name:          req.Name,
		TenantID:      tenantUUID,
		UserID:        userUUID,
		Scopes:        req.Scopes,
		ExpiresInDays: *req.ExpiresInDays,
		IPAddress:     r.RemoteAddr,
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	return params, nil
}

func ValidateApiKeyRequestToParams(r *http.Request) (*domain.APIKey_ValidateParams, error) {
	req := &ApiKeyValidateRequest{}
	if err := api.BindRequest(r, req); err != nil {
		return nil, err
	}

	params := &domain.APIKey_ValidateParams{
		RawAPIKey: req.ApiKey,
		IPAddress: r.RemoteAddr,
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	return params, nil
}

func UpdateApiKeyMetadataRequestToParams(r *http.Request, apikeyID string) (*domain.APIKey_UpdateMetadataParams, error) {
	req := &UpdateApiKeyMetadataRequest{}
	if err := api.BindRequest(r, req); err != nil {
		return nil, err
	}

	idObj, err := prefixid.ParseApiKeyID(apikeyID)
	if err != nil {
		return nil, err
	}

	rawID, err := uuid.Parse(idObj.Raw().String())
	if err != nil {
		return nil, err
	}

	params := &domain.APIKey_UpdateMetadataParams{
		ID:            rawID,
		Name:          req.Name,
		Scopes:        req.Scopes,
		IsActive:      req.IsActive,
		ExpiresInDays: req.ExpiresInDays,
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	return params, nil
}

func ListApiKeysParamsFromRequest(r *http.Request) (*domain.APIKey_ListParams, error) {
	q := r.URL.Query()

	var tenantUUID, userUUID *uuid.UUID

	if v := q.Get("tenant_id"); v != "" {
		tenantID, err := prefixid.ParseTenantID(v)
		if err != nil {
			return nil, err
		}
		parsed, err := uuid.Parse(tenantID.Raw().String())
		if err != nil {
			return nil, err
		}
		tenantUUID = &parsed
	}

	if v := q.Get("user_id"); v != "" {
		userID, err := prefixid.ParseUserID(v)
		if err != nil {
			return nil, err
		}
		parsed, err := uuid.Parse(userID.Raw().String())
		if err != nil {
			return nil, err
		}
		userUUID = &parsed
	}

	page := 1
	pageSize := 20

	if v := q.Get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			page = p
		}
	}
	if v := q.Get("pageSize"); v != "" {
		if ps, err := strconv.Atoi(v); err == nil {
			pageSize = ps
		}
	}

	return &domain.APIKey_ListParams{
		TenantID: tenantUUID,
		UserID:   userUUID,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
```

- [ ] **Step 2: Rewrite apikey handler.go to remove gen type dependencies**

Replace `/home/jason/jdk/prism/internal/features/apikey/handler.go`:

```go
package apikey

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jasonKoogler/prism/internal/common/api"
	"github.com/jasonKoogler/prism/internal/features/apikey/dto"
	"github.com/jasonKoogler/prism/internal/ports"
)

type apiKeyHandler struct {
	apiKeyService ports.ApiKeyService
}

func NewApiKeyHandler(apiKeyService ports.ApiKeyService) *apiKeyHandler {
	return &apiKeyHandler{
		apiKeyService: apiKeyService,
	}
}

// listApiKeys godoc
// @Summary List API keys
// @Description Get paginated list of API keys, optionally filtered by tenant or user
// @Tags apikey
// @Produce json
// @Param tenant_id query string false "Filter by tenant ID"
// @Param user_id query string false "Filter by user ID"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Success 200 {array} domain.APIKey
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /apikey [get]
func (h *apiKeyHandler) listApiKeys(w http.ResponseWriter, r *http.Request) error {
	domainParams, err := dto.ListApiKeysParamsFromRequest(r)
	if err != nil {
		return api.InvalidJSONError()
	}

	keys, err := h.apiKeyService.List(r.Context(), domainParams)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, keys)
}

// createApiKey godoc
// @Summary Create a new API key
// @Description Generate a new API key with specified scopes and expiration
// @Tags apikey
// @Accept json
// @Produce json
// @Param body body dto.ApiKeyCreateRequest true "API key creation parameters"
// @Success 201 {object} domain.APIKey
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /apikey [post]
func (h *apiKeyHandler) createApiKey(w http.ResponseWriter, r *http.Request) error {
	params, err := dto.CreateApiKeyRequestToParams(r)
	if err != nil {
		return api.InvalidJSONError()
	}

	createdKey, err := h.apiKeyService.Create(r.Context(), params)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusCreated, createdKey)
}

// validateApiKey godoc
// @Summary Validate an API key
// @Description Check if a raw API key is valid
// @Tags apikey
// @Accept json
// @Produce json
// @Param body body dto.ApiKeyValidateRequest true "API key to validate"
// @Success 200 {object} domain.APIKey
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /apikey/validate [post]
func (h *apiKeyHandler) validateApiKey(w http.ResponseWriter, r *http.Request) error {
	params, err := dto.ValidateApiKeyRequestToParams(r)
	if err != nil {
		return api.InvalidJSONError()
	}

	validatedKey, err := h.apiKeyService.Validate(r.Context(), params.RawAPIKey, params.IPAddress)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, validatedKey)
}

// revokeApiKey godoc
// @Summary Revoke an API key
// @Description Disable an API key by its ID
// @Tags apikey
// @Param apikeyID path string true "API key ID"
// @Success 204 "No Content"
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /apikey/{apikeyID} [delete]
func (h *apiKeyHandler) revokeApiKey(w http.ResponseWriter, r *http.Request) error {
	apikeyID := chi.URLParam(r, "apikeyID")
	if apikeyID == "" {
		return api.MissingIDError()
	}

	err := h.apiKeyService.Revoke(r.Context(), apikeyID)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusNoContent, nil)
}

// getApiKey godoc
// @Summary Get API key by ID
// @Description Retrieve a single API key's metadata
// @Tags apikey
// @Produce json
// @Param apikeyID path string true "API key ID"
// @Success 200 {object} domain.APIKey
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /apikey/{apikeyID} [get]
func (h *apiKeyHandler) getApiKey(w http.ResponseWriter, r *http.Request) error {
	apikeyID := chi.URLParam(r, "apikeyID")
	if apikeyID == "" {
		return api.MissingIDError()
	}

	apiKey, err := h.apiKeyService.Get(r.Context(), apikeyID)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, apiKey)
}

// updateApiKeyMetadata godoc
// @Summary Update API key metadata
// @Description Update name, scopes, active status, or expiration of an API key
// @Tags apikey
// @Accept json
// @Produce json
// @Param apikeyID path string true "API key ID"
// @Param body body dto.UpdateApiKeyMetadataRequest true "Fields to update"
// @Success 200 {object} domain.APIKey
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /apikey/{apikeyID}/metadata [put]
func (h *apiKeyHandler) updateApiKeyMetadata(w http.ResponseWriter, r *http.Request) error {
	apikeyID := chi.URLParam(r, "apikeyID")
	if apikeyID == "" {
		return api.MissingIDError()
	}

	params, err := dto.UpdateApiKeyMetadataRequestToParams(r, apikeyID)
	if err != nil {
		return api.InvalidJSONError()
	}

	updatedKey, err := h.apiKeyService.UpdateMetadata(r.Context(), params)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, updatedKey)
}
```

- [ ] **Step 3: Rewrite apikey server.go with RegisterRoutes**

Replace `/home/jason/jdk/prism/internal/features/apikey/server.go`:

```go
package apikey

import (
	"github.com/go-chi/chi/v5"
	"github.com/jasonKoogler/prism/internal/common/api"
	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
	"github.com/jasonKoogler/prism/internal/ports"
)

type Server struct {
	apiKeyHandler *apiKeyHandler
	config        *config.Config
	logger        *log.Logger
}

func NewServer(apiKeyService ports.ApiKeyService, config *config.Config, logger *log.Logger) *Server {
	return &Server{
		apiKeyHandler: NewApiKeyHandler(apiKeyService),
		config:        config,
		logger:        logger,
	}
}

// RegisterRoutes registers all API key endpoints on the given chi.Router.
func (s *Server) RegisterRoutes(r chi.Router) {
	r.Route("/apikey", func(r chi.Router) {
		r.Get("/", api.Make(s.apiKeyHandler.listApiKeys, s.logger))
		r.Post("/", api.Make(s.apiKeyHandler.createApiKey, s.logger))
		r.Post("/validate", api.Make(s.apiKeyHandler.validateApiKey, s.logger))
		r.Get("/{apikeyID}", api.Make(s.apiKeyHandler.getApiKey, s.logger))
		r.Delete("/{apikeyID}", api.Make(s.apiKeyHandler.revokeApiKey, s.logger))
		r.Put("/{apikeyID}/metadata", api.Make(s.apiKeyHandler.updateApiKeyMetadata, s.logger))
	})
}
```

- [ ] **Step 4: Verify apikey package compiles**

```bash
cd /home/jason/jdk/prism && go build ./internal/features/apikey/...
```

- [ ] **Step 5: Commit**

```bash
cd /home/jason/jdk/prism
git add internal/features/apikey/
git commit -m "refactor: rewrite prism apikey feature for direct chi routing with swaggo"
```

---

### Task 8: Prism — Update app wiring, add swagger UI, and main annotations

**Files:**
- Modify: `/home/jason/jdk/prism/internal/app/routes.go`
- Modify: `/home/jason/jdk/prism/internal/app/app.go`
- Modify: `/home/jason/jdk/prism/cmd/main.go`

- [ ] **Step 1: Rewrite routes.go to use RegisterRoutes**

Replace `/home/jason/jdk/prism/internal/app/routes.go`:

```go
package app

import (
	"github.com/go-chi/chi/v5"
	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
	"github.com/jasonKoogler/prism/internal/features/apikey"
	"github.com/jasonKoogler/prism/internal/features/audit"
	"github.com/jasonKoogler/prism/internal/ports"
)

// RegisterAuditRoutes registers the audit routes with the provided router.
func RegisterAuditRoutes(r chi.Router, auditService ports.AuditService, cfg *config.Config, logger *log.Logger) {
	auditHandler := audit.NewAuditHandler(auditService, cfg, logger)
	auditServer := audit.NewServer(auditHandler, cfg, logger)
	auditServer.RegisterRoutes(r)
	logger.Info("Registered audit routes")
}

// RegisterApiKeyRoutes registers the API key routes with the provided router.
func RegisterApiKeyRoutes(r chi.Router, apiKeyService ports.ApiKeyService, cfg *config.Config, logger *log.Logger) {
	apiKeyServer := apikey.NewServer(apiKeyService, cfg, logger)
	apiKeyServer.RegisterRoutes(r)
	logger.Info("Registered API key routes")
}
```

- [ ] **Step 2: Wire audit and apikey routes in app.go Start()**

In `/home/jason/jdk/prism/internal/app/app.go`, in the `Start()` method, add route registration before the `a.srv.Start()` call. After the service proxy registration block and before `publicHandlers`:

```go
	// Register feature API routes
	if a.auditService != nil {
		RegisterAuditRoutes(a.srv.GetRouter(), a.auditService, a.cfg, a.logger)
	}
	if a.apiKeyService != nil {
		RegisterApiKeyRoutes(a.srv.GetRouter(), a.apiKeyService, a.cfg, a.logger)
	}
```

Note: Check that `a.auditService` and `a.apiKeyService` fields exist on the App struct. If they don't exist as typed service interfaces, look for the repository fields and service initialization in the options. The services may need to be created in `NewApp` or passed via options.

- [ ] **Step 3: Replace static doc serving with swagger UI in app.go**

In the `publicHandlers` map in `Start()`, replace:
```go
"/docs": http.FileServer(http.Dir("./docs")),
```

With nothing (remove the `/docs` entry). Instead, add swagger UI route:
```go
	// Swagger UI
	a.srv.GetRouter().Get("/swagger/*", httpSwagger.WrapHandler)
```

Add the import:
```go
import httpSwagger "github.com/swaggo/http-swagger/v2"
```

- [ ] **Step 4: Add swaggo main annotation block to Prism cmd/main.go**

Add above the `main` function:
```go
// @title           Prism API Gateway
// @version         1.0
// @description     API gateway with service routing, audit logging, API key management, and service discovery.

// @host            localhost:8080
// @BasePath        /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
```

Add blank import:
```go
import (
	// ... existing imports ...
	_ "github.com/jasonKoogler/prism/docs" // swagger docs
)
```

- [ ] **Step 5: Commit**

```bash
cd /home/jason/jdk/prism
git add internal/app/ cmd/main.go
git commit -m "refactor: wire prism routes via RegisterRoutes, add swagger UI"
```

---

### Task 9: Prism — Delete oapi-codegen artifacts, run swag init, verify

**Files:**
- Delete: `/home/jason/jdk/prism/internal/features/audit/gen/` (entire directory)
- Delete: `/home/jason/jdk/prism/internal/features/apikey/gen/` (entire directory)
- Delete: `/home/jason/jdk/prism/internal/features/gateway/gen/` (entire directory, if exists)
- Delete: `/home/jason/jdk/prism/internal/features/discovery/gen/` (entire directory, if exists)
- Delete: `/home/jason/jdk/prism/internal/api/shared/models.gen.go`
- Delete: `/home/jason/jdk/prism/internal/api/client/generated.go`
- Delete: `/home/jason/jdk/prism/api/openapi/` (entire directory)
- Delete: `/home/jason/jdk/prism/scripts/openapi-http.sh`
- Modify: `/home/jason/jdk/prism/Makefile`
- Modify: `/home/jason/jdk/prism/go.mod`

- [ ] **Step 1: Verify no remaining imports of gen packages**

```bash
cd /home/jason/jdk/prism
grep -r "features/audit/gen\|features/apikey/gen\|features/gateway/gen\|features/discovery/gen\|api/shared\|api/client" --include="*.go" internal/ cmd/ | grep -v "_test.go" | grep -v "\.gen\.go"
```

Fix any remaining imports before deleting.

- [ ] **Step 2: Delete all generated code directories**

```bash
cd /home/jason/jdk/prism
rm -rf internal/features/audit/gen/
rm -rf internal/features/apikey/gen/
rm -rf internal/features/gateway/gen/
rm -rf internal/features/discovery/gen/
```

- [ ] **Step 3: Check and delete shared models and client if unused**

```bash
cd /home/jason/jdk/prism
grep -r "api/shared\|api/client" --include="*.go" internal/ cmd/
```

If nothing imports them:
```bash
rm -f internal/api/shared/models.gen.go
rm -f internal/api/client/generated.go
# Remove directories if empty
rmdir internal/api/shared/ 2>/dev/null
rmdir internal/api/client/ 2>/dev/null
rmdir internal/api/ 2>/dev/null
```

- [ ] **Step 4: Delete hand-written OpenAPI specs and generation scripts**

```bash
cd /home/jason/jdk/prism
rm -rf api/openapi/
rm -f scripts/openapi-http.sh
```

- [ ] **Step 5: Clean up go.mod**

```bash
cd /home/jason/jdk/prism
go mod tidy
```

This removes `github.com/oapi-codegen/runtime`, `github.com/getkin/kin-openapi`, and any other unused dependencies.

- [ ] **Step 6: Verify compilation**

```bash
cd /home/jason/jdk/prism && go build ./...
```

Fix any remaining issues.

- [ ] **Step 7: Update Makefile**

Replace the openapi-related targets in `/home/jason/jdk/prism/Makefile` with swagger targets:

Remove: `openapi`, `ensure-dependencies`, `chmod`, `bundle-and-generate`, `generate-feature`, `openapi-bundle` targets.

Add:
```makefile
## Swagger
swagger: ## Generate swagger docs
	swag init -g cmd/main.go --parseDependency --parseInternal

sdk: swagger ## Generate swagger docs and TypeScript SDK
	@echo "TypeScript SDK generation can be added here (openapi-ts)"
```

- [ ] **Step 8: Run swag init**

```bash
cd /home/jason/jdk/prism
swag init -g cmd/main.go --parseDependency --parseInternal
```

Expected: Creates `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`.

- [ ] **Step 9: Verify full compilation**

```bash
cd /home/jason/jdk/prism && go build ./...
```

- [ ] **Step 10: Commit**

```bash
cd /home/jason/jdk/prism
git add -A
git commit -m "feat: complete prism swaggo migration — remove oapi-codegen, generate swagger docs"
```

---

### Note: Gateway Feature (Prism)

The gateway's `ServiceAPIHandler` uses `http.ServeMux` (not chi) and multiplexes multiple HTTP methods per handler function (`handleServices` handles both GET and POST). swaggo annotations require one annotated function per operation. Adding annotations would require splitting each handler into per-method functions and converting to chi routing — a minor refactor that's better done as a follow-up rather than complicating this migration. The gateway endpoints are internal admin APIs.

---

## Task 10: Final verification — both services

- [ ] **Step 1: Verify Aegis compiles and generates swagger**

```bash
cd /home/jason/jdk/aegis
make swagger
go build ./...
```

- [ ] **Step 2: Verify Prism compiles and generates swagger**

```bash
cd /home/jason/jdk/prism
make swagger
go build ./...
```

- [ ] **Step 3: Verify go.work still works**

```bash
cd /home/jason/jdk/aegis
go build ./...
```

The `go.work` at the aegis root includes both repos. Make sure the workspace builds cleanly.

- [ ] **Step 4: Spot-check swagger output**

```bash
cd /home/jason/jdk/aegis && cat docs/swagger.json | head -50
cd /home/jason/jdk/prism && cat docs/swagger.json | head -50
```

Verify the generated spec contains the expected endpoints, security definitions, and type schemas.

- [ ] **Step 5: Final commit (if any fixes needed)**

```bash
cd /home/jason/jdk/aegis && git add -A && git status
cd /home/jason/jdk/prism && git add -A && git status
```
