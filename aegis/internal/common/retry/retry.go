package retry

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
)

type Options struct {
	MaxRetries     uint64
	InitialDelay   time.Duration
	MaxDelay       time.Duration
	MaxElapsedTime time.Duration
}

func WithBackoff(ctx context.Context, operation func() error, opts Options) error {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = opts.InitialDelay
	b.MaxInterval = opts.MaxDelay
	b.MaxElapsedTime = opts.MaxElapsedTime

	return backoff.Retry(func() error {
		err := operation()
		if err != nil {
			select {
			case <-ctx.Done():
				return backoff.Permanent(ctx.Err())
			default:
				return err // Retry
			}
		}
		return nil // Success
	}, backoff.WithContext(backoff.WithMaxRetries(b, opts.MaxRetries), ctx))
}
