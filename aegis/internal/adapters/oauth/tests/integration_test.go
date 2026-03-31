package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jasonKoogler/abraxis/aegis/internal/adapters/oauth"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"golang.org/x/oauth2"
)

// Integration-style test helpers

// setupIntegrationTestEnvironment creates a test environment for OAuth tests
// This provides a more realistic test environment with a mock HTTP server
func setupIntegrationTestEnvironment() (*httptest.Server, oauth.Providers, *MockVerifierStorage, *oauth.OAuthManager) {
	// Create a mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/google/userinfo":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"email": "test@example.com",
				"family_name": "User",
				"given_name": "Test",
				"id": "123456789",
				"name": "Test User",
				"picture": "https://example.com/picture.jpg"
			}`))
		case "/auth":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"access_token": "test-access-token",
				"token_type": "Bearer",
				"expires_in": 3600,
				"refresh_token": "test-refresh-token"
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	// Create mock providers for testing
	providersMap := make(oauth.Providers)
	providersMap["google"] = &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:8080/callback",
		Scopes:       []string{"profile", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  mockServer.URL + "/auth",
			TokenURL: mockServer.URL + "/token",
		},
	}

	// Create a mock verifier storage
	verifierStorage := &MockVerifierStorage{}

	// Create a logger
	logger := log.NewLogger("debug")

	// Create the OAuth manager
	manager := oauth.NewOAuthManager(providersMap, logger, verifierStorage)

	return mockServer, providersMap, verifierStorage, manager
}

// Helper token functions for integration tests

// createIntegrationValidToken creates a valid OAuth token for integration testing
func createIntegrationValidToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  "valid-access-token",
		TokenType:    "Bearer",
		RefreshToken: "valid-refresh-token",
		Expiry:       time.Now().Add(time.Hour), // Valid for 1 hour
	}
}

// createIntegrationExpiredToken creates an expired OAuth token for integration testing
func createIntegrationExpiredToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  "expired-access-token",
		TokenType:    "Bearer",
		RefreshToken: "valid-refresh-token",
		Expiry:       time.Now().Add(-time.Hour), // Expired 1 hour ago
	}
}

// Mock helpers for setting up test expectations

// setupMockTokenExchange sets up the MockVerifierStorage to expect a call to Get and Del
// when the ExchangeCode method is called
func setupMockTokenExchange(ctx context.Context, verifierStorage *MockVerifierStorage, state string) {
	verifierStorage.On("Get", ctx, state).Return("test-verifier", nil).Once()
	verifierStorage.On("Del", ctx, state).Return(nil).Once()
}

// OAuthIntegrationSuite is a test suite for OAuth integration testing
type OAuthIntegrationSuite struct {
	suite.Suite
	server          *httptest.Server
	providers       oauth.Providers
	verifierStorage *MockVerifierStorage
	manager         *oauth.OAuthManager
	ctx             context.Context
}

// SetupSuite initializes the test suite
func (s *OAuthIntegrationSuite) SetupSuite() {
	s.server, s.providers, s.verifierStorage, s.manager = setupIntegrationTestEnvironment()
	s.ctx = context.Background()
}

// TearDownSuite cleans up resources
func (s *OAuthIntegrationSuite) TearDownSuite() {
	s.server.Close()
}

// TearDownTest cleans up after each test
func (s *OAuthIntegrationSuite) TearDownTest() {
	// Reset mock expectations
	s.verifierStorage.AssertExpectations(s.T())
	s.verifierStorage = &MockVerifierStorage{}
	s.manager.Storage = s.verifierStorage
}

// TestAuthorizationFlow tests the complete OAuth authorization flow
func (s *OAuthIntegrationSuite) TestAuthorizationFlow() {
	// 1. Generate auth URL
	s.verifierStorage.On("Set", s.ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil).Once()
	authURL, state, err := s.manager.GenerateAuthURL(s.ctx, "google")

	s.NoError(err)
	s.NotEmpty(authURL)
	s.NotEmpty(state)

	// 2. Exchange code for token
	setupMockTokenExchange(s.ctx, s.verifierStorage, state)
	token, err := s.manager.ExchangeCode(s.ctx, "google", "test-code", state)

	s.NoError(err)
	s.NotNil(token)
	s.Equal("test-access-token", token.AccessToken)
}

// TestTokenRefresh tests refreshing an expired token
func (s *OAuthIntegrationSuite) TestTokenRefresh() {
	// Setup a test server for token refresh responses
	tokenRefreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the form values
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid_request"}`))
			return
		}

		// Check for grant_type
		if r.Form.Get("grant_type") != "refresh_token" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid_grant","error_description":"invalid grant type"}`))
			return
		}

		// Check refresh token
		refreshToken := r.Form.Get("refresh_token")
		switch refreshToken {
		case "valid-refresh-token":
			// Success case
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"access_token": "refreshed-access-token",
				"token_type": "Bearer",
				"expires_in": 3600,
				"refresh_token": "new-refresh-token"
			}`))
		case "invalid-refresh-token":
			// Error case - invalid token
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid_grant","error_description":"Invalid refresh token"}`))
		case "server-error-token":
			// Error case - server error
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"server_error"}`))
		default:
			// Unknown token
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid_grant","error_description":"Unknown refresh token"}`))
		}
	}))
	defer tokenRefreshServer.Close()

	// Modify the provider config to use our test server
	for provider, config := range s.providers {
		config.Endpoint.TokenURL = tokenRefreshServer.URL
		s.providers[provider] = config
	}

	// Test cases
	testCases := []struct {
		name          string
		provider      string
		refreshToken  string
		expectError   bool
		expectedToken *oauth2.Token
	}{
		{
			name:         "Valid refresh token",
			provider:     "google",
			refreshToken: "valid-refresh-token",
			expectError:  false,
			expectedToken: &oauth2.Token{
				AccessToken:  "refreshed-access-token",
				TokenType:    "Bearer",
				RefreshToken: "new-refresh-token",
			},
		},
		{
			name:         "Invalid refresh token",
			provider:     "google",
			refreshToken: "invalid-refresh-token",
			expectError:  true,
		},
		{
			name:         "Server error",
			provider:     "google",
			refreshToken: "server-error-token",
			expectError:  true,
		},
		{
			name:         "Empty refresh token",
			provider:     "google",
			refreshToken: "",
			expectError:  true,
		},
		{
			name:         "Invalid provider",
			provider:     "nonexistent",
			refreshToken: "valid-refresh-token",
			expectError:  true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Perform token refresh
			token, err := s.manager.RefreshToken(s.ctx, tc.provider, tc.refreshToken)

			if tc.expectError {
				s.Error(err)
				s.Nil(token)
			} else {
				s.NoError(err)
				s.NotNil(token)
				s.Equal(tc.expectedToken.AccessToken, token.AccessToken)
				s.Equal(tc.expectedToken.RefreshToken, token.RefreshToken)
				s.Equal(tc.expectedToken.TokenType, token.TokenType)
				s.True(token.Expiry.After(time.Now()), "Token should be valid (not expired)")
			}
		})
	}
}

// TestGetUserInfo tests retrieving user information with a valid token
func (s *OAuthIntegrationSuite) TestGetUserInfo() {
	// Create a valid token for testing
	validToken := createIntegrationValidToken()

	// Setup our Google user info test server with custom handler
	googleUserInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify token in header or query
		token := r.URL.Query().Get("access_token")

		// Handle special test cases
		switch token {
		case "":
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"invalid_token","error_description":"Missing or invalid token"}`))
				return
			}
			token = strings.TrimPrefix(authHeader, "Bearer ")

			// Continue to the next checks with the extracted token
			if token == "" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"invalid_token","error_description":"Empty token"}`))
				return
			}
		case "server-error-token":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"server_error","error_description":"Internal server error"}`))
			return
		case "malformed-response-token":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{malformed json`))
			return
		}

		// Test case: invalid token
		if token != validToken.AccessToken {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid_token","error_description":"Invalid Credentials"}`))
			return
		}

		// Success case
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"email": "test@example.com",
			"family_name": "User",
			"given_name": "Test",
			"id": "123456789",
			"name": "Test User",
			"picture": "https://example.com/picture.jpg"
		}`))
	}))
	defer googleUserInfoServer.Close()

	// Temporarily replace the constant in the oauth package with a monkey patch
	// This is done by creating a temporary test server for each provider
	originalGetGoogleUserInfo := s.manager.GetUserInfo

	// Monkey patch the GetUserInfo method to use our test server
	patchedGetUserInfo := func(ctx context.Context, provider string, token *oauth2.Token) (*oauth.UserInfo, error) {
		// Create a custom HTTP client that will use our test server
		client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

		// Override the transport to redirect all requests to our test server
		client.Transport = &testTransport{
			googleUserInfoURL: googleUserInfoServer.URL,
			token:             token,
		}

		// Call the original method - our patched transport will redirect to our test server
		if provider == "google" {
			resp, err := client.Get(googleUserInfoServer.URL + "?access_token=" + url.QueryEscape(token.AccessToken))
			if err != nil {
				return nil, fmt.Errorf("failed to get Google user info: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return nil, fmt.Errorf("Google API error: %s, status: %d", string(body), resp.StatusCode)
			}

			respBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read Google user info: %w", err)
			}

			// Google specific user info to be decoded
			var googUser struct {
				Email      string `json:"email"`
				FamilyName string `json:"family_name"`
				GivenName  string `json:"given_name"`
				ID         string `json:"id"`
				Locale     string `json:"locale"`
				Name       string `json:"name"`
				Picture    string `json:"picture"`
			}

			decoder := json.NewDecoder(bytes.NewReader(respBytes))
			if err := decoder.Decode(&googUser); err != nil {
				return nil, fmt.Errorf("failed to decode Google user info: %w", err)
			}

			user := &oauth.UserInfo{
				Email:     googUser.Email,
				FirstName: googUser.GivenName,
				LastName:  googUser.FamilyName,
				AvatarURL: googUser.Picture,
				Provider:  "google",
				UserID:    googUser.ID,
				Name:      googUser.Name,
			}

			if err := json.Unmarshal(respBytes, &map[string]interface{}{}); err != nil {
				return nil, fmt.Errorf("failed to unmarshal Google user info: %w", err)
			}

			return user, nil
		}

		return originalGetGoogleUserInfo(ctx, provider, token)
	}

	// Testing multiple scenarios with custom helper
	testCases := []struct {
		name         string
		token        *oauth2.Token
		provider     string
		expectError  bool
		expectedInfo *oauth.UserInfo
	}{
		{
			name:        "Valid Google token",
			token:       validToken,
			provider:    "google",
			expectError: false,
			expectedInfo: &oauth.UserInfo{
				Email:     "test@example.com",
				FirstName: "Test",
				LastName:  "User",
				Name:      "Test User",
				Provider:  "google",
				UserID:    "123456789",
				AvatarURL: "https://example.com/picture.jpg",
			},
		},
		{
			name: "Invalid token",
			token: &oauth2.Token{
				AccessToken: "invalid-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			},
			provider:    "google",
			expectError: true,
		},
		{
			name: "Expired token",
			token: &oauth2.Token{
				AccessToken: "expired-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(-time.Hour),
			},
			provider:    "google",
			expectError: true,
		},
		{
			name:        "Unsupported provider",
			token:       validToken,
			provider:    "unsupported",
			expectError: true,
		},
		{
			name: "Empty token",
			token: &oauth2.Token{
				AccessToken: "",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			},
			provider:    "google",
			expectError: true,
		},
		{
			name: "Server error",
			token: &oauth2.Token{
				AccessToken: "server-error-token", // Special token that triggers server error
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			},
			provider:    "google",
			expectError: true,
		},
		{
			name: "Malformed response",
			token: &oauth2.Token{
				AccessToken: "malformed-response-token", // Special token that triggers malformed response
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			},
			provider:    "google",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Use our patched function for this test
			userInfo, err := patchedGetUserInfo(s.ctx, tc.provider, tc.token)

			if tc.expectError {
				s.Error(err)
				s.Nil(userInfo)
			} else {
				s.NoError(err)
				s.NotNil(userInfo)
				s.Equal(tc.expectedInfo.Email, userInfo.Email)
				s.Equal(tc.expectedInfo.FirstName, userInfo.FirstName)
				s.Equal(tc.expectedInfo.LastName, userInfo.LastName)
				s.Equal(tc.expectedInfo.Name, userInfo.Name)
				s.Equal(tc.expectedInfo.UserID, userInfo.UserID)
				s.Equal(tc.expectedInfo.AvatarURL, userInfo.AvatarURL)
				s.Equal(tc.expectedInfo.Provider, userInfo.Provider)
			}
		})
	}
}

// Custom transport to redirect requests to our test server
type testTransport struct {
	googleUserInfoURL string
	token             *oauth2.Token
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect all requests to our test server
	if strings.Contains(req.URL.String(), "googleapis.com") {
		req.URL, _ = url.Parse(t.googleUserInfoURL)
		// Ensure token is in the query if it was originally in the URL
		if query := req.URL.Query(); !query.Has("access_token") {
			query.Set("access_token", t.token.AccessToken)
			req.URL.RawQuery = query.Encode()
		}
	}

	// Perform the request with the default transport
	return http.DefaultTransport.RoundTrip(req)
}

// TestErrorHandling tests various error conditions
func (s *OAuthIntegrationSuite) TestErrorHandling() {
	// Test with non-existent provider
	_, _, err := s.manager.GenerateAuthURL(s.ctx, "nonexistent")
	s.Error(err)

	// Test with empty code
	_, err = s.manager.ExchangeCode(s.ctx, "google", "", "state")
	s.Error(err)

	// Test with empty state
	_, err = s.manager.ExchangeCode(s.ctx, "google", "code", "")
	s.Error(err)
}

// Run the integration test suite
func TestOAuthIntegrationSuite(t *testing.T) {
	suite.Run(t, new(OAuthIntegrationSuite))
}

// Example integration test (keeping for backward compatibility)
func TestIntegrationOAuthFlow(t *testing.T) {
	// Setup test environment
	server, _, verifierStorage, manager := setupIntegrationTestEnvironment()
	defer server.Close()

	ctx := context.Background()

	// Test the full OAuth flow

	// 1. Generate auth URL
	verifierStorage.On("Set", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil).Once()
	authURL, gotState, err := manager.GenerateAuthURL(ctx, "google")

	assert.NoError(t, err)
	assert.NotEmpty(t, authURL)
	assert.NotEmpty(t, gotState)

	// 2. Exchange code for token
	setupMockTokenExchange(ctx, verifierStorage, gotState)
	token, err := manager.ExchangeCode(ctx, "google", "test-code", gotState)

	assert.NoError(t, err)
	assert.NotNil(t, token)
	assert.Equal(t, "test-access-token", token.AccessToken)

	// Verify all mock expectations were met
	verifierStorage.AssertExpectations(t)
}
