package middleware

import (
	"log/slog"
	"net/http"

	domainerrors "radar/internal/domain/errors"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// ErrorMiddleware error handling middleware
type ErrorMiddleware struct {
	logger *slog.Logger
}

// NewErrorMiddleware creates a new error handling middleware
func NewErrorMiddleware(logger *slog.Logger) *ErrorMiddleware {
	return &ErrorMiddleware{
		logger: logger,
	}
}

// HandleErrors error handling middleware function
func (m *ErrorMiddleware) HandleErrors(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Execute next handler
		err := next(c)
		if err == nil {
			return nil
		}

		// Handle error
		return m.handleError(c, err)
	}
}

// handleError handles various types of errors
func (m *ErrorMiddleware) handleError(c echo.Context, err error) error {
	// Try to parse as AppError
	var appErr domainerrors.AppError
	if errors.As(err, &appErr) {
		// Use AppError information
		return c.JSON(appErr.HTTPCode(), domainerrors.Response{
			Success: false,
			Code:    appErr.HTTPCode(),
			Message: appErr.Message(),
			Error: &domainerrors.ErrorInfo{
				Code:    appErr.ErrorCode(),
				Details: appErr.Details(),
			},
		})
	}

	// Check if it's Echo's HTTPError
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		return c.JSON(httpErr.Code, domainerrors.Response{
			Success: false,
			Code:    httpErr.Code,
			Message: httpErr.Message.(string),
			Error: &domainerrors.ErrorInfo{
				Code:    "HTTP_ERROR",
				Details: httpErr.Message.(string),
			},
		})
	}

	// Default to internal error, log error and return generic error
	m.logger.Error("Unhandled error",
		"error", err.Error(),
		"path", c.Request().URL.Path,
		"method", c.Request().Method,
	)

	return c.JSON(http.StatusInternalServerError, domainerrors.Response{
		Success: false,
		Code:    http.StatusInternalServerError,
		Message: "Internal server error",
		Error: &domainerrors.ErrorInfo{
			Code:    "INTERNAL_ERROR",
			Details: err.Error(), // Send error message directly to frontend for debugging
		},
	})
}
