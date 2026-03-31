package discovery

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/jasonKoogler/prism/internal/config"
	"github.com/jasonKoogler/prism/internal/ports"
)

// ServiceRegistry provides a thread-safe registry for managing services dynamically
type ServiceRegistry struct {
	mu         sync.RWMutex
	services   map[string]*ports.ServiceEntry
	repository ports.ServiceRepository
}

var _ ports.ServiceRegistry = &ServiceRegistry{}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(repository ports.ServiceRepository) *ServiceRegistry {
	return &ServiceRegistry{
		services:   make(map[string]*ports.ServiceEntry),
		repository: repository,
	}
}

// Register adds a new service to the registry
func (sr *ServiceRegistry) Register(svcConfig config.ServiceConfig) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	// Check if service already exists
	if _, exists := sr.services[svcConfig.Name]; exists {
		return fmt.Errorf("service %s already registered", svcConfig.Name)
	}

	// Parse the service URL
	serviceURL, err := url.Parse(svcConfig.URL)
	if err != nil {
		return fmt.Errorf("invalid service URL: %w", err)
	}

	// Create a new reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(serviceURL)

	// Configure the proxy with default settings
	proxy.Transport = &http.Transport{
		ResponseHeaderTimeout: svcConfig.Timeout,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
	}

	// Add the service to the registry
	sr.services[svcConfig.Name] = &ports.ServiceEntry{
		Config: svcConfig,
		Proxy:  proxy,
	}

	// Persist the updated services
	if sr.repository != nil {
		go sr.persistServices()
	}

	return nil
}

// Deregister removes a service from the registry
func (sr *ServiceRegistry) Deregister(serviceName string) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if _, exists := sr.services[serviceName]; !exists {
		return fmt.Errorf("service %s not found", serviceName)
	}

	delete(sr.services, serviceName)

	// Persist the updated services
	if sr.repository != nil {
		go sr.persistServices()
	}

	return nil
}

// Get retrieves a service by name
func (sr *ServiceRegistry) Get(serviceName string) (*ports.ServiceEntry, bool) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	service, exists := sr.services[serviceName]
	return service, exists
}

// List returns a list of all registered services
func (sr *ServiceRegistry) List() []config.ServiceConfig {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	services := make([]config.ServiceConfig, 0, len(sr.services))
	for _, entry := range sr.services {
		services = append(services, entry.Config)
	}

	return services
}

// Update updates an existing service configuration
func (sr *ServiceRegistry) Update(svcConfig config.ServiceConfig) error {
	// First deregister the existing service
	if err := sr.Deregister(svcConfig.Name); err != nil {
		return err
	}

	// Then register the updated service
	return sr.Register(svcConfig)
}

// LoadFromConfig initializes the registry from a config object
func (sr *ServiceRegistry) LoadFromConfig(cfg *config.Config) error {
	for _, svc := range cfg.Services {
		if err := sr.Register(svc); err != nil {
			return err
		}
	}
	return nil
}

// LoadFromRepository loads services from the repository
func (sr *ServiceRegistry) LoadFromRepository() error {
	if sr.repository == nil {
		return nil
	}

	services, err := sr.repository.Load()
	if err != nil {
		return err
	}

	for _, svc := range services {
		if err := sr.Register(svc); err != nil {
			return err
		}
	}

	return nil
}

// persistServices saves the current services to the repository
func (sr *ServiceRegistry) persistServices() {
	if sr.repository == nil {
		return
	}

	services := sr.List()
	if err := sr.repository.Save(services); err != nil {
		// Log the error but don't fail the operation
		fmt.Printf("Error persisting services: %v\n", err)
	}
}
