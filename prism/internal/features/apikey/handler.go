package apikey

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/jasonKoogler/prism/internal/common/api"
	"github.com/jasonKoogler/prism/internal/features/apikey/dto"
	"github.com/jasonKoogler/prism/internal/ports"
)

type apiKeyHandler struct {
	apiKeyService ports.ApiKeyService
}

func NewApiKeyHandler(apiKeyService ports.ApiKeyService) *apiKeyHandler {
	return &apiKeyHandler{
		apiKeyService: apiKeyService,
	}
}

// listApiKeys godoc
// @Summary      List API keys
// @Description  Get paginated list of API keys filtered by tenant or user
// @Tags         apikey
// @Produce      json
// @Param        tenant_id  query  string  false  "Filter by tenant ID"
// @Param        user_id    query  string  false  "Filter by user ID"
// @Param        page       query  int     false  "Page number"  default(1)
// @Param        pageSize   query  int     false  "Page size"    default(20)
// @Success      200  {array}   dto.ApiKeyResponse
// @Failure      400  {object}  api.APIError
// @Failure      500  {object}  api.APIError
// @Security     BearerAuth
// @Router       /apikey [get]
func (h *apiKeyHandler) listApiKeys(w http.ResponseWriter, r *http.Request) error {
	domainParams, err := dto.ListApiKeysParamsFromRequest(r)
	if err != nil {
		return api.InvalidJSONError()
	}

	keys, err := h.apiKeyService.List(r.Context(), domainParams)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, keys)
}

// createApiKey godoc
// @Summary      Create API key
// @Description  Create a new API key for a tenant or user
// @Tags         apikey
// @Accept       json
// @Produce      json
// @Param        body  body      dto.ApiKeyCreateRequest  true  "API key creation request"
// @Success      201   {object}  dto.ApiKeyCreateResponse
// @Failure      400   {object}  api.APIError
// @Failure      500   {object}  api.APIError
// @Security     BearerAuth
// @Router       /apikey [post]
func (h *apiKeyHandler) createApiKey(w http.ResponseWriter, r *http.Request) error {
	params, err := dto.CreateApiKeyRequestToParams(r)
	if err != nil {
		return api.InvalidJSONError()
	}

	createdKey, err := h.apiKeyService.Create(r.Context(), params)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusCreated, createdKey)
}

// validateApiKey godoc
// @Summary      Validate API key
// @Description  Validate an API key and return its details if valid
// @Tags         apikey
// @Accept       json
// @Produce      json
// @Param        body  body      dto.ApiKeyValidateRequest  true  "API key validation request"
// @Success      200   {object}  dto.ApiKeyResponse
// @Failure      400   {object}  api.APIError
// @Failure      500   {object}  api.APIError
// @Security     BearerAuth
// @Router       /apikey/validate [post]
func (h *apiKeyHandler) validateApiKey(w http.ResponseWriter, r *http.Request) error {
	params, err := dto.ValidateApiKeyRequestToParams(r)
	if err != nil {
		return api.InvalidJSONError()
	}

	validatedKey, err := h.apiKeyService.Validate(r.Context(), params.RawAPIKey, params.IPAddress)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, validatedKey)
}

// revokeApiKey godoc
// @Summary      Revoke API key
// @Description  Revoke an existing API key by ID
// @Tags         apikey
// @Param        apikeyID  path  string  true  "API key ID"
// @Success      204  "No Content"
// @Failure      400  {object}  api.APIError
// @Failure      500  {object}  api.APIError
// @Security     BearerAuth
// @Router       /apikey/{apikeyID} [delete]
func (h *apiKeyHandler) revokeApiKey(w http.ResponseWriter, r *http.Request) error {
	apikeyID := chi.URLParam(r, "apikeyID")
	if apikeyID == "" {
		return api.MissingIDError()
	}

	err := h.apiKeyService.Revoke(r.Context(), apikeyID)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusNoContent, nil)
}

// getApiKey godoc
// @Summary      Get API key
// @Description  Retrieve an API key by ID
// @Tags         apikey
// @Produce      json
// @Param        apikeyID  path  string  true  "API key ID"
// @Success      200  {object}  dto.ApiKeyResponse
// @Failure      400  {object}  api.APIError
// @Failure      404  {object}  api.APIError
// @Failure      500  {object}  api.APIError
// @Security     BearerAuth
// @Router       /apikey/{apikeyID} [get]
func (h *apiKeyHandler) getApiKey(w http.ResponseWriter, r *http.Request) error {
	apikeyID := chi.URLParam(r, "apikeyID")
	if apikeyID == "" {
		return api.MissingIDError()
	}

	apiKey, err := h.apiKeyService.Get(r.Context(), apikeyID)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, apiKey)
}

// updateApiKeyMetadata godoc
// @Summary      Update API key metadata
// @Description  Update the metadata of an existing API key
// @Tags         apikey
// @Accept       json
// @Produce      json
// @Param        apikeyID  path  string                          true  "API key ID"
// @Param        body      body  dto.UpdateApiKeyMetadataRequest  true  "Metadata update request"
// @Success      200  {object}  dto.ApiKeyResponse
// @Failure      400  {object}  api.APIError
// @Failure      500  {object}  api.APIError
// @Security     BearerAuth
// @Router       /apikey/{apikeyID}/metadata [put]
func (h *apiKeyHandler) updateApiKeyMetadata(w http.ResponseWriter, r *http.Request) error {
	apikeyID := chi.URLParam(r, "apikeyID")
	if apikeyID == "" {
		return api.MissingIDError()
	}

	params, err := dto.UpdateApiKeyMetadataRequestToParams(r, apikeyID)
	if err != nil {
		return api.InvalidJSONError()
	}

	updatedKey, err := h.apiKeyService.UpdateMetadata(r.Context(), params)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, updatedKey)
}
