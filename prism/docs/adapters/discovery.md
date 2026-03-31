# Service Discovery Adapter

## Overview

The Service Discovery adapter provides mechanisms for registering, discovering, and monitoring services in a distributed system. It supports multiple backends including Consul, Kubernetes, etcd, and a local in-memory implementation for development and testing.

This adapter enables dynamic service-to-service communication, facilitating resilient microservice architectures by:

1. Automatically discovering available service instances
2. Health monitoring and unavailable instance detection
3. Dynamic updates when service topology changes
4. Support for various deployment environments (local, Kubernetes, etc.)

## Port Interface

The adapter implements the `ServiceDiscovery` interface from the `ports` package:

```go
type ServiceDiscovery interface {
    // RegisterInstance registers a service instance
    RegisterInstance(ctx context.Context, instance *ServiceInstance) error

    // DeregisterInstance removes a service instance
    DeregisterInstance(ctx context.Context, instanceID string) error

    // UpdateInstanceStatus updates a service instance status
    UpdateInstanceStatus(ctx context.Context, instanceID, status string) error

    // ListInstances returns all available instances for a service
    // If serviceName is empty, returns all instances
    ListInstances(ctx context.Context, serviceName ...string) ([]*ServiceInstance, error)

    // GetInstance returns a specific instance by ID
    GetInstance(ctx context.Context, instanceID string) (*ServiceInstance, error)

    // ListServices returns information about all registered services
    ListServices(ctx context.Context) ([]*ServiceInfo, error)

    // WatchServices returns a channel that receives instance updates
    WatchServices(ctx context.Context, serviceName ...string) (<-chan *ServiceInstance, error)

    // Heartbeat sends a heartbeat for a service instance
    Heartbeat(ctx context.Context, instanceID string) error
}
```

## Service Instance Data Model

A service instance contains the following information:

```go
type ServiceInstance struct {
    ID            string            // Unique instance identifier
    ServiceName   string            // Name of the service
    Version       string            // Service version
    Address       string            // Host or IP address
    Port          int               // Port number
    Status        string            // Current status (ACTIVE, INACTIVE, etc.)
    Tags          []string          // Service tags for filtering
    Metadata      map[string]string // Additional metadata
    RegisteredAt  time.Time         // Registration timestamp
    LastHeartbeat time.Time         // Last heartbeat timestamp
}
```

## Provider Implementations

### Local Discovery

A simple in-memory implementation for local development and testing:

- No external dependencies
- Fast in-process operations
- Automatic cleanup of expired/stale instances
- Suitable for development and single-node deployments

### Consul Discovery

Integration with HashiCorp Consul for robust service discovery:

- Full support for Consul service registration
- Health check integration
- Service metadata and tagging
- Watch functionality for real-time updates
- TLS support for secure communication

### Kubernetes Discovery

Native integration with Kubernetes service discovery:

- Uses Kubernetes API for service discovery
- Pod annotations for service information
- Automatic reconciliation with pod lifecycle
- Namespace-aware service registration
- Leverages Kubernetes watch API for real-time updates

### etcd Discovery

Integration with etcd for distributed service registry:

- Key-value based service registration
- TTL-based expiry for automatic cleanup
- Watch functionality for real-time updates
- Support for etcd v3 API
- Distributed consensus for high availability

## Configuration Options

The adapter is configured using the `DiscoveryConfig` structure:

```go
type DiscoveryConfig struct {
    Provider string         // Provider type: "consul", "kubernetes", "etcd", "local"

    // Common settings
    HeartbeatInterval time.Duration // How often to send heartbeats
    HeartbeatTimeout  time.Duration // When to consider a service stale
    DeregisterTimeout time.Duration // When to automatically deregister

    // Provider-specific configurations
    Consul     *ConsulConfig     // Consul configuration
    Kubernetes *KubernetesConfig // Kubernetes configuration
    Etcd       *EtcdConfig       // etcd configuration
    Local      *LocalConfig      // Local discovery configuration
}
```

## Usage Examples

### Creating a Service Discovery Adapter

```go
// Load configuration from environment
discoveryConfig := discovery.LoadDiscoveryConfig()

// Create service discovery adapter
serviceDiscovery, err := discovery.NewServiceDiscovery(ctx, discoveryConfig)
if err != nil {
    log.Fatal("Failed to create service discovery", err)
}
```

### Registering a Service Instance

```go
// Create service instance
instance := &ports.ServiceInstance{
    ServiceName: "user-service",
    Version:     "1.0.0",
    Address:     "10.0.0.1",
    Port:        8080,
    Status:      "ACTIVE",
    Tags:        []string{"api", "users"},
    Metadata: map[string]string{
        "health_check_path": "/health",
        "requires_auth":     "true",
        "allowed_methods":   "GET,POST,PUT,DELETE",
    },
}

// Register the instance
err := serviceDiscovery.RegisterInstance(ctx, instance)
```

### Discovering Service Instances

```go
// List all instances of a specific service
instances, err := serviceDiscovery.ListInstances(ctx, "user-service")
if err != nil {
    log.Error("Failed to list instances", err)
    return
}

// Use the instances
for _, instance := range instances {
    log.Info("Found instance",
        "id", instance.ID,
        "address", instance.Address,
        "port", instance.Port)
}
```

### Watching for Service Changes

```go
// Create a channel to receive service updates
instanceCh, err := serviceDiscovery.WatchServices(ctx, "user-service")
if err != nil {
    log.Error("Failed to watch services", err)
    return
}

// Process service updates
go func() {
    for {
        select {
        case <-ctx.Done():
            return
        case instance, ok := <-instanceCh:
            if !ok {
                log.Warn("Watch channel closed")
                return
            }

            if instance.Status == "ACTIVE" {
                log.Info("Service instance activated",
                    "id", instance.ID,
                    "address", instance.Address)
                // Handle new/updated instance
            } else {
                log.Info("Service instance deactivated",
                    "id", instance.ID)
                // Handle removed/inactive instance
            }
        }
    }
}()
```

### Sending Heartbeats

```go
// Regular heartbeat sender
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            err := serviceDiscovery.Heartbeat(ctx, instanceID)
            if err != nil {
                log.Error("Failed to send heartbeat", err)
            }
        }
    }
}()
```

## Provider-Specific Considerations

### Consul Configuration

```go
consulConfig := &ports.ConsulConfig{
    Address: "localhost:8500",
    Token:   "secret-token",
    TLS: ports.TLSConfig{
        Enabled:     true,
        CACertPath:  "/path/to/ca.pem",
        CertPath:    "/path/to/cert.pem",
        KeyPath:     "/path/to/key.pem",
    },
}
```

### Kubernetes Configuration

```go
kubeConfig := &ports.KubernetesConfig{
    Namespace:      "default",               // Namespace to use
    LabelSelector:  "app=my-service",        // Label selector for filtering
    KubeconfigPath: "/path/to/kubeconfig",   // Path to kubeconfig (optional)
    InCluster:      true,                    // Use in-cluster config
}
```

### etcd Configuration

```go
etcdConfig := &ports.EtcdConfig{
    Endpoints: []string{"localhost:2379"},
    Username:  "root",
    Password:  "secret",
    TLS: ports.TLSConfig{
        Enabled:     true,
        CACertPath:  "/path/to/ca.pem",
        CertPath:    "/path/to/cert.pem",
        KeyPath:     "/path/to/key.pem",
    },
}
```

## Integration with App

The App struct integrates with the service discovery adapter through:

```go
// WithServiceDiscovery sets a custom service discovery for the App
func WithServiceDiscovery(discovery ports.ServiceDiscovery) AppOption

// WithDefaultServiceDiscovery creates a default service discovery based on config
func WithDefaultServiceDiscovery(ctx context.Context) AppOption
```

## Integration with Service Proxy

The Service Discovery adapter integrates closely with the HTTP adapter's Service Proxy to enable dynamic service routing:

```go
// App initialization with both components
app, err := app.NewApp(
    app.WithConfig(cfg),
    app.WithDefaultServiceDiscovery(ctx),
    app.WithDefaultServiceProxy(),
)

// Start watching for service discovery events
go app.startServiceWatcher(ctx)
```

The service watcher automatically registers/deregisters services with the proxy when they are discovered or become unavailable.

## Fault Tolerance

The adapter implements several mechanisms for fault tolerance:

1. **Heartbeat Monitoring**: Regular health checks to detect service failures
2. **Automatic Cleanup**: Stale service instances are automatically removed
3. **Watch Reconnection**: Automatic reconnection if watch connections are lost
4. **Error Handling**: Graceful degradation when discovery backends are unavailable

## Security Considerations

- TLS support for secure communication with discovery backends
- Token-based authentication for Consul and etcd
- RBAC integration for Kubernetes
- Service metadata for enforcing authentication and authorization requirements
