package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jasonKoogler/abraxis/prism/internal/config"
	"github.com/jasonKoogler/abraxis/prism/internal/ports"
)

// ServiceRepository provides persistence for service configurations
type ServiceRepository struct {
	filePath string
	mu       sync.RWMutex
}

var _ ports.ServiceRepository = &ServiceRepository{}

// NewServiceRepository creates a new service repository
func NewServiceRepository(configDir string) (*ServiceRepository, error) {
	// Ensure the config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	filePath := filepath.Join(configDir, "services.json")

	return &ServiceRepository{
		filePath: filePath,
	}, nil
}

// Load loads service configurations from storage
func (sr *ServiceRepository) Load() ([]config.ServiceConfig, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	// Check if the file exists
	if _, err := os.Stat(sr.filePath); os.IsNotExist(err) {
		// File doesn't exist, return empty slice
		return []config.ServiceConfig{}, nil
	}

	// Read the file
	data, err := os.ReadFile(sr.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read services file: %w", err)
	}

	// Parse the JSON
	var services []serviceDTO
	if err := json.Unmarshal(data, &services); err != nil {
		return nil, fmt.Errorf("failed to parse services file: %w", err)
	}

	// Convert DTOs to domain objects
	result := make([]config.ServiceConfig, len(services))
	for i, svc := range services {
		timeout, err := time.ParseDuration(svc.Timeout)
		if err != nil {
			timeout = 30 * time.Second // Default timeout
		}

		// todo: this should be a domain.Service object
		result[i] = config.ServiceConfig{
			Name:            svc.Name,
			URL:             svc.URL,
			HealthCheckPath: svc.HealthCheckPath,
			RequiresAuth:    svc.RequiresAuth,
			Timeout:         timeout,
			RetryCount:      svc.RetryCount,
			AllowedRoles:    svc.AllowedRoles,
			AllowedMethods:  svc.AllowedMethods,
		}
	}

	return result, nil
}

// Save saves service configurations to storage
func (sr *ServiceRepository) Save(services []config.ServiceConfig) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	// Convert domain objects to DTOs
	dtos := make([]serviceDTO, len(services))
	for i, svc := range services {
		dtos[i] = serviceDTO{
			Name:            svc.Name,
			URL:             svc.URL,
			HealthCheckPath: svc.HealthCheckPath,
			RequiresAuth:    svc.RequiresAuth,
			Timeout:         svc.Timeout.String(),
			RetryCount:      svc.RetryCount,
			AllowedRoles:    svc.AllowedRoles,
			AllowedMethods:  svc.AllowedMethods,
		}
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(dtos, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal services: %w", err)
	}

	// Write to file
	if err := os.WriteFile(sr.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write services file: %w", err)
	}

	return nil
}

// serviceDTO is a data transfer object for service configuration
type serviceDTO struct {
	Name            string   `json:"name"`
	URL             string   `json:"url"`
	HealthCheckPath string   `json:"health_check_path"`
	RequiresAuth    bool     `json:"requires_auth"`
	Timeout         string   `json:"timeout"`
	RetryCount      int      `json:"retry_count"`
	AllowedRoles    []string `json:"allowed_roles"`
	AllowedMethods  []string `json:"allowed_methods"`
}
