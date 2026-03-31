# Phase 3: gRPC Contract Design — Aegis ↔ Prism

**Date:** 2026-03-28
**Status:** Approved
**Scope:** Define the gRPC service contract between Aegis (auth) and Prism (gateway), implement the server in Aegis, implement the client + cache sync in Prism.

---

## 1. Overview

Aegis and Prism communicate over gRPC for internal service-to-service operations. The contract enables:
- Real-time sync of auth data (roles, permissions, token revocations) from Aegis to Prism
- Real-time sync of OPA policies from Aegis to Prism
- On-demand permission checks for cache misses
- On-demand API key validation

Prism operates local-first: JWT validation, revocation checks, and policy evaluation all happen locally using cached data. gRPC calls only occur during sync and for cache-miss fallbacks.

---

## 2. Proto Definition

**Location:** `github.com/jasonKoogler/aegis/api/grpc/aegispb/`

Prism imports the generated Go code as a module dependency: `go get github.com/jasonKoogler/aegis/api/grpc/aegispb`.

### Service

```protobuf
syntax = "proto3";
package aegispb;
option go_package = "github.com/jasonKoogler/aegis/api/grpc/aegispb";

service AegisAuth {
    // Streaming sync — sends full snapshot then incremental updates
    rpc SyncAuthData(SyncRequest) returns (stream AuthDataEvent);
    rpc SyncPolicies(SyncRequest) returns (stream PolicyEvent);

    // On-demand fallback
    rpc CheckPermission(CheckPermissionRequest) returns (CheckPermissionResponse);
    rpc ValidateAPIKey(ValidateAPIKeyRequest) returns (ValidateAPIKeyResponse);
}
```

### Messages

```protobuf
// --- Sync ---

message SyncRequest {
    string last_version = 1; // empty = full sync from scratch
}

message AuthDataEvent {
    string version = 1; // monotonic version for cursor tracking
    oneof event {
        UserRolesSnapshot user_roles = 2;
        TokenRevoked token_revoked = 3;
        SyncComplete sync_complete = 4;
    }
}

message UserRolesSnapshot {
    string user_id = 1;
    string tenant_id = 2;
    repeated string roles = 3;
    repeated string permissions = 4;
}

message TokenRevoked {
    string jti = 1;        // token ID to revoke
    int64 expires_at = 2;  // unix timestamp — TTL for revocation entry
}

message SyncComplete {} // marker: initial snapshot done, now streaming incremental

message PolicyEvent {
    string version = 1;
    map<string, string> policies = 2; // name → rego content (full set every time)
}

// --- CheckPermission ---

message CheckPermissionRequest {
    string user_id = 1;
    string action = 2;
    string resource_type = 3;
    string resource_id = 4;
    string tenant_id = 5;
}

message CheckPermissionResponse {
    bool allowed = 1;
    string reason = 2;
}

// --- ValidateAPIKey ---

message ValidateAPIKeyRequest {
    string api_key = 1;
}

message ValidateAPIKeyResponse {
    bool valid = 1;
    string owner_id = 2;
    string tenant_id = 3;
    repeated string scopes = 4;
}
```

---

## 3. Sync Protocol

### SyncAuthData

1. Client sends `SyncRequest` with `last_version` (empty on first connect)
2. Server decides: if `last_version` is recent enough, send delta. Otherwise, full snapshot.
3. Server streams `UserRolesSnapshot` events for each user with roles
4. Server sends `SyncComplete` marker when initial snapshot is done
5. Server continues streaming incremental `UserRolesSnapshot` and `TokenRevoked` events as changes occur
6. Each event has a monotonic `version` string — client stores this for reconnection

### SyncPolicies

1. Client sends `SyncRequest` with `last_version`
2. Server sends a `PolicyEvent` with the full policy map
3. Server sends new `PolicyEvent` whenever any policy changes (always full set)

### Reconnection

- Client reconnects with exponential backoff (1s, 2s, 4s, 8s, max 30s)
- On reconnect, client sends `SyncRequest` with last seen `version`
- Server sends delta if possible, full snapshot otherwise
- Client processes events identically in both cases (snapshot = overwrite, revocation = add to set)

---

## 4. Aegis gRPC Server

### Package Structure

```
aegis/internal/grpc/
    server.go          — gRPC server setup, listen on :9090
    sync_auth.go       — SyncAuthData implementation
    sync_policies.go   — SyncPolicies implementation
    permissions.go     — CheckPermission implementation
    apikey.go          — ValidateAPIKey implementation
    eventbus.go        — internal fan-out event bus
```

### Event Bus

Channel-based pub/sub for notifying gRPC stream handlers when data changes:

```go
type EventBus struct {
    mu          sync.RWMutex
    subscribers map[string]chan AuthDataEvent
}
```

Each `SyncAuthData` stream registers a subscriber, receives events via channel, unregisters on disconnect. Fan-out to all connected Prism instances.

### Startup

In `cmd/main.go`, start gRPC server alongside HTTP server on a separate port (configurable, default `:9090`). Both share the same services and repositories.

### Service Integration

- `SyncAuthData` queries all user roles from the database for initial snapshot. Publishes events from auth services when roles/permissions change or tokens are revoked.
- `SyncPolicies` loads all OPA policies from the policy store. Watches for policy changes.
- `CheckPermission` delegates to the existing `authz` adapter with the request data.
- `ValidateAPIKey` delegates to the existing `APIKeyService`.

---

## 5. Prism gRPC Client

### Package Structure

```
prism/internal/features/gateway/adapters/aegis/
    client.go          — connection management, reconnection, CheckPermission, ValidateAPIKey
    auth_sync.go       — SyncAuthData processing, Redis cache writes
    policy_sync.go     — SyncPolicies processing, authz.UpdatePolicies()
```

### Redis Cache Layout

| Key Pattern | Value | TTL |
|-------------|-------|-----|
| `aegis:roles:{user_id}` | JSON `{"roles":["admin"],"permissions":["read","write"]}` | 60s |
| `aegis:revoked:{jti}` | `""` (empty, existence = revoked) | token's remaining lifetime |
| `aegis:sync:version` | last seen version string | none |

### Startup Integration

1. Create Aegis gRPC client in `app.go`
2. Start `SyncAuthData` and `SyncPolicies` as background goroutines
3. Gate readiness probe on `SyncComplete` received from both streams
4. On disconnect, reconnect with exponential backoff (1s → 30s max)
5. On reconnect, send `SyncRequest` with last version from `aegis:sync:version`

---

## 6. Middleware Integration

After this phase, Prism's request auth flow becomes:

```
Request arrives
    │
    ├─ 1. Extract JWT from Authorization header (existing)
    ├─ 2. Validate JWT signature locally (existing TokenValidator)
    ├─ 3. Check revocation: Redis GET aegis:revoked:{jti}
    │     └─ If found → 401 Unauthorized
    ├─ 4. Look up cached roles: Redis GET aegis:roles:{user_id}
    │     ├─ Cache hit → enrich context with roles/permissions
    │     └─ Cache miss → gRPC CheckPermission fallback, cache result
    ├─ 5. Policy evaluation via authz (existing, uses cached/enriched data)
    │
    └─ Route to backend service
```

Steps 1-2 are in `auth.AuthMiddleware.Authenticate`.
Step 3 is new — added to the auth middleware after JWT validation.
Steps 4-5 are in the authz/policy evaluation layer.

---

## 7. Configuration

### Aegis Config Additions

```
GRPC_PORT=9090
GRPC_ENABLED=true
```

### Prism Config Additions

```
AEGIS_GRPC_ADDRESS=localhost:9090
AEGIS_SYNC_ENABLED=true
AEGIS_CACHE_TTL=60s
AEGIS_RECONNECT_MAX_BACKOFF=30s
```

---

## 8. Port & Network

- Aegis HTTP: `:8080` (client-facing, proxied through Prism)
- Aegis gRPC: `:9090` (internal, Prism-only)
- gRPC port should be firewalled to internal network only in production
- Both servers start from the same `cmd/main.go`, share services

---

## 9. Testing Strategy

- **Aegis gRPC server:** Integration tests using real gRPC client against test server. Test full sync, incremental events, reconnection with cursor.
- **Prism gRPC client:** Integration tests against a real Aegis gRPC server (dockertest or test process). Verify Redis cache population, revocation checks, policy sync.
- **Event bus:** Unit tests for fan-out, subscriber registration/deregistration.
- **Middleware:** Integration test for the full auth flow: JWT validation → revocation check → cache lookup → policy evaluation.
