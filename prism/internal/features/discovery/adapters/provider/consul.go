package provider

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/jasonKoogler/abraxis/prism/internal/ports"
)

// ConsulDialTimeout is the timeout for connecting to Consul - exported for testing
var ConsulDialTimeout = 5 * time.Second

// ConsulDialer is the dialer function used by the Consul client - exported for testing
var ConsulDialer = (&net.Dialer{
	Timeout: ConsulDialTimeout,
}).DialContext

// ConsulDiscovery provides service discovery using HashiCorp Consul
type ConsulDiscovery struct {
	client        *consulapi.Client
	config        ports.DiscoveryConfig
	watchChannels map[string][]chan *ports.ServiceInstance
	watchCancels  map[string]context.CancelFunc
	mu            sync.RWMutex
	ctx           context.Context
	ctxCancel     context.CancelFunc
}

var _ ports.ServiceDiscoverer = &ConsulDiscovery{}

// For testing only - allows tests to inject mock clients
// var SetConsulClient func(d *ConsulDiscovery, client interface{})

// NewConsulDiscovery creates a new Consul-based service discovery
func NewConsulDiscovery(ctx context.Context, config ports.DiscoveryConfig) (*ConsulDiscovery, error) {
	// Create Consul client configuration
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Transport.DialContext = ConsulDialer

	if config.Consul != nil {
		if config.Consul.Address != "" {
			consulConfig.Address = config.Consul.Address
		}

		if config.Consul.Token != "" {
			consulConfig.Token = config.Consul.Token
		}

		if config.Consul.TLS.Enabled {
			consulConfig.TLSConfig = consulapi.TLSConfig{
				CAFile:   config.Consul.TLS.CACertPath,
				CertFile: config.Consul.TLS.CertPath,
				KeyFile:  config.Consul.TLS.KeyPath,
			}
		}
	}

	// Create Consul client
	client, err := consulapi.NewClient(consulConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Consul client: %w", err)
	}

	// Create context for watch operations
	watchCtx, watchCtxCancel := context.WithCancel(context.Background())

	return &ConsulDiscovery{
		client:        client,
		config:        config,
		watchChannels: make(map[string][]chan *ports.ServiceInstance),
		watchCancels:  make(map[string]context.CancelFunc),
		ctx:           watchCtx,
		ctxCancel:     watchCtxCancel,
	}, nil
}

func (d *ConsulDiscovery) GetProviderName() string {
	return "consul"
}

// RegisterInstance registers a service instance
func (d *ConsulDiscovery) RegisterInstance(ctx context.Context, instance *ports.ServiceInstance) error {
	// Create Consul service registration
	registration := &consulapi.AgentServiceRegistration{
		ID:      instance.ID,
		Name:    instance.ServiceName,
		Address: instance.Address,
		Port:    instance.Port,
		Tags:    instance.Tags,
		Meta:    instance.Metadata,
	}

	// Add version as a tag and metadata
	if instance.Version != "" {
		registration.Tags = append(registration.Tags, "version="+instance.Version)

		if registration.Meta == nil {
			registration.Meta = make(map[string]string)
		}
		registration.Meta["version"] = instance.Version
	}

	// Add health check if heartbeats are enabled
	checkInterval := d.config.HeartbeatInterval.String()
	deregisterTimeout := d.config.DeregisterTimeout.String()

	// Create TTL check for the service
	registration.Check = &consulapi.AgentServiceCheck{
		TTL:                            checkInterval,
		DeregisterCriticalServiceAfter: deregisterTimeout,
	}

	// Register service with Consul
	if err := d.client.Agent().ServiceRegister(registration); err != nil {
		return fmt.Errorf("failed to register service with Consul: %w", err)
	}

	// Set initial status
	if instance.Status == "" || instance.Status == "ACTIVE" {
		// Pass the health check to mark the service as healthy
		if err := d.client.Agent().PassTTL("service:"+instance.ID, "Service is healthy"); err != nil {
			return fmt.Errorf("failed to pass initial health check: %w", err)
		}
	}

	return nil
}

// DeregisterInstance removes a service instance
func (d *ConsulDiscovery) DeregisterInstance(ctx context.Context, instanceID string) error {
	// Deregister service from Consul
	if err := d.client.Agent().ServiceDeregister(instanceID); err != nil {
		return fmt.Errorf("failed to deregister service from Consul: %w", err)
	}

	return nil
}

// UpdateInstanceStatus updates a service instance status
func (d *ConsulDiscovery) UpdateInstanceStatus(ctx context.Context, instanceID, status string) error {
	switch status {
	case "ACTIVE":
		// Pass the health check to mark the service as healthy
		return d.client.Agent().PassTTL("service:"+instanceID, "Service is healthy")

	case "INACTIVE", "DEREGISTERED":
		// Fail the health check to mark the service as unhealthy
		return d.client.Agent().FailTTL("service:"+instanceID, "Service is unhealthy")

	default:
		return fmt.Errorf("unsupported status: %s", status)
	}
}

// ListInstances returns all available instances for a service
func (d *ConsulDiscovery) ListInstances(ctx context.Context, serviceName ...string) ([]*ports.ServiceInstance, error) {
	var instances []*ports.ServiceInstance

	// List all services
	serviceMap, err := d.client.Agent().Services()
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	// If no service name provided, list all services
	if len(serviceName) == 0 || serviceName[0] == "" {
		for _, service := range serviceMap {
			instance, err := d.consulServiceToInstance(service)
			if err != nil {
				return nil, err
			}

			// Check if the service is healthy
			healthy, err := d.isServiceHealthy(service.ID)
			if err != nil {
				return nil, err
			}

			if healthy {
				instance.Status = "ACTIVE"
			} else {
				instance.Status = "INACTIVE"
			}

			instances = append(instances, instance)
		}
	} else {
		// List specific service
		servicesToFind := serviceName[0]

		// Get instances for the specific service
		for _, service := range serviceMap {
			if service.Service == servicesToFind {
				instance, err := d.consulServiceToInstance(service)
				if err != nil {
					return nil, err
				}

				// Check if the service is healthy
				healthy, err := d.isServiceHealthy(service.ID)
				if err != nil {
					return nil, err
				}

				if healthy {
					instance.Status = "ACTIVE"
				} else {
					instance.Status = "INACTIVE"
				}

				instances = append(instances, instance)
			}
		}
	}

	return instances, nil
}

// GetInstance returns a specific instance by ID
func (d *ConsulDiscovery) GetInstance(ctx context.Context, instanceID string) (*ports.ServiceInstance, error) {
	// Get service from Consul
	service, meta, err := d.client.Agent().Service(instanceID, &consulapi.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service from Consul: %w", err)
	}

	// todo: find a better use for the meta data
	fmt.Printf("Consul Discovery: GetInstance: meta: %v\n", meta)

	if service == nil {
		return nil, fmt.Errorf("service not found: %s", instanceID)
	}

	// Convert to service instance
	instance, err := d.consulServiceToInstance(service)
	if err != nil {
		return nil, err
	}

	// Check if the service is healthy
	healthy, err := d.isServiceHealthy(instanceID)
	if err != nil {
		return nil, err
	}

	if healthy {
		instance.Status = "ACTIVE"
	} else {
		instance.Status = "INACTIVE"
	}

	return instance, nil
}

// WatchServices returns a channel that receives instance updates
func (d *ConsulDiscovery) WatchServices(ctx context.Context, serviceName ...string) (<-chan *ports.ServiceInstance, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Create channel for updates
	ch := make(chan *ports.ServiceInstance, 10)

	// Determine which services to watch
	var watchServices []string
	if len(serviceName) == 0 || serviceName[0] == "" {
		// Watch all services
		catalogServices, meta, err := d.client.Catalog().Services(&consulapi.QueryOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list catalog services: %w", err)
		}

		// todo: find a better use for the meta data
		fmt.Printf("Consul Discovery: WatchServices: meta: %v\n", meta)

		for service := range catalogServices {
			if service != "consul" { // Skip Consul service
				watchServices = append(watchServices, service)
			}
		}
	} else {
		watchServices = serviceName
	}

	// Register watch channels
	for _, service := range watchServices {
		if _, exists := d.watchChannels[service]; !exists {
			d.watchChannels[service] = []chan *ports.ServiceInstance{}

			// Start watching this service if not already watching
			if _, exists := d.watchCancels[service]; !exists {
				watchCtx, cancel := context.WithCancel(d.ctx)
				d.watchCancels[service] = cancel

				go d.watchService(watchCtx, service)
			}
		}

		d.watchChannels[service] = append(d.watchChannels[service], ch)
	}

	// Send initial state
	go func() {
		defer func() {
			// Safely handle panic if channel is closed
			if r := recover(); r != nil {
				// Channel is likely closed, just return
				return
			}
		}()

		for _, service := range watchServices {
			instances, err := d.ListInstances(ctx, service)
			if err != nil {
				continue
			}

			for _, instance := range instances {
				select {
				case ch <- instance:
					// Successfully sent
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// Handle context cancellation - this ensures cleanup when the watch is canceled
	go func() {
		<-ctx.Done()
		// Just remove the channel from the map, don't close it
		// The channel will be closed by whoever created it
		d.removeWatchChannel(ch)
		// Now it's safe to close the channel after it's been removed from all maps
		close(ch)
	}()

	return ch, nil
}

// Heartbeat updates the TTL check for a service
func (d *ConsulDiscovery) Heartbeat(ctx context.Context, instanceID string) error {
	// Check if service exists
	service, _, err := d.client.Agent().Service(instanceID, &consulapi.QueryOptions{})
	if err != nil {
		return fmt.Errorf("failed to get service: %w", err)
	}

	if service == nil {
		return fmt.Errorf("service not found: %s", instanceID)
	}

	// Pass the health check to mark the service as healthy
	err = d.client.Agent().PassTTL("service:"+instanceID, "Service is healthy")
	if err != nil {
		// If TTL check doesn't exist, refresh registration
		if strings.Contains(err.Error(), "check not found") ||
			strings.Contains(err.Error(), "does not have associated TTL") {
			// Get service instance to re-register
			instance, err := d.GetInstance(ctx, instanceID)
			if err != nil {
				return fmt.Errorf("failed to get service instance: %w", err)
			}

			// Re-register service
			return d.RegisterInstance(ctx, instance)
		}
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	return nil
}

// Close shuts down the Consul service discovery
func (d *ConsulDiscovery) Close() error {
	// Cancel the main context first to stop all background operations
	d.ctxCancel()

	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel all watch operations
	for service, cancel := range d.watchCancels {
		cancel()
		delete(d.watchCancels, service)
	}

	// Instead of closing channels here, just clear the maps
	// Any channel that was returned by WatchServices will be closed by the goroutine
	// created in WatchServices when the context is cancelled
	d.watchChannels = make(map[string][]chan *ports.ServiceInstance)

	return nil
}

// Helper methods

// consulServiceToInstance converts a Consul service to a ServiceInstance
func (d *ConsulDiscovery) consulServiceToInstance(service *consulapi.AgentService) (*ports.ServiceInstance, error) {
	instance := &ports.ServiceInstance{
		ID:          service.ID,
		ServiceName: service.Service,
		Address:     service.Address,
		Port:        service.Port,
		Tags:        []string{},
		Metadata:    make(map[string]string),
	}

	// Copy metadata
	if service.Meta != nil {
		for k, v := range service.Meta {
			instance.Metadata[k] = v
		}
	}

	// Extract version from metadata or tags
	if service.Meta != nil {
		if version, ok := service.Meta["version"]; ok {
			instance.Version = version
		}
	}

	// Copy tags, filtering out version tags to avoid duplication
	if service.Tags != nil {
		for _, tag := range service.Tags {
			if strings.HasPrefix(tag, "version=") {
				if instance.Version == "" {
					instance.Version = strings.TrimPrefix(tag, "version=")
				}
			} else {
				instance.Tags = append(instance.Tags, tag)
			}
		}
	}

	// Try to parse registration time from metadata
	if service.Meta != nil {
		if regTimeStr, ok := service.Meta["registered_at"]; ok {
			if regTime, err := time.Parse(time.RFC3339, regTimeStr); err == nil {
				instance.RegisteredAt = regTime
			}
		}
	}

	return instance, nil
}

// isServiceHealthy checks if a service is healthy
func (d *ConsulDiscovery) isServiceHealthy(serviceID string) (bool, error) {
	checks, err := d.client.Agent().Checks()
	if err != nil {
		return false, fmt.Errorf("failed to get health checks: %w", err)
	}

	for _, check := range checks {
		if check.ServiceID == serviceID {
			return check.Status == "passing", nil
		}
	}

	// If no health check found, consider it unhealthy
	return false, nil
}

// watchService watches for changes in a service
func (d *ConsulDiscovery) watchService(ctx context.Context, serviceName string) {
	var lastIndex uint64

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Watch for service changes
			services, meta, err := d.client.Health().Service(serviceName, "", false, &consulapi.QueryOptions{
				WaitIndex: lastIndex,
				WaitTime:  5 * time.Minute,
			})

			if err != nil {
				time.Sleep(5 * time.Second)
				continue
			}

			// Update the last index for blocking query
			lastIndex = meta.LastIndex

			// Convert to service instances and notify watchers
			var instances []*ports.ServiceInstance
			for _, service := range services {
				instance, err := d.consulServiceToInstance(service.Service)
				if err != nil {
					continue
				}

				// Set status based on health checks
				isHealthy := true
				for _, check := range service.Checks {
					if check.Status != "passing" {
						isHealthy = false
						break
					}
				}

				if isHealthy {
					instance.Status = "ACTIVE"
				} else {
					instance.Status = "INACTIVE"
				}

				instances = append(instances, instance)
			}

			// Notify watchers
			d.notifyWatchers(serviceName, instances)
		}
	}
}

// notifyWatchers sends instance updates to registered watchers
func (d *ConsulDiscovery) notifyWatchers(serviceName string, instances []*ports.ServiceInstance) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Get channels watching this service
	channels, exists := d.watchChannels[serviceName]
	if !exists {
		return
	}

	// Send updates to all channels
	for _, instance := range instances {
		for _, ch := range channels {
			ch := ch         // Create a copy of the variable for the goroutine
			inst := instance // Create a copy of the instance for the goroutine

			// Use a new context that's not tied to the overall context
			// This prevents sending on closed channels after the context is done
			sendCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

			go func() {
				defer cancel()

				select {
				case ch <- inst:
					// Successfully sent
				case <-sendCtx.Done():
					// Timeout or context cancelled
				}
			}()
		}
	}
}

// removeWatchChannel removes a watch channel from all services
func (d *ConsulDiscovery) removeWatchChannel(ch chan *ports.ServiceInstance) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Collect channels to close after removing them from the maps
	// to avoid concurrent map iteration while removing entries
	var channelsToRemove []chan *ports.ServiceInstance

	for service, channels := range d.watchChannels {
		for i, c := range channels {
			if c == ch {
				// Remove channel
				d.watchChannels[service] = append(channels[:i], channels[i+1:]...)
				channelsToRemove = append(channelsToRemove, c)

				// Clean up empty service watch list
				if len(d.watchChannels[service]) == 0 {
					delete(d.watchChannels, service)
				}

				break
			}
		}
	}

	// Only close the channel once even if it was registered for multiple services
	// Avoid closing channels that might still be in use by buffering operations
	// The sender should be responsible for closing the channel
	// Do not close the channel here, it will be closed by the watch context cancellation
}

// ListServices returns information about all registered services
func (d *ConsulDiscovery) ListServices(ctx context.Context) ([]*ports.ServiceInfo, error) {
	// Get catalog services
	catalogServices, meta, err := d.client.Catalog().Services(&consulapi.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list catalog services: %w", err)
	}

	// todo: find a better use for the meta data
	fmt.Printf("Consul Discovery: ListServices: meta: %v\n", meta)

	// Process services
	var services []*ports.ServiceInfo
	serviceVersionMap := make(map[string]map[string]int) // serviceName -> (version -> count)

	for serviceName, tags := range catalogServices {
		// Skip Consul service
		if serviceName == "consul" {
			continue
		}

		fmt.Printf("Consul Discovery: ListServices: serviceName: %s, tags: %v\n", serviceName, tags)

		// Get all service instances
		serviceInstances, _, err := d.client.Health().Service(serviceName, "", false, &consulapi.QueryOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get service instances: %w", err)
		}

		// Initialize serviceVersionMap for this service
		if _, exists := serviceVersionMap[serviceName]; !exists {
			serviceVersionMap[serviceName] = make(map[string]int)
		}

		for _, serviceInstance := range serviceInstances {
			// Skip non-passing services
			passing := true
			for _, check := range serviceInstance.Checks {
				if check.Status != "passing" {
					passing = false
					break
				}
			}
			if !passing {
				continue
			}

			version := "unknown"

			// Try to get version from metadata
			if serviceInstance.Service.Meta != nil {
				if v, ok := serviceInstance.Service.Meta["version"]; ok {
					version = v
				}
			}

			// If not found in metadata, check tags
			if version == "unknown" {
				for _, tag := range serviceInstance.Service.Tags {
					if strings.HasPrefix(tag, "version=") {
						version = strings.TrimPrefix(tag, "version=")
						break
					}
				}
			}

			// Increment count for this version
			serviceVersionMap[serviceName][version]++
		}
	}

	// Convert map to service info slice
	for serviceName, versions := range serviceVersionMap {
		for version, count := range versions {
			services = append(services, &ports.ServiceInfo{
				Name:    serviceName,
				Version: version,
				Count:   count,
			})
		}
	}

	return services, nil
}
