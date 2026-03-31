package authz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogger is a simple logger for testing
type TestLogger struct{}

func (l *TestLogger) Printf(format string, v ...any) {}
func (l *TestLogger) Println(v ...any)               {}

// TestRoleProvider is a simple role provider for testing
type TestRoleProvider struct {
	roles map[string][]string
}

func NewTestRoleProvider() *TestRoleProvider {
	return &TestRoleProvider{
		roles: map[string][]string{
			"user1": {"admin", "editor"},
			"user2": {"viewer"},
		},
	}
}

func (p *TestRoleProvider) GetRoles(ctx context.Context, userID string) ([]string, error) {
	if roles, ok := p.roles[userID]; ok {
		return roles, nil
	}
	return []string{}, nil
}

// Simple policy for testing
const testPolicy = `
package authz

default allow = false

allow if {
    input.user.roles[_] == "admin"
}

allow if {
    input.user.roles[_] == "editor"
    input.action == "edit"
}

allow if {
    input.user.roles[_] == "viewer"
    input.action == "view"
}
`

func TestNewAgent(t *testing.T) {
	tests := []struct {
		name          string
		options       []Option
		expectError   bool
		errorContains string
	}{
		{
			name: "with local policies",
			options: []Option{
				WithLocalPolicies(map[string]string{"test.rego": testPolicy}),
				WithDefaultQuery("data.authz.allow"),
			},
			expectError: false,
		},
		{
			name: "with external OPA",
			options: []Option{
				WithExternalOPA("http://localhost:8181"),
				WithDefaultQuery("data.authz.allow"),
			},
			expectError: false,
		},
		{
			name: "with memory cache",
			options: []Option{
				WithLocalPolicies(map[string]string{"test.rego": testPolicy}),
				WithDefaultQuery("data.authz.allow"),
				WithMemoryCache(time.Minute, 100),
			},
			expectError: false,
		},
		{
			name: "with no cache",
			options: []Option{
				WithLocalPolicies(map[string]string{"test.rego": testPolicy}),
				WithDefaultQuery("data.authz.allow"),
				WithNoCache(),
			},
			expectError: false,
		},
		{
			name: "with logger",
			options: []Option{
				WithLocalPolicies(map[string]string{"test.rego": testPolicy}),
				WithDefaultQuery("data.authz.allow"),
				WithLogger(&TestLogger{}),
			},
			expectError: false,
		},
		{
			name: "with role provider",
			options: []Option{
				WithLocalPolicies(map[string]string{"test.rego": testPolicy}),
				WithDefaultQuery("data.authz.allow"),
				WithRoleProvider(NewTestRoleProvider()),
			},
			expectError: false,
		},
		{
			name: "missing policies for local source",
			options: []Option{
				WithDefaultQuery("data.authz.allow"),
				// Setting source to local but not providing policies
				func(c *Config) { c.Source = PolicySourceLocal },
			},
			expectError:   true,
			errorContains: "local policies are required when using PolicySourceLocal",
		},
		{
			name: "missing URL for external source",
			options: []Option{
				WithDefaultQuery("data.authz.allow"),
				// Setting source to external but not providing URL
				func(c *Config) { c.Source = PolicySourceExternal },
			},
			expectError:   true,
			errorContains: "external OPA URL is required when using PolicySourceExternal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := New(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, agent)
		})
	}
}

func TestAgentEvaluate(t *testing.T) {
	// Create a new agent with local policies
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": testPolicy}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	tests := []struct {
		name           string
		input          map[string]interface{}
		expectedResult bool
		expectedReason string
	}{
		{
			name: "admin can do anything",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id":    "admin",
					"roles": []string{"admin"},
				},
				"action": "delete",
			},
			expectedResult: true,
		},
		{
			name: "editor can edit",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id":    "editor",
					"roles": []string{"editor"},
				},
				"action": "edit",
			},
			expectedResult: true,
		},
		{
			name: "editor cannot delete",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id":    "editor",
					"roles": []string{"editor"},
				},
				"action": "delete",
			},
			expectedResult: false,
		},
		{
			name: "viewer can view",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id":    "viewer",
					"roles": []string{"viewer"},
				},
				"action": "view",
			},
			expectedResult: true,
		},
		{
			name: "viewer cannot edit",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id":    "viewer",
					"roles": []string{"viewer"},
				},
				"action": "edit",
			},
			expectedResult: false,
		},
		{
			name: "unknown role cannot do anything",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id":    "unknown",
					"roles": []string{"unknown"},
				},
				"action": "view",
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := agent.Evaluate(context.Background(), tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, decision.Allowed)
		})
	}
}

func TestAgentEvaluateWithRoleProvider(t *testing.T) {
	// Create a new agent with local policies and role provider
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": testPolicy}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
		WithRoleProvider(NewTestRoleProvider()),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Create a context transformer that enriches input with roles from the role provider
	transformer := func(ctx context.Context, input interface{}) (interface{}, error) {
		if inputMap, ok := input.(map[string]interface{}); ok {
			if user, ok := inputMap["user"].(map[string]interface{}); ok {
				if id, ok := user["id"].(string); ok {
					roles, err := agent.config.RoleProvider.GetRoles(ctx, id)
					if err == nil {
						user["roles"] = roles
					}
				}
			}
		}
		return input, nil
	}

	// Add the transformer to the agent
	agent.config.ContextTransformers = append(agent.config.ContextTransformers, transformer)

	tests := []struct {
		name           string
		input          map[string]interface{}
		expectedResult bool
	}{
		{
			name: "user1 has admin role and can do anything",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id": "user1",
				},
				"action": "delete",
			},
			expectedResult: true,
		},
		{
			name: "user2 has viewer role and can view",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id": "user2",
				},
				"action": "view",
			},
			expectedResult: true,
		},
		{
			name: "user2 has viewer role and cannot edit",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id": "user2",
				},
				"action": "edit",
			},
			expectedResult: false,
		},
		{
			name: "unknown user has no roles and cannot do anything",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"id": "unknown",
				},
				"action": "view",
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := agent.Evaluate(context.Background(), tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, decision.Allowed)
		})
	}
}

func TestAgentEvaluateWithCache(t *testing.T) {
	// Create a new agent with local policies and memory cache
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": testPolicy}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
		WithMemoryCache(time.Minute, 100),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// First evaluation should not be cached
	input := map[string]interface{}{
		"user": map[string]interface{}{
			"id":    "admin",
			"roles": []string{"admin"},
		},
		"action": "delete",
	}

	decision1, err := agent.Evaluate(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, decision1.Allowed)
	assert.False(t, decision1.Cached)

	// Second evaluation with the same input should be cached
	decision2, err := agent.Evaluate(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, decision2.Allowed)
	assert.True(t, decision2.Cached)
}

func TestAgentEvaluateQuery(t *testing.T) {
	// Create a new agent with local policies
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": testPolicy}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Test with a custom query path
	input := map[string]interface{}{
		"user": map[string]interface{}{
			"id":    "admin",
			"roles": []string{"admin"},
		},
		"action": "delete",
	}

	decision, err := agent.EvaluateQuery(context.Background(), "data.authz.allow", input)
	require.NoError(t, err)
	assert.True(t, decision.Allowed)
}

func TestAgentUpdatePolicies(t *testing.T) {
	// Create a new agent with local policies
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": testPolicy}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Test initial policy
	input := map[string]interface{}{
		"user": map[string]interface{}{
			"id":    "viewer",
			"roles": []string{"viewer"},
		},
		"action": "edit",
	}

	decision1, err := agent.Evaluate(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, decision1.Allowed)

	// Update policy to allow viewers to edit
	newPolicy := `
package authz

default allow = false

allow if {
    input.user.roles[_] == "admin"
}

allow if {
    input.user.roles[_] == "editor"
    input.action == "edit"
}

allow if {
    input.user.roles[_] == "viewer"
}
`
	err = agent.UpdatePolicies(map[string]string{"test.rego": newPolicy})
	require.NoError(t, err)

	// Test with updated policy
	decision2, err := agent.Evaluate(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, decision2.Allowed)
}

func TestAgentWithExternalOPA(t *testing.T) {
	// Create a mock OPA server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&input)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Check if the input contains an admin role
		isAdmin := false
		if user, ok := input["input"].(map[string]interface{})["user"].(map[string]interface{}); ok {
			if roles, ok := user["roles"].([]interface{}); ok {
				for _, role := range roles {
					if role == "admin" {
						isAdmin = true
						break
					}
				}
			}
		}

		// Return a response based on the input
		response := map[string]interface{}{
			"result": isAdmin,
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	// Create a new agent with external OPA
	agent, err := New(
		WithExternalOPA(server.URL),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Test with admin role
	input1 := map[string]interface{}{
		"user": map[string]interface{}{
			"id":    "admin",
			"roles": []string{"admin"},
		},
		"action": "delete",
	}

	decision1, err := agent.Evaluate(context.Background(), input1)
	require.NoError(t, err)
	assert.True(t, decision1.Allowed)

	// Test with non-admin role
	input2 := map[string]interface{}{
		"user": map[string]interface{}{
			"id":    "viewer",
			"roles": []string{"viewer"},
		},
		"action": "view",
	}

	decision2, err := agent.Evaluate(context.Background(), input2)
	require.NoError(t, err)
	assert.False(t, decision2.Allowed)
}

func TestContextTransformer(t *testing.T) {
	// Create a context transformer that adds a role
	transformer := func(ctx context.Context, input interface{}) (interface{}, error) {
		if inputMap, ok := input.(map[string]interface{}); ok {
			if user, ok := inputMap["user"].(map[string]interface{}); ok {
				roles := []string{"added_role"}
				if existingRoles, ok := user["roles"].([]string); ok {
					roles = append(existingRoles, "added_role")
				}
				user["roles"] = roles
			}
		}
		return input, nil
	}

	// Create a new agent with local policies and context transformer
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": testPolicy}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
		WithContextTransformer(transformer),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Test with input that should be transformed
	input := map[string]interface{}{
		"user": map[string]interface{}{
			"id":    "user",
			"roles": []string{},
		},
		"action": "edit",
	}

	// Create a custom policy that allows the added_role to edit
	customPolicy := `
package authz

default allow = false

allow if {
    input.user.roles[_] == "added_role"
    input.action == "edit"
}
`
	err = agent.UpdatePolicies(map[string]string{"test.rego": customPolicy})
	require.NoError(t, err)

	// The transformer should add the "added_role" role, which should allow editing
	decision, err := agent.Evaluate(context.Background(), input)
	require.NoError(t, err)
	assert.True(t, decision.Allowed)
}
