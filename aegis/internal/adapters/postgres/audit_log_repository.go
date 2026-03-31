package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/db"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
	"github.com/jasonKoogler/abraxis/aegis/internal/ports"
)

// AuditLogRepository implements the AuditLogRepository interface
type AuditLogRepository struct {
	db *db.PostgresPool
}

var _ ports.AuditLogRepository = &AuditLogRepository{}

// NewAuditLogRepository creates a new audit log repository
func NewAuditLogRepository(db *db.PostgresPool) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

// Create creates a new audit log entry
func (a *AuditLogRepository) Create(ctx context.Context, log *domain.AuditLog) (*domain.AuditLog, error) {
	query := `
		INSERT INTO audit_logs (
			id, event_type, actor_type, actor_id, tenant_id, resource_type, 
			resource_id, ip_address, user_agent, event_data, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		) RETURNING id
	`

	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}

	log.CreatedAt = time.Now()

	// Convert event data to JSON
	eventDataJSON, err := json.Marshal(log.EventData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data: %w", err)
	}

	_, err = a.db.Exec(ctx, query,
		log.ID, log.EventType, log.ActorType, log.ActorID, log.TenantID,
		log.ResourceType, log.ResourceID, log.IPAddress, log.UserAgent,
		eventDataJSON, log.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	return log, nil
}

// GetByID retrieves an audit log entry by ID
func (a *AuditLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error) {
	query := `
		SELECT 
			id, event_type, actor_type, actor_id, tenant_id, resource_type, 
			resource_id, ip_address, user_agent, event_data, created_at
		FROM audit_logs
		WHERE id = $1
	`

	var log domain.AuditLog
	var eventDataJSON []byte

	err := a.db.QueryRow(ctx, query, id).Scan(
		&log.ID, &log.EventType, &log.ActorType, &log.ActorID, &log.TenantID,
		&log.ResourceType, &log.ResourceID, &log.IPAddress, &log.UserAgent,
		&eventDataJSON, &log.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit log by ID %s: %w", id, err)
	}

	// Unmarshal event data
	if len(eventDataJSON) > 0 {
		var eventData map[string]interface{}
		if err := json.Unmarshal(eventDataJSON, &eventData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
		}
		log.EventData = eventData
	}

	return &log, nil
}

// ListByTenant lists all audit log entries for a tenant
func (a *AuditLogRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*domain.AuditLog, error) {
	query := `
		SELECT 
			id, event_type, actor_type, actor_id, tenant_id, resource_type, 
			resource_id, ip_address, user_agent, event_data, created_at
		FROM audit_logs
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return a.queryAuditLogs(ctx, query, tenantID, pageSize, page*pageSize)
}

// ListByUser lists all audit log entries for a user
func (a *AuditLogRepository) ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.AuditLog, error) {
	query := `
		SELECT 
			id, event_type, actor_type, actor_id, tenant_id, resource_type, 
			resource_id, ip_address, user_agent, event_data, created_at
		FROM audit_logs
		WHERE actor_type = 'user' AND actor_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return a.queryAuditLogs(ctx, query, userID, pageSize, page*pageSize)
}

// ListByEventType lists all audit log entries by event type
func (a *AuditLogRepository) ListByEventType(ctx context.Context, eventType string, page, pageSize int) ([]*domain.AuditLog, error) {
	query := `
		SELECT 
			id, event_type, actor_type, actor_id, tenant_id, resource_type, 
			resource_id, ip_address, user_agent, event_data, created_at
		FROM audit_logs
		WHERE event_type = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return a.queryAuditLogs(ctx, query, eventType, pageSize, page*pageSize)
}

// ListByResource lists all audit log entries for a resource
func (a *AuditLogRepository) ListByResource(ctx context.Context, resourceType string, resourceID uuid.UUID, page, pageSize int) ([]*domain.AuditLog, error) {
	query := `
		SELECT 
			id, event_type, actor_type, actor_id, tenant_id, resource_type, 
			resource_id, ip_address, user_agent, event_data, created_at
		FROM audit_logs
		WHERE resource_type = $1 AND resource_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := a.db.Query(ctx, query, resourceType, resourceID, pageSize, page*pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs for resource %s/%s: %w", resourceType, resourceID, err)
	}
	defer rows.Close()

	logs := []*domain.AuditLog{}
	for rows.Next() {
		var log domain.AuditLog
		var eventDataJSON []byte

		if err := rows.Scan(
			&log.ID, &log.EventType, &log.ActorType, &log.ActorID, &log.TenantID,
			&log.ResourceType, &log.ResourceID, &log.IPAddress, &log.UserAgent,
			&eventDataJSON, &log.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan audit log data: %w", err)
		}

		// Unmarshal event data
		if len(eventDataJSON) > 0 {
			var eventData map[string]interface{}
			if err := json.Unmarshal(eventDataJSON, &eventData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			log.EventData = eventData
		}

		logs = append(logs, &log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit log rows: %w", err)
	}

	return logs, nil
}

// queryAuditLogs is a helper function to query audit logs
func (a *AuditLogRepository) queryAuditLogs(ctx context.Context, query string, args ...interface{}) ([]*domain.AuditLog, error) {
	rows, err := a.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	logs := []*domain.AuditLog{}
	for rows.Next() {
		var log domain.AuditLog
		var eventDataJSON []byte

		if err := rows.Scan(
			&log.ID, &log.EventType, &log.ActorType, &log.ActorID, &log.TenantID,
			&log.ResourceType, &log.ResourceID, &log.IPAddress, &log.UserAgent,
			&eventDataJSON, &log.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan audit log data: %w", err)
		}

		// Unmarshal event data
		if len(eventDataJSON) > 0 {
			var eventData map[string]interface{}
			if err := json.Unmarshal(eventDataJSON, &eventData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
			}
			log.EventData = eventData
		}

		logs = append(logs, &log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit log rows: %w", err)
	}

	return logs, nil
}
