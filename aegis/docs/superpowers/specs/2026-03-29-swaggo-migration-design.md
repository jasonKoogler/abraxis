# Swaggo Migration Design Spec

**Date:** 2026-03-29
**Scope:** Aegis + Prism — replace oapi-codegen (spec-first) with swaggo (code-first)

## Motivation

The current oapi-codegen pipeline is cumbersome: hand-write OpenAPI YAML specs → generate server interfaces + types → implement handlers against generated interfaces. swaggo inverts this — annotate handlers directly, generate the spec from code. Single source of truth, less friction, code-first workflow.

## Architecture Change

### Before (oapi-codegen)

```
Hand-written OpenAPI YAML → oapi-codegen → ServerInterface + types
Handlers implement ServerInterface → HandlerFromMux() registers routes
```

### After (swaggo)

```
Handlers with swaggo annotations → swag init → docs/swagger.json
Handlers register routes directly on chi.Router
httpSwagger serves Swagger UI at /swagger/*
```

## Aegis Changes

### Handler & Route Structure

- **Remove** `ServerInterface` implementation constraint from `internal/adapters/http/server.go`
- **Add** swaggo annotations (`@Summary`, `@Router`, `@Param`, `@Success`, `@Failure`, `@Security`) to each of the 9 handler methods
- **Add** `RegisterRoutes(r chi.Router)` method to `Server` that wires routes directly on chi
- **Replace** `ConditionalAuthMiddleware` wrapping the generated handler with chi route groups — public routes (login, register, refresh, social login, callback) in one group, protected routes (logout, user CRUD) in another with auth middleware
- **Update** `internal/app/app.go` to call `server.RegisterRoutes(router)` instead of `ports.HandlerFromMux()`

### DTO & Type Strategy

- **Delete** `internal/ports/openapi-server.gen.go` and `internal/ports/openapi-types.gen.go`
- **Annotate domain models directly** (1:1 matches): `domain.User`, `domain.UserStatus`, `domain.TokenPair` — add swaggo `example` tags
- **Create** `internal/adapters/http/types.go` with hand-written DTOs for request/response-only types:
  - `PasswordLoginRequest`
  - `UserRegistrationRequest`
  - `UpdateUserRequest`
  - `ErrorResponse`
  - Query parameter types (pagination, etc.)
- **Keep** existing converter functions in `user_types.go` (`RegistrationRequestToUserParams`, `UpdateUserRequestToParams`, etc.)

### Endpoints (9 total)

| Method | Path | Auth | Handler |
|--------|------|------|---------|
| POST | /auth/login | Public | loginUserWithPassword |
| POST | /auth/register | Public | registerUser |
| POST | /auth/refresh | Public | refreshToken |
| POST | /auth/logout | Protected | logoutUser |
| GET | /auth/{provider} | Public | initiateSocialLogin |
| GET | /auth/{provider}/callback | Public | providerCallback |
| GET | /users | Protected | listUsers |
| GET | /users/{userID} | Protected | getUser |
| POST | /users/{userID} | Protected | updateUser |

## Prism Changes

### Handler & Route Structure

- **Remove** generated `gen/` directories from each feature (audit, apikey, gateway, discovery)
- **Add** swaggo annotations to each feature's handler methods
- **Add** `RegisterRoutes(r chi.Router)` to each feature's handler/server struct
- **Update** `internal/app/routes.go` to call each feature's `RegisterRoutes()` instead of `gen.HandlerFromMux()`

### DTO & Type Strategy

- **Delete** all `internal/features/*/gen/` directories
- **Delete** `internal/api/shared/models.gen.go` and `internal/api/client/generated.go`
- **Keep** existing DTOs in `internal/features/*/dto/` packages — they handle real type mismatches (custom `id.ID` types, `net.IP` vs `*string`, etc.)
- **Add** swaggo tags to existing DTO structs
- **Create** hand-written replacements for any generated types that don't already exist in `dto/` packages
- **Annotate domain models directly** (1:1 matches): `domain.AuditLogAggregate`, `domain.AuditLogAggregateResponse`
- **Keep** all existing converter functions in `dto/converter.go` files

### Verification Needed During Implementation

- **Prism auth routes** (`/auth/*`): Verify whether these are proxy/pass-through endpoints to Aegis or dead code from the separation. If dead, remove rather than annotate.
- **`internal/api/client/generated.go`**: Check if anything imports this. Likely dead post-separation (Prism uses gRPC to talk to Aegis). Delete if unused.
- **`internal/api/shared/models.gen.go`**: Determine where cross-feature shared types should live — either `internal/api/shared/types.go` (hand-written) or pushed into the owning feature's `dto/` package.

## Swagger Setup (Both Services)

### Dependencies

```
github.com/swaggo/swag v1.16.6          # Annotation parser + CLI
github.com/swaggo/http-swagger/v2 v2.0.2  # chi-compatible Swagger UI handler
```

### Main Annotation Block (`cmd/main.go`)

Each service gets a swaggo metadata block:

```go
// @title           Aegis Auth Service API (or Prism API Gateway)
// @version         1.0
// @description     ...
// @host            localhost:8080
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
```

### Generated Output

`swag init -g cmd/main.go --parseDependency --parseInternal` produces:
- `docs/docs.go` — Go package with embedded spec
- `docs/swagger.json` — OpenAPI 2.0 spec
- `docs/swagger.yaml` — YAML variant

These are committed to the repo.

### Docs Import (`cmd/main.go`)

Each service must blank-import its generated `docs` package to register the embedded spec:

```go
_ "github.com/jasonKoogler/aegis/docs"  // Aegis
_ "github.com/jasonKoogler/prism/docs"  // Prism
```

### Swagger UI Route

```go
import httpSwagger "github.com/swaggo/http-swagger/v2"

r.Get("/swagger/*", httpSwagger.WrapHandler)
```

Replaces existing static doc serving.

### Note on Spec Version

swaggo v1 generates Swagger 2.0 (not OpenAPI 3.0). This is fine for standard REST endpoints. If OpenAPI 3.0 features (`oneOf`, `anyOf`, `nullable`) are needed later, swaggo v2 can be adopted when it reaches stable.

## Build Pipeline

### Makefile Targets

**Aegis:**
```makefile
swagger:
	swag init -g cmd/main.go --parseDependency --parseInternal
```

**Prism:**
```makefile
swagger:
	swag init -g cmd/main.go --parseDependency --parseInternal

sdk: swagger
	# Future: openapi-ts generation for TypeScript client
```

### Cleanup

**Aegis — delete:**
- `api/openapi/` (entire directory — hand-written specs)
- `redocly.yaml`
- `internal/ports/openapi-server.gen.go`
- `internal/ports/openapi-types.gen.go`
- `internal/common/client/` (generated client, if unused)

**Prism — delete:**
- `api/openapi/` (entire directory — specs + configs + bundled)
- `scripts/openapi-http.sh`
- `internal/features/*/gen/` (all generated server code)
- `internal/api/shared/models.gen.go`
- `internal/api/client/generated.go`

**Both — remove from go.mod:**
- `github.com/oapi-codegen/runtime`
- Any other oapi-codegen dependencies

## What Stays Untouched

- All business logic, service layer, repositories
- Domain models (except adding swaggo tags to 5 total: 3 Aegis, 2 Prism)
- Middleware logic (restructured but not rewritten)
- gRPC code (proto, server, client)
- JWT/JWKS code
- Database, Redis, config

## Risk Assessment

**Low risk.** The migration is mechanical:
1. Swap route declaration style (generated → manual chi registration)
2. Add comment annotations to existing handlers
3. Replace generated types with hand-written equivalents
4. No behavioral changes

The main risk is missing a type or route during the mechanical translation, caught immediately by compilation errors.
