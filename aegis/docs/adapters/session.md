# Session Adapter

## Overview

The Session adapter provides robust session management capabilities for user authentication and authorization. It enables the application to create, retrieve, update, and invalidate user sessions across multiple devices and login sources. The adapter supports both Redis-backed persistent sessions for production use and in-memory sessions for development and testing.

The adapter is critical for maintaining user authentication state, tracking user activity, enforcing security policies, and enabling features like session revocation and concurrent session management.

## Port Interface

The adapter implements the `SessionManager` interface defined in the `ports` package:

```go
type SessionManager interface {
    // CreateSession creates a new session for the user
    CreateSession(ctx context.Context, userID string, params *domain.SessionCreateParams) (string, error)

    // GetSession retrieves the session data
    GetSession(ctx context.Context, sessionID, userID string) (*domain.SessionData, error)

    // UpdateSession updates the session data
    UpdateSession(ctx context.Context, userID, sessionKey string, params *domain.SessionUpdateParams) error

    // ListSessionsByUser lists all sessions for a user
    ListSessionsByUser(ctx context.Context, userID string, cursor uint64, count int64) ([]map[string]domain.SessionData, uint64, error)

    // InvalidateUserSession invalidates a user's session
    InvalidateUserSession(ctx context.Context, userID, sessionID string) error

    // CleanupExpiredSessions cleans up expired sessions
    CleanupExpiredSessions(ctx context.Context) error

    // RevokeAllSessions revokes all sessions for a user
    RevokeAllSessions(ctx context.Context, userID string) error

    // Close closes the session manager
    Close() error
}
```

## Key Components

The Session adapter provides two implementations of the `SessionManager` interface:

### RedisSessionManager

A production-ready session manager that stores session data in Redis:

```go
type RedisSessionManager struct {
    mu                     sync.Mutex
    redisClient            *redis.RedisClient
    logger                 *log.Logger
    refreshTokenExpiration time.Duration
}
```

Features:

- Persistent session storage across application restarts
- Atomic session operations
- Automatic session expiration
- Support for concurrent sessions
- High performance with Redis
- Distributed session management for clustered deployments

### MemorySessionManager

A lightweight session manager that stores session data in memory:

```go
type MemorySessionManager struct {
    mu     sync.Mutex
    logger *log.Logger
}
```

Features:

- Simple in-memory storage for development and testing
- No external dependencies
- Fast local operations
- Suitable for single-instance deployments

## Session Data Model

Sessions are modeled using the `SessionData` structure:

```go
type SessionData struct {
    UserID            string      // Associated user ID
    SessionID         string      // Unique session identifier
    IP                string      // User's IP address
    UserAgent         string      // User's browser/client
    DeviceFingerprint string      // Device identifier
    Location          string      // Geographic location
    CreatedAt         time.Time   // When the session was created
    LastLogin         time.Time   // Last activity timestamp
    Roles             RoleMap     // User's roles
    AuthMethod        AuthMethod  // Password or OAuth
    Provider          string      // OAuth provider if applicable
    OAuthToken        *OAuthToken // OAuth tokens if applicable
    Revoked           bool        // Whether session is revoked
}
```

And are created using the `SessionCreateParams` structure:

```go
type SessionCreateParams struct {
    UserID        string
    Roles         RoleMap
    SessionParams *SessionMetaDataParams
    AuthMethod    AuthMethod
    Provider      string
    OAuthToken    *OAuthToken
}
```

## Implementation Details

### Session Keys

Sessions are identified using a composite key structure:

```
session:{userID}:{sessionID}
```

This pattern allows:

- Efficient retrieval of a specific session
- Easy listing of all sessions for a user
- Simple revocation of all sessions for a user

### Redis Implementation

The Redis implementation:

1. **Uses JSON serialization** for session data
2. **Applies TTL (Time-To-Live)** for automatic session expiration
3. **Employs Redis transactions** for atomic operations
4. **Implements batch processing** for efficient cleanup
5. **Provides cursor-based pagination** for listing sessions

### Security Considerations

The session adapter incorporates several security features:

- **Session Metadata Tracking** - IP, user agent, device fingerprint, and location
- **Authentication Method Tracking** - Password vs. OAuth provider
- **Last Login Tracking** - For detecting suspicious activity
- **Session Revocation** - For immediate security response
- **Automatic Expiration** - To limit session lifetime

## Usage Examples

### Creating a Redis Session Manager

```go
// Create a Redis client
redisClient, err := redis.NewRedisClient(ctx, logger, &config.RedisConfig{
    Host:     "localhost",
    Port:     "6379",
    Password: "",
})
if err != nil {
    log.Fatalf("Failed to create Redis client: %v", err)
}

// Create a session manager
sessionManager := session.NewRedisSessionManager(redisClient, logger)
```

### Creating a Memory Session Manager

```go
// Create a memory-based session manager for development
sessionManager := session.NewMemorySessionManager(logger)
```

### Creating a User Session

```go
// Prepare session parameters
sessionParams := &domain.SessionMetaDataParams{
    IP:                r.RemoteAddr,
    UserAgent:         r.UserAgent(),
    DeviceFingerprint: r.Header.Get("X-Device-Fingerprint"),
    Location:          r.Header.Get("X-Location"),
}

// Create session create parameters
createParams := &domain.SessionCreateParams{
    UserID:        user.ID.String(),
    Roles:         user.Roles,
    SessionParams: sessionParams,
    AuthMethod:    domain.AuthMethodPassword,
}

// Create the session
sessionID, err := sessionManager.CreateSession(ctx, user.ID.String(), createParams)
if err != nil {
    return fmt.Errorf("failed to create session: %w", err)
}

// Use the session ID (typically stored in a token)
fmt.Printf("Created session: %s\n", sessionID)
```

### Retrieving Session Data

```go
// Get session data using user ID and session ID
sessionData, err := sessionManager.GetSession(ctx, sessionID, userID)
if err != nil {
    if errors.Is(err, domain.ErrSessionNotFound) {
        return fmt.Errorf("session expired or invalid")
    }
    return fmt.Errorf("failed to get session: %w", err)
}

// Use session data
fmt.Printf("User logged in from %s using %s\n",
    sessionData.IP,
    sessionData.UserAgent)
```

### Listing User Sessions

```go
// List all active sessions for a user
sessions, nextCursor, err := sessionManager.ListSessionsByUser(ctx, userID, 0, 10)
if err != nil {
    return fmt.Errorf("failed to list sessions: %w", err)
}

// Display session information
for _, sessionMap := range sessions {
    for sessionID, data := range sessionMap {
        fmt.Printf("Session %s: %s on %s\n",
            sessionID,
            data.UserAgent,
            data.LastLogin.Format(time.RFC3339))
    }
}

// If there are more sessions, paginate with the cursor
if nextCursor != 0 {
    moreSessions, _, err := sessionManager.ListSessionsByUser(ctx, userID, nextCursor, 10)
    // Process more sessions...
}
```

### Invalidating a Session (Logout)

```go
// Invalidate a specific session
err := sessionManager.InvalidateUserSession(ctx, userID, sessionID)
if err != nil {
    return fmt.Errorf("failed to invalidate session: %w", err)
}
```

### Security Action: Revoking All User Sessions

```go
// Security action: revoke all sessions for a user
err := sessionManager.RevokeAllSessions(ctx, userID)
if err != nil {
    return fmt.Errorf("failed to revoke all sessions: %w", err)
}
```

### Maintenance: Cleaning Up Expired Sessions

```go
// Schedule periodic cleanup of expired sessions
ticker := time.NewTicker(24 * time.Hour)
go func() {
    for {
        select {
        case <-ticker.C:
            err := sessionManager.CleanupExpiredSessions(context.Background())
            if err != nil {
                log.Errorf("Failed to clean up expired sessions: %v", err)
            }
        case <-ctx.Done():
            ticker.Stop()
            return
        }
    }
}()
```

## Integration with Auth Manager

The Session adapter integrates with the Auth Manager for complete authentication flow:

```go
// In Auth Manager implementation
func (am *AuthManager) LoginWithPassword(ctx context.Context, email, password string,
    sessionParams *domain.SessionMetaDataParams) (*domain.AuthResponse, error) {

    // Authenticate user
    user, err := am.userService.GetByEmail(ctx, email)
    if err != nil {
        return nil, err
    }

    // Verify password
    if !passwordhasher.VerifyPassword(password, user.PasswordHash) {
        return nil, domain.ErrInvalidCredentials
    }

    // Create session
    createParams := &domain.SessionCreateParams{
        UserID:        user.ID.String(),
        Roles:         user.Roles,
        SessionParams: sessionParams,
        AuthMethod:    domain.AuthMethodPassword,
    }

    sessionID, err := am.sessionManager.CreateSession(ctx, user.ID.String(), createParams)
    if err != nil {
        return nil, err
    }

    // Generate tokens
    tokenPair, err := am.tokenManager.GenerateTokenPair(
        user.ID.String(),
        sessionID,
        user.Provider,
        user.Roles,
        nil, // No OAuth token for password login
    )

    return &domain.AuthResponse{
        TokenPair: tokenPair,
        SessionID: sessionID,
    }, nil
}
```

## Performance Considerations

### Redis Implementation

- **Connection Pooling**: Uses Redis connection pooling for efficient resource usage
- **Pipelining**: Batches Redis commands when possible
- **JSON Serialization**: Uses efficient JSON serialization for data storage
- **Cursor-based Pagination**: Implements cursor-based scanning for listing sessions
- **Background Cleanup**: Runs cleanup operations in the background

### Memory Implementation

- **Thread Safety**: Uses mutex for thread-safe operations
- **Low Overhead**: Minimal overhead for fast local operations
- **No External Dependencies**: Eliminates network latency

## Operational Considerations

When operating the Session adapter in production:

1. **Redis Persistence**: Configure Redis with appropriate persistence options
2. **Redis Clustering**: Consider Redis clustering for high availability
3. **Monitoring**: Monitor Redis performance metrics
4. **Session Cleanup**: Schedule regular cleanup of expired sessions
5. **TTL Configuration**: Configure appropriate TTL for sessions based on security requirements
