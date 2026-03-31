package prefixid

import (
	commonID "github.com/jasonKoogler/prism/internal/common/id"
	"github.com/jasonKoogler/prism/internal/common/uuid"
)

type TenantID struct{ commonID.PrefixedID }

func TenantIDFromUUID(uuid uuid.UUID) (commonID.ID, error) {
	id, err := commonID.FromUUID(TenantPrefix, uuid)
	if err != nil {
		return nil, err
	}
	return &TenantID{*id.(*commonID.PrefixedID)}, nil
}

func ParseTenantID(value string) (commonID.ID, error) {
	id, err := commonID.ParsePrefixedID(value, TenantPrefix)
	if err != nil {
		return nil, err
	}
	return &TenantID{*id.(*commonID.PrefixedID)}, nil
}
