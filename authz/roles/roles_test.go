package roles

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractUserID(t *testing.T) {
	tests := []struct {
		name          string
		input         interface{}
		expectedID    string
		expectError   bool
		errorContains string
	}{
		{
			name: "user object with ID",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id": "user123",
				},
			},
			expectedID:  "user123",
			expectError: false,
		},
		{
			name: "subject field",
			input: map[string]interface{}{
				"subject": "user456",
			},
			expectedID:  "user456",
			expectError: false,
		},
		{
			name: "user object without ID",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "John Doe",
				},
			},
			expectError:   true,
			errorContains: "user ID not found",
		},
		{
			name:          "no user or subject",
			input:         map[string]interface{}{},
			expectError:   true,
			errorContains: "user ID not found",
		},
		{
			name:          "non-map input",
			input:         "not a map",
			expectError:   true,
			errorContains: "user ID not found",
		},
		{
			name: "user ID not a string",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id": 123,
				},
			},
			expectError:   true,
			errorContains: "user ID not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ExtractUserID(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedID, id)
		})
	}
}

func TestEnrichInputWithRoles(t *testing.T) {
	tests := []struct {
		name          string
		input         interface{}
		roles         []string
		expectedInput interface{}
	}{
		{
			name: "existing user object",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id": "user123",
				},
			},
			roles: []string{"admin", "editor"},
			expectedInput: map[string]interface{}{
				"user": map[string]interface{}{
					"id":    "user123",
					"roles": []string{"admin", "editor"},
				},
			},
		},
		{
			name: "existing user object with roles",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id":    "user123",
					"roles": []string{"existing"},
				},
			},
			roles: []string{"admin", "editor"},
			expectedInput: map[string]interface{}{
				"user": map[string]interface{}{
					"id":    "user123",
					"roles": []string{"admin", "editor"},
				},
			},
		},
		{
			name: "no user object",
			input: map[string]interface{}{
				"action": "read",
			},
			roles: []string{"viewer"},
			expectedInput: map[string]interface{}{
				"action": "read",
				"user": map[string]interface{}{
					"roles": []string{"viewer"},
				},
			},
		},
		{
			name:          "non-map input",
			input:         "not a map",
			roles:         []string{"admin"},
			expectedInput: "not a map", // Should return input unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EnrichInputWithRoles(tt.input, tt.roles)
			assert.Equal(t, tt.expectedInput, result)
		})
	}
}

// MockRoleProvider is a simple implementation of the RoleProvider interface for testing
type MockRoleProvider struct {
	roles map[string][]string
	err   error
}

func NewMockRoleProvider() *MockRoleProvider {
	return &MockRoleProvider{
		roles: map[string][]string{
			"user1": {"admin", "editor"},
			"user2": {"viewer"},
		},
	}
}

func (p *MockRoleProvider) GetRoles(ctx context.Context, userID string) ([]string, error) {
	if p.err != nil {
		return nil, p.err
	}
	if roles, ok := p.roles[userID]; ok {
		return roles, nil
	}
	return []string{}, nil
}

func TestCreateRoleTransformer(t *testing.T) {
	provider := NewMockRoleProvider()
	transformer := CreateRoleTransformer(provider)
	require.NotNil(t, transformer)

	tests := []struct {
		name           string
		input          interface{}
		providerError  error
		expectedOutput interface{}
	}{
		{
			name: "valid user ID",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id": "user1",
				},
			},
			providerError: nil,
			expectedOutput: map[string]interface{}{
				"user": map[string]interface{}{
					"id":    "user1",
					"roles": []string{"admin", "editor"},
				},
			},
		},
		{
			name: "unknown user ID",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id": "unknown",
				},
			},
			providerError: nil,
			expectedOutput: map[string]interface{}{
				"user": map[string]interface{}{
					"id":    "unknown",
					"roles": []string{},
				},
			},
		},
		{
			name: "provider error",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id": "user1",
				},
			},
			providerError: assert.AnError,
			expectedOutput: map[string]interface{}{ // Should return input unchanged on error
				"user": map[string]interface{}{
					"id": "user1",
				},
			},
		},
		{
			name: "invalid user ID format",
			input: map[string]interface{}{
				"not_user": "something",
			},
			providerError: nil,
			expectedOutput: map[string]interface{}{ // Should return input unchanged if can't extract ID
				"not_user": "something",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the provider error for this test
			provider.err = tt.providerError

			// Call the transformer
			result, err := transformer(context.Background(), tt.input)

			// Should never return an error, even if provider has an error
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedOutput, result)
		})
	}
}
