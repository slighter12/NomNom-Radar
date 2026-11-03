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

	// maxRequestIDLength bounds externally supplied request IDs echoed into headers, bodies, and logs.
	maxRequestIDLength = 128
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

// IsAcceptableRequestID reports whether an externally supplied request ID is
// 1-128 characters drawn only from ASCII letters, digits, '.', '_', and '-'.
func IsAcceptableRequestID(requestID string) bool {
	if requestID == "" || len(requestID) > maxRequestIDLength {
		return false
	}

	for i := 0; i < len(requestID); i++ {
		ch := requestID[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9', ch == '.', ch == '_', ch == '-':
		default:
			return false
		}
	}

	return true
}
