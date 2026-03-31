package tests

import (
	"context"
	"testing"

	"github.com/jasonKoogler/aegis/internal/adapters/oauth"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/oauth2"
)

// Mocks are now defined in mocks_test.go

func TestNewOAuthManager(t *testing.T) {
	// Create mocks
	mockProviders := NewMockProviders()
	mockStorage := &MockVerifierStorage{}
	logger := log.NewLogger("debug")

	// Create OAuthManager
	manager := oauth.NewOAuthManager(mockProviders.AsProviders(), logger, mockStorage)

	// Assert manager is created with the expected values
	assert.NotNil(t, manager)
	assert.Equal(t, mockProviders.AsProviders(), manager.Providers)
	assert.Equal(t, mockStorage, manager.Storage)
}

func TestGenerateAuthURL(t *testing.T) {
	ctx := context.Background()
	mockProviders := NewMockProviders()
	mockStorage := &MockVerifierStorage{}
	logger := log.NewLogger("debug")

	// This is a different approach - we need to populate the mock data first
	// before creating the manager, so that the manager sees the data in its providers map

	testCases := []struct {
		name          string
		providerName  string
		setupMocks    func()
		expectedError bool
	}{
		{
			name:          "Empty provider name",
			providerName:  "",
			setupMocks:    func() {},
			expectedError: true,
		},
		{
			name:         "Provider not found",
			providerName: "unknown",
			setupMocks: func() {
				mockProviders.On("Get", "unknown").Return(nil, oauth.ErrProviderNotFound).Once()
				// Create a dummy config for other providers to avoid nil map issues
				mockProviders.On("Get", "google").Return(&oauth2.Config{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"profile", "email"},
					Endpoint:     oauth2.Endpoint{AuthURL: "https://example.com/auth"},
				}, nil).Maybe()
			},
			expectedError: true,
		},
		{
			name:         "Success",
			providerName: "google",
			setupMocks: func() {
				config := &oauth2.Config{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"profile", "email"},
					Endpoint:     oauth2.Endpoint{AuthURL: "https://example.com/auth"},
				}
				mockProviders.On("Get", "google").Return(config, nil).Once()
				mockStorage.On("Set", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil).Once()
			},
			expectedError: false,
		},
		{
			name:         "Storage error",
			providerName: "google",
			setupMocks: func() {
				config := &oauth2.Config{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"profile", "email"},
					Endpoint:     oauth2.Endpoint{AuthURL: "https://example.com/auth"},
				}
				mockProviders.On("Get", "google").Return(config, nil).Once()
				mockStorage.On("Set", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(assert.AnError).Once()
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset the mocks for this test case
			mockProviders = NewMockProviders()
			mockStorage = &MockVerifierStorage{}

			// Setup test case first
			tc.setupMocks()

			// Create the manager with a pre-populated mock
			config := &oauth2.Config{
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				RedirectURL:  "http://localhost:8080/callback",
				Scopes:       []string{"profile", "email"},
				Endpoint:     oauth2.Endpoint{AuthURL: "https://example.com/auth"},
			}
			// Pre-populate the mock with the data
			if tc.providerName == "google" {
				mockProviders.data["google"] = config
			}

			// Now create the manager with our prepared mock data
			manager := oauth.NewOAuthManager(mockProviders.AsProviders(), logger, mockStorage)

			// Call the method
			authURL, state, err := manager.GenerateAuthURL(ctx, tc.providerName)

			// Assertions
			if tc.expectedError {
				assert.Error(t, err)
				assert.Empty(t, authURL)
				assert.Empty(t, state)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, authURL)
				assert.NotEmpty(t, state)
				assert.Contains(t, authURL, "https://example.com/auth")
			}

			// Verify mock expectations for the storage
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestExchangeCode(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name          string
		providerName  string
		code          string
		state         string
		setupMocks    func(mockProviders *MockProviders, mockStorage *MockVerifierStorage)
		expectedError bool
	}{
		{
			name:          "Empty provider name",
			providerName:  "",
			code:          "code",
			state:         "state",
			setupMocks:    func(mockProviders *MockProviders, mockStorage *MockVerifierStorage) {},
			expectedError: true,
		},
		{
			name:         "Provider not found",
			providerName: "unknown",
			code:         "code",
			state:        "state",
			setupMocks: func(mockProviders *MockProviders, mockStorage *MockVerifierStorage) {
				mockProviders.On("Get", "unknown").Return(nil, oauth.ErrProviderNotFound).Once()
			},
			expectedError: true,
		},
		{
			name:         "Empty code",
			providerName: "google",
			code:         "",
			state:        "state",
			setupMocks: func(mockProviders *MockProviders, mockStorage *MockVerifierStorage) {
				config := &oauth2.Config{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"profile", "email"},
					Endpoint:     oauth2.Endpoint{TokenURL: "https://example.com/token"},
				}
				mockProviders.On("Get", "google").Return(config, nil).Once()
				// Add provider to the data map
				mockProviders.data["google"] = config
			},
			expectedError: true,
		},
		{
			name:         "Empty state",
			providerName: "google",
			code:         "code",
			state:        "",
			setupMocks: func(mockProviders *MockProviders, mockStorage *MockVerifierStorage) {
				config := &oauth2.Config{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"profile", "email"},
					Endpoint:     oauth2.Endpoint{TokenURL: "https://example.com/token"},
				}
				mockProviders.On("Get", "google").Return(config, nil).Once()
				// Add provider to the data map
				mockProviders.data["google"] = config
			},
			expectedError: true,
		},
		{
			name:         "Verifier not found",
			providerName: "google",
			code:         "code",
			state:        "state",
			setupMocks: func(mockProviders *MockProviders, mockStorage *MockVerifierStorage) {
				config := &oauth2.Config{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					RedirectURL:  "http://localhost:8080/callback",
					Scopes:       []string{"profile", "email"},
					Endpoint:     oauth2.Endpoint{TokenURL: "https://example.com/token"},
				}
				mockProviders.On("Get", "google").Return(config, nil).Once()
				mockStorage.On("Get", ctx, "state").Return("", assert.AnError).Once()
				// Add provider to the data map
				mockProviders.data["google"] = config
			},
			expectedError: true,
		},
		// Note: Testing the actual token exchange would require mocking the HTTP client and
		// oauth2.Config.Exchange method, which is beyond the scope of this test.
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset the mocks for this test case
			mockProviders := NewMockProviders()
			mockStorage := &MockVerifierStorage{}
			logger := log.NewLogger("debug")

			// Setup test case
			tc.setupMocks(mockProviders, mockStorage)

			// Create the manager using our prepared mock
			manager := oauth.NewOAuthManager(mockProviders.AsProviders(), logger, mockStorage)

			// Call the method
			token, err := manager.ExchangeCode(ctx, tc.providerName, tc.code, tc.state)

			// Assertions
			if tc.expectedError {
				assert.Error(t, err)
				assert.Nil(t, token)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, token)
			}

			// Verify storage mock expectations
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestRefreshToken(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name          string
		providerName  string
		refreshToken  string
		setupMocks    func(mockProviders *MockProviders, mockStorage *MockVerifierStorage)
		expectedError bool
	}{
		{
			name:          "Empty provider name",
			providerName:  "",
			refreshToken:  "refresh-token",
			setupMocks:    func(mockProviders *MockProviders, mockStorage *MockVerifierStorage) {},
			expectedError: true,
		},
		{
			name:         "Provider not found",
			providerName: "unknown",
			refreshToken: "refresh-token",
			setupMocks: func(mockProviders *MockProviders, mockStorage *MockVerifierStorage) {
				mockProviders.On("Get", "unknown").Return(nil, oauth.ErrProviderNotFound).Once()
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset the mocks for this test case
			mockProviders := NewMockProviders()
			mockStorage := &MockVerifierStorage{}
			logger := log.NewLogger("debug")

			// Setup test case
			tc.setupMocks(mockProviders, mockStorage)

			// Create the manager using our prepared mock
			manager := oauth.NewOAuthManager(mockProviders.AsProviders(), logger, mockStorage)

			// Call the method
			token, err := manager.RefreshToken(ctx, tc.providerName, tc.refreshToken)

			// Assertions
			if tc.expectedError {
				assert.Error(t, err)
				assert.Nil(t, token)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, token)
			}

			// Verify storage mock expectations
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestGetProviderConfig(t *testing.T) {
	// Create mocks
	mockProviders := NewMockProviders()
	mockStorage := &MockVerifierStorage{}
	logger := log.NewLogger("debug")

	// Setup mocks
	expectedConfig := &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://localhost:8080/callback",
		Scopes:       []string{"profile", "email"},
		Endpoint:     oauth2.Endpoint{AuthURL: "https://example.com/auth"},
	}

	// Add mock expectations
	mockProviders.On("Get", "google").Return(expectedConfig, nil).Once()
	mockProviders.On("Get", "unknown").Return(nil, assert.AnError).Once()

	// Pre-populate the mock data directly
	mockProviders.data["google"] = expectedConfig

	// Create manager with our prepared mock
	manager := oauth.NewOAuthManager(mockProviders.AsProviders(), logger, mockStorage)

	// Test getting existing provider
	config, err := manager.GetProviderConfig("google")
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, config)

	// Test getting non-existent provider
	config, err = manager.GetProviderConfig("unknown")
	assert.Error(t, err)
	assert.Nil(t, config)
}
