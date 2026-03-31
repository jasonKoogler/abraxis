package prefixid

import (
	commonID "github.com/jasonKoogler/prism/internal/common/id"
	"github.com/jasonKoogler/prism/internal/common/uuid"
)

type ResourceID struct{ commonID.PrefixedID }

func ResourceIDFromUUID(uuid uuid.UUID) (commonID.ID, error) {
	id, err := commonID.FromUUID(ResourcePrefix, uuid)
	if err != nil {
		return nil, err
	}
	return &ResourceID{*id.(*commonID.PrefixedID)}, nil
}

func ParseResourceID(value string) (commonID.ID, error) {
	id, err := commonID.ParsePrefixedID(value, ResourcePrefix)
	if err != nil {
		return nil, err
	}
	return &ResourceID{*id.(*commonID.PrefixedID)}, nil
}
