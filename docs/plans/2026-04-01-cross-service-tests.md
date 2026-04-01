# Cross-Service Integration Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Test the Aegis ↔ Prism integration: JWT round-trip (Aegis signs, Prism validates via JWKS), gRPC sync, and token revocation flow.

**Architecture:** Build both service binaries, start as subprocesses with env vars pointing to dockertest Postgres + Redis containers. Test externally via HTTP. The `tests/integration/` package orchestrates both services.

**Tech Stack:** dockertest/v3, os/exec, net/http, testify

**Spec:** `docs/specs/2026-04-01-integration-tests-design.md` (Sub-project 3)

---

### Task 1: Create cross-service test harness and JWT round-trip test

**Files:**
- Create: `/home/jason/jdk/abraxis/tests/integration/harness_test.go`
- Create: `/home/jason/jdk/abraxis/tests/integration/cross_service_test.go`

This task builds a test harness that starts real Aegis and Prism servers as subprocesses, then tests JWT validation across services.

- [ ] **Step 1: Build both service binaries**

Before writing tests, verify both services build:
```bash
cd /home/jason/jdk/abraxis/aegis && go build -o /tmp/aegis-test ./cmd/
cd /home/jason/jdk/abraxis/prism && go build -o /tmp/prism-test ./cmd/
```

- [ ] **Step 2: Create harness_test.go**

Create `/home/jason/jdk/abraxis/tests/integration/harness_test.go` (package `integration`):

This file provides `StartFullStack(t) *FullStack` that:
1. Starts Postgres via dockertest, creates both `aegis_db` and `prism_db`
2. Runs Aegis migrations against `aegis_db`, Prism migrations against `prism_db`
3. Starts Redis via dockertest
4. Builds Aegis binary (`go build -o <tmpdir>/aegis ./aegis/cmd/`)
5. Starts Aegis as subprocess with env vars: DB, Redis, HTTP port, gRPC port, session manager, JWT config, OAuth config (placeholder Google)
6. Waits for Aegis `/health` to respond
7. Builds Prism binary (`go build -o <tmpdir>/prism ./prism/cmd/`)
8. Starts Prism as subprocess with env vars: DB, Redis, HTTP port, Aegis gRPC address, Aegis JWKS URL, sync enabled
9. Waits for Prism `/health` to respond
10. Returns `*FullStack` with AegisURL, PrismURL, cleanup

The harness should:
- Use `os/exec.Command` to start subprocesses
- Set env vars via `cmd.Env`
- Use random ports for all services (find free ports via `net.Listen("tcp", ":0")`)
- Pipe subprocess stdout/stderr to `t.Log` for debugging
- Kill subprocesses on cleanup via `cmd.Process.Kill()`

Key env vars for Aegis subprocess:
```
ENV=development
LOG_LEVEL=error
POSTGRES_HOST=localhost
POSTGRES_PORT=<dockertest port>
POSTGRES_USER=testuser
POSTGRES_PASSWORD=testpass
POSTGRES_DB=aegis_db
POSTGRES_SSL_MODE=disable
POSTGRES_TIMEZONE=UTC
POSTGRES_TIMEOUT=30s
REDIS_HOST=localhost
REDIS_PORT=<dockertest port>
REDIS_PASSWORD=testredis
REDIS_USERNAME=default
HTTP_SERVER_PORT=<random>
HTTP_SERVER_READ_TIMEOUT=10s
HTTP_SERVER_WRITE_TIMEOUT=10s
HTTP_SERVER_IDLE_TIMEOUT=10s
HTTP_SERVER_SHUTDOWN_TIMEOUT=5s
USE_REDIS_RATE_LIMITER=false
SESSION_MANAGER=redis
USE_CUSTOM_JWT=true
JWT_ISSUER=aegis-test
ACCESS_TOKEN_EXPIRATION=15m
REFRESH_TOKEN_EXPIRATION=24h
TOKEN_ROTATION_INTERVAL=168h
OAUTH_VERIFIER_STORAGE=memory
GOOGLE_KEY=testkey
GOOGLE_SECRET=testsecret
GOOGLE_CALLBACK_URL=http://localhost/callback
GOOGLE_SCOPES=email
GRPC_PORT=<random>
GRPC_ENABLED=true
```

Key env vars for Prism subprocess (same DB/Redis connection, DIFFERENT database name, plus Aegis connection):
```
ENV=development
LOG_LEVEL=error
POSTGRES_HOST=localhost
POSTGRES_PORT=<same dockertest port>
POSTGRES_USER=testuser
POSTGRES_PASSWORD=testpass
POSTGRES_DB=prism_db
POSTGRES_SSL_MODE=disable
POSTGRES_TIMEZONE=UTC
POSTGRES_TIMEOUT=30s
REDIS_HOST=localhost
REDIS_PORT=<same dockertest port>
REDIS_PASSWORD=testredis
REDIS_USERNAME=default
HTTP_SERVER_PORT=<different random>
HTTP_SERVER_READ_TIMEOUT=10s
HTTP_SERVER_WRITE_TIMEOUT=10s
HTTP_SERVER_IDLE_TIMEOUT=10s
HTTP_SERVER_SHUTDOWN_TIMEOUT=5s
USE_REDIS_RATE_LIMITER=false
SESSION_MANAGER=redis
USE_CUSTOM_JWT=true
JWT_ISSUER=aegis-test
ACCESS_TOKEN_EXPIRATION=15m
REFRESH_TOKEN_EXPIRATION=24h
TOKEN_ROTATION_INTERVAL=168h
OAUTH_VERIFIER_STORAGE=memory
GOOGLE_KEY=testkey
GOOGLE_SECRET=testsecret
GOOGLE_CALLBACK_URL=http://localhost/callback
GOOGLE_SCOPES=email
AEGIS_GRPC_ADDRESS=localhost:<aegis grpc port>
AEGIS_SYNC_ENABLED=true
AEGIS_CACHE_TTL=5s
AEGIS_RECONNECT_MAX_BACKOFF=5s
AEGIS_JWKS_URL=http://localhost:<aegis http port>/.well-known/jwks.json
AEGIS_JWKS_REFRESH_INTERVAL=10s
```

For Postgres: use a SINGLE Postgres container but create BOTH databases. The `SetupPostgres` helper from testutil creates one DB. For cross-service tests, you need two DBs in one Postgres instance. Options:
- Call `SetupPostgres` twice (two containers) — simpler but more resources
- Or create a custom setup that creates one container with two DBs — more efficient

Use the simpler approach (two SetupPostgres calls) unless resource constraints matter.

Actually, even simpler: use one Postgres container, run `CREATE DATABASE prism_db` after initial setup, then run Prism migrations against it. The testutil `SetupPostgres` can be called once for aegis_db, then use the returned connection info to create prism_db and run its migrations.

Read `tests/testutil/postgres.go` to understand the current helper and decide the best approach.

- [ ] **Step 3: Create cross_service_test.go**

Create `/home/jason/jdk/abraxis/tests/integration/cross_service_test.go` (package `integration`):

```go
func TestCrossService(t *testing.T) {
    stack := StartFullStack(t)

    var accessToken, refreshToken string

    t.Run("register_in_aegis", func(t *testing.T) {
        // POST aegis/auth/register
        // Save tokens from response headers
    })

    t.Run("jwt_validated_by_prism", func(t *testing.T) {
        // Use the Aegis-issued JWT to call a Prism protected endpoint
        // e.g., GET prism/apikey (requires auth)
        // If Prism validates the JWT via JWKS → 200 (or 200 with empty list)
        // Without token → 401
    })

    t.Run("prism_readiness_with_aegis", func(t *testing.T) {
        // GET prism/ready → 200
        // This proves JWKS is loaded and Aegis sync is established
    })

    t.Run("logout_revokes_across_services", func(t *testing.T) {
        // Login fresh in Aegis to get a clean token
        // POST aegis/auth/login → get new token
        // Verify token works on Prism endpoint → 200
        // POST aegis/auth/logout → session revoked
        // Wait briefly for gRPC revocation propagation
        // Verify same token rejected by Prism → 401
        // NOTE: This depends on the revocation gRPC stream working end-to-end.
        // If it doesn't work yet, note it as DONE_WITH_CONCERNS.
    })
}
```

Use `net/http.Client` for all requests. `github.com/stretchr/testify/require` for assertions.

- [ ] **Step 4: Add testify to tests module**

```bash
cd /home/jason/jdk/abraxis/tests
go get github.com/stretchr/testify@v1.10.0
```

- [ ] **Step 5: Run the tests**

```bash
cd /home/jason/jdk/abraxis/tests
go test -v -count=1 -timeout 300s ./integration/
```

The timeout is longer (300s) because this builds two binaries and starts multiple containers.

Iterate until tests pass. Debug subprocess output if services fail to start.

- [ ] **Step 6: Commit and push**

```bash
cd /home/jason/jdk/abraxis
git add tests/
git commit -m "feat: add cross-service integration tests (JWT round-trip, gRPC sync, revocation)"
git push origin main
```
