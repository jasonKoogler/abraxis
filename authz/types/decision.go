package types

import (
	"time"
)

// Decision represents an authorization decision
type Decision struct {
	// Allowed indicates if access is allowed
	Allowed bool `json:"allowed"`

	// Reason provides a reason for the decision (if available)
	Reason string `json:"reason,omitempty"`

	// Cached indicates if this result was served from cache
	Cached bool `json:"cached,omitempty"`

	// Timestamp when the decision was made
	Timestamp time.Time `json:"timestamp,omitempty"`
}
