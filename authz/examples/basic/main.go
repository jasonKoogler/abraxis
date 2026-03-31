package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jasonKoogler/abraxis/authz"
)

func main() {
	// Create a logger
	logger := log.New(os.Stdout, "[AGENT] ", log.LstdFlags)

	// Define local policies
	policies := map[string]string{
		"authz.rego": `
package authz

default allow = false

# Simple role-based access control
allow {
    input.user.roles[_] == "admin"
}

allow {
    input.user.roles[_] == "viewer"
    input.action == "read"
}
`,
	}

	// Create a new Agent instance with memory cache
	a, err := authz.New(
		authz.WithLocalPolicies(policies),
		authz.WithDefaultQuery("data.authz.allow"),
		authz.WithLogger(logger),
		authz.WithMemoryCache(time.Minute*5, 1000), // 5 minute TTL, max 1000 entries
	)

	if err != nil {
		logger.Fatalf("Failed to create agent: %v", err)
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// Protected endpoint with middleware
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Access granted!"})
	})

	// Apply middleware
	mux.Handle("/api/protected", a.Middleware(extractInput)(protectedHandler))

	// Public endpoint
	mux.HandleFunc("/api/public", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "This is public!"})
	})

	// Manual policy evaluation example
	mux.HandleFunc("/api/evaluate", func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var input map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Evaluate policy
		decision, err := a.Evaluate(r.Context(), input)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error evaluating policy: %v", err), http.StatusInternalServerError)
			return
		}

		// Return decision
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"allowed":   decision.Allowed,
			"cached":    decision.Cached,
			"reason":    decision.Reason,
			"timestamp": decision.Timestamp,
		})
	})

	// Start server
	logger.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		logger.Fatalf("Server error: %v", err)
	}
}

// extractInput extracts the input for policy evaluation from the request
func extractInput(r *http.Request) (interface{}, error) {
	// In a real application, this would extract user information from
	// authentication context (e.g., JWT token), and combine with request data

	// For this example, we'll use a simple extraction
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		return nil, fmt.Errorf("missing user ID")
	}

	roles := []string{}
	roleHeader := r.Header.Get("X-User-Roles")
	if roleHeader != "" {
		// Parse comma-separated roles
		if err := json.Unmarshal([]byte(roleHeader), &roles); err != nil {
			return nil, fmt.Errorf("invalid roles format")
		}
	}

	// Extract resource type from path or query params
	resourceType := r.URL.Query().Get("type")
	if resourceType == "" {
		resourceType = "default"
	}

	// Map HTTP method to action
	action := "read"
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		action = "write"
	case http.MethodDelete:
		action = "delete"
	}

	// Build input object
	input := map[string]interface{}{
		"user": map[string]interface{}{
			"id":    userID,
			"roles": roles,
		},
		"resource": map[string]interface{}{
			"type": resourceType,
			"path": r.URL.Path,
		},
		"action": action,
		"context": map[string]interface{}{
			"time": time.Now().Format(time.RFC3339),
		},
	}

	return input, nil
}
