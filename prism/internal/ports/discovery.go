package ports

import (
	"context"
	"time"
)

// ServiceInstance represents a discovered service instance
type ServiceInstance struct {
	ID            string            `json:"id"`
	ServiceName   string            `json:"service_name"`
	Version       string            `json:"version"`
	Address       string            `json:"address"`
	Port          int               `json:"port"`
	Status        string            `json:"status"`
	Tags          []string          `json:"tags"`
	Metadata      map[string]string `json:"metadata"`
	RegisteredAt  time.Time         `json:"registered_at"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
}

// ServiceInfo represents basic service information
type ServiceInfo struct {
	Name     string            `json:"name"`
	Version  string            `json:"version"`
	Count    int               `json:"count"`
	Metadata map[string]string `json:"metadata"`
}

// ServiceDiscoverer defines the interface for service discovery operations
type ServiceDiscoverer interface {
	// GetProviderName returns the name of the provider
	GetProviderName() string

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

// DiscoveryConfig holds configuration for service discovery
type DiscoveryConfig struct {
	// Provider specifies the discovery mechanism (local, consul, kubernetes, etcd)
	Provider string

	// Consul specific configuration
	Consul *ConsulConfig

	// Kubernetes specific configuration
	Kubernetes *KubernetesConfig

	// Etcd specific configuration
	Etcd *EtcdConfig

	// Local specific configuration
	Local *LocalConfig

	// Common configuration
	HeartbeatInterval time.Duration
	HeartbeatTimeout  time.Duration
	DeregisterTimeout time.Duration
}

// ConsulConfig holds Consul-specific configuration
type ConsulConfig struct {
	Address string
	Token   string
	TLS     TLSConfig
}

// KubernetesConfig holds Kubernetes-specific configuration
type KubernetesConfig struct {
	InCluster  bool
	ConfigPath string
	Namespace  string
}

// EtcdConfig holds etcd-specific configuration
type EtcdConfig struct {
	Endpoints []string
	Username  string
	Password  string
	TLS       TLSConfig
}

// LocalConfig holds local discovery configuration
type LocalConfig struct {
	// Used for testing and development
	PurgeInterval time.Duration
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enabled    bool
	CACertPath string
	CertPath   string
	KeyPath    string
}
