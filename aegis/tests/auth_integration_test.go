package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasonKoogler/abraxis/tests/testutil"
	"github.com/stretchr/testify/require"
)

func TestAuthFlows(t *testing.T) {
	// Resolve absolute path to migrations directory for the Docker-based migrate runner.
	cwd, err := os.Getwd()
	require.NoError(t, err)
	migrationsPath := filepath.Join(cwd, "..", "deploy", "migrations")
	absPath, err := filepath.Abs(migrationsPath)
	require.NoError(t, err)

	pg := testutil.SetupPostgres(t, absPath)
	rd := testutil.SetupRedis(t)
	server := StartAegisServer(t, pg, rd)

	// Shared state across subtests (sequential execution).
	var accessToken string
	var refreshToken string

	t.Run("register_user", func(t *testing.T) {
		body := map[string]string{
			"email":     "testuser@example.com",
			"password":  "SecurePass123!",
			"firstName": "Test",
			"lastName":  "User",
			"phone":     "+1234567890",
		}
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err)

		resp, err := http.Post(server.BaseURL+"/auth/register", "application/json", bytes.NewReader(jsonBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode, "register should return 201")

		authHeader := resp.Header.Get("Authorization")
		require.NotEmpty(t, authHeader, "Authorization header must be set after registration")
		require.Contains(t, authHeader, "Bearer ", "Authorization header must contain Bearer token")

		xRefresh := resp.Header.Get("X-Refresh-Token")
		require.NotEmpty(t, xRefresh, "X-Refresh-Token header must be set after registration")

		xSession := resp.Header.Get("X-Session-ID")
		require.NotEmpty(t, xSession, "X-Session-ID header must be set after registration")

		// Save tokens for subsequent tests — strip "Bearer " prefix from access token.
		accessToken = authHeader[len("Bearer "):]
		refreshToken = xRefresh
	})

	t.Run("login_with_password", func(t *testing.T) {
		body := map[string]string{
			"email":    "testuser@example.com",
			"password": "SecurePass123!",
		}
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err)

		resp, err := http.Post(server.BaseURL+"/auth/login", "application/json", bytes.NewReader(jsonBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode, "login should return 200")

		authHeader := resp.Header.Get("Authorization")
		require.NotEmpty(t, authHeader, "Authorization header must be set after login")
		require.Contains(t, authHeader, "Bearer ", "Authorization header must contain Bearer token")

		xRefresh := resp.Header.Get("X-Refresh-Token")
		require.NotEmpty(t, xRefresh, "X-Refresh-Token header must be set after login")

		xSession := resp.Header.Get("X-Session-ID")
		require.NotEmpty(t, xSession, "X-Session-ID header must be set after login")

		// Update tokens with the ones from login.
		accessToken = authHeader[len("Bearer "):]
		refreshToken = xRefresh
	})

	t.Run("access_protected_endpoint", func(t *testing.T) {
		// With a valid token -> 200.
		req, err := http.NewRequest(http.MethodGet, server.BaseURL+"/users", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+accessToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode, "GET /users with valid token should return 200")

		// Without a token -> 401.
		reqNoAuth, err := http.NewRequest(http.MethodGet, server.BaseURL+"/users", nil)
		require.NoError(t, err)

		respNoAuth, err := http.DefaultClient.Do(reqNoAuth)
		require.NoError(t, err)
		defer respNoAuth.Body.Close()

		require.Equal(t, http.StatusUnauthorized, respNoAuth.StatusCode, "GET /users without token should return 401")
	})

	t.Run("refresh_token", func(t *testing.T) {
		// The refresh handler extracts the token from the Authorization header via
		// domain.ExtractTokenFromHeader, so we send the refresh token as a Bearer token.
		req, err := http.NewRequest(http.MethodPost, server.BaseURL+"/auth/refresh", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+refreshToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode, "refresh should return 200")

		// The refresh handler returns an AuthResponse as JSON: {"TokenPair":{"access_token":"...","refresh_token":"..."},"SessionID":"..."}
		var authResp struct {
			TokenPair struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
			} `json:"TokenPair"`
			SessionID string `json:"SessionID"`
		}
		err = json.NewDecoder(resp.Body).Decode(&authResp)
		require.NoError(t, err)
		require.NotEmpty(t, authResp.TokenPair.AccessToken, "new access token must be non-empty")
		require.NotEmpty(t, authResp.TokenPair.RefreshToken, "new refresh token must be non-empty")

		// Update tokens for subsequent tests.
		accessToken = authResp.TokenPair.AccessToken
		refreshToken = authResp.TokenPair.RefreshToken
	})

	t.Run("logout_invalidates_session", func(t *testing.T) {
		// Logout requires authentication (the auth middleware wraps the logout route).
		req, err := http.NewRequest(http.MethodPost, server.BaseURL+"/auth/logout", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+accessToken)

		// Use a client that does NOT follow redirects, so we can inspect the 302.
		noRedirectClient := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		resp, err := noRedirectClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusFound, resp.StatusCode, "logout should return 302 redirect")

		// After logout the session should be invalidated — using the same access token
		// should now be rejected.
		reqAfter, err := http.NewRequest(http.MethodGet, server.BaseURL+"/users", nil)
		require.NoError(t, err)
		reqAfter.Header.Set("Authorization", "Bearer "+accessToken)

		respAfter, err := http.DefaultClient.Do(reqAfter)
		require.NoError(t, err)
		defer respAfter.Body.Close()

		require.Equal(t, http.StatusUnauthorized, respAfter.StatusCode,
			"accessing protected endpoint after logout should return 401")
	})
}
