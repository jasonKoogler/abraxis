package ports

import (
	"context"

	"github.com/jasonKoogler/aegis/internal/domain"
)

type SessionManager interface {
	// CreateSession creates a new session for the user and stores session data in Redis.
	// it returns the sessionID and error
	CreateSession(ctx context.Context, userID string, params *domain.SessionCreateParams) (string, error)

	// GetSession retrieves the session data from Redis
	GetSession(ctx context.Context, sessionID, userID string) (*domain.SessionData, error)

	// GetUserBySessionID retrieves a user by their session ID.
	// GetUserBySessionID(ctx context.Context, sessionID string) (*domain.User, error)

	// UpdateSession updates the session data in Redis
	UpdateSession(ctx context.Context, userID, sessionKey string, params *domain.SessionUpdateParams) error

	// ListSessionsByUser lists all sessions for a user.
	ListSessionsByUser(ctx context.Context, userID string, cursor uint64, count int64) ([]map[string]domain.SessionData, uint64, error)

	// InvalidateUserSession invalidates a user's session.
	InvalidateUserSession(ctx context.Context, userID, sessionID string) error

	// CleanupExpiredSessions cleans up expired sessions from Redis
	CleanupExpiredSessions(ctx context.Context) error

	// RevokeAllSessions revokes all sessions for a user.
	RevokeAllSessions(ctx context.Context, userID string) error

	// Close closes the session manager
	Close() error
}
