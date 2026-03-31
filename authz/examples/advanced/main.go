package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jasonKoogler/abraxis/authz"
	"github.com/jasonKoogler/abraxis/authz/cache"
	"github.com/jasonKoogler/abraxis/authz/roles"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Create a logger
	logger := log.New(os.Stdout, "[AGENT] ", log.LstdFlags)

	// Define local policies
	policies := map[string]string{
		"authz.rego": `
package authz

default allow = false

# RBAC rules
allow {
    # Get roles assigned to user
    role := input.user.roles[_]
    
    # Check if role has permission for the action and resource
    role_permissions[role][input.action][input.resource.type]
}

# Role definitions
role_permissions = {
    "admin": {
        "read": {"financial": true, "hr": true},
        "write": {"financial": true, "hr": true}
    },
    "viewer": {
        "read": {"financial": true},
        "write": {}
    }
}
`,
	}

	// Set up Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// Create Redis role provider
	roleProvider := roles.NewRedisRoleProvider(redisClient,
		roles.WithKeyPrefix("auth:roles:"),
		roles.WithDefaultTTL(time.Hour*24),
		roles.WithDefaultRoles([]string{"viewer"}),
	)

	// Seed some example roles
	ctx := context.Background()
	roleProvider.SetRoles(ctx, "alice", []string{"admin"})
	roleProvider.SetRoles(ctx, "bob", []string{"viewer"})

	// Create role transformer
	roleTransformer := roles.CreateRoleTransformer(roleProvider)

	// Create Redis cache
	redisCache := cache.NewRedisCache(redisClient,
		cache.WithKeyPrefix("agent:policy:cache:"),
	)

	// Create a new Agent instance with all the bells and whistles
	a, err := authz.New(
		authz.WithLocalPolicies(policies),
		authz.WithDefaultQuery("data.authz.allow"),
		authz.WithLogger(logger),
		authz.WithExternalCache(redisCache, time.Minute*15),                         // Use Redis cache
		authz.WithCacheKeyFields([]string{"user.roles", "action", "resource.type"}), // Partial caching
		authz.WithWebhook("/api/policy-update", "my-secret-key", nil),
		authz.WithRoleProvider(roleProvider),
		authz.WithContextTransformer(roleTransformer),
	)

	if err != nil {
		logger.Fatalf("Failed to create agent: %v", err)
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// Register webhook handler
	a.RegisterWebhook(mux)

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

	// Cache clear endpoint
	mux.HandleFunc("/api/cache/clear", func(w http.ResponseWriter, r *http.Request) {
		if redisCache != nil {
			redisCache.Clear()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "success",
				"message": "Cache cleared",
			})
		} else {
			http.Error(w, "Cache not available", http.StatusInternalServerError)
		}
	})

	// Start server
	logger.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		logger.Fatalf("Server error: %v", err)
	}
}

// extractInput extracts the input for policy evaluation from the request
func extractInput(r *http.Request) (interface{}, error) {
	// Extract user ID from header
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		return nil, fmt.Errorf("missing user ID")
	}

	// Extract resource type from query parameters
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
	input := map[string]any{
		"user": map[string]any{
			"id": userID,
			// Roles will be added by the role provider
		},
		"resource": map[string]any{
			"type": resourceType,
			"path": r.URL.Path,
		},
		"action": action,
		"context": map[string]any{
			"time": time.Now().Format(time.RFC3339),
		},
	}

	return input, nil
}
