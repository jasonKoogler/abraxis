package dto

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/prism/internal/common/api"
	"github.com/jasonKoogler/abraxis/prism/internal/domain"
	"github.com/jasonKoogler/abraxis/prism/internal/domain/prefixid"
)

func CreateApiKeyRequestToParams(r *http.Request) (*domain.APIKey_CreateParams, error) {
	req := &ApiKeyCreateRequest{}
	if err := api.BindRequest(r, req); err != nil {
		return nil, err
	}

	var tenantUUID, userUUID uuid.UUID
	if req.TenantID != nil {
		tenantUUID = *req.TenantID
	}

	if req.UserID != nil {
		userUUID = *req.UserID
	}

	params := &domain.APIKey_CreateParams{
		Name:          req.Name,
		TenantID:      tenantUUID,
		UserID:        userUUID,
		Scopes:        req.Scopes,
		ExpiresInDays: derefIntOrDefault(req.ExpiresInDays, 30),
		IPAddress:     r.RemoteAddr,
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	return params, nil
}

func ValidateApiKeyRequestToParams(r *http.Request) (*domain.APIKey_ValidateParams, error) {
	req := &ApiKeyValidateRequest{}
	if err := api.BindRequest(r, req); err != nil {
		return nil, err
	}

	params := &domain.APIKey_ValidateParams{
		RawAPIKey: req.ApiKey,
		IPAddress: r.RemoteAddr,
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	return params, nil
}

func UpdateApiKeyMetadataRequestToParams(r *http.Request, apikeyID string) (*domain.APIKey_UpdateMetadataParams, error) {
	req := &UpdateApiKeyMetadataRequest{}
	if err := api.BindRequest(r, req); err != nil {
		return nil, err
	}

	idObj, err := prefixid.ParseApiKeyID(apikeyID)
	if err != nil {
		return nil, err
	}

	rawID, err := uuid.Parse(idObj.Raw().String())
	if err != nil {
		return nil, err
	}

	params := &domain.APIKey_UpdateMetadataParams{
		ID:            rawID,
		Name:          req.Name,
		Scopes:        req.Scopes,
		IsActive:      req.IsActive,
		ExpiresInDays: req.ExpiresInDays,
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	return params, nil
}

func ListApiKeysParamsFromRequest(r *http.Request) (*domain.APIKey_ListParams, error) {
	q := r.URL.Query()

	var tenantUUID, userUUID *uuid.UUID

	if tid := q.Get("tenant_id"); tid != "" {
		tenantID, err := prefixid.ParseTenantID(tid)
		if err != nil {
			return nil, err
		}

		parsed, err := uuid.Parse(tenantID.Raw().String())
		if err != nil {
			return nil, err
		}
		tenantUUID = &parsed
	}

	if uid := q.Get("user_id"); uid != "" {
		userID, err := prefixid.ParseUserID(uid)
		if err != nil {
			return nil, err
		}

		parsed, err := uuid.Parse(userID.Raw().String())
		if err != nil {
			return nil, err
		}
		userUUID = &parsed
	}

	page := 1
	pageSize := 20

	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			page = v
		}
	}

	if ps := q.Get("pageSize"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil {
			pageSize = v
		}
	}

	return &domain.APIKey_ListParams{
		TenantID: tenantUUID,
		UserID:   userUUID,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func derefIntOrDefault(p *int, defaultVal int) int {
	if p == nil {
		return defaultVal
	}
	return *p
}
