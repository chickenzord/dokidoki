package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

// HTTPLogger returns a Chi-compatible request logger middleware that routes
// logs through log/slog, logging heartbeat requests at debug level.
func HTTPLogger() func(next http.Handler) http.Handler {
	return chiMiddleware.RequestLogger(&httpLogFormatter{})
}

type httpLogFormatter struct{}

func (f *httpLogFormatter) NewLogEntry(r *http.Request) chiMiddleware.LogEntry {
	return &httpLogEntry{req: r}
}

type httpLogEntry struct {
	req *http.Request
}

func (e *httpLogEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, extra any) {
	path := e.req.URL.Path
	isHeartbeat := strings.HasSuffix(path, "/cluster/heartbeat")

	level := slog.LevelInfo
	if isHeartbeat {
		level = slog.LevelDebug
	} else if status >= 500 {
		level = slog.LevelError
	} else if status >= 400 {
		level = slog.LevelWarn
	}

	slog.Log(
		e.req.Context(),
		level,
		"HTTP request",
		"method", e.req.Method,
		"path", path,
		"status", status,
		"bytes", bytes,
		"duration", elapsed.Round(time.Microsecond),
	)
}

func (e *httpLogEntry) Panic(v any, stack []byte) {
	slog.Error("HTTP panic recovered", "panic", v, "stack", string(stack))
}
