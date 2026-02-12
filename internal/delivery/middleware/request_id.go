package middleware

import (
	"log/slog"

	deliverycontext "radar/internal/delivery/context"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

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
		requestID := c.Request().Header.Get(deliverycontext.HeaderXRequestID)

		// Generate a new Request ID if not provided by the client
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Store Request ID in echo.Context for response use
		deliverycontext.SetRequestID(c, requestID)

		// Add Request ID to response headers
		c.Response().Header().Set(deliverycontext.HeaderXRequestID, requestID)

		// Create a child logger with requestID
		reqLogger := m.logger.With(slog.String("request_id", requestID))

		// Store requestID and logger in context.Context for service layer use
		ctx := c.Request().Context()
		ctx = deliverycontext.WithRequestID(ctx, requestID)
		ctx = deliverycontext.WithLogger(ctx, reqLogger)
		c.SetRequest(c.Request().WithContext(ctx))

		return next(c)
	}
}
