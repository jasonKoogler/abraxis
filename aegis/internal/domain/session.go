package domain

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SessionParams is the data used to create a new session
// Please use SessionParamsFromHTTP to create a new SessionParams from an HTTP request
type SessionMetaDataParams struct {
	IP                string
	UserAgent         string
	DeviceFingerprint string
	Location          string
	AuthMethod        AuthMethod
	Provider          string
	OAuthToken        *OAuthToken
}

func SessionMetaDataParamsFromHTTP(r *http.Request) *SessionMetaDataParams {
	return &SessionMetaDataParams{
		IP:                r.RemoteAddr,
		UserAgent:         r.UserAgent(),
		DeviceFingerprint: r.Header.Get(HeaderDeviceFingerprint),
		Location:          r.Header.Get(HeaderLocation),
		// todo: might need to grab authmethod, provider, and oauthToken from the request
	}
}

// SessionData is the data stored in Redis for a session.
// it is used to store the session data in Redis and to generate the session key
// New sessions should use the SessionMetaDataParams to store the data in Redis
type SessionData struct {
	UserID            string // Store only the UserID instead of the entire User object
	SessionID         string
	IP                string
	UserAgent         string
	DeviceFingerprint string
	Location          string
	CreatedAt         time.Time
	LastLogin         time.Time
	Roles             RoleMap
	AuthMethod        AuthMethod
	Provider          string
	OAuthToken        *OAuthToken
	Revoked           bool
}

type AuthMethod string

const (
	AuthMethodPassword AuthMethod = "password"
	AuthMethodOAuth    AuthMethod = "oauth"
)

func MakeSessionKey(userID, sessionID string) string {
	return fmt.Sprintf("session:%s:%s", userID, sessionID)
}

// UserIDFromSessionKey extracts the userID from the session key
func UserIDFromSessionKey(sessionKey string) (string, error) {
	parts := strings.Split(sessionKey, ":")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid session payload format")
	}
	return parts[1], nil
}

// SessionIDFromSessionKey extracts the sessionID from the session key
func SessionIDFromSessionKey(sessionKey string) (string, error) {
	parts := strings.Split(sessionKey, ":")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid session payload format")
	}
	return parts[2], nil
}

// UserAndSessionIDFromSessionKey extracts the userID and sessionID from the session key
func UserAndSessionIDFromSessionKey(sessionKey string) (string, string, error) {
	parts := strings.Split(sessionKey, ":")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("invalid session payload format")
	}
	return parts[1], parts[2], nil
}

type SessionCreateParams struct {
	UserID        string
	Roles         RoleMap
	SessionParams *SessionMetaDataParams
	AuthMethod    AuthMethod
	Provider      string
	OAuthToken    *OAuthToken
}

type SessionUpdateParams struct {
	UserID        string
	Roles         RoleMap
	SessionParams *SessionMetaDataParams
	AuthMethod    AuthMethod
	Provider      string
	OAuthToken    *OAuthToken
}
