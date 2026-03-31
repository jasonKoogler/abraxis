# Phase 4A: JWKS + Asymmetric JWT + Readiness Probes

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace HMAC-SHA256 shared secret JWT signing with Ed25519 asymmetric signing, add a JWKS endpoint to Aegis, add a JWKS fetcher to Prism, and gate Prism's readiness on JWKS + sync completion.

**Architecture:** Aegis generates an Ed25519 key pair, signs JWTs with the private key, and exposes the public key via `/.well-known/jwks.json`. Prism fetches the JWKS on startup, caches the public keys, and validates JWTs using EdDSA verification instead of HMAC. Prism's readiness probe fails until JWKS is loaded and initial gRPC sync is complete.

**Tech Stack:** Go 1.24, golang-jwt/jwt/v5, go-jose/go-jose/v4 (JWKS), crypto/ed25519

**Spec:** `docs/superpowers/specs/2026-03-28-aegis-prism-separation-design.md`, Section 4

---

## File Structure

### Aegis changes:
```
internal/domain/jwt.go         — replace HS256 with Ed25519 signing, add kid to tokens
internal/domain/jwks.go        — NEW: KeyManager, JWKS endpoint handler, key generation
internal/config/config.go      — add Ed25519 key config (file path or auto-generate)
internal/app/app.go            — register /.well-known/jwks.json endpoint
cmd/main.go                    — wire key manager
```

### Prism changes:
```
internal/domain/jwt.go         — replace HMAC validation with JWKS-based EdDSA validation
internal/domain/jwks.go        — NEW: JWKSFetcher, periodic refresh, key cache
internal/config/config.go      — add JWKS URL config
internal/app/app.go            — wire JWKS fetcher, gate readiness
internal/app/server.go         — update readiness check
```

---

### Task 1: Aegis — Create KeyManager and JWKS endpoint

**Files:**
- Create: `internal/domain/jwks.go` (in Aegis)

The KeyManager handles Ed25519 key pair generation, storage, and JWKS response generation.

- [ ] **Step 1: Create jwks.go in Aegis**

Create `/home/jason/jdk/aegis/internal/domain/jwks.go`:

```go
package domain

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// KeyManager manages Ed25519 signing keys and serves JWKS.
type KeyManager struct {
	mu         sync.RWMutex
	keys       []SigningKey
	activeKID  string
}

// SigningKey holds an Ed25519 key pair with a key ID.
type SigningKey struct {
	KID        string
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// NewKeyManager creates a KeyManager and generates an initial key pair.
func NewKeyManager() (*KeyManager, error) {
	km := &KeyManager{}
	if err := km.GenerateKey(); err != nil {
		return nil, err
	}
	return km, nil
}

// GenerateKey generates a new Ed25519 key pair and sets it as active.
// Previous keys are kept for overlapping rotation.
func (km *KeyManager) GenerateKey() error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate Ed25519 key: %w", err)
	}

	// Generate KID from public key hash
	hash := sha256.Sum256(pub)
	kid := base64.RawURLEncoding.EncodeToString(hash[:8])

	km.mu.Lock()
	defer km.mu.Unlock()

	km.keys = append(km.keys, SigningKey{
		KID:        kid,
		PrivateKey: priv,
		PublicKey:  pub,
	})
	km.activeKID = kid

	return nil
}

// ActiveKey returns the current active signing key.
func (km *KeyManager) ActiveKey() SigningKey {
	km.mu.RLock()
	defer km.mu.RUnlock()

	for _, k := range km.keys {
		if k.KID == km.activeKID {
			return k
		}
	}
	return km.keys[len(km.keys)-1]
}

// JWKSHandler returns an HTTP handler that serves the JWKS JSON.
func (km *KeyManager) JWKSHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		km.mu.RLock()
		defer km.mu.RUnlock()

		jwks := JWKS{Keys: make([]JWK, len(km.keys))}
		for i, k := range km.keys {
			jwks.Keys[i] = JWK{
				KTY: "OKP",
				CRV: "Ed25519",
				KID: k.KID,
				X:   base64.RawURLEncoding.EncodeToString(k.PublicKey),
				Use: "sig",
				Alg: "EdDSA",
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		json.NewEncoder(w).Encode(jwks)
	}
}

// JWKS represents a JSON Web Key Set.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key (Ed25519/OKP format).
type JWK struct {
	KTY string `json:"kty"`
	CRV string `json:"crv"`
	KID string `json:"kid"`
	X   string `json:"x"`
	Use string `json:"use"`
	Alg string `json:"alg"`
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

- [ ] **Step 3: Commit**

```bash
cd /home/jason/jdk/aegis && git add internal/domain/jwks.go && git commit -m "feat: add Ed25519 KeyManager and JWKS endpoint handler"
```

---

### Task 2: Aegis — Switch JWT signing to Ed25519

**Files:**
- Modify: `/home/jason/jdk/aegis/internal/domain/jwt.go`

Replace HMAC-SHA256 signing with Ed25519 (EdDSA). The TokenManager needs the KeyManager instead of a secret key.

- [ ] **Step 1: Read current jwt.go and update it**

Read `/home/jason/jdk/aegis/internal/domain/jwt.go`, then make these changes:

**Update TokenManager struct:**

Replace:
```go
type TokenManager struct {
	secretKey         []byte
	issuer            string
	accessExpiration  time.Duration
	refreshExpiration time.Duration
}
```

With:
```go
type TokenManager struct {
	keyManager        *KeyManager
	issuer            string
	accessExpiration  time.Duration
	refreshExpiration time.Duration
}
```

**Update NewTokenManager:**

Replace:
```go
func NewTokenManager(secretKey []byte, issuer string, accessExpiration, refreshExpiration time.Duration) *TokenManager {
	return &TokenManager{
		secretKey:         secretKey,
		issuer:            issuer,
```

With:
```go
func NewTokenManager(keyManager *KeyManager, issuer string, accessExpiration, refreshExpiration time.Duration) *TokenManager {
	return &TokenManager{
		keyManager:        keyManager,
		issuer:            issuer,
```

**Update GenerateTokenPair — use Ed25519:**

Replace line `token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)` with:
```go
activeKey := m.keyManager.ActiveKey()
token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
token.Header["kid"] = activeKey.KID
```

Replace line `accessToken, err := token.SignedString(m.secretKey)` with:
```go
accessToken, err := token.SignedString(activeKey.PrivateKey)
```

**Update makeRefreshToken — same changes:**

Replace `token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)` with:
```go
activeKey := m.keyManager.ActiveKey()
token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
token.Header["kid"] = activeKey.KID
```

Replace `refreshToken, err := token.SignedString(m.secretKey)` with:
```go
refreshToken, err := token.SignedString(activeKey.PrivateKey)
```

**Update RefreshToken method — same pattern for re-signing.**

**Update ValidateToken — accept EdDSA:**

Replace:
```go
func (m *TokenManager) ValidateToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secretKey, nil
	})
```

With:
```go
func (m *TokenManager) ValidateToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != "EdDSA" {
			return nil, errors.New("unexpected signing method")
		}
		kid, _ := token.Header["kid"].(string)
		key := m.keyManager.FindPublicKey(kid)
		if key == nil {
			return nil, errors.New("unknown key ID")
		}
		return key, nil
	})
```

**Update ParseRefreshToken similarly.**

**Add FindPublicKey method to KeyManager** (in jwks.go):

```go
// FindPublicKey finds a public key by KID. Returns nil if not found.
func (km *KeyManager) FindPublicKey(kid string) ed25519.PublicKey {
	km.mu.RLock()
	defer km.mu.RUnlock()

	for _, k := range km.keys {
		if k.KID == kid {
			return k.PublicKey
		}
	}
	return nil
}
```

- [ ] **Step 2: Update all callers of NewTokenManager**

Search for `NewTokenManager` calls and update them to pass `keyManager` instead of `secretKey`. This is likely in the auth_manager.go service. Read and update.

- [ ] **Step 3: Verify compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

- [ ] **Step 4: Commit**

```bash
cd /home/jason/jdk/aegis && git add -A && git commit -m "feat: switch JWT signing from HMAC to Ed25519

Replace HS256 with EdDSA signing. Tokens now include kid header
for key identification. KeyManager provides the signing key."
```

---

### Task 3: Aegis — Wire KeyManager and JWKS endpoint into startup

**Files:**
- Modify: `/home/jason/jdk/aegis/internal/app/app.go`
- Modify: `/home/jason/jdk/aegis/internal/app/options.go`

- [ ] **Step 1: Add keyManager to App struct**

Read `internal/app/app.go`. Add to the App struct:
```go
keyManager *domain.KeyManager
```

In the `Start()` method, register the JWKS endpoint as a public handler:
```go
"/health": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"ok"}`))
}),
"/.well-known/jwks.json": a.keyManager.JWKSHandler(),
"/docs": http.FileServer(http.Dir("./docs")),
```

- [ ] **Step 2: Add WithKeyManager option**

In `internal/app/options.go`, add:

```go
func WithKeyManager(km *domain.KeyManager) AppOption {
	return func(a *App) error {
		a.keyManager = km
		return nil
	}
}
```

Update `NewApp()` — if keyManager is nil, create a default one:
```go
if app.keyManager == nil {
	km, err := domain.NewKeyManager()
	if err != nil {
		return nil, err
	}
	app.keyManager = km
}
```

Update the auth service creation to pass keyManager instead of secretKey. Find where `NewTokenManager` or `NewAuthManager` is called in `NewApp()` or in `WithDefaultAuthService()` and pass `app.keyManager`.

- [ ] **Step 3: Verify compilation**

```bash
cd /home/jason/jdk/aegis && go build ./...
```

- [ ] **Step 4: Commit**

```bash
cd /home/jason/jdk/aegis && git add -A && git commit -m "feat: register JWKS endpoint and wire KeyManager into startup"
```

---

### Task 4: Prism — Create JWKSFetcher and switch to EdDSA validation

**Files:**
- Create: `/home/jason/jdk/prism/internal/domain/jwks.go`
- Modify: `/home/jason/jdk/prism/internal/domain/jwt.go`
- Modify: `/home/jason/jdk/prism/internal/config/config.go`

- [ ] **Step 1: Create JWKSFetcher in Prism**

Create `/home/jason/jdk/prism/internal/domain/jwks.go`:

```go
package domain

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// JWKSFetcher fetches and caches JWKS from Aegis.
type JWKSFetcher struct {
	url        string
	mu         sync.RWMutex
	keys       map[string]ed25519.PublicKey // kid → public key
	ready      atomic.Bool
	httpClient *http.Client
	interval   time.Duration
	stopCh     chan struct{}
}

// NewJWKSFetcher creates a new fetcher that periodically refreshes from the given URL.
func NewJWKSFetcher(jwksURL string, refreshInterval time.Duration) *JWKSFetcher {
	return &JWKSFetcher{
		url:        jwksURL,
		keys:       make(map[string]ed25519.PublicKey),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		interval:   refreshInterval,
		stopCh:     make(chan struct{}),
	}
}

// Start begins fetching JWKS. Blocks until the first fetch succeeds, then
// refreshes periodically in the background.
func (f *JWKSFetcher) Start(ctx context.Context) error {
	// Initial fetch — must succeed before returning
	if err := f.fetch(ctx); err != nil {
		return fmt.Errorf("initial JWKS fetch failed: %w", err)
	}
	f.ready.Store(true)

	// Background refresh
	go func() {
		ticker := time.NewTicker(f.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := f.fetch(ctx); err != nil {
					// Log but don't fail — keep using cached keys
					_ = err
				}
			case <-f.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Stop stops the background refresh.
func (f *JWKSFetcher) Stop() {
	close(f.stopCh)
}

// IsReady returns true after the first successful fetch.
func (f *JWKSFetcher) IsReady() bool {
	return f.ready.Load()
}

// FindPublicKey looks up a public key by KID.
func (f *JWKSFetcher) FindPublicKey(kid string) ed25519.PublicKey {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.keys[kid]
}

// fetch downloads and parses the JWKS.
func (f *JWKSFetcher) fetch(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return err
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return err
	}

	newKeys := make(map[string]ed25519.PublicKey, len(jwks.Keys))
	for _, jwk := range jwks.Keys {
		if jwk.KTY != "OKP" || jwk.CRV != "Ed25519" {
			continue
		}

		pubBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
		if err != nil {
			continue
		}

		if len(pubBytes) != ed25519.PublicKeySize {
			continue
		}

		newKeys[jwk.KID] = ed25519.PublicKey(pubBytes)
	}

	f.mu.Lock()
	f.keys = newKeys
	f.mu.Unlock()

	return nil
}

// JWKS represents a JSON Web Key Set.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key.
type JWK struct {
	KTY string `json:"kty"`
	CRV string `json:"crv"`
	KID string `json:"kid"`
	X   string `json:"x"`
	Use string `json:"use"`
	Alg string `json:"alg"`
}
```

- [ ] **Step 2: Update Prism's TokenValidator to use JWKS**

Read `/home/jason/jdk/prism/internal/domain/jwt.go`, then replace the `TokenValidator` to use JWKSFetcher instead of a shared secret:

Replace the struct:
```go
type TokenValidator struct {
	secretKey []byte
	issuer    string
}

func NewTokenValidator(secretKey []byte, issuer string) *TokenValidator {
	return &TokenValidator{
		secretKey: secretKey,
		issuer:    issuer,
	}
}
```

With:
```go
// PublicKeyProvider finds a public key by key ID.
type PublicKeyProvider interface {
	FindPublicKey(kid string) ed25519.PublicKey
}

type TokenValidator struct {
	keyProvider PublicKeyProvider
}

func NewTokenValidator(keyProvider PublicKeyProvider) *TokenValidator {
	return &TokenValidator{
		keyProvider: keyProvider,
	}
}
```

Add `"crypto/ed25519"` to imports.

Update `ValidateToken`:
```go
func (v *TokenValidator) ValidateToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != "EdDSA" {
			return nil, errors.New("unexpected signing method")
		}
		kid, _ := token.Header["kid"].(string)
		key := v.keyProvider.FindPublicKey(kid)
		if key == nil {
			return nil, errors.New("unknown key ID")
		}
		return key, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		if claims.ExpiresAt.Before(time.Now()) {
			return nil, ErrTokenExpired
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
```

- [ ] **Step 3: Add JWKS config to Prism**

Read `/home/jason/jdk/prism/internal/config/config.go`. Add to the `AegisConfig` struct (which should already exist from Phase 3B):

```go
JWKSURL         string
JWKSRefreshInterval time.Duration
```

In `LoadConfig()`, update the Aegis section:
```go
JWKSURL:              getEnvString("AEGIS_JWKS_URL", "http://localhost:8080/.well-known/jwks.json"),
JWKSRefreshInterval:  getEnvDuration("AEGIS_JWKS_REFRESH_INTERVAL", "5m"),
```

- [ ] **Step 4: Verify compilation**

```bash
cd /home/jason/jdk/prism && go build ./...
```

Fix any callers of `NewTokenValidator` — they currently pass `(secretKey, issuer)` but now need `(keyProvider)`. Find and update them in `app.go`, `options.go`, and `cmd/main.go`.

- [ ] **Step 5: Commit**

```bash
cd /home/jason/jdk/prism && git add -A && git commit -m "feat: switch JWT validation from HMAC to Ed25519 via JWKS

Add JWKSFetcher that caches public keys from Aegis. TokenValidator
now validates EdDSA signatures using kid-matched public keys."
```

---

### Task 5: Prism — Wire JWKSFetcher and readiness probes

**Files:**
- Modify: `/home/jason/jdk/prism/internal/app/app.go`
- Modify: `/home/jason/jdk/prism/internal/app/options.go`
- Modify: `/home/jason/jdk/prism/internal/app/server.go`

- [ ] **Step 1: Add JWKSFetcher to App struct**

Read `internal/app/app.go`. Add:
```go
jwksFetcher *domain.JWKSFetcher
```

In `Start()`, before creating the token validator, start the JWKS fetcher:
```go
// Start JWKS fetcher (blocks until first fetch succeeds)
if a.jwksFetcher != nil {
	if err := a.jwksFetcher.Start(context.Background()); err != nil {
		a.logger.Error("JWKS fetch failed - auth validation will not work", log.Error(err))
	} else {
		a.logger.Info("JWKS loaded from Aegis")
	}
}
```

Update where `TokenValidator` is created — it should now use the JWKS fetcher:
```go
if a.tokenValidator == nil && a.jwksFetcher != nil {
	a.tokenValidator = domain.NewTokenValidator(a.jwksFetcher)
}
```

- [ ] **Step 2: Add WithJWKSFetcher option**

In `options.go`:
```go
func WithDefaultJWKSFetcher() AppOption {
	return func(a *App) error {
		if a.cfg.Aegis.JWKSURL == "" {
			return nil
		}
		a.jwksFetcher = domain.NewJWKSFetcher(
			a.cfg.Aegis.JWKSURL,
			a.cfg.Aegis.JWKSRefreshInterval,
		)
		return nil
	}
}
```

Add to `WithAllDefaultServices` before the token validator creation.

Remove the old `WithTokenValidator` option that creates from secret key, or update it. The token validator should now be created from the JWKS fetcher, not a static secret.

- [ ] **Step 3: Update readiness probe**

Read `internal/app/server.go`. Find the `/ready` endpoint and update it to check both JWKS and Aegis sync readiness:

The readiness handler should check:
1. JWKS fetcher is ready (has keys loaded)
2. Aegis gRPC client is ready (initial sync complete) — if configured

```go
s.router.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
	// Check readiness conditions
	ready := true
	reasons := []string{}

	// These checks will be populated when the app wires in readiness checkers
	w.Header().Set("Content-Type", "application/json")
	if ready {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "not_ready",
			"reasons": reasons,
		})
	}
})
```

Add a `ReadinessChecker` interface to the App:

```go
type ReadinessChecker interface {
	IsReady() bool
}
```

Store checkers in the App struct:
```go
readinessCheckers []ReadinessChecker
```

In `Start()`, add checkers:
```go
if a.jwksFetcher != nil {
	a.readinessCheckers = append(a.readinessCheckers, a.jwksFetcher)
}
if a.aegisClient != nil {
	a.readinessCheckers = append(a.readinessCheckers, a.aegisClient)
}
```

Wire readiness into the `/ready` endpoint by passing the checkers to the server or registering a custom handler.

- [ ] **Step 4: Update .env.example**

Add to Prism's `.env.example`:
```
AEGIS_JWKS_URL=http://localhost:8080/.well-known/jwks.json
AEGIS_JWKS_REFRESH_INTERVAL=5m
```

- [ ] **Step 5: Verify compilation**

```bash
cd /home/jason/jdk/prism && go build ./...
```

- [ ] **Step 6: Commit**

```bash
cd /home/jason/jdk/prism && git add -A && git commit -m "feat: wire JWKS fetcher and readiness probes

Readiness probe gates on JWKS loaded + Aegis sync complete.
Liveness probe remains unconditional."
```

---

### Task 6: Final verification — both repos

**Files:** None modified

- [ ] **Step 1: Verify Aegis**

```bash
cd /home/jason/jdk/aegis && go build ./... && go vet ./... && echo "AEGIS OK"
```

- [ ] **Step 2: Verify Prism**

```bash
cd /home/jason/jdk/prism && go build ./... && go vet ./... && echo "PRISM OK"
```

- [ ] **Step 3: Check Aegis has JWKS endpoint**

```bash
cd /home/jason/jdk/aegis && grep -rn "jwks\|well-known\|KeyManager" --include="*.go" internal/app/ internal/domain/ | head -20
```

Expected: KeyManager in jwks.go, JWKS handler registration in app.go.

- [ ] **Step 4: Check Prism has no HMAC references**

```bash
cd /home/jason/jdk/prism && grep -rn "SigningMethodHMAC\|SigningMethodHS256\|secretKey\|JWTSecret" --include="*.go" internal/ | head -20
```

Expected: No matches — all HMAC/shared-secret code should be replaced.

- [ ] **Step 5: Check Aegis signing uses EdDSA**

```bash
cd /home/jason/jdk/aegis && grep -rn "SigningMethodEdDSA\|EdDSA\|ed25519" --include="*.go" internal/domain/ | head -10
```

Expected: EdDSA references in jwt.go and jwks.go.

- [ ] **Step 6: Commit any fixes**

```bash
cd /home/jason/jdk/aegis && git add -A && git commit -m "fix: resolve remaining JWKS issues" 2>/dev/null
cd /home/jason/jdk/prism && git add -A && git commit -m "fix: resolve remaining JWKS issues" 2>/dev/null
```
