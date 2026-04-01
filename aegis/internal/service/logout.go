package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/util"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
)

// Logout logs the user out by invalidating the session, performing
// any necessary provider-specific cleanup, and publishing a token
// revocation event so downstream consumers (e.g. Prism) can reject
// the token immediately.
func (am *AuthManager) Logout(ctx context.Context, userID, sessionID string) error {
	// Validate input parameters
	if !util.HasUserPrefix(userID) {
		return domain.ErrInvalidUserID
	}
	if sessionID == "" {
		return domain.ErrInvalidSessionID
	}

	// Retrieve session details to determine provider information (including any OAuth token)
	session, err := am.sessionManager.GetSession(ctx, sessionID, userID)
	if err != nil {
		return am.wrapError(err, "failed to retrieve session")
	}

	// Invalidate the user session (revoke all associated tokens)
	if err := am.sessionManager.InvalidateUserSession(ctx, userID, sessionID); err != nil {
		return am.wrapError(err, "failed to revoke tokens for session")
	}

	// Publish token revocation to the gRPC event bus so that downstream
	// services (Prism) can reject the token before it naturally expires.
	// The JTI and ExpiresAt come from the UserContextData that the auth
	// middleware placed on the context during request validation.
	if am.tokenRevoker != nil {
		if usrCtx, ok := domain.UserContextDataFromContext(ctx); ok && usrCtx.JTI != "" {
			am.tokenRevoker.PublishTokenRevoked(usrCtx.JTI, usrCtx.ExpiresAt)
			am.logger.Info("Published token revocation event",
				log.String("jti", usrCtx.JTI),
				log.String("userID", userID))
		}
	}

	// Perform provider-specific logout if required based on the session's provider and OAuth token (if available)
	if err := am.providerLogout(ctx, session.Provider, userID, session.OAuthToken); err != nil {
		return am.wrapError(err, "failed to perform provider-specific logout")
	}

	am.logger.Info("User logged out successfully", log.String("userID", userID))
	return nil
}

// providerLogout determines which provider-specific logout call should be made.
func (am *AuthManager) providerLogout(ctx context.Context, provider, userID string, oauthToken *domain.OAuthToken) error {
	switch provider {
	case "facebook":
		return am.facebookLogout(ctx, userID, oauthToken)
	case "google":
		return am.googleLogout(ctx, userID, oauthToken)
	case "github":
		return am.githubLogout(ctx, userID, oauthToken)
	case "twitter":
		return am.twitterLogout(ctx, userID, oauthToken)
	default:
		am.logger.Info("No provider-specific logout required",
			log.String("provider", provider),
			log.String("userID", userID))
		return nil
	}
}

// facebookLogout handles Facebook-specific logout logic.
func (am *AuthManager) facebookLogout(ctx context.Context, userID string, oauthToken *domain.OAuthToken) error {
	if oauthToken == nil {
		am.logger.Info("No OAuth token provided for Facebook logout", log.String("userID", userID))
		return nil
	}

	// Facebook revokes permissions by sending a DELETE to the Graph API
	endpoint := "https://graph.facebook.com/me/permissions?access_token=" + url.QueryEscape(oauthToken.AccessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("facebook revoke token failed: %s", body)
	}

	return nil
}

// googleLogout handles Google-specific logout logic.
func (am *AuthManager) googleLogout(ctx context.Context, userID string, oauthToken *domain.OAuthToken) error {
	if oauthToken == nil {
		am.logger.Info("No OAuth token provided for Google logout", log.String("userID", userID))
		return nil
	}

	// Google token revocation endpoint
	data := url.Values{}
	data.Set("token", oauthToken.AccessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/revoke", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("google revoke token failed: %s", body)
	}

	return nil
}

// twitterLogout handles Twitter-specific logout logic.
func (am *AuthManager) twitterLogout(ctx context.Context, userID string, oauthToken *domain.OAuthToken) error {
	if oauthToken == nil {
		am.logger.Info("No OAuth token provided for Twitter logout", log.String("userID", userID))
		return nil
	}

	// Assuming Twitter OAuth2 revocation endpoint is used.
	endpoint := "https://api.twitter.com/2/oauth2/revoke"
	data := url.Values{}
	data.Set("token", oauthToken.AccessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("twitter revoke token failed: %s", body)
	}

	return nil
}

// githubLogout handles GitHub-specific logout logic.
func (am *AuthManager) githubLogout(ctx context.Context, userID string, oauthToken *domain.OAuthToken) error {
	if oauthToken == nil {
		am.logger.Info("No OAuth token provided for GitHub logout", log.String("userID", userID))
		return nil
	}

	// Retrieve GitHub provider configuration from the OAuth manager.
	providerConfig, err := am.oauthManager.GetProviderConfig("github")
	if err != nil {
		am.logger.Info("GitHub provider configuration not found, skipping token revocation", log.String("userID", userID))
		return nil
	}

	clientID := providerConfig.ClientID
	clientSecret := providerConfig.ClientSecret
	if clientID == "" || clientSecret == "" {
		am.logger.Info("GitHub client credentials not set, cannot revoke token", log.String("userID", userID))
		return nil
	}

	// GitHub token revocation endpoint
	endpoint := "https://api.github.com/applications/" + clientID + "/token"
	payload := fmt.Sprintf(`{"access_token": "%s"}`, oauthToken.AccessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Set HTTP Basic Authentication header using clientID and clientSecret.
	authStr := clientID + ":" + clientSecret
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authStr))
	req.Header.Set("Authorization", "Basic "+encodedAuth)

	// GitHub returns 204 No Content upon successful revocation.
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("github revoke token failed: %s", body)
	}

	return nil
}
