package aegis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	pb "github.com/jasonKoogler/abraxis/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/abraxis/prism/internal/common/log"
	goredis "github.com/redis/go-redis/v9"
)

const (
	syncVersionKey     = "aegis:sync:version"
	rolesKeyPrefix     = "aegis:roles:"
	revokedKeyPrefix   = "aegis:revoked:"
)

// cachedUserRoles is the JSON structure stored in Redis for each user's
// role/permission snapshot.
type cachedUserRoles struct {
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	TenantID    string   `json:"tenant_id"`
}

// syncAuthDataLoop reconnects to the SyncAuthData stream with
// exponential backoff whenever the stream ends or errors.
func (c *Client) syncAuthDataLoop(ctx context.Context) {
	attempt := 0
	for {
		err := c.runAuthSync(ctx)
		if ctx.Err() != nil {
			return
		}

		wait := c.backoff(attempt)
		c.logger.Error("auth data sync disconnected, reconnecting",
			log.Error(err),
			log.String("backoff", wait.String()),
		)

		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		attempt++
	}
}

// runAuthSync opens a single SyncAuthData stream and processes events
// until the stream ends or an error occurs.
func (c *Client) runAuthSync(ctx context.Context) error {
	// Retrieve last seen version from Redis so we can resume.
	lastVersion, err := c.redisClient.Get(ctx, syncVersionKey).Result()
	if err == goredis.Nil {
		lastVersion = ""
	} else if err != nil {
		c.logger.Error("failed to read sync version from redis", log.Error(err))
		lastVersion = ""
	}

	stream, err := c.client.SyncAuthData(ctx, &pb.SyncRequest{
		LastVersion: lastVersion,
	})
	if err != nil {
		return fmt.Errorf("opening auth data stream: %w", err)
	}

	c.logger.Info("auth data sync stream opened", log.String("last_version", lastVersion))

	for {
		event, err := stream.Recv()
		if err == io.EOF {
			return fmt.Errorf("auth data stream ended")
		}
		if err != nil {
			return fmt.Errorf("receiving auth data event: %w", err)
		}

		switch ev := event.GetEvent().(type) {
		case *pb.AuthDataEvent_UserRoles:
			if err := c.handleUserRoles(ctx, ev.UserRoles); err != nil {
				c.logger.Error("failed to cache user roles", log.Error(err))
			}

		case *pb.AuthDataEvent_TokenRevoked:
			if err := c.handleTokenRevoked(ctx, ev.TokenRevoked); err != nil {
				c.logger.Error("failed to cache token revocation", log.Error(err))
			}

		case *pb.AuthDataEvent_SyncComplete:
			c.ready.Store(true)
			c.logger.Info("aegis auth data initial sync complete")
		}

		// Persist the version watermark.
		if event.GetVersion() != "" {
			if err := c.redisClient.Set(ctx, syncVersionKey, event.GetVersion(), c.cacheTTL).Err(); err != nil {
				c.logger.Error("failed to persist sync version", log.Error(err))
			}
		}
	}
}

// handleUserRoles writes a user's roles and permissions snapshot to Redis.
func (c *Client) handleUserRoles(ctx context.Context, snap *pb.UserRolesSnapshot) error {
	data, err := json.Marshal(cachedUserRoles{
		Roles:       snap.GetRoles(),
		Permissions: snap.GetPermissions(),
		TenantID:    snap.GetTenantId(),
	})
	if err != nil {
		return fmt.Errorf("marshaling user roles: %w", err)
	}

	key := rolesKeyPrefix + snap.GetUserId()
	if err := c.redisClient.Set(ctx, key, data, c.cacheTTL).Err(); err != nil {
		return fmt.Errorf("writing user roles to redis: %w", err)
	}

	c.logger.Info("cached user roles",
		log.String("user_id", snap.GetUserId()),
		log.String("tenant_id", snap.GetTenantId()),
	)
	return nil
}

// handleTokenRevoked records a revoked token JTI in Redis. The key
// expires when the original token would have expired, so the
// revocation entry is automatically cleaned up.
func (c *Client) handleTokenRevoked(ctx context.Context, rev *pb.TokenRevoked) error {
	ttl := time.Until(time.Unix(rev.GetExpiresAt(), 0))
	if ttl <= 0 {
		// Token already expired; no need to track.
		return nil
	}

	key := revokedKeyPrefix + rev.GetJti()
	if err := c.redisClient.Set(ctx, key, "1", ttl).Err(); err != nil {
		return fmt.Errorf("writing revoked token to redis: %w", err)
	}

	c.logger.Info("cached token revocation", log.String("jti", rev.GetJti()))
	return nil
}

// IsTokenRevoked checks Redis to determine whether a token with the
// given JTI has been revoked.
func (c *Client) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	key := revokedKeyPrefix + jti
	_, err := c.redisClient.Get(ctx, key).Result()
	if err == goredis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking revoked token: %w", err)
	}
	return true, nil
}

// GetCachedRoles retrieves a user's cached roles and permissions from
// Redis. Returns nil if no cache entry exists.
func (c *Client) GetCachedRoles(ctx context.Context, userID string) (*cachedUserRoles, error) {
	key := rolesKeyPrefix + userID
	data, err := c.redisClient.Get(ctx, key).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading cached roles: %w", err)
	}

	var roles cachedUserRoles
	if err := json.Unmarshal([]byte(data), &roles); err != nil {
		return nil, fmt.Errorf("unmarshaling cached roles: %w", err)
	}
	return &roles, nil
}
