# Phase 3A: Aegis gRPC Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a gRPC server to Aegis that exposes auth data sync, permission checking, and API key validation to Prism.

**Architecture:** Aegis serves gRPC on a separate port (`:9090`) alongside its HTTP server (`:8080`). The gRPC service uses streaming RPCs for real-time sync of auth data and policies, plus unary RPCs for on-demand permission checks and API key validation. An internal event bus fans out changes to connected clients.

**Tech Stack:** Go 1.24, google.golang.org/grpc, google.golang.org/protobuf, Redis, PostgreSQL

**Spec:** `docs/superpowers/specs/2026-03-28-phase3-grpc-contract-design.md`

---

## File Structure

### New files in Aegis:

```
api/grpc/aegispb/
    aegis.proto              — service + message definitions
    aegis.pb.go              — generated (protoc)
    aegis_grpc.pb.go         — generated (protoc)

internal/grpc/
    server.go                — gRPC server setup, listen on configurable port
    eventbus.go              — channel-based fan-out event bus
    sync_auth.go             — SyncAuthData RPC implementation
    sync_policies.go         — SyncPolicies RPC implementation
    permissions.go           — CheckPermission RPC implementation
    apikey.go                — ValidateAPIKey RPC implementation

internal/config/config.go   — add GRPCConfig
internal/app/app.go         — add grpcServer field, start alongside HTTP
internal/app/options.go     — add WithGRPCServer option
cmd/main.go                 — create and wire gRPC server
```

---

### Task 1: Create proto definition and generate code

**Files:**
- Create: `api/grpc/aegispb/aegis.proto`
- Modify: `scripts/protogen.sh` (update output path)

- [ ] **Step 1: Create the proto file**

Create `api/grpc/aegispb/aegis.proto`:

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

// --- Sync ---

message SyncRequest {
    string last_version = 1; // empty = full sync from scratch
}

message AuthDataEvent {
    string version = 1;
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
    string jti = 1;
    int64 expires_at = 2;
}

message SyncComplete {}

message PolicyEvent {
    string version = 1;
    map<string, string> policies = 2;
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

- [ ] **Step 2: Update protogen.sh output paths**

Update `scripts/protogen.sh`:

Change line 8:
```bash
# Before:
OUTPUT_DIR="${ROOT_DIR}/internal/ports/proto"
# After:
OUTPUT_DIR="${ROOT_DIR}/api/grpc/aegispb"
```

- [ ] **Step 3: Add gRPC dependencies**

```bash
cd /home/jason/jdk/aegis
go get google.golang.org/grpc@latest
go get google.golang.org/protobuf@latest
go get google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

- [ ] **Step 4: Generate Go code from proto**

```bash
cd /home/jason/jdk/aegis && bash scripts/protogen.sh
```

Verify the generated files exist:
```bash
ls api/grpc/aegispb/aegis.pb.go api/grpc/aegispb/aegis_grpc.pb.go
```

- [ ] **Step 5: Verify compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

- [ ] **Step 6: Commit**

```bash
cd /home/jason/jdk/aegis
git add -A
git commit -m "feat: add aegis gRPC proto definition and generated code

Define AegisAuth service with SyncAuthData, SyncPolicies,
CheckPermission, and ValidateAPIKey RPCs."
```

---

### Task 2: Create the event bus

**Files:**
- Create: `internal/grpc/eventbus.go`

The event bus fans out auth data changes to all connected gRPC stream clients.

- [ ] **Step 1: Create the event bus**

Create `internal/grpc/eventbus.go`:

```go
package grpc

import (
	"sync"
	"sync/atomic"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
)

// AuthEventBus fans out auth data events to all connected stream subscribers.
type AuthEventBus struct {
	mu          sync.RWMutex
	subscribers map[string]chan *pb.AuthDataEvent
	version     atomic.Int64
}

// NewAuthEventBus creates a new event bus.
func NewAuthEventBus() *AuthEventBus {
	return &AuthEventBus{
		subscribers: make(map[string]chan *pb.AuthDataEvent),
	}
}

// Subscribe registers a new subscriber and returns a channel to receive events.
// The caller must call Unsubscribe when done.
func (b *AuthEventBus) Subscribe(id string) <-chan *pb.AuthDataEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan *pb.AuthDataEvent, 64)
	b.subscribers[id] = ch
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *AuthEventBus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, ok := b.subscribers[id]; ok {
		close(ch)
		delete(b.subscribers, id)
	}
}

// Publish sends an event to all subscribers. Non-blocking — drops events
// for slow subscribers rather than blocking the publisher.
func (b *AuthEventBus) Publish(event *pb.AuthDataEvent) {
	ver := b.version.Add(1)
	event.Version = formatVersion(ver)

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Subscriber too slow, drop event.
			// They will re-sync on reconnect.
		}
	}
}

// NextVersion returns the current version string.
func (b *AuthEventBus) NextVersion() string {
	return formatVersion(b.version.Load())
}

func formatVersion(v int64) string {
	return fmt.Sprintf("%d", v)
}

// PolicyEventBus fans out policy change events.
type PolicyEventBus struct {
	mu          sync.RWMutex
	subscribers map[string]chan *pb.PolicyEvent
	version     atomic.Int64
}

// NewPolicyEventBus creates a new policy event bus.
func NewPolicyEventBus() *PolicyEventBus {
	return &PolicyEventBus{
		subscribers: make(map[string]chan *pb.PolicyEvent),
	}
}

// Subscribe registers a new subscriber.
func (b *PolicyEventBus) Subscribe(id string) <-chan *pb.PolicyEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan *pb.PolicyEvent, 16)
	b.subscribers[id] = ch
	return ch
}

// Unsubscribe removes a subscriber.
func (b *PolicyEventBus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, ok := b.subscribers[id]; ok {
		close(ch)
		delete(b.subscribers, id)
	}
}

// Publish sends a policy event to all subscribers.
func (b *PolicyEventBus) Publish(event *pb.PolicyEvent) {
	ver := b.version.Add(1)
	event.Version = formatVersion(ver)

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
```

Note: Add `"fmt"` to the import block — `formatVersion` uses `fmt.Sprintf`.

- [ ] **Step 2: Verify compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

- [ ] **Step 3: Commit**

```bash
cd /home/jason/jdk/aegis
git add internal/grpc/eventbus.go
git commit -m "feat: add auth and policy event buses for gRPC streaming"
```

---

### Task 3: Implement SyncAuthData RPC

**Files:**
- Create: `internal/grpc/sync_auth.go`

This RPC streams all current user role data as snapshots, sends a SyncComplete marker, then streams incremental updates from the event bus.

- [ ] **Step 1: Create sync_auth.go**

Create `internal/grpc/sync_auth.go`:

```go
package grpc

import (
	"context"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/jasonKoogler/aegis/internal/ports"
	"github.com/google/uuid"
)

// SyncAuthData streams auth data to connected clients.
// First sends a full snapshot of all user roles, then streams incremental updates.
func (s *AegisAuthServer) SyncAuthData(req *pb.SyncRequest, stream pb.AegisAuth_SyncAuthDataServer) error {
	subscriberID := uuid.New().String()
	s.logger.Info("SyncAuthData client connected", log.String("subscriber_id", subscriberID))

	// Subscribe to events BEFORE sending snapshot to avoid missing events
	// that occur during snapshot transmission.
	eventCh := s.authEventBus.Subscribe(subscriberID)
	defer s.authEventBus.Unsubscribe(subscriberID)

	// Send full snapshot if no last_version provided or if we can't do a delta
	if req.LastVersion == "" {
		if err := s.sendAuthSnapshot(stream); err != nil {
			return err
		}
	}

	// Send SyncComplete marker
	if err := stream.Send(&pb.AuthDataEvent{
		Version: s.authEventBus.NextVersion(),
		Event:   &pb.AuthDataEvent_SyncComplete{SyncComplete: &pb.SyncComplete{}},
	}); err != nil {
		return err
	}

	s.logger.Info("SyncAuthData snapshot complete, streaming incremental updates",
		log.String("subscriber_id", subscriberID))

	// Stream incremental updates from event bus
	for {
		select {
		case <-stream.Context().Done():
			s.logger.Info("SyncAuthData client disconnected", log.String("subscriber_id", subscriberID))
			return nil
		case event, ok := <-eventCh:
			if !ok {
				return nil
			}
			if err := stream.Send(event); err != nil {
				return err
			}
		}
	}
}

// sendAuthSnapshot sends all current user roles as UserRolesSnapshot events.
func (s *AegisAuthServer) sendAuthSnapshot(stream pb.AegisAuth_SyncAuthDataServer) error {
	page := 1
	pageSize := 100

	for {
		users, err := s.userRepo.ListAll(stream.Context(), page, pageSize)
		if err != nil {
			return err
		}

		for _, user := range users {
			roles := user.GetRoles()
			if roles == nil || len(roles) == 0 {
				continue
			}

			// Send one snapshot per user-tenant combination
			for tenantID, tenantRoles := range roles {
				roleStrings := make([]string, len(tenantRoles))
				for i, r := range tenantRoles {
					roleStrings[i] = string(r)
				}

				event := &pb.AuthDataEvent{
					Version: s.authEventBus.NextVersion(),
					Event: &pb.AuthDataEvent_UserRoles{
						UserRoles: &pb.UserRolesSnapshot{
							UserId:   user.ID.String(),
							TenantId: tenantID.String(),
							Roles:    roleStrings,
						},
					},
				}

				if err := stream.Send(event); err != nil {
					return err
				}
			}
		}

		if len(users) < pageSize {
			break
		}
		page++
	}

	return nil
}

// PublishUserRolesChanged publishes a role change event for a specific user.
// Call this from auth services when roles/permissions change.
func (s *AegisAuthServer) PublishUserRolesChanged(ctx context.Context, userID uuid.UUID) error {
	roles, err := s.userRepo.GetRoles(ctx, userID)
	if err != nil {
		return err
	}

	if roles == nil {
		return nil
	}

	for tenantID, tenantRoles := range *roles {
		roleStrings := make([]string, len(tenantRoles))
		for i, r := range tenantRoles {
			roleStrings[i] = string(r)
		}

		s.authEventBus.Publish(&pb.AuthDataEvent{
			Event: &pb.AuthDataEvent_UserRoles{
				UserRoles: &pb.UserRolesSnapshot{
					UserId:   userID.String(),
					TenantId: tenantID.String(),
					Roles:    roleStrings,
				},
			},
		})
	}

	return nil
}

// PublishTokenRevoked publishes a token revocation event.
// Call this from auth services on logout, password change, etc.
func (s *AegisAuthServer) PublishTokenRevoked(jti string, expiresAt int64) {
	s.authEventBus.Publish(&pb.AuthDataEvent{
		Event: &pb.AuthDataEvent_TokenRevoked{
			TokenRevoked: &pb.TokenRevoked{
				Jti:       jti,
				ExpiresAt: expiresAt,
			},
		},
	})
}
```

Note: The `AegisAuthServer` struct is defined in Task 5 (server.go). The `userRepo` field is of type `ports.UserRepository`. Adjust field names if they differ from the ports interface.

- [ ] **Step 2: Commit**

```bash
cd /home/jason/jdk/aegis
git add internal/grpc/sync_auth.go
git commit -m "feat: implement SyncAuthData gRPC streaming RPC"
```

---

### Task 4: Implement SyncPolicies, CheckPermission, and ValidateAPIKey RPCs

**Files:**
- Create: `internal/grpc/sync_policies.go`
- Create: `internal/grpc/permissions.go`
- Create: `internal/grpc/apikey.go`

- [ ] **Step 1: Create sync_policies.go**

```go
package grpc

import (
	"os"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/google/uuid"
)

// SyncPolicies streams OPA policy updates to connected clients.
func (s *AegisAuthServer) SyncPolicies(req *pb.SyncRequest, stream pb.AegisAuth_SyncPoliciesServer) error {
	subscriberID := uuid.New().String()
	s.logger.Info("SyncPolicies client connected", log.String("subscriber_id", subscriberID))

	eventCh := s.policyEventBus.Subscribe(subscriberID)
	defer s.policyEventBus.Unsubscribe(subscriberID)

	// Send current policies
	policies, err := s.loadPolicies()
	if err != nil {
		return err
	}

	if err := stream.Send(&pb.PolicyEvent{
		Version:  s.policyEventBus.NextVersion(),
		Policies: policies,
	}); err != nil {
		return err
	}

	s.logger.Info("SyncPolicies initial snapshot sent", log.String("subscriber_id", subscriberID))

	// Stream incremental updates
	for {
		select {
		case <-stream.Context().Done():
			s.logger.Info("SyncPolicies client disconnected", log.String("subscriber_id", subscriberID))
			return nil
		case event, ok := <-eventCh:
			if !ok {
				return nil
			}
			if err := stream.Send(event); err != nil {
				return err
			}
		}
	}
}

// loadPolicies reads all policy files from the configured policy directory.
func (s *AegisAuthServer) loadPolicies() (map[string]string, error) {
	policies := make(map[string]string)

	if s.policyFilePath == "" {
		return policies, nil
	}

	content, err := os.ReadFile(s.policyFilePath)
	if err != nil {
		return nil, err
	}

	policies["authz.rego"] = string(content)
	return policies, nil
}

// PublishPoliciesChanged reloads and publishes all policies.
// Call this when policy files are updated.
func (s *AegisAuthServer) PublishPoliciesChanged() error {
	policies, err := s.loadPolicies()
	if err != nil {
		return err
	}

	s.policyEventBus.Publish(&pb.PolicyEvent{
		Policies: policies,
	})

	return nil
}
```

- [ ] **Step 2: Create permissions.go**

```go
package grpc

import (
	"context"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
)

// CheckPermission evaluates whether a user has permission for a given action.
func (s *AegisAuthServer) CheckPermission(ctx context.Context, req *pb.CheckPermissionRequest) (*pb.CheckPermissionResponse, error) {
	if s.authzAdapter == nil {
		return &pb.CheckPermissionResponse{
			Allowed: false,
			Reason:  "authorization not configured",
		}, nil
	}

	input := map[string]interface{}{
		"user": map[string]interface{}{
			"id": req.UserId,
		},
		"request": map[string]interface{}{
			"action": req.Action,
		},
		"resource": map[string]interface{}{
			"type": req.ResourceType,
			"id":   req.ResourceId,
		},
		"tenant_id": req.TenantId,
	}

	decision, err := s.authzAdapter.Evaluate(ctx, input)
	if err != nil {
		return &pb.CheckPermissionResponse{
			Allowed: false,
			Reason:  err.Error(),
		}, nil
	}

	return &pb.CheckPermissionResponse{
		Allowed: decision.Allowed,
		Reason:  decision.Reason,
	}, nil
}
```

- [ ] **Step 3: Create apikey.go**

```go
package grpc

import (
	"context"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
)

// ValidateAPIKey validates an API key and returns its metadata.
func (s *AegisAuthServer) ValidateAPIKey(ctx context.Context, req *pb.ValidateAPIKeyRequest) (*pb.ValidateAPIKeyResponse, error) {
	apiKey, err := s.apiKeyService.ValidateAPIKey(ctx, req.ApiKey, "grpc")
	if err != nil {
		return &pb.ValidateAPIKeyResponse{
			Valid: false,
		}, nil
	}

	return &pb.ValidateAPIKeyResponse{
		Valid:    true,
		OwnerId: apiKey.UserID.String(),
		TenantId: apiKey.TenantID.String(),
		Scopes:  apiKey.Scopes,
	}, nil
}
```

- [ ] **Step 4: Commit**

```bash
cd /home/jason/jdk/aegis
git add internal/grpc/
git commit -m "feat: implement SyncPolicies, CheckPermission, ValidateAPIKey RPCs"
```

---

### Task 5: Create gRPC server setup

**Files:**
- Create: `internal/grpc/server.go`

This defines the `AegisAuthServer` struct and the gRPC server startup.

- [ ] **Step 1: Create server.go**

```go
package grpc

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/aegis/internal/adapters/authz"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/jasonKoogler/aegis/internal/ports"
	"github.com/jasonKoogler/aegis/internal/service"
)

// AegisAuthServer implements the AegisAuth gRPC service.
type AegisAuthServer struct {
	pb.UnimplementedAegisAuthServer

	logger         *log.Logger
	userRepo       ports.UserRepository
	apiKeyService  *service.APIKeyService
	authzAdapter   *authz.Adapter
	policyFilePath string

	authEventBus   *AuthEventBus
	policyEventBus *PolicyEventBus
}

// ServerConfig holds configuration for the gRPC server.
type ServerConfig struct {
	Port           string
	Logger         *log.Logger
	UserRepo       ports.UserRepository
	APIKeyService  *service.APIKeyService
	AuthzAdapter   *authz.Adapter
	PolicyFilePath string
}

// NewAegisAuthServer creates a new gRPC server instance.
func NewAegisAuthServer(cfg ServerConfig) *AegisAuthServer {
	return &AegisAuthServer{
		logger:         cfg.Logger,
		userRepo:       cfg.UserRepo,
		apiKeyService:  cfg.APIKeyService,
		authzAdapter:   cfg.AuthzAdapter,
		policyFilePath: cfg.PolicyFilePath,
		authEventBus:   NewAuthEventBus(),
		policyEventBus: NewPolicyEventBus(),
	}
}

// Start starts the gRPC server on the configured port.
// This should be called in a goroutine.
func (s *AegisAuthServer) Start(port string) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAegisAuthServer(grpcServer, s)

	// Enable reflection for debugging with grpcurl
	reflection.Register(grpcServer)

	s.logger.Info("gRPC server starting", log.String("port", port))

	return grpcServer.Serve(lis)
}

// GetAuthEventBus returns the auth event bus for publishing events from services.
func (s *AegisAuthServer) GetAuthEventBus() *AuthEventBus {
	return s.authEventBus
}

// GetPolicyEventBus returns the policy event bus for publishing events.
func (s *AegisAuthServer) GetPolicyEventBus() *PolicyEventBus {
	return s.policyEventBus
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

Fix any type mismatches between the server struct fields and the RPC implementations. Common issues:
- `ports.UserRepository` methods may return different types than expected
- `authz.Adapter.Evaluate()` return type needs to match what `permissions.go` expects
- `service.APIKeyService.ValidateAPIKey()` signature may differ

- [ ] **Step 3: Commit**

```bash
cd /home/jason/jdk/aegis
git add internal/grpc/server.go
git commit -m "feat: add AegisAuthServer struct and gRPC server startup"
```

---

### Task 6: Add gRPC config and wire into startup

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/options.go`
- Modify: `cmd/main.go`

- [ ] **Step 1: Add GRPCConfig to config.go**

Add to `internal/config/config.go`:

In the `Config` struct, add:
```go
GRPC GRPCConfig
```

Add the type definition:
```go
// GRPCConfig holds gRPC server configuration
type GRPCConfig struct {
	Port    string
	Enabled bool
}
```

In `LoadConfig()`, add to the config initialization:
```go
GRPC: GRPCConfig{
	Port:    getEnvString("GRPC_PORT", "9090"),
	Enabled: os.Getenv("GRPC_ENABLED") == "true",
},
```

Add the helper if it doesn't exist:
```go
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
```

- [ ] **Step 2: Add gRPC server to App struct in app.go**

Add to the `App` struct:
```go
grpcServer *aegisgrpc.AegisAuthServer
```

Add import:
```go
aegisgrpc "github.com/jasonKoogler/aegis/internal/grpc"
```

In the `Start()` method, add before `a.srv.Start()`:
```go
// Start gRPC server if enabled
if a.cfg.GRPC.Enabled && a.grpcServer != nil {
	go func() {
		if err := a.grpcServer.Start(a.cfg.GRPC.Port); err != nil {
			a.logger.Error("gRPC server failed", log.Error(err))
		}
	}()
	a.logger.Info("gRPC server started", log.String("port", a.cfg.GRPC.Port))
}
```

- [ ] **Step 3: Add WithGRPCServer option to options.go**

```go
// WithGRPCServer sets the gRPC server
func WithGRPCServer(server *aegisgrpc.AegisAuthServer) AppOption {
	return func(a *App) error {
		a.grpcServer = server
		return nil
	}
}

// WithDefaultGRPCServer creates a default gRPC server from app dependencies
func WithDefaultGRPCServer() AppOption {
	return func(a *App) error {
		if !a.cfg.GRPC.Enabled {
			return nil
		}

		a.grpcServer = aegisgrpc.NewAegisAuthServer(aegisgrpc.ServerConfig{
			Logger:         a.logger,
			UserRepo:       a.userRepo,
			APIKeyService:  a.apiKeyService,
			AuthzAdapter:   a.authzService,
			PolicyFilePath: a.cfg.Auth.AuthZ.PolicyFilePath,
		})

		return nil
	}
}
```

Add import:
```go
aegisgrpc "github.com/jasonKoogler/aegis/internal/grpc"
```

Add `WithDefaultGRPCServer(),` to the `WithAllDefaultServices` options list.

- [ ] **Step 4: Update .env.example**

Add to `.env.example`:
```
# gRPC Server
GRPC_PORT=9090
GRPC_ENABLED=true
```

- [ ] **Step 5: Verify compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

Fix any issues. The most likely problems are:
- Import cycle between `internal/grpc` and `internal/app`
- Missing fields in `ServerConfig`
- Type mismatches on `authzAdapter` field (check if it's `*authz.Adapter` vs the interface)

- [ ] **Step 6: Commit**

```bash
cd /home/jason/jdk/aegis
git add -A
git commit -m "feat: wire gRPC server into aegis startup

Add GRPCConfig, WithGRPCServer option, and start gRPC server
alongside HTTP server on separate port."
```

---

### Task 7: Final verification

**Files:** None modified

- [ ] **Step 1: Full compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

- [ ] **Step 2: Run tests**

```bash
cd /home/jason/jdk/aegis && go test ./... -count=1 -short 2>&1 | tail -30
```

- [ ] **Step 3: go vet**

```bash
cd /home/jason/jdk/aegis && go vet ./...
```

- [ ] **Step 4: Verify gRPC server files**

```bash
find /home/jason/jdk/aegis/internal/grpc/ -type f | sort
find /home/jason/jdk/aegis/api/grpc/ -type f | sort
```

Expected:
```
internal/grpc/apikey.go
internal/grpc/eventbus.go
internal/grpc/permissions.go
internal/grpc/server.go
internal/grpc/sync_auth.go
internal/grpc/sync_policies.go
api/grpc/aegispb/aegis.pb.go
api/grpc/aegispb/aegis.proto
api/grpc/aegispb/aegis_grpc.pb.go
```

- [ ] **Step 5: Commit any fixes**

```bash
cd /home/jason/jdk/aegis && git add -A && git commit -m "fix: resolve compilation issues in gRPC server"
```
