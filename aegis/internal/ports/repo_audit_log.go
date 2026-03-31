package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
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
}
