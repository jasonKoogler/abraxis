package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/domain"
	"github.com/jasonKoogler/prism/internal/ports"
)

// BackendServiceStatus represents the status of a backend service
type BackendServiceStatus string

const (
	// ServiceStatusUp indicates the service is healthy and available
	ServiceStatusUp BackendServiceStatus = "up"
	// ServiceStatusDown indicates the service is unavailable
	ServiceStatusDown BackendServiceStatus = "down"
	// ServiceStatusDegraded indicates the service is available but experiencing issues
	ServiceStatusDegraded BackendServiceStatus = "degraded"
	// ServiceStatusUnknown indicates the service status is unknown
	ServiceStatusUnknown BackendServiceStatus = "unknown"
)

// BackendServiceInfo represents information about a backend service
type BackendServiceInfo struct {
	Name            string               `json:"name"`
	URL             string               `json:"url"`
	Status          BackendServiceStatus `json:"status"`
	LastChecked     time.Time            `json:"last_checked"`
	ResponseTime    time.Duration        `json:"response_time"`
	HealthCheckPath string               `json:"health_check_path"`
	Version         string               `json:"version,omitempty"`
	Description     string               `json:"description,omitempty"`
	Tags            []string             `json:"tags,omitempty"`
}

// BackendServiceManager handles the registration and management of backend services
type BackendServiceManager struct {
	apiRouteRepo     ports.APIRouteRepository
	auditService     ports.AuditService
	logger           *log.Logger
	serviceCache     map[string]*BackendServiceInfo
	serviceCacheLock sync.RWMutex
	httpClient       *http.Client
}

// NewBackendServiceManager creates a new backend service manager
func NewBackendServiceManager(
	apiRouteRepo ports.APIRouteRepository,
	auditService ports.AuditService,
	logger *log.Logger,
) *BackendServiceManager {
	return &BackendServiceManager{
		apiRouteRepo: apiRouteRepo,
		auditService: auditService,
		logger:       logger,
		serviceCache: make(map[string]*BackendServiceInfo),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     60 * time.Second,
			},
		},
	}
}

// RegisterRouteParams contains parameters for registering a new API route
type RegisterRouteParams struct {
	PathPattern            string
	HTTPMethod             string
	BackendService         string
	BackendPath            string
	RequiresAuthentication bool
	RequiredScopes         []string
	RateLimitPerMinute     int
	CacheTTLSeconds        int
	TenantID               *uuid.UUID // Optional, only set for tenant-specific routes
}

// RegisterRoute registers a new API route
func (m *BackendServiceManager) RegisterRoute(ctx context.Context, params *RegisterRouteParams, actorID uuid.UUID, actorType string) (*domain.APIRoute, error) {
	// Validate inputs
	if params.PathPattern == "" {
		return nil, fmt.Errorf("path pattern is required")
	}
	if params.HTTPMethod == "" {
		return nil, fmt.Errorf("HTTP method is required")
	}
	if params.BackendService == "" {
		return nil, fmt.Errorf("backend service is required")
	}

	// Validate path pattern format
	if err := m.validatePathPattern(params.PathPattern); err != nil {
		return nil, err
	}

	// Validate HTTP method
	if err := m.validateHTTPMethod(params.HTTPMethod); err != nil {
		return nil, err
	}

	// Check if route already exists
	existingRoute, err := m.apiRouteRepo.GetByPathAndMethod(ctx, params.PathPattern, params.HTTPMethod, params.TenantID)
	if err == nil && existingRoute != nil {
		return nil, fmt.Errorf("route with path '%s' and method '%s' already exists", params.PathPattern, params.HTTPMethod)
	}

	// Check if backend service exists and is healthy
	serviceInfo, err := m.getOrDiscoverService(ctx, params.BackendService)
	if err != nil {
		return nil, fmt.Errorf("backend service validation failed: %w", err)
	}

	// Create the API route
	route := &domain.APIRoute{
		ID:                     uuid.New(),
		PathPattern:            params.PathPattern,
		HTTPMethod:             strings.ToUpper(params.HTTPMethod),
		BackendService:         params.BackendService,
		BackendPath:            params.BackendPath,
		RequiresAuthentication: params.RequiresAuthentication,
		RequiredScopes:         params.RequiredScopes,
		RateLimitPerMinute:     params.RateLimitPerMinute,
		CacheTTLSeconds:        params.CacheTTLSeconds,
		IsActive:               true,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	// Set tenant ID if provided
	if params.TenantID != nil {
		route.TenantID = *params.TenantID
	}

	// Save to repository
	createdRoute, err := m.apiRouteRepo.Create(ctx, route)
	if err != nil {
		return nil, fmt.Errorf("failed to create API route: %w", err)
	}

	// Log the event
	eventData := map[string]interface{}{
		"path_pattern":            params.PathPattern,
		"http_method":             params.HTTPMethod,
		"backend_service":         params.BackendService,
		"backend_service_status":  string(serviceInfo.Status),
		"requires_authentication": params.RequiresAuthentication,
	}

	if len(params.RequiredScopes) > 0 {
		eventData["required_scopes"] = params.RequiredScopes
	}

	var tenantID *uuid.UUID
	if params.TenantID != nil {
		tenantID = params.TenantID
		eventData["tenant_id"] = params.TenantID.String()
	}

	tenantIDStr := ""
	if tenantID != nil {
		tenantIDStr = domain.FormatTenantID(*tenantID)
	}
	m.auditService.LogEvent(ctx, &domain.AuditLogReq{
		EventType:    "apiroute.created",
		ActorType:    actorType,
		ActorID:      actorID.String(),
		TenantID:     tenantIDStr,
		ResourceType: "apiroute",
		ResourceID:   createdRoute.ID.String(),
		EventData:    eventData,
	})

	m.logger.Info("API route registered",
		log.String("path", params.PathPattern),
		log.String("method", params.HTTPMethod),
		log.String("backend_service", params.BackendService),
		log.String("route_id", createdRoute.ID.String()))

	return createdRoute, nil
}

// GetRoute retrieves an API route by ID
func (m *BackendServiceManager) GetRoute(ctx context.Context, id uuid.UUID) (*domain.APIRoute, error) {
	return m.apiRouteRepo.GetByID(ctx, id)
}

// FindRoute finds a route by path pattern and HTTP method
func (m *BackendServiceManager) FindRoute(ctx context.Context, pathPattern, httpMethod string, tenantID *uuid.UUID) (*domain.APIRoute, error) {
	return m.apiRouteRepo.GetByPathAndMethod(ctx, pathPattern, httpMethod, tenantID)
}

// UpdateRouteParams contains parameters for updating an API route
type UpdateRouteParams struct {
	BackendService         *string
	BackendPath            *string
	RequiresAuthentication *bool
	RequiredScopes         []string
	RateLimitPerMinute     *int
	CacheTTLSeconds        *int
	IsActive               *bool
}

// UpdateRoute updates an API route
func (m *BackendServiceManager) UpdateRoute(ctx context.Context, id uuid.UUID, params *UpdateRouteParams, actorID uuid.UUID, actorType string) (*domain.APIRoute, error) {
	// Get the existing route
	route, err := m.apiRouteRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get API route: %w", err)
	}

	// Check if backend service is being updated
	if params.BackendService != nil && *params.BackendService != route.BackendService {
		// Validate the new backend service
		_, err := m.getOrDiscoverService(ctx, *params.BackendService)
		if err != nil {
			return nil, fmt.Errorf("backend service validation failed: %w", err)
		}
		route.BackendService = *params.BackendService
	}

	// Update other fields if provided
	if params.BackendPath != nil {
		route.BackendPath = *params.BackendPath
	}
	if params.RequiresAuthentication != nil {
		route.RequiresAuthentication = *params.RequiresAuthentication
	}
	if params.RequiredScopes != nil {
		route.RequiredScopes = params.RequiredScopes
	}
	if params.RateLimitPerMinute != nil {
		route.RateLimitPerMinute = *params.RateLimitPerMinute
	}
	if params.CacheTTLSeconds != nil {
		route.CacheTTLSeconds = *params.CacheTTLSeconds
	}
	if params.IsActive != nil {
		route.IsActive = *params.IsActive
	}

	route.UpdatedAt = time.Now()

	// Save changes
	updatedRoute, err := m.apiRouteRepo.Update(ctx, id, route)
	if err != nil {
		return nil, fmt.Errorf("failed to update API route: %w", err)
	}

	// Log the event
	eventData := map[string]interface{}{
		"path_pattern":    route.PathPattern,
		"http_method":     route.HTTPMethod,
		"backend_service": route.BackendService,
		"is_active":       route.IsActive,
	}

	var tenantID *uuid.UUID
	if route.TenantID != uuid.Nil {
		tenantID = &route.TenantID
	}

	tenantIDStr := ""
	if tenantID != nil {
		tenantIDStr = domain.FormatTenantID(*tenantID)
	}
	m.auditService.LogEvent(ctx, &domain.AuditLogReq{
		EventType:    "apiroute.updated",
		ActorType:    actorType,
		ActorID:      actorID.String(),
		TenantID:     tenantIDStr,
		ResourceType: "apiroute",
		ResourceID:   route.ID.String(),
		EventData:    eventData,
	})

	m.logger.Info("API route updated",
		log.String("id", id.String()),
		log.String("path", route.PathPattern),
		log.String("method", route.HTTPMethod))

	return updatedRoute, nil
}

// DeleteRoute deletes an API route
func (m *BackendServiceManager) DeleteRoute(ctx context.Context, id uuid.UUID, actorID uuid.UUID, actorType string) error {
	// Get the route first for logging
	route, err := m.apiRouteRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get API route: %w", err)
	}

	// Delete the route
	err = m.apiRouteRepo.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete API route: %w", err)
	}

	// Log the event
	eventData := map[string]interface{}{
		"path_pattern":    route.PathPattern,
		"http_method":     route.HTTPMethod,
		"backend_service": route.BackendService,
	}

	var tenantID *uuid.UUID
	if route.TenantID != uuid.Nil {
		tenantID = &route.TenantID
	}

	tenantIDStr := ""
	if tenantID != nil {
		tenantIDStr = domain.FormatTenantID(*tenantID)
	}
	m.auditService.LogEvent(ctx, &domain.AuditLogReq{
		EventType:    "apiroute.deleted",
		ActorType:    actorType,
		ActorID:      actorID.String(),
		TenantID:     tenantIDStr,
		ResourceType: "apiroute",
		ResourceID:   route.ID.String(),
		EventData:    eventData,
	})

	m.logger.Info("API route deleted",
		log.String("id", id.String()),
		log.String("path", route.PathPattern),
		log.String("method", route.HTTPMethod))

	return nil
}

// ListRoutesByTenant lists all API routes for a tenant
func (m *BackendServiceManager) ListRoutesByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*domain.APIRoute, error) {
	return m.apiRouteRepo.ListByTenant(ctx, tenantID, page, pageSize)
}

// ListRoutesByBackendService lists all API routes for a backend service
func (m *BackendServiceManager) ListRoutesByBackendService(ctx context.Context, backendService string, page, pageSize int) ([]*domain.APIRoute, error) {
	return m.apiRouteRepo.ListByBackendService(ctx, backendService, page, pageSize)
}

// RegisterBackendServiceParams contains parameters for registering a backend service
type RegisterBackendServiceParams struct {
	Name            string
	URL             string
	HealthCheckPath string
	Description     string
	Tags            []string
}

// RegisterBackendService registers a new backend service
func (m *BackendServiceManager) RegisterBackendService(ctx context.Context, params *RegisterBackendServiceParams, actorID uuid.UUID, actorType string) (*BackendServiceInfo, error) {
	// Validate inputs
	if params.Name == "" {
		return nil, fmt.Errorf("service name is required")
	}
	if params.URL == "" {
		return nil, fmt.Errorf("service URL is required")
	}

	// Validate URL format
	if err := m.ValidateBackendServiceURL(params.URL); err != nil {
		return nil, err
	}

	// Set default health check path if not provided
	healthCheckPath := params.HealthCheckPath
	if healthCheckPath == "" {
		healthCheckPath = "/health"
	}

	// Check if service already exists
	m.serviceCacheLock.RLock()
	_, exists := m.serviceCache[params.Name]
	m.serviceCacheLock.RUnlock()

	if exists {
		return nil, fmt.Errorf("backend service '%s' already exists", params.Name)
	}

	// Check service health
	status, responseTime, version, err := m.checkServiceHealth(ctx, params.URL, healthCheckPath)
	if err != nil {
		m.logger.Warn("Service health check failed during registration",
			log.String("service", params.Name),
			log.String("url", params.URL),
			log.Error(err))
		// Continue with registration even if health check fails
		status = ServiceStatusUnknown
	}

	// Create service info
	serviceInfo := &BackendServiceInfo{
		Name:            params.Name,
		URL:             params.URL,
		Status:          status,
		LastChecked:     time.Now(),
		ResponseTime:    responseTime,
		HealthCheckPath: healthCheckPath,
		Version:         version,
		Description:     params.Description,
		Tags:            params.Tags,
	}

	// Add to cache
	m.serviceCacheLock.Lock()
	m.serviceCache[params.Name] = serviceInfo
	m.serviceCacheLock.Unlock()

	// Log the event
	eventData := map[string]interface{}{
		"name":              params.Name,
		"url":               params.URL,
		"health_check_path": healthCheckPath,
		"status":            string(status),
	}

	if version != "" {
		eventData["version"] = version
	}

	m.auditService.LogEvent(ctx, &domain.AuditLogReq{
		EventType:    "backend_service.registered",
		ActorType:    actorType,
		ActorID:      actorID.String(),
		ResourceType: "backend_service",
		EventData:    eventData,
	})

	m.logger.Info("Backend service registered",
		log.String("name", params.Name),
		log.String("url", params.URL),
		log.String("status", string(status)))

	return serviceInfo, nil
}

// GetBackendService gets information about a backend service
func (m *BackendServiceManager) GetBackendService(ctx context.Context, name string) (*BackendServiceInfo, error) {
	// Check cache first
	m.serviceCacheLock.RLock()
	serviceInfo, exists := m.serviceCache[name]
	m.serviceCacheLock.RUnlock()

	if !exists {
		return nil, fmt.Errorf("backend service '%s' not found", name)
	}

	// If the service info is stale (older than 5 minutes), refresh it
	if time.Since(serviceInfo.LastChecked) > 5*time.Minute {
		// Refresh service health in the background
		go func() {
			status, responseTime, version, err := m.checkServiceHealth(context.Background(), serviceInfo.URL, serviceInfo.HealthCheckPath)
			if err != nil {
				m.logger.Warn("Service health check failed during refresh",
					log.String("service", name),
					log.String("url", serviceInfo.URL),
					log.Error(err))
				// Don't update status if health check fails
				return
			}

			m.serviceCacheLock.Lock()
			serviceInfo.Status = status
			serviceInfo.LastChecked = time.Now()
			serviceInfo.ResponseTime = responseTime
			if version != "" {
				serviceInfo.Version = version
			}
			m.serviceCacheLock.Unlock()
		}()
	}

	return serviceInfo, nil
}

// ListBackendServices lists all registered backend services
func (m *BackendServiceManager) ListBackendServices(ctx context.Context) []*BackendServiceInfo {
	m.serviceCacheLock.RLock()
	defer m.serviceCacheLock.RUnlock()

	services := make([]*BackendServiceInfo, 0, len(m.serviceCache))
	for _, service := range m.serviceCache {
		services = append(services, service)
	}

	return services
}

// RefreshBackendServiceHealth refreshes the health status of a backend service
func (m *BackendServiceManager) RefreshBackendServiceHealth(ctx context.Context, name string) (*BackendServiceInfo, error) {
	// Get service info
	m.serviceCacheLock.RLock()
	serviceInfo, exists := m.serviceCache[name]
	m.serviceCacheLock.RUnlock()

	if !exists {
		return nil, fmt.Errorf("backend service '%s' not found", name)
	}

	// Check service health
	status, responseTime, version, err := m.checkServiceHealth(ctx, serviceInfo.URL, serviceInfo.HealthCheckPath)
	if err != nil {
		m.logger.Warn("Service health check failed during refresh",
			log.String("service", name),
			log.String("url", serviceInfo.URL),
			log.Error(err))
		// Update status to down if health check fails
		status = ServiceStatusDown
	}

	// Update service info
	m.serviceCacheLock.Lock()
	serviceInfo.Status = status
	serviceInfo.LastChecked = time.Now()
	serviceInfo.ResponseTime = responseTime
	if version != "" {
		serviceInfo.Version = version
	}
	m.serviceCacheLock.Unlock()

	return serviceInfo, nil
}

// ValidateBackendServiceURL validates a backend service URL
func (m *BackendServiceManager) ValidateBackendServiceURL(serviceURL string) error {
	// Parse the URL to validate it
	parsedURL, err := url.Parse(serviceURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}

	// Check host
	if parsedURL.Host == "" {
		return fmt.Errorf("URL host is required")
	}

	return nil
}

// CheckBackendServiceHealth checks if a backend service is healthy
func (m *BackendServiceManager) CheckBackendServiceHealth(ctx context.Context, serviceURL, healthCheckPath string, timeout time.Duration) error {
	if healthCheckPath == "" {
		healthCheckPath = "/health"
	}

	// Create a client with timeout
	client := &http.Client{
		Timeout: timeout,
	}

	// Create the health check URL
	healthURL := fmt.Sprintf("%s%s", serviceURL, healthCheckPath)

	// Create a request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("health check returned non-success status: %d", resp.StatusCode)
	}

	return nil
}

// Helper methods

// validatePathPattern validates a path pattern format
func (m *BackendServiceManager) validatePathPattern(pathPattern string) error {
	// Path pattern must start with a slash
	if !strings.HasPrefix(pathPattern, "/") {
		return fmt.Errorf("path pattern must start with a slash")
	}

	// Check for invalid characters
	invalidChars := []string{"?", "#", " "}
	for _, char := range invalidChars {
		if strings.Contains(pathPattern, char) {
			return fmt.Errorf("path pattern contains invalid character: '%s'", char)
		}
	}

	// Validate path parameter format (e.g., /:id/ or /{id})
	// This is a simplified validation - in a real implementation, you would use a more robust parser
	bracketParamRegex := regexp.MustCompile(`\{([^{}]+)\}`)
	colonParamRegex := regexp.MustCompile(`/:([^/]+)`)

	// Check for mixed parameter styles
	if bracketParamRegex.MatchString(pathPattern) && colonParamRegex.MatchString(pathPattern) {
		return fmt.Errorf("path pattern cannot mix parameter styles (use either /:id/ or /{id})")
	}

	return nil
}

// validateHTTPMethod validates an HTTP method
func (m *BackendServiceManager) validateHTTPMethod(method string) error {
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	method = strings.ToUpper(method)

	for _, validMethod := range validMethods {
		if method == validMethod {
			return nil
		}
	}

	return fmt.Errorf("invalid HTTP method: %s", method)
}

// getOrDiscoverService gets a service from the cache or discovers it
func (m *BackendServiceManager) getOrDiscoverService(ctx context.Context, serviceName string) (*BackendServiceInfo, error) {
	// Check cache first
	m.serviceCacheLock.RLock()
	serviceInfo, exists := m.serviceCache[serviceName]
	m.serviceCacheLock.RUnlock()

	if exists {
		return serviceInfo, nil
	}

	// Service not in cache, try to discover it
	// In a real implementation, this would use service discovery
	// For now, we'll just return an error
	return nil, fmt.Errorf("backend service '%s' not found", serviceName)
}

// checkServiceHealth checks the health of a service and returns its status
func (m *BackendServiceManager) checkServiceHealth(ctx context.Context, serviceURL, healthCheckPath string) (BackendServiceStatus, time.Duration, string, error) {
	startTime := time.Now()

	// Create the health check URL
	healthURL := fmt.Sprintf("%s%s", serviceURL, healthCheckPath)

	// Create a request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return ServiceStatusUnknown, 0, "", fmt.Errorf("failed to create health check request: %w", err)
	}

	// Send the request
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return ServiceStatusDown, 0, "", fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	// Calculate response time
	responseTime := time.Since(startTime)

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ServiceStatusDegraded, responseTime, "", fmt.Errorf("health check returned non-success status: %d", resp.StatusCode)
	}

	// Try to parse response body for version info
	version := ""
	body, err := io.ReadAll(resp.Body)
	if err == nil && len(body) > 0 {
		var healthData map[string]interface{}
		if err := json.Unmarshal(body, &healthData); err == nil {
			// Look for version in common fields
			for _, field := range []string{"version", "Version", "VERSION"} {
				if v, ok := healthData[field]; ok {
					if vStr, ok := v.(string); ok {
						version = vStr
						break
					}
				}
			}
		}
	}

	return ServiceStatusUp, responseTime, version, nil
}
