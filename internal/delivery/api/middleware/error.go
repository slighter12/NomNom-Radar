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
		_ = response.AppError(c, httpErrorAppError(httpErr.Code))

		return
	}

	// For 500 errors, do not expose internal error details to the client
	_ = response.AppError(c, domainerrors.ErrInternalError)
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

func httpErrorAppError(statusCode int) domainerrors.AppError {
	switch statusCode {
	case 400:
		return domainerrors.ErrInvalidInput
	case 401:
		return domainerrors.ErrUnauthorized
	case 403:
		return domainerrors.ErrForbidden
	case 404:
		return domainerrors.ErrNotFound
	case 409:
		return domainerrors.ErrConflict
	default:
		if statusCode >= 500 {
			return domainerrors.ErrInternalError
		}
		if statusCode < 400 {
			return domainerrors.ErrInternalError
		}

		return domainerrors.NewBaseError(
			statusCode,
			domainerrors.ErrRequestFailed.ErrorCode(),
			domainerrors.ErrRequestFailed.Message(),
			"",
		)
	}
}
