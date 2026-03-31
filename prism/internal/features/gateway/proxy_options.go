package gateway

import (
	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
	"github.com/jasonKoogler/prism/internal/ports"
)

// ServiceProxyOption defines a functional option for configuring ServiceProxy
type ServiceProxyOption func(*ServiceProxy) error

// WithProxyConfig sets the configuration for ServiceProxy
func WithProxyConfig(cfg *config.Config) ServiceProxyOption {
	return func(sp *ServiceProxy) error {
		if cfg == nil {
			return ErrNilConfig
		}
		sp.config = cfg
		return nil
	}
}

// WithProxyLogger sets the logger for ServiceProxy
func WithProxyLogger(logger *log.Logger) ServiceProxyOption {
	return func(sp *ServiceProxy) error {
		if logger == nil {
			return ErrNilLogger
		}
		sp.logger = logger
		return nil
	}
}

// WithServiceRegistry sets a custom service registry for ServiceProxy
func WithServiceRegistry(registry ports.ServiceRegistry) ServiceProxyOption {
	return func(sp *ServiceProxy) error {
		if registry == nil {
			return ErrNilRegistry
		}
		sp.registry = registry
		return nil
	}
}

// WithConfigDir sets the configuration directory for ServiceProxy
func WithConfigDir(configDir string) ServiceProxyOption {
	return func(sp *ServiceProxy) error {
		if configDir == "" {
			return ErrEmptyConfigDir
		}
		sp.configDir = configDir
		return nil
	}
}

// WithCircuitBreaker sets a circuit breaker for ServiceProxy
func WithCircuitBreaker(cb ports.CircuitBreaker) ServiceProxyOption {
	return func(sp *ServiceProxy) error {
		if cb == nil {
			return ErrNilCircuitBreaker
		}
		sp.circuitBreaker = cb
		return nil
	}
}

// WithRoutingTable sets a custom routing table for ServiceProxy
func WithRoutingTable(routingTable *RoutingTable) ServiceProxyOption {
	return func(sp *ServiceProxy) error {
		if routingTable == nil {
			return ErrNilRoutingTable
		}
		sp.routingTable = routingTable
		return nil
	}
}
