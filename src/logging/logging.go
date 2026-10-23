// Package logging sets up structured logging with log/slog and carries a
// request-scoped logger (tagged with the request ID) through context.Context.
package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestIDHeader is read from incoming requests and always set on responses.
const RequestIDHeader = "X-Request-ID"

// incoming request IDs are reused only if they look like an ID, so clients
// can't inject arbitrary text into the logs
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type loggerKey struct{}

// New returns a logger for the given gin mode: JSON lines in release mode, for
// log collectors, and human-readable text in debug and test mode.
func New(mode string, w io.Writer) *slog.Logger {
	if mode == gin.ReleaseMode {
		return slog.New(slog.NewJSONHandler(w, nil))
	}
	return slog.New(slog.NewTextHandler(w, nil))
}

// WithLogger returns a copy of ctx that carries logger.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext returns the logger stored in ctx by Middleware, or slog.Default()
// if there is none (e.g. in tests or background work).
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// Middleware gives every request an ID, stores a logger tagged with it in the
// request context, and logs one line per request when it completes.
func Middleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		requestID := c.GetHeader(RequestIDHeader)
		if !validRequestID.MatchString(requestID) {
			requestID = newRequestID()
		}
		c.Header(RequestIDHeader, requestID)

		requestLogger := logger.With("request_id", requestID)
		c.Request = c.Request.WithContext(WithLogger(c.Request.Context(), requestLogger))

		c.Next()

		status := c.Writer.Status()
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		}
		requestLogger.LogAttrs(c.Request.Context(), level, "request",
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("route", c.FullPath()),
			slog.Int("status", status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("client_ip", c.ClientIP()),
			slog.Int("bytes", max(c.Writer.Size(), 0)), // Size is -1 when no body was written
		)
	}
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error
	return hex.EncodeToString(b)
}
