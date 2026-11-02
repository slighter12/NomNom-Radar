package middleware

import (
	"log/slog"

	"radar/internal/platform/observability"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// maxRequestIDLength bounds client-supplied request IDs echoed into headers, bodies, and logs.
const maxRequestIDLength = 128

// RequestIDMiddleware generates or extracts a unique Request ID for each request and creates a request-scoped logger
type RequestIDMiddleware struct {
	logger *slog.Logger
}

// NewRequestIDMiddleware creates a new Request ID middleware
func NewRequestIDMiddleware(logger *slog.Logger) *RequestIDMiddleware {
	return &RequestIDMiddleware{
		logger: logger,
	}
}

// Process handles the generation or extraction of the Request ID and creates a logger with requestID
func (m *RequestIDMiddleware) Process(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Attempt to get Request ID from request headers
		requestID := c.Request().Header.Get(observability.HeaderXRequestID)

		// Generate a new Request ID when the client omits it or sends an unsafe value
		if !isAcceptableRequestID(requestID) {
			requestID = uuid.New().String()
		}

		// Add Request ID to response headers
		c.Response().Header().Set(observability.HeaderXRequestID, requestID)

		// Create a child logger with requestID
		reqLogger := m.logger.With(slog.String("request_id", requestID))

		// Store requestID and logger in context.Context for service layer use
		ctx := c.Request().Context()
		ctx = observability.WithCorrelationID(ctx, requestID)
		ctx = observability.WithLogger(ctx, reqLogger)
		c.SetRequest(c.Request().WithContext(ctx))

		return next(c)
	}
}

// isAcceptableRequestID reports whether a client-supplied request ID is
// 1-128 characters drawn only from ASCII letters, digits, '.', '_', and '-'.
func isAcceptableRequestID(requestID string) bool {
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
