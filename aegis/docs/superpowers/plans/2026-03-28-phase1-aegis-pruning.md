# Phase 1: Aegis Gateway Code Pruning

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all gateway-related code from Aegis so it is purely an auth service.

**Architecture:** Aegis currently contains service proxy, service discovery (4 providers), circuit breaker, schema registry, and routing table code that belongs in Prism (the API gateway). This plan removes that code, updates all wiring, cleans up dead files, and verifies the auth service still compiles and tests pass.

**Tech Stack:** Go 1.24, chi/v5, pgx/v5, go-redis/v9

**Spec:** `docs/superpowers/specs/2026-03-28-aegis-prism-separation-design.md`

**Phase Context:** This is Phase 1 of 4. Phases 2-4 (Prism pruning, gRPC contract, integration) follow independently.

---

### Task 1: Verify baseline — ensure current code compiles and tests pass

**Files:** None modified

- [ ] **Step 1: Run go build to verify compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

Expected: Successful compilation, no errors.

- [ ] **Step 2: Run existing tests**

```bash
cd /home/jason/jdk/aegis && go test ./internal/... -count=1 -short 2>&1 | tail -30
```

Expected: Tests pass (or note which tests fail before pruning so we don't introduce regressions).

- [ ] **Step 3: Record baseline state**

Note any pre-existing failures so they aren't confused with pruning regressions.

---

### Task 2: Remove orphaned and dead files

**Files:**
- Delete: `login.ts`
- Delete: `scripts/openapi-http copy.sh`
- Delete: `scripts/openapi-http copy 2.sh`
- Delete: `policies/gauth_policy.rego`

- [ ] **Step 1: Delete orphaned files**

```bash
cd /home/jason/jdk/aegis
rm -f login.ts
rm -f "scripts/openapi-http copy.sh"
rm -f "scripts/openapi-http copy 2.sh"
rm -f policies/gauth_policy.rego
```

- [ ] **Step 2: Update AuthZ config default to point to authz.rego**

In `internal/config/config.go`, the `AuthZConfig` struct has a default policy path pointing to `gauth_policy.rego`. Update it:

```go
// Before:
PolicyFilePath string `env:"AUTHZ_POLICY_FILE_PATH" envDefault:"policies/gauth_policy.rego"`

// After:
PolicyFilePath string `env:"AUTHZ_POLICY_FILE_PATH" envDefault:"policies/authz.rego"`
```

- [ ] **Step 3: Verify compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

Expected: Still compiles — these files weren't imported by Go code.

- [ ] **Step 4: Commit**

```bash
cd /home/jason/jdk/aegis
git add -A
git commit -m "chore: remove orphaned files and consolidate policy path

Remove login.ts, duplicate scripts, and gauth_policy.rego.
Update default policy path to authz.rego."
```

---

### Task 3: Remove discovery adapter directory

**Files:**
- Delete: `internal/adapters/discovery/` (entire directory — 18 files)
- Delete: `internal/ports/discovery.go`

- [ ] **Step 1: Delete discovery adapter and port**

```bash
cd /home/jason/jdk/aegis
rm -rf internal/adapters/discovery/
rm -f internal/ports/discovery.go
```

- [ ] **Step 2: Verify what breaks**

```bash
cd /home/jason/jdk/aegis && go build ./... 2>&1 | head -40
```

Expected: Compilation errors in `internal/app/options.go` (references `adapters "github.com/jasonKoogler/aegis/internal/adapters/discovery"` and `ports.ServiceDiscoverer`), and `internal/app/app.go` (references `ports.ServiceDiscoverer`, `a.serviceDiscovery`).

These will be fixed in Task 6 when we update app wiring.

- [ ] **Step 3: Commit (broken state is OK — we're batching related deletions)**

Do not commit yet — wait until Task 6 restores compilation.

---

### Task 4: Remove circuit breaker adapter directory

**Files:**
- Delete: `internal/adapters/circuitbreaker/` (entire directory — 11 files)
- Delete: `internal/ports/circuitbreaker.go`

- [ ] **Step 1: Delete circuit breaker adapter and port**

```bash
cd /home/jason/jdk/aegis
rm -rf internal/adapters/circuitbreaker/
rm -f internal/ports/circuitbreaker.go
```

Do not commit yet — wait until Task 6 restores compilation.

---

### Task 5: Remove gateway HTTP files, services, domain types, and ports

**Files:**
- Delete: `internal/adapters/http/proxy.go`
- Delete: `internal/adapters/http/proxy_options.go`
- Delete: `internal/adapters/http/routing.go`
- Delete: `internal/adapters/http/service_api.go`
- Delete: `internal/adapters/http/service_registry.go`
- Delete: `internal/adapters/http/schema_registry_handlers.go`
- Delete: `internal/service/service_schema_registry.go`
- Delete: `internal/service/service_backend.go`
- Delete: `internal/service/service_proto.go`
- Delete: `internal/domain/api_route.go`
- Delete: `internal/domain/schema.go`
- Delete: `internal/ports/repo_api_routes.go`
- Delete: `internal/ports/repo_schema.go`
- Delete: `internal/ports/proto/schema_registry_grpc.pb.go`
- Delete: `internal/ports/proto/schema_registry.pb.go`

Note: `internal/ports/openapi-server.gen.go` and `internal/ports/openapi-types.gen.go` are KEPT — they define the auth `ServerInterface` and `HandlerFromMux` used for route registration. They will be replaced by swaggo in a later phase.

- [ ] **Step 1: Delete gateway HTTP handlers**

```bash
cd /home/jason/jdk/aegis
rm -f internal/adapters/http/proxy.go
rm -f internal/adapters/http/proxy_options.go
rm -f internal/adapters/http/routing.go
rm -f internal/adapters/http/service_api.go
rm -f internal/adapters/http/service_registry.go
rm -f internal/adapters/http/schema_registry_handlers.go
```

- [ ] **Step 2: Delete gateway services**

```bash
cd /home/jason/jdk/aegis
rm -f internal/service/service_schema_registry.go
rm -f internal/service/service_backend.go
rm -f internal/service/service_proto.go
```

- [ ] **Step 3: Delete gateway domain types**

```bash
cd /home/jason/jdk/aegis
rm -f internal/domain/api_route.go
rm -f internal/domain/schema.go
```

- [ ] **Step 4: Delete gateway ports**

```bash
cd /home/jason/jdk/aegis
rm -f internal/ports/repo_api_routes.go
rm -f internal/ports/repo_schema.go
rm -rf internal/ports/proto/
# Keep openapi-server.gen.go and openapi-types.gen.go — they define auth routes
```

Do not commit yet — wait until Task 6 restores compilation.

---

### Task 6: Update app wiring — remove gateway references

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/app/options.go`
- Modify: `internal/app/errors.go`
- Modify: `internal/config/config.go`

This is the critical task that restores compilation after Tasks 3-5 deleted files.

- [ ] **Step 1: Rewrite `internal/app/app.go` — remove gateway fields and methods**

Remove from the `App` struct:
- `circuitBreaker     ports.CircuitBreaker`
- `serviceProxy       *adaptersHTTP.ServiceProxy`
- `serviceDiscovery   ports.ServiceDiscoverer`

Remove these methods entirely:
- `startServiceWatcher`
- `registerServiceInstance`
- `deregisterServiceInstance`
- `RegisterServiceRoute`
- `RegisterPublicRoute`
- `RegisterProtectedRoute`
- `GetServiceRoutes`
- `GetAllRoutes`
- `GetPublicRoutes`
- `GetProtectedRoutes`

Remove from the `Start()` method:
- The entire block that initializes and registers the service proxy (lines 127-165)
- The service discovery watcher block (lines 168-172)

Remove from the `NewApp()` constructor:
- The block that creates default service proxy (lines 103-111)

Remove unused imports:
- `"fmt"` (check if still needed by remaining code — it is, used in `extractAuthZInput`)
- `"reflect"` (check if still needed — it is, used in `CreateAuthMiddleware`)
- `"strconv"` (no longer needed after removing `registerServiceInstance`)
- `"strings"` (check — still needed for `ConditionalAuthMiddleware` and `createCorsMiddleware`)
- `"time"` (no longer needed after removing `registerServiceInstance`)

The resulting `App` struct should be:

```go
type App struct {
	ctx                context.Context
	cfg                *config.Config
	logger             *log.Logger
	userRepo           ports.UserRepository
	userService        *service.UserService
	authService        *service.AuthManager
	authzService       *authz.Adapter
	rateLimiter        ports.RateLimiter
	srv                *Server
	redisClient        *redis.RedisClient
	auditService       *service.AuditService
	auditRepo          ports.AuditLogRepository
	apiKeyService      *service.APIKeyService
	apiKeyRepo         ports.APIKeyRepository
	tenantService      *service.TenantDomainService
	tenantRepo         ports.TenantRepository
	permissionService  *service.PermissionService
	permissionRepo     ports.PermissionRepository
	rolePermissionRepo ports.RolePermissionRepository
}
```

The resulting `NewApp()` should remove the service proxy block at the end:

```go
func NewApp(opts ...AppOption) (*App, error) {
	app := &App{}
	app.ctx = context.Background()

	for _, opt := range opts {
		if err := opt(app); err != nil {
			return nil, err
		}
	}

	if app.cfg == nil {
		return nil, ErrConfigRequired
	}
	if app.logger == nil {
		app.logger = log.NewLogger(app.cfg.LogLevel.String())
	}
	if app.userRepo == nil {
		return nil, ErrUserRepositoryRequired
	}
	if app.userService == nil {
		app.userService = service.NewUserService(app.userRepo)
	}
	if app.authService == nil {
		var err error
		app.authService, err = service.NewAuthManager(&app.cfg.Auth, app.logger, app.userService)
		if err != nil {
			return nil, err
		}
	}
	if app.redisClient == nil {
		redisClient, err := redis.NewRedisClient(app.ctx, app.logger, &app.cfg.Auth.AuthN.RedisConfig)
		if err != nil {
			return nil, err
		}
		app.redisClient = redisClient
	}
	if app.srv == nil {
		var err error
		app.srv, err = NewServer(app.cfg, app.logger, WithAuthService(app.authService))
		if err != nil {
			return nil, err
		}
	}

	return app, nil
}
```

The resulting `Start()` should be:

```go
func (a *App) Start() error {
	a.srv.GetRouter().Use(createCorsMiddleware(a.cfg))
	a.srv.GetRouter().Use(chimiddleware.RequestID)
	a.srv.GetRouter().Use(chimiddleware.RealIP)
	a.srv.GetRouter().Use(chimiddleware.Recoverer)
	a.srv.GetRouter().Use(chimiddleware.Logger)

	authHandler := ports.HandlerFromMux(adaptersHTTP.NewServer(a.authService, a.userService, a.cfg, a.logger), a.srv.GetRouter())
	wrappedAuthHandler := ConditionalAuthMiddleware(a.authService, authHandler)

	publicHandlers := map[string]http.Handler{
		"/health": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}),
		"/docs": http.FileServer(http.Dir("./docs")),
	}

	protectedHandlers := map[string]http.Handler{
		"/auth": wrappedAuthHandler,
	}

	ctx := context.Background()
	if err := a.srv.Start(ctx, publicHandlers, protectedHandlers); err != nil {
		return err
	}

	a.srv.Wait()
	return nil
}
```

Update import block — remove unused imports:

```go
import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jasonKoogler/aegis/internal/adapters/authz"
	adaptersHTTP "github.com/jasonKoogler/aegis/internal/adapters/http"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/jasonKoogler/aegis/internal/common/redis"
	"github.com/jasonKoogler/aegis/internal/config"
	"github.com/jasonKoogler/aegis/internal/domain"
	"github.com/jasonKoogler/aegis/internal/ports"
	"github.com/jasonKoogler/aegis/internal/service"
)
```

Removed: `"strconv"`, `"time"`.

- [ ] **Step 2: Rewrite `internal/app/options.go` — remove gateway options**

Remove these functions entirely:
- `WithCircuitBreaker` (lines 207-216)
- `WithServiceProxy` (lines 218-227)
- `WithDefaultCircuitBreaker` (lines 229-269)
- `WithServiceDiscovery` (lines 305-314)
- `WithDefaultServiceDiscovery` (lines 316-338)
- `WithDefaultServiceProxy` (lines 601-631)

Remove from `WithAllDefaultServices` (lines 575-599) these lines:
- `WithDefaultServiceDiscovery(ctx),`
- `WithDefaultCircuitBreaker(),`

The resulting `WithAllDefaultServices` should be:

```go
func WithAllDefaultServices(ctx context.Context) AppOption {
	return func(a *App) error {
		options := []AppOption{
			WithDefaultUserService(),
			WithDefaultAuthService(),
			WithDefaultAuthZService(),
			WithDefaultAuditService(),
			WithDefaultAPIKeyService(),
			WithDefaultTenantService(),
			WithDefaultPermissionService(),
			WithDefaultServer(),
		}

		for _, opt := range options {
			_ = opt(a)
		}

		return nil
	}
}
```

Update import block — remove unused imports:

```go
import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jasonKoogler/aegis/internal/adapters/authz"
	adaptersHTTP "github.com/jasonKoogler/aegis/internal/adapters/http"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/jasonKoogler/aegis/internal/common/redis"
	"github.com/jasonKoogler/aegis/internal/config"
	"github.com/jasonKoogler/aegis/internal/ports"
	"github.com/jasonKoogler/aegis/internal/service"
)
```

Removed: `adapters "github.com/jasonKoogler/aegis/internal/adapters/discovery"`, `"github.com/jasonKoogler/aegis/internal/adapters/circuitbreaker"`.

Note: `adaptersHTTP` import may no longer be needed in options.go if no remaining option references it. Check after edits — if unused, remove it too.

- [ ] **Step 3: Update `internal/app/errors.go` — remove gateway errors**

Remove these error variables:
- `ErrNilCircuitBreaker`
- `ErrNilServiceProxy`
- `ErrNilServiceDiscovery`

The remaining errors file should be:

```go
package app

import "errors"

var (
	ErrNilConfig      = errors.New("config cannot be nil")
	ErrConfigRequired = errors.New("config is required")

	ErrNilLogger      = errors.New("logger cannot be nil")
	ErrLoggerRequired = errors.New("logger is required")

	ErrNilUserRepository      = errors.New("user repository cannot be nil")
	ErrUserRepositoryRequired = errors.New("user repository is required")

	ErrNilUserService      = errors.New("user service cannot be nil")
	ErrUserServiceRequired = errors.New("user service is required")

	ErrNilAuthService      = errors.New("auth service cannot be nil")
	ErrAuthServiceRequired = errors.New("auth service is required")

	ErrNilAuthZService      = errors.New("authorization service cannot be nil")
	ErrAuthZServiceRequired = errors.New("authorization service is required")

	ErrNilRateLimiter = errors.New("rate limiter cannot be nil")

	ErrNilRedisClient = errors.New("redis client cannot be nil")

	ErrNilServer = errors.New("server cannot be nil")

	ErrNilAuditService         = errors.New("audit service cannot be nil")
	ErrAuditServiceRequired    = errors.New("audit service is required")
	ErrNilAuditRepository      = errors.New("audit repository cannot be nil")
	ErrAuditRepositoryRequired = errors.New("audit repository is required")

	ErrNilAPIKeyService         = errors.New("API key service cannot be nil")
	ErrAPIKeyServiceRequired    = errors.New("API key service is required")
	ErrNilAPIKeyRepository      = errors.New("API key repository cannot be nil")
	ErrAPIKeyRepositoryRequired = errors.New("API key repository is required")

	ErrNilTenantService         = errors.New("tenant service cannot be nil")
	ErrTenantServiceRequired    = errors.New("tenant service is required")
	ErrNilTenantRepository      = errors.New("tenant repository cannot be nil")
	ErrTenantRepositoryRequired = errors.New("tenant repository is required")

	ErrNilPermissionService             = errors.New("permission service cannot be nil")
	ErrPermissionServiceRequired        = errors.New("permission service is required")
	ErrNilPermissionRepository          = errors.New("permission repository cannot be nil")
	ErrPermissionRepositoryRequired     = errors.New("permission repository is required")
	ErrNilRolePermissionRepository      = errors.New("role-permission repository cannot be nil")
	ErrRolePermissionRepositoryRequired = errors.New("role-permission repository is required")

	ErrAppInitialization = errors.New("failed to initialize app")
)
```

- [ ] **Step 4: Update `internal/config/config.go` — remove gateway config types**

Remove from the `Config` struct:
- `Services       []ServiceConfig`
- `CircuitBreaker *CircuitBreakerConfig`

Remove these type definitions entirely:
- `ServiceConfig` struct (lines 228-238)
- `RouteConfig` struct (lines 241-247)
- `CircuitBreakerConfig` struct (lines 622-631)
- `CircuitBreakerRedisConfig` struct (lines 634-639)

Remove from `LoadConfig()` function:
- The `Services` field initialization (lines 448-454)
- The `CircuitBreaker` field initialization (lines 469-478)

- [ ] **Step 5: Verify compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

Expected: Successful compilation. If there are additional references to removed types, fix them.

- [ ] **Step 6: Commit**

```bash
cd /home/jason/jdk/aegis
git add -A
git commit -m "refactor: remove all gateway code from aegis

Remove service proxy, service discovery (consul/etcd/k8s/local),
circuit breaker, schema registry, routing table, and related
domain types, ports, and handlers.

Aegis is now purely an auth service. Gateway functionality
belongs in Prism."
```

---

### Task 7: Clean up go.mod — remove unused gateway dependencies

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Run go mod tidy**

```bash
cd /home/jason/jdk/aegis && go mod tidy
```

This will remove dependencies that are no longer imported after pruning:
- `github.com/hashicorp/consul/api` (discovery)
- `go.etcd.io/etcd/client/v3` (discovery)
- `k8s.io/client-go` (discovery)
- `k8s.io/api` (discovery)
- `google.golang.org/grpc` (schema registry proto — verify still needed)
- `google.golang.org/protobuf` (schema registry proto — verify still needed)

Some of these may still be needed by other code. `go mod tidy` handles this automatically.

- [ ] **Step 2: Verify compilation after tidy**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

- [ ] **Step 3: Commit**

```bash
cd /home/jason/jdk/aegis
git add go.mod go.sum
git commit -m "chore: go mod tidy after gateway code removal"
```

---

### Task 8: Clean up stale documentation

**Files:**
- Delete: `docs/architecture/schema_registry.md`
- Delete: `docs/architecture/circuit_breaker.md`
- Delete: `docs/architecture/service_discovery.md`
- Delete: `docs/adapters/circuitbreaker.md`
- Delete: `docs/adapters/discovery.md`
- Delete: `docs/adapters/storage.md` (if it references gateway storage)
- Delete: `docs/adapters/http.md` (if it primarily documents proxy/gateway — read first)
- Delete: `docs/api/service_api.md`
- Delete: `api/grpc/proto/schema_registry.proto`

Note: `api/openapi/` is KEPT for now — it's the source for the generated auth route types. Will be replaced by swaggo in a later phase.

- [ ] **Step 1: Delete gateway documentation**

```bash
cd /home/jason/jdk/aegis
rm -f docs/architecture/schema_registry.md
rm -f docs/architecture/circuit_breaker.md
rm -f docs/architecture/service_discovery.md
rm -f docs/adapters/circuitbreaker.md
rm -f docs/adapters/discovery.md
rm -f docs/adapters/storage.md
rm -f docs/api/service_api.md
rm -rf api/grpc/
# Keep api/openapi/ — source for generated auth route types, replaced by swaggo later
```

- [ ] **Step 2: Read `docs/adapters/http.md` before deciding**

If it primarily documents the service proxy and gateway routing, delete it. If it documents auth handlers and middleware, keep it. Read the file to decide.

- [ ] **Step 3: Clean up scripts**

Remove the oapi-codegen script (keeping protogen.sh for now — may be needed for future gRPC work):

```bash
cd /home/jason/jdk/aegis
rm -f scripts/openapi-http.sh
rm -f scripts/generate-docs.sh
```

- [ ] **Step 4: Commit**

```bash
cd /home/jason/jdk/aegis
git add -A
git commit -m "docs: remove gateway-related documentation and API specs

Remove schema registry, circuit breaker, service discovery docs,
service API specs, and oapi-codegen pipeline. These belong in Prism."
```

---

### Task 9: Clean up Makefile — remove gateway targets

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Read the Makefile**

```bash
cd /home/jason/jdk/aegis && cat Makefile
```

Identify and remove:
- Any commented-out gateway targets
- OpenAPI generation targets (if they reference the deleted oapi-codegen pipeline)
- Discovery-specific test targets (`test-discovery-consul`, `test-discovery-etcd`, etc.)
- Any targets referencing deleted files or directories

Keep:
- `build`, `test`, `test-verbose`, `test-coverage`, `clean`
- Proto generation targets (will be needed for gRPC in Phase 3)
- Docker compose targets for auth service

- [ ] **Step 2: Edit the Makefile to remove gateway targets**

Remove the specific targets identified in Step 1. The exact edits depend on reading the file.

- [ ] **Step 3: Verify make targets work**

```bash
cd /home/jason/jdk/aegis && make build
```

- [ ] **Step 4: Commit**

```bash
cd /home/jason/jdk/aegis
git add Makefile
git commit -m "chore: clean up Makefile, remove gateway-related targets"
```

---

### Task 10: Final verification

**Files:** None modified

- [ ] **Step 1: Full compilation check**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

Expected: Clean compilation.

- [ ] **Step 2: Run all tests**

```bash
cd /home/jason/jdk/aegis && go test ./internal/... -count=1 -short 2>&1 | tail -30
```

Expected: All remaining tests pass. Discovery and circuit breaker tests should be gone (deleted with their directories). Auth tests should still pass.

- [ ] **Step 3: Check for any remaining references to pruned code**

```bash
cd /home/jason/jdk/aegis && grep -r "ServiceProxy\|ServiceDiscoverer\|CircuitBreaker\|SchemaRegistry\|RoutingTable\|ServiceRegistration" --include="*.go" internal/ 2>/dev/null | grep -v "_test.go" | head -20
```

Expected: No matches in non-test Go files. If any remain, they need to be cleaned up.

- [ ] **Step 4: Check for unused imports or dead code**

```bash
cd /home/jason/jdk/aegis && go vet ./...
```

Expected: No warnings.

- [ ] **Step 5: Verify the file tree looks clean**

```bash
cd /home/jason/jdk/aegis && find internal/ -name "*.go" | sort
```

Review: No gateway-related files should remain. Auth-related files should all be present.

- [ ] **Step 6: Commit any final fixes if needed**

Only if Steps 3-4 revealed issues:

```bash
cd /home/jason/jdk/aegis
git add -A
git commit -m "fix: clean up remaining gateway references"
```
