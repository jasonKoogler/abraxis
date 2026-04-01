package uuid

import (
	googleuuid "github.com/google/uuid"
)

// UUID wraps google/uuid.UUID for use in the prefixed ID system.
type UUID struct {
	googleuuid.UUID
}

// New creates a new random UUID.
func New() (UUID, error) {
	id := googleuuid.New()
	return UUID{id}, nil
}

// Parse parses a standard UUID string.
func Parse(value string) (UUID, error) {
	id, err := googleuuid.Parse(value)
	if err != nil {
		return UUID{}, err
	}
	return UUID{id}, nil
}

// Nil returns the nil UUID.
func Nil() UUID {
	return UUID{googleuuid.Nil}
}
