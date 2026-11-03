package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"radar/internal/delivery"
	"radar/internal/delivery/api/response"
	domainerrors "radar/internal/domain/errors"

	"github.com/labstack/echo/v4"
	"github.com/slighter12/go-lib/errors/stack"
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
	if errors.Is(err, delivery.ErrResponseHandled) {
		return
	}
	// Record the source error even when the response is already committed so
	// the request logger still reports the failure.
	setSourceErrorLog(c, err)
	if c.Response().Committed {
		return
	}

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

// RecoverPanics converts a panic in an inner middleware or handler into an
// INTERNAL_ERROR carrying the panic value's type and stack, never its content. Register it inside HandleErrors so the panic
// becomes the canonical 500 response before the request logger records it.
func (m *ErrorMiddleware) RecoverPanics(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			recoveredErr, isErr := recovered.(error)
			if isErr && errors.Is(recoveredErr, http.ErrAbortHandler) {
				panic(recovered)
			}

			// Record only the panic value's type: its content may carry secrets
			// or PII that no log sanitizer can reliably detect.
			panicErr := fmt.Errorf("recovered panic (%T)", recovered)
			// Report every panic as INTERNAL_ERROR while keeping the panic
			// stack in the error tree for the source error log.
			err = stack.Replace(stack.With(panicErr), domainerrors.ErrInternalError)
		}()

		return next(c)
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
