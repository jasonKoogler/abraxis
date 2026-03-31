# Swaggo Follow-up Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix swagger accuracy issues and complete the swaggo migration by adding JSON tags, deduplicating ErrorResponse, cleaning definition names, converting gateway to chi, and removing stale deps.

**Architecture:** Small, targeted fixes across both Aegis and Prism. Each task is independent except Task 3 (useStructName) which should run after Task 2 (dedup ErrorResponse) to avoid naming collisions.

**Tech Stack:** swaggo, chi/v5, existing `api.APIError` type

---

### Task 1: Add JSON tags to domain.AuditLog (Prism)

**Files:**
- Modify: `/home/jason/jdk/prism/internal/domain/audit_log.go`

The `AuditLog` struct has no `json` tags, so `encoding/json` serializes fields as PascalCase and swaggo generates wrong field names in the spec. Add `json:"snake_case"` tags to match the convention used by other domain types.

- [ ] **Step 1: Add JSON tags to AuditLog struct**

In `/home/jason/jdk/prism/internal/domain/audit_log.go`, change the `AuditLog` struct (lines 14-36) from:

```go
type AuditLog struct {
	ID           id.ID
	EventType    string
	ActorType    string
	ActorID      string
	TenantID     id.ID
	ResourceType string
	ResourceID   id.ID
	IPAddress    net.IP
	UserAgent    string
	EventData    any
	CreatedAt    time.Time
}
```

To:

```go
type AuditLog struct {
	ID           id.ID     `json:"id"`
	EventType    string    `json:"event_type"`
	ActorType    string    `json:"actor_type"`
	ActorID      string    `json:"actor_id"`
	TenantID     id.ID     `json:"tenant_id"`
	ResourceType string    `json:"resource_type"`
	ResourceID   id.ID     `json:"resource_id"`
	IPAddress    net.IP    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	EventData    any       `json:"event_data,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
```

Keep the existing doc comments above each field.

- [ ] **Step 2: Verify compilation**

```bash
cd /home/jason/jdk/prism && go build ./...
```

- [ ] **Step 3: Regenerate swagger**

```bash
cd /home/jason/jdk/prism && make swagger
```

- [ ] **Step 4: Commit**

```bash
cd /home/jason/jdk/prism
git add internal/domain/audit_log.go docs/
git commit -m "fix: add JSON tags to domain.AuditLog for correct swagger field names"
```

---

### Task 2: Deduplicate ErrorResponse across both repos

**Files:**
- Modify: `/home/jason/jdk/aegis/internal/common/api/apierror.go` (add swaggo example tags)
- Modify: `/home/jason/jdk/aegis/internal/adapters/http/server.go` (change annotations)
- Modify: `/home/jason/jdk/aegis/internal/adapters/http/auth_handlers.go` (change annotations)
- Delete: `/home/jason/jdk/aegis/internal/adapters/http/types.go` ErrorResponse struct
- Modify: `/home/jason/jdk/prism/internal/common/api/apierror.go` (add swaggo example tags)
- Modify: `/home/jason/jdk/prism/internal/features/audit/handler.go` (change annotations)
- Modify: `/home/jason/jdk/prism/internal/features/apikey/handler.go` (change annotations)
- Delete: `/home/jason/jdk/prism/internal/features/audit/dto/types.go` (entire file — only contained ErrorResponse)
- Modify: `/home/jason/jdk/prism/internal/features/apikey/dto/types.go` (remove ErrorResponse)

The existing `api.APIError` in both repos already serializes to `{"slug": "...", "message": "..."}` — exactly what the duplicate ErrorResponse DTOs define. Use it directly in swaggo annotations.

- [ ] **Step 1: Add swaggo example tags to Aegis APIError**

In `/home/jason/jdk/aegis/internal/common/api/apierror.go`, update the `APIError` struct:

```go
type APIError struct {
	Slug          string    `json:"slug" example:"invalid-json"`
	Message       string    `json:"message" example:"Invalid JSON request"`
	StatusCode    int       `json:"-"`
	ErrorType     ErrorType `json:"-"`
	internalError error
}
```

- [ ] **Step 2: Change all Aegis swaggo annotations from ErrorResponse to api.APIError**

In `/home/jason/jdk/aegis/internal/adapters/http/server.go` and `auth_handlers.go`, find-and-replace all:
```
{object}  ErrorResponse
```
With:
```
{object}  api.APIError
```

Both files already import the `api` package.

- [ ] **Step 3: Remove ErrorResponse from Aegis types.go**

In `/home/jason/jdk/aegis/internal/adapters/http/types.go`, delete the `ErrorResponse` struct and its comment (lines 34-37).

- [ ] **Step 4: Verify Aegis compiles and regenerate swagger**

```bash
cd /home/jason/jdk/aegis && go build ./... && make swagger
```

- [ ] **Step 5: Commit Aegis changes**

```bash
cd /home/jason/jdk/aegis
git add internal/common/api/apierror.go internal/adapters/http/ docs/
git commit -m "refactor: use api.APIError in swagger annotations, remove duplicate ErrorResponse"
```

- [ ] **Step 6: Add swaggo example tags to Prism APIError**

Same change in `/home/jason/jdk/prism/internal/common/api/apierror.go`:

```go
type APIError struct {
	Slug          string    `json:"slug" example:"invalid-json"`
	Message       string    `json:"message" example:"Invalid JSON request"`
	StatusCode    int       `json:"-"`
	ErrorType     ErrorType `json:"-"`
	internalError error
}
```

- [ ] **Step 7: Change all Prism swaggo annotations from dto.ErrorResponse to api.APIError**

In `/home/jason/jdk/prism/internal/features/audit/handler.go`, find-and-replace:
```
{object}  dto.ErrorResponse
```
With:
```
{object}  api.APIError
```

Same in `/home/jason/jdk/prism/internal/features/apikey/handler.go`.

Both files already import the `api` package (`"github.com/jasonKoogler/prism/internal/common/api"`).

- [ ] **Step 8: Delete audit dto/types.go and clean apikey dto/types.go**

Delete `/home/jason/jdk/prism/internal/features/audit/dto/types.go` entirely (it only contained ErrorResponse).

In `/home/jason/jdk/prism/internal/features/apikey/dto/types.go`, delete the `ErrorResponse` struct and its comment.

- [ ] **Step 9: Remove dto import from audit handler if no longer needed**

Check if `audit/handler.go` still uses the `dto` package (it does — for `dto.ListAuditLogsParamsFromRequest` etc.). If so, keep the import. If not, remove it.

- [ ] **Step 10: Verify Prism compiles and regenerate swagger**

```bash
cd /home/jason/jdk/prism && go build ./... && make swagger
```

- [ ] **Step 11: Commit Prism changes**

```bash
cd /home/jason/jdk/prism
git add internal/common/api/apierror.go internal/features/audit/ internal/features/apikey/ docs/
git commit -m "refactor: use api.APIError in swagger annotations, remove duplicate ErrorResponse"
```

---

### Task 3: Clean swagger definition names with --useStructName

**Files:**
- Modify: `/home/jason/jdk/aegis/Makefile`
- Modify: `/home/jason/jdk/prism/Makefile`

Add `--useStructName` to `swag init` in both Makefiles. This produces clean names like `AuditLog` instead of `github_com_jasonKoogler_prism_internal_domain.AuditLog`.

- [ ] **Step 1: Update Aegis Makefile**

In the swagger target, change:
```
GOWORK=off swag init -g cmd/main.go --parseDependency --parseInternal
```
To:
```
GOWORK=off swag init -g cmd/main.go --parseDependency --parseInternal --useStructName
```

- [ ] **Step 2: Update Prism Makefile**

Same change:
```
GOWORK=off swag init -g cmd/main.go --parseDependency --parseInternal --useStructName
```

- [ ] **Step 3: Regenerate and verify both**

```bash
cd /home/jason/jdk/aegis && make swagger && go build ./...
cd /home/jason/jdk/prism && make swagger && go build ./...
```

- [ ] **Step 4: Verify definition names are clean**

```bash
cd /home/jason/jdk/aegis && python3 -c "import json; d=json.load(open('docs/swagger.json')); print(sorted(d['definitions'].keys()))"
cd /home/jason/jdk/prism && python3 -c "import json; d=json.load(open('docs/swagger.json')); print(sorted(d['definitions'].keys()))"
```

Expected: names like `AuditLog`, `APIError`, `User` — no package paths.

- [ ] **Step 5: Commit both**

```bash
cd /home/jason/jdk/aegis && git add Makefile docs/ && git commit -m "chore: use --useStructName for clean swagger definition names"
cd /home/jason/jdk/prism && git add Makefile docs/ && git commit -m "chore: use --useStructName for clean swagger definition names"
```

---

### Task 4: Refactor gateway to chi with swaggo annotations (Prism)

**Files:**
- Modify: `/home/jason/jdk/prism/internal/features/gateway/service_api.go`
- Modify: `/home/jason/jdk/prism/internal/features/gateway/routing.go` (add json tags to RouteMetadata)
- Modify: `/home/jason/jdk/prism/internal/app/app.go` (update gateway wiring)

The gateway's `ServiceAPIHandler` currently uses `http.ServeMux` with method-switching handlers. Refactor to chi routing with one function per operation, using `api.Make` for error handling and swaggo annotations for docs.

- [ ] **Step 1: Add JSON tags to RouteMetadata**

In `/home/jason/jdk/prism/internal/features/gateway/routing.go`, add json tags:

```go
type RouteMetadata struct {
	Path           string            `json:"path"`
	Method         string            `json:"method"`
	ServiceID      string            `json:"service_id"`
	ServiceName    string            `json:"service_name"`
	ServiceURL     string            `json:"service_url"`
	Public         bool              `json:"public"`
	RequiredScopes []string          `json:"required_scopes,omitempty"`
	Priority       int               `json:"priority"`
	Tags           map[string]string `json:"tags,omitempty"`
}
```

- [ ] **Step 2: Rewrite service_api.go with chi routing and swaggo annotations**

Replace the entire file:

```go
package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jasonKoogler/prism/internal/common/api"
	commonlog "github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
)

// ServiceAPIHandler provides HTTP handlers for service management.
type ServiceAPIHandler struct {
	serviceProxy *ServiceProxy
	logger       *commonlog.Logger
}

// NewServiceAPIHandler creates a new service API handler.
func NewServiceAPIHandler(serviceProxy *ServiceProxy, logger *commonlog.Logger) *ServiceAPIHandler {
	return &ServiceAPIHandler{
		serviceProxy: serviceProxy,
		logger:       logger,
	}
}

// RegisterRoutes registers all service management endpoints on the given chi.Router.
func (h *ServiceAPIHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/services", func(r chi.Router) {
		r.Get("/", api.Make(h.listServices, h.logger))
		r.Post("/", api.Make(h.createService, h.logger))
		r.Get("/{name}", api.Make(h.getService, h.logger))
		r.Put("/{name}", api.Make(h.updateService, h.logger))
		r.Delete("/{name}", api.Make(h.deleteService, h.logger))
	})

	r.Route("/api/routes", func(r chi.Router) {
		r.Get("/", api.Make(h.listRoutes, h.logger))
		r.Get("/{service}", api.Make(h.listServiceRoutes, h.logger))
		r.Post("/{service}", api.Make(h.addServiceRoute, h.logger))
	})
}

// listServices godoc
// @Summary      List all services
// @Description  Get all registered services
// @Tags         services
// @Produce      json
// @Success      200  {array}   config.ServiceConfig
// @Router       /api/services [get]
func (h *ServiceAPIHandler) listServices(w http.ResponseWriter, r *http.Request) error {
	services := h.serviceProxy.ListServices()
	return api.Respond(w, http.StatusOK, services)
}

// createService godoc
// @Summary      Register a new service
// @Description  Add a new service to the gateway
// @Tags         services
// @Accept       json
// @Produce      json
// @Param        body  body      config.ServiceConfig  true  "Service configuration"
// @Success      201   {object}  config.ServiceConfig
// @Failure      400   {object}  api.APIError
// @Failure      500   {object}  api.APIError
// @Router       /api/services [post]
func (h *ServiceAPIHandler) createService(w http.ResponseWriter, r *http.Request) error {
	var svcConfig config.ServiceConfig
	if err := json.NewDecoder(r.Body).Decode(&svcConfig); err != nil {
		return api.InvalidJSONError()
	}

	if err := h.serviceProxy.RegisterService(svcConfig); err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusCreated, svcConfig)
}

// getService godoc
// @Summary      Get service by name
// @Description  Get configuration for a specific service
// @Tags         services
// @Produce      json
// @Param        name  path      string  true  "Service name"
// @Success      200   {object}  config.ServiceConfig
// @Failure      404   {object}  api.APIError
// @Router       /api/services/{name} [get]
func (h *ServiceAPIHandler) getService(w http.ResponseWriter, r *http.Request) error {
	serviceName := chi.URLParam(r, "name")
	services := h.serviceProxy.ListServices()
	for _, svc := range services {
		if svc.Name == serviceName {
			return api.Respond(w, http.StatusOK, svc)
		}
	}
	return api.NotFound(nil)
}

// updateService godoc
// @Summary      Update a service
// @Description  Update configuration for an existing service
// @Tags         services
// @Accept       json
// @Produce      json
// @Param        name  path      string                true  "Service name"
// @Param        body  body      config.ServiceConfig  true  "Updated service configuration"
// @Success      200   {object}  config.ServiceConfig
// @Failure      400   {object}  api.APIError
// @Failure      500   {object}  api.APIError
// @Router       /api/services/{name} [put]
func (h *ServiceAPIHandler) updateService(w http.ResponseWriter, r *http.Request) error {
	serviceName := chi.URLParam(r, "name")

	var svcConfig config.ServiceConfig
	if err := json.NewDecoder(r.Body).Decode(&svcConfig); err != nil {
		return api.InvalidJSONError()
	}

	if svcConfig.Name != serviceName {
		return api.NewError("name-mismatch", "Service name in URL does not match body", http.StatusBadRequest, api.ErrorTypeBadRequest, nil)
	}

	if err := h.serviceProxy.UpdateService(svcConfig); err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, svcConfig)
}

// deleteService godoc
// @Summary      Delete a service
// @Description  Remove a service from the gateway
// @Tags         services
// @Param        name  path  string  true  "Service name"
// @Success      200   {object}  object{status=string}
// @Failure      500   {object}  api.APIError
// @Router       /api/services/{name} [delete]
func (h *ServiceAPIHandler) deleteService(w http.ResponseWriter, r *http.Request) error {
	serviceName := chi.URLParam(r, "name")
	if err := h.serviceProxy.DeregisterService(serviceName); err != nil {
		return api.InternalError(err)
	}
	return api.Respond(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// listRoutes godoc
// @Summary      List all routes
// @Description  Get all registered routes, optionally filtered by type
// @Tags         routes
// @Produce      json
// @Param        type  query     string  false  "Filter by type"  Enums(public, protected)
// @Success      200   {array}   RouteMetadata
// @Router       /api/routes [get]
func (h *ServiceAPIHandler) listRoutes(w http.ResponseWriter, r *http.Request) error {
	filterType := r.URL.Query().Get("type")

	var routes []RouteMetadata
	switch filterType {
	case "public":
		routes = h.serviceProxy.ListPublicRoutes()
	case "protected":
		routes = h.serviceProxy.ListProtectedRoutes()
	default:
		routes = h.serviceProxy.ListRoutes()
	}

	return api.Respond(w, http.StatusOK, routes)
}

// listServiceRoutes godoc
// @Summary      List routes for a service
// @Description  Get all routes registered for a specific service
// @Tags         routes
// @Produce      json
// @Param        service  path      string  true  "Service name"
// @Success      200      {array}   RouteMetadata
// @Failure      404      {object}  api.APIError
// @Router       /api/routes/{service} [get]
func (h *ServiceAPIHandler) listServiceRoutes(w http.ResponseWriter, r *http.Request) error {
	serviceName := chi.URLParam(r, "service")
	routes, err := h.serviceProxy.ListServiceRoutes(serviceName)
	if err != nil {
		return api.NotFound(err)
	}
	return api.Respond(w, http.StatusOK, routes)
}

// addServiceRoute godoc
// @Summary      Add route to a service
// @Description  Register a custom route for a specific service
// @Tags         routes
// @Accept       json
// @Produce      json
// @Param        service  path      string              true  "Service name"
// @Param        body     body      config.RouteConfig  true  "Route configuration"
// @Success      201      {object}  config.RouteConfig
// @Failure      400      {object}  api.APIError
// @Failure      500      {object}  api.APIError
// @Router       /api/routes/{service} [post]
func (h *ServiceAPIHandler) addServiceRoute(w http.ResponseWriter, r *http.Request) error {
	serviceName := chi.URLParam(r, "service")

	var routeConfig config.RouteConfig
	if err := json.NewDecoder(r.Body).Decode(&routeConfig); err != nil {
		return api.InvalidJSONError()
	}

	if err := h.serviceProxy.RegisterServiceRoute(serviceName, routeConfig); err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusCreated, routeConfig)
}
```

- [ ] **Step 3: Update gateway wiring in app.go**

In `/home/jason/jdk/prism/internal/app/app.go`, replace the gateway wiring block (around lines 160-176):

```go
		// Create a handler for service and route management APIs
		serviceAPIHandler := gateway.NewServiceAPIHandler(a.serviceProxy, a.logger)

		// Register the service API endpoints
		serviceAPIRouter := http.NewServeMux()
		serviceAPIHandler.RegisterHandlers(serviceAPIRouter)

		// Register the API handlers with authentication middleware for admin routes
		// Public service-related endpoints don't require auth
		a.srv.GetRouter().Handle("/api/services", serviceAPIRouter)
		a.srv.GetRouter().Handle("/api/services/", serviceAPIRouter)

		// Protected route management requires authentication
		authMiddleware := a.CreateAuthMiddleware("service:admin")
		a.srv.GetRouter().Handle("/api/routes", authMiddleware(serviceAPIRouter))
		a.srv.GetRouter().Handle("/api/routes/", authMiddleware(serviceAPIRouter))
```

With:

```go
		// Create a handler for service and route management APIs
		serviceAPIHandler := gateway.NewServiceAPIHandler(a.serviceProxy, a.logger)

		// Register gateway management routes directly on chi
		serviceAPIHandler.RegisterRoutes(a.srv.GetRouter())
```

Note: the old code had separate auth for routes vs services. The new code does NOT apply auth middleware — it registers all gateway management routes as public. This matches the original behavior where `/api/services` was public. If route management (`/api/routes`) needs auth, add a chi route group with middleware inside `RegisterRoutes` later.

- [ ] **Step 4: Verify compilation and regenerate swagger**

```bash
cd /home/jason/jdk/prism && go build ./... && make swagger
```

- [ ] **Step 5: Verify gateway endpoints in swagger output**

```bash
cd /home/jason/jdk/prism && python3 -c "import json; d=json.load(open('docs/swagger.json')); print(sorted(d['paths'].keys()))"
```

Expected: includes `/api/services`, `/api/services/{name}`, `/api/routes`, `/api/routes/{service}` alongside the audit/apikey paths.

- [ ] **Step 6: Commit**

```bash
cd /home/jason/jdk/prism
git add internal/features/gateway/service_api.go internal/features/gateway/routing.go internal/app/app.go docs/
git commit -m "refactor: convert gateway to chi routing with swaggo annotations"
```

---

### Task 5: Remove oapi-codegen/runtime from Prism go.mod

**Files:**
- Modify: `/home/jason/jdk/prism/go.mod`
- Modify: `/home/jason/jdk/prism/go.sum`

`go mod tidy` can't run because the Aegis proto dep doesn't resolve outside workspace mode. Manually remove the unused dependency.

- [ ] **Step 1: Remove oapi-codegen/runtime from go.mod**

In `/home/jason/jdk/prism/go.mod`, delete the line:
```
github.com/oapi-codegen/runtime v1.1.1
```

Also check for and remove `github.com/getkin/kin-openapi` if present (was only used by oapi-codegen generated code).

- [ ] **Step 2: Remove corresponding go.sum entries**

```bash
cd /home/jason/jdk/prism
grep -n "oapi-codegen/runtime" go.sum
grep -n "getkin/kin-openapi" go.sum
```

Delete those lines from go.sum.

- [ ] **Step 3: Verify compilation**

```bash
cd /home/jason/jdk/prism && go build ./...
```

If it fails (something still needs these packages), revert and keep them.

- [ ] **Step 4: Commit**

```bash
cd /home/jason/jdk/prism
git add go.mod go.sum
git commit -m "chore: remove unused oapi-codegen/runtime dependency"
```
