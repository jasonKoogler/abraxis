package domain

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// jwksResponse represents the JWKS JSON response from the auth server.
type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

// jwkKey represents a single key in the JWKS response.
type jwkKey struct {
	Kty string `json:"kty"` // Key type (OKP for Ed25519)
	Crv string `json:"crv"` // Curve (Ed25519)
	Kid string `json:"kid"` // Key ID
	X   string `json:"x"`   // Base64url-encoded public key
	Use string `json:"use"` // Key usage (sig)
	Alg string `json:"alg"` // Algorithm (EdDSA)
}

// JWKSFetcher fetches and caches Ed25519 public keys from a JWKS endpoint.
// It implements the PublicKeyProvider interface for use with TokenValidator.
type JWKSFetcher struct {
	url             string
	refreshInterval time.Duration
	httpClient      *http.Client

	mu   sync.RWMutex
	keys map[string]ed25519.PublicKey

	ready  atomic.Bool
	cancel context.CancelFunc
	done   chan struct{}
}

// NewJWKSFetcher creates a new JWKSFetcher that retrieves public keys from
// the given URL. If refreshInterval is zero, it defaults to 5 minutes.
func NewJWKSFetcher(url string, refreshInterval time.Duration) *JWKSFetcher {
	if refreshInterval <= 0 {
		refreshInterval = 5 * time.Minute
	}
	return &JWKSFetcher{
		url:             url,
		refreshInterval: refreshInterval,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		keys: make(map[string]ed25519.PublicKey),
		done: make(chan struct{}),
	}
}

// Start blocks until the first JWKS fetch succeeds, then refreshes keys in the
// background at the configured interval. The background goroutine stops when
// Stop is called or ctx is cancelled.
func (f *JWKSFetcher) Start(ctx context.Context) error {
	// First fetch must succeed before we return.
	if err := f.fetch(ctx); err != nil {
		return fmt.Errorf("initial JWKS fetch: %w", err)
	}
	f.ready.Store(true)

	refreshCtx, cancel := context.WithCancel(ctx)
	f.cancel = cancel

	go f.refreshLoop(refreshCtx)
	return nil
}

// Stop stops the background refresh goroutine and waits for it to exit.
func (f *JWKSFetcher) Stop() {
	if f.cancel != nil {
		f.cancel()
		<-f.done
	}
}

// IsReady returns true after the first successful JWKS fetch.
func (f *JWKSFetcher) IsReady() bool {
	return f.ready.Load()
}

// FindPublicKey returns the Ed25519 public key for the given key ID, or nil
// if no key with that ID is cached.
func (f *JWKSFetcher) FindPublicKey(kid string) ed25519.PublicKey {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.keys[kid]
}

// refreshLoop periodically re-fetches the JWKS until the context is cancelled.
func (f *JWKSFetcher) refreshLoop(ctx context.Context) {
	defer close(f.done)

	ticker := time.NewTicker(f.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = f.fetch(ctx) // errors are non-fatal for refreshes
		}
	}
}

// fetch retrieves the JWKS from the configured URL and updates the key cache.
func (f *JWKSFetcher) fetch(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read JWKS response: %w", err)
	}

	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("parse JWKS response: %w", err)
	}

	keys := make(map[string]ed25519.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "OKP" || k.Crv != "Ed25519" {
			continue // skip non-Ed25519 keys
		}
		if k.Kid == "" || k.X == "" {
			continue // skip keys without kid or public key material
		}

		pubBytes, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			continue // skip malformed keys
		}
		if len(pubBytes) != ed25519.PublicKeySize {
			continue // skip keys with wrong size
		}

		keys[k.Kid] = ed25519.PublicKey(pubBytes)
	}

	if len(keys) == 0 {
		return fmt.Errorf("no valid Ed25519 keys found in JWKS response")
	}

	f.mu.Lock()
	f.keys = keys
	f.mu.Unlock()

	return nil
}
