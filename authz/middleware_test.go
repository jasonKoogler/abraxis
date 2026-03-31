package authz

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Custom test policy for middleware tests
const middlewareTestPolicy = `
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

# Allow access to public paths
allow if {
    input.path == "/public"
}

# Allow if explicitly set in the input
allow if {
    input.allow == true
}
`

func TestRequireAuth(t *testing.T) {
	// Create a new agent with local policies
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": middlewareTestPolicy}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Create a middleware that requires authentication
	middleware := RequireAuth(agent)

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("success"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// Wrap the handler with the middleware
	wrappedHandler := middleware(handler)

	tests := []struct {
		name           string
		path           string
		headers        map[string]string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "allowed path with admin role",
			path:           "/admin",
			headers:        map[string]string{"X-User-Roles": "admin"},
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name:           "denied path without role",
			path:           "/admin",
			headers:        map[string]string{},
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Forbidden",
		},
		{
			name:           "allowed public path",
			path:           "/public",
			headers:        map[string]string{},
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name: "denied path with non-admin role",
			path: "/admin",
			headers: map[string]string{
				"X-User-ID":    "viewer",
				"X-User-Roles": "viewer",
			},
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request
			req := httptest.NewRequest("GET", tt.path, nil)

			// Add headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Create a response recorder
			rr := httptest.NewRecorder()

			// Serve the request
			wrappedHandler.ServeHTTP(rr, req)

			// Check the status code
			assert.Equal(t, tt.expectedStatus, rr.Code)

			// Check the response body
			assert.Contains(t, rr.Body.String(), tt.expectedBody)
		})
	}
}

func TestWithInputTransformer(t *testing.T) {
	// Create a new agent with local policies
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": middlewareTestPolicy}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Create a middleware with an input transformer
	middleware := RequireAuth(agent,
		WithInputTransformer(func(r *http.Request) map[string]interface{} {
			// Extract user info from headers
			userID := r.Header.Get("X-User-ID")
			userRoles := r.Header.Get("X-User-Roles")

			// Create roles slice
			var roles []string
			if userRoles != "" {
				roles = []string{userRoles}
			}

			// Check for special header
			specialHeader := r.Header.Get("X-Special")
			if specialHeader == "allow" {
				return map[string]interface{}{
					"allow": true,
				}
			}

			// Return input with user info
			return map[string]interface{}{
				"user": map[string]interface{}{
					"id":    userID,
					"roles": roles,
				},
				"path":   r.URL.Path,
				"method": r.Method,
			}
		}),
	)

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("success"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// Wrap the handler with the middleware
	wrappedHandler := middleware(handler)

	tests := []struct {
		name           string
		path           string
		headers        map[string]string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "denied without special header",
			path:           "/admin",
			headers:        map[string]string{},
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Forbidden",
		},
		{
			name: "allowed with special header",
			path: "/admin",
			headers: map[string]string{
				"X-Special": "allow",
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name: "allowed with admin role",
			path: "/admin",
			headers: map[string]string{
				"X-User-ID":    "admin",
				"X-User-Roles": "admin",
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request
			req := httptest.NewRequest("GET", tt.path, nil)

			// Add headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Create a response recorder
			rr := httptest.NewRecorder()

			// Serve the request
			wrappedHandler.ServeHTTP(rr, req)

			// Check the status code
			assert.Equal(t, tt.expectedStatus, rr.Code)

			// Check the response body
			assert.Contains(t, rr.Body.String(), tt.expectedBody)
		})
	}
}

func TestWithUnauthorizedHandler(t *testing.T) {
	// Create a new agent with local policies
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": middlewareTestPolicy}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Create a custom unauthorized handler
	unauthorizedHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		err := json.NewEncoder(w).Encode(map[string]string{
			"error":   "Unauthorized",
			"message": "You must be logged in to access this resource",
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Create a middleware with the custom unauthorized handler
	middleware := RequireAuth(agent,
		WithUnauthorizedHandler(unauthorizedHandler),
	)

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("success"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// Wrap the handler with the middleware
	wrappedHandler := middleware(handler)

	tests := []struct {
		name           string
		path           string
		headers        map[string]string
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "allowed path",
			path: "/admin",
			headers: map[string]string{
				"X-User-ID":    "admin",
				"X-User-Roles": "admin",
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name:           "denied path with custom handler",
			path:           "/admin",
			headers:        map[string]string{},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "You must be logged in to access this resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request
			req := httptest.NewRequest("GET", tt.path, nil)

			// Add headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Create a response recorder
			rr := httptest.NewRecorder()

			// Serve the request
			wrappedHandler.ServeHTTP(rr, req)

			// Check the status code
			assert.Equal(t, tt.expectedStatus, rr.Code)

			// Check the response body
			assert.Contains(t, rr.Body.String(), tt.expectedBody)
		})
	}
}

func TestWithCacheKey(t *testing.T) {
	// Create a new agent with local policies and memory cache
	agent, err := New(
		WithLocalPolicies(map[string]string{"test.rego": middlewareTestPolicy}),
		WithDefaultQuery("data.authz.allow"),
		WithLogger(&TestLogger{}),
		WithMemoryCache(0, 100),
	)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Create a middleware with a custom cache key function
	middleware := RequireAuth(agent,
		WithCacheKey(func(r *http.Request) string {
			// Use the user ID and path as the cache key
			return r.Header.Get("X-User-ID") + ":" + r.URL.Path
		}),
	)

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("success"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// Wrap the handler with the middleware
	wrappedHandler := middleware(handler)

	// Create a request with admin role
	req1 := httptest.NewRequest("GET", "/admin", nil)
	req1.Header.Set("X-User-ID", "admin")
	req1.Header.Set("X-User-Roles", "admin")

	// Create a response recorder
	rr1 := httptest.NewRecorder()

	// Serve the request
	wrappedHandler.ServeHTTP(rr1, req1)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Create another request with the same user and path
	req2 := httptest.NewRequest("GET", "/admin", nil)
	req2.Header.Set("X-User-ID", "admin")
	req2.Header.Set("X-User-Roles", "admin")

	// Create a response recorder
	rr2 := httptest.NewRecorder()

	// Serve the request
	wrappedHandler.ServeHTTP(rr2, req2)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr2.Code)

	// The second request should have used the cached result
	// But we can't easily verify this from the outside
	// We'd need to mock the cache or expose cache hits
}
