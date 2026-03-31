package verifier

import (
	"context"

	"github.com/jasonKoogler/abraxis/aegis/internal/adapters/oauth"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/redis"
)

type RedisVerifierStorage struct {
	redisClient *redis.RedisClient
}

var _ oauth.VerifierStorage = &RedisVerifierStorage{}

func NewRedisVerifierStorage(redisClient *redis.RedisClient) *RedisVerifierStorage {
	return &RedisVerifierStorage{redisClient: redisClient}
}

func (s *RedisVerifierStorage) Set(ctx context.Context, state, verifier string) error {
	return s.redisClient.Set(ctx, state, verifier, 0).Err()
}

func (s *RedisVerifierStorage) Get(ctx context.Context, state string) (string, error) {
	return s.redisClient.Get(ctx, state).Result()
}

func (s *RedisVerifierStorage) Del(ctx context.Context, state string) error {
	return s.redisClient.Del(ctx, state).Err()
}
