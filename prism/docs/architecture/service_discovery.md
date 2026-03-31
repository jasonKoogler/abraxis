# Service Discovery Integration

This document explains how the service discovery mechanism integrates with the routing system and service proxy to provide dynamic service registration and routing capabilities.

## Architecture Overview

The service discovery system has several key components that work together:

1. **Service Discovery Interface**: Defines a common API for different discovery backends
2. **Discovery Implementations**: Concrete implementations for different providers (Consul, Kubernetes, etc.)
3. **Service Watcher**: Monitors the discovery system for service changes
4. **Service Registration**: Process of registering discovered services with the proxy
5. **Health Checking**: Monitoring the health status of registered services

## Service Discovery Interface

The `ServiceDiscovery` interface in `internal/ports/discovery.go` defines the contract that all discovery implementations must follow:

```go
type ServiceDiscovery interface {
    // RegisterInstance registers a service instance
    RegisterInstance(ctx context.Context, instance *ServiceInstance) error

    // DeregisterInstance removes a service instance
    DeregisterInstance(ctx context.Context, instanceID string) error

    // UpdateInstanceStatus updates a service instance status
    UpdateInstanceStatus(ctx context.Context, instanceID, status string) error

    // ListInstances returns all available instances for a service
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

The `ServiceInstance` struct represents a single instance of a service:

```go
type ServiceInstance struct {
    ID            string
    ServiceName   string
    Version       string
    Address       string
    Port          int
    Status        string
    Tags          []string
    Metadata      map[string]string
    RegisteredAt  time.Time
    LastHeartbeat time.Time
}
```

## Discovery Implementations

The system supports multiple discovery providers through the factory pattern in `internal/adapters/discovery/discovery.go`:

```go
func NewServiceDiscovery(ctx context.Context, config ports.DiscoveryConfig) (ports.ServiceDiscovery, error) {
    // ...
    switch config.Provider {
    case "consul":
        return NewConsulDiscovery(ctx, config)
    case "kubernetes":
        return NewKubernetesDiscovery(ctx, config)
    case "etcd":
        return NewEtcdDiscovery(ctx, config)
    case "local", "":
        return NewLocalDiscovery(ctx, config)
    default:
        return nil, fmt.Errorf("unsupported service discovery provider: %s", config.Provider)
    }
}
```

### Local Discovery

The `LocalDiscovery` implementation provides in-memory service discovery for local development and testing. It maintains:

- A map of service instances by ID
- A map of service instances by service name
- Channels to notify watchers of service changes
- Automatic purging of expired instances

### External Service Discovery

For production environments, the system can use external service discovery mechanisms:

1. **Consul Discovery**: Integrates with HashiCorp Consul
2. **Kubernetes Discovery**: Uses Kubernetes service discovery
3. **Etcd Discovery**: Integrates with etcd for service discovery

## Service Watcher

The application maintains a service watcher that continuously monitors for service changes through the discovery system. This is implemented in the `startServiceWatcher` method of the `App` struct in `internal/app/app.go`:

```go
func (a *App) startServiceWatcher(ctx context.Context) {
    // Create a watcher for all services
    instanceCh, err := a.serviceDiscovery.WatchServices(watchCtx)
    if err != nil {
        a.logger.Error("Failed to start service watcher", log.Error(err))
        return
    }

    // Get initial list of services
    instances, err := a.serviceDiscovery.ListInstances(ctx)
    if err != nil {
        a.logger.Error("Failed to list service instances", log.Error(err))
    } else {
        // Register all existing services
        for _, instance := range instances {
            a.registerServiceInstance(instance)
        }
    }

    // Watch for service changes
    for {
        select {
        case <-ctx.Done():
            return
        case instance, ok := <-instanceCh:
            if !ok {
                // Reconnect logic...
                continue
            }

            // Check if the instance is still active
            if instance.Status == "active" {
                a.registerServiceInstance(instance)
            } else {
                a.deregisterServiceInstance(instance)
            }
        }
    }
}
```

The watcher:

1. Subscribes to a channel of service instance updates
2. Loads the initial set of instances to establish a baseline
3. Processes instance updates as they come in
4. Automatically reconnects if the channel is closed

## Service Registration and Deregistration

When a service instance is discovered, it's registered with the service proxy through the `registerServiceInstance` method:

```go
func (a *App) registerServiceInstance(instance *ports.ServiceInstance) {
    // Create a service config from the instance
    svcConfig := config.ServiceConfig{
        Name:            instance.ServiceName,
        URL:             fmt.Sprintf("%s:%d", instance.Address, instance.Port),
        HealthCheckPath: instance.Metadata["health_check_path"],
        RequiresAuth:    instance.Metadata["requires_auth"] == "true",
        // Additional configuration...
    }

    // Register the service with the proxy
    if err := a.serviceProxy.RegisterService(svcConfig); err != nil {
        a.logger.Error("Failed to register service with proxy",
            log.String("name", instance.ServiceName),
            log.Error(err))
    }
}
```

Similarly, when a service instance is no longer available, it's deregistered through the `deregisterServiceInstance` method:

```go
func (a *App) deregisterServiceInstance(instance *ports.ServiceInstance) {
    // Remove the service from the proxy
    if err := a.serviceProxy.DeregisterService(instance.ServiceName); err != nil {
        a.logger.Error("Failed to deregister service from proxy",
            log.String("name", instance.ServiceName),
            log.Error(err))
    }
}
```

## Service Metadata

Service instances can include metadata that influences how they're registered with the proxy:

- `health_check_path`: Path for health checks
- `requires_auth`: Whether the service requires authentication
- `allowed_methods`: Comma-separated list of allowed HTTP methods
- `timeout`: Request timeout duration
- `retry_count`: Number of retry attempts for failed requests

## Integration with Service Proxy

The service discovery and service proxy components work together to enable dynamic routing:

1. The discovery system finds and monitors service instances
2. The service watcher receives updates about instances
3. The service proxy is updated with the latest instance information
4. The routing table is updated to reflect the available services and their routes

This integration enables:

- Automatic load balancing across service instances
- Fault tolerance through circuit breaking
- Dynamic routing based on service availability
- Health monitoring and automatic service deregistration

## Health Checking

Health checking occurs at two levels:

1. **Discovery-level Health Checks**: Performed by the discovery mechanism (e.g., Consul, Kubernetes)
2. **Proxy-level Health Checks**: Performed by the service proxy through configured health check paths

The health check results determine whether a service is considered available for routing.

## Configuration

Service discovery is configured through the `DiscoveryConfig` struct, which can be loaded from environment variables:

```go
type DiscoveryConfig struct {
    Provider          string
    Consul            *ConsulConfig
    Kubernetes        *KubernetesConfig
    Etcd              *EtcdConfig
    Local             *LocalConfig
    HeartbeatInterval time.Duration
    HeartbeatTimeout  time.Duration
    DeregisterTimeout time.Duration
}
```

Environment variables include:

- `SERVICE_DISCOVERY_PROVIDER`: The discovery provider to use ("consul", "kubernetes", "etcd", or "local")
- Provider-specific configuration variables (e.g., `CONSUL_ADDRESS`, `K8S_NAMESPACE`)
- Heartbeat and timeout configurations

## Sequence Diagram

```
┌─────────┐     ┌──────────────┐     ┌─────────────┐     ┌───────────┐
│ Service │     │ Discovery    │     │ App         │     │ Service   │
│ Instance│     │ System       │     │ (Watcher)   │     │ Proxy     │
└────┬────┘     └──────┬───────┘     └──────┬──────┘     └─────┬─────┘
     │                 │                    │                  │
     │   Register      │                    │                  │
     │───────────────► │                    │                  │
     │                 │                    │                  │
     │                 │  Update Channel    │                  │
     │                 │ ──────────────────►│                  │
     │                 │                    │                  │
     │                 │                    │   Register       │
     │                 │                    │─────────────────►│
     │                 │                    │                  │
     │  Health Check   │                    │                  │
     │◄───────────────┐│                    │                  │
     │                 │                    │                  │
     │                 │  Status Update     │                  │
     │                 │ ──────────────────►│                  │
     │                 │                    │                  │
     │                 │                    │   Update Status  │
     │                 │                    │─────────────────►│
     │                 │                    │                  │
     │    Deregister   │                    │                  │
     │───────────────► │                    │                  │
     │                 │                    │                  │
     │                 │  Update Channel    │                  │
     │                 │ ──────────────────►│                  │
     │                 │                    │                  │
     │                 │                    │   Deregister     │
     │                 │                    │─────────────────►│
     │                 │                    │                  │
└────┴────┘     └──────┴───────┘     └──────┴──────┘     └─────┴─────┘
```

## Best Practices

1. **Graceful Registration and Deregistration**: Services should register on startup and deregister on shutdown
2. **Health Check Endpoints**: All services should provide a standardized health check endpoint
3. **Consistent Metadata**: Use consistent metadata fields across all services
4. **Redundancy**: Use multiple discovery instances in production for high availability
5. **Monitoring**: Monitor the discovery system itself for availability and performance
