package postgres

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jasonKoogler/prism/internal/common/db"
	"github.com/jasonKoogler/prism/internal/common/uuid"
	"github.com/jasonKoogler/prism/internal/domain"
	"github.com/jasonKoogler/prism/internal/ports"
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

	return a.queryAuditLogs(ctx, query, pageSize, tenantID, page*pageSize)
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

	return a.queryAuditLogs(ctx, query, pageSize, userID, page*pageSize)
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

	return a.queryAuditLogs(ctx, query, pageSize, eventType, page*pageSize)
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
	return a.queryAuditLogs(ctx, query, pageSize, resourceType, resourceID, page*pageSize)
}

// ListByDateRange lists all audit log entries within a date range
func (a *AuditLogRepository) ListByDateRange(ctx context.Context, startDate, endDate time.Time, page, pageSize int) ([]*domain.AuditLog, error) {
	query := `
		SELECT 
			id, event_type, actor_type, actor_id, tenant_id, resource_type, 
			resource_id, ip_address, user_agent, event_data, created_at
		FROM audit_logs
		WHERE created_at >= $1 AND created_at <= $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	return a.queryAuditLogs(ctx, query, pageSize, startDate, endDate, page*pageSize)
}

// ListByFilters lists audit logs with combined filters
func (a *AuditLogRepository) ListByFilters(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID,
	eventType, resourceType string, resourceID uuid.UUID, startDate, endDate time.Time, page, pageSize int) ([]*domain.AuditLog, error) {

	var emptyUUID uuid.UUID // Zero value for UUID comparison

	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT 
			id, event_type, actor_type, actor_id, tenant_id, resource_type, 
			resource_id, ip_address, user_agent, event_data, created_at
		FROM audit_logs
		WHERE 1=1
	`)

	args := []interface{}{}
	argCounter := 1

	// Add filter conditions
	if tenantID != emptyUUID {
		queryBuilder.WriteString(fmt.Sprintf(" AND tenant_id = $%d", argCounter))
		args = append(args, tenantID)
		argCounter++
	}

	if userID != emptyUUID {
		queryBuilder.WriteString(fmt.Sprintf(" AND actor_type = 'user' AND actor_id = $%d", argCounter))
		args = append(args, userID)
		argCounter++
	}

	if eventType != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND event_type = $%d", argCounter))
		args = append(args, eventType)
		argCounter++
	}

	if resourceType != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND resource_type = $%d", argCounter))
		args = append(args, resourceType)
		argCounter++
	}

	if resourceID != emptyUUID {
		queryBuilder.WriteString(fmt.Sprintf(" AND resource_id = $%d", argCounter))
		args = append(args, resourceID)
		argCounter++
	}

	if !startDate.IsZero() {
		queryBuilder.WriteString(fmt.Sprintf(" AND created_at >= $%d", argCounter))
		args = append(args, startDate)
		argCounter++
	}

	if !endDate.IsZero() {
		queryBuilder.WriteString(fmt.Sprintf(" AND created_at <= $%d", argCounter))
		args = append(args, endDate)
		argCounter++
	}

	queryBuilder.WriteString(" ORDER BY created_at DESC")
	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCounter, argCounter+1))
	args = append(args, pageSize, page*pageSize)

	return a.queryAuditLogs(ctx, queryBuilder.String(), pageSize, args...)
}

// CountByTenant counts audit log entries for a tenant
func (a *AuditLogRepository) CountByTenant(ctx context.Context, tenantID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE tenant_id = $1
	`
	var count int
	err := a.db.QueryRow(ctx, query, tenantID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit logs for tenant %s: %w", tenantID, err)
	}
	return count, nil
}

// CountByUser counts audit log entries for a user
func (a *AuditLogRepository) CountByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE actor_type = 'user' AND actor_id = $1
	`
	var count int
	err := a.db.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit logs for user %s: %w", userID, err)
	}
	return count, nil
}

// CountByEventType counts audit log entries by event type
func (a *AuditLogRepository) CountByEventType(ctx context.Context, eventType string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE event_type = $1
	`
	var count int
	err := a.db.QueryRow(ctx, query, eventType).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit logs for event type %s: %w", eventType, err)
	}
	return count, nil
}

// CountByResource counts audit log entries for a resource
func (a *AuditLogRepository) CountByResource(ctx context.Context, resourceType string, resourceID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE resource_type = $1 AND resource_id = $2
	`
	var count int
	err := a.db.QueryRow(ctx, query, resourceType, resourceID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit logs for resource %s/%s: %w", resourceType, resourceID, err)
	}
	return count, nil
}

// CountByFilters counts audit logs with combined filters
func (a *AuditLogRepository) CountByFilters(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID,
	eventType, resourceType string, resourceID uuid.UUID, startDate, endDate time.Time) (int, error) {

	var emptyUUID uuid.UUID // Zero value for UUID comparison

	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT COUNT(*)
		FROM audit_logs
		WHERE 1=1
	`)

	args := []interface{}{}
	argCounter := 1

	// Add filter conditions
	if tenantID != emptyUUID {
		queryBuilder.WriteString(fmt.Sprintf(" AND tenant_id = $%d", argCounter))
		args = append(args, tenantID)
		argCounter++
	}

	if userID != emptyUUID {
		queryBuilder.WriteString(fmt.Sprintf(" AND actor_type = 'user' AND actor_id = $%d", argCounter))
		args = append(args, userID)
		argCounter++
	}

	if eventType != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND event_type = $%d", argCounter))
		args = append(args, eventType)
		argCounter++
	}

	if resourceType != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND resource_type = $%d", argCounter))
		args = append(args, resourceType)
		argCounter++
	}

	if resourceID != emptyUUID {
		queryBuilder.WriteString(fmt.Sprintf(" AND resource_id = $%d", argCounter))
		args = append(args, resourceID)
		argCounter++
	}

	if !startDate.IsZero() {
		queryBuilder.WriteString(fmt.Sprintf(" AND created_at >= $%d", argCounter))
		args = append(args, startDate)
		argCounter++
	}

	if !endDate.IsZero() {
		queryBuilder.WriteString(fmt.Sprintf(" AND created_at <= $%d", argCounter))
		args = append(args, endDate)
		argCounter++
	}

	var count int
	err := a.db.QueryRow(ctx, queryBuilder.String(), args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit logs with filters: %w", err)
	}
	return count, nil
}

// AggregateByEventType aggregates audit logs by event type
func (a *AuditLogRepository) AggregateByEventType(ctx context.Context, startDate, endDate time.Time) ([]*domain.AuditLogAggregate, error) {
	query := `
		SELECT event_type as group_key, COUNT(*) as count
		FROM audit_logs
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY event_type
		ORDER BY count DESC
	`

	return a.queryAggregation(ctx, query, startDate, endDate)
}

// AggregateByActorType aggregates audit logs by actor type
func (a *AuditLogRepository) AggregateByActorType(ctx context.Context, startDate, endDate time.Time) ([]*domain.AuditLogAggregate, error) {
	query := `
		SELECT actor_type as group_key, COUNT(*) as count
		FROM audit_logs
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY actor_type
		ORDER BY count DESC
	`

	return a.queryAggregation(ctx, query, startDate, endDate)
}

// AggregateByTenant aggregates audit logs by tenant
func (a *AuditLogRepository) AggregateByTenant(ctx context.Context, startDate, endDate time.Time) ([]*domain.AuditLogAggregate, error) {
	query := `
		SELECT tenant_id::text as group_key, COUNT(*) as count
		FROM audit_logs
		WHERE tenant_id IS NOT NULL AND created_at >= $1 AND created_at <= $2
		GROUP BY tenant_id
		ORDER BY count DESC
	`

	return a.queryAggregation(ctx, query, startDate, endDate)
}

// ExportToCSV exports audit logs to CSV format based on filters
func (a *AuditLogRepository) ExportToCSV(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID,
	eventType, resourceType string, resourceID uuid.UUID, startDate, endDate time.Time) ([]byte, error) {

	var emptyUUID uuid.UUID // Zero value for UUID comparison

	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT 
			id, event_type, actor_type, actor_id, tenant_id, resource_type, 
			resource_id, ip_address, user_agent, event_data, created_at
		FROM audit_logs
		WHERE 1=1
	`)

	args := []interface{}{}
	argCounter := 1

	// Add filter conditions
	if tenantID != emptyUUID {
		queryBuilder.WriteString(fmt.Sprintf(" AND tenant_id = $%d", argCounter))
		args = append(args, tenantID)
		argCounter++
	}

	if userID != emptyUUID {
		queryBuilder.WriteString(fmt.Sprintf(" AND actor_type = 'user' AND actor_id = $%d", argCounter))
		args = append(args, userID)
		argCounter++
	}

	if eventType != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND event_type = $%d", argCounter))
		args = append(args, eventType)
		argCounter++
	}

	if resourceType != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND resource_type = $%d", argCounter))
		args = append(args, resourceType)
		argCounter++
	}

	if resourceID != emptyUUID {
		queryBuilder.WriteString(fmt.Sprintf(" AND resource_id = $%d", argCounter))
		args = append(args, resourceID)
		argCounter++
	}

	if !startDate.IsZero() {
		queryBuilder.WriteString(fmt.Sprintf(" AND created_at >= $%d", argCounter))
		args = append(args, startDate)
		argCounter++
	}

	if !endDate.IsZero() {
		queryBuilder.WriteString(fmt.Sprintf(" AND created_at <= $%d", argCounter))
		args = append(args, endDate)
		argCounter++
	}

	queryBuilder.WriteString(" ORDER BY created_at DESC")
	queryBuilder.WriteString(" LIMIT 10000") // Reasonable limit for CSV export

	logs, err := a.queryAuditLogs(ctx, queryBuilder.String(), 10000, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs for CSV export: %w", err)
	}

	// Create CSV
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{
		"ID", "Event Type", "Actor Type", "Actor ID", "Tenant ID",
		"Resource Type", "Resource ID", "IP Address", "User Agent",
		"Event Data", "Created At",
	}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write log entries
	for _, log := range logs {
		eventDataJSON, err := json.Marshal(log.EventData)
		if err != nil {
			eventDataJSON = []byte("{}")
		}

		ipAddress := ""
		if log.IPAddress != nil {
			ipAddress = log.IPAddress.String()
		}

		row := []string{
			log.ID.String(),
			log.EventType,
			log.ActorType,
			log.ActorID,
			log.TenantID.String(),
			log.ResourceType,
			log.ResourceID.String(),
			ipAddress,
			log.UserAgent,
			string(eventDataJSON),
			log.CreatedAt.Format(time.RFC3339),
		}

		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("failed to flush CSV writer: %w", err)
	}

	return buf.Bytes(), nil
}

// queryAuditLogs is a helper function to query audit logs
func (a *AuditLogRepository) queryAuditLogs(ctx context.Context, query string, pageSize int, args ...interface{}) ([]*domain.AuditLog, error) {
	rows, err := a.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	logs := make([]*domain.AuditLog, 0, pageSize)
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

// queryAggregation is a helper function to query aggregated audit logs
func (a *AuditLogRepository) queryAggregation(ctx context.Context, query string, args ...interface{}) ([]*domain.AuditLogAggregate, error) {
	rows, err := a.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit log aggregation: %w", err)
	}
	defer rows.Close()

	aggregates := make([]*domain.AuditLogAggregate, 0)
	for rows.Next() {
		var aggregate domain.AuditLogAggregate

		if err := rows.Scan(&aggregate.GroupKey, &aggregate.Count); err != nil {
			return nil, fmt.Errorf("failed to scan audit log aggregate: %w", err)
		}

		aggregates = append(aggregates, &aggregate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit log aggregate rows: %w", err)
	}

	return aggregates, nil
}
