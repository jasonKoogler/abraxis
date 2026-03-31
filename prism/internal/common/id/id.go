package id

import (
	"errors"
	"strings"

	"github.com/jasonKoogler/prism/internal/common/uuid"
	// "github.com/google/uuid"
)

// ID is the interface for all ID types in the system
type ID interface {
	// Raw returns the raw UUID
	Raw() uuid.UUID
	// String returns the full string representation including prefix
	String() string
	// UUID extracts and returns the UUID part of the ID
	UUID() (uuid.UUID, error)
	// Prefix returns the prefix part of the ID (without the trailing underscore)
	Prefix() string
}

// PrefixedID is the base type for all prefixed ID implementations
type PrefixedID struct {
	raw    uuid.UUID // the raw UUID
	value  string    // the full ID string along with the prefix
	prefix string    // the prefix part of the ID (without the trailing underscore)
}

// FromString creates an ID from a string and a prefix
func FromUUID(prefix string, value uuid.UUID) (ID, error) {

	normalizedPrefix := prefix
	if !strings.HasSuffix(normalizedPrefix, "_") {
		normalizedPrefix = normalizedPrefix + "_"
	}

	return &PrefixedID{
		raw:    value,
		value:  normalizedPrefix + value.String(),
		prefix: strings.TrimSuffix(normalizedPrefix, "_"),
	}, nil
}

// ParseID parses any string ID with a prefix into a proper ID type
func ParseID(value string) (ID, error) {
	parts := strings.SplitN(value, "_", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidFormat
	}

	prefix := parts[0]
	idPart := parts[1]

	// Validate UUID format
	raw, err := uuid.Parse(idPart)
	if err != nil {
		return nil, ErrInvalidUUID
	}

	return &PrefixedID{
		raw:    raw,
		value:  value,
		prefix: prefix,
	}, nil
}

// ParsePrefixedID parses a string into a prefixed ID with expected prefix
func ParsePrefixedID(value string, expectedPrefix string) (ID, error) {
	if !strings.HasSuffix(expectedPrefix, "_") {
		expectedPrefix = expectedPrefix + "_"
	}

	if !strings.HasPrefix(value, expectedPrefix) {
		return nil, ErrInvalidPrefix
	}

	idPart := strings.TrimPrefix(value, expectedPrefix)
	_, err := uuid.Parse(idPart)
	if err != nil {
		return nil, ErrInvalidUUID
	}

	return &PrefixedID{
		value:  value,
		prefix: strings.TrimSuffix(expectedPrefix, "_"),
	}, nil
}

func (i *PrefixedID) Raw() uuid.UUID {
	return i.raw
}

// String returns the string representation
func (i *PrefixedID) String() string {
	return i.value
}

// UUID extracts and returns the UUID part from the ID
func (i *PrefixedID) UUID() (uuid.UUID, error) {
	idPart := strings.TrimPrefix(i.value, i.prefix+"_")
	return uuid.Parse(idPart)
}

// Prefix returns the prefix part of the ID
func (i *PrefixedID) Prefix() string {
	return i.prefix
}

// Common errors
var (
	ErrInvalidPrefix = errors.New("invalid prefix")
	ErrInvalidUUID   = errors.New("invalid uuid")
	ErrInvalidFormat = errors.New("invalid ID format, expected prefix_uuid")
)

// Add more types as needed

// // Example usage with database-generated UUIDs
// func Example() {
// 	// When retrieving a UUID from database
// 	dbUUID := "550e8400-e29b-41d4-a716-446655440000" // From database

// 	// Create a typed ID from the database UUID
// 	apiKeyID, _ := ApiKeyIDFromUUID(uuid.MustParse(dbUUID))

// 	// Get the prefixed string representation for client
// 	_ = apiKeyID.String() // "apikey_550e8400-e29b-41d4-a716-446655440000"

// 	// When receiving a prefixed ID from client, parse it
// 	parsedID, _ := ParseApiKeyID("apikey_550e8400-e29b-41d4-a716-446655440000")

// 	// Extract the UUID for database operations
// 	_, _ = parsedID.UUID() // Original UUID for use with database
// }
