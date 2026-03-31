package authz

import (
	"context"
	"net/http"
	"time"

	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
)

// Example shows how to use the Authz adapter
func Example() {
	// Create a logger
	logger := log.NewLogger("debug")

	// Create the Authz adapter
	adapter, err := New(Config{
		RedisAddr:     "localhost:6379",
		RedisPassword: "",
		RedisDB:       0,
		CacheTTL:      time.Minute * 5,
		MaxCacheSize:  1000,
		Logger:        logger,
		WebhookPath:   "/api/policy-update",
		WebhookSecret: "my-secret-key",
	})
	if err != nil {
		logger.Fatal("Failed to create authz adapter", log.Error(err))
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// Register webhook handler
	adapter.RegisterWebhook(mux)

	// Protected endpoint with middleware
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Access granted!"))
	})

	// Apply middleware
	mux.Handle("/api/protected", adapter.Middleware(APIInputExtractor)(protectedHandler))

	// Direct policy evaluation example
	ctx := context.Background()
	input := map[string]interface{}{
		"user": map[string]interface{}{
			"id":    "user123",
			"roles": []string{"admin"},
		},
		"resource": map[string]interface{}{
			"type":  "document",
			"id":    "doc123",
			"owner": "user123",
		},
		"action": "read",
	}

	decision, err := adapter.Evaluate(ctx, input)
	if err != nil {
		logger.Printf("Error evaluating policy: %v", err)
	} else {
		logger.Printf("Decision: allowed=%v, reason=%s", decision.Allowed, decision.Reason)
	}

	// Start server
	logger.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		logger.Fatal("Server error", log.Error(err))
	}
}
