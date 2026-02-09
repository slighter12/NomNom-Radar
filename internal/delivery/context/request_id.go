package context

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// ContextKey is a custom type for context keys to avoid collisions.
type ContextKey string

const (
	// KeyRequestID is the key for storing request ID in context.
	KeyRequestID ContextKey = "request_id"

	// KeyLogger is the key for storing request-scoped logger in context.
	KeyLogger ContextKey = "logger"

	// HeaderXRequestID is the HTTP header name for request ID.
	HeaderXRequestID = "X-Request-Id"
)

// GetRequestID extracts the request ID from echo.Context.
// If not found, generates a new UUID.
func GetRequestID(c echo.Context) string {
	val := c.Get(string(KeyRequestID))
	if id, ok := val.(string); ok && id != "" {
		return id
	}

	return uuid.New().String()
}

// SetRequestID sets the request ID in echo.Context.
func SetRequestID(c echo.Context, requestID string) {
	c.Set(string(KeyRequestID), requestID)
}

// GetRequestIDFromContext extracts the request ID from standard context.Context.
// If not found, returns empty string.
func GetRequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(KeyRequestID).(string); ok {
		return id
	}

	return ""
}

// WithRequestID returns a new context with the request ID.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, KeyRequestID, requestID)
}

// GetLogger extracts the request-scoped logger from context.Context.
// If not found, returns nil.
func GetLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(KeyLogger).(*slog.Logger); ok {
		return logger
	}

	return nil
}

// GetLoggerOrDefault extracts the request-scoped logger from context.Context.
// If not found, returns the provided fallback logger.
func GetLoggerOrDefault(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if logger := GetLogger(ctx); logger != nil {
		return logger
	}

	return fallback
}

// WithLogger returns a new context with the logger.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, KeyLogger, logger)
}
