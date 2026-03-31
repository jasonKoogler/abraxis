package domain

import (
	"net"
	"time"

	"github.com/google/uuid"
)

// AuditLog represents a security audit log entry
type AuditLog struct {
	ID           uuid.UUID   `json:"id"`
	EventType    string      `json:"event_type"`
	ActorType    string      `json:"actor_type"`
	ActorID      uuid.UUID   `json:"actor_id,omitempty"`
	TenantID     uuid.UUID   `json:"tenant_id,omitempty"`
	ResourceType string      `json:"resource_type,omitempty"`
	ResourceID   uuid.UUID   `json:"resource_id,omitempty"`
	IPAddress    net.IP      `json:"ip_address,omitempty"`
	UserAgent    string      `json:"user_agent,omitempty"`
	EventData    interface{} `json:"event_data,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
}
