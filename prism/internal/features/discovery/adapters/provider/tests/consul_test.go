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

// // Mock consul client for testing
// type MockConsulClient struct {
// 	mock.Mock
// }

// // Mock Consul Catalog
// type MockCatalog struct {
// 	mock.Mock
// }

// func (m *MockCatalog) Register(reg *consulapi.CatalogRegistration, q *consulapi.WriteOptions) (*consulapi.WriteMeta, error) {
// 	args := m.Called(reg, q)
// 	return args.Get(0).(*consulapi.WriteMeta), args.Error(1)
// }

// func (m *MockCatalog) Deregister(dereg *consulapi.CatalogDeregistration, q *consulapi.WriteOptions) (*consulapi.WriteMeta, error) {
// 	args := m.Called(dereg, q)
// 	return args.Get(0).(*consulapi.WriteMeta), args.Error(1)
// }

// func (m *MockCatalog) Service(service, tag string, q *consulapi.QueryOptions) ([]*consulapi.CatalogService, *consulapi.QueryMeta, error) {
// 	args := m.Called(service, tag, q)
// 	return args.Get(0).([]*consulapi.CatalogService), args.Get(1).(*consulapi.QueryMeta), args.Error(2)
// }

// func (m *MockCatalog) Services(q *consulapi.QueryOptions) (map[string][]string, *consulapi.QueryMeta, error) {
// 	args := m.Called(q)
// 	return args.Get(0).(map[string][]string), args.Get(1).(*consulapi.QueryMeta), args.Error(2)
// }

// func (m *MockCatalog) Node(node string, q *consulapi.QueryOptions) (*consulapi.CatalogNode, *consulapi.QueryMeta, error) {
// 	args := m.Called(node, q)
// 	return args.Get(0).(*consulapi.CatalogNode), args.Get(1).(*consulapi.QueryMeta), args.Error(2)
// }

// // Mock Consul Health
// type MockHealth struct {
// 	mock.Mock
// }

// func (m *MockHealth) Node(node string, q *consulapi.QueryOptions) ([]*consulapi.HealthCheck, *consulapi.QueryMeta, error) {
// 	args := m.Called(node, q)
// 	return args.Get(0).([]*consulapi.HealthCheck), args.Get(1).(*consulapi.QueryMeta), args.Error(2)
// }

// func (m *MockHealth) Checks(service string, q *consulapi.QueryOptions) ([]*consulapi.HealthCheck, *consulapi.QueryMeta, error) {
// 	args := m.Called(service, q)
// 	return args.Get(0).([]*consulapi.HealthCheck), args.Get(1).(*consulapi.QueryMeta), args.Error(2)
// }

// func (m *MockHealth) Service(service, tag string, passingOnly bool, q *consulapi.QueryOptions) ([]*consulapi.ServiceEntry, *consulapi.QueryMeta, error) {
// 	args := m.Called(service, tag, passingOnly, q)
// 	return args.Get(0).([]*consulapi.ServiceEntry), args.Get(1).(*consulapi.QueryMeta), args.Error(2)
// }

// func (m *MockHealth) State(state string, q *consulapi.QueryOptions) ([]*consulapi.HealthCheck, *consulapi.QueryMeta, error) {
// 	args := m.Called(state, q)
// 	return args.Get(0).([]*consulapi.HealthCheck), args.Get(1).(*consulapi.QueryMeta), args.Error(2)
// }

// // Mock Consul Agent
// type MockAgent struct {
// 	mock.Mock
// }

// func (m *MockAgent) UpdateTTL(checkID, output, status string) error {
// 	args := m.Called(checkID, output, status)
// 	return args.Error(0)
// }

// func (m *MockAgent) ServiceRegister(service *consulapi.AgentServiceRegistration) error {
// 	args := m.Called(service)
// 	return args.Error(0)
// }

// func (m *MockAgent) ServiceDeregister(serviceID string) error {
// 	args := m.Called(serviceID)
// 	return args.Error(0)
// }

// func (m *MockAgent) Self() (map[string]map[string]interface{}, error) {
// 	args := m.Called()
// 	return args.Get(0).(map[string]map[string]interface{}), args.Error(1)
// }

// func (m *MockConsulClient) Catalog() *MockCatalog {
// 	args := m.Called()
// 	return args.Get(0).(*MockCatalog)
// }

// func (m *MockConsulClient) Health() *MockHealth {
// 	args := m.Called()
// 	return args.Get(0).(*MockHealth)
// }

// func (m *MockConsulClient) Agent() *MockAgent {
// 	args := m.Called()
// 	return args.Get(0).(*MockAgent)
// }

// TestConsulDiscoveryWithDockertest tests the Consul discovery implementation with a real Consul server
func TestConsulDiscoveryWithDockertest(t *testing.T) {
	// skipIfNoDocker(t)

	// Setup a Consul container
	consulAddress, cleanup, err := SetupConsulContainer(t)
	if err != nil {
		t.Skip("Failed to create Consul container - skipping test")
	}
	defer cleanup()
	// Create Consul discovery with the real Consul server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := ports.DiscoveryConfig{
		Provider: "consul",
		Consul: &ports.ConsulConfig{
			Address: consulAddress,
		},
		HeartbeatInterval: 1 * time.Second,
		HeartbeatTimeout:  5 * time.Second,
		DeregisterTimeout: 10 * time.Second,
	}

	consulDiscovery, err := provider.NewConsulDiscovery(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, consulDiscovery)

	// Test basic operations
	t.Run("register_and_get_instance", func(t *testing.T) {
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

		// Register instance
		err := consulDiscovery.RegisterInstance(ctx, instance)
		require.NoError(t, err)

		// Get instance
		retrieved, err := consulDiscovery.GetInstance(ctx, "test-service-1")
		require.NoError(t, err)
		assert.Equal(t, instance.ID, retrieved.ID)
		assert.Equal(t, instance.ServiceName, retrieved.ServiceName)
		assert.Equal(t, instance.Version, retrieved.Version)
		assert.Equal(t, instance.Address, retrieved.Address)
		assert.Equal(t, instance.Port, retrieved.Port)
		assert.Equal(t, instance.Status, retrieved.Status)
		assert.ElementsMatch(t, instance.Tags, retrieved.Tags)
		assert.Equal(t, instance.Metadata["region"], retrieved.Metadata["region"])
	})

	t.Run("list_instances", func(t *testing.T) {
		// Register another instance
		instance2 := &ports.ServiceInstance{
			ID:          "test-service-2",
			ServiceName: "test-service",
			Version:     "1.0.0",
			Address:     "localhost",
			Port:        8081,
			Status:      "ACTIVE",
			Tags:        []string{"api", "v1"},
		}

		err := consulDiscovery.RegisterInstance(ctx, instance2)
		require.NoError(t, err)

		// List instances
		instances, err := consulDiscovery.ListInstances(ctx, "test-service")
		require.NoError(t, err)
		assert.Len(t, instances, 2)

		// Verify both instances are returned
		var foundInstance1, foundInstance2 bool
		for _, instance := range instances {
			if instance.ID == "test-service-1" {
				foundInstance1 = true
			}
			if instance.ID == "test-service-2" {
				foundInstance2 = true
			}
		}
		assert.True(t, foundInstance1, "test-service-1 not found in instances list")
		assert.True(t, foundInstance2, "test-service-2 not found in instances list")
	})

	t.Run("heartbeat", func(t *testing.T) {
		// Send heartbeat
		err := consulDiscovery.Heartbeat(ctx, "test-service-1")
		require.NoError(t, err)

		// Verify instance is still active
		instance, err := consulDiscovery.GetInstance(ctx, "test-service-1")
		require.NoError(t, err)
		assert.Equal(t, "ACTIVE", instance.Status)
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

		err := consulDiscovery.RegisterInstance(ctx, otherInstance)
		require.NoError(t, err)

		// Give Consul a moment to register the service
		time.Sleep(1 * time.Second)

		// List services to verify both are registered
		services, err := consulDiscovery.ListServices(ctx)
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

		// There might be multiple service info entries with different versions
		// so we need to count the total number of test-service instances
		var testServiceCount int
		for _, service := range services {
			if service.Name == "test-service" {
				testServiceCount += service.Count
			}
		}

		t.Logf("Found test-service count: %d", testServiceCount)
		t.Logf("Found other-service count: %d", otherServiceInfo.Count)

		assert.Equal(t, 2, testServiceCount, "Should have 2 test-service instances")
		assert.Equal(t, 1, otherServiceInfo.Count, "Should have 1 other-service instance")
	})

	t.Run("deregister_instance", func(t *testing.T) {
		// Deregister an instance
		err := consulDiscovery.DeregisterInstance(ctx, "test-service-2")
		require.NoError(t, err)

		// Verify it's gone
		_, err = consulDiscovery.GetInstance(ctx, "test-service-2")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get service from Consul")

		// Only one instance of test-service should remain
		instances, err := consulDiscovery.ListInstances(ctx, "test-service")
		require.NoError(t, err)
		assert.Len(t, instances, 1)
		assert.Equal(t, "test-service-1", instances[0].ID)
	})

	t.Run("watch_services", func(t *testing.T) {
		// Start a watch for test-service
		watchCtx, watchCancel := context.WithCancel(ctx)
		defer watchCancel()

		watchCh, err := consulDiscovery.WatchServices(watchCtx, "test-service")
		require.NoError(t, err)

		// Register a new instance to trigger the watch
		watchInstance := &ports.ServiceInstance{
			ID:          "watch-test",
			ServiceName: "test-service",
			Version:     "1.0.0",
			Address:     "localhost",
			Port:        8100,
			Status:      "ACTIVE",
			Tags:        []string{"watch"},
		}

		err = consulDiscovery.RegisterInstance(ctx, watchInstance)
		require.NoError(t, err)

		// Use a buffered channel to collect events and a timeout
		receivedEvents := make(chan string, 10)
		timeout := time.After(10 * time.Second)
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
					receivedEvents <- received.ID
				case <-timeout:
					return // Timeout
				case <-watchCtx.Done():
					return // Context cancelled
				}
			}
		}()

		// Wait for a short time to collect events
		time.Sleep(2 * time.Second)

		// Check if we've received the events
		var receivedIDs []string
		collectDone := false

		for !collectDone {
			select {
			case id, ok := <-receivedEvents:
				if !ok {
					collectDone = true
					break
				}
				receivedIDs = append(receivedIDs, id)
			case <-time.After(1 * time.Second):
				collectDone = true
			}
		}

		// Verify we've received events for both instances
		t.Logf("Received IDs: %v", receivedIDs)
		foundOriginal := false
		foundWatch := false

		for _, id := range receivedIDs {
			if id == "test-service-1" {
				foundOriginal = true
			}
			if id == "watch-test" {
				foundWatch = true
			}
		}

		assert.True(t, foundOriginal, "Should have received event for existing instance")
		assert.True(t, foundWatch, "Should have received event for new instance")

		// Deregister the watch instance
		err = consulDiscovery.DeregisterInstance(ctx, "watch-test")
		require.NoError(t, err)

		// Cancel the watch context to clean up
		watchCancel()
	})
}

// // TestConsulDiscovery uses mocks to test the Consul discovery adapter
// func TestConsulDiscovery(t *testing.T) {
// 	// Create mock consul client
// 	mockConsul := &MockConsulClient{}
// 	mockAgent := &MockAgent{}
// 	mockCatalog := &MockCatalog{}
// 	mockHealth := &MockHealth{}

// 	// Setup mocks
// 	mockConsul.On("Agent").Return(mockAgent)
// 	mockConsul.On("Catalog").Return(mockCatalog)
// 	mockConsul.On("Health").Return(mockHealth)

// 	// Setup agent mocks for registration
// 	mockAgent.On("ServiceRegister", mock.Anything).Return(nil)
// 	mockAgent.On("ServiceDeregister", mock.AnythingOfType("string")).Return(nil)
// 	mockAgent.On("Services").Return(map[string]*consulapi.AgentService{
// 		"test-service-1": {
// 			ID:      "test-service-1",
// 			Service: "test-service",
// 			Tags:    []string{"v1", "api"},
// 			Port:    8080,
// 			Address: "localhost",
// 			Meta: map[string]string{
// 				"version": "1.0.0",
// 				"region":  "us-east-1",
// 			},
// 		},
// 	}, nil)
// 	mockAgent.On("Self").Return(map[string]map[string]interface{}{
// 		"Config": {
// 			"NodeName": "test-node",
// 		},
// 	}, nil)

// 	// Setup catalog mocks
// 	mockCatalog.On("Services", mock.Anything).Return(map[string][]string{
// 		"test-service": {"v1", "api"},
// 	}, &consulapi.QueryMeta{}, nil)

// 	mockCatalog.On("Service", "test-service", "", mock.Anything).Return([]*consulapi.CatalogService{
// 		{
// 			ID:          "test-service-1",
// 			ServiceName: "test-service",
// 			ServiceTags: []string{"v1", "api"},
// 			ServiceMeta: map[string]string{
// 				"version": "1.0.0",
// 				"region":  "us-east-1",
// 			},
// 			ServicePort:    8080,
// 			ServiceAddress: "localhost",
// 		},
// 	}, &consulapi.QueryMeta{}, nil)

// 	// Setup health mocks
// 	mockHealth.On("Service", "test-service", "", true, mock.Anything).Return([]*consulapi.ServiceEntry{
// 		{
// 			Service: &consulapi.AgentService{
// 				ID:      "test-service-1",
// 				Service: "test-service",
// 				Tags:    []string{"v1", "api"},
// 				Port:    8080,
// 				Address: "localhost",
// 				Meta: map[string]string{
// 					"version": "1.0.0",
// 					"region":  "us-east-1",
// 				},
// 			},
// 			Checks: consulapi.HealthChecks{
// 				&consulapi.HealthCheck{
// 					Status: "passing",
// 				},
// 			},
// 		},
// 	}, &consulapi.QueryMeta{}, nil)

// 	// Create the consul discovery with the mock
// 	consulConfig := ports.DiscoveryConfig{
// 		Provider: "consul",
// 		Consul: &ports.ConsulConfig{
// 			Address: "localhost:8500",
// 		},
// 		HeartbeatInterval: 1 * time.Second,
// 		HeartbeatTimeout:  5 * time.Second,
// 		DeregisterTimeout: 10 * time.Second,
// 	}

// 	// Create consul discovery
// 	ctx := context.Background()
// 	consulDiscovery, err := discovery.NewConsulDiscovery(ctx, consulConfig)
// 	require.NoError(t, err)

// 	// Inject the mock client
// 	discovery.SetConsulClient(consulDiscovery, mockConsul)

// 	// Verify the provider name
// 	assert.Equal(t, "consul", consulDiscovery.GetProviderName())

// 	// Run the various test cases
// 	t.Run("register_instance", func(t *testing.T) {
// 		instance := &ports.ServiceInstance{
// 			ID:          "test-service-1",
// 			ServiceName: "test-service",
// 			Version:     "1.0.0",
// 			Address:     "localhost",
// 			Port:        8080,
// 			Status:      "ACTIVE",
// 			Tags:        []string{"api", "v1"},
// 			Metadata: map[string]string{
// 				"region": "us-east-1",
// 			},
// 		}

// 		err := consulDiscovery.RegisterInstance(ctx, instance)
// 		require.NoError(t, err)

// 		// Verify the mock was called with expected args
// 		mockAgent.AssertCalled(t, "ServiceRegister", mock.Anything)
// 	})

// 	t.Run("get_instance", func(t *testing.T) {
// 		instance, err := consulDiscovery.GetInstance(ctx, "test-service-1")
// 		require.NoError(t, err)
// 		assert.Equal(t, "test-service-1", instance.ID)
// 		assert.Equal(t, "test-service", instance.ServiceName)
// 	})

// 	t.Run("list_instances", func(t *testing.T) {
// 		instances, err := consulDiscovery.ListInstances(ctx, "test-service")
// 		require.NoError(t, err)
// 		assert.Len(t, instances, 1)
// 	})

// 	t.Run("deregister_instance", func(t *testing.T) {
// 		err := consulDiscovery.DeregisterInstance(ctx, "test-service-1")
// 		require.NoError(t, err)

// 		// Verify the mock was called
// 		mockAgent.AssertCalled(t, "ServiceDeregister", "test-service-1")
// 	})
// }

// // Helper function to create a mock Consul discovery service
// func createMockConsulDiscovery(t *testing.T, mockClient *MockConsulClient, config ports.DiscoveryConfig) *discovery.ConsulDiscovery {
// 	// Create a new ConsulDiscovery instance
// 	ctx := context.Background()
// 	consulDiscovery, err := discovery.NewConsulDiscovery(ctx, config)
// 	require.NoError(t, err)

// 	// Set the mock client using reflection
// 	discovery.SetConsulClient(consulDiscovery, mockClient)

// 	return consulDiscovery
// }

func TestConsulDiscoveryIntegration(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION_TESTS") != "" {
		t.Skip("Skipping integration test")
	}

	// Create Consul container
	consulAddress, purgeFunc, err := SetupConsulContainer(t)
	require.NoError(t, err)
	defer purgeFunc()

	t.Logf("Started Consul container at: %s", consulAddress)

	// Create discovery service
	ctx := context.Background()
	consulConfig := ports.DiscoveryConfig{
		Provider: "consul",
		Consul: &ports.ConsulConfig{
			Address: consulAddress,
		},
		HeartbeatInterval: 1 * time.Second,
		HeartbeatTimeout:  5 * time.Second,
		DeregisterTimeout: 10 * time.Second,
	}

	// Create discovery service
	consulDiscovery, err := provider.NewConsulDiscovery(ctx, consulConfig)
	require.NoError(t, err)

	t.Run("verify_provider_name", func(t *testing.T) {
		assert.Equal(t, "consul", consulDiscovery.GetProviderName())
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

		err := consulDiscovery.RegisterInstance(ctx, instance)
		require.NoError(t, err)

		// Get the instance back
		retrieved, err := consulDiscovery.GetInstance(ctx, "test-service-1")
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

		err := consulDiscovery.RegisterInstance(ctx, instance2)
		require.NoError(t, err)

		// List all instances of the service
		instances, err := consulDiscovery.ListInstances(ctx, "test-service")
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

		err := consulDiscovery.RegisterInstance(ctx, otherInstance)
		require.NoError(t, err)

		// Give Consul a moment to register the service
		time.Sleep(1 * time.Second)

		// List services to verify both are registered
		services, err := consulDiscovery.ListServices(ctx)
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

		// There might be multiple service info entries with different versions
		// so we need to count the total number of test-service instances
		var testServiceCount int
		for _, service := range services {
			if service.Name == "test-service" {
				testServiceCount += service.Count
			}
		}

		t.Logf("Found test-service count: %d", testServiceCount)
		t.Logf("Found other-service count: %d", otherServiceInfo.Count)

		assert.Equal(t, 2, testServiceCount, "Should have 2 test-service instances")
		assert.Equal(t, 1, otherServiceInfo.Count, "Should have 1 other-service instance")
	})

	t.Run("deregister_instance", func(t *testing.T) {
		// Deregister one of the instances
		err := consulDiscovery.DeregisterInstance(ctx, "test-service-1")
		require.NoError(t, err)

		// Verify it's gone
		_, err = consulDiscovery.GetInstance(ctx, "test-service-1")
		assert.Error(t, err, "Instance should be deregistered")
		assert.Contains(t, err.Error(), "failed to get service from Consul")

		// List services to verify count decreased
		services, err := consulDiscovery.ListServices(ctx)
		require.NoError(t, err)

		// Find the test-service among the services
		var testServiceCount int
		for _, service := range services {
			if service.Name == "test-service" {
				testServiceCount += service.Count
			}
		}

		assert.Equal(t, 1, testServiceCount, "Should have 1 service instance remaining after deregistration")
	})

	t.Run("watch_services", func(t *testing.T) {
		// Setup a watcher
		watchCtx, watchCancel := context.WithCancel(ctx)
		defer watchCancel()

		// Watch for test-service changes
		watchCh, err := consulDiscovery.WatchServices(watchCtx, "test-service")
		require.NoError(t, err)

		// Create a new instance to trigger a watch event
		newInstance := &ports.ServiceInstance{
			ID:          "test-service-3",
			ServiceName: "test-service",
			Version:     "1.0.2",
			Address:     "localhost",
			Port:        8082,
			Status:      "ACTIVE",
			Tags:        []string{"api", "v3", "watch-test"},
			Metadata: map[string]string{
				"region": "ap-south-1",
			},
		}

		err = consulDiscovery.RegisterInstance(ctx, newInstance)
		require.NoError(t, err)

		// Use a buffered channel to collect events and a timeout
		receivedEvents := make(chan *ports.ServiceInstance, 10)
		timeout := time.After(10 * time.Second)
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
				case <-timeout:
					return // Timeout
				case <-watchCtx.Done():
					return // Context cancelled
				}
			}
		}()

		// Wait for a short time to collect events
		time.Sleep(2 * time.Second)

		// Check if we've received the events
		var instancesReceived []*ports.ServiceInstance
		collectDone := false

		for !collectDone {
			select {
			case instance, ok := <-receivedEvents:
				if !ok {
					collectDone = true
					break
				}
				instancesReceived = append(instancesReceived, instance)
			case <-time.After(1 * time.Second):
				collectDone = true
			}
		}

		// Log all received instances
		t.Logf("Received %d instances", len(instancesReceived))
		for i, inst := range instancesReceived {
			t.Logf("Instance %d: ID=%s, Status=%s", i, inst.ID, inst.Status)
		}

		// Verify we received our new instance
		var foundNewInstance bool
		for _, instance := range instancesReceived {
			if instance.ID == "test-service-3" {
				foundNewInstance = true
				// The status might be ACTIVE or INACTIVE initially
				// For tests, we'll accept either status
				assert.Contains(t, []string{"ACTIVE", "INACTIVE"}, instance.Status,
					"Instance status should be either ACTIVE or INACTIVE")
				break
			}
		}

		assert.True(t, foundNewInstance, "Should have received the new instance")

		// Deregister to trigger a delete event
		err = consulDiscovery.DeregisterInstance(ctx, "test-service-3")
		require.NoError(t, err)

		// Cancel the watch context to clean up
		watchCancel()
	})
}
