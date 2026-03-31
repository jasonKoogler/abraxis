package domain

import (
	"fmt"
	"net"
	"time"

	"github.com/jasonKoogler/abraxis/prism/internal/common/id"
	"github.com/jasonKoogler/abraxis/prism/internal/domain/prefixid"
)

// AuditLog represents a security audit log entry
type AuditLog struct {
	// ID is the unique identifier for the audit log entry with the "aud" prefix
	ID id.ID `json:"id"`
	// EventType categorizes the type of action performed (e.g., "login", "create", "update", "delete")
	EventType string `json:"event_type"`
	// ActorType identifies the type of entity that performed the action (e.g., "user", "system", "service")
	ActorType string `json:"actor_type"`
	// ActorID is the unique identifier of the actor who performed the action such as a user or api key (usr_1234567890, apk_1234567890)
	ActorID string `json:"actor_id"`
	// TenantID is the unique identifier of the tenant where the action occurred with the "tnt" prefix
	TenantID id.ID `json:"tenant_id"`
	// ResourceType identifies the type of resource that was affected (e.g., "user", "apikey", "service")
	ResourceType string `json:"resource_type"`
	// ResourceID is the unique identifier of the resource that was affected with the "res" prefix
	ResourceID id.ID `json:"resource_id"`
	// IPAddress stores the IP address from which the action was performed
	IPAddress net.IP `json:"ip_address"`
	// UserAgent contains the user agent string from the client that performed the action
	UserAgent string `json:"user_agent"`
	// EventData stores additional contextual information about the event in a flexible format
	EventData any `json:"event_data,omitempty"`
	// CreatedAt is the timestamp when the audit log entry was created
	CreatedAt time.Time `json:"created_at"`
}

func NewAuditLog(req *AuditLogReq) (*AuditLog, error) {
	params, err := req.Parse()
	if err != nil {
		return nil, err
	}

	id, err := prefixid.NewAuditLogID()
	if err != nil {
		return nil, err
	}

	return &AuditLog{
		ID:           id,
		EventType:    params.EventType,
		ActorType:    params.ActorType,
		ActorID:      params.ActorID,
		TenantID:     params.TenantID,
		ResourceType: params.ResourceType,
		ResourceID:   params.ResourceID,
		IPAddress:    params.IPAddress,
		UserAgent:    params.UserAgent,
		EventData:    params.EventData,
		CreatedAt:    time.Now(),
	}, nil
}

type AuditLogReq struct {
	EventType    string
	ActorType    string
	ActorID      string
	TenantID     string
	ResourceType string
	ResourceID   string
	IPAddress    net.IP
	UserAgent    string
	EventData    any
}

func (r *AuditLogReq) Parse() (*AuditLogParams, error) {
	tenantID, err := prefixid.ParseTenantID(r.TenantID)
	if err != nil {
		return nil, err
	}
	resourceID, err := prefixid.ParseResourceID(r.ResourceID)
	if err != nil {
		return nil, err
	}
	return &AuditLogParams{
		EventType:    r.EventType,
		ActorType:    r.ActorType,
		ActorID:      r.ActorID,
		TenantID:     tenantID,
		ResourceType: r.ResourceType,
		ResourceID:   resourceID,
		IPAddress:    r.IPAddress,
		UserAgent:    r.UserAgent,
		EventData:    r.EventData,
	}, nil
}

type AuditLogParams struct {
	EventType    string
	ActorType    string
	ActorID      string
	TenantID     id.ID
	ResourceType string
	ResourceID   id.ID
	IPAddress    net.IP
	UserAgent    string
	EventData    any
}

type ListAuditLogsReq struct {
	TenantID     *string `json:"tenant_id,omitempty"`
	UserID       *string `json:"user_id,omitempty"`
	EventType    *string `json:"event_type,omitempty"`
	ResourceType *string `json:"resource_type,omitempty"`
	ResourceID   *string `json:"resource_id,omitempty"`
	StartDate    *string `json:"start_date,omitempty"` // Format: 2006-01-02T15:04:05Z
	EndDate      *string `json:"end_date,omitempty"`   // Format: 2006-01-02T15:04:05Z
	Page         *int    `json:"page,omitempty"`
	PageSize     *int    `json:"page_size,omitempty"`
}

type ListAuditLogsParams struct {
	TenantID     id.ID
	UserID       id.ID
	EventType    string
	ResourceType string
	ResourceID   id.ID
	StartDate    time.Time
	EndDate      time.Time
	Page         int
	PageSize     int
}

func (r *ListAuditLogsReq) Parse() (*ListAuditLogsParams, error) {
	var tenantID, userID, resourceID id.ID
	var err error
	if r.TenantID != nil {
		tenantID, err = prefixid.ParseTenantID(*r.TenantID)
		if err != nil {
			return nil, err
		}
	}
	if r.UserID != nil {
		userID, err = prefixid.ParseUserID(*r.UserID)
		if err != nil {
			return nil, err
		}
	}
	if r.ResourceID != nil {
		resourceID, err = prefixid.ParseResourceID(*r.ResourceID)
		if err != nil {
			return nil, err
		}
	}

	// Parse eventType and resourceType if not nil
	eventType := ""
	if r.EventType != nil {
		eventType = *r.EventType
	}

	resourceType := ""
	if r.ResourceType != nil {
		resourceType = *r.ResourceType
	}

	// Default time range if not specified
	startDate := time.Time{}
	if r.StartDate != nil {
		startDate, err = time.Parse(time.RFC3339, *r.StartDate)
		if err != nil {
			return nil, fmt.Errorf("invalid start_date format: %w", err)
		}
	}

	endDate := time.Now()
	if r.EndDate != nil {
		endDate, err = time.Parse(time.RFC3339, *r.EndDate)
		if err != nil {
			return nil, fmt.Errorf("invalid end_date format: %w", err)
		}
	}

	// Default pagination
	page := 0
	if r.Page != nil {
		page = *r.Page
	}

	pageSize := 20
	if r.PageSize != nil {
		pageSize = *r.PageSize
	}

	return &ListAuditLogsParams{
		TenantID:     tenantID,
		UserID:       userID,
		EventType:    eventType,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		StartDate:    startDate,
		EndDate:      endDate,
		Page:         page,
		PageSize:     pageSize,
	}, nil
}

// PaginationMetadata contains metadata about paginated results
type PaginationMetadata struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// AuditLogListResponse represents the response for a list of audit logs
type AuditLogListResponse struct {
	Data       []*AuditLog        `json:"data"`
	Pagination PaginationMetadata `json:"pagination"`
}

// AuditLogAggregate represents aggregated audit log data
type AuditLogAggregate struct {
	GroupKey string `json:"group_key"`
	Count    int    `json:"count"`
}

// AuditLogAggregateResponse represents the response for aggregated audit logs
type AuditLogAggregateResponse struct {
	Data       []*AuditLogAggregate `json:"data"`
	GroupBy    string               `json:"group_by"`
	StartDate  time.Time            `json:"start_date,omitempty"`
	EndDate    time.Time            `json:"end_date,omitempty"`
	TotalCount int                  `json:"total_count"`
}

type ExportAuditLogsReq struct {
	TenantID     *string
	UserID       *string
	EventType    *string
	ResourceType *string
	ResourceID   *string
	StartDate    *string
	EndDate      *string
}

func (r *ExportAuditLogsReq) Parse() (*ExportAuditLogsParams, error) {
	var tenantID, userID, resourceID id.ID
	var err error
	if r.TenantID != nil {
		tenantID, err = prefixid.ParseTenantID(*r.TenantID)
		if err != nil {
			return nil, err
		}
	}
	if r.UserID != nil {
		userID, err = prefixid.ParseUserID(*r.UserID)
		if err != nil {
			return nil, err
		}
	}
	if r.ResourceID != nil {
		resourceID, err = prefixid.ParseResourceID(*r.ResourceID)
		if err != nil {
			return nil, err
		}
	}

	startDate := time.Time{}
	if r.StartDate != nil {
		startDate, err = time.Parse(time.RFC3339, *r.StartDate)
		if err != nil {
			return nil, fmt.Errorf("invalid start_date format: %w", err)
		}
	}

	endDate := time.Time{}
	if r.EndDate != nil {
		endDate, err = time.Parse(time.RFC3339, *r.EndDate)
		if err != nil {
			return nil, fmt.Errorf("invalid end_date format: %w", err)
		}
	}

	eventType := ""
	if r.EventType != nil {
		eventType = *r.EventType
	}

	resourceType := ""
	if r.ResourceType != nil {
		resourceType = *r.ResourceType
	}

	return &ExportAuditLogsParams{
		TenantID:     tenantID,
		UserID:       userID,
		EventType:    eventType,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		StartDate:    startDate,
		EndDate:      endDate,
	}, nil
}

type ExportAuditLogsParams struct {
	TenantID     id.ID
	UserID       id.ID
	EventType    string
	ResourceType string
	ResourceID   id.ID
	StartDate    time.Time
	EndDate      time.Time
}
