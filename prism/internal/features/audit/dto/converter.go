package dto

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jasonKoogler/prism/internal/domain"
)

// ListAuditLogsParamsFromRequest extracts list audit log parameters from the HTTP request query string.
func ListAuditLogsParamsFromRequest(r *http.Request) (*domain.ListAuditLogsReq, error) {
	q := r.URL.Query()
	return &domain.ListAuditLogsReq{
		TenantID:     optionalString(q.Get("tenant_id")),
		UserID:       optionalString(q.Get("user_id")),
		EventType:    optionalString(q.Get("event_type")),
		ResourceType: optionalString(q.Get("resource_type")),
		ResourceID:   optionalString(q.Get("resource_id")),
		Page:         optionalInt(q.Get("page")),
		PageSize:     optionalInt(q.Get("pageSize")),
		StartDate:    optionalString(q.Get("start_date")),
		EndDate:      optionalString(q.Get("end_date")),
	}, nil
}

// AggregateParamsFromRequest extracts start_date and end_date query parameters for aggregation.
func AggregateParamsFromRequest(r *http.Request) (startDate, endDate *time.Time) {
	q := r.URL.Query()
	if v := q.Get("start_date"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			startDate = &t
		}
	}
	if v := q.Get("end_date"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			endDate = &t
		}
	}
	return
}

// ExportAuditLogsParamsFromRequest extracts export audit log parameters from the HTTP request query string.
func ExportAuditLogsParamsFromRequest(r *http.Request) (*domain.ExportAuditLogsReq, error) {
	q := r.URL.Query()
	return &domain.ExportAuditLogsReq{
		TenantID: optionalString(q.Get("tenant_id")),
	}, nil
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optionalInt(s string) *int {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}
