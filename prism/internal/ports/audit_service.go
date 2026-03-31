package ports

import (
	"context"
	"time"

	"github.com/jasonKoogler/abraxis/prism/internal/domain"
)

type AuditService interface {
	LogEvent(ctx context.Context, req *domain.AuditLogReq) error

	// LogUserEvent(ctx context.Context, eventType string, userID uuid.UUID,
	// 	tenantID *uuid.UUID, ipAddress net.IP, userAgent string, eventData interface{}) error

	// LogAPIKeyEvent(ctx context.Context, eventType string, apiKeyID uuid.UUID,
	// 	userID uuid.UUID, tenantID *uuid.UUID, ipAddress net.IP, userAgent string, eventData interface{}) error

	// LogSystemEvent(ctx context.Context, eventType string,
	// 	tenantID *uuid.UUID, resourceType string, resourceID *uuid.UUID, eventData interface{}) error

	// LogAdminEvent(ctx context.Context, eventType string, adminID uuid.UUID,
	// 	tenantID *uuid.UUID, resourceType string, resourceID *uuid.UUID,
	// 	ipAddress net.IP, userAgent string, eventData interface{}) error

	GetAuditLog(ctx context.Context, id string) (*domain.AuditLog, error)

	ListAuditLogs(ctx context.Context, req *domain.ListAuditLogsReq) (*domain.AuditLogListResponse, error)

	// AggregateAuditLogs aggregates audit logs by the specified group
	AggregateAuditLogs(ctx context.Context, groupBy string, startDate, endDate time.Time) (*domain.AuditLogAggregateResponse, error)

	// ExportAuditLogs exports audit logs as CSV data
	ExportAuditLogs(ctx context.Context, req *domain.ExportAuditLogsReq) ([]byte, error)

	// ListAuditLogsByTenant(ctx context.Context, tenantID string, page, pageSize int) ([]*domain.AuditLog, error)

	// ListAuditLogsByUser(ctx context.Context, userID string, page, pageSize int) ([]*domain.AuditLog, error)

	// ListAuditLogsByEventType(ctx context.Context, eventType string, page, pageSize int) ([]*domain.AuditLog, error)

	// ListAuditLogsByResource(ctx context.Context, resourceType string, resourceID uuid.UUID, page, pageSize int) ([]*domain.AuditLog, error)
}
