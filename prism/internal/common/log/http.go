package log

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func NewHTTPLogger(logger *Logger) func(next http.Handler) http.Handler {
	return middleware.RequestLogger(logger)
}

func (l *Logger) NewLogEntry(r *http.Request) middleware.LogEntry {
	entry := &LogEntry{Logger: zap.New(l.Core())}

	logFields := []zap.Field{
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("proto", r.Proto),
		zap.String("method", r.Method),
		zap.String("host", r.Host),
		zap.String("request_uri", r.RequestURI),
		zap.String("user_agent", r.UserAgent()),
		zap.String("referer", r.Referer()),
	}

	entry.Logger = entry.Logger.With(logFields...)
	entry.Logger.Info("request started")

	return entry
}

type LogEntry Logger

func (l *LogEntry) Panic(v interface{}, stack []byte) {
	l.Logger.Panic(v.(string), zap.String("stack", string(stack)))
}

func (l *LogEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, extra interface{}) {
	l.Logger.Info("request complete",
		zap.Int("status", status),
		zap.Int("bytes", bytes),
		zap.Duration("elapsed", elapsed),
	)
}

func GetLogEntry(r *http.Request) LogEntry {
	entry := middleware.GetLogEntry(r).(*LogEntry)
	return *entry
}
