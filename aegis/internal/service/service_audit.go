package service

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/jasonKoogler/aegis/internal/domain"
	"github.com/jasonKoogler/aegis/internal/ports"
)

// Common event types for audit logging
const (
	// Authentication events
	EventLogin              = "user.login"
	EventLoginFailed        = "user.login.failed"
	EventLogout             = "user.logout"
	EventPasswordReset      = "user.password.reset"
	EventPasswordChanged    = "user.password.changed"
	EventMFAEnabled         = "user.mfa.enabled"
	EventMFADisabled        = "user.mfa.disabled"
	EventSessionRevoked     = "user.session.revoked"
	EventAllSessionsRevoked = "user.sessions.revoked_all"

	// User management events
	EventUserCreated  = "user.created"
	EventUserUpdated  = "user.updated"
	EventUserDeleted  = "user.deleted"
	EventUserLocked   = "user.locked"
	EventUserUnlocked = "user.unlocked"

	// Role and permission events
	EventRoleAssigned      = "role.assigned"
	EventRoleRevoked       = "role.revoked"
	EventRoleCreated       = "role.created"
	EventRoleUpdated       = "role.updated"
	EventRoleDeleted       = "role.deleted"
	EventPermissionCreated = "permission.created"
	EventPermissionUpdated = "permission.updated"
	EventPermissionDeleted = "permission.deleted"

	// API key events
	EventAPIKeyCreated = "apikey.created"
	EventAPIKeyUpdated = "apikey.updated"
	EventAPIKeyRevoked = "apikey.revoked"
	EventAPIKeyUsed    = "apikey.used"

	// OAuth events
	EventOAuthAuthorize = "oauth.authorize"
	EventOAuthToken     = "oauth.token"
	EventOAuthRevoke    = "oauth.revoke"

	// Tenant events
	EventTenantCreated = "tenant.created"
	EventTenantUpdated = "tenant.updated"
	EventTenantDeleted = "tenant.deleted"

	// System events
	EventSystemStartup  = "system.startup"
	EventSystemShutdown = "system.shutdown"
	EventConfigChanged  = "system.config.changed"
)

// Actor types for audit logging
const (
	ActorUser    = "user"
	ActorSystem  = "system"
	ActorAPIKey  = "apikey"
	ActorService = "service"
	ActorAdmin   = "admin"
)

// AuditService handles security event logging
type AuditService struct {
	auditRepo ports.AuditLogRepository
	logger    *log.Logger
}

// NewAuditService creates a new audit service
func NewAuditService(auditRepo ports.AuditLogRepository, logger *log.Logger) *AuditService {
	return &AuditService{
		auditRepo: auditRepo,
		logger:    logger,
	}
}

// LogEvent logs a security event
func (s *AuditService) LogEvent(ctx context.Context, eventType, actorType string, actorID uuid.UUID,
	tenantID *uuid.UUID, resourceType string, resourceID *uuid.UUID,
	ipAddress net.IP, userAgent string, eventData interface{}) error {

	// Create the audit log entry
	auditLog := &domain.AuditLog{
		EventType: eventType,
		ActorType: actorType,
		ActorID:   actorID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		EventData: eventData,
		CreatedAt: time.Now(),
	}

	// Set optional fields if provided
	if tenantID != nil {
		auditLog.TenantID = *tenantID
	}

	if resourceType != "" {
		auditLog.ResourceType = resourceType
	}

	if resourceID != nil {
		auditLog.ResourceID = *resourceID
	}

	// Save to repository
	_, err := s.auditRepo.Create(ctx, auditLog)
	if err != nil {
		s.logger.Error("Failed to create audit log entry",
			log.String("event_type", eventType),
			log.String("actor_type", actorType),
			log.String("actor_id", actorID.String()),
			log.Error(err))
		return fmt.Errorf("failed to create audit log entry: %w", err)
	}

	// Also log to application logs for visibility
	s.logger.Info("Security event",
		log.String("event_type", eventType),
		log.String("actor_type", actorType),
		log.String("actor_id", actorID.String()))

	return nil
}

// LogUserEvent is a convenience method for logging user events
func (s *AuditService) LogUserEvent(ctx context.Context, eventType string, userID uuid.UUID,
	tenantID *uuid.UUID, ipAddress net.IP, userAgent string, eventData interface{}) error {

	return s.LogEvent(ctx, eventType, ActorUser, userID, tenantID, "user", &userID, ipAddress, userAgent, eventData)
}

// LogAPIKeyEvent is a convenience method for logging API key events
func (s *AuditService) LogAPIKeyEvent(ctx context.Context, eventType string, apiKeyID uuid.UUID,
	userID uuid.UUID, tenantID *uuid.UUID, ipAddress net.IP, userAgent string, eventData interface{}) error {

	return s.LogEvent(ctx, eventType, ActorAPIKey, apiKeyID, tenantID, "apikey", &apiKeyID, ipAddress, userAgent, eventData)
}

// LogSystemEvent is a convenience method for logging system events
func (s *AuditService) LogSystemEvent(ctx context.Context, eventType string,
	tenantID *uuid.UUID, resourceType string, resourceID *uuid.UUID, eventData interface{}) error {

	systemID := uuid.Nil // System events use nil UUID as the actor ID
	return s.LogEvent(ctx, eventType, ActorSystem, systemID, tenantID, resourceType, resourceID, nil, "", eventData)
}

// LogAdminEvent is a convenience method for logging admin actions
func (s *AuditService) LogAdminEvent(ctx context.Context, eventType string, adminID uuid.UUID,
	tenantID *uuid.UUID, resourceType string, resourceID *uuid.UUID,
	ipAddress net.IP, userAgent string, eventData interface{}) error {

	return s.LogEvent(ctx, eventType, ActorAdmin, adminID, tenantID, resourceType, resourceID, ipAddress, userAgent, eventData)
}

// GetAuditLog retrieves an audit log entry by ID
func (s *AuditService) GetAuditLog(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error) {
	return s.auditRepo.GetByID(ctx, id)
}

// ListAuditLogsByTenant lists all audit log entries for a tenant
func (s *AuditService) ListAuditLogsByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*domain.AuditLog, error) {
	return s.auditRepo.ListByTenant(ctx, tenantID, page, pageSize)
}

// ListAuditLogsByUser lists all audit log entries for a user
func (s *AuditService) ListAuditLogsByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.AuditLog, error) {
	return s.auditRepo.ListByUser(ctx, userID, page, pageSize)
}

// ListAuditLogsByEventType lists all audit log entries by event type
func (s *AuditService) ListAuditLogsByEventType(ctx context.Context, eventType string, page, pageSize int) ([]*domain.AuditLog, error) {
	return s.auditRepo.ListByEventType(ctx, eventType, page, pageSize)
}

// ListAuditLogsByResource lists all audit log entries for a resource
func (s *AuditService) ListAuditLogsByResource(ctx context.Context, resourceType string, resourceID uuid.UUID, page, pageSize int) ([]*domain.AuditLog, error) {
	return s.auditRepo.ListByResource(ctx, resourceType, resourceID, page, pageSize)
}
