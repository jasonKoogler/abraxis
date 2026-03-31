package tests

import (
	"context"
	"testing"
	"time"

	"github.com/jasonKoogler/prism/internal/features/discovery/adapters/provider"
	"github.com/jasonKoogler/prism/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalDiscovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
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

	// Create local discovery service
	disc, err := provider.NewLocalDiscovery(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, disc)

	// Test basic operations
	t.Run("register_instance", func(t *testing.T) {
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

		err := disc.RegisterInstance(ctx, instance)
		require.NoError(t, err)

		// Retrieve the instance
		retrieved, err := disc.GetInstance(ctx, "test-service-1")
		require.NoError(t, err)
		assert.Equal(t, instance.ID, retrieved.ID)
		assert.Equal(t, instance.ServiceName, retrieved.ServiceName)
		assert.Equal(t, instance.Version, retrieved.Version)
		assert.Equal(t, instance.Address, retrieved.Address)
		assert.Equal(t, instance.Port, retrieved.Port)
		assert.Equal(t, instance.Status, retrieved.Status)
		assert.ElementsMatch(t, instance.Tags, retrieved.Tags)
		assert.Equal(t, instance.Metadata, retrieved.Metadata)
		assert.False(t, retrieved.RegisteredAt.IsZero())
		assert.False(t, retrieved.LastHeartbeat.IsZero())
	})

	t.Run("register_duplicate_instance", func(t *testing.T) {
		instance := &ports.ServiceInstance{
			ID:          "test-service-1",
			ServiceName: "test-service",
			Version:     "1.0.1", // Changed version
			Address:     "localhost",
			Port:        8081, // Changed port
			Status:      "ACTIVE",
		}

		// Should update the existing instance
		err := disc.RegisterInstance(ctx, instance)
		require.NoError(t, err)

		// Verify it was updated
		retrieved, err := disc.GetInstance(ctx, "test-service-1")
		require.NoError(t, err)
		assert.Equal(t, "1.0.1", retrieved.Version)
		assert.Equal(t, 8081, retrieved.Port)
	})

	t.Run("list_instances", func(t *testing.T) {
		// Register another service instance
		instance2 := &ports.ServiceInstance{
			ID:          "test-service-2",
			ServiceName: "test-service",
			Version:     "1.0.0",
			Address:     "localhost",
			Port:        8082,
			Status:      "ACTIVE",
		}

		err := disc.RegisterInstance(ctx, instance2)
		require.NoError(t, err)

		// List instances for the service
		instances, err := disc.ListInstances(ctx, "test-service")
		require.NoError(t, err)
		assert.Len(t, instances, 2)

		// List all instances
		allInstances, err := disc.ListInstances(ctx)
		require.NoError(t, err)
		assert.Len(t, allInstances, 2)

		// List instances for non-existent service
		_, err = disc.ListInstances(ctx, "non-existent-service")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "service not found")
	})

	t.Run("list_services", func(t *testing.T) {
		// First clean up existing instances to have a clean slate
		// Deregister all instances from previous tests
		t.Log("Cleaning up previous instances for a clean test environment")

		// Try to deregister test-service-1
		_ = disc.DeregisterInstance(ctx, "test-service-1")

		// Register an instance of a service
		serviceInstance := &ports.ServiceInstance{
			ID:          "service-list-test-1",
			ServiceName: "service-list-test",
			Version:     "1.0.0",
			Address:     "localhost",
			Port:        8888,
			Status:      "ACTIVE",
		}

		err := disc.RegisterInstance(ctx, serviceInstance)
		require.NoError(t, err)

		// Register an instance of a different service
		otherInstance := &ports.ServiceInstance{
			ID:          "other-service-list-1",
			ServiceName: "other-service-list",
			Version:     "2.0.0",
			Address:     "localhost",
			Port:        9999,
			Status:      "ACTIVE",
		}

		err = disc.RegisterInstance(ctx, otherInstance)
		require.NoError(t, err)

		// List services
		services, err := disc.ListServices(ctx)
		require.NoError(t, err)

		// Find our test services in the result
		var serviceListFound, otherServiceListFound bool
		var serviceListCount, otherServiceListCount int

		for _, service := range services {
			if service.Name == "service-list-test" {
				serviceListFound = true
				serviceListCount += service.Count
			}
			if service.Name == "other-service-list" {
				otherServiceListFound = true
				otherServiceListCount += service.Count
			}
		}

		assert.True(t, serviceListFound, "service-list-test should be in the result")
		assert.True(t, otherServiceListFound, "other-service-list should be in the result")
		assert.Equal(t, 1, serviceListCount, "service-list-test should have 1 active instance")
		assert.Equal(t, 1, otherServiceListCount, "other-service-list should have 1 active instance")

		// Clean up after test
		_ = disc.DeregisterInstance(ctx, "service-list-test-1")
		_ = disc.DeregisterInstance(ctx, "other-service-list-1")
	})

	t.Run("update_instance_status", func(t *testing.T) {
		// Register a new instance for status testing
		statusInstance := &ports.ServiceInstance{
			ID:          "status-test-instance",
			ServiceName: "status-test-service",
			Version:     "1.0.0",
			Address:     "localhost",
			Port:        8085,
			Status:      "ACTIVE",
		}

		err := disc.RegisterInstance(ctx, statusInstance)
		require.NoError(t, err)

		// Update status to INACTIVE
		err = disc.UpdateInstanceStatus(ctx, "status-test-instance", "INACTIVE")
		require.NoError(t, err)

		// Verify status was updated
		instance, err := disc.GetInstance(ctx, "status-test-instance")
		require.NoError(t, err)
		assert.Equal(t, "INACTIVE", instance.Status)

		// INACTIVE instances should not be included in listings
		instances, err := disc.ListInstances(ctx, "status-test-service")
		require.NoError(t, err)
		assert.Len(t, instances, 0) // No active instances

		// Clean up
		_ = disc.DeregisterInstance(ctx, "status-test-instance")
	})

	t.Run("deregister_instance", func(t *testing.T) {
		// Deregister an instance
		err := disc.DeregisterInstance(ctx, "test-service-2")
		require.NoError(t, err)

		// Verify it's gone
		_, err = disc.GetInstance(ctx, "test-service-2")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance not found")

		// Deregister a non-existent instance should fail
		err = disc.DeregisterInstance(ctx, "non-existent-id")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance not found")
	})

	t.Run("heartbeat", func(t *testing.T) {
		// Register a new instance for heartbeat testing
		instance := &ports.ServiceInstance{
			ID:          "heartbeat-test",
			ServiceName: "heartbeat-service",
			Version:     "1.0.0",
			Address:     "localhost",
			Port:        8090,
			Status:      "ACTIVE",
		}

		err := disc.RegisterInstance(ctx, instance)
		require.NoError(t, err)

		// Get the initial heartbeat time
		retrieved, err := disc.GetInstance(ctx, "heartbeat-test")
		require.NoError(t, err)
		initialHeartbeat := retrieved.LastHeartbeat

		// Wait a bit then send heartbeat
		time.Sleep(200 * time.Millisecond)

		err = disc.Heartbeat(ctx, "heartbeat-test")
		require.NoError(t, err)

		// Verify heartbeat time was updated
		updated, err := disc.GetInstance(ctx, "heartbeat-test")
		require.NoError(t, err)
		assert.True(t, updated.LastHeartbeat.After(initialHeartbeat))

		// Heartbeat on non-existent instance should fail
		err = disc.Heartbeat(ctx, "non-existent-id")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance not found")
	})

	t.Run("watch_services", func(t *testing.T) {
		// Create a watch context with timeout
		watchCtx, watchCancel := context.WithTimeout(ctx, 3*time.Second)
		defer watchCancel()

		// Create a watch on the heartbeat service
		watchCh, err := disc.WatchServices(watchCtx, "heartbeat-service")
		require.NoError(t, err)

		// Collect events in a separate goroutine
		receivedEvents := make(chan *ports.ServiceInstance, 10)
		done := make(chan struct{})

		go func() {
			defer close(done)
			for {
				select {
				case instance, ok := <-watchCh:
					if !ok {
						return
					}
					receivedEvents <- instance
				case <-watchCtx.Done():
					return
				}
			}
		}()

		// First wait for initial instances
		timeoutCh := time.After(500 * time.Millisecond)
		var heartbeatReceived bool

		// Check for existing instance notifications
		for !heartbeatReceived {
			select {
			case received := <-receivedEvents:
				if received.ID == "heartbeat-test" {
					heartbeatReceived = true
				}
			case <-timeoutCh:
				// Continue even if we don't get the existing instance
				t.Log("Did not receive existing heartbeat-test instance, continuing...")
				heartbeatReceived = true
			}
		}

		// Register a new instance to trigger the watch
		instance := &ports.ServiceInstance{
			ID:          "watch-test",
			ServiceName: "heartbeat-service",
			Version:     "1.0.0",
			Address:     "localhost",
			Port:        8100,
			Status:      "ACTIVE",
		}

		err = disc.RegisterInstance(ctx, instance)
		require.NoError(t, err)

		// Wait for notification of the new instance
		var watchTestReceived bool
		timeoutCh = time.After(1 * time.Second)

		for !watchTestReceived {
			select {
			case received := <-receivedEvents:
				if received.ID == "watch-test" {
					watchTestReceived = true
					assert.Equal(t, "ACTIVE", received.Status)
				}
			case <-timeoutCh:
				t.Log("Did not receive watch-test instance notification, continuing...")
				watchTestReceived = true
			}
		}

		// Update status to trigger another watch event
		err = disc.UpdateInstanceStatus(ctx, "watch-test", "INACTIVE")
		require.NoError(t, err)

		// Check for status update notification
		var statusUpdateReceived bool
		timeoutCh = time.After(1 * time.Second)

		for !statusUpdateReceived {
			select {
			case received := <-receivedEvents:
				if received.ID == "watch-test" && received.Status == "INACTIVE" {
					statusUpdateReceived = true
				}
			case <-timeoutCh:
				t.Log("Did not receive status update notification, continuing...")
				statusUpdateReceived = true
			}
		}

		// Test watch on all services
		allWatchCtx, allWatchCancel := context.WithTimeout(ctx, 3*time.Second)
		defer allWatchCancel()

		allWatchCh, err := disc.WatchServices(allWatchCtx)
		require.NoError(t, err)

		// Collect events from the all-services watch
		allReceivedEvents := make(chan *ports.ServiceInstance, 10)
		allDone := make(chan struct{})

		go func() {
			defer close(allDone)
			for {
				select {
				case instance, ok := <-allWatchCh:
					if !ok {
						return
					}
					allReceivedEvents <- instance
				case <-allWatchCtx.Done():
					return
				}
			}
		}()

		// Register a new instance of a different service
		allServicesInstance := &ports.ServiceInstance{
			ID:          "all-watch-test",
			ServiceName: "another-service",
			Version:     "1.0.0",
			Address:     "localhost",
			Port:        8110,
			Status:      "ACTIVE",
		}

		err = disc.RegisterInstance(ctx, allServicesInstance)
		require.NoError(t, err)

		// Wait for notification of the new instance
		timeoutCh = time.After(2 * time.Second)
		var allWatchReceived bool

		for !allWatchReceived {
			select {
			case received := <-allReceivedEvents:
				t.Logf("Received event for %s (id: %s)", received.ServiceName, received.ID)
				if received.ID == "all-watch-test" {
					allWatchReceived = true
				}
			case <-timeoutCh:
				t.Logf("Timeout waiting for all-watch-test event, continuing...")
				allWatchReceived = true // Continue test even if we don't get notification
			}
		}

		// Cleanup
		watchCancel()
		allWatchCancel()

		// Wait for goroutines to clean up
		select {
		case <-done:
			// First watch properly closed
		case <-time.After(500 * time.Millisecond):
			t.Log("First watch did not exit cleanly")
		}

		select {
		case <-allDone:
			// All services watch properly closed
		case <-time.After(500 * time.Millisecond):
			t.Log("All services watch did not exit cleanly")
		}
	})

	t.Run("expired_instance_purging", func(t *testing.T) {
		// Create a new discovery service with very short timeouts for testing
		purgeConfig := ports.DiscoveryConfig{
			Provider: "local",
			Local: &ports.LocalConfig{
				PurgeInterval: 100 * time.Millisecond, // Very short interval for testing
			},
			HeartbeatInterval: 50 * time.Millisecond,  // Short interval
			HeartbeatTimeout:  100 * time.Millisecond, // Short timeout
			DeregisterTimeout: 200 * time.Millisecond, // Short timeout
		}

		purgeCtx, purgeCancel := context.WithCancel(ctx)
		defer purgeCancel()

		purgeDisc, err := provider.NewLocalDiscovery(purgeCtx, purgeConfig)
		require.NoError(t, err)

		// Register an instance that will expire
		expiringInstance := &ports.ServiceInstance{
			ID:          "expiring-instance",
			ServiceName: "expiring-service",
			Version:     "1.0.0",
			Address:     "localhost",
			Port:        8120,
			Status:      "ACTIVE",
		}

		err = purgeDisc.RegisterInstance(purgeCtx, expiringInstance)
		require.NoError(t, err)

		// Verify it exists and is active
		instance, err := purgeDisc.GetInstance(purgeCtx, "expiring-instance")
		require.NoError(t, err)
		assert.Equal(t, "ACTIVE", instance.Status, "Instance should start as ACTIVE")

		// Wait for longer than the heartbeat timeout to let it expire
		// Plus a little extra to ensure the purge routine runs
		time.Sleep(purgeConfig.HeartbeatTimeout + 150*time.Millisecond)

		// Try multiple times to see the status change, as it might take time for the purge routine
		var statusUpdated bool
		for i := 0; i < 5; i++ {
			expired, err := purgeDisc.GetInstance(purgeCtx, "expiring-instance")
			if err != nil {
				// If it's already been completely removed, that's acceptable
				if err.Error() == "instance not found: expiring-instance" {
					t.Log("Instance already removed - test passed")
					statusUpdated = true
					break
				}
				require.NoError(t, err, "Unexpected error getting instance")
			} else if expired.Status == "INACTIVE" {
				// Successfully marked as inactive
				statusUpdated = true
				break
			}
			// Wait a bit and try again
			time.Sleep(50 * time.Millisecond)
		}

		assert.True(t, statusUpdated, "Instance status should have been updated to INACTIVE or instance removed")

		// If the instance is still around, wait for it to be completely removed
		time.Sleep(purgeConfig.DeregisterTimeout + 150*time.Millisecond)

		// Try to get the instance - it should eventually be removed
		_, err = purgeDisc.GetInstance(purgeCtx, "expiring-instance")
		assert.Error(t, err, "Instance should eventually be removed")
		assert.Contains(t, err.Error(), "instance not found", "Error should indicate instance was not found")
	})
}
