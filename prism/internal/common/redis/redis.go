package redis

import (
	"context"
	"fmt"

	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	*redis.Client
}

func NewRedisClient(ctx context.Context, logger *log.Logger, cfg *config.RedisConfig) (*RedisClient, error) {
	rcl := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		Username: cfg.Username,
		DB:       0,
	})

	_, err := rcl.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	logger.Info("redis connected successfully")
	// logger.Info("redis ping", log.String("result", str))

	return &RedisClient{rcl}, err
}
