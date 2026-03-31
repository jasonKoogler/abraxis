// Package roles provides role provider implementations for the Agent library
package roles

import (
	"context"
	"errors"
)

// Helper functions for working with roles

// ExtractUserID extracts the user ID from the input
func ExtractUserID(input interface{}) (string, error) {
	// Try to extract user ID based on common input patterns
	if inputMap, ok := input.(map[string]interface{}); ok {
		if user, ok := inputMap["user"].(map[string]interface{}); ok {
			if id, ok := user["id"].(string); ok {
				return id, nil
			}
		}
		if subject, ok := inputMap["subject"].(string); ok {
			return subject, nil
		}
	}
	return "", errors.New("user ID not found in input")
}

// EnrichInputWithRoles enriches the input with roles
func EnrichInputWithRoles(input interface{}, roles []string) interface{} {
	if inputMap, ok := input.(map[string]interface{}); ok {
		if user, ok := inputMap["user"].(map[string]interface{}); ok {
			user["roles"] = roles
			return inputMap
		}
		// If there's no user object, create one
		inputMap["user"] = map[string]interface{}{
			"roles": roles,
		}
		return inputMap
	}
	// If input is not a map, return it unchanged
	return input
}

// CreateRoleTransformer creates a context transformer that enriches the input with roles
func CreateRoleTransformer(provider interface {
	GetRoles(ctx context.Context, userID string) ([]string, error)
}) func(ctx context.Context, input interface{}) (interface{}, error) {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		// Extract user ID from input
		userID, err := ExtractUserID(input)
		if err != nil {
			// If we can't extract the user ID, return the input unchanged
			return input, nil
		}

		// Get roles from provider
		roles, err := provider.GetRoles(ctx, userID)
		if err != nil {
			// If there's an error getting roles, log it but don't fail the request
			// Just return the input unchanged
			return input, nil
		}

		// Enrich input with roles
		return EnrichInputWithRoles(input, roles), nil
	}
}
