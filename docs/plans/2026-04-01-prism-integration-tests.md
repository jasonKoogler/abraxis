# Prism Integration Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integration tests for Prism's API key lifecycle, audit log queries, and gateway route management against real Postgres + Redis.

**Architecture:** Same pattern as Aegis tests. `StartPrismServer` constructs a real `app.App` with all dependencies on random port. Tests send real HTTP requests. Audit logs are seeded directly in the DB. No mocks.

**Tech Stack:** dockertest/v3, testify, pgxpool, go-redis, net/http

**Spec:** `docs/specs/2026-04-01-integration-tests-design.md` (Sub-project 2)

---

### Task 1: Create Prism test helper and API key integration tests

**Files:**
- Create: `/home/jason/jdk/abraxis/prism/tests/helper_test.go`
- Create: `/home/jason/jdk/abraxis/prism/tests/apikey_integration_test.go`
- Modify: `/home/jason/jdk/abraxis/prism/go.mod` (add dockertest)

This task creates the Prism test server helper AND the API key tests, since they're tightly coupled (the helper needs tests to validate it works).

- [ ] **Step 1: Add dockertest dependency**

```bash
cd /home/jason/jdk/abraxis/prism
go get github.com/ory/dockertest/v3@v3.11.0
```

- [ ] **Step 2: Create helper_test.go**

Create `/home/jason/jdk/abraxis/prism/tests/helper_test.go` (package `tests`):

Provides `StartPrismServer(t, pg, rd) *PrismTestServer` that:
1. Builds a `config.Config` programmatically (no env vars)
2. Creates Postgres pool, Redis client, repositories (audit, apikey, tenant)
3. Creates `app.NewApp` with `WithAllDefaultServices`
4. Starts in background goroutine on a random port
5. Waits for `/health` to respond
6. Returns `BaseURL` and Postgres pool (for seeding data)

Key config differences from Aegis:
- No OAuth needed
- No gRPC server
- Prism needs Aegis connection config but since we're testing standalone, disable JWKS/sync:
  - Don't set `AEGIS_JWKS_URL` or JWKS fetcher
  - Don't set `AEGIS_SYNC_ENABLED`
- The service proxy is nil (no backend services to proxy to) — this is fine, gateway management endpoints still work

Read these files to understand exact types and construct the config correctly:
- `prism/internal/config/config.go` for Config struct
- `prism/cmd/main.go` for the construction pattern
- `prism/internal/app/options.go` for available options
- `prism/internal/app/app.go` for the Start() method to understand what it needs

IMPORTANT: Prism's `app.go` has the same potential issues Aegis had (CORS middleware returning nil, logger compatibility). Apply the same fixes if needed:
- Check `createCorsMiddleware` — return pass-through if no origins configured
- Check logger middleware usage

- [ ] **Step 3: Create apikey_integration_test.go**

Create `/home/jason/jdk/abraxis/prism/tests/apikey_integration_test.go` (package `tests`):

```go
func TestApiKeyLifecycle(t *testing.T) {
    pg := testutil.SetupPostgres(t, "../deploy/migrations")
    rd := testutil.SetupRedis(t)
    server := StartPrismServer(t, pg, rd)

    var createdKeyID, rawApiKey string

    t.Run("create_api_key", func(t *testing.T) {
        // POST /apikey with name, scopes, expires_in_days
        // Expect 201
        // Save the returned key ID and raw API key (only shown once)
    })

    t.Run("get_api_key", func(t *testing.T) {
        // GET /apikey/{createdKeyID}
        // Expect 200, verify name matches
    })

    t.Run("list_api_keys", func(t *testing.T) {
        // GET /apikey
        // Expect 200, verify list contains the created key
    })

    t.Run("validate_api_key", func(t *testing.T) {
        // POST /apikey/validate with raw API key
        // Expect 200, verify valid response
    })

    t.Run("update_api_key_metadata", func(t *testing.T) {
        // PUT /apikey/{createdKeyID}/metadata with new name
        // Expect 200, verify name changed
    })

    t.Run("revoke_api_key", func(t *testing.T) {
        // DELETE /apikey/{createdKeyID}
        // Expect 204
    })

    t.Run("validate_revoked_key_fails", func(t *testing.T) {
        // POST /apikey/validate with the same raw API key
        // Expect error (key is revoked)
    })
}
```

Read the actual handler code to understand:
- Request/response formats for each endpoint
- The `api_key` field name in validate request body
- How the raw API key is returned on creation (response body structure)
- What the key ID format looks like (prefixed IDs like `apk_xxx`?)

Use `github.com/stretchr/testify/require` for assertions.

- [ ] **Step 4: Run the tests**

```bash
cd /home/jason/jdk/abraxis/prism
go test -v -count=1 -timeout 180s ./tests/
```

Iterate until all subtests pass. Fix any Prism bugs found along the way (same pattern as Aegis — nil CORS middleware, logger issues, etc.)

- [ ] **Step 5: Commit**

```bash
cd /home/jason/jdk/abraxis
git add prism/tests/ prism/go.mod prism/go.sum
git commit -m "feat: add prism API key lifecycle integration tests"
```

---

### Task 2: Add audit log query integration tests

**Files:**
- Create: `/home/jason/jdk/abraxis/prism/tests/audit_integration_test.go`

- [ ] **Step 1: Create audit_integration_test.go**

Create `/home/jason/jdk/abraxis/prism/tests/audit_integration_test.go` (package `tests`):

```go
func TestAuditLogQueries(t *testing.T) {
    pg := testutil.SetupPostgres(t, "../deploy/migrations")
    rd := testutil.SetupRedis(t)
    server := StartPrismServer(t, pg, rd)

    // Seed audit log data directly in Postgres
    seedAuditLogs(t, pg.Pool)

    t.Run("list_audit_logs", func(t *testing.T) {
        // GET /audit
        // Expect 200, verify returns seeded logs
    })

    t.Run("list_with_filters", func(t *testing.T) {
        // GET /audit?event_type=login
        // Expect 200, verify only matching logs returned
    })

    t.Run("get_audit_log_by_id", func(t *testing.T) {
        // GET /audit/{auditID}
        // Expect 200, verify specific log entry
    })

    t.Run("aggregate_by_event_type", func(t *testing.T) {
        // GET /audit/aggregate/event_type
        // Expect 200, verify aggregation counts
    })

    t.Run("export_csv", func(t *testing.T) {
        // GET /audit/export?tenant_id=xxx
        // Expect 200, Content-Type: text/csv
        // Verify CSV has header row and data rows
    })
}
```

The `seedAuditLogs` helper inserts test data directly using `pg.Pool`:
```go
func seedAuditLogs(t *testing.T, pool *pgxpool.Pool) {
    // Insert 5-10 audit log entries with varied event_type, actor_type, tenant_id
    // Use direct SQL INSERT statements
}
```

Read `prism/internal/domain/audit_log.go` for the `AuditLog` struct and field names to match the DB columns.

Read the audit handler code (`prism/internal/features/audit/handler.go`) to understand:
- How list endpoint handles pagination and filters
- What format aggregation returns
- How export generates CSV

- [ ] **Step 2: Run the tests**

```bash
cd /home/jason/jdk/abraxis/prism
go test -v -count=1 -timeout 180s ./tests/
```

All audit subtests should pass alongside the API key tests from Task 1.

- [ ] **Step 3: Commit**

```bash
cd /home/jason/jdk/abraxis
git add prism/tests/audit_integration_test.go
git commit -m "feat: add prism audit log query integration tests"
```

- [ ] **Step 4: Push**

```bash
git push origin main
```
