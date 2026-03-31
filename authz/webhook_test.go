package authz

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogger is already defined in agent_test.go

func TestWebhookHandler(t *testing.T) {
	// Create a simple policy for testing
	policy := `
package authz

default allow = false

allow if {
    input.user.roles[_] == "admin"
}
`

	// Create a new agent
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": policy}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Create a webhook config
	secret := "test-secret"
	config := &WebhookConfig{
		Secret:         secret,
		Endpoint:       "/webhook",
		AllowedSources: []string{"127.0.0.1"},
	}

	// Set the webhook config on the agent
	agent.config.WebhookConfig = config
	agent.config.Source = PolicySourceLocal

	// Create a test server with the webhook handler
	mux := http.NewServeMux()
	agent.RegisterWebhook(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create a test policy update
	updatedPolicy := `
package authz

default allow = false

allow if {
    input.user.roles[_] == "admin"
}

allow if {
    input.user.roles[_] == "editor"
}
`

	// Create a policy update
	update := PolicyUpdate{
		Policies: map[string]string{
			"test.rego": updatedPolicy,
		},
	}

	// Marshal the policies for signature calculation
	policiesJSON, err := json.Marshal(update.Policies)
	require.NoError(t, err)

	// Calculate HMAC signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(policiesJSON)
	update.Signature = hex.EncodeToString(h.Sum(nil))

	// Marshal the full update
	updateJSON, err := json.Marshal(update)
	require.NoError(t, err)

	// Create a request with the payload and signature
	req, err := http.NewRequest("POST", server.URL+"/webhook", bytes.NewBuffer(updateJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "127.0.0.1") // Set the IP to an allowed source

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check the response
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify the policy was updated by testing with a user that has the editor role
	input := map[string]interface{}{
		"user": map[string]interface{}{
			"roles": []string{"editor"},
		},
	}
	decision, err := agent.Evaluate(req.Context(), input)
	require.NoError(t, err)
	assert.True(t, decision.Allowed)
}

func TestWebhookHandlerInvalidSignature(t *testing.T) {
	// Create a new agent
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": "package authz\ndefault allow = false"}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Create a webhook config
	secret := "test-secret"
	config := &WebhookConfig{
		Secret:         secret,
		Endpoint:       "/webhook",
		AllowedSources: []string{"127.0.0.1"},
	}

	// Set the webhook config on the agent
	agent.config.WebhookConfig = config
	agent.config.Source = PolicySourceLocal

	// Create a test server with the webhook handler
	mux := http.NewServeMux()
	agent.RegisterWebhook(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create a policy update
	update := PolicyUpdate{
		Policies: map[string]string{
			"test.rego": "package authz\ndefault allow = true",
		},
		Signature: "invalid-signature", // Invalid signature
	}

	// Marshal the update
	updateJSON, err := json.Marshal(update)
	require.NoError(t, err)

	// Create a request with the payload and invalid signature
	req, err := http.NewRequest("POST", server.URL+"/webhook", bytes.NewBuffer(updateJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "127.0.0.1") // Set the IP to an allowed source

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check the response (should be unauthorized)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWebhookHandlerUnauthorizedIP(t *testing.T) {
	// Create a new agent
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": "package authz\ndefault allow = false"}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Create a webhook config with allowed sources
	config := &WebhookConfig{
		Secret:         "test-secret",
		Endpoint:       "/webhook",
		AllowedSources: []string{"192.168.1.1"}, // Not matching the test request IP
	}

	// Set the webhook config on the agent
	agent.config.WebhookConfig = config
	agent.config.Source = PolicySourceLocal

	// Create a test server with the webhook handler
	mux := http.NewServeMux()
	agent.RegisterWebhook(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create a policy update
	update := PolicyUpdate{
		Policies: map[string]string{
			"test.rego": "package authz\ndefault allow = true",
		},
	}

	// Marshal the update
	updateJSON, err := json.Marshal(update)
	require.NoError(t, err)

	// Create a request with the payload
	req, err := http.NewRequest("POST", server.URL+"/webhook", bytes.NewBuffer(updateJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "127.0.0.1") // Not in allowed sources

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check the response (should be forbidden)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestWebhookHandlerInvalidPayload(t *testing.T) {
	// Create a new agent
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": "package authz\ndefault allow = false"}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Create a webhook config
	config := &WebhookConfig{
		Secret:         "test-secret",
		Endpoint:       "/webhook",
		AllowedSources: []string{"127.0.0.1"},
	}

	// Set the webhook config on the agent
	agent.config.WebhookConfig = config
	agent.config.Source = PolicySourceLocal

	// Create a test server with the webhook handler
	mux := http.NewServeMux()
	agent.RegisterWebhook(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create an invalid JSON payload
	invalidPayload := []byte(`{"policies": {invalid json}`)

	// Create a request with the invalid payload
	req, err := http.NewRequest("POST", server.URL+"/webhook", bytes.NewBuffer(invalidPayload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "127.0.0.1") // Set the IP to an allowed source

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check the response (should be bad request)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestWebhookHandlerWrongMethod(t *testing.T) {
	// Create a new agent
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": "package authz\ndefault allow = false"}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Create a webhook config
	config := &WebhookConfig{
		Secret:         "test-secret",
		Endpoint:       "/webhook",
		AllowedSources: []string{"127.0.0.1"},
	}

	// Set the webhook config on the agent
	agent.config.WebhookConfig = config
	agent.config.Source = PolicySourceLocal

	// Create a test server with the webhook handler
	mux := http.NewServeMux()
	agent.RegisterWebhook(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Create a GET request (webhook only accepts POST)
	req, err := http.NewRequest("GET", server.URL+"/webhook", nil)
	require.NoError(t, err)
	req.Header.Set("X-Forwarded-For", "127.0.0.1") // Set the IP to an allowed source

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check the response (should be method not allowed)
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestIsAllowedIP(t *testing.T) {
	tests := []struct {
		name           string
		ip             string
		allowedSources []string
		expected       bool
	}{
		{
			name:           "IP in allowed list",
			ip:             "192.168.1.1",
			allowedSources: []string{"192.168.1.1", "10.0.0.1"},
			expected:       true,
		},
		{
			name:           "IP not in allowed list",
			ip:             "192.168.1.2",
			allowedSources: []string{"192.168.1.1", "10.0.0.1"},
			expected:       false,
		},
		{
			name:           "IP in CIDR range",
			ip:             "192.168.1.5",
			allowedSources: []string{"192.168.1.0/24"},
			expected:       true,
		},
		{
			name:           "IP not in CIDR range",
			ip:             "10.0.0.1",
			allowedSources: []string{"192.168.1.0/24"},
			expected:       false,
		},
		{
			name:           "Invalid IP",
			ip:             "invalid-ip",
			allowedSources: []string{"192.168.1.1"},
			expected:       false,
		},
		{
			name:           "Invalid CIDR",
			ip:             "192.168.1.1",
			allowedSources: []string{"invalid-cidr"},
			expected:       false,
		},
		{
			name:           "Empty allowed sources",
			ip:             "192.168.1.1",
			allowedSources: []string{},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAllowedIP(tt.ip, tt.allowedSources)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVerifySignature(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"test":"data"}`)

	// Calculate a valid signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	validSignature := hex.EncodeToString(h.Sum(nil))

	tests := []struct {
		name      string
		payload   []byte
		signature string
		secret    string
		expected  bool
	}{
		{
			name:      "Valid signature",
			payload:   payload,
			signature: validSignature,
			secret:    secret,
			expected:  true,
		},
		{
			name:      "Invalid signature",
			payload:   payload,
			signature: "invalid-signature",
			secret:    secret,
			expected:  false,
		},
		{
			name:      "Empty signature",
			payload:   payload,
			signature: "",
			secret:    secret,
			expected:  false,
		},
		{
			name:      "Different payload",
			payload:   []byte(`{"different":"payload"}`),
			signature: validSignature,
			secret:    secret,
			expected:  false,
		},
		{
			name:      "Different secret",
			payload:   payload,
			signature: validSignature,
			secret:    "different-secret",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := verifySignature(tt.payload, tt.signature, tt.secret)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name         string
		remoteAddr   string
		forwardedFor string
		expectedIP   string
		expectError  bool
	}{
		{
			name:         "Remote address only",
			remoteAddr:   "192.168.1.1:1234",
			forwardedFor: "",
			expectedIP:   "192.168.1.1",
			expectError:  false,
		},
		{
			name:         "X-Forwarded-For header",
			remoteAddr:   "10.0.0.1:1234",
			forwardedFor: "192.168.1.1",
			expectedIP:   "192.168.1.1",
			expectError:  false,
		},
		{
			name:         "Multiple X-Forwarded-For IPs",
			remoteAddr:   "10.0.0.1:1234",
			forwardedFor: "192.168.1.1, 10.0.0.2, 10.0.0.3",
			expectedIP:   "192.168.1.1",
			expectError:  false,
		},
		{
			name:         "Remote address without port",
			remoteAddr:   "192.168.1.1",
			forwardedFor: "",
			expectedIP:   "192.168.1.1",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/webhook", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.forwardedFor)
			}

			ip, err := getClientIP(req)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedIP, ip)
			}
		})
	}
}
