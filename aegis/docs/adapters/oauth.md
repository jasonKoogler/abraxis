# OAuth Adapter

## Overview

The OAuth adapter provides a comprehensive solution for authenticating users through popular OAuth 2.0 providers like Google, Facebook, and Twitter. It handles the OAuth 2.0 flow, including authorization URL generation, code exchange, token management, and user information retrieval.

The adapter follows the OAuth 2.0 authorization code flow with PKCE (Proof Key for Code Exchange) for enhanced security, protecting against authorization code interception attacks.

## Key Components

The OAuth adapter is composed of several interconnected components:

### OAuthManager

The core component that orchestrates the OAuth authentication process:

```go
type OAuthManager struct {
    Providers Providers
    Storage   VerifierStorage
    rwmu      sync.RWMutex
}
```

- Manages the OAuth flow with multiple providers
- Handles state and PKCE code verifier generation
- Provides token exchange and refresh functionality
- Retrieves standardized user information

### Providers

A map-based registry of OAuth provider configurations:

```go
type Providers map[string]*oauth2.Config
```

- Stores OAuth2 configurations for different providers
- Provides type-safe access to provider-specific settings
- Handles endpoint differences between providers

### VerifierStorage

An interface for storing PKCE code verifiers securely:

```go
type VerifierStorage interface {
    Get(ctx context.Context, state string) (string, error)
    Set(ctx context.Context, state, verifier string) error
    Del(ctx context.Context, state string) error
}
```

With implementations:

1. **MemoryVerifierStorage** - In-memory storage for development/testing
2. **RedisVerifierStorage** - Redis-backed storage for production deployments

### UserInfo

A standardized structure for user information across providers:

```go
type UserInfo struct {
    Provider          string                 `json:"provider"`
    Email             string                 `json:"email"`
    Name              string                 `json:"name"`
    FirstName         string                 `json:"first_name"`
    LastName          string                 `json:"last_name"`
    UserID            string                 `json:"user_id"`
    AvatarURL         string                 `json:"avatar_url"`
    // Additional fields...
    RawData           map[string]interface{} `json:"-"` // Raw provider data
}
```

## Implementation Details

### OAuth Flow with PKCE

The adapter implements the OAuth 2.0 authorization code flow with PKCE:

1. **GenerateAuthURL**: Creates an authorization URL with state and PKCE verifier
2. **ExchangeCode**: Exchanges the code for tokens using the stored verifier
3. **RefreshToken**: Refreshes access tokens when they expire

The PKCE flow enhances security by:

- Generating a random code verifier
- Storing it securely in VerifierStorage
- Creating a code challenge from the verifier using S256
- Verifying the exchange with the original verifier

### Provider Integration

The adapter supports multiple OAuth providers:

- **Google**: Complete implementation with user profile information
- **Facebook**: Support for authentication and basic profile data
- **Twitter**: Basic implementation for authentication

Each provider has specialized user information extraction logic to map provider-specific responses to the standardized UserInfo structure.

### Thread Safety

The adapter ensures thread safety through:

- Read-write mutexes for token refresh operations
- Thread-safe verifier storage implementations
- Immutable provider configurations

## Configuration Options

### Provider Configuration

OAuth providers are configured using the `Oauth2ProviderConfig` structure:

```go
type Oauth2ProviderConfig struct {
    Name         string   // Provider identifier (e.g., "google", "facebook")
    ClientID     string   // OAuth client ID
    ClientSecret string   // OAuth client secret
    RedirectURL  string   // Redirect URL for callback
    Scopes       []string // Requested OAuth scopes
}
```

### Verifier Storage Options

Two storage options are available for PKCE verifiers:

1. **Memory Storage**: For development and single-instance deployments

   ```go
   storage := verifier.NewMemoryVerifierStorage()
   ```

2. **Redis Storage**: For production and distributed deployments
   ```go
   storage := verifier.NewRedisVerifierStorage(redisClient)
   ```

## Usage Examples

### Creating the OAuth Manager

```go
// Configure OAuth providers
providerConfigs := []config.Oauth2ProviderConfig{
    {
        Name:         "google",
        ClientID:     "your-google-client-id",
        ClientSecret: "your-google-client-secret",
        RedirectURL:  "https://example.com/auth/google/callback",
        Scopes:       []string{"profile", "email"},
    },
    {
        Name:         "facebook",
        ClientID:     "your-facebook-client-id",
        ClientSecret: "your-facebook-client-secret",
        RedirectURL:  "https://example.com/auth/facebook/callback",
        Scopes:       []string{"public_profile", "email"},
    },
}

// Create providers map
providers, err := oauth.NewProviders(providerConfigs)
if err != nil {
    log.Fatalf("Failed to create OAuth providers: %v", err)
}

// Create verifier storage
storage := verifier.NewRedisVerifierStorage(redisClient)

// Initialize OAuth manager
oauthManager := oauth.NewOAuthManager(providers, logger, storage)
```

### Initiating OAuth Flow

```go
// Generate authorization URL with state
authURL, state, err := oauthManager.GenerateAuthURL(ctx, "google")
if err != nil {
    return fmt.Errorf("failed to generate auth URL: %w", err)
}

// Store state in session or return to client
session.Set("oauth_state", state)

// Redirect user to authorization URL
http.Redirect(w, r, authURL, http.StatusFound)
```

### Handling OAuth Callback

```go
// Get state and code from request
state := r.URL.Query().Get("state")
code := r.URL.Query().Get("code")

// Validate state (should match stored state)
expectedState := session.Get("oauth_state")
if state != expectedState {
    return fmt.Errorf("state mismatch")
}

// Exchange code for token
token, err := oauthManager.ExchangeCode(ctx, "google", code, state)
if err != nil {
    return fmt.Errorf("failed to exchange code: %w", err)
}

// Get user info
userInfo, err := oauthManager.GetUserInfo(ctx, "google", token)
if err != nil {
    return fmt.Errorf("failed to get user info: %w", err)
}

// Use userInfo to create or update user account
user, err := userService.FindOrCreateUser(ctx, userInfo)
if err != nil {
    return fmt.Errorf("failed to process user: %w", err)
}

// Create session, issue JWT, etc.
```

### Refreshing Tokens

```go
// When access token expires, refresh it
newToken, err := oauthManager.RefreshToken(ctx, "google", refreshToken)
if err != nil {
    return fmt.Errorf("failed to refresh token: %w", err)
}

// Update stored token
user.OAuthToken = newToken.AccessToken
user.RefreshToken = newToken.RefreshToken
user.TokenExpiry = newToken.Expiry

if err := userService.UpdateUser(ctx, user); err != nil {
    return fmt.Errorf("failed to update user tokens: %w", err)
}
```

## Integration with Other Components

### Auth Manager Integration

The OAuth adapter integrates with the authentication manager:

```go
// In Auth Manager implementation
func (am *AuthManager) OAuthGetLoginURL(ctx context.Context, provider string) (string, string, error) {
    return am.oauthManager.GenerateAuthURL(ctx, provider)
}

func (am *AuthManager) OAuthCallback(ctx context.Context, provider, code, state string,
    sessionParams *domain.SessionMetaDataParams) (*domain.TokenPair, string, error) {

    // Exchange code for token
    token, err := am.oauthManager.ExchangeCode(ctx, provider, code, state)
    if err != nil {
        return nil, "", err
    }

    // Get user info
    userInfo, err := am.oauthManager.GetUserInfo(ctx, provider, token)
    if err != nil {
        return nil, "", err
    }

    // Find or create user
    user, err := am.userService.GetUserByProviderUserID(ctx, provider, userInfo.UserID)
    if err != nil {
        // Create new user from OAuth profile
        user, err = am.createUserFromOAuth(ctx, userInfo, provider)
        if err != nil {
            return nil, "", err
        }
    }

    // Create session and tokens
    return am.createTokensAndSession(ctx, user, sessionParams)
}
```

## Security Considerations

1. **PKCE Implementation**: Uses secure code challenge method (S256)
2. **State Parameter**: Prevents CSRF attacks in the OAuth flow
3. **Secure Storage**: Options for secure verifier storage
4. **Thread Safety**: Ensures safe concurrent access
5. **Token Handling**: Proper refresh token management
6. **Sensitive Data**: OAuth secrets not exposed in responses

## Error Handling

The adapter provides specific error types:

- `ErrProviderRequired`: Missing provider name
- `ErrProviderNotFound`: Unknown OAuth provider
- `ErrCodeRequired`: Missing authorization code

All errors are properly wrapped and propagated with context.
