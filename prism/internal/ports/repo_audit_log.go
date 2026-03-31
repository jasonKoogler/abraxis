package ports

import (
	"context"
	"time"

	"github.com/jasonKoogler/prism/internal/common/uuid"
	"github.com/jasonKoogler/prism/internal/domain"
)

// AuditLogRepository defines the interface for audit log operations
type AuditLogRepository interface {
	// Create creates a new audit log entry
	Create(ctx context.Context, log *domain.AuditLog) (*domain.AuditLog, error)

	// GetByID retrieves an audit log entry by ID
	GetByID(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error)

	// ListByTenant lists all audit log entries for a tenant
	ListByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*domain.AuditLog, error)

	// ListByUser lists all audit log entries for a user
	ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.AuditLog, error)

	// ListByEventType lists all audit log entries by event type
	ListByEventType(ctx context.Context, eventType string, page, pageSize int) ([]*domain.AuditLog, error)

	// ListByResource lists all audit log entries for a resource
	ListByResource(ctx context.Context, resourceType string, resourceID uuid.UUID, page, pageSize int) ([]*domain.AuditLog, error)

	// ListByDateRange lists all audit log entries within a date range
	ListByDateRange(ctx context.Context, startDate, endDate time.Time, page, pageSize int) ([]*domain.AuditLog, error)

	// ListByFilters lists audit logs with combined filters
	ListByFilters(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, eventType, resourceType string,
		resourceID uuid.UUID, startDate, endDate time.Time, page, pageSize int) ([]*domain.AuditLog, error)

	// CountByTenant counts audit log entries for a tenant
	CountByTenant(ctx context.Context, tenantID uuid.UUID) (int, error)

	// CountByUser counts audit log entries for a user
	CountByUser(ctx context.Context, userID uuid.UUID) (int, error)

	// CountByEventType counts audit log entries by event type
	CountByEventType(ctx context.Context, eventType string) (int, error)

	// CountByResource counts audit log entries for a resource
	CountByResource(ctx context.Context, resourceType string, resourceID uuid.UUID) (int, error)

	// CountByFilters counts audit logs with combined filters
	CountByFilters(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, eventType, resourceType string,
		resourceID uuid.UUID, startDate, endDate time.Time) (int, error)

	// AggregateByEventType aggregates audit logs by event type
	AggregateByEventType(ctx context.Context, startDate, endDate time.Time) ([]*domain.AuditLogAggregate, error)

	// AggregateByActorType aggregates audit logs by actor type
	AggregateByActorType(ctx context.Context, startDate, endDate time.Time) ([]*domain.AuditLogAggregate, error)

	// AggregateByTenant aggregates audit logs by tenant
	AggregateByTenant(ctx context.Context, startDate, endDate time.Time) ([]*domain.AuditLogAggregate, error)

	// ExportToCSV exports audit logs to CSV format based on filters
	ExportToCSV(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, eventType, resourceType string,
		resourceID uuid.UUID, startDate, endDate time.Time) ([]byte, error)
}
