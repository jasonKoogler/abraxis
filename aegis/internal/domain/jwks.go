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

// SigningKey holds an Ed25519 key pair and its key ID.
type SigningKey struct {
	KID        string
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// KeyManager manages Ed25519 signing keys with support for key rotation.
// The active key is used for signing new tokens. Old keys are retained
// so that tokens signed before a rotation can still be validated.
type KeyManager struct {
	mu        sync.RWMutex
	activeKey *SigningKey
	keys      map[string]*SigningKey // all keys indexed by KID
}

// NewKeyManager creates a new KeyManager with a freshly generated Ed25519 key pair.
func NewKeyManager() (*KeyManager, error) {
	key, err := generateSigningKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate initial signing key: %w", err)
	}

	km := &KeyManager{
		activeKey: key,
		keys:      map[string]*SigningKey{key.KID: key},
	}
	return km, nil
}

// ActiveKey returns the current signing key used for new tokens.
func (km *KeyManager) ActiveKey() *SigningKey {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.activeKey
}

// FindPublicKey returns the public key for the given KID, or an error if not found.
func (km *KeyManager) FindPublicKey(kid string) (ed25519.PublicKey, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	key, ok := km.keys[kid]
	if !ok {
		return nil, fmt.Errorf("key not found for kid: %s", kid)
	}
	return key.PublicKey, nil
}

// RotateKey generates a new Ed25519 key pair and makes it the active signing key.
// The previous key is retained for validation of previously issued tokens.
func (km *KeyManager) RotateKey() error {
	key, err := generateSigningKey()
	if err != nil {
		return fmt.Errorf("failed to generate new signing key: %w", err)
	}

	km.mu.Lock()
	defer km.mu.Unlock()
	km.keys[key.KID] = key
	km.activeKey = key
	return nil
}

// jwksKey represents a single key in the JWKS response (OKP / Ed25519).
type jwksKey struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	KID string `json:"kid"`
	X   string `json:"x"`
	Use string `json:"use"`
	Alg string `json:"alg"`
}

// jwksResponse is the top-level JWKS JSON structure.
type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

// JWKSHandler returns an http.HandlerFunc that serves the JWKS endpoint
// at /.well-known/jwks.json. It exposes all public keys (active + rotated).
func (km *KeyManager) JWKSHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		km.mu.RLock()
		keys := make([]jwksKey, 0, len(km.keys))
		for _, sk := range km.keys {
			keys = append(keys, jwksKey{
				Kty: "OKP",
				Crv: "Ed25519",
				KID: sk.KID,
				X:   base64.RawURLEncoding.EncodeToString(sk.PublicKey),
				Use: "sig",
				Alg: "EdDSA",
			})
		}
		km.mu.RUnlock()

		resp := jwksResponse{Keys: keys}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "failed to encode JWKS response", http.StatusInternalServerError)
		}
	}
}

// generateSigningKey creates a new Ed25519 key pair and derives a KID
// from the SHA-256 hash of the public key (first 16 bytes, base64url-encoded).
func generateSigningKey() (*SigningKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("ed25519 key generation failed: %w", err)
	}

	hash := sha256.Sum256(pub)
	kid := base64.RawURLEncoding.EncodeToString(hash[:16])

	return &SigningKey{
		KID:        kid,
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}
