package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/jasonKoogler/prism/internal/ports"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdDiscovery provides service discovery using etcd
type EtcdDiscovery struct {
	client        *clientv3.Client
	config        ports.DiscoveryConfig
	leaseID       clientv3.LeaseID
	watchChannels map[string][]chan *ports.ServiceInstance
	instanceCache map[string]*ports.ServiceInstance
	mu            sync.RWMutex
	ctx           context.Context
	ctxCancel     context.CancelFunc
	watchCancels  map[string]context.CancelFunc
}

var _ ports.ServiceDiscoverer = &EtcdDiscovery{}

const (
	// Base etcd key prefix for service discovery
	etcdKeyPrefix = "/services"
)

// For testing only - allows tests to inject mock clients
// var SetEtcdClient func(d *EtcdDiscovery, client interface{})

// NewEtcdDiscovery creates a new etcd-based service discovery
func NewEtcdDiscovery(ctx context.Context, config ports.DiscoveryConfig) (*EtcdDiscovery, error) {
	// Create etcd client configuration
	etcdConfig := clientv3.Config{
		Endpoints:   config.Etcd.Endpoints,
		DialTimeout: 5 * time.Second,
	}

	if config.Etcd.Username != "" && config.Etcd.Password != "" {
		etcdConfig.Username = config.Etcd.Username
		etcdConfig.Password = config.Etcd.Password
	}

	if config.Etcd.TLS.Enabled {
		// In a real implementation, you would configure TLS here
	}

	// Create etcd client
	client, err := clientv3.New(etcdConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	// Create context for watch operations
	watchCtx, watchCtxCancel := context.WithCancel(context.Background())

	// Create lease for automatic cleanup of service registrations
	lease, err := client.Grant(ctx, int64(config.DeregisterTimeout.Seconds()))
	if err != nil {
		watchCtxCancel()
		client.Close()
		return nil, fmt.Errorf("failed to create etcd lease: %w", err)
	}

	discovery := &EtcdDiscovery{
		client:        client,
		config:        config,
		leaseID:       lease.ID,
		watchChannels: make(map[string][]chan *ports.ServiceInstance),
		instanceCache: make(map[string]*ports.ServiceInstance),
		ctx:           watchCtx,
		ctxCancel:     watchCtxCancel,
		watchCancels:  make(map[string]context.CancelFunc),
	}

	// Start lease keep-alive
	keepAliveCh, err := client.KeepAlive(watchCtx, lease.ID)
	if err != nil {
		watchCtxCancel()
		client.Close()
		return nil, fmt.Errorf("failed to start lease keep-alive: %w", err)
	}

	// Monitor lease keep-alive
	go func() {
		for {
			select {
			case <-watchCtx.Done():
				return
			case _, ok := <-keepAliveCh:
				if !ok {
					// Lease keep-alive failed, try to renew
					lease, err := client.Grant(context.Background(), int64(config.DeregisterTimeout.Seconds()))
					if err != nil {
						// Log error and retry after a delay
						time.Sleep(5 * time.Second)
						continue
					}

					discovery.mu.Lock()
					discovery.leaseID = lease.ID
					discovery.mu.Unlock()

					keepAliveCh, err = client.KeepAlive(watchCtx, lease.ID)
					if err != nil {
						// Log error and retry after a delay
						time.Sleep(5 * time.Second)
						continue
					}
				}
			}
		}
	}()

	return discovery, nil
}

func (d *EtcdDiscovery) GetProviderName() string {
	return "etcd"
}

// RegisterInstance registers a service instance
func (d *EtcdDiscovery) RegisterInstance(ctx context.Context, instance *ports.ServiceInstance) error {
	// Set registered time if not already set
	if instance.RegisteredAt.IsZero() {
		instance.RegisteredAt = time.Now()
	}

	// Set last heartbeat to current time
	instance.LastHeartbeat = time.Now()

	// Set status to active if not already set
	if instance.Status == "" {
		instance.Status = "ACTIVE"
	}

	// Marshal instance to JSON
	data, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("failed to marshal instance data: %w", err)
	}

	// Get lease ID to use
	d.mu.RLock()
	leaseID := d.leaseID
	d.mu.RUnlock()

	// Create etcd key for the instance
	key := path.Join(etcdKeyPrefix, instance.ServiceName, instance.ID)

	// Store instance in etcd with lease
	_, err = d.client.Put(ctx, key, string(data), clientv3.WithLease(leaseID))
	if err != nil {
		return fmt.Errorf("failed to register instance in etcd: %w", err)
	}

	// Update local cache
	d.mu.Lock()
	d.instanceCache[instance.ID] = instance
	d.mu.Unlock()

	return nil
}

// DeregisterInstance removes a service instance
func (d *EtcdDiscovery) DeregisterInstance(ctx context.Context, instanceID string) error {
	// Get service name from cache
	d.mu.RLock()
	instance, exists := d.instanceCache[instanceID]
	d.mu.RUnlock()

	if !exists {
		// Try to find it in etcd
		instances, err := d.ListInstances(ctx)
		if err != nil {
			return fmt.Errorf("failed to list instances: %w", err)
		}

		for _, inst := range instances {
			if inst.ID == instanceID {
				instance = inst
				exists = true
				break
			}
		}

		if !exists {
			return fmt.Errorf("instance not found: %s", instanceID)
		}
	}

	// Delete from etcd
	key := path.Join(etcdKeyPrefix, instance.ServiceName, instanceID)
	_, err := d.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to deregister instance from etcd: %w", err)
	}

	// Update cache
	d.mu.Lock()
	delete(d.instanceCache, instanceID)
	d.mu.Unlock()

	return nil
}

// UpdateInstanceStatus updates a service instance status
func (d *EtcdDiscovery) UpdateInstanceStatus(ctx context.Context, instanceID, status string) error {
	// Get instance from cache
	d.mu.RLock()
	instance, exists := d.instanceCache[instanceID]
	d.mu.RUnlock()

	if !exists {
		return fmt.Errorf("instance not found in cache: %s", instanceID)
	}

	// Update status
	instance.Status = status
	instance.LastHeartbeat = time.Now()

	// Marshal updated instance
	data, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("failed to marshal instance data: %w", err)
	}

	// Get lease ID to use
	d.mu.RLock()
	leaseID := d.leaseID
	d.mu.RUnlock()

	// Update in etcd
	key := path.Join(etcdKeyPrefix, instance.ServiceName, instanceID)
	_, err = d.client.Put(ctx, key, string(data), clientv3.WithLease(leaseID))
	if err != nil {
		return fmt.Errorf("failed to update instance in etcd: %w", err)
	}

	return nil
}

// ListInstances returns all available instances for a service
func (d *EtcdDiscovery) ListInstances(ctx context.Context, serviceName ...string) ([]*ports.ServiceInstance, error) {
	var instances []*ports.ServiceInstance

	if len(serviceName) == 0 || serviceName[0] == "" {
		// List all services
		resp, err := d.client.Get(ctx, etcdKeyPrefix, clientv3.WithPrefix())
		if err != nil {
			return nil, fmt.Errorf("failed to list services from etcd: %w", err)
		}

		for _, kv := range resp.Kvs {
			instance, err := d.unmarshalInstance(kv.Value)
			if err != nil {
				continue
			}

			// Filter out non-active instances
			if instance.Status != "ACTIVE" {
				continue
			}

			instances = append(instances, instance)

			// Update cache
			d.mu.Lock()
			d.instanceCache[instance.ID] = instance
			d.mu.Unlock()
		}
	} else {
		// List specific service
		service := serviceName[0]
		key := path.Join(etcdKeyPrefix, service)

		resp, err := d.client.Get(ctx, key, clientv3.WithPrefix())
		if err != nil {
			return nil, fmt.Errorf("failed to list service instances from etcd: %w", err)
		}

		for _, kv := range resp.Kvs {
			instance, err := d.unmarshalInstance(kv.Value)
			if err != nil {
				continue
			}

			// Filter out non-active instances
			if instance.Status != "ACTIVE" {
				continue
			}

			instances = append(instances, instance)

			// Update cache
			d.mu.Lock()
			d.instanceCache[instance.ID] = instance
			d.mu.Unlock()
		}
	}

	return instances, nil
}

// GetInstance returns a specific instance by ID
func (d *EtcdDiscovery) GetInstance(ctx context.Context, instanceID string) (*ports.ServiceInstance, error) {
	// Check cache first
	d.mu.RLock()
	instance, exists := d.instanceCache[instanceID]
	d.mu.RUnlock()

	if exists {
		return instance, nil
	}

	// Not in cache, need to search in etcd
	// This is inefficient but necessary since we don't know the service name
	instances, err := d.ListInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	for _, inst := range instances {
		if inst.ID == instanceID {
			return inst, nil
		}
	}

	return nil, fmt.Errorf("instance not found: %s", instanceID)
}

// ListServices returns information about all registered services
func (d *EtcdDiscovery) ListServices(ctx context.Context) ([]*ports.ServiceInfo, error) {
	// Get all service directories
	resp, err := d.client.Get(ctx, etcdKeyPrefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, fmt.Errorf("failed to list services from etcd: %w", err)
	}

	// Track services and versions
	serviceVersions := make(map[string]map[string]int) // serviceName -> (version -> count)

	// For each key, extract the service name and instance ID
	for _, kv := range resp.Kvs {
		key := string(kv.Key)

		// Skip the base directory
		if key == etcdKeyPrefix {
			continue
		}

		// Parse key: /services/serviceName/instanceID
		parts := strings.Split(strings.TrimPrefix(key, etcdKeyPrefix+"/"), "/")
		if len(parts) < 2 {
			continue
		}

		serviceName := parts[0]
		instanceID := parts[1]

		// Get the instance data
		instanceKey := path.Join(etcdKeyPrefix, serviceName, instanceID)
		instResp, err := d.client.Get(ctx, instanceKey)
		if err != nil || len(instResp.Kvs) == 0 {
			continue
		}

		instance, err := d.unmarshalInstance(instResp.Kvs[0].Value)
		if err != nil {
			continue
		}

		// Skip non-active instances
		if instance.Status != "ACTIVE" {
			continue
		}

		// Add to service version map
		if _, exists := serviceVersions[serviceName]; !exists {
			serviceVersions[serviceName] = make(map[string]int)
		}

		serviceVersions[serviceName][instance.Version]++
	}

	// Create service info objects
	var services []*ports.ServiceInfo
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
func (d *EtcdDiscovery) WatchServices(ctx context.Context, serviceName ...string) (<-chan *ports.ServiceInstance, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Create channel for updates
	ch := make(chan *ports.ServiceInstance, 10)

	// Determine which key to watch
	watchKey := etcdKeyPrefix
	serviceFilter := ""

	if len(serviceName) > 0 && serviceName[0] != "" {
		serviceFilter = serviceName[0]
		watchKey = path.Join(etcdKeyPrefix, serviceFilter)
	}

	// Register channel
	d.watchChannels[serviceFilter] = append(d.watchChannels[serviceFilter], ch)

	// Start watching if not already watching this service
	if _, exists := d.watchCancels[serviceFilter]; !exists {
		watchCtx, cancel := context.WithCancel(d.ctx)
		d.watchCancels[serviceFilter] = cancel

		go d.watchService(watchCtx, watchKey, serviceFilter)
	}

	// Send initial state
	go func() {
		instances, err := d.ListInstances(ctx, serviceName...)
		if err != nil {
			return
		}

		for _, instance := range instances {
			select {
			case ch <- instance:
				// Successfully sent
			case <-ctx.Done():
				return
			}
		}
	}()

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		d.removeWatchChannel(ch, serviceFilter)
	}()

	return ch, nil
}

// Heartbeat sends a heartbeat for a service instance
func (d *EtcdDiscovery) Heartbeat(ctx context.Context, instanceID string) error {
	// Get instance from cache
	d.mu.RLock()
	instance, exists := d.instanceCache[instanceID]
	d.mu.RUnlock()

	if !exists {
		return fmt.Errorf("instance not found in cache: %s", instanceID)
	}

	// Update heartbeat time
	instance.LastHeartbeat = time.Now()

	// If instance was previously marked as inactive, make it active again
	if instance.Status == "INACTIVE" {
		instance.Status = "ACTIVE"
	}

	// Marshal updated instance
	data, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("failed to marshal instance data: %w", err)
	}

	// Get lease ID to use
	d.mu.RLock()
	leaseID := d.leaseID
	d.mu.RUnlock()

	// Update in etcd
	key := path.Join(etcdKeyPrefix, instance.ServiceName, instanceID)
	_, err = d.client.Put(ctx, key, string(data), clientv3.WithLease(leaseID))
	if err != nil {
		return fmt.Errorf("failed to update heartbeat in etcd: %w", err)
	}

	return nil
}

// Close shuts down the etcd service discovery
func (d *EtcdDiscovery) Close() error {
	d.ctxCancel()

	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel all watch operations
	for service, cancel := range d.watchCancels {
		cancel()
		delete(d.watchCancels, service)
	}

	// Close all watch channels
	for service, channels := range d.watchChannels {
		for _, ch := range channels {
			close(ch)
		}
		delete(d.watchChannels, service)
	}

	// Close etcd client
	return d.client.Close()
}

// Helper methods

// unmarshalInstance converts JSON data to a ServiceInstance
func (d *EtcdDiscovery) unmarshalInstance(data []byte) (*ports.ServiceInstance, error) {
	var instance ports.ServiceInstance
	if err := json.Unmarshal(data, &instance); err != nil {
		return nil, fmt.Errorf("failed to unmarshal instance data: %w", err)
	}
	return &instance, nil
}

// watchService watches for changes in a service
func (d *EtcdDiscovery) watchService(ctx context.Context, watchKey, serviceFilter string) {
	// Watch for changes
	watchChan := d.client.Watch(ctx, watchKey, clientv3.WithPrefix())

	for {
		select {
		case <-ctx.Done():
			return

		case watchResp, ok := <-watchChan:
			if !ok {
				// Channel closed, retry after a delay
				time.Sleep(5 * time.Second)
				watchChan = d.client.Watch(ctx, watchKey, clientv3.WithPrefix())
				continue
			}

			for _, event := range watchResp.Events {
				switch event.Type {
				case clientv3.EventTypePut:
					// New or updated instance
					instance, err := d.unmarshalInstance(event.Kv.Value)
					if err != nil {
						continue
					}

					// Update cache
					d.mu.Lock()
					d.instanceCache[instance.ID] = instance
					d.mu.Unlock()

					// Notify watchers
					d.notifyWatchers(instance, serviceFilter)

				case clientv3.EventTypeDelete:
					// Deleted instance
					key := string(event.Kv.Key)
					parts := strings.Split(strings.TrimPrefix(key, etcdKeyPrefix+"/"), "/")
					if len(parts) < 2 {
						continue
					}

					serviceName := parts[0]
					instanceID := parts[1]

					// Remove from cache
					d.mu.Lock()
					instance, exists := d.instanceCache[instanceID]
					if exists {
						delete(d.instanceCache, instanceID)
					} else {
						instance = &ports.ServiceInstance{
							ID:          instanceID,
							ServiceName: serviceName,
							Status:      "DEREGISTERED",
						}
					}
					d.mu.Unlock()

					// If we had the instance in cache, update status and notify
					if exists {
						instance.Status = "DEREGISTERED"
						d.notifyWatchers(instance, serviceFilter)
					}
				}
			}
		}
	}
}

// notifyWatchers sends instance updates to registered watchers
func (d *EtcdDiscovery) notifyWatchers(instance *ports.ServiceInstance, serviceFilter string) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// If watching a specific service, only notify channels for that service
	if serviceFilter != "" && serviceFilter != instance.ServiceName {
		return
	}

	// Get channels watching this service or all services
	var channels []chan *ports.ServiceInstance

	if serviceChans, exists := d.watchChannels[instance.ServiceName]; exists {
		channels = append(channels, serviceChans...)
	}

	if allChans, exists := d.watchChannels[""]; exists {
		channels = append(channels, allChans...)
	}

	// Send update to all channels
	for _, ch := range channels {
		go func(c chan *ports.ServiceInstance, inst *ports.ServiceInstance) {
			select {
			case c <- inst:
				// Successfully sent
			case <-time.After(100 * time.Millisecond):
				// Channel might be blocked, skip this update
			case <-d.ctx.Done():
				return
			}
		}(ch, instance)
	}
}

// removeWatchChannel removes a watch channel from the specified service
func (d *EtcdDiscovery) removeWatchChannel(ch chan *ports.ServiceInstance, serviceFilter string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	channels, exists := d.watchChannels[serviceFilter]
	if !exists {
		close(ch)
		return
	}

	for i, c := range channels {
		if c == ch {
			// Remove channel
			d.watchChannels[serviceFilter] = append(channels[:i], channels[i+1:]...)

			// Clean up empty service watch list
			if len(d.watchChannels[serviceFilter]) == 0 {
				delete(d.watchChannels, serviceFilter)

				// Stop watching this service if no more channels
				if cancel, exists := d.watchCancels[serviceFilter]; exists {
					cancel()
					delete(d.watchCancels, serviceFilter)
				}
			}

			break
		}
	}

	close(ch)
}
