package middleware

import (
	"errors"
	"log/slog"

	"radar/internal/delivery"
	"radar/internal/delivery/api/response"
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
	setSourceErrorLog(c, err)

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

	// For 500 errors, do not expose internal error details to the client
	_ = response.InternalServerError(c, "INTERNAL_ERROR", "Internal server error, please try again later")
}

// HandleErrors converts returned Go errors into HTTP responses inside the
// middleware chain so the deferred request logger can record the final status.
func (m *ErrorMiddleware) HandleErrors(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		if err == nil {
			return nil
		}
		if errors.Is(err, delivery.ErrResponseHandled) {
			return nil
		}

		m.HandleHTTPError(err, c)

		return nil
	}
}
