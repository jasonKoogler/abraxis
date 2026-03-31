# Aegis + Prism Service Separation Design

**Date:** 2026-03-28
**Status:** Approved
**Scope:** Separate Aegis (auth service) and Prism (API gateway) into independent, interlocking services with clear domain boundaries and well-defined communication contracts.

---

## 1. Overview

Aegis and Prism currently share significant code — both contain full auth stacks and full gateway stacks, forked from the same original codebase. This design defines how to cleanly separate them so each service owns its domain exclusively, while interlocking through typed gRPC contracts and a shared authorization library (`github.com/jasonKoogler/authz`).

### Guiding Principles

- **Single owner per domain** — no duplicated business logic across services
- **Local-first evaluation** — Prism validates tokens and evaluates policies without per-request network calls to Aegis
- **Explicit contracts** — gRPC defines the service boundary, not shared Go types
- **Independent deployability** — each service has its own database, its own release cycle, its own failure domain
- **Code-first API specs** — swaggo annotations generate OpenAPI specs (replaces oapi-codegen pipeline)

---

## 2. Service Boundaries

### Aegis (Auth Service) Owns

| Domain | Responsibilities |
|--------|-----------------|
| **Users** | User CRUD, registration, profile management |
| **Authentication** | JWT issuance (Ed25519 signing), token refresh, OAuth2 flows (Google, Facebook, Twitter), session management |
| **Authorization data** | Roles, permissions, role-permission assignments, OPA policy authoring and storage |
| **Tenants** | Tenant CRUD, tenant isolation logic |
| **API Keys** | Key generation, hashing (SHA-256), validation, management |
| **Audit (auth)** | Login/logout, password changes, role assignments, permission changes, user creation/deletion, OAuth linkage, token revocations |
| **Auth rate limiting** | Login attempt throttling, password reset abuse prevention, OAuth callback flooding |
| **Crypto** | Password hashing (Argon2id), JWE encryption (AES-256-GCM), Ed25519 key management |
| **Policy distribution** | Push policy updates to subscribers (Prism) via gRPC stream |
| **JWKS** | Expose `/.well-known/jwks.json` for public key distribution |

### Prism (API Gateway) Owns

| Domain | Responsibilities |
|--------|-----------------|
| **Routing** | ServiceProxy, RoutingTable, route management, pattern-based request routing |
| **Service discovery** | Consul, etcd, Kubernetes, local providers — discovering and watching backend services |
| **Circuit breaker** | Memory + Redis implementations, fault tolerance for backend calls |
| **Gateway rate limiting** | IP/route-based throttling, burst control |
| **Schema registry** | Proto management, gRPC service contracts, dynamic client generation |
| **Service management** | Service registration, deregistration, health monitoring |
| **Audit (gateway)** | Request routing decisions, rate limit hits, circuit breaker trips, service registration events, policy evaluation failures |
| **Lightweight auth** | JWT validation (public key, signature + claims only), revocation list check (Redis), local OPA policy evaluation via `authz`, cached roles/permissions |

### Prism Does NOT Own

- User management, OAuth flows, JWT issuance, session storage, password hashing, tenant management, API key management, role/permission CRUD. These are Aegis concerns accessed through Aegis's API.

### Tenant Awareness in Prism

Prism reads `tenant_id` from validated JWT claims for routing decisions and rate limiting scoping. It never manages tenants — Aegis is the source of truth.

---

## 3. Communication Architecture

### 3.1 Request Flow

```
Client Request
    |
    v
+----------+
|  PRISM   |  1. JWT validation (local — Ed25519 public key from JWKS)
| Gateway  |  2. Revocation check (Redis set lookup by jti)
|          |  3. Policy evaluation (local OPA via authz, cached policies)
|          |  4. Gateway rate limit check
|          |  5. Circuit breaker check
|          |  6. Route to backend service
+----+-----+
     |
     +---> Backend Service A
     +---> Backend Service B
     +---> Aegis (for auth endpoints: login, register, OAuth, etc.)
```

### 3.2 REST API — Client-Facing (Proxied Through Prism)

Prism routes these to Aegis like any other backend service:

- `POST /auth/login` — password login
- `POST /auth/register` — user registration
- `POST /auth/refresh` — token refresh
- `POST /auth/logout` — logout (triggers revocation)
- `GET /auth/oauth/{provider}` — OAuth initiation
- `GET /auth/oauth/{provider}/callback` — OAuth callback
- `GET/PUT /users/{id}` — user profile
- CRUD for roles, permissions, tenants, API keys

All documented via swaggo annotations in Aegis.

### 3.3 gRPC — Internal Service-to-Service (Hot Path)

```protobuf
service AegisAuth {
    // Full sync on startup or reconnection
    rpc GetAllAuthData(GetAllRequest) returns (AuthDataSnapshot);
    rpc GetAllPolicies(GetAllRequest) returns (PolicySnapshot);

    // Streaming updates for incremental sync
    rpc WatchAuthData(WatchRequest) returns (stream AuthDataUpdate);
    rpc WatchPolicies(WatchRequest) returns (stream PolicyUpdate);

    // Fallback for cache misses
    rpc CheckPermission(PermissionRequest) returns (PermissionResponse);

    // API key validation (not cached — infrequent calls)
    rpc ValidateAPIKey(ValidateAPIKeyRequest) returns (ValidateAPIKeyResponse);

    // Token validation fallback (only if local validation fails due to key issues)
    rpc ValidateToken(ValidateTokenRequest) returns (TokenClaims);
}
```

Note: Proto message types (`AuthDataSnapshot`, `PolicySnapshot`, `AuthDataUpdate`, `PolicyUpdate`, etc.) will be fully defined during implementation. The service definition above establishes the RPC contract shape; message fields will be designed when building the gRPC layer.

### 3.4 Cache Sync Flow

```
Aegis                              Prism
  |                                  |
  |<--- gRPC GetAllAuthData ---------|  (full sync on startup/reconnect)
  |---- AuthDataSnapshot ----------->|---> populate Redis cache
  |                                  |
  |<--- gRPC WatchAuthData ----------|  (subscribe to stream)
  |                                  |
  |---- stream: roles updated ------>|---> update Redis cache
  |---- stream: perms updated ----->|---> update Redis cache
  |---- stream: user revoked ------>|---> add to revocation Redis set
  |                                  |
  |<--- gRPC GetAllPolicies ---------|  (full sync on startup/reconnect)
  |---- PolicySnapshot ------------->|---> load into local OPA via authz.UpdatePolicies()
  |                                  |
  |<--- gRPC WatchPolicies ----------|  (subscribe to stream)
  |                                  |
  |---- stream: policy updated ----->|---> update local OPA via authz.UpdatePolicies()
  |                                  |
```

---

## 4. JWT Strategy

### Signing

- **Algorithm:** Ed25519 (EdDSA) — fast, small keys, modern
- **Key management:** Aegis holds the private key, never exposed
- **Public key distribution:** `GET /.well-known/jwks.json` endpoint on Aegis
- **Key rotation:** Overlapping keys — publish new key before signing with it, retire old key after all tokens signed with it expire. Keys identified by `kid` in JWT header.

### Token Lifetimes

- **Access tokens:** 5-10 minute expiry, self-contained (claims include user ID, tenant ID, roles)
- **Refresh tokens:** Longer-lived, stored server-side in Aegis's database, validated on use

### Prism's JWKS Handling

- Fetch on startup, periodic refresh (every 5 minutes or configurable)
- Cache multiple keys, match by `kid`
- Readiness probe fails until JWKS is loaded — load balancer won't send traffic to unready instances
- Liveness probe stays healthy (instance isn't broken, just not ready)

---

## 5. Revocation

### Primary Mechanism

Short-lived access tokens (5-10 min). A revoked user's token naturally expires within minutes.

### Immediate Revocation

- Aegis pushes revocation events on the `WatchAuthData` gRPC stream
- Prism writes revoked token `jti` values to a Redis set with TTL matching the token's remaining lifetime
- On every request, after JWT signature validation, Prism checks the revocation set

### Revocation Triggers

- Logout
- Password change
- Role change
- Account lock/delete
- Admin action

### Safety Bound

Max revocation set size of 100k entries. If exceeded, trigger a full cache flush — safer to re-validate everyone than to miss a revocation.

---

## 6. Bootstrap & Resilience

### Prism Startup Sequence

1. **Fetch JWKS** from Aegis (`/.well-known/jwks.json`)
   - If unavailable: retry with exponential backoff, readiness probe fails until loaded
2. **Full sync** via gRPC (`GetAllAuthData`, `GetAllPolicies`)
   - If unavailable: start with empty cache, fall back to per-request gRPC calls
3. **Subscribe to streams** (`WatchAuthData`, `WatchPolicies`)
   - If disconnected: reconnect with exponential backoff, full re-sync on reconnect

### Degradation Modes

| Aegis State | Prism Behavior |
|-------------|---------------|
| Healthy | Local validation + cached data, zero gRPC calls per request |
| Temporarily down | Serves from cache, entries have 60s TTL. Auth'd requests work until cache expires |
| Down for extended period | Cache expires, new users can't auth, existing valid tokens still validate (JWT is self-contained). Revocations stop flowing. |
| Comes back | Auto-reconnect, full re-sync, resume streams |

### Permission Cache TTL

60 seconds. The gRPC stream is the primary sync mechanism. TTL is a safety net in case the stream lags.

---

## 7. Fallback Rules

| Scenario | Action |
|----------|--------|
| Invalid JWT signature | **Reject.** Never fallback. |
| Expired token | **Reject.** Client must refresh. |
| Revoked token (in Redis set) | **Reject.** |
| Valid token, no cached permissions | **Fallback** — gRPC `CheckPermission` to Aegis, then cache result |
| Valid token, cache hit | **Allow/deny locally.** No network call. |
| Policy evaluation error | **Deny.** Fail closed, log the error. |
| API key presented | **gRPC** `ValidateAPIKey` to Aegis. Not cached. |

---

## 8. Shared `authz` Module

The `github.com/jasonKoogler/authz` module (v0.3.0) remains a shared dependency. It is a library, not shared business logic.

### Role in Each Service

- **Aegis:** Uses `authz` for policy evaluation on auth-domain requests (e.g., "can this user assign this role?"). Manages and stores OPA policies. Pushes policy updates to subscribers.
- **Prism:** Uses `authz` for gateway-level policy evaluation on every request. Loads policies from Aegis via gRPC. Updates local OPA via `authz.UpdatePolicies()`.

### Evolution

- Add a `ContextTransformer` for gateway-specific context enrichment (route metadata, service name) for Prism's use
- The existing webhook endpoint in `authz` serves as an alternative policy push mechanism for simpler deployments (no gRPC)
- No structural changes needed to the library

---

## 9. Audit Log Split

### Aegis Audits (Auth Domain)

- Login/logout events
- Password changes
- Role assignments and permission changes
- User creation/deletion
- OAuth provider linkage
- Token revocations
- API key creation/deletion

### Prism Audits (Gateway Domain)

- Request routing decisions
- Rate limit hits
- Circuit breaker trips
- Service registration/deregistration
- Policy evaluation failures

Each service writes to its own database. A unified audit view is a read-side concern (aggregate later), not write-side coupling.

---

## 10. API Spec Strategy

### Code-First with swaggo

Both services use swaggo (`swag init`) to generate OpenAPI specs from annotated Go handlers. This replaces the oapi-codegen spec-first pipeline.

### Flow

1. Write Go handler with swaggo annotations
2. Run `swag init` to generate OpenAPI spec
3. Optionally run `openapi-ts` to generate TypeScript client SDK

### Inter-Service SDK

Prism doesn't need a generated Go client for Aegis's REST API — it proxies those requests transparently. The gRPC proto definition is the typed contract for service-to-service calls.

---

## 11. Pruning Plan

### 11.1 Remove from Aegis

**Adapters (remove entirely):**
- `internal/adapters/http/proxy.go` — ServiceProxy
- `internal/adapters/discovery/` — all 4 discovery providers + tests
- `internal/adapters/circuitbreaker/` — both implementations + tests

**Services (remove):**
- `internal/service/service_schema_registry.go`
- `internal/service/service_backend.go`
- `internal/service/service_proto.go`

**Domain types (remove):**
- `internal/domain/api_route.go`
- Any ServiceInstance/ServiceRegistry domain types

**Ports (remove):**
- `internal/ports/discovery.go`
- `internal/ports/circuitbreaker.go`
- Service registry/route repository interfaces

**Cleanup:**
- `scripts/openapi-http copy.sh` and `openapi-http copy 2.sh` — duplicate scripts
- `login.ts` at project root — orphaned file
- `policies/gauth_policy.rego` — consolidate to single policy file
- oapi-codegen pipeline files — replaced by swaggo
- Commented-out Makefile sections

### 11.2 Add to Aegis

- gRPC server implementation (`AegisAuth` service)
- Proto definition file for `AegisAuth`
- `/.well-known/jwks.json` endpoint
- Ed25519 key pair generation and management
- Policy management API (CRUD policies, store in database)
- Auth-specific rate limiting (login throttling as domain logic)
- swaggo annotations on all REST handlers
- Revocation event publishing (on the gRPC stream)

### 11.3 Remove from Prism

**Features to slim:**
- `internal/features/auth/` — remove OAuth provider implementations, session storage, JWT issuance, password hashing. Keep JWT validation (signature check via JWKS) and `authz` policy evaluation.
- `internal/features/user/` — remove user repository, user CRUD handlers.

**Domain types (remove or reduce):**
- Full User model — replace with lightweight `AuthenticatedUser` struct from JWT claims
- Password/credential-related types
- OAuth provider types
- Session types

### 11.4 Add to Prism

- gRPC client for Aegis (`WatchAuthData`, `WatchPolicies`, `GetAllAuthData`, `GetAllPolicies`, `CheckPermission`, `ValidateAPIKey`)
- JWKS fetcher with periodic refresh
- Revocation list in Redis (fed by gRPC stream)
- Permission/role cache in Redis (fed by gRPC stream, 60s TTL)
- Readiness probe gated on JWKS loaded
- Reconnection logic with exponential backoff and full re-sync
- swaggo annotations on gateway REST handlers
- Gateway-specific `ContextTransformer` for `authz`

### 11.5 Shared `authz` Module

- Add gateway `ContextTransformer` (route metadata, service name enrichment)
- No structural changes

---

## 12. Database Separation

- **Aegis database:** users, tenants, roles, permissions, role_permissions, api_keys, sessions, audit_logs (auth), policies
- **Prism database:** api_routes, service_registry, gateway_config, audit_logs (gateway)
- Completely separate databases, no shared tables
- Prism caches auth data in Redis (from gRPC stream), never in its own Postgres

---

## 13. Rate Limiting Split

- **Prism:** Gateway-level rate limiting — IP/route-based throttling, burst control. Protects backend services from traffic spikes. Redis-backed for distributed deployment.
- **Aegis:** Auth-specific rate limiting — login attempt throttling, password reset abuse prevention, OAuth callback flooding. Domain logic, not gateway logic. Can be memory or Redis-backed.

---

## 14. Circuit Breaker

Lives in Prism only. Protects against failing backend services (including Aegis). Aegis is just another backend from the circuit breaker's perspective. Memory + Redis implementations retained in Prism.

---

## 15. Service Discovery

Lives in Prism only. Aegis registers itself via standard mechanisms (Kubernetes service, Consul agent sidecar) and is discovered by Prism. Aegis does not contain discovery code.
