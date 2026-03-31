package provider

import (
	"encoding/json"

	"github.com/jasonKoogler/prism/internal/ports"
)

// SerializeInstance converts a ServiceInstance to a JSON byte array
func SerializeInstance(instance *ports.ServiceInstance) ([]byte, error) {
	// Marshal the instance to JSON
	return json.Marshal(instance)
}

// DeserializeInstance converts a JSON byte array back to a ServiceInstance
func DeserializeInstance(data []byte) (*ports.ServiceInstance, error) {
	// Create a new instance
	instance := &ports.ServiceInstance{}

	// Unmarshal the JSON data
	err := json.Unmarshal(data, instance)
	if err != nil {
		return nil, err
	}

	return instance, nil
}

// DeserializeInstanceWithDefaults converts a JSON byte array to a ServiceInstance
// and sets default values for any missing fields
func DeserializeInstanceWithDefaults(data []byte) (*ports.ServiceInstance, error) {
	instance, err := DeserializeInstance(data)
	if err != nil {
		return nil, err
	}

	// Set default values if not present
	if instance.Status == "" {
		instance.Status = "UNKNOWN"
	}

	if instance.Version == "" {
		instance.Version = "0.0.0"
	}

	if instance.Metadata == nil {
		instance.Metadata = make(map[string]string)
	}

	return instance, nil
}

// SerializeMap converts a map to a JSON byte array
func SerializeMap(data map[string]string) ([]byte, error) {
	return json.Marshal(data)
}

// DeserializeMap converts a JSON byte array back to a map
func DeserializeMap(data []byte) (map[string]string, error) {
	result := make(map[string]string)
	err := json.Unmarshal(data, &result)
	return result, err
}
