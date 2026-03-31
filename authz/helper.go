package authz

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// hashInput generates a stable hash of the input object
func hashInput(queryPath string, input interface{}) (string, error) {
	// Normalize the input
	normalized, err := normalizeForHashing(input)
	if err != nil {
		return "", err
	}

	// Combine query path and normalized input
	combined := fmt.Sprintf("%s:%s", queryPath, normalized)

	// Generate SHA-256 hash
	hasher := sha256.New()
	hasher.Write([]byte(combined))
	hash := hex.EncodeToString(hasher.Sum(nil))

	return hash, nil
}

// normalizeForHashing normalizes an object for stable hashing
func normalizeForHashing(v interface{}) (string, error) {
	switch val := v.(type) {
	case nil:
		return "null", nil
	case bool:
		if val {
			return "true", nil
		}
		return "false", nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprintf("%v", val), nil
	case string:
		return val, nil
	case []interface{}:
		var items []string
		for _, item := range val {
			normalized, err := normalizeForHashing(item)
			if err != nil {
				return "", err
			}
			items = append(items, normalized)
		}
		// Sort array items for consistent hashing
		sort.Strings(items)
		return fmt.Sprintf("[%s]", strings.Join(items, ",")), nil
	case map[string]interface{}:
		var items []string
		// Get sorted keys for consistent hashing
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := val[k]
			normalized, err := normalizeForHashing(v)
			if err != nil {
				return "", err
			}
			items = append(items, fmt.Sprintf("%s:%s", k, normalized))
		}
		return fmt.Sprintf("{%s}", strings.Join(items, ",")), nil
	default:
		// For other types, try to convert to JSON
		bytes, err := json.Marshal(val)
		if err != nil {
			return "", fmt.Errorf("failed to normalize value for hashing: %w", err)
		}
		return string(bytes), nil
	}
}

// splitPath splits a field path string into parts
func splitPath(path string) []string {
	// For simplicity, we'll use a basic split
	// In a real implementation, you might want to handle escaping
	return strings.Split(path, ".")
}

// getNestedValue retrieves a nested value from a map using a path
func getNestedValue(data map[string]interface{}, path []string) interface{} {
	if len(path) == 0 {
		return nil
	}

	// Get the value at the current path segment
	value, ok := data[path[0]]
	if !ok {
		return nil
	}

	// If this is the last segment, return the value
	if len(path) == 1 {
		return value
	}

	// Otherwise, recursively get the nested value
	if nestedMap, ok := value.(map[string]interface{}); ok {
		return getNestedValue(nestedMap, path[1:])
	}

	// If the value is not a map, we can't traverse further
	return nil
}

// setNestedValue sets a nested value in a map using a path
func setNestedValue(data map[string]interface{}, path []string, value interface{}) {
	if len(path) == 0 {
		return
	}

	// If this is the last segment, set the value
	if len(path) == 1 {
		data[path[0]] = value
		return
	}

	// Otherwise, create or get the nested map
	nestedMap, ok := data[path[0]].(map[string]interface{})
	if !ok {
		// Create a new map if one doesn't exist
		nestedMap = make(map[string]interface{})
		data[path[0]] = nestedMap
	}

	// Recursively set the nested value
	setNestedValue(nestedMap, path[1:], value)
}
