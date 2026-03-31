package session

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/jasonKoogler/aegis/internal/common/redis"
	"github.com/jasonKoogler/aegis/internal/domain"
	"github.com/jasonKoogler/aegis/internal/ports"
	goredis "github.com/redis/go-redis/v9"
)

type RedisSessionManager struct {
	mu                     sync.Mutex
	redisClient            *redis.RedisClient
	logger                 *log.Logger
	refreshTokenExpiration time.Duration
}

var _ ports.SessionManager = &RedisSessionManager{}

func NewRedisSessionManager(redisClient *redis.RedisClient, logger *log.Logger) *RedisSessionManager {
	return &RedisSessionManager{
		mu:          sync.Mutex{},
		redisClient: redisClient,
		logger:      logger,
	}
}

// CreateSession creates a new session for the user and stores session data in Redis.
// it returns the sessionID and error
func (r *RedisSessionManager) CreateSession(ctx context.Context, userID string, params *domain.SessionCreateParams) (string, error) {
	// Generate session ID and the session key
	sessionID := uuid.New().String()
	sessionKey := domain.MakeSessionKey(userID, sessionID)

	sessionData := &domain.SessionData{
		SessionID:         sessionKey,
		IP:                params.SessionParams.IP,
		UserAgent:         params.SessionParams.UserAgent,
		DeviceFingerprint: params.SessionParams.DeviceFingerprint,
		Location:          params.SessionParams.Location,
		CreatedAt:         time.Now(),
		LastLogin:         time.Now(),
		Roles:             params.Roles,
		AuthMethod:        params.AuthMethod,
		Provider:          params.Provider,
		OAuthToken:        params.OAuthToken,
	}

	sessionDataBytes, err := marshalSessionMetaData(sessionData)
	if err != nil {
		return "", err
	}

	// Store session data atomically
	err = r.redisClient.Watch(ctx, func(tx *goredis.Tx) error {
		pipe := tx.TxPipeline()
		pipe.Set(ctx, sessionKey, sessionDataBytes, r.refreshTokenExpiration)
		_, execErr := pipe.Exec(ctx)
		return execErr
	}, sessionKey)

	if err != nil {
		return "", domain.ErrSessionCreationFailed
	}

	return sessionID, nil
}

// GetSession retrieves the session data from Redis
func (r *RedisSessionManager) GetSession(ctx context.Context, sessionID, userID string) (*domain.SessionData, error) {
	sessionKey := domain.MakeSessionKey(userID, sessionID)
	sessionBytes, err := r.redisClient.Get(ctx, sessionKey).Bytes()
	if err != nil {
		return nil, err
	}

	if len(sessionBytes) == 0 {
		return nil, domain.ErrSessionNotFound
	}

	sessionData, err := unmarshalSessionMetaData(sessionBytes)
	if err != nil {
		return nil, err
	}

	return &domain.SessionData{
		SessionID:         sessionID,
		IP:                sessionData.IP,
		UserAgent:         sessionData.UserAgent,
		DeviceFingerprint: sessionData.DeviceFingerprint,
		Location:          sessionData.Location,
		CreatedAt:         sessionData.CreatedAt,
		LastLogin:         sessionData.LastLogin,
		Roles:             sessionData.Roles,
	}, nil
}

// UpdateSession updates the session data in Redis using the provided update parameters
func (r *RedisSessionManager) UpdateSession(ctx context.Context, userID, sessionID string, params *domain.SessionUpdateParams) error {
	sessionKey := domain.MakeSessionKey(userID, sessionID)

	// Get existing session data
	sessionData, err := r.getSessionData(ctx, sessionKey)
	if err != nil {
		return fmt.Errorf("failed to get session data: %w", err)
	}

	// Update session fields
	if params.SessionParams != nil {
		sessionData.IP = params.SessionParams.IP
		sessionData.UserAgent = params.SessionParams.UserAgent
		sessionData.DeviceFingerprint = params.SessionParams.DeviceFingerprint
		sessionData.Location = params.SessionParams.Location
	}

	if params.Roles != nil {
		sessionData.Roles = params.Roles
	}

	if params.AuthMethod != "" {
		sessionData.AuthMethod = params.AuthMethod
	}

	if params.Provider != "" {
		sessionData.Provider = params.Provider
	}

	if params.OAuthToken != nil {
		sessionData.OAuthToken = params.OAuthToken
	}

	sessionData.LastLogin = time.Now()

	// Marshal and store updated session data
	sessionDataBytes, err := marshalSessionMetaData(sessionData)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	return r.redisClient.Set(ctx, sessionKey, sessionDataBytes, r.refreshTokenExpiration).Err()
}

// getSessionData retrieves and unmarshals session data from Redis
func (r *RedisSessionManager) getSessionData(ctx context.Context, sessionKey string) (*domain.SessionData, error) {
	sessionDataBytes, err := r.redisClient.Get(ctx, sessionKey).Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get session data: %w", err)
	}

	sessionData, err := unmarshalSessionMetaData(sessionDataBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	return sessionData, nil
}

// ListSessionsByUser lists all sessions for a user.
// it returns a slice of session metadata and the next cursor or an error
// the key of the returned map is the sessionID for the users session
func (r *RedisSessionManager) ListSessionsByUser(ctx context.Context, userID string, cursor uint64, count int64) ([]map[string]domain.SessionData, uint64, error) {
	pattern := fmt.Sprintf("session:%s:*", userID)

	keys, nextCursor, err := r.redisClient.Scan(ctx, cursor, pattern, count).Result()
	if err != nil {
		return nil, 0, err
	}

	sessionMapSlice := make([]map[string]domain.SessionData, len(keys))
	for _, key := range keys {
		sessionBytes, err := r.redisClient.Get(ctx, key).Bytes()
		if err != nil {
			return nil, 0, err
		}

		sessionData, err := unmarshalSessionMetaData(sessionBytes)
		if err != nil {
			return nil, 0, err
		}

		sessionMap := make(map[string]domain.SessionData)
		sessionMap[key] = domain.SessionData{
			SessionID:         key,
			IP:                sessionData.IP,
			UserAgent:         sessionData.UserAgent,
			DeviceFingerprint: sessionData.DeviceFingerprint,
			Location:          sessionData.Location,
		}
		sessionMapSlice = append(sessionMapSlice, sessionMap)
	}

	return sessionMapSlice, nextCursor, nil
}

func (r *RedisSessionManager) InvalidateUserSession(ctx context.Context, userID, sessionID string) error {
	sessionKey := domain.MakeSessionKey(userID, sessionID)
	return r.redisClient.Del(ctx, sessionKey).Err()
}

func (r *RedisSessionManager) CleanupExpiredSessions(ctx context.Context) error {
	const batchSize = int64(1000)
	var cursor uint64

	for {
		keys, nextCursor, err := r.redisClient.Scan(ctx, cursor, "session:*", batchSize).Result()
		if err != nil {
			return fmt.Errorf("failed to scan sessions: %w", err)
		}

		if len(keys) > 0 {
			pipe := r.redisClient.Pipeline()
			for _, key := range keys {
				// Use context with timeout for each TTL check
				ttlCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
				ttlCmd := pipe.TTL(ttlCtx, key)
				ttl, err := ttlCmd.Result()
				cancel()
				if err != nil {
					r.logger.Warn("Failed to get TTL for session key",
						log.String("key", key),
						log.Error(err))
					continue
				}
				if ttl <= 0 {
					pipe.Del(ctx, key)
				}
			}

			// Execute pipeline with timeout
			execCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err = pipe.Exec(execCtx)
			cancel()
			if err != nil {
				r.logger.Error("Failed to execute session cleanup pipeline",
					log.Error(err))
				return fmt.Errorf("failed to execute pipeline: %w", err)
			}
		}

		if nextCursor == 0 {
			break
		}
		cursor = nextCursor
	}

	r.logger.Info("Session cleanup completed successfully")
	return nil
}

func (r *RedisSessionManager) RevokeAllSessions(ctx context.Context, userID string) error {
	pattern := fmt.Sprintf("session:%s:*", userID)
	return r.redisClient.Del(ctx, pattern).Err()
}

func (r *RedisSessionManager) Close() error {
	return r.redisClient.Close()
}

func marshalSessionMetaData(sessionData *domain.SessionData) ([]byte, error) {
	jsonData, err := json.Marshal(sessionData)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

func unmarshalSessionMetaData(data []byte) (*domain.SessionData, error) {
	var sessionData domain.SessionData
	if err := json.Unmarshal(data, &sessionData); err != nil {
		return nil, err
	}
	return &sessionData, nil
}
