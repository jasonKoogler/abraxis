package tests

import (
	"testing"

	"github.com/jasonKoogler/aegis/internal/adapters/oauth"
	"github.com/jasonKoogler/aegis/internal/config"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

func TestNewProviders(t *testing.T) {
	tests := []struct {
		name        string
		configs     []config.Oauth2ProviderConfig
		expectedErr bool
	}{
		{
			name: "Valid google provider",
			configs: []config.Oauth2ProviderConfig{
				{
					Name:         "google",
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"profile", "email"},
				},
			},
			expectedErr: false,
		},
		{
			name: "Valid facebook provider",
			configs: []config.Oauth2ProviderConfig{
				{
					Name:         "facebook",
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"public_profile", "email"},
				},
			},
			expectedErr: false,
		},
		{
			name: "Multiple valid providers",
			configs: []config.Oauth2ProviderConfig{
				{
					Name:         "google",
					ClientID:     "google-client-id",
					ClientSecret: "google-client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"profile", "email"},
				},
				{
					Name:         "facebook",
					ClientID:     "facebook-client-id",
					ClientSecret: "facebook-client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"public_profile", "email"},
				},
			},
			expectedErr: false,
		},
		{
			name: "Missing provider name",
			configs: []config.Oauth2ProviderConfig{
				{
					Name:         "",
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"profile", "email"},
				},
			},
			expectedErr: true,
		},
		{
			name: "Missing client ID",
			configs: []config.Oauth2ProviderConfig{
				{
					Name:         "google",
					ClientID:     "",
					ClientSecret: "test-client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"profile", "email"},
				},
			},
			expectedErr: true,
		},
		{
			name: "Missing client secret",
			configs: []config.Oauth2ProviderConfig{
				{
					Name:         "google",
					ClientID:     "test-client-id",
					ClientSecret: "",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"profile", "email"},
				},
			},
			expectedErr: true,
		},
		{
			name: "Missing redirect URL",
			configs: []config.Oauth2ProviderConfig{
				{
					Name:         "google",
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					RedirectURL:  "",
					Scopes:       []string{"profile", "email"},
				},
			},
			expectedErr: true,
		},
		{
			name: "Empty scopes",
			configs: []config.Oauth2ProviderConfig{
				{
					Name:         "google",
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{},
				},
			},
			expectedErr: true,
		},
		{
			name: "Unsupported provider",
			configs: []config.Oauth2ProviderConfig{
				{
					Name:         "unknown",
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"profile", "email"},
				},
			},
			expectedErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			providers, err := oauth.NewProviders(tc.configs)

			if tc.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, providers)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, providers)

				// Test provider configuration
				for _, cfg := range tc.configs {
					provider, err := providers.Get(cfg.Name)
					assert.NoError(t, err)
					assert.NotNil(t, provider)

					assert.Equal(t, cfg.ClientID, provider.ClientID)
					assert.Equal(t, cfg.ClientSecret, provider.ClientSecret)
					assert.Equal(t, cfg.RedirectURL, provider.RedirectURL)
					assert.Equal(t, cfg.Scopes, provider.Scopes)

					// Check endpoint based on provider name
					switch cfg.Name {
					case "google":
						// Get the actual endpoint from the provider and check it field by field
						actualEndpoint := provider.Endpoint
						expectedAuthURL := "https://accounts.google.com/o/oauth2/auth"
						expectedTokenURL := "https://oauth2.googleapis.com/token"
						expectedDeviceAuthURL := "https://oauth2.googleapis.com/device/code"
						expectedAuthStyle := oauth2.AuthStyle(1) // Direct value comparison

						assert.Equal(t, expectedAuthURL, actualEndpoint.AuthURL)
						assert.Equal(t, expectedTokenURL, actualEndpoint.TokenURL)
						assert.Equal(t, expectedDeviceAuthURL, actualEndpoint.DeviceAuthURL)
						assert.Equal(t, int(expectedAuthStyle), int(actualEndpoint.AuthStyle))
					case "facebook":
						// Get the actual endpoint from the provider and check it field by field
						actualEndpoint := provider.Endpoint
						expectedAuthURL := "https://www.facebook.com/v3.2/dialog/oauth"
						expectedTokenURL := "https://graph.facebook.com/v3.2/oauth/access_token"
						expectedDeviceAuthURL := ""
						expectedAuthStyle := oauth2.AuthStyle(0) // Direct value comparison

						assert.Equal(t, expectedAuthURL, actualEndpoint.AuthURL)
						assert.Equal(t, expectedTokenURL, actualEndpoint.TokenURL)
						assert.Equal(t, expectedDeviceAuthURL, actualEndpoint.DeviceAuthURL)
						assert.Equal(t, int(expectedAuthStyle), int(actualEndpoint.AuthStyle))
					}
				}
			}
		})
	}
}

func TestProvidersGet(t *testing.T) {
	configs := []config.Oauth2ProviderConfig{
		{
			Name:         "google",
			ClientID:     "google-client-id",
			ClientSecret: "google-client-secret",
			RedirectURL:  "http://localhost:8080/callback",
			Scopes:       []string{"profile", "email"},
		},
	}

	providers, err := oauth.NewProviders(configs)
	assert.NoError(t, err)
	assert.NotNil(t, providers)

	// Test getting an existing provider
	provider, err := providers.Get("google")
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "google-client-id", provider.ClientID)

	// Test getting a non-existent provider
	provider, err = providers.Get("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "provider nonexistent not found")
}
