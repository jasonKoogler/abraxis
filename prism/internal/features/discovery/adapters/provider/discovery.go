package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jasonKoogler/abraxis/prism/internal/ports"
)

// NewServiceDiscovery creates a new service discovery based on configuration
func NewServiceDiscovery(ctx context.Context, config ports.DiscoveryConfig) (ports.ServiceDiscoverer, error) {
	// Set default values if not specified
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 15 * time.Second
	}

	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 60 * time.Second
	}

	if config.DeregisterTimeout == 0 {
		config.DeregisterTimeout = 120 * time.Second
	}

	// Create the appropriate service discovery implementation
	switch config.Provider {
	case "consul":
		if config.Consul == nil {
			return nil, fmt.Errorf("consul configuration is required for consul provider")
		}
		return NewConsulDiscovery(ctx, config)

	case "kubernetes":
		if config.Kubernetes == nil {
			return nil, fmt.Errorf("kubernetes configuration is required for kubernetes provider")
		}
		return NewKubernetesDiscovery(ctx, config)

	case "etcd":
		if config.Etcd == nil {
			return nil, fmt.Errorf("etcd configuration is required for etcd provider")
		}
		return NewEtcdDiscovery(ctx, config)

	case "local", "":
		// Default to local discovery if not specified
		return NewLocalDiscovery(ctx, config)

	default:
		return nil, fmt.Errorf("unsupported service discovery provider: %s", config.Provider)
	}
}

// LoadDiscoveryConfig loads service discovery configuration from environment variables
func LoadDiscoveryConfig() ports.DiscoveryConfig {
	return ports.DiscoveryConfig{
		Provider: getEnv("SERVICE_DISCOVERY_PROVIDER", "local"),
		Consul: &ports.ConsulConfig{
			Address: getEnv("CONSUL_ADDRESS", "localhost:8500"),
			Token:   getEnv("CONSUL_TOKEN", ""),
			TLS: ports.TLSConfig{
				Enabled:    getEnvBool("CONSUL_TLS_ENABLED", false),
				CACertPath: getEnv("CONSUL_TLS_CA_CERT", ""),
				CertPath:   getEnv("CONSUL_TLS_CERT", ""),
				KeyPath:    getEnv("CONSUL_TLS_KEY", ""),
			},
		},
		Kubernetes: &ports.KubernetesConfig{
			InCluster:  getEnvBool("K8S_IN_CLUSTER", false),
			ConfigPath: getEnv("K8S_CONFIG_PATH", ""),
			Namespace:  getEnv("K8S_NAMESPACE", "default"),
		},
		Etcd: &ports.EtcdConfig{
			Endpoints: strings.Split(getEnv("ETCD_ENDPOINTS", "localhost:2379"), ","),
			Username:  getEnv("ETCD_USERNAME", ""),
			Password:  getEnv("ETCD_PASSWORD", ""),
			TLS: ports.TLSConfig{
				Enabled:    getEnvBool("ETCD_TLS_ENABLED", false),
				CACertPath: getEnv("ETCD_TLS_CA_CERT", ""),
				CertPath:   getEnv("ETCD_TLS_CERT", ""),
				KeyPath:    getEnv("ETCD_TLS_KEY", ""),
			},
		},
		Local: &ports.LocalConfig{
			PurgeInterval: getEnvDuration("LOCAL_PURGE_INTERVAL", 30*time.Second),
		},
		HeartbeatInterval: getEnvDuration("HEARTBEAT_INTERVAL", 15*time.Second),
		HeartbeatTimeout:  getEnvDuration("HEARTBEAT_TIMEOUT", 60*time.Second),
		DeregisterTimeout: getEnvDuration("DEREGISTER_TIMEOUT", 120*time.Second),
	}
}

// Helper functions for environment variables
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		b, err := strconv.ParseBool(value)
		if err != nil {
			return defaultValue
		}
		return b
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		d, err := time.ParseDuration(value)
		if err != nil {
			return defaultValue
		}
		return d
	}
	return defaultValue
}
