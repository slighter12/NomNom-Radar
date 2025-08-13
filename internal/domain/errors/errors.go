package errors

import (
	"net/http"

	"github.com/pkg/errors"
)

// AppError unified application error interface
type AppError interface {
	error
	HTTPCode() int     // HTTP status code
	ErrorCode() string // Business error code
	Message() string   // User-friendly error message
	Details() string   // Detailed error information (optional)
}

// BaseError basic error structure that implements AppError interface
type BaseError struct {
	httpCode  int
	errorCode string
	message   string
	details   string
}

// NewBaseError creates a new base error
func NewBaseError(httpCode int, errorCode, message, details string) *BaseError {
	return &BaseError{
		httpCode:  httpCode,
		errorCode: errorCode,
		message:   message,
		details:   details,
	}
}

// Error implements error interface
func (e *BaseError) Error() string {
	return e.message
}

// WrapMessage wraps the error with additional context message
func (e *BaseError) WrapMessage(message string) error {
	return errors.Wrap(e, message)
}

// HTTPCode returns HTTP status code
func (e *BaseError) HTTPCode() int {
	return e.httpCode
}

// ErrorCode returns business error code
func (e *BaseError) ErrorCode() string {
	return e.errorCode
}

// Message returns user-friendly error message
func (e *BaseError) Message() string {
	return e.message
}

// Details returns detailed error information
func (e *BaseError) Details() string {
	return e.details
}

// WithDetails adds detailed error information
func (e *BaseError) WithDetails(details string) *BaseError {
	return &BaseError{
		httpCode:  e.httpCode,
		errorCode: e.errorCode,
		message:   e.message,
		details:   details,
	}
}

// Predefined error types
var (
	// User-related errors
	ErrUserNotFound = NewBaseError(
		http.StatusNotFound,
		"USER_NOT_FOUND",
		"User not found",
		"",
	)

	ErrUserAlreadyExists = NewBaseError(
		http.StatusConflict,
		"USER_ALREADY_EXISTS",
		"User with this email already exists",
		"",
	)

	ErrUserCreationFailed = NewBaseError(
		http.StatusInternalServerError,
		"USER_CREATION_FAILED",
		"Failed to create user",
		"",
	)

	// Authentication-related errors
	ErrAuthNotFound = NewBaseError(
		http.StatusUnauthorized,
		"AUTH_NOT_FOUND",
		"Authentication method not found",
		"",
	)

	ErrInvalidCredentials = NewBaseError(
		http.StatusUnauthorized,
		"INVALID_CREDENTIALS",
		"Invalid email or password",
		"",
	)

	ErrRefreshTokenInvalid = NewBaseError(
		http.StatusUnauthorized,
		"REFRESH_TOKEN_INVALID",
		"Invalid or expired refresh token",
		"",
	)

	ErrPasswordHashFailed = NewBaseError(
		http.StatusInternalServerError,
		"PASSWORD_HASH_FAILED",
		"Password processing error",
		"",
	)

	// OAuth-related errors
	ErrOAuthFailed = NewBaseError(
		http.StatusUnauthorized,
		"OAUTH_FAILED",
		"OAuth authentication failed",
		"",
	)

	ErrOAuthCodeInvalid = NewBaseError(
		http.StatusBadRequest,
		"OAUTH_CODE_INVALID",
		"Invalid authorization code",
		"",
	)

	ErrOAuthTokenInvalid = NewBaseError(
		http.StatusBadRequest,
		"OAUTH_TOKEN_INVALID",
		"Invalid ID token",
		"",
	)

	// Merchant-related errors
	ErrMerchantAlreadyExists = NewBaseError(
		http.StatusConflict,
		"MERCHANT_ALREADY_EXISTS",
		"Merchant with this email already exists",
		"",
	)

	// Validation-related errors
	ErrValidationFailed = NewBaseError(
		http.StatusBadRequest,
		"VALIDATION_FAILED",
		"Validation failed",
		"",
	)

	// General errors
	ErrInternalError = NewBaseError(
		http.StatusInternalServerError,
		"INTERNAL_ERROR",
		"Internal server error",
		"",
	)
)

// DatabaseExecute error structure that implements AppError interface
type DatabaseExecuteError struct {
	err     error
	details string
}

// NewDatabaseExecuteError creates a Database-related errors
func NewDatabaseExecuteError(err error, details string) AppError {
	return &DatabaseExecuteError{
		err:     err,
		details: details,
	}
}

// Error implements error interface
func (e *DatabaseExecuteError) Error() string {
	return errors.Wrap(e.err, "Database execute failed").Error()
}

// HTTPCode returns HTTP status code
func (e *DatabaseExecuteError) HTTPCode() int {
	return http.StatusInternalServerError
}

// ErrorCode returns business error code
func (e *DatabaseExecuteError) ErrorCode() string {
	return "DATABASE_EXECUTE_FAILED"
}

// Message returns user-friendly error message
func (e *DatabaseExecuteError) Message() string {
	return "Database execute failed"
}

// Details returns detailed error information
func (e *DatabaseExecuteError) Details() string {
	return e.details
}
