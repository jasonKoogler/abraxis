package audit

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/jasonKoogler/abraxis/prism/internal/common/api"
	"github.com/jasonKoogler/abraxis/prism/internal/common/log"
	"github.com/jasonKoogler/abraxis/prism/internal/config"
	"github.com/jasonKoogler/abraxis/prism/internal/domain"
	"github.com/jasonKoogler/abraxis/prism/internal/domain/prefixid"
	"github.com/jasonKoogler/abraxis/prism/internal/features/audit/dto"
	"github.com/jasonKoogler/abraxis/prism/internal/ports"
)

type auditHandler struct {
	auditService ports.AuditService

	config *config.Config
	logger *log.Logger
}

func NewAuditHandler(auditService ports.AuditService, config *config.Config, logger *log.Logger) *auditHandler {
	return &auditHandler{
		auditService: auditService,
		config:       config,
		logger:       logger,
	}
}

// listAuditLogs godoc
// @Summary      List audit logs
// @Description  Get paginated, filtered list of audit logs
// @Tags         audit
// @Produce      json
// @Param        tenant_id     query  string  false  "Filter by tenant ID"
// @Param        user_id       query  string  false  "Filter by user ID"
// @Param        event_type    query  string  false  "Filter by event type"
// @Param        resource_type query  string  false  "Filter by resource type"
// @Param        resource_id   query  string  false  "Filter by resource ID"
// @Param        page          query  int     false  "Page number"  default(1)
// @Param        pageSize      query  int     false  "Page size"    default(20)
// @Param        start_date    query  string  false  "Start date (RFC3339)"
// @Param        end_date      query  string  false  "End date (RFC3339)"
// @Success      200  {object}  domain.AuditLogListResponse
// @Failure      400  {object}  api.APIError
// @Failure      500  {object}  api.APIError
// @Security     BearerAuth
// @Router       /audit [get]
func (h *auditHandler) listAuditLogs(w http.ResponseWriter, r *http.Request) error {
	domainReq, err := dto.ListAuditLogsParamsFromRequest(r)
	if err != nil {
		return api.InvalidJSONError()
	}

	logs, err := h.auditService.ListAuditLogs(r.Context(), domainReq)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidRequest) {
			return api.NewError(
				"invalid-request",
				"Invalid request parameters - at least one filter must be specified",
				http.StatusBadRequest,
				api.ErrorTypeBadRequest,
				err,
			)
		}
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, logs)
}

// getAuditLog godoc
// @Summary      Get audit log by ID
// @Description  Retrieve a single audit log entry
// @Tags         audit
// @Produce      json
// @Param        auditID  path  string  true  "Audit log ID"
// @Success      200  {object}  domain.AuditLog
// @Failure      400  {object}  api.APIError
// @Failure      404  {object}  api.APIError
// @Failure      500  {object}  api.APIError
// @Security     BearerAuth
// @Router       /audit/{auditID} [get]
func (h *auditHandler) getAuditLog(w http.ResponseWriter, r *http.Request) error {
	auditID := chi.URLParam(r, "auditID")

	auditLog, err := h.auditService.GetAuditLog(r.Context(), auditID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return api.NotFound(err)
		}
		if errors.Is(err, prefixid.ErrInvalidID) {
			return api.InvalidQueryParamError("audit_id")
		}
		return api.InternalError(err)
	}
	return api.Respond(w, http.StatusOK, auditLog)
}

// aggregateAuditLogs godoc
// @Summary      Aggregate audit logs
// @Description  Group and count audit logs by a specified field
// @Tags         audit
// @Produce      json
// @Param        groupBy     path   string  true   "Group by field"  Enums(event_type, actor_type, tenant)
// @Param        start_date  query  string  false  "Start date (RFC3339)"
// @Param        end_date    query  string  false  "End date (RFC3339)"
// @Success      200  {object}  domain.AuditLogAggregateResponse
// @Failure      500  {object}  api.APIError
// @Security     BearerAuth
// @Router       /audit/aggregate/{groupBy} [get]
func (h *auditHandler) aggregateAuditLogs(w http.ResponseWriter, r *http.Request) error {
	groupBy := chi.URLParam(r, "groupBy")
	startDatePtr, endDatePtr := dto.AggregateParamsFromRequest(r)

	var startDate, endDate time.Time
	if startDatePtr != nil {
		startDate = *startDatePtr
	}
	if endDatePtr != nil {
		endDate = *endDatePtr
	}

	aggregation, err := h.auditService.AggregateAuditLogs(r.Context(), groupBy, startDate, endDate)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, aggregation)
}

// exportAuditLogs godoc
// @Summary      Export audit logs
// @Description  Export audit logs as CSV file
// @Tags         audit
// @Produce      text/csv
// @Param        tenant_id  query  string  false  "Filter by tenant ID"
// @Success      200  {file}  file  "CSV file download"
// @Failure      400  {object}  api.APIError
// @Failure      500  {object}  api.APIError
// @Security     BearerAuth
// @Router       /audit/export [get]
func (h *auditHandler) exportAuditLogs(w http.ResponseWriter, r *http.Request) error {
	domainReq, err := dto.ExportAuditLogsParamsFromRequest(r)
	if err != nil {
		return api.InvalidJSONError()
	}

	csvData, err := h.auditService.ExportAuditLogs(r.Context(), domainReq)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidRequest) {
			return api.NewError(
				"invalid-request",
				"Invalid request parameters",
				http.StatusBadRequest,
				api.ErrorTypeBadRequest,
				err,
			)
		}
		return api.InternalError(err)
	}

	// Set headers for CSV download
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=audit_logs.csv")

	// Write CSV data directly to response
	_, err = w.Write(csvData)
	if err != nil {
		return api.InternalError(err)
	}

	return nil
}
