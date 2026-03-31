package tests

import (
	"os"
	"testing"

	"github.com/jasonKoogler/aegis/internal/adapters/oauth"
	"golang.org/x/oauth2"
)

// This file serves as a placeholder for package testing and can include setup/teardown
// functions that will be run before/after all tests in the package.

func TestMain(m *testing.M) {
	// Any setup before running all tests in the package

	// Run all tests
	code := m.Run()

	// Any teardown after running all tests

	// Exit with the test status code
	os.Exit(code)
}

// Test that ensures the mocks properly implement the interfaces
func TestMockInterfaces(t *testing.T) {
	// Verify MockVerifierStorage implements oauth.VerifierStorage
	var _ oauth.VerifierStorage = &MockVerifierStorage{}

	// Verify MockProviders implements the correct provider interface
	var _ interface {
		Get(string) (*oauth2.Config, error)
	} = &MockProviders{}
}
