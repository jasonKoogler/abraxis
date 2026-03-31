package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jasonKoogler/prism/internal/features/discovery/adapters/provider"
	"github.com/jasonKoogler/prism/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEtcdDiscoveryIntegration(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION_TESTS") != "" {
		t.Skip("Skipping integration test")
	}

	// Create etcd container
	etcdEndpoints, purgeFunc, err := SetupEtcdContainer(t)
	require.NoError(t, err)
	defer purgeFunc()

	t.Logf("Started etcd container at: %v", etcdEndpoints)

	// Create discovery service config
	ctx := context.Background()
	etcdConfig := ports.DiscoveryConfig{
		Provider: "etcd",
		Etcd: &ports.EtcdConfig{
			Endpoints: etcdEndpoints,
		},
		HeartbeatInterval: 1 * time.Second,
		HeartbeatTimeout:  5 * time.Second,
		DeregisterTimeout: 10 * time.Second,
	}

	// Create discovery service
	etcdDiscovery, err := provider.NewEtcdDiscovery(ctx, etcdConfig)
	require.NoError(t, err)

	t.Run("verify_provider_name", func(t *testing.T) {
		assert.Equal(t, "etcd", etcdDiscovery.GetProviderName())
	})

	t.Run("register_and_get_instance", func(t *testing.T) {
		// Register a test instance
		instance := &ports.ServiceInstance{
			ID:          "test-service-1",
			ServiceName: "test-service",
			Version:     "1.0.0",
			Address:     "localhost",
			Port:        8080,
			Status:      "ACTIVE",
			Tags:        []string{"api", "v1"},
			Metadata: map[string]string{
				"region": "us-east-1",
			},
		}

		err := etcdDiscovery.RegisterInstance(ctx, instance)
		require.NoError(t, err)

		// Get the instance back
		retrieved, err := etcdDiscovery.GetInstance(ctx, "test-service-1")
		require.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "test-service-1", retrieved.ID)
		assert.Equal(t, "test-service", retrieved.ServiceName)
		assert.Equal(t, "1.0.0", retrieved.Version)
		assert.Equal(t, "localhost", retrieved.Address)
		assert.Equal(t, 8080, retrieved.Port)
		assert.Equal(t, "ACTIVE", retrieved.Status)
		assert.ElementsMatch(t, []string{"api", "v1"}, retrieved.Tags)
		assert.Equal(t, "us-east-1", retrieved.Metadata["region"])
	})

	t.Run("list_instances", func(t *testing.T) {
		// Register another instance of the same service
		instance2 := &ports.ServiceInstance{
			ID:          "test-service-2",
			ServiceName: "test-service",
			Version:     "1.0.1",
			Address:     "localhost",
			Port:        8081,
			Status:      "ACTIVE",
			Tags:        []string{"api", "v2"},
			Metadata: map[string]string{
				"region": "us-west-1",
			},
		}

		err := etcdDiscovery.RegisterInstance(ctx, instance2)
		require.NoError(t, err)

		// List all instances of the service
		instances, err := etcdDiscovery.ListInstances(ctx, "test-service")
		require.NoError(t, err)
		assert.Len(t, instances, 2)

		// Verify both instances are in the list
		var found1, found2 bool
		for _, instance := range instances {
			if instance.ID == "test-service-1" {
				found1 = true
				assert.Equal(t, "1.0.0", instance.Version)
			} else if instance.ID == "test-service-2" {
				found2 = true
				assert.Equal(t, "1.0.1", instance.Version)
			}
		}
		assert.True(t, found1, "test-service-1 should be in the list")
		assert.True(t, found2, "test-service-2 should be in the list")
	})

	t.Run("list_services", func(t *testing.T) {
		// Register a different service
		otherInstance := &ports.ServiceInstance{
			ID:          "other-service-1",
			ServiceName: "other-service",
			Version:     "1.0.0",
			Address:     "localhost",
			Port:        9090,
			Status:      "ACTIVE",
			Tags:        []string{"api", "other"},
			Metadata: map[string]string{
				"region": "eu-west-1",
			},
		}

		err := etcdDiscovery.RegisterInstance(ctx, otherInstance)
		require.NoError(t, err)

		// Give a moment for the registration to propagate
		time.Sleep(200 * time.Millisecond)

		// List all services
		services, err := etcdDiscovery.ListServices(ctx)
		require.NoError(t, err)

		// Verify both our services are listed
		var testServiceInfo, otherServiceInfo *ports.ServiceInfo
		for _, service := range services {
			if service.Name == "test-service" {
				testServiceInfo = service
			} else if service.Name == "other-service" {
				otherServiceInfo = service
			}
		}

		require.NotNil(t, testServiceInfo, "test-service should be in the list")
		require.NotNil(t, otherServiceInfo, "other-service should be in the list")

		// Count all instances of each service across all versions
		var testServiceCount, otherServiceCount int
		for _, service := range services {
			if service.Name == "test-service" {
				testServiceCount += service.Count
			} else if service.Name == "other-service" {
				otherServiceCount += service.Count
			}
		}

		assert.Equal(t, 2, testServiceCount, "Should have 2 test-service instances")
		assert.Equal(t, 1, otherServiceCount, "Should have 1 other-service instance")
	})

	t.Run("deregister_instance", func(t *testing.T) {
		// Deregister one of the instances
		err := etcdDiscovery.DeregisterInstance(ctx, "test-service-1")
		require.NoError(t, err)

		// Verify it's gone
		_, err = etcdDiscovery.GetInstance(ctx, "test-service-1")
		assert.Error(t, err, "Instance should be deregistered")

		// List services to verify count decreased
		services, err := etcdDiscovery.ListServices(ctx)
		require.NoError(t, err)

		var testServiceInfo *ports.ServiceInfo
		for _, service := range services {
			if service.Name == "test-service" {
				testServiceInfo = service
				break
			}
		}

		require.NotNil(t, testServiceInfo)
		assert.Equal(t, 1, testServiceInfo.Count) // One instance remains after deregistration
	})

	t.Run("watch_services", func(t *testing.T) {
		// Setup a watcher
		watchCtx, watchCancel := context.WithTimeout(ctx, 10*time.Second)
		defer watchCancel()

		// Watch for test-service changes
		watchCh, err := etcdDiscovery.WatchServices(watchCtx, "test-service")
		require.NoError(t, err)

		// Register a new instance to trigger a watch event
		newInstance := &ports.ServiceInstance{
			ID:          "test-service-3",
			ServiceName: "test-service",
			Version:     "1.0.2",
			Address:     "localhost",
			Port:        8082,
			Status:      "ACTIVE",
			Tags:        []string{"api", "v3"},
			Metadata: map[string]string{
				"region": "ap-south-1",
			},
		}

		// Use a buffered channel to collect events
		receivedEvents := make(chan *ports.ServiceInstance, 10)
		done := make(chan struct{})

		// Collect events in a separate goroutine
		go func() {
			defer close(done)
			for {
				select {
				case received, ok := <-watchCh:
					if !ok {
						return // Channel closed
					}
					receivedEvents <- received
				case <-watchCtx.Done():
					return // Context cancelled
				}
			}
		}()

		// Now register the instance after we're set up to receive events
		err = etcdDiscovery.RegisterInstance(ctx, newInstance)
		require.NoError(t, err)

		// Wait for a short time to collect events
		time.Sleep(2 * time.Second)

		// Verify we received our new instance
		var foundNewInstance bool
		timeout := time.After(5 * time.Second)

		for !foundNewInstance {
			select {
			case received := <-receivedEvents:
				if received.ID == "test-service-3" {
					assert.Equal(t, "test-service", received.ServiceName)
					assert.Equal(t, "1.0.2", received.Version)
					assert.Equal(t, "ACTIVE", received.Status)
					foundNewInstance = true
				}
			case <-timeout:
				t.Fatal("Timeout waiting for instance event")
				return
			}
		}

		// Deregister to trigger another event
		err = etcdDiscovery.DeregisterInstance(ctx, "test-service-3")
		require.NoError(t, err)

		// Cancel the watch context to clean up
		watchCancel()

		// Allow a short time for cleanup to complete
		select {
		case <-done:
			// Properly closed
		case <-time.After(1 * time.Second):
			t.Log("Watch goroutine did not exit cleanly")
		}
	})
}
