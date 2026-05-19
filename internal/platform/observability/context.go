package observability

import (
	"context"
	"log/slog"
)

type contextKey string

const (
	correlationIDKey contextKey = "correlation_id"
	loggerKey        contextKey = "logger"

	// HeaderXRequestID is the HTTP header used to carry the correlation ID.
	HeaderXRequestID = "X-Request-Id"
)

// WithCorrelationID returns a context carrying the request correlation ID.
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// CorrelationIDFromContext extracts the request correlation ID from context.
func CorrelationIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}

	return ""
}

// WithLogger returns a context carrying the request-scoped logger.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// LoggerFromContext extracts the request-scoped logger from context.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}

	return nil
}

// LoggerFromContextOrDefault returns the request-scoped logger or fallback.
func LoggerFromContextOrDefault(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if logger := LoggerFromContext(ctx); logger != nil {
		return logger
	}

	return fallback
}
