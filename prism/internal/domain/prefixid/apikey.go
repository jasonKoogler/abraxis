package prefixid

import (
	commonID "github.com/jasonKoogler/prism/internal/common/id"
	"github.com/jasonKoogler/prism/internal/common/uuid"
)

type ApiKeyID struct{ commonID.PrefixedID }

// ApiKeyIDFromUUID creates an ApiKeyID from a UUID
func ApiKeyIDFromUUID(uuid uuid.UUID) (commonID.ID, error) {
	id, err := commonID.FromUUID(ApiKeyPrefix, uuid)
	if err != nil {
		return nil, err
	}
	return &ApiKeyID{*id.(*commonID.PrefixedID)}, nil
}

// ParseApiKeyID parses a string into an ApiKeyID
func ParseApiKeyID(value string) (commonID.ID, error) {
	id, err := commonID.ParsePrefixedID(value, ApiKeyPrefix)
	if err != nil {
		return nil, err
	}
	return &ApiKeyID{*id.(*commonID.PrefixedID)}, nil
}
