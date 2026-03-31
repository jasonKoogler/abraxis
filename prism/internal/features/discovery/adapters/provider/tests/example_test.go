package tests

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/jasonKoogler/prism/internal/features/discovery/adapters/provider"
	"github.com/jasonKoogler/prism/internal/ports"
)

// ExampleServiceDiscovery demonstrates how to use the service discovery package
// in an application context.
func Example_serviceDiscovery() {
	// Create a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a discovery configuration
	config := ports.DiscoveryConfig{
		Provider: "local", // Use local provider for this example
		Local: &ports.LocalConfig{
			PurgeInterval: 30 * time.Second,
		},
		HeartbeatInterval: 15 * time.Second,
		HeartbeatTimeout:  60 * time.Second,
		DeregisterTimeout: 120 * time.Second,
	}

	// Create a service discovery client
	disc, err := provider.NewServiceDiscovery(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create service discovery: %v", err)
	}

	// Register this service instance
	serviceID := "api-service-1"
	instance := &ports.ServiceInstance{
		ID:          serviceID,
		ServiceName: "api-service",
		Version:     "1.0.0",
		Address:     "localhost",
		Port:        8080,
		Status:      "ACTIVE",
		Tags:        []string{"api", "v1"},
		Metadata: map[string]string{
			"region": "us-east-1",
		},
	}

	if err := disc.RegisterInstance(ctx, instance); err != nil {
		log.Fatalf("Failed to register service: %v", err)
	}
	log.Println("Service registered successfully")

	// Start heartbeat goroutine
	go func() {
		ticker := time.NewTicker(config.HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := disc.Heartbeat(ctx, serviceID); err != nil {
					log.Printf("Heartbeat failed: %v", err)
				}
			}
		}
	}()

	// Example: Discover other services
	go func() {
		// Wait for services to register
		time.Sleep(1 * time.Second)

		// List all instances of a particular service
		instances, err := disc.ListInstances(ctx, "auth-service")
		if err != nil {
			log.Printf("Failed to list auth service instances: %v", err)
			return
		}

		if len(instances) == 0 {
			log.Println("No auth service instances found")
			return
		}

		// Use the first available instance
		authService := instances[0]
		authURL := fmt.Sprintf("http://%s:%d/auth", authService.Address, authService.Port)
		log.Printf("Using auth service at: %s", authURL)

		// Make a request to the service
		resp, err := http.Get(authURL)
		if err != nil {
			log.Printf("Failed to call auth service: %v", err)
			return
		}
		defer resp.Body.Close()

		log.Printf("Auth service responded with status: %d", resp.StatusCode)
	}()

	// Example: Watch for service changes
	go func() {
		// Create a watch for a specific service
		watchCh, err := disc.WatchServices(ctx, "database-service")
		if err != nil {
			log.Printf("Failed to watch database service: %v", err)
			return
		}

		// Process service updates
		for {
			select {
			case <-ctx.Done():
				return
			case instance, ok := <-watchCh:
				if !ok {
					log.Println("Watch channel closed")
					return
				}

				if instance.Status == "ACTIVE" {
					log.Printf("Database instance %s is now available at %s:%d",
						instance.ID, instance.Address, instance.Port)
				} else {
					log.Printf("Database instance %s is now %s",
						instance.ID, instance.Status)
				}
			}
		}
	}()

	// In a real application, you would run your HTTP server here
	// For this example, we'll just wait for a bit
	time.Sleep(100 * time.Millisecond)

	// When shutting down, deregister the service
	if err := disc.DeregisterInstance(ctx, serviceID); err != nil {
		log.Printf("Failed to deregister service: %v", err)
	} else {
		log.Println("Service deregistered successfully")
	}

	// Output: Service registered successfully
	// Service deregistered successfully
}

// TestDiscoveryUsagePattern is a test to validate the usage pattern
func TestDiscoveryUsagePattern(t *testing.T) {
	// This is not an actual test, but demonstrates how to integrate
	// service discovery in an application
	t.Skip("This is an example, not a test")
}

// Example of a client factory that uses service discovery
func Example_clientFactory() {
	// Create a service discovery client
	ctx := context.Background()
	disc, err := provider.NewServiceDiscovery(ctx, provider.LoadDiscoveryConfig())
	if err != nil {
		log.Fatalf("Failed to create service discovery: %v", err)
	}

	// Create a client factory that uses service discovery
	clientFactory := NewExampleClientFactory(disc)

	// Create a client for a specific service
	client, err := clientFactory.CreateAuthClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create auth client: %v", err)
	}

	// Use the client
	result, err := client.Authenticate("user", "password")
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	fmt.Printf("Authentication result: %v\n", result)

	// Output:
}

// Example client factory implementation
type ExampleClientFactory struct {
	discovery ports.ServiceDiscoverer
}

func NewExampleClientFactory(discovery ports.ServiceDiscoverer) *ExampleClientFactory {
	return &ExampleClientFactory{
		discovery: discovery,
	}
}

// AuthClient is an example client interface
type AuthClient interface {
	Authenticate(username, password string) (bool, error)
}

// authClientImpl is an example implementation
type authClientImpl struct {
	baseURL string
}

func (c *authClientImpl) Authenticate(username, password string) (bool, error) {
	// In a real implementation, this would make an HTTP request to the service
	// For this example, we just return a successful result
	return true, nil
}

// CreateAuthClient creates a client for the auth service
func (f *ExampleClientFactory) CreateAuthClient(ctx context.Context) (AuthClient, error) {
	// Find an available auth service instance
	instances, err := f.discovery.ListInstances(ctx, "auth-service")
	if err != nil {
		return nil, fmt.Errorf("failed to list auth service instances: %w", err)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no auth service instances available")
	}

	// Use the first available instance
	instance := instances[0]
	baseURL := fmt.Sprintf("http://%s:%d", instance.Address, instance.Port)

	return &authClientImpl{
		baseURL: baseURL,
	}, nil
}
