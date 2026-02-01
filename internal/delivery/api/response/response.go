package response

import (
	"net/http"

	deliverycontext "radar/internal/delivery/context"
	domainerrors "radar/internal/domain/errors"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// SuccessResponse defines the structure for successful responses
type SuccessResponse struct {
	Data any       `json:"data"`
	Meta *MetaInfo `json:"meta"`
}

// ErrorResponse defines the structure for error responses
type ErrorResponse struct {
	Error *ErrorInfo `json:"error"`
	Meta  *MetaInfo  `json:"meta"`
}

// ErrorInfo contains detailed error information
type ErrorInfo struct {
	Code    string `json:"code"`              // Machine-readable error code, e.g., "VALIDATION_FAILED"
	Message string `json:"message"`           // User-friendly error message
	Details any    `json:"details,omitempty"` // Additional error context (only for 4xx errors)
}

// MetaInfo represents response metadata
type MetaInfo struct {
	RequestID string `json:"request_id"` // Request tracking ID
}

// Success returns a successful response
func Success(c echo.Context, statusCode int, data any) error {
	return c.JSON(statusCode, SuccessResponse{
		Data: data,
		Meta: &MetaInfo{
			RequestID: deliverycontext.GetRequestID(c),
		},
	})
}

// Error returns an error response
func Error(c echo.Context, statusCode int, errorCode string, message string, details any) error {
	// Details should not be included for 5xx errors or authentication/authorization errors
	if statusCode >= 500 || statusCode == 401 || statusCode == 403 {
		details = nil
	}

	return c.JSON(statusCode, ErrorResponse{
		Error: &ErrorInfo{
			Code:    errorCode,
			Message: message,
			Details: details,
		},
		Meta: &MetaInfo{
			RequestID: deliverycontext.GetRequestID(c),
		},
	})
}

// BadRequest returns a 400 error
func BadRequest(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusBadRequest, errorCode, message, nil)
}

// BadRequestWithDetails returns a 400 error with details
func BadRequestWithDetails(c echo.Context, errorCode string, message string, details any) error {
	return Error(c, http.StatusBadRequest, errorCode, message, details)
}

// BindingError returns a binding error response
func BindingError(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusBadRequest, errorCode, message, nil)
}

// Unauthorized returns a 401 error
func Unauthorized(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusUnauthorized, errorCode, message, nil)
}

// Forbidden returns a 403 error
func Forbidden(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusForbidden, errorCode, message, nil)
}

// NotFound returns a 404 error
func NotFound(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusNotFound, errorCode, message, nil)
}

// Conflict returns a 409 error
func Conflict(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusConflict, errorCode, message, nil)
}

// InternalServerError returns a 500 error
func InternalServerError(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusInternalServerError, errorCode, message, nil)
}

// HandleAppError handles application errors, converting domain errors to appropriate HTTP responses
func HandleAppError(c echo.Context, err error) error {
	var appErr domainerrors.AppError
	if errors.As(err, &appErr) {
		return Error(c, appErr.HTTPCode(), appErr.ErrorCode(), appErr.Message(), nil)
	}

	return errors.WithStack(err)
}
