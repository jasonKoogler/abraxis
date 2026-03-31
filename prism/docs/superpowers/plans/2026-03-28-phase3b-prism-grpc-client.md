# Phase 3B: Prism gRPC Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a gRPC client to Prism that syncs auth data and policies from Aegis, caches them in Redis, and integrates with the gateway's auth middleware.

**Architecture:** Prism connects to Aegis's gRPC server on startup, subscribes to `SyncAuthData` and `SyncPolicies` streams, populates a Redis cache, and uses cached data for local JWT validation, revocation checks, and policy evaluation. On disconnect, it reconnects with exponential backoff and re-syncs.

**Tech Stack:** Go 1.24, google.golang.org/grpc, go-redis/v9, authz module

**Spec:** `/home/jason/jdk/aegis/docs/superpowers/specs/2026-03-28-phase3-grpc-contract-design.md`

**Dependency:** Plan 3A (Aegis gRPC Server) must be completed first — Prism imports `github.com/jasonKoogler/aegis/api/grpc/aegispb`.

---

## File Structure

### New files in Prism:

```
internal/features/gateway/adapters/aegis/
    client.go              — gRPC client, connection management, reconnection
    auth_sync.go           — SyncAuthData processing, Redis cache writes
    policy_sync.go         — SyncPolicies processing, authz.UpdatePolicies()

internal/config/config.go — add AegisConfig
internal/app/app.go       — add aegis client, start sync goroutines
internal/app/options.go   — add WithAegisClient option
cmd/main.go               — create and wire aegis client
```

### Modified files:

```
internal/features/auth/middleware.go — add revocation check after JWT validation
go.mod                               — add aegis module dependency
```

---

### Task 1: Add Aegis module dependency and create gRPC client

**Files:**
- Create: `internal/features/gateway/adapters/aegis/client.go`
- Modify: `go.mod`

- [ ] **Step 1: Add Aegis as a dependency**

```bash
cd /home/jason/jdk/prism
go get github.com/jasonKoogler/aegis/api/grpc/aegispb
go get google.golang.org/grpc@latest
```

If the module is private/local, add to `go.work` (already gitignored):
```
use (
    .
    ../aegis
    ../authz
)
```

- [ ] **Step 2: Create the gRPC client**

Create `internal/features/gateway/adapters/aegis/client.go`:

```go
package aegis

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/common/redis"
	"github.com/jasonKoogler/prism/internal/features/auth/adapters/authz"
)

// Client manages the gRPC connection to Aegis and syncs auth data.
type Client struct {
	conn         *grpc.ClientConn
	client       pb.AegisAuthClient
	logger       *log.Logger
	redisClient  *redis.RedisClient
	authzAdapter *authz.Adapter
	address      string
	cacheTTL     time.Duration
	maxBackoff   time.Duration

	ready    atomic.Bool
	stopOnce sync.Once
	stopCh   chan struct{}
}

// ClientConfig holds configuration for the Aegis gRPC client.
type ClientConfig struct {
	Address      string
	Logger       *log.Logger
	RedisClient  *redis.RedisClient
	AuthzAdapter *authz.Adapter
	CacheTTL     time.Duration
	MaxBackoff   time.Duration
}

// NewClient creates a new Aegis gRPC client.
func NewClient(cfg ClientConfig) (*Client, error) {
	conn, err := grpc.NewClient(cfg.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:         conn,
		client:       pb.NewAegisAuthClient(conn),
		logger:       cfg.Logger,
		redisClient:  cfg.RedisClient,
		authzAdapter: cfg.AuthzAdapter,
		address:      cfg.Address,
		cacheTTL:     cfg.CacheTTL,
		maxBackoff:   cfg.MaxBackoff,
		stopCh:       make(chan struct{}),
	}, nil
}

// Start begins the auth data and policy sync goroutines.
func (c *Client) Start(ctx context.Context) {
	go c.syncAuthDataLoop(ctx)
	go c.syncPoliciesLoop(ctx)
}

// Stop closes the gRPC connection.
func (c *Client) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
		if c.conn != nil {
			c.conn.Close()
		}
	})
}

// IsReady returns true when the initial sync is complete.
func (c *Client) IsReady() bool {
	return c.ready.Load()
}

// CheckPermission calls the Aegis CheckPermission RPC (fallback for cache misses).
func (c *Client) CheckPermission(ctx context.Context, userID, action, resourceType, resourceID, tenantID string) (bool, string, error) {
	resp, err := c.client.CheckPermission(ctx, &pb.CheckPermissionRequest{
		UserId:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceId:   resourceID,
		TenantId:     tenantID,
	})
	if err != nil {
		return false, "", err
	}
	return resp.Allowed, resp.Reason, nil
}

// ValidateAPIKey calls the Aegis ValidateAPIKey RPC.
func (c *Client) ValidateAPIKey(ctx context.Context, apiKey string) (*pb.ValidateAPIKeyResponse, error) {
	return c.client.ValidateAPIKey(ctx, &pb.ValidateAPIKeyRequest{
		ApiKey: apiKey,
	})
}

// backoff calculates exponential backoff duration.
func (c *Client) backoff(attempt int) time.Duration {
	d := time.Duration(1<<uint(attempt)) * time.Second
	if d > c.maxBackoff {
		d = c.maxBackoff
	}
	return d
}
```

- [ ] **Step 3: Verify compilation**

```bash
cd /home/jason/jdk/prism && go build ./...
```

- [ ] **Step 4: Commit**

```bash
cd /home/jason/jdk/prism
git add -A
git commit -m "feat: add Aegis gRPC client with connection management"
```

---

### Task 2: Implement auth data sync with Redis cache

**Files:**
- Create: `internal/features/gateway/adapters/aegis/auth_sync.go`

- [ ] **Step 1: Create auth_sync.go**

```go
package aegis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/prism/internal/common/log"
)

const (
	rolesCachePrefix      = "aegis:roles:"
	revokedCachePrefix    = "aegis:revoked:"
	syncVersionKey        = "aegis:sync:version"
)

// CachedRoles represents cached role/permission data for a user.
type CachedRoles struct {
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// syncAuthDataLoop connects to Aegis SyncAuthData and processes events.
// Reconnects with exponential backoff on failure.
func (c *Client) syncAuthDataLoop(ctx context.Context) {
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		err := c.runAuthSync(ctx)
		if err != nil {
			c.logger.Error("auth sync disconnected", log.Error(err))
		}

		// Backoff before reconnecting
		delay := c.backoff(attempt)
		c.logger.Info("reconnecting auth sync", log.String("delay", delay.String()))
		select {
		case <-time.After(delay):
			attempt++
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		}
	}
}

// runAuthSync performs a single sync session.
func (c *Client) runAuthSync(ctx context.Context) error {
	// Get last version for cursor-based reconnection
	lastVersion := c.getLastVersion(ctx)

	stream, err := c.client.SyncAuthData(ctx, &pb.SyncRequest{
		LastVersion: lastVersion,
	})
	if err != nil {
		return fmt.Errorf("failed to start auth sync: %w", err)
	}

	c.logger.Info("auth sync connected", log.String("last_version", lastVersion))

	for {
		event, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("auth sync recv error: %w", err)
		}

		switch e := event.Event.(type) {
		case *pb.AuthDataEvent_UserRoles:
			if err := c.cacheUserRoles(ctx, e.UserRoles); err != nil {
				c.logger.Error("failed to cache user roles", log.Error(err))
			}

		case *pb.AuthDataEvent_TokenRevoked:
			if err := c.cacheTokenRevocation(ctx, e.TokenRevoked); err != nil {
				c.logger.Error("failed to cache token revocation", log.Error(err))
			}

		case *pb.AuthDataEvent_SyncComplete:
			c.ready.Store(true)
			c.logger.Info("auth sync initial snapshot complete")
		}

		// Store version for cursor-based reconnection
		c.setLastVersion(ctx, event.Version)
	}
}

// cacheUserRoles writes user roles to Redis.
func (c *Client) cacheUserRoles(ctx context.Context, snapshot *pb.UserRolesSnapshot) error {
	key := fmt.Sprintf("%s%s", rolesCachePrefix, snapshot.UserId)

	data, err := json.Marshal(CachedRoles{
		Roles:       snapshot.Roles,
		Permissions: snapshot.Permissions,
	})
	if err != nil {
		return err
	}

	return c.redisClient.Set(ctx, key, string(data), c.cacheTTL)
}

// cacheTokenRevocation adds a token JTI to the revocation set.
func (c *Client) cacheTokenRevocation(ctx context.Context, revoked *pb.TokenRevoked) error {
	key := fmt.Sprintf("%s%s", revokedCachePrefix, revoked.Jti)

	// TTL = time until token expires
	ttl := time.Until(time.Unix(revoked.ExpiresAt, 0))
	if ttl <= 0 {
		return nil // Already expired, no need to cache
	}

	return c.redisClient.Set(ctx, key, "", ttl)
}

// IsTokenRevoked checks if a token JTI is in the revocation set.
func (c *Client) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	key := fmt.Sprintf("%s%s", revokedCachePrefix, jti)
	val, err := c.redisClient.Get(ctx, key)
	if err != nil {
		return false, nil // Redis error = treat as not revoked (fail open for availability)
	}
	return val != "", nil
}

// GetCachedRoles looks up cached roles for a user.
func (c *Client) GetCachedRoles(ctx context.Context, userID string) (*CachedRoles, error) {
	key := fmt.Sprintf("%s%s", rolesCachePrefix, userID)
	val, err := c.redisClient.Get(ctx, key)
	if err != nil || val == "" {
		return nil, nil // Cache miss
	}

	var roles CachedRoles
	if err := json.Unmarshal([]byte(val), &roles); err != nil {
		return nil, err
	}
	return &roles, nil
}

func (c *Client) getLastVersion(ctx context.Context) string {
	val, err := c.redisClient.Get(ctx, syncVersionKey)
	if err != nil {
		return ""
	}
	return val
}

func (c *Client) setLastVersion(ctx context.Context, version string) {
	_ = c.redisClient.Set(ctx, syncVersionKey, version, 0)
}
```

Note: The `redisClient.Set` and `redisClient.Get` method signatures may differ from what's shown. The implementer should check `internal/common/redis/` for the actual RedisClient API and adjust accordingly.

- [ ] **Step 2: Commit**

```bash
cd /home/jason/jdk/prism
git add internal/features/gateway/adapters/aegis/auth_sync.go
git commit -m "feat: implement auth data sync with Redis cache"
```

---

### Task 3: Implement policy sync

**Files:**
- Create: `internal/features/gateway/adapters/aegis/policy_sync.go`

- [ ] **Step 1: Create policy_sync.go**

```go
package aegis

import (
	"context"
	"fmt"
	"time"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/prism/internal/common/log"
)

// syncPoliciesLoop connects to Aegis SyncPolicies and processes events.
func (c *Client) syncPoliciesLoop(ctx context.Context) {
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		err := c.runPolicySync(ctx)
		if err != nil {
			c.logger.Error("policy sync disconnected", log.Error(err))
		}

		delay := c.backoff(attempt)
		c.logger.Info("reconnecting policy sync", log.String("delay", delay.String()))
		select {
		case <-time.After(delay):
			attempt++
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		}
	}
}

// runPolicySync performs a single policy sync session.
func (c *Client) runPolicySync(ctx context.Context) error {
	stream, err := c.client.SyncPolicies(ctx, &pb.SyncRequest{})
	if err != nil {
		return fmt.Errorf("failed to start policy sync: %w", err)
	}

	c.logger.Info("policy sync connected")

	for {
		event, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("policy sync recv error: %w", err)
		}

		if len(event.Policies) > 0 {
			if err := c.authzAdapter.UpdatePolicies(event.Policies); err != nil {
				c.logger.Error("failed to update policies", log.Error(err))
			} else {
				c.logger.Info("policies updated",
					log.String("version", event.Version),
					log.Int("count", len(event.Policies)))
			}
		}
	}
}
```

Note: `c.authzAdapter.UpdatePolicies()` — the implementer should check the actual `authz.Adapter` API. The underlying `authz.Agent` has `UpdatePolicies(policies map[string]string) error`. The Prism adapter may need a thin wrapper to expose this method.

- [ ] **Step 2: Commit**

```bash
cd /home/jason/jdk/prism
git add internal/features/gateway/adapters/aegis/policy_sync.go
git commit -m "feat: implement policy sync from Aegis"
```

---

### Task 4: Update auth middleware for revocation checks

**Files:**
- Modify: `internal/features/auth/middleware.go`

- [ ] **Step 1: Read current middleware.go**

Read `internal/features/auth/middleware.go` to understand the current structure.

- [ ] **Step 2: Add revocation check to Authenticate method**

The `AuthMiddleware` struct currently holds a `*domain.TokenValidator`. Add an optional `RevocationChecker` interface so the middleware can check if a token is revoked after JWT validation:

Add to `middleware.go`:

```go
// RevocationChecker checks if a token has been revoked.
type RevocationChecker interface {
	IsTokenRevoked(ctx context.Context, jti string) (bool, error)
}
```

Update the `AuthMiddleware` struct:
```go
type AuthMiddleware struct {
	validator          *domain.TokenValidator
	revocationChecker  RevocationChecker // nil = no revocation checking
}
```

Update `NewAuthMiddleware`:
```go
func NewAuthMiddleware(validator *domain.TokenValidator, opts ...AuthMiddlewareOption) *AuthMiddleware {
	m := &AuthMiddleware{validator: validator}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

type AuthMiddlewareOption func(*AuthMiddleware)

func WithRevocationChecker(checker RevocationChecker) AuthMiddlewareOption {
	return func(m *AuthMiddleware) {
		m.revocationChecker = checker
	}
}
```

Update `Authenticate` to check revocation after JWT validation:
```go
func (amw *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString, err := domain.ExtractTokenFromHeader(r)
		if err != nil || tokenString == "" {
			http.Error(w, "Unauthorized: no token", http.StatusUnauthorized)
			return
		}

		claims, err := amw.validator.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		// Check revocation if checker is configured
		if amw.revocationChecker != nil && claims.ID != "" {
			revoked, err := amw.revocationChecker.IsTokenRevoked(r.Context(), claims.ID)
			if err == nil && revoked {
				http.Error(w, "Unauthorized: token revoked", http.StatusUnauthorized)
				return
			}
		}

		ctx := domain.ContextWithUserContextData(r.Context(), claims.GetUserContextData())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
```

- [ ] **Step 3: Update callers of NewAuthMiddleware**

Search for all callers of `auth.NewAuthMiddleware` in the codebase and update them. The existing callers pass just the `TokenValidator` — they don't need the revocation checker yet (it's optional via functional options). So existing calls like `auth.NewAuthMiddleware(tokenValidator)` continue to work unchanged.

The revocation checker will be wired in Task 5 when we connect the Aegis client.

- [ ] **Step 4: Update middleware/combined.go**

Update `NewCombinedMiddleware` to accept optional auth middleware options:

```go
func NewCombinedMiddleware(tokenValidator *domain.TokenValidator, authzAdapter *authz.Adapter, authOpts ...auth.AuthMiddlewareOption) *CombinedMiddleware {
	return &CombinedMiddleware{
		authMiddleware: auth.NewAuthMiddleware(tokenValidator, authOpts...),
		authzAdapter:   authzAdapter,
	}
}
```

- [ ] **Step 5: Verify compilation**

```bash
cd /home/jason/jdk/prism && go build ./...
```

- [ ] **Step 6: Commit**

```bash
cd /home/jason/jdk/prism
git add -A
git commit -m "feat: add revocation checking to auth middleware

Optional RevocationChecker interface allows middleware to reject
revoked tokens. Uses functional options pattern — existing callers
are unaffected."
```

---

### Task 5: Add config and wire Aegis client into startup

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/options.go`
- Modify: `cmd/main.go`

- [ ] **Step 1: Add AegisConfig to config.go**

Add to the `Config` struct in `internal/config/config.go`:
```go
Aegis AegisConfig
```

Add the type:
```go
// AegisConfig holds configuration for the Aegis gRPC client
type AegisConfig struct {
	GRPCAddress string
	SyncEnabled bool
	CacheTTL    time.Duration
	MaxBackoff  time.Duration
}
```

Add to `LoadConfig()`:
```go
Aegis: AegisConfig{
	GRPCAddress: getEnvString("AEGIS_GRPC_ADDRESS", "localhost:9090"),
	SyncEnabled: os.Getenv("AEGIS_SYNC_ENABLED") == "true",
	CacheTTL:    getEnvDuration("AEGIS_CACHE_TTL", "60s"),
	MaxBackoff:  getEnvDuration("AEGIS_RECONNECT_MAX_BACKOFF", "30s"),
},
```

Add the helper if missing:
```go
func getEnvString(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
```

- [ ] **Step 2: Add Aegis client to App struct**

In `internal/app/app.go`, add to the App struct:
```go
aegisClient *aegis.Client
```

Add import:
```go
"github.com/jasonKoogler/prism/internal/features/gateway/adapters/aegis"
```

In `Start()`, before starting the HTTP server, add:
```go
// Start Aegis sync if configured
if a.cfg.Aegis.SyncEnabled && a.aegisClient != nil {
	a.aegisClient.Start(context.Background())
	a.logger.Info("Aegis gRPC sync started", log.String("address", a.cfg.Aegis.GRPCAddress))
}
```

In `Shutdown()`, add:
```go
if a.aegisClient != nil {
	a.aegisClient.Stop()
}
```

Update `CreateAuthMiddleware` to pass the revocation checker:
```go
// In the proxy handler section where auth middleware is created:
var authOpts []auth.AuthMiddlewareOption
if a.aegisClient != nil {
	authOpts = append(authOpts, auth.WithRevocationChecker(a.aegisClient))
}
authMiddlewareInstance := auth.NewAuthMiddleware(a.tokenValidator, authOpts...)
```

- [ ] **Step 3: Add WithAegisClient option to options.go**

```go
// WithAegisClient sets the Aegis gRPC client
func WithAegisClient(client *aegis.Client) AppOption {
	return func(a *App) error {
		a.aegisClient = client
		return nil
	}
}

// WithDefaultAegisClient creates a default Aegis client from config
func WithDefaultAegisClient() AppOption {
	return func(a *App) error {
		if !a.cfg.Aegis.SyncEnabled {
			return nil
		}

		client, err := aegis.NewClient(aegis.ClientConfig{
			Address:      a.cfg.Aegis.GRPCAddress,
			Logger:       a.logger,
			RedisClient:  a.redisClient,
			AuthzAdapter: a.authzService,
			CacheTTL:     a.cfg.Aegis.CacheTTL,
			MaxBackoff:   a.cfg.Aegis.MaxBackoff,
		})
		if err != nil {
			a.logger.Warn("failed to create Aegis client", log.Error(err))
			return nil
		}

		a.aegisClient = client
		return nil
	}
}
```

Add `WithDefaultAegisClient(),` to the `WithAllDefaultServices` options list.

- [ ] **Step 4: Update .env.example**

Add:
```
# Aegis gRPC Client
AEGIS_GRPC_ADDRESS=localhost:9090
AEGIS_SYNC_ENABLED=true
AEGIS_CACHE_TTL=60s
AEGIS_RECONNECT_MAX_BACKOFF=30s
```

- [ ] **Step 5: Verify compilation**

```bash
cd /home/jason/jdk/prism && go build ./...
```

- [ ] **Step 6: Commit**

```bash
cd /home/jason/jdk/prism
git add -A
git commit -m "feat: wire Aegis gRPC client into Prism startup

Add AegisConfig, WithAegisClient option, revocation checker
integration with auth middleware. Sync starts automatically
when AEGIS_SYNC_ENABLED=true."
```

---

### Task 6: Final verification

**Files:** None modified

- [ ] **Step 1: Full compilation**

```bash
cd /home/jason/jdk/prism && go build ./...
```

- [ ] **Step 2: Run tests**

```bash
cd /home/jason/jdk/prism && go test ./... -count=1 -short 2>&1 | tail -30
```

- [ ] **Step 3: go vet**

```bash
cd /home/jason/jdk/prism && go vet ./...
```

- [ ] **Step 4: Verify new files**

```bash
find /home/jason/jdk/prism/internal/features/gateway/adapters/aegis/ -type f | sort
```

Expected:
```
internal/features/gateway/adapters/aegis/auth_sync.go
internal/features/gateway/adapters/aegis/client.go
internal/features/gateway/adapters/aegis/policy_sync.go
```

- [ ] **Step 5: Commit any fixes**

```bash
cd /home/jason/jdk/prism && git add -A && git commit -m "fix: resolve compilation issues in Aegis client"
```
