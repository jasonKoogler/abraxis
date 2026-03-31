package oauth

import (
	"errors"
	"fmt"

	"github.com/jasonKoogler/aegis/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
	"golang.org/x/oauth2/google"
)

// Note: Ensure that the "os" package is imported in your main import block.

// // ProviderConfig defines the configuration required for any OAuth provider.
// type ProviderConfig struct {
// 	Name         string          // Unique name for the provider (e.g. "google", "facebook")
// 	ClientID     string          // OAuth Client ID
// 	ClientSecret string          // OAuth Client Secret
// 	RedirectURL  string          // OAuth redirect URL
// 	Scopes       []string        // OAuth scopes
// 	Endpoint     oauth2.Endpoint // OAuth endpoint (auth and token URLs)
// }

// Providers is a lookup map for OAuth provider configurations.
type Providers map[string]*oauth2.Config

// NewProviders constructs a Providers instance from a slice of ProviderConfig.
// Returns an error if any required field is missing.
func NewProviders(configs []config.Oauth2ProviderConfig) (Providers, error) {
	providers := make(Providers)

	for _, cfg := range configs {
		// Validate required fields
		if cfg.Name == "" {
			return nil, errors.New("provider name is required")
		}
		if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURL == "" {
			return nil, fmt.Errorf("incomplete configuration for provider %s", cfg.Name)
		}
		if len(cfg.Scopes) == 0 {
			return nil, fmt.Errorf("scopes are required for provider %s", cfg.Name)
		}

		var endpoint oauth2.Endpoint
		switch cfg.Name {
		case "google":
			endpoint = google.Endpoint
		case "facebook":
			endpoint = facebook.Endpoint
		default:
			return nil, fmt.Errorf("unsupported provider: %s", cfg.Name)
		}

		providers[cfg.Name] = &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
			Endpoint:     endpoint,
		}
	}
	return providers, nil
}

// Get retrieves an oauth2.Config for the given provider name.
func (p Providers) Get(name string) (*oauth2.Config, error) {
	config, exists := p[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	return config, nil
}
