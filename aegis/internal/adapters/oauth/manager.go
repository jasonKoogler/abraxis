package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"

	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
	"golang.org/x/oauth2"
)

// OAuthManager handles OAuth authentication logic
type OAuthManager struct {
	Providers Providers
	// Logger    *log.Logger
	Storage VerifierStorage

	rwmu sync.RWMutex
}

// NewOAuthManager creates a new OAuthManager instance
func NewOAuthManager(providers Providers, logger *log.Logger, storage VerifierStorage) *OAuthManager {
	return &OAuthManager{
		Providers: providers,
		// Logger:    logger,
		Storage: storage,
	}
}

// GenerateAuthURL creates an authorization URL for the specified provider
// Now returns only authURL and state, handles verifier storage internally
func (m *OAuthManager) GenerateAuthURL(ctx context.Context, providerName string) (string, string, error) {
	if providerName == "" {
		return "", "", ErrProviderRequired
	}

	config, err := m.Providers.Get(providerName)
	if err != nil {
		return "", "", ErrProviderNotFound
	}

	state, err := generateState()
	if err != nil {
		return "", "", err
	}

	verifier := oauth2.GenerateVerifier()
	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier))

	// Store verifier internally
	if err := m.Storage.Set(ctx, state, verifier); err != nil {
		return "", "", fmt.Errorf("failed to store verifier: %w", err)
	}

	return authURL, state, nil
}

// ExchangeCode exchanges the authorization code for tokens
// Now takes state instead of verifier, retrieves verifier internally
func (m *OAuthManager) ExchangeCode(ctx context.Context, providerName, code, state string) (*oauth2.Token, error) {
	if providerName == "" {
		return nil, ErrProviderRequired
	}

	config, err := m.Providers.Get(providerName)
	if err != nil {
		return nil, ErrProviderNotFound
	}

	if code == "" {
		return nil, ErrCodeRequired
	}

	if state == "" {
		return nil, errors.New("state is required")
	}

	// Retrieve verifier from storage
	verifier, err := m.Storage.Get(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("failed to get verifier: %w", err)
	}

	// Clean up verifier
	defer func() {
		if err := m.Storage.Del(ctx, state); err != nil {
			// m.Logger.Printf("failed to delete verifier: %v", err)
			fmt.Printf("failed to delete verifier: %v", err)
		}
	}()

	return config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
}

// RefreshToken refreshes an access token using the refresh token
// This is a wrapper around the oauth2.TokenSource.Token method
// It must be safe for concurrent use by multiple goroutines
func (m *OAuthManager) RefreshToken(ctx context.Context, providerName, refreshToken string) (*oauth2.Token, error) {
	m.rwmu.Lock()
	defer m.rwmu.Unlock()

	if providerName == "" {
		return nil, ErrProviderRequired
	}

	config, err := m.Providers.Get(providerName)
	if err != nil {
		return nil, ErrProviderNotFound
	}

	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	return config.TokenSource(ctx, token).Token()
}

func (m *OAuthManager) GetProviderConfig(provider string) (*oauth2.Config, error) {
	return m.Providers.Get(provider)
}

// generateState creates a random state string
func generateState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Error definitions
var (
	ErrProviderRequired = errors.New("provider is required")
	ErrProviderNotFound = errors.New("provider not found")
	ErrCodeRequired     = errors.New("code is required")
)
