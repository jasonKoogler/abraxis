package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jasonKoogler/abraxis/aegis/internal/adapters/oauth"
	"github.com/jasonKoogler/abraxis/aegis/internal/adapters/oauth/verifier"
	"github.com/jasonKoogler/abraxis/aegis/internal/adapters/session"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/redis"
	"github.com/jasonKoogler/abraxis/aegis/internal/config"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
	"github.com/jasonKoogler/abraxis/aegis/internal/ports"
	"golang.org/x/oauth2"
)

// AuthManager handles authentication operations.
type AuthManager struct {
	// redisClient *redis.RedisClient
	sessionManager ports.SessionManager
	tokenManager   *domain.TokenManager
	logger         *log.Logger
	authConfig     *config.AuthConfig
	userService    *UserService
	oauthManager   *oauth.OAuthManager

	// rwmu sync.RWMutex

	tokenExpiration        time.Duration
	refreshTokenExpiration time.Duration

	tokenRotationInterval time.Duration

	// authZAgent  *authz.Agent
	// authZConfig *config.AuthZConfig
}

// var _ ports.AuthService = &AuthManager{}

// NewAuthManager creates a new AuthManager.
// todo: whether to pass the redis client explicitly in parameters or initialize it here
func NewAuthManager(cfg *config.AuthConfig, logger *log.Logger, userService *UserService, keyManager *domain.KeyManager) (*AuthManager, error) {
	if cfg == nil || logger == nil || userService == nil || keyManager == nil {
		return nil, fmt.Errorf("all dependencies must be provided")
	}

	var sessionManager ports.SessionManager
	if cfg.AuthN.SessionManager == "redis" {
		redisClient, err := redis.NewRedisClient(context.Background(), logger, &config.RedisConfig{
			Host:     cfg.AuthN.RedisConfig.Host,
			Port:     cfg.AuthN.RedisConfig.Port,
			Password: cfg.AuthN.RedisConfig.Password,
			Username: cfg.AuthN.RedisConfig.Username,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create redis client: %w", err)
		}
		sessionManager = session.NewRedisSessionManager(redisClient, logger)
	} else if cfg.AuthN.SessionManager == "memory" {
		sessionManager = session.NewMemorySessionManager(logger)
	} else {
		return nil, fmt.Errorf("invalid session manager: %s", cfg.AuthN.SessionManager)
	}

	if cfg.AuthN.AccessTokenExpiration < 1 {
		return nil, fmt.Errorf("token expiration must be positive")
	}

	tokenManager := domain.NewTokenManager(keyManager, cfg.AuthN.JWTIssuer, cfg.AuthN.AccessTokenExpiration, cfg.AuthN.RefreshTokenExpiration)

	// Initialize OAuthManager
	oauthProviders, err := oauth.NewProviders(cfg.AuthN.OAuthConfig.Providers)
	if err != nil {
		return nil, fmt.Errorf("failed to create oauth providers: %w", err)
	}

	// Initialize VerifierStorage
	var verifierStorage oauth.VerifierStorage
	if cfg.AuthN.OAuthConfig.VerifierStorage == "redis" {
		redisClient, err := redis.NewRedisClient(context.Background(), logger, &config.RedisConfig{
			Host:     cfg.AuthN.RedisConfig.Host,
			Port:     cfg.AuthN.RedisConfig.Port,
			Password: cfg.AuthN.RedisConfig.Password,
			Username: cfg.AuthN.RedisConfig.Username,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create redis client: %w", err)
		}
		verifierStorage = verifier.NewRedisVerifierStorage(redisClient)
	} else {
		verifierStorage = verifier.NewMemoryVerifierStorage()
	}
	oauthManager := oauth.NewOAuthManager(oauthProviders, logger, verifierStorage)

	tokenExpiration := cfg.AuthN.AccessTokenExpiration * time.Hour
	refreshTokenExpiration := cfg.AuthN.RefreshTokenExpiration * time.Hour
	tokenRotationInterval := cfg.AuthN.TokenRotationInterval * time.Hour

	// var keyExtractor provider.KeyExtractor = func(r *http.Request) string {
	// 	userID := r.Context().Value("userID").(string)
	// 	return userID
	// }
	// var authZAgent *authz.Agent
	// if authZCfg.CacheProvider == "redis" {
	// 	redisClient, err := redis.NewRedisClient(context.Background(), logger, &config.RedisConfig{
	// 		Host:     authZCfg.RedisConfig.Host,
	// 		Port:     authZCfg.RedisConfig.Port,
	// 		Password: authZCfg.RedisConfig.Password,
	// 		Username: authZCfg.RedisConfig.Username,
	// 	})
	// 	authZAgent, err = authz.NewAgent(
	// 		authz.WithDataProvider(provider.NewDataProvider(redisClient, keyExtractor)),
	// 		authz.WithLogger(logger),
	// 		authz.WithPolicyFile(authZCfg.PolicyFile),
	// 	)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to create authz agent with redis client: %w", err)
	// 	}
	// } else {
	// 	authZAgent, err = authz.NewAgent(
	// 		authz.WithDataProvider(provider.NewDataProvider(nil, keyExtractor)),
	// 		authz.WithLogger(logger),
	// 		authz.WithPolicyFile(authZCfg.PolicyFile),
	// 	)
	// }
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create authz agent: %w", err)
	// }

	return &AuthManager{
		sessionManager:         sessionManager,
		tokenManager:           tokenManager,
		logger:                 logger,
		authConfig:             cfg,
		userService:            userService,
		oauthManager:           oauthManager,
		tokenExpiration:        tokenExpiration,
		refreshTokenExpiration: refreshTokenExpiration,
		tokenRotationInterval:  tokenRotationInterval,
		// authZAgent:             authZAgent,
		// authZConfig:            authZCfg,
	}, nil
}

// Close closes any resources held by the AuthManager.
func (am *AuthManager) Close() error {
	if err := am.sessionManager.Close(); err != nil {
		am.logger.Error("Failed to close session manager", log.Error(err))
		return err
	}
	return nil
}

// GetUserByID retrieves a user by their ID.
func (am *AuthManager) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	usr, err := am.userService.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		am.logError(ctx, "Failed to get user data from database", err)
		return nil, fmt.Errorf("failed to get user data: %w", err)
	}

	am.logger.Info("User retrieved successfully", log.String("userID", userID))
	return usr, nil
}

// GetUserByEmail retrieves a user by their email.
func (am *AuthManager) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))

	usr, err := am.userService.GetByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		am.logError(ctx, "Failed to get user data from database", err)
		return nil, fmt.Errorf("failed to get user data: %w", err)
	}

	return usr, nil
}

// func (am *AuthManager) GetUserIDFromRefreshToken(ctx context.Context, refreshToken string) (string, error) {
// 	claims, err := am.tokenManager.ValidateToken(refreshToken)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to validate refresh token: %w", err)
// 	}
// 	return claims.Subject, nil
// }

func (am *AuthManager) RegisterUser(ctx context.Context, params *domain.UserCreateParams, sessionMetaDataParams *domain.SessionMetaDataParams) (*domain.TokenPair, string, error) {
	user, err := am.userService.Create(ctx, params)
	if err != nil {
		return nil, "", err
	}

	sessionParams := &domain.SessionCreateParams{
		UserID:        user.FormatID(),
		Roles:         domain.RoleMap{},
		SessionParams: sessionMetaDataParams,
		AuthMethod:    domain.AuthMethodPassword,
		Provider:      string(params.Provider),
		OAuthToken:    nil,
	}
	sessionID, err := am.sessionManager.CreateSession(ctx, user.FormatID(), sessionParams)
	if err != nil {
		am.logger.Error("Failed to create session", log.Error(err))
		if err := am.userService.Delete(ctx, user.FormatID()); err != nil {
			am.logError(ctx, "Failed to delete user after session creation failed", err)
		}
		return nil, "", err
	}

	tokenPair, err := am.tokenManager.GenerateTokenPair(user.FormatID(), sessionID, params.Provider, domain.RoleMap{}, nil)
	if err != nil {
		return nil, "", err
	}

	return tokenPair, sessionID, nil
}

// AuthenticateWithPassword authenticates a user using email and password.
// Returns the token pair and session ID, or an error if the authentication fails.
func (am *AuthManager) AuthenticateWithPassword(ctx context.Context, email, password string, sessionMetaDataParams *domain.SessionMetaDataParams) (*domain.AuthResponse, error) {

	user, err := am.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	valid, err := user.ValidatePasswordHash(password)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, domain.ErrInvalidCredentials
	}

	roles := user.GetRoles()
	sessionParams := &domain.SessionCreateParams{
		UserID:        user.FormatID(),
		Roles:         roles,
		SessionParams: sessionMetaDataParams,
		AuthMethod:    domain.AuthMethodPassword,
		Provider:      string(domain.AuthProviderPassword),
		OAuthToken:    nil,
	}
	sessionID, err := am.sessionManager.CreateSession(ctx, user.FormatID(), sessionParams)
	if err != nil {
		return nil, err
	}

	tokenPair, err := am.tokenManager.GenerateTokenPair(user.FormatID(), sessionID, domain.AuthProviderPassword, roles, nil)
	if err != nil {
		return nil, err
	}

	resp := &domain.AuthResponse{
		TokenPair: tokenPair,
		SessionID: sessionID,
	}

	return resp, nil
}

// ValidateAccessToken validates an access token and retrieves the associated user.
func (am *AuthManager) ValidateAccessToken(ctx context.Context, tokenString string) (*domain.UserContextData, error) {
	claims, err := am.tokenManager.ValidateToken(tokenString)
	if err != nil {
		if errors.Is(err, domain.ErrTokenExpired) {
			return nil, domain.ErrTokenExpired
		}
		return nil, domain.ErrInvalidToken
	}

	// Validate session exists and is active
	session, err := am.sessionManager.GetSession(ctx, claims.SessionID, claims.Subject)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return nil, domain.ErrInvalidSession
		}
		return nil, fmt.Errorf("failed to validate session: %w", err)
	}

	if session.Revoked {
		return nil, domain.ErrTokenRevoked
	}

	return claims.GetUserContextData(), nil
}

// RefreshTokens refreshes the access and refresh tokens.
// todo: add support for social providers
// todo: add checks against values stored in context
func (am *AuthManager) RefreshTokens(ctx context.Context, refreshToken string, sessionMetaDataParams *domain.SessionMetaDataParams) (*domain.AuthResponse, error) {

	// Validate refresh token
	claims, err := am.tokenManager.ValidateToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}
	if claims.TokenType != domain.TypeRefreshToken {
		return nil, domain.ErrInvalidToken
	}

	// Get user and session
	user, err := am.GetUserByID(ctx, claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	session, err := am.sessionManager.GetSession(ctx, claims.SessionID, claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// If this is an OAuth session, refresh the OAuth token
	var newOAuthToken *domain.OAuthToken
	if session.AuthMethod == domain.AuthMethodOAuth {
		newOAuthToken, err = am.refreshOAuthToken(ctx, session.Provider, session.OAuthToken)
		if err != nil {
			am.Logout(ctx, claims.Subject, claims.SessionID)
			return nil, fmt.Errorf("failed to refresh OAuth token: %w", err)
		}

		// Update session with new OAuth token
		sessionParams := &domain.SessionUpdateParams{
			UserID:        claims.Subject,
			SessionParams: sessionMetaDataParams,
			AuthMethod:    session.AuthMethod,
			Provider:      session.Provider,
			OAuthToken:    newOAuthToken,
		}
		if err := am.sessionManager.UpdateSession(ctx, claims.Subject, claims.SessionID, sessionParams); err != nil {
			return nil, fmt.Errorf("failed to update session: %w", err)
		}
	}

	// Generate new token pair
	tokenPair, err := am.tokenManager.GenerateTokenPair(
		user.FormatID(),
		claims.SessionID,
		domain.AuthProvider(session.Provider),
		user.GetRoles(),
		newOAuthToken,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	resp := &domain.AuthResponse{
		TokenPair: tokenPair,
		SessionID: claims.SessionID,
	}

	return resp, nil
}

func (am *AuthManager) refreshOAuthToken(ctx context.Context, provider string, token *domain.OAuthToken) (*domain.OAuthToken, error) {
	oauthToken, err := am.oauthManager.RefreshToken(ctx, provider, token.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh OAuth token: %w", err)
	}

	return &domain.OAuthToken{
		AccessToken:  oauthToken.AccessToken,
		RefreshToken: oauthToken.RefreshToken,
		Expiry:       oauthToken.Expiry,
	}, nil
}

// RevokeTokens revokes all refresh tokens and sessions associated with the user.
func (am *AuthManager) RevokeAllSessions(ctx context.Context, userID string) error {

	err := am.sessionManager.RevokeAllSessions(ctx, userID)
	if err != nil {
		am.logger.Error("Failed to revoke user sessions", log.Error(err))
		return fmt.Errorf("failed to revoke user sessions: %w", err)
	}

	return nil

}

func (am *AuthManager) logError(ctx context.Context, operation string, err error) {
	userID, _ := ctx.Value("userID").(string)
	am.logger.Error("Authentication error",
		log.String("operation", operation),
		log.String("userID", userID),
		log.Error(err),
	)
}

// Add detailed error logging
func (am *AuthManager) logErrorWithStackTrace(ctx context.Context, operation string, err error) {
	userID, _ := ctx.Value("userID").(string)
	am.logger.Error("Authentication error",
		log.String("operation", operation),
		log.String("userID", userID),
		log.Error(err),
		log.Stack("stack"),
	)
}

// Add error wrapping
func (am *AuthManager) wrapError(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

func (am *AuthManager) GetTokenManager() *domain.TokenManager {
	return am.tokenManager
}

// Add OAuth methods to AuthManager
// returns the auth URL, state, and error
func (am *AuthManager) OAuthGetLoginURL(ctx context.Context, provider string) (string, string, error) {
	return am.oauthManager.GenerateAuthURL(ctx, provider)
}

func (am *AuthManager) OAuthCallback(ctx context.Context, provider, code, state string, sessionMetaDataParams *domain.SessionMetaDataParams) (*domain.TokenPair, string, error) {
	// Exchange code for OAuth token
	oauthToken, err := am.oauthManager.ExchangeCode(ctx, provider, code, state)
	if err != nil {
		return nil, "", err
	}

	// Get or create user based on OAuth provider data
	user, err := am.getOrCreateUserFromOAuth(ctx, provider, oauthToken)
	if err != nil {
		return nil, "", err
	}

	sessionParams := &domain.SessionCreateParams{
		UserID:        user.FormatID(),
		Roles:         user.GetRoles(),
		SessionParams: sessionMetaDataParams,
		AuthMethod:    domain.AuthMethodOAuth,
		Provider:      string(provider),
		OAuthToken: &domain.OAuthToken{
			AccessToken:  oauthToken.AccessToken,
			RefreshToken: oauthToken.RefreshToken,
			Expiry:       oauthToken.Expiry,
		},
	}

	sessionID, err := am.sessionManager.CreateSession(ctx, user.FormatID(), sessionParams)
	if err != nil {
		return nil, "", err
	}

	tokenPair, err := am.tokenManager.GenerateTokenPair(
		user.FormatID(),
		sessionID,
		domain.AuthProvider(provider),
		user.GetRoles(),
		&domain.OAuthToken{
			AccessToken:  oauthToken.AccessToken,
			RefreshToken: oauthToken.RefreshToken,
			Expiry:       oauthToken.Expiry,
		},
	)
	if err != nil {
		return nil, "", err
	}

	return tokenPair, sessionID, nil
}

// Helper method to get or create user from OAuth data
func (am *AuthManager) getOrCreateUserFromOAuth(ctx context.Context, provider string, token *oauth2.Token) (*domain.User, error) {
	// Get user info from OAuth provider
	userInfo, err := am.oauthManager.GetUserInfo(ctx, provider, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info from provider: %w", err)
	}

	// Try to find existing user by provider ID
	user, err := am.userService.GetUserByProviderUserID(ctx, provider, userInfo.UserID)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get user by provider user ID: %w", err)
	}

	providerUserID := userInfo.UserID

	// If user doesn't exist, create new user
	createParams := &domain.UserCreateParams{
		Email:           userInfo.Email,
		Provider:        domain.AuthProvider(provider),
		ProviderUserID:  &providerUserID,
		FirstName:       userInfo.FirstName,
		LastName:        userInfo.LastName,
		AvatarURL:       userInfo.AvatarURL,
		ProviderRawData: userInfo.RawData,
	}

	user, err = am.userService.Create(ctx, createParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// func (am *AuthManager) createSession(ctx context.Context, params *domain.SessionCreationParams) (string, error) {
// 	sessionMeta := &domain.SessionMetaData{
// 		SessionID:         util.GenerateUUID(),
// 		IP:                params.SessionParams.IP,
// 		UserAgent:         params.SessionParams.UserAgent,
// 		DeviceFingerprint: params.SessionParams.DeviceFingerprint,
// 		Location:          params.SessionParams.Location,
// 		CreatedAt:         time.Now(),
// 		LastLogin:         time.Now(),
// 		Roles:             params.Roles,
// 		AuthMethod:        params.AuthMethod,
// 		Provider:          params.Provider,
// 		OAuthToken:        params.OAuthToken,
// 	}

// 	return am.sessionManager.CreateSession(ctx, params.User.FormatID(), sessionMeta)
// }

// // AuthenticationMiddleware is an example middleware that validates the access token and sets user context.
// func (am *AuthManager) AuthenticationMiddleware(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// Example: extract token from Authorization header (expects format 'Bearer <token>')
// 		authHeader := r.Header.Get("Authorization")
// 		if authHeader == "" {
// 			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
// 			return
// 		}

// 		// Normally, you would split the header and validate the token; here we assume the token is provided directly.
// 		token := authHeader
// 		userCtx, err := am.ValidateAccessToken(r.Context(), token)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusUnauthorized)
// 			return
// 		}
// 		// Store the user context in the request context
// 		ctx := context.WithValue(r.Context(), "user", userCtx)
// 		next.ServeHTTP(w, r.WithContext(ctx))
// 	})
// }

// // CombinedMiddleware chains the authentication middleware with the authz agent's authorization middleware.
// // It first ensures the request is authenticated, then enforces authorization based on the provided policy.
// func (am *AuthManager) CombinedMiddleware(next http.Handler) http.Handler {
// 	// Define an input extractor function for the authz middleware.
// 	inputExtractor := func(r *http.Request) (any, error) {
// 		// Expect that the AuthenticationMiddleware has set a "user" value in the context
// 		userCtx := r.Context().Value("user")
// 		if userCtx == nil {
// 			return nil, fmt.Errorf("user not found in context")
// 		}
// 		// Build the input for policy evaluation, e.g., embedding the user context
// 		input := map[string]any{
// 			"user": userCtx,
// 			// Additional fields can be added here
// 		}
// 		return input, nil
// 	}

// 	// Chain the middlewares: first authenticate, then authorize.
// 	return am.authZAgent.Middleware(inputExtractor)(am.AuthenticationMiddleware(next))
// }

// Example usage in a router setup (using Chi router):
// func main() {
// 	r := chi.NewRouter()
// 	// Chain the combined middleware for protected endpoints
// 	r.With(authManager.CombinedMiddleware).Get("/protected", protectedHandler)
// 	http.ListenAndServe(":8080", r)
// }
