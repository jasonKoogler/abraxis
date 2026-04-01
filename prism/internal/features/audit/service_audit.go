package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/jasonKoogler/abraxis/prism/internal/common/api"
	"github.com/jasonKoogler/abraxis/prism/internal/common/log"
	"github.com/jasonKoogler/abraxis/prism/internal/common/uuid"
	"github.com/jasonKoogler/abraxis/prism/internal/domain"
	"github.com/jasonKoogler/abraxis/prism/internal/domain/prefixid"
	"github.com/jasonKoogler/abraxis/prism/internal/ports"
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

var _ ports.AuditService = &AuditService{}

// NewAuditService creates a new audit service
func NewAuditService(auditRepo ports.AuditLogRepository, logger *log.Logger) *AuditService {
	return &AuditService{
		auditRepo: auditRepo,
		logger:    logger,
	}
}

// LogEvent logs a security event
func (s *AuditService) LogEvent(ctx context.Context, req *domain.AuditLogReq) error {

	auditLog, err := domain.NewAuditLog(req)
	if err != nil {
		return err
	}

	// Save to repository
	_, err = s.auditRepo.Create(ctx, auditLog)
	if err != nil {
		s.logger.Error("Failed to create audit log entry",
			log.String("event_type", auditLog.EventType),
			log.String("actor_type", auditLog.ActorType),
			log.String("actor_id", auditLog.ActorID),
			log.Error(err))
		return fmt.Errorf("failed to create audit log entry: %w", err)
	}

	// Also log to application logs for visibility
	s.logger.Info("Security event",
		log.String("event_type", auditLog.EventType),
		log.String("actor_type", auditLog.ActorType),
		log.String("actor_id", auditLog.ActorID))

	return nil
}

// // LogUserEvent is a convenience method for logging user events
// func (s *AuditService) LogUserEvent(ctx context.Context, eventType string, userID uuid.UUID,
// 	tenantID *uuid.UUID, ipAddress net.IP, userAgent string, eventData interface{}) error {

// 	return s.LogEvent(ctx, eventType, ActorUser, userID, tenantID, "user", &userID, ipAddress, userAgent, eventData)
// }

// // LogAPIKeyEvent is a convenience method for logging API key events
// func (s *AuditService) LogAPIKeyEvent(ctx context.Context, eventType string, apiKeyID uuid.UUID,
// 	userID uuid.UUID, tenantID *uuid.UUID, ipAddress net.IP, userAgent string, eventData interface{}) error {

// 	return s.LogEvent(ctx, eventType, ActorAPIKey, apiKeyID, tenantID, "apikey", &apiKeyID, ipAddress, userAgent, eventData)
// }

// // LogSystemEvent is a convenience method for logging system events
// func (s *AuditService) LogSystemEvent(ctx context.Context, eventType string,
// 	tenantID *uuid.UUID, resourceType string, resourceID *uuid.UUID, eventData interface{}) error {

// 	systemID := uuid.Nil // System events use nil UUID as the actor ID
// 	return s.LogEvent(ctx, eventType, ActorSystem, systemID, tenantID, resourceType, resourceID, nil, "", eventData)
// }

// // LogAdminEvent is a convenience method for logging admin actions
// func (s *AuditService) LogAdminEvent(ctx context.Context, eventType string, adminID uuid.UUID,
// 	tenantID *uuid.UUID, resourceType string, resourceID *uuid.UUID,
// 	ipAddress net.IP, userAgent string, eventData interface{}) error {

// 	return s.LogEvent(ctx, eventType, ActorAdmin, adminID, tenantID, resourceType, resourceID, ipAddress, userAgent, eventData)
// }

// GetAuditLog retrieves an audit log entry by ID
func (s *AuditService) GetAuditLog(ctx context.Context, id string) (*domain.AuditLog, error) {
	pid, err := prefixid.ParseAuditLogID(id)
	if err != nil {
		return nil, err
	}
	return s.auditRepo.GetByID(ctx, pid.Raw())
}

// ListAuditLogs retrieves a list of audit logs based on filters with pagination
func (s *AuditService) ListAuditLogs(ctx context.Context, req *domain.ListAuditLogsReq) (*domain.AuditLogListResponse, error) {
	params, err := req.Parse()
	if err != nil {
		return nil, err
	}

	// Parse pagination parameters. ParsePagination returns 1-based page numbers
	// but all repository methods expect 0-based page indices for OFFSET calculation.
	page, pageSize := api.ParsePagination(&params.Page, &params.PageSize)
	repoPage := page - 1

	var logs []*domain.AuditLog
	var totalCount int

	// Use the most specific filter available
	if params.TenantID != nil && params.UserID != nil && params.ResourceType != "" && params.ResourceID != nil {
		// Most specific case - all filters
		logs, err = s.auditRepo.ListByFilters(
			ctx,
			params.TenantID.Raw(),
			params.UserID.Raw(),
			params.EventType,
			params.ResourceType,
			params.ResourceID.Raw(),
			params.StartDate,
			params.EndDate,
			repoPage,
			pageSize,
		)
		if err != nil {
			return nil, err
		}

		totalCount, err = s.auditRepo.CountByFilters(
			ctx,
			params.TenantID.Raw(),
			params.UserID.Raw(),
			params.EventType,
			params.ResourceType,
			params.ResourceID.Raw(),
			params.StartDate,
			params.EndDate,
		)
		if err != nil {
			return nil, err
		}
	} else if !params.StartDate.IsZero() || !params.EndDate.Equal(time.Time{}) {
		// Date range filter
		logs, err = s.auditRepo.ListByDateRange(ctx, params.StartDate, params.EndDate, repoPage, pageSize)
		if err != nil {
			return nil, err
		}
		// For simplicity, we're not implementing a count for this case
		// In a real system, you'd implement this with a separate count query
		totalCount = len(logs)
	} else if params.TenantID != nil {
		// Filter by tenant
		logs, err = s.auditRepo.ListByTenant(ctx, params.TenantID.Raw(), repoPage, pageSize)
		if err != nil {
			return nil, err
		}
		totalCount, err = s.auditRepo.CountByTenant(ctx, params.TenantID.Raw())
		if err != nil {
			return nil, err
		}
	} else if params.UserID != nil {
		// Filter by user
		logs, err = s.auditRepo.ListByUser(ctx, params.UserID.Raw(), repoPage, pageSize)
		if err != nil {
			return nil, err
		}
		totalCount, err = s.auditRepo.CountByUser(ctx, params.UserID.Raw())
		if err != nil {
			return nil, err
		}
	} else if params.EventType != "" {
		// Filter by event type
		logs, err = s.auditRepo.ListByEventType(ctx, params.EventType, repoPage, pageSize)
		if err != nil {
			return nil, err
		}
		totalCount, err = s.auditRepo.CountByEventType(ctx, params.EventType)
		if err != nil {
			return nil, err
		}
	} else if params.ResourceType != "" && params.ResourceID != nil {
		// Filter by resource
		logs, err = s.auditRepo.ListByResource(
			ctx,
			params.ResourceType,
			params.ResourceID.Raw(),
			repoPage,
			pageSize,
		)
		if err != nil {
			return nil, err
		}
		totalCount, err = s.auditRepo.CountByResource(
			ctx,
			params.ResourceType,
			params.ResourceID.Raw(),
		)
		if err != nil {
			return nil, err
		}
	} else {
		// No filters - return error as this could return too many logs
		return nil, domain.ErrInvalidRequest
	}

	// Calculate pagination metadata
	totalPages := (totalCount + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	return &domain.AuditLogListResponse{
		Data: logs,
		Pagination: domain.PaginationMetadata{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: totalCount,
			TotalPages: totalPages,
		},
	}, nil
}

// AggregateAuditLogs aggregates audit logs by the specified group
func (s *AuditService) AggregateAuditLogs(ctx context.Context, groupBy string, startDate, endDate time.Time) (*domain.AuditLogAggregateResponse, error) {
	var aggregates []*domain.AuditLogAggregate
	var err error

	// Set default time range if not specified
	if startDate.IsZero() {
		startDate = time.Now().AddDate(0, -1, 0) // Default to 1 month ago
	}

	if endDate.IsZero() {
		endDate = time.Now()
	}

	if startDate.After(endDate) {
		return nil, fmt.Errorf("start_date cannot be after end_date")
	}

	startDate, err = time.Parse(time.RFC3339, startDate.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to parse start_date: %w", err)
	}

	endDate, err = time.Parse(time.RFC3339, endDate.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to parse end_date: %w", err)
	}
	// Select aggregation function based on groupBy parameter
	switch groupBy {
	case "event_type":
		aggregates, err = s.auditRepo.AggregateByEventType(ctx, startDate, endDate)
	case "actor_type":
		aggregates, err = s.auditRepo.AggregateByActorType(ctx, startDate, endDate)
	case "tenant":
		aggregates, err = s.auditRepo.AggregateByTenant(ctx, startDate, endDate)
	default:
		return nil, fmt.Errorf("invalid group_by parameter: %s", groupBy)
	}

	if err != nil {
		return nil, err
	}

	// Calculate total count
	totalCount := 0
	for _, agg := range aggregates {
		totalCount += agg.Count
	}

	return &domain.AuditLogAggregateResponse{
		Data:       aggregates,
		GroupBy:    groupBy,
		StartDate:  startDate,
		EndDate:    endDate,
		TotalCount: totalCount,
	}, nil
}

// ExportAuditLogs exports audit logs as CSV data
func (s *AuditService) ExportAuditLogs(ctx context.Context, req *domain.ExportAuditLogsReq) ([]byte, error) {
	params, err := req.Parse()
	if err != nil {
		return nil, err
	}

	// Convert ID values to UUID
	var tenantID, userID, resourceID uuid.UUID
	var emptyUUID uuid.UUID

	if params.TenantID != nil {
		tenantID = params.TenantID.Raw()
	} else {
		tenantID = emptyUUID
	}

	if params.UserID != nil {
		userID = params.UserID.Raw()
	} else {
		userID = emptyUUID
	}

	if params.ResourceID != nil {
		resourceID = params.ResourceID.Raw()
	} else {
		resourceID = emptyUUID
	}

	// Export to CSV
	return s.auditRepo.ExportToCSV(
		ctx,
		tenantID,
		userID,
		params.EventType,
		params.ResourceType,
		resourceID,
		params.StartDate,
		params.EndDate,
	)
}

// ListAuditLogsByTenant lists all audit log entries for a tenant
func (s *AuditService) listAuditLogsByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*domain.AuditLog, error) {
	return s.auditRepo.ListByTenant(ctx, tenantID, page, pageSize)
}

// ListAuditLogsByUser lists all audit log entries for a user
func (s *AuditService) listAuditLogsByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.AuditLog, error) {
	return s.auditRepo.ListByUser(ctx, userID, page, pageSize)
}

// ListAuditLogsByEventType lists all audit log entries by event type
func (s *AuditService) listAuditLogsByEventType(ctx context.Context, eventType string, page, pageSize int) ([]*domain.AuditLog, error) {
	return s.auditRepo.ListByEventType(ctx, eventType, page, pageSize)
}

// ListAuditLogsByResource lists all audit log entries for a resource
func (s *AuditService) listAuditLogsByResource(ctx context.Context, resourceType string, resourceID uuid.UUID, page, pageSize int) ([]*domain.AuditLog, error) {
	return s.auditRepo.ListByResource(ctx, resourceType, resourceID, page, pageSize)
}
