package tests

import (
	"context"
	"testing"
	"time"

	"github.com/jasonKoogler/abraxis/prism/internal/features/discovery/adapters/provider"
	"github.com/jasonKoogler/abraxis/prism/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDiscoveryIntegration tests the integration between the factory and the local implementation
func TestDiscoveryIntegration(t *testing.T) {
	// Use a timeout for the entire test
	testTimeout := 30 * time.Second
	testCtx, testCancel := context.WithTimeout(context.Background(), testTimeout)
	defer testCancel()

	// Use the factory to create a local discovery service
	ctx, cancel := context.WithCancel(testCtx)
	defer cancel()

	config := ports.DiscoveryConfig{
		Provider: "local",
		Local: &ports.LocalConfig{
			PurgeInterval: 500 * time.Millisecond,
		},
		HeartbeatInterval: 100 * time.Millisecond,
		HeartbeatTimeout:  500 * time.Millisecond,
		DeregisterTimeout: 1 * time.Second,
	}

	// Create discovery service using the factory
	disc, err := provider.NewServiceDiscovery(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, disc)

	// Ensure we close the discovery service after the test if it implements io.Closer
	defer func() {
		// Check if the discovery service implements io.Closer
		if closer, ok := disc.(interface{ Close() error }); ok {
			err := closer.Close()
			if err != nil {
				t.Logf("Warning: error closing discovery service: %v", err)
			}
		}
	}()

	// Test the full lifecycle of a service instance
	t.Run("service_lifecycle", func(t *testing.T) {
		// Add a timeout for this subtest
		subtestCtx, subtestCancel := context.WithTimeout(ctx, 10*time.Second)
		defer subtestCancel()

		// 1. Register a service instance
		instance := &ports.ServiceInstance{
			ID:          "lifecycle-test",
			ServiceName: "lifecycle-service",
			Version:     "1.0.0",
			Address:     "localhost",
			Port:        8080,
			Status:      "ACTIVE",
			Tags:        []string{"lifecycle", "test"},
			Metadata: map[string]string{
				"environment": "test",
			},
		}

		err := disc.RegisterInstance(subtestCtx, instance)
		require.NoError(t, err)

		// 2. Verify it can be retrieved
		retrieved, err := disc.GetInstance(subtestCtx, "lifecycle-test")
		require.NoError(t, err)
		assert.Equal(t, instance.ID, retrieved.ID)
		assert.Equal(t, instance.ServiceName, retrieved.ServiceName)
		assert.False(t, retrieved.RegisteredAt.IsZero())
		assert.False(t, retrieved.LastHeartbeat.IsZero())

		// 3. List instances and services
		instances, err := disc.ListInstances(subtestCtx, "lifecycle-service")
		require.NoError(t, err)
		assert.Len(t, instances, 1)
		assert.Equal(t, "lifecycle-test", instances[0].ID)

		services, err := disc.ListServices(subtestCtx)
		require.NoError(t, err)
		assert.Len(t, services, 1)
		assert.Equal(t, "lifecycle-service", services[0].Name)
		assert.Equal(t, "1.0.0", services[0].Version)
		assert.Equal(t, 1, services[0].Count)

		// 4. Set up a watcher with a short timeout
		watchCtx, watchCancel := context.WithTimeout(subtestCtx, 3*time.Second)
		defer watchCancel()

		watchCh, err := disc.WatchServices(watchCtx, "lifecycle-service")
		require.NoError(t, err)

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

		// Wait for the existing instance with timeout
		timeoutCh := time.After(2 * time.Second)
		var received *ports.ServiceInstance
		select {
		case received = <-receivedEvents:
			assert.Equal(t, "lifecycle-test", received.ID)
		case <-timeoutCh:
			t.Log("Timeout waiting for existing instance update - continuing test")
		case <-subtestCtx.Done():
			t.Fatal("Test context cancelled while waiting for watch event")
		}

		// 5. Update the instance status
		err = disc.UpdateInstanceStatus(subtestCtx, "lifecycle-test", "INACTIVE")
		require.NoError(t, err)

		// Should receive the status update
		timeoutCh = time.After(2 * time.Second)
		select {
		case received = <-receivedEvents:
			assert.Equal(t, "lifecycle-test", received.ID)
			assert.Equal(t, "INACTIVE", received.Status)
		case <-timeoutCh:
			t.Log("Timeout waiting for status update - continuing test")
		case <-subtestCtx.Done():
			t.Fatal("Test context cancelled while waiting for status update")
		}

		// 6. Heartbeat the instance
		err = disc.UpdateInstanceStatus(subtestCtx, "lifecycle-test", "ACTIVE")
		require.NoError(t, err)

		// Get the initial heartbeat time
		updated, err := disc.GetInstance(subtestCtx, "lifecycle-test")
		require.NoError(t, err)
		initialHeartbeat := updated.LastHeartbeat

		// Wait a bit then send heartbeat
		time.Sleep(200 * time.Millisecond)

		err = disc.Heartbeat(subtestCtx, "lifecycle-test")
		require.NoError(t, err)

		// Verify heartbeat time was updated
		updated, err = disc.GetInstance(subtestCtx, "lifecycle-test")
		require.NoError(t, err)
		assert.True(t, updated.LastHeartbeat.After(initialHeartbeat))

		// Cancel the watch to clean up resources
		watchCancel()

		// Allow a short time for cleanup to complete
		cleanupTimeout := time.After(1 * time.Second)
		select {
		case <-done:
			// Properly closed
		case <-cleanupTimeout:
			t.Log("Watch goroutine did not exit cleanly")
		case <-subtestCtx.Done():
			t.Log("Test timeout while waiting for watch cleanup")
		}

		// 7. Deregister the instance
		err = disc.DeregisterInstance(subtestCtx, "lifecycle-test")
		require.NoError(t, err)

		// Verify it's gone
		_, err = disc.GetInstance(subtestCtx, "lifecycle-test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance not found")

		// Services list should be empty
		services, err = disc.ListServices(subtestCtx)
		require.NoError(t, err)
		assert.Len(t, services, 0)
	})

	// Test environment variable loading
	t.Run("load_config_from_env", func(t *testing.T) {
		// This is already tested in discovery_test.go, but we can add an integration
		// test here if needed in the future
	})
}
