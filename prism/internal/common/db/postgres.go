package db

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
)

type PostgresPool struct {
	*pgxpool.Pool
}

func NewPostgresPool(ctx context.Context, cfg *config.PostgresConfig, logger *log.Logger) (*PostgresPool, error) {
	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Path:   cfg.DB,
	}

	q := dsn.Query()
	q.Add("sslmode", cfg.SSLMode)

	dsn.RawQuery = q.Encode()

	var err error

	for i := 0; i < 10; i++ {
		logger.Printf("Connecting to database, attempt: %d", i)
		pool, err := pgxpool.New(ctx, dsn.String())
		if err == nil {
			err = pool.Ping(ctx)
			if err == nil {
				logger.Println("Successfully connected to the database")
				return &PostgresPool{pool}, nil
			}
		}

		logger.Printf("failed to connect to db, (attempt: %d): %v", i, err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(i) * time.Second):
		}
	}

	return nil, fmt.Errorf("failed to connect to db after 10 attempts: %v", err)
}
