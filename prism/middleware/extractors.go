package middleware

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// StandardInputExtractor extracts standard authorization input from an HTTP request
func StandardInputExtractor(r *http.Request) (interface{}, error) {
	// Extract user ID from request context or header
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		return nil, fmt.Errorf("missing user ID")
	}

	// Map HTTP method to action
	action := mapMethodToAction(r.Method)

	// Extract resource information from URL path and query parameters
	resourceType := r.URL.Query().Get("type")
	if resourceType == "" {
		resourceType = "default"
	}

	resourceID := r.URL.Query().Get("id")

	// Extract tenant ID if available
	tenantID, _ := ExtractTenantIDFromRequest(r)

	// Build input object
	input := map[string]interface{}{
		"user": map[string]interface{}{
			"id": userID,
			// Roles will be added by the role provider if configured
		},
		"resource": map[string]interface{}{
			"type":      resourceType,
			"id":        resourceID,
			"path":      r.URL.Path,
			"tenant_id": tenantID.String(),
		},
		"action": action,
	}

	return input, nil
}

// APIInputExtractor extracts API-specific authorization input from an HTTP request
func APIInputExtractor(r *http.Request) (interface{}, error) {
	// Get standard input first
	input, err := StandardInputExtractor(r)
	if err != nil {
		return nil, err
	}

	// Cast to map to add API-specific fields
	inputMap, ok := input.(map[string]interface{})
	if !ok {
		return input, nil
	}

	// Add API version from header or default to v1
	apiVersion := r.Header.Get("X-API-Version")
	if apiVersion == "" {
		apiVersion = "v1"
	}

	// Add API context
	inputMap["api"] = map[string]interface{}{
		"version": apiVersion,
	}

	return inputMap, nil
}

// ResourceInputExtractor creates an input extractor for a specific resource type
func ResourceInputExtractor(resourceType string, additionalAttrs map[string]interface{}) func(*http.Request) (interface{}, error) {
	return func(r *http.Request) (interface{}, error) {
		// Get standard input first
		input, err := StandardInputExtractor(r)
		if err != nil {
			return nil, err
		}

		// Cast to map to add resource-specific fields
		inputMap, ok := input.(map[string]interface{})
		if !ok {
			return input, nil
		}

		// Get the resource map
		resourceMap, ok := inputMap["resource"].(map[string]interface{})
		if !ok {
			resourceMap = make(map[string]interface{})
			inputMap["resource"] = resourceMap
		}

		// Set the resource type
		resourceMap["type"] = resourceType

		// Add additional attributes
		for k, v := range additionalAttrs {
			resourceMap[k] = v
		}

		return inputMap, nil
	}
}

// ExtractTenantIDFromRequest attempts to retrieve the tenant ID from the request.
// It first tries a URL parameter named "tenantID"; if not found, it looks for an "X-Tenant-ID" header.
func ExtractTenantIDFromRequest(r *http.Request) (uuid.UUID, bool) {
	tenantIDStr := chi.URLParam(r, "tenantID")
	if tenantIDStr == "" {
		tenantIDStr = r.Header.Get("X-Tenant-ID")
	}
	if tenantIDStr == "" {
		return uuid.UUID{}, false
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return uuid.UUID{}, false
	}
	return tenantID, true
}

// PathParamExtractor creates an input extractor that includes URL path parameters
func PathParamExtractor(r *http.Request) (interface{}, error) {
	// Get standard input first
	input, err := StandardInputExtractor(r)
	if err != nil {
		return nil, err
	}

	// Cast to map to add path parameters
	inputMap, ok := input.(map[string]interface{})
	if !ok {
		return input, nil
	}

	// Extract path parameters using chi router
	// This assumes the request is being handled by a chi router
	rctx := chi.RouteContext(r.Context())
	if rctx != nil {
		params := make(map[string]string)
		for i, key := range rctx.URLParams.Keys {
			if i < len(rctx.URLParams.Values) {
				params[key] = rctx.URLParams.Values[i]
			}
		}

		// Add path parameters to the input
		if len(params) > 0 {
			inputMap["params"] = params
		}
	}

	return inputMap, nil
}

// OwnershipExtractor creates an input extractor that checks resource ownership
func OwnershipExtractor(ownerIDParam string) func(*http.Request) (interface{}, error) {
	return func(r *http.Request) (interface{}, error) {
		// Get path parameters first
		input, err := PathParamExtractor(r)
		if err != nil {
			return nil, err
		}

		// Cast to map to add ownership information
		inputMap, ok := input.(map[string]interface{})
		if !ok {
			return input, nil
		}

		// Get the params map
		paramsMap, ok := inputMap["params"].(map[string]string)
		if !ok {
			return input, nil
		}

		// Get the resource map
		resourceMap, ok := inputMap["resource"].(map[string]interface{})
		if !ok {
			resourceMap = make(map[string]interface{})
			inputMap["resource"] = resourceMap
		}

		// Add owner ID if the parameter exists
		if ownerID, exists := paramsMap[ownerIDParam]; exists {
			resourceMap["owner"] = ownerID
		}

		return inputMap, nil
	}
}
