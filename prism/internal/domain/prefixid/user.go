package prefixid

import (
	commonID "github.com/jasonKoogler/prism/internal/common/id"
	"github.com/jasonKoogler/prism/internal/common/uuid"
)

type UserID struct{ commonID.PrefixedID }

// UserIDFromUUID creates a UserID from a UUID
func UserIDFromUUID(uuid uuid.UUID) (commonID.ID, error) {
	id, err := commonID.FromUUID(UserPrefix, uuid)
	if err != nil {
		return nil, err
	}
	return &UserID{*id.(*commonID.PrefixedID)}, nil
}

// ParseUserID parses a string into a UserID
func ParseUserID(value string) (commonID.ID, error) {
	id, err := commonID.ParsePrefixedID(value, UserPrefix)
	if err != nil {
		return nil, err
	}
	return &UserID{*id.(*commonID.PrefixedID)}, nil
}
