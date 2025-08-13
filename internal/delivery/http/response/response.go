package response

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Response unified API response structure
type Response struct {
	Success bool       `json:"success"`
	Code    int        `json:"code"`    // HTTP status code
	Message string     `json:"message"` // User-friendly message
	Data    any        `json:"data,omitempty"`
	Error   *ErrorInfo `json:"error,omitempty"`
}

// ErrorInfo detailed error information
type ErrorInfo struct {
	Code    string `json:"code"`    // Business error code, e.g., "USER_NOT_FOUND"
	Details string `json:"details"` // Detailed error description
}

// Success successful response
func Success(c echo.Context, statusCode int, data any, message string) error {
	if message == "" {
		message = "Success"
	}

	return c.JSON(statusCode, Response{
		Success: true,
		Code:    statusCode,
		Message: message,
		Data:    data,
	})
}

// Error error response
func Error(c echo.Context, statusCode int, errorCode string, message string, details string) error {
	if message == "" {
		message = http.StatusText(statusCode)
	}

	return c.JSON(statusCode, Response{
		Success: false,
		Code:    statusCode,
		Message: message,
		Error: &ErrorInfo{
			Code:    errorCode,
			Details: details,
		},
	})
}

// BadRequest 400 error
func BadRequest(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusBadRequest, errorCode, message, "")
}

// BindingError binding error response
func BindingError(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusBadRequest, errorCode, message, "")
}

// Unauthorized 401 error
func Unauthorized(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusUnauthorized, errorCode, message, "")
}

// Forbidden 403 error
func Forbidden(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusForbidden, errorCode, message, "")
}

// NotFound 404 error
func NotFound(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusNotFound, errorCode, message, "")
}

// Conflict 409 error
func Conflict(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusConflict, errorCode, message, "")
}

// InternalServerError 500 error
func InternalServerError(c echo.Context, errorCode string, message string) error {
	return Error(c, http.StatusInternalServerError, errorCode, message, "")
}
