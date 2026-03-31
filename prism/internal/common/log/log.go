package log

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// DebugLevel is the debug log level.
	DebugLevel = zapcore.DebugLevel

	// InfoLevel is the info log level.
	InfoLevel = zapcore.InfoLevel

	// WarnLevel is the warn log level.
	WarnLevel = zapcore.WarnLevel

	// ErrorLevel is the error log level.
	ErrorLevel = zapcore.ErrorLevel
)

type Logger struct {
	*zap.Logger
}

func NewLogger(level string) *Logger {
	lvl := zap.NewAtomicLevel()
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl.SetLevel(zap.InfoLevel)
	}

	var logger *zap.Logger
	localEnv := os.Getenv("LOCAL_ENV")

	if localEnv == "true" {
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		logger = zap.New(zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.Lock(os.Stdout),
			lvl,
		))
	} else {
		encoderConfig := zap.NewProductionEncoderConfig()
		logger = zap.New(zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.Lock(os.Stdout),
			lvl,
		))
	}

	return &Logger{logger}
}

func Any(key string, value interface{}) zapcore.Field {
	return zap.Any(key, value)
}

func (l *Logger) Print(args ...interface{}) {
	l.Logger.Sugar().Info(args...)
}

func (l *Logger) Printf(format string, args ...interface{}) {
	l.Logger.Sugar().Infof(format, args...)
}

func (l *Logger) Println(args ...interface{}) {
	l.Logger.Sugar().Info(args...)
}

type Fields = []zapcore.Field
type Field = zapcore.Field

func String(key string, val string) zapcore.Field {
	return zap.String(key, val)
}

func Int64(key string, val int64) zapcore.Field {
	return zap.Int64(key, val)
}

func Time(key string, val time.Time) zapcore.Field {
	return zap.Time(key, val)
}

func Error(err error) zapcore.Field {
	return zap.Error(err)
}

// Stack returns a field with the stack trace.
// Expensive and time consuming to generate.
func Stack(stack string) zapcore.Field {
	return zap.Stack(stack)
}
