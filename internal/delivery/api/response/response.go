package response

import (
	"net/http"

	domainerrors "radar/internal/domain/errors"
	"radar/internal/platform/observability"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
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

const ErrorLogContextKey = "response_error_for_log"

type ErrorLogInfo struct {
	StatusCode int
	Code       string
	Message    string
	Details    any
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
			RequestID: requestID(c),
		},
	})
}

// Error returns an error response
func Error(c echo.Context, statusCode int, errorCode string, message string, details any) error {
	// Details should not be included for 5xx errors or authentication/authorization errors
	if statusCode >= 500 || statusCode == 401 || statusCode == 403 {
		details = nil
	}

	c.Set(ErrorLogContextKey, ErrorLogInfo{
		StatusCode: statusCode,
		Code:       errorCode,
		Message:    message,
		Details:    details,
	})

	return c.JSON(statusCode, ErrorResponse{
		Error: &ErrorInfo{
			Code:    errorCode,
			Message: message,
			Details: details,
		},
		Meta: &MetaInfo{
			RequestID: requestID(c),
		},
	})
}

func requestID(c echo.Context) string {
	if id := observability.CorrelationIDFromContext(c.Request().Context()); id != "" {
		return id
	}

	return uuid.New().String()
}

func appErrorDetails(appErr domainerrors.AppError) any {
	details := appErr.Details()
	if details == "" {
		return nil
	}

	return details
}

// AppError renders a canonical AppError directly through its HTTP metadata.
func AppError(c echo.Context, appErr domainerrors.AppError) error {
	return Error(c, appErr.HTTPCode(), appErr.ErrorCode(), appErr.Message(), appErrorDetails(appErr))
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

// AuthRequired returns the canonical 401 error for missing authentication.
func AuthRequired(c echo.Context) error {
	return AppError(c, domainerrors.ErrUnauthorized)
}

// InvalidToken returns the canonical 401 error for invalid authentication tokens.
func InvalidToken(c echo.Context) error {
	return AppError(c, domainerrors.ErrInvalidToken)
}

// Forbidden returns a 403 error
func Forbidden(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusForbidden, errorCode, message, nil)
}

// ForbiddenAccess returns the canonical 403 error for failed authorization.
func ForbiddenAccess(c echo.Context) error {
	return AppError(c, domainerrors.ErrForbidden)
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
