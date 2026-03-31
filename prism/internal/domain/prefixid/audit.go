package prefixid

import (
	commonID "github.com/jasonKoogler/abraxis/prism/internal/common/id"
	"github.com/jasonKoogler/abraxis/prism/internal/common/uuid"
)

type AuditLogID struct{ commonID.PrefixedID }

func NewAuditLogID() (commonID.ID, error) {
	uid, err := uuid.New()
	if err != nil {
		return nil, err
	}
	return AuditLogIDFromUUID(uid)
}

func AuditLogIDFromUUID(uuid uuid.UUID) (commonID.ID, error) {
	id, err := commonID.FromUUID(AuditLogPrefix, uuid)
	if err != nil {
		return nil, err
	}
	return &AuditLogID{*id.(*commonID.PrefixedID)}, nil
}

func ParseAuditLogID(value string) (commonID.ID, error) {
	id, err := commonID.ParsePrefixedID(value, AuditLogPrefix)
	if err != nil {
		return nil, err
	}
	return &AuditLogID{*id.(*commonID.PrefixedID)}, nil
}
