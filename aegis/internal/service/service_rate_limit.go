package service

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
	"github.com/jasonKoogler/abraxis/aegis/internal/ports"
)

// RateLimitType defines the type of rate limit
type RateLimitType string

const (
	// RateLimitTypeIP limits by IP address
	RateLimitTypeIP RateLimitType = "ip"
	// RateLimitTypeUser limits by user ID
	RateLimitTypeUser RateLimitType = "user"
	// RateLimitTypeAPIKey limits by API key
	RateLimitTypeAPIKey RateLimitType = "apikey"
	// RateLimitTypeTenant limits by tenant ID
	RateLimitTypeTenant RateLimitType = "tenant"
	// RateLimitTypeRoute limits by API route
	RateLimitTypeRoute RateLimitType = "route"
	// RateLimitTypeGlobal applies to all traffic
	RateLimitTypeGlobal RateLimitType = "global"
)

// RateLimitStrategy defines how rate limits are applied
type RateLimitStrategy string

const (
	// StrategyFixed applies a fixed rate limit
	StrategyFixed RateLimitStrategy = "fixed"
	// StrategyAdaptive adjusts rate limits based on load
	StrategyAdaptive RateLimitStrategy = "adaptive"
	// StrategyTokenBucket uses a token bucket algorithm
	StrategyTokenBucket RateLimitStrategy = "token_bucket"
	// StrategyLeakyBucket uses a leaky bucket algorithm
	StrategyLeakyBucket RateLimitStrategy = "leaky_bucket"
)

// RateLimitConfig represents a rate limit configuration
type RateLimitConfig struct {
	ID                uuid.UUID         `json:"id"`
	Name              string            `json:"name"`
	Type              RateLimitType     `json:"type"`
	Strategy          RateLimitStrategy `json:"strategy"`
	Target            string            `json:"target"` // IP, user ID, API key, tenant ID, or route pattern
	RequestsPerMinute int               `json:"requests_per_minute"`
	BurstSize         int               `json:"burst_size"`
	TenantID          *uuid.UUID        `json:"tenant_id,omitempty"` // Optional, only set for tenant-specific limits
	IsActive          bool              `json:"is_active"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	// Adaptive rate limiting parameters
	MinRequestsPerMinute int     `json:"min_requests_per_minute,omitempty"`
	MaxRequestsPerMinute int     `json:"max_requests_per_minute,omitempty"`
	CurrentLoad          float64 `json:"current_load,omitempty"` // 0.0-1.0 representing system load
}

// RateLimitViolation represents a rate limit violation event
type RateLimitViolation struct {
	ID           uuid.UUID     `json:"id"`
	ConfigID     uuid.UUID     `json:"config_id"`
	Type         RateLimitType `json:"type"`
	Target       string        `json:"target"`
	IPAddress    net.IP        `json:"ip_address,omitempty"`
	UserID       *uuid.UUID    `json:"user_id,omitempty"`
	TenantID     *uuid.UUID    `json:"tenant_id,omitempty"`
	APIKeyID     *uuid.UUID    `json:"api_key_id,omitempty"`
	Route        string        `json:"route,omitempty"`
	RequestCount int           `json:"request_count"`
	Threshold    int           `json:"threshold"`
	OccurredAt   time.Time     `json:"occurred_at"`
}

// RateLimitService handles rate limiting for the auth service
type RateLimitService struct {
	rateLimiter      ports.RateLimiter
	auditService     *AuditService
	logger           *log.Logger
	configs          map[string]*RateLimitConfig    // In-memory cache of rate limit configs
	configsByID      map[uuid.UUID]*RateLimitConfig // Configs indexed by ID
	configLock       sync.RWMutex
	violations       []*RateLimitViolation // Recent violations (circular buffer)
	violationIndex   int                   // Current index in violations buffer
	violationLock    sync.RWMutex
	maxViolations    int           // Maximum number of violations to store
	adaptiveUpdateCh chan struct{} // Channel for triggering adaptive updates
	stopCh           chan struct{} // Channel for stopping background goroutines
}

// NewRateLimitService creates a new rate limit service
func NewRateLimitService(
	rateLimiter ports.RateLimiter,
	auditService *AuditService,
	logger *log.Logger,
) *RateLimitService {
	service := &RateLimitService{
		rateLimiter:      rateLimiter,
		auditService:     auditService,
		logger:           logger,
		configs:          make(map[string]*RateLimitConfig),
		configsByID:      make(map[uuid.UUID]*RateLimitConfig),
		maxViolations:    1000,
		violations:       make([]*RateLimitViolation, 1000),
		adaptiveUpdateCh: make(chan struct{}, 1),
		stopCh:           make(chan struct{}),
	}

	// Start background goroutine for adaptive rate limiting
	go service.adaptiveRateLimitUpdater()

	return service
}

// Close stops all background goroutines
func (s *RateLimitService) Close() {
	close(s.stopCh)
}

// CreateRateLimitParams contains parameters for creating a rate limit
type CreateRateLimitParams struct {
	Name                 string
	Type                 RateLimitType
	Strategy             RateLimitStrategy
	Target               string
	RequestsPerMinute    int
	BurstSize            int
	TenantID             *uuid.UUID
	MinRequestsPerMinute int
	MaxRequestsPerMinute int
}

// CreateRateLimit creates a new rate limit configuration
func (s *RateLimitService) CreateRateLimit(ctx context.Context, params *CreateRateLimitParams, actorID uuid.UUID, actorType string) (*RateLimitConfig, error) {
	// Validate inputs
	if params.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if params.Type == "" {
		return nil, fmt.Errorf("type is required")
	}
	if params.Target == "" && params.Type != RateLimitTypeGlobal {
		return nil, fmt.Errorf("target is required for non-global rate limits")
	}
	if params.RequestsPerMinute <= 0 {
		return nil, fmt.Errorf("requests per minute must be positive")
	}
	if params.BurstSize <= 0 {
		return nil, fmt.Errorf("burst size must be positive")
	}

	// Set default strategy if not provided
	if params.Strategy == "" {
		params.Strategy = StrategyFixed
	}

	// Validate adaptive parameters if using adaptive strategy
	if params.Strategy == StrategyAdaptive {
		if params.MinRequestsPerMinute <= 0 {
			params.MinRequestsPerMinute = params.RequestsPerMinute / 2
		}
		if params.MaxRequestsPerMinute <= 0 {
			params.MaxRequestsPerMinute = params.RequestsPerMinute * 2
		}
		if params.MinRequestsPerMinute >= params.RequestsPerMinute {
			return nil, fmt.Errorf("min requests per minute must be less than requests per minute")
		}
		if params.MaxRequestsPerMinute <= params.RequestsPerMinute {
			return nil, fmt.Errorf("max requests per minute must be greater than requests per minute")
		}
	}

	// Create the rate limit config
	config := &RateLimitConfig{
		ID:                   uuid.New(),
		Name:                 params.Name,
		Type:                 params.Type,
		Strategy:             params.Strategy,
		Target:               params.Target,
		RequestsPerMinute:    params.RequestsPerMinute,
		BurstSize:            params.BurstSize,
		IsActive:             true,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
		MinRequestsPerMinute: params.MinRequestsPerMinute,
		MaxRequestsPerMinute: params.MaxRequestsPerMinute,
		CurrentLoad:          0.5, // Default to middle load
	}

	// Set tenant ID if provided
	if params.TenantID != nil {
		config.TenantID = params.TenantID
	}

	// Add to in-memory cache
	s.configLock.Lock()
	cacheKey := s.makeCacheKey(config.Type, config.Target)
	s.configs[cacheKey] = config
	s.configsByID[config.ID] = config
	s.configLock.Unlock()

	// Log the event
	eventData := map[string]interface{}{
		"name":                params.Name,
		"type":                string(params.Type),
		"strategy":            string(params.Strategy),
		"target":              params.Target,
		"requests_per_minute": params.RequestsPerMinute,
		"burst_size":          params.BurstSize,
	}

	if params.Strategy == StrategyAdaptive {
		eventData["min_requests_per_minute"] = params.MinRequestsPerMinute
		eventData["max_requests_per_minute"] = params.MaxRequestsPerMinute
	}

	var tenantID *uuid.UUID
	if params.TenantID != nil {
		tenantID = params.TenantID
		eventData["tenant_id"] = params.TenantID.String()
	}

	s.auditService.LogEvent(
		ctx,
		"ratelimit.created",
		actorType,
		actorID,
		tenantID,
		"ratelimit",
		&config.ID,
		nil, // IP address not available here
		"",  // User agent not available here
		eventData,
	)

	s.logger.Info("Rate limit created",
		log.String("name", params.Name),
		log.String("type", string(params.Type)),
		log.String("strategy", string(params.Strategy)),
		log.String("target", params.Target),
		log.String("requests_per_minute", strconv.Itoa(params.RequestsPerMinute)))

	return config, nil
}

// GetRateLimit gets a rate limit configuration by ID
func (s *RateLimitService) GetRateLimit(ctx context.Context, id uuid.UUID) (*RateLimitConfig, error) {
	s.configLock.RLock()
	defer s.configLock.RUnlock()

	config, exists := s.configsByID[id]
	if !exists {
		return nil, fmt.Errorf("rate limit not found")
	}

	return config, nil
}

// UpdateRateLimitParams contains parameters for updating a rate limit
type UpdateRateLimitParams struct {
	Name                 *string
	Strategy             *RateLimitStrategy
	RequestsPerMinute    *int
	BurstSize            *int
	IsActive             *bool
	MinRequestsPerMinute *int
	MaxRequestsPerMinute *int
}

// UpdateRateLimit updates a rate limit configuration
func (s *RateLimitService) UpdateRateLimit(ctx context.Context, id uuid.UUID, params *UpdateRateLimitParams, actorID uuid.UUID, actorType string) (*RateLimitConfig, error) {
	// Get the existing config
	s.configLock.Lock()
	defer s.configLock.Unlock()

	config, exists := s.configsByID[id]
	if !exists {
		return nil, fmt.Errorf("rate limit not found")
	}

	// Update fields if provided
	if params.Name != nil {
		config.Name = *params.Name
	}
	if params.Strategy != nil {
		config.Strategy = *params.Strategy
	}
	if params.RequestsPerMinute != nil {
		if *params.RequestsPerMinute <= 0 {
			return nil, fmt.Errorf("requests per minute must be positive")
		}
		config.RequestsPerMinute = *params.RequestsPerMinute
	}
	if params.BurstSize != nil {
		if *params.BurstSize <= 0 {
			return nil, fmt.Errorf("burst size must be positive")
		}
		config.BurstSize = *params.BurstSize
	}
	if params.IsActive != nil {
		config.IsActive = *params.IsActive
	}

	// Update adaptive parameters if provided
	if config.Strategy == StrategyAdaptive {
		if params.MinRequestsPerMinute != nil {
			if *params.MinRequestsPerMinute <= 0 {
				return nil, fmt.Errorf("min requests per minute must be positive")
			}
			config.MinRequestsPerMinute = *params.MinRequestsPerMinute
		}
		if params.MaxRequestsPerMinute != nil {
			if *params.MaxRequestsPerMinute <= 0 {
				return nil, fmt.Errorf("max requests per minute must be positive")
			}
			config.MaxRequestsPerMinute = *params.MaxRequestsPerMinute
		}

		// Validate min/max relationship
		if config.MinRequestsPerMinute >= config.RequestsPerMinute {
			return nil, fmt.Errorf("min requests per minute must be less than requests per minute")
		}
		if config.MaxRequestsPerMinute <= config.RequestsPerMinute {
			return nil, fmt.Errorf("max requests per minute must be greater than requests per minute")
		}
	}

	config.UpdatedAt = time.Now()

	// Update in-memory cache
	cacheKey := s.makeCacheKey(config.Type, config.Target)
	s.configs[cacheKey] = config
	s.configsByID[config.ID] = config

	// Trigger adaptive update if needed
	if config.Strategy == StrategyAdaptive {
		select {
		case s.adaptiveUpdateCh <- struct{}{}:
			// Signal sent
		default:
			// Channel full, update already pending
		}
	}

	// Log the event
	eventData := map[string]interface{}{
		"name":                config.Name,
		"type":                string(config.Type),
		"strategy":            string(config.Strategy),
		"target":              config.Target,
		"requests_per_minute": config.RequestsPerMinute,
		"burst_size":          config.BurstSize,
		"is_active":           config.IsActive,
	}

	if config.Strategy == StrategyAdaptive {
		eventData["min_requests_per_minute"] = config.MinRequestsPerMinute
		eventData["max_requests_per_minute"] = config.MaxRequestsPerMinute
		eventData["current_load"] = config.CurrentLoad
	}

	var tenantID *uuid.UUID
	if config.TenantID != nil {
		tenantID = config.TenantID
		eventData["tenant_id"] = config.TenantID.String()
	}

	s.auditService.LogEvent(
		ctx,
		"ratelimit.updated",
		actorType,
		actorID,
		tenantID,
		"ratelimit",
		&config.ID,
		nil, // IP address not available here
		"",  // User agent not available here
		eventData,
	)

	s.logger.Info("Rate limit updated",
		log.String("id", id.String()),
		log.String("name", config.Name),
		log.String("requests_per_minute", strconv.Itoa(config.RequestsPerMinute)))

	return config, nil
}

// DeleteRateLimit deletes a rate limit configuration
func (s *RateLimitService) DeleteRateLimit(ctx context.Context, id uuid.UUID, actorID uuid.UUID, actorType string) error {
	// Get the existing config for logging
	s.configLock.Lock()
	defer s.configLock.Unlock()

	config, exists := s.configsByID[id]
	if !exists {
		return fmt.Errorf("rate limit not found")
	}

	// Remove from in-memory cache
	cacheKey := s.makeCacheKey(config.Type, config.Target)
	delete(s.configs, cacheKey)
	delete(s.configsByID, id)

	// Log the event
	eventData := map[string]interface{}{
		"name":   config.Name,
		"type":   string(config.Type),
		"target": config.Target,
	}

	var tenantID *uuid.UUID
	if config.TenantID != nil {
		tenantID = config.TenantID
		eventData["tenant_id"] = config.TenantID.String()
	}

	s.auditService.LogEvent(
		ctx,
		"ratelimit.deleted",
		actorType,
		actorID,
		tenantID,
		"ratelimit",
		&config.ID,
		nil, // IP address not available here
		"",  // User agent not available here
		eventData,
	)

	s.logger.Info("Rate limit deleted",
		log.String("id", id.String()),
		log.String("name", config.Name),
		log.String("type", string(config.Type)),
		log.String("target", config.Target))

	return nil
}

// ListRateLimits lists all rate limit configurations
func (s *RateLimitService) ListRateLimits(ctx context.Context) ([]*RateLimitConfig, error) {
	s.configLock.RLock()
	defer s.configLock.RUnlock()

	configs := make([]*RateLimitConfig, 0, len(s.configs))
	for _, config := range s.configs {
		configs = append(configs, config)
	}

	return configs, nil
}

// ListRateLimitsByTenant lists all rate limit configurations for a tenant
func (s *RateLimitService) ListRateLimitsByTenant(ctx context.Context, tenantID uuid.UUID) ([]*RateLimitConfig, error) {
	s.configLock.RLock()
	defer s.configLock.RUnlock()

	configs := make([]*RateLimitConfig, 0)
	for _, config := range s.configs {
		if config.TenantID != nil && *config.TenantID == tenantID {
			configs = append(configs, config)
		}
	}

	return configs, nil
}

// ListRateLimitsByType lists all rate limit configurations of a specific type
func (s *RateLimitService) ListRateLimitsByType(ctx context.Context, limitType RateLimitType) ([]*RateLimitConfig, error) {
	s.configLock.RLock()
	defer s.configLock.RUnlock()

	configs := make([]*RateLimitConfig, 0)
	for _, config := range s.configs {
		if config.Type == limitType {
			configs = append(configs, config)
		}
	}

	return configs, nil
}

// CheckRateLimitParams contains parameters for checking a rate limit
type CheckRateLimitParams struct {
	Type      RateLimitType
	Target    string
	TenantID  *uuid.UUID
	UserID    *uuid.UUID
	IPAddress net.IP
	Route     string
}

// CheckRateLimit checks if a request is allowed based on rate limits
func (s *RateLimitService) CheckRateLimit(ctx context.Context, params *CheckRateLimitParams) (bool, ports.RateLimitInfo, error) {
	// Check specific limit first
	if params.Type != "" && params.Target != "" {
		allowed, info, err := s.checkSpecificRateLimit(ctx, params.Type, params.Target)
		if err == nil {
			return allowed, info, nil
		}
		// Continue to other checks if specific limit not found
	}

	// Check tenant-wide limit if tenant ID provided
	if params.TenantID != nil {
		allowed, info, err := s.checkSpecificRateLimit(ctx, RateLimitTypeTenant, params.TenantID.String())
		if err == nil {
			return allowed, info, nil
		}
	}

	// Check user limit if user ID provided
	if params.UserID != nil {
		allowed, info, err := s.checkSpecificRateLimit(ctx, RateLimitTypeUser, params.UserID.String())
		if err == nil {
			return allowed, info, nil
		}
	}

	// Check IP limit if IP address provided
	if params.IPAddress != nil {
		allowed, info, err := s.checkSpecificRateLimit(ctx, RateLimitTypeIP, params.IPAddress.String())
		if err == nil {
			return allowed, info, nil
		}
	}

	// Check route limit if route provided
	if params.Route != "" {
		allowed, info, err := s.checkSpecificRateLimit(ctx, RateLimitTypeRoute, params.Route)
		if err == nil {
			return allowed, info, nil
		}
	}

	// Finally, check global limit
	allowed, info, err := s.checkSpecificRateLimit(ctx, RateLimitTypeGlobal, "")
	if err == nil {
		return allowed, info, nil
	}

	// No applicable rate limits found, allow the request
	return true, ports.RateLimitInfo{}, nil
}

// checkSpecificRateLimit checks a specific rate limit
func (s *RateLimitService) checkSpecificRateLimit(ctx context.Context, limitType RateLimitType, target string) (bool, ports.RateLimitInfo, error) {
	s.configLock.RLock()
	cacheKey := s.makeCacheKey(limitType, target)
	config, exists := s.configs[cacheKey]
	s.configLock.RUnlock()

	if !exists {
		return false, ports.RateLimitInfo{}, fmt.Errorf("rate limit not found")
	}

	// Check if the config is active
	if !config.IsActive {
		return true, ports.RateLimitInfo{}, nil
	}

	// Check the rate limit
	allowed, info := s.rateLimiter.Allow(cacheKey)

	// Record violation if not allowed
	if !allowed {
		s.recordViolation(config, target, info.Limit)
	}

	return allowed, info, nil
}

// recordViolation records a rate limit violation
func (s *RateLimitService) recordViolation(config *RateLimitConfig, target string, threshold int) {
	violation := &RateLimitViolation{
		ID:           uuid.New(),
		ConfigID:     config.ID,
		Type:         config.Type,
		Target:       target,
		RequestCount: threshold + 1, // Exceeded by at least 1
		Threshold:    threshold,
		OccurredAt:   time.Now(),
	}

	// Set specific fields based on type
	switch config.Type {
	case RateLimitTypeIP:
		violation.IPAddress = net.ParseIP(target)
	case RateLimitTypeUser:
		userID, err := uuid.Parse(target)
		if err == nil {
			violation.UserID = &userID
		}
	case RateLimitTypeTenant:
		tenantID, err := uuid.Parse(target)
		if err == nil {
			violation.TenantID = &tenantID
		}
	case RateLimitTypeAPIKey:
		apiKeyID, err := uuid.Parse(target)
		if err == nil {
			violation.APIKeyID = &apiKeyID
		}
	case RateLimitTypeRoute:
		violation.Route = target
	}

	// Store violation in circular buffer
	s.violationLock.Lock()
	s.violations[s.violationIndex] = violation
	s.violationIndex = (s.violationIndex + 1) % s.maxViolations
	s.violationLock.Unlock()

	// Log violation
	s.logger.Warn("Rate limit exceeded",
		log.String("type", string(config.Type)),
		log.String("target", target),
		log.String("config_name", config.Name),
		log.String("threshold", strconv.Itoa(threshold)))

	// Update load for adaptive rate limiting
	if config.Strategy == StrategyAdaptive {
		s.updateAdaptiveLoad(config.ID, 0.1) // Increase load by 10%
	}
}

// GetRecentViolations gets recent rate limit violations
func (s *RateLimitService) GetRecentViolations(ctx context.Context, limit int) []*RateLimitViolation {
	if limit <= 0 || limit > s.maxViolations {
		limit = s.maxViolations
	}

	s.violationLock.RLock()
	defer s.violationLock.RUnlock()

	// Collect non-nil violations, starting from the most recent
	result := make([]*RateLimitViolation, 0, limit)
	count := 0
	idx := (s.violationIndex - 1 + s.maxViolations) % s.maxViolations

	for count < limit {
		if s.violations[idx] != nil {
			result = append(result, s.violations[idx])
			count++
		}
		idx = (idx - 1 + s.maxViolations) % s.maxViolations
		if idx == ((s.violationIndex - 1 + s.maxViolations) % s.maxViolations) {
			// We've gone all the way around
			break
		}
	}

	return result
}

// UpdateSystemLoad updates the system load for adaptive rate limiting
func (s *RateLimitService) UpdateSystemLoad(ctx context.Context, load float64) error {
	if load < 0.0 || load > 1.0 {
		return fmt.Errorf("load must be between 0.0 and 1.0")
	}

	// Update all adaptive rate limits
	s.configLock.Lock()
	for _, config := range s.configsByID {
		if config.Strategy == StrategyAdaptive {
			config.CurrentLoad = load
		}
	}
	s.configLock.Unlock()

	// Trigger adaptive update
	select {
	case s.adaptiveUpdateCh <- struct{}{}:
		// Signal sent
	default:
		// Channel full, update already pending
	}

	return nil
}

// updateAdaptiveLoad updates the load for a specific rate limit
func (s *RateLimitService) updateAdaptiveLoad(configID uuid.UUID, delta float64) {
	s.configLock.Lock()
	defer s.configLock.Unlock()

	config, exists := s.configsByID[configID]
	if !exists || config.Strategy != StrategyAdaptive {
		return
	}

	// Update load, keeping it between 0.0 and 1.0
	config.CurrentLoad += delta
	if config.CurrentLoad < 0.0 {
		config.CurrentLoad = 0.0
	} else if config.CurrentLoad > 1.0 {
		config.CurrentLoad = 1.0
	}

	// Trigger adaptive update
	select {
	case s.adaptiveUpdateCh <- struct{}{}:
		// Signal sent
	default:
		// Channel full, update already pending
	}
}

// adaptiveRateLimitUpdater runs in the background to update adaptive rate limits
func (s *RateLimitService) adaptiveRateLimitUpdater() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.adaptiveUpdateCh:
			s.updateAdaptiveRateLimits()
		case <-ticker.C:
			s.updateAdaptiveRateLimits()
		case <-s.stopCh:
			return
		}
	}
}

// updateAdaptiveRateLimits updates all adaptive rate limits based on current load
func (s *RateLimitService) updateAdaptiveRateLimits() {
	s.configLock.Lock()
	defer s.configLock.Unlock()

	for _, config := range s.configsByID {
		if config.Strategy != StrategyAdaptive {
			continue
		}

		// Calculate new rate based on load
		// At load 0.0, use max rate; at load 1.0, use min rate
		loadFactor := 1.0 - config.CurrentLoad
		newRate := int(float64(config.MinRequestsPerMinute) +
			loadFactor*float64(config.MaxRequestsPerMinute-config.MinRequestsPerMinute))

		// Update the rate limit
		if newRate != config.RequestsPerMinute {
			oldRate := config.RequestsPerMinute
			config.RequestsPerMinute = newRate
			config.UpdatedAt = time.Now()

			s.logger.Info("Adaptive rate limit updated",
				log.String("id", config.ID.String()),
				log.String("name", config.Name),
				log.String("old_rate", strconv.Itoa(oldRate)),
				log.String("new_rate", strconv.Itoa(newRate)),
				log.String("load", fmt.Sprintf("%.2f", config.CurrentLoad)))
		}
	}
}

// Helper methods

// makeCacheKey creates a cache key for a rate limit config
func (s *RateLimitService) makeCacheKey(limitType RateLimitType, target string) string {
	return fmt.Sprintf("%s:%s", limitType, target)
}
