# Integration Tests Design Spec

**Date:** 2026-04-01
**Scope:** Three sub-projects for comprehensive integration testing across Abraxis

## Overview

Three sub-projects, each with its own plan and implementation cycle:

1. **Sub-project 1: Aegis standalone** — auth flows against real Postgres + Redis
2. **Sub-project 2: Prism standalone** — gateway/audit/apikey flows against real Postgres + Redis
3. **Sub-project 3: Cross-service** — JWT round-trip, gRPC sync, revocation (Aegis ↔ Prism)

No mocks, fakes, or stubs. Real databases via dockertest. Real service instances.

## Shared Test Infrastructure

### Module: `tests/testutil/` (`github.com/jasonKoogler/abraxis/tests`)

A separate Go module at the monorepo root containing container setup helpers. Added to `go.work`. Does NOT import any service-specific code.

**`postgres.go`** — `SetupPostgres(t, migrationsPath) (*pgxpool.Pool, func())`
- Spins up `postgres:16-alpine` via dockertest
- Runs migrations from the provided path
- Returns connection pool + cleanup function
- Assigns random port to avoid conflicts with running Docker Compose

**`redis.go`** — `SetupRedis(t) (*redis.Client, func())`
- Spins up `redis:7-alpine` via dockertest
- Returns client + cleanup function

Both helpers skip tests when Docker is unavailable (`t.Skip("docker not available")`).

### Test File Locations

- Aegis tests: `aegis/tests/` (inside aegis module — can import `internal/`)
- Prism tests: `prism/tests/` (inside prism module — can import `internal/`)
- Cross-service tests: `tests/integration/` (in tests module — starts services as binaries)

---

## Sub-project 1: Aegis Standalone Integration Tests

### File: `aegis/tests/auth_integration_test.go`

Tests run against a real Aegis server (HTTP) backed by real Postgres + Redis. The test creates a real `app.App` with all real dependencies, starts it on a random port, and sends HTTP requests.

### Test Cases

```
TestAuthFlows(t *testing.T)
├── register_user
│   POST /auth/register → 201
│   Verify user exists in Postgres
│   Verify auth headers set (Authorization, X-Refresh-Token, X-Session-ID)
│
├── login_with_password
│   POST /auth/login → 200
│   Verify token pair returned in headers
│   Verify access token is valid JWT with correct claims (issuer, subject, roles)
│
├── access_protected_endpoint
│   GET /users/{userID} with Bearer token → 200
│   Verify user data returned matches registered user
│   GET /users/{userID} without token → 401
│
├── refresh_token
│   POST /auth/refresh with refresh token → 200
│   Verify new token pair returned
│   Verify new access token works on protected endpoint
│
└── logout_invalidates_session
    POST /auth/logout with token → 302
    Verify old token no longer works on protected endpoint → 401
```

### Setup/Teardown

```go
func TestAuthFlows(t *testing.T) {
    // 1. Start Postgres via dockertest, run aegis migrations
    pool, cleanupPG := testutil.SetupPostgres(t, "../deploy/migrations")
    defer cleanupPG()

    // 2. Start Redis via dockertest
    redisClient, cleanupRedis := testutil.SetupRedis(t)
    defer cleanupRedis()

    // 3. Create real Aegis app with all dependencies
    // Uses random port, real config, real services
    cfg := testConfig(pgHost, pgPort, redisHost, redisPort)
    app, _ := app.NewApp(opts...)
    go app.Start()
    defer app.Shutdown()

    // 4. Wait for /health to respond
    // 5. Run subtests
}
```

---

## Sub-project 2: Prism Standalone Integration Tests (future)

### File: `prism/tests/gateway_integration_test.go`

Same pattern as Aegis — real Prism server, real Postgres + Redis. Tests audit log CRUD, API key lifecycle, and gateway route management.

### Test Cases (outline)

- Create API key → validate it → revoke it → validation fails
- Create audit log → list with filters → aggregate → export CSV
- Register service route → list routes → update → delete

---

## Sub-project 3: Cross-Service Integration Tests (future)

### File: `tests/integration/cross_service_test.go`

Starts both Aegis and Prism (as built binaries or in-process), connects them via gRPC + JWKS.

### Test Cases (outline)

- Register user in Aegis → get JWT → call Prism with JWT → Prism validates via JWKS → 200
- Register user in Aegis → get JWT → revoke token in Aegis → Prism receives revocation event → rejects token → 401
- Aegis gRPC sync → Prism receives role/permission updates → authz decisions change

---

## Testing Principles

- **No mocks.** Real Postgres, real Redis, real HTTP servers.
- **dockertest** for container lifecycle. Tests skip when Docker unavailable.
- **Random ports** for all services — no conflicts with running Docker Compose.
- **Cleanup** via `defer` — containers removed after test completes.
- **`testify/require`** for assertions — fail fast on unexpected results.

## What this does NOT cover

- OAuth flows (require real OAuth providers)
- Frontend/E2E tests
- Load/performance testing
