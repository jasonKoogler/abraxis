package authz

import (
	"fmt"
	"net/http"
)

// StandardInputExtractor extracts standard authorization input from an HTTP request
func StandardInputExtractor(r *http.Request) (interface{}, error) {
	// Extract user ID from request context or header
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		return nil, fmt.Errorf("missing user ID")
	}

	// Map HTTP method to action
	action := "read"
	switch r.Method {
	case http.MethodPost:
		action = "create"
	case http.MethodPut, http.MethodPatch:
		action = "update"
	case http.MethodDelete:
		action = "delete"
	}

	// Extract resource information from URL path and query parameters
	resourceType := r.URL.Query().Get("type")
	if resourceType == "" {
		resourceType = "default"
	}

	resourceID := r.URL.Query().Get("id")

	// Build input object
	input := map[string]interface{}{
		"user": map[string]interface{}{
			"id": userID,
			// Roles will be added by the role provider if configured
		},
		"resource": map[string]interface{}{
			"type": resourceType,
			"id":   resourceID,
			"path": r.URL.Path,
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
