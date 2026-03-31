package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jasonKoogler/prism/internal/ports"
)

// LocalDiscovery provides a simple in-memory service discovery implementation
// suitable for local development and testing
type LocalDiscovery struct {
	instances      map[string]*ports.ServiceInstance            // instanceID -> instance
	serviceMap     map[string]map[string]*ports.ServiceInstance // serviceName -> (instanceID -> instance)
	watchChannels  map[string][]chan *ports.ServiceInstance     // serviceName -> channels
	config         ports.DiscoveryConfig
	mu             sync.RWMutex
	purgeTicker    *time.Ticker
	purgeCtx       context.Context
	purgeCtxCancel context.CancelFunc
}

var _ ports.ServiceDiscoverer = &LocalDiscovery{}

// NewLocalDiscovery creates a new local service discovery
func NewLocalDiscovery(ctx context.Context, config ports.DiscoveryConfig) (*LocalDiscovery, error) {
	purgeCtx, purgeCtxCancel := context.WithCancel(context.Background())

	// Set default purge interval if not provided
	purgeInterval := 30 * time.Second
	if config.Local != nil && config.Local.PurgeInterval > 0 {
		purgeInterval = config.Local.PurgeInterval
	}

	disc := &LocalDiscovery{
		instances:      make(map[string]*ports.ServiceInstance),
		serviceMap:     make(map[string]map[string]*ports.ServiceInstance),
		watchChannels:  make(map[string][]chan *ports.ServiceInstance),
		config:         config,
		purgeTicker:    time.NewTicker(purgeInterval),
		purgeCtx:       purgeCtx,
		purgeCtxCancel: purgeCtxCancel,
	}

	// Start purging expired instances
	go disc.purgeExpiredInstances()

	return disc, nil
}

func (d *LocalDiscovery) GetProviderName() string {
	return "local"
}

// RegisterInstance registers a service instance
func (d *LocalDiscovery) RegisterInstance(ctx context.Context, instance *ports.ServiceInstance) error {
	// Make a copy of the instance to avoid race conditions
	instanceCopy := *instance

	d.mu.Lock()

	// Set registered time if not already set
	if instanceCopy.RegisteredAt.IsZero() {
		instanceCopy.RegisteredAt = time.Now()
	}

	// Set last heartbeat to current time
	instanceCopy.LastHeartbeat = time.Now()

	// Set status to active if not already set
	if instanceCopy.Status == "" {
		instanceCopy.Status = "ACTIVE"
	}

	// Store instance
	d.instances[instanceCopy.ID] = &instanceCopy

	// Update service map
	if _, exists := d.serviceMap[instanceCopy.ServiceName]; !exists {
		d.serviceMap[instanceCopy.ServiceName] = make(map[string]*ports.ServiceInstance)
	}
	d.serviceMap[instanceCopy.ServiceName][instanceCopy.ID] = &instanceCopy

	// Create a copy for notification outside the lock
	notifyInstance := instanceCopy

	d.mu.Unlock()

	// Notify watchers without holding the lock
	d.notifyWatchers(&notifyInstance)

	return nil
}

// DeregisterInstance removes a service instance
func (d *LocalDiscovery) DeregisterInstance(ctx context.Context, instanceID string) error {
	var instanceToNotify *ports.ServiceInstance

	d.mu.Lock()
	instance, exists := d.instances[instanceID]
	if !exists {
		d.mu.Unlock()
		return fmt.Errorf("instance not found: %s", instanceID)
	}

	// Make a copy for notification
	instanceCopy := *instance
	instanceCopy.Status = "DEREGISTERED"
	instanceToNotify = &instanceCopy

	// Remove instance
	delete(d.instances, instanceID)
	delete(d.serviceMap[instance.ServiceName], instanceID)

	// Clean up empty service map entry
	if len(d.serviceMap[instance.ServiceName]) == 0 {
		delete(d.serviceMap, instance.ServiceName)
	}
	d.mu.Unlock()

	// Notify watchers without holding the lock
	d.notifyWatchers(instanceToNotify)

	return nil
}

// UpdateInstanceStatus updates a service instance status
func (d *LocalDiscovery) UpdateInstanceStatus(ctx context.Context, instanceID, status string) error {
	var instanceToNotify *ports.ServiceInstance

	d.mu.Lock()
	instance, exists := d.instances[instanceID]
	if !exists {
		d.mu.Unlock()
		return fmt.Errorf("instance not found: %s", instanceID)
	}

	// Update status
	instance.Status = status

	// Make a copy for notification
	instanceCopy := *instance
	instanceToNotify = &instanceCopy

	d.mu.Unlock()

	// Notify watchers without holding the lock
	d.notifyWatchers(instanceToNotify)

	return nil
}

// ListInstances returns all available instances for a service
func (d *LocalDiscovery) ListInstances(ctx context.Context, serviceName ...string) ([]*ports.ServiceInstance, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var instances []*ports.ServiceInstance

	if len(serviceName) == 0 || serviceName[0] == "" {
		// Return all instances
		for _, instance := range d.instances {
			if instance.Status == "ACTIVE" {
				instances = append(instances, instance)
			}
		}
	} else {
		// Return instances for the specified service
		service := serviceName[0]
		serviceInstances, exists := d.serviceMap[service]
		if !exists {
			return nil, fmt.Errorf("service not found: %s", service)
		}

		for _, instance := range serviceInstances {
			if instance.Status == "ACTIVE" {
				instances = append(instances, instance)
			}
		}
	}

	return instances, nil
}

// GetInstance returns a specific instance by ID
func (d *LocalDiscovery) GetInstance(ctx context.Context, instanceID string) (*ports.ServiceInstance, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	instance, exists := d.instances[instanceID]
	if !exists {
		return nil, fmt.Errorf("instance not found: %s", instanceID)
	}

	return instance, nil
}

// ListServices returns information about all registered services
func (d *LocalDiscovery) ListServices(ctx context.Context) ([]*ports.ServiceInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var services []*ports.ServiceInfo
	serviceVersions := make(map[string]map[string]int) // serviceName -> (version -> count)

	// Count instances per service and version
	for serviceName, serviceInstances := range d.serviceMap {
		versions := make(map[string]int)

		for _, instance := range serviceInstances {
			if instance.Status == "ACTIVE" {
				versions[instance.Version]++
			}
		}

		serviceVersions[serviceName] = versions
	}

	// Create service info objects
	for serviceName, versions := range serviceVersions {
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

// WatchServices returns a channel that receives instance updates
func (d *LocalDiscovery) WatchServices(ctx context.Context, serviceName ...string) (<-chan *ports.ServiceInstance, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Create channel for updates
	ch := make(chan *ports.ServiceInstance, 10)

	// Determine which services to watch
	var watchServices []string
	if len(serviceName) == 0 || serviceName[0] == "" {
		// Watch all services
		for service := range d.serviceMap {
			watchServices = append(watchServices, service)
		}
	} else {
		watchServices = serviceName
	}

	// Register watch channels
	for _, service := range watchServices {
		d.watchChannels[service] = append(d.watchChannels[service], ch)

		// Send initial state
		if serviceInstances, exists := d.serviceMap[service]; exists {
			for _, instance := range serviceInstances {
				if instance.Status == "ACTIVE" {
					go func(inst *ports.ServiceInstance) {
						ch <- inst
					}(instance)
				}
			}
		}
	}

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		d.removeWatchChannel(ch)
	}()

	return ch, nil
}

// Heartbeat sends a heartbeat for a service instance
func (d *LocalDiscovery) Heartbeat(ctx context.Context, instanceID string) error {
	var instanceToNotify *ports.ServiceInstance
	var statusChanged bool

	d.mu.Lock()
	instance, exists := d.instances[instanceID]
	if !exists {
		d.mu.Unlock()
		return fmt.Errorf("instance not found: %s", instanceID)
	}

	// Update heartbeat time
	instance.LastHeartbeat = time.Now()

	// If instance was previously marked as inactive, make it active again
	if instance.Status == "INACTIVE" {
		instance.Status = "ACTIVE"
		statusChanged = true
	}

	// Make a copy for notification if status changed
	if statusChanged {
		instanceCopy := *instance
		instanceToNotify = &instanceCopy
	}

	d.mu.Unlock()

	// Notify watchers without holding the lock if status changed
	if statusChanged && instanceToNotify != nil {
		d.notifyWatchers(instanceToNotify)
	}

	return nil
}

// Close shuts down the local discovery service
func (d *LocalDiscovery) Close() error {
	// First stop the purge ticker and cancel the context
	d.purgeCtxCancel()
	d.purgeTicker.Stop()

	// Create a snapshot of channels to close
	channelsToClose := make([]chan *ports.ServiceInstance, 0)

	// Get a snapshot of all channels while holding the lock
	d.mu.Lock()
	for _, channels := range d.watchChannels {
		for _, ch := range channels {
			channelsToClose = append(channelsToClose, ch)
		}
	}

	// Clear the watch channels map while we have the lock
	d.watchChannels = make(map[string][]chan *ports.ServiceInstance)
	d.mu.Unlock()

	// Now close all channels without holding the lock
	for _, ch := range channelsToClose {
		// Close each channel safely
		safeClose(ch)
	}

	return nil
}

// safeClose attempts to close a channel if it's not already closed
func safeClose(ch chan *ports.ServiceInstance) {
	defer func() {
		// Recover from panic if channel is already closed
		if r := recover(); r != nil {
			// Channel was already closed
		}
	}()

	close(ch)
}

// Helper methods

// notifyWatchers sends instance update to all registered watchers
func (d *LocalDiscovery) notifyWatchers(instance *ports.ServiceInstance) {
	// Make a copy of the instance to avoid race conditions
	instanceCopy := *instance

	// Get a snapshot of channels to notify
	var channels []chan *ports.ServiceInstance

	d.mu.RLock()
	// Get channels watching this service
	serviceChannels, hasServiceChannels := d.watchChannels[instance.ServiceName]
	if hasServiceChannels {
		channels = append(channels, serviceChannels...)
	}

	// Also include channels watching all services
	allServiceChannels, hasAllChannels := d.watchChannels[""]
	if hasAllChannels {
		channels = append(channels, allServiceChannels...)
	}
	d.mu.RUnlock()

	if len(channels) == 0 {
		return // No watchers to notify
	}

	// Send update to all channels without holding the lock
	for _, ch := range channels {
		ch := ch // Create a local copy to avoid race in goroutines
		go func() {
			// Recover from panic if channel is closed
			defer func() {
				if r := recover(); r != nil {
					// Channel is likely closed, just return
				}
			}()

			// Try to send with a timeout to prevent blocking if the channel is full
			select {
			case ch <- &instanceCopy:
				// Successfully sent
			case <-time.After(100 * time.Millisecond):
				// Channel might be blocked, skip this update
			case <-d.purgeCtx.Done():
				// Context cancelled
				return
			}
		}()
	}
}

// removeWatchChannel removes a watch channel from all services
func (d *LocalDiscovery) removeWatchChannel(ch chan *ports.ServiceInstance) {
	d.mu.Lock()

	// Track if we found the channel
	found := false

	// First pass: find and remove the channel from all service lists
	for service, channels := range d.watchChannels {
		for i, c := range channels {
			if c == ch {
				// Remove channel from the slice
				d.watchChannels[service] = append(channels[:i], channels[i+1:]...)
				found = true

				// Clean up empty service watch list
				if len(d.watchChannels[service]) == 0 {
					delete(d.watchChannels, service)
				}

				// We found this channel in this service, but it might be in others too
				break
			}
		}
	}

	d.mu.Unlock()

	// Only close the channel if we found and removed it
	if found {
		safeClose(ch)
	}
}

// purgeExpiredInstances periodically checks for expired instances
func (d *LocalDiscovery) purgeExpiredInstances() {
	for {
		select {
		case <-d.purgeCtx.Done():
			return
		case <-d.purgeTicker.C:
			d.checkExpiredInstances()
		}
	}
}

// checkExpiredInstances marks instances as inactive if they haven't sent a heartbeat
// and deregisters instances that have been inactive for too long
func (d *LocalDiscovery) checkExpiredInstances() {
	// Create maps to track instances that need notifications
	var inactiveInstances []*ports.ServiceInstance
	var deregisteredInstances []*ports.ServiceInstance
	var instancesToRemove []string

	d.mu.Lock()
	now := time.Now()

	for id, instance := range d.instances {
		// Skip already deregistered instances
		if instance.Status == "DEREGISTERED" {
			continue
		}

		timeSinceHeartbeat := now.Sub(instance.LastHeartbeat)

		if timeSinceHeartbeat > d.config.HeartbeatTimeout && instance.Status == "ACTIVE" {
			// Mark as inactive if heartbeat timeout exceeded
			instance.Status = "INACTIVE"

			// Make a copy for notification
			instanceCopy := *instance
			inactiveInstances = append(inactiveInstances, &instanceCopy)
		} else if timeSinceHeartbeat > d.config.DeregisterTimeout && instance.Status == "INACTIVE" {
			// Deregister if deregister timeout exceeded
			instance.Status = "DEREGISTERED"

			// Make a copy for notification
			instanceCopy := *instance
			deregisteredInstances = append(deregisteredInstances, &instanceCopy)

			// Mark for removal
			instancesToRemove = append(instancesToRemove, id)
		}
	}

	// Remove deregistered instances
	for _, id := range instancesToRemove {
		instance := d.instances[id]
		delete(d.instances, id)
		if service, ok := d.serviceMap[instance.ServiceName]; ok {
			delete(service, id)

			// Clean up empty service map entry
			if len(d.serviceMap[instance.ServiceName]) == 0 {
				delete(d.serviceMap, instance.ServiceName)
			}
		}
	}
	d.mu.Unlock()

	// Send notifications without holding the lock
	for _, instance := range inactiveInstances {
		d.notifyWatchers(instance)
	}

	for _, instance := range deregisteredInstances {
		d.notifyWatchers(instance)
	}
}
