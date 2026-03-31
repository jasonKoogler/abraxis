package decorator

import (
	"context"
	"fmt"

	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
	"go.uber.org/zap/zapcore"
)

type commandLoggingDecorator[C any] struct {
	base   CommandHandler[C]
	logger *log.Logger
}

func (d commandLoggingDecorator[C]) Handle(ctx context.Context, cmd C) (err error) {
	handlerType := generateActionName(cmd)

	logger := d.logger.With(zapcore.Field{
		Key:    "command",
		Type:   zapcore.StringType,
		String: handlerType,
	}, zapcore.Field{
		Key:    "command_data",
		Type:   zapcore.StringType,
		String: fmt.Sprintf("%#v", cmd),
	})

	logger.Debug("Executing command")
	defer func() {
		if err == nil {
			logger.Info("Command executed successfully")
		} else {
			logger.Error("Command failed", zapcore.Field{
				Key:    "error",
				Type:   zapcore.StringType,
				String: err.Error(),
			})
		}
	}()

	return d.base.Handle(ctx, cmd)
}

type queryLoggingDecorator[C any, R any] struct {
	base   QueryHandler[C, R]
	logger *log.Logger
}

func (d queryLoggingDecorator[C, R]) Handle(ctx context.Context, query C) (result R, err error) {
	handlerType := generateActionName(query)

	logger := d.logger.With(zapcore.Field{
		Key:    "query",
		Type:   zapcore.StringType,
		String: handlerType,
	}, zapcore.Field{
		Key:    "query_data",
		Type:   zapcore.StringType,
		String: fmt.Sprintf("%#v", query),
	})

	logger.Debug("Executing query")
	defer func() {
		if err == nil {
			logger.Info("Query executed successfully")
		} else {
			logger.Error("Query failed", zapcore.Field{
				Key:    "error",
				Type:   zapcore.StringType,
				String: err.Error(),
			})
		}
	}()

	return d.base.Handle(ctx, query)
}
