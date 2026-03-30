package middleware

import (
	"errors"
	"log/slog"

	"radar/internal/delivery"
	"radar/internal/delivery/api/response"
	deliverycontext "radar/internal/delivery/context"
	domainerrors "radar/internal/domain/errors"

	"github.com/labstack/echo/v4"
)

// ErrorMiddleware handles errors in the HTTP pipeline
type ErrorMiddleware struct {
	logger *slog.Logger
}

// NewErrorMiddleware creates a new error handling middleware
func NewErrorMiddleware(logger *slog.Logger) *ErrorMiddleware {
	return &ErrorMiddleware{
		logger: logger,
	}
}

// HandleHTTPError handles errors as Echo's HTTPErrorHandler
func (m *ErrorMiddleware) HandleHTTPError(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	if errors.Is(err, delivery.ErrResponseHandled) {
		return
	}

	// Attempt to parse as AppError
	if appErr, ok := errors.AsType[domainerrors.AppError](err); ok {
		_ = response.AppError(c, appErr)

		return
	}

	// Check if it is an Echo HTTPError
	if httpErr, ok := errors.AsType[*echo.HTTPError](err); ok {
		message := "An error occurred"
		if msg, ok := httpErr.Message.(string); ok {
			message = msg
		}

		_ = response.Error(c, httpErr.Code, "HTTP_ERROR", message, nil)

		return
	}

	// Default to internal error, log the error but return a generic message (do not expose internal details)
	m.logger.Error("Unhandled error",
		slog.Any("error", err),
		slog.String("request_id", deliverycontext.GetRequestID(c)),
		slog.String("path", c.Request().URL.Path),
		slog.String("method", c.Request().Method),
	)

	// For 500 errors, do not expose internal error details to the client
	_ = response.InternalServerError(c, "INTERNAL_ERROR", "Internal server error, please try again later")
}
