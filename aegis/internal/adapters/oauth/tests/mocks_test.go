package tests

import (
	"context"

	"github.com/jasonKoogler/aegis/internal/adapters/oauth"
	"github.com/stretchr/testify/mock"
	"golang.org/x/oauth2"
)

// MockVerifierStorage is a mock implementation of the VerifierStorage interface
type MockVerifierStorage struct {
	mock.Mock
}

func (m *MockVerifierStorage) Get(ctx context.Context, state string) (string, error) {
	args := m.Called(ctx, state)
	return args.String(0), args.Error(1)
}

func (m *MockVerifierStorage) Set(ctx context.Context, state, verifier string) error {
	args := m.Called(ctx, state, verifier)
	return args.Error(0)
}

func (m *MockVerifierStorage) Del(ctx context.Context, state string) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

// MockProviders implements both the mock.Mock functionality and the Providers interface
type MockProviders struct {
	mock.Mock
	data map[string]*oauth2.Config
}

func NewMockProviders() *MockProviders {
	return &MockProviders{
		data: make(map[string]*oauth2.Config),
	}
}

func (m *MockProviders) Get(name string) (*oauth2.Config, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	// Store the result in our map so the actual manager can access it
	config := args.Get(0).(*oauth2.Config)
	m.data[name] = config

	return config, args.Error(1)
}

// AsProviders converts our MockProviders to the oauth.Providers type
func (m *MockProviders) AsProviders() oauth.Providers {
	providers := make(oauth.Providers)

	// Copy the predefined providers into the map
	for k, v := range m.data {
		providers[k] = v
	}

	return providers
}

// Constants that need to be exported for tests
var (
	GoogleUserInfoURL   = "https://www.googleapis.com/oauth2/v3/userinfo"
	TwitterUserInfoURL  = "https://api.twitter.com/1.1/account/verify_credentials.json"
	FacebookUserInfoURL = "https://graph.facebook.com/v3.2/me"
)
