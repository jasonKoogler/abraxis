package decorator

import (
	"context"

	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
)

func ApplyQueryDecorator[H any, R any](handler QueryHandler[H, R], logger *log.Logger, metricsClient MetricsClient) QueryHandler[H, R] {
	return queryLoggingDecorator[H, R]{
		base: queryMetricsDecorator[H, R]{
			base:   handler,
			client: metricsClient,
		},
		logger: logger,
	}
}

type QueryHandler[Q any, R any] interface {
	Handle(ctx context.Context, q Q) (R, error)
}
