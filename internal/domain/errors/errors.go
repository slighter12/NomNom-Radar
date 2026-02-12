package errors

import (
	"net/http"

	"radar/internal/errors"
)

// AppError defines the interface for application-specific errors
type AppError interface {
	error
	HTTPCode() int     // HTTP status code
	ErrorCode() string // Business error code
	Message() string   // User-friendly error message
	Details() string   // Detailed error information (optional)
}

// BaseError is a basic error structure that implements the AppError interface
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

// Error implements the error interface
func (e *BaseError) Error() string {
	return e.message
}

// WrapMessage wraps the error with additional context message
func (e *BaseError) WrapMessage(message string) error {
	return errors.Wrap(e, message)
}

// HTTPCode returns the HTTP status code
func (e *BaseError) HTTPCode() int {
	return e.httpCode
}

// ErrorCode returns the business error code
func (e *BaseError) ErrorCode() string {
	return e.errorCode
}

// Message returns the user-friendly error message
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
		"找不到該使用者",
		"",
	)

	ErrUserAlreadyExists = NewBaseError(
		http.StatusConflict,
		"USER_ALREADY_EXISTS",
		"此電子郵件已被註冊",
		"",
	)

	ErrUserCreationFailed = NewBaseError(
		http.StatusInternalServerError,
		"USER_CREATION_FAILED",
		"建立使用者失敗",
		"",
	)

	ErrUserUpdateFailed = NewBaseError(
		http.StatusInternalServerError,
		"USER_UPDATE_FAILED",
		"更新使用者失敗",
		"",
	)

	// Authentication-related errors
	ErrAuthNotFound = NewBaseError(
		http.StatusUnauthorized,
		"AUTH_NOT_FOUND",
		"找不到認證方式",
		"",
	)

	ErrInvalidCredentials = NewBaseError(
		http.StatusUnauthorized,
		"INVALID_CREDENTIALS",
		"電子郵件或密碼錯誤",
		"",
	)

	ErrRefreshTokenInvalid = NewBaseError(
		http.StatusUnauthorized,
		"REFRESH_TOKEN_INVALID",
		"無效或已過期的重新整理權杖",
		"",
	)

	ErrPasswordHashFailed = NewBaseError(
		http.StatusInternalServerError,
		"PASSWORD_HASH_FAILED",
		"密碼處理錯誤",
		"",
	)

	ErrPasswordStrength = NewBaseError(
		http.StatusBadRequest,
		"PASSWORD_STRENGTH",
		"密碼強度不足",
		"",
	)

	ErrPasswordForbiddenWords = NewBaseError(
		http.StatusBadRequest,
		"PASSWORD_FORBIDDEN_WORDS",
		"密碼包含禁止使用的字詞或模式",
		"",
	)

	// OAuth-related errors
	ErrOAuthFailed = NewBaseError(
		http.StatusUnauthorized,
		"OAUTH_FAILED",
		"OAuth 認證失敗",
		"",
	)

	ErrOAuthCodeInvalid = NewBaseError(
		http.StatusBadRequest,
		"OAUTH_CODE_INVALID",
		"無效的授權碼",
		"",
	)

	ErrOAuthTokenInvalid = NewBaseError(
		http.StatusBadRequest,
		"OAUTH_TOKEN_INVALID",
		"無效的 ID 權杖",
		"",
	)

	// Merchant-related errors
	ErrMerchantAlreadyExists = NewBaseError(
		http.StatusConflict,
		"MERCHANT_ALREADY_EXISTS",
		"此電子郵件已被註冊為商家",
		"",
	)

	// Validation-related errors
	ErrValidationFailed = NewBaseError(
		http.StatusBadRequest,
		"VALIDATION_FAILED",
		"輸入資料驗證失敗",
		"",
	)

	// Address-related errors
	ErrAddressNotFound = NewBaseError(
		http.StatusNotFound,
		"ADDRESS_NOT_FOUND",
		"找不到該地址",
		"",
	)

	ErrPrimaryAddressConflict = NewBaseError(
		http.StatusConflict,
		"PRIMARY_ADDRESS_CONFLICT",
		"該使用者已設定主要地址",
		"",
	)

	ErrAddressOwnershipViolation = NewBaseError(
		http.StatusForbidden,
		"ADDRESS_OWNERSHIP_VIOLATION",
		"您沒有權限存取此地址",
		"",
	)

	// Refresh token-related errors
	ErrRefreshTokenNotFound = NewBaseError(
		http.StatusNotFound,
		"REFRESH_TOKEN_NOT_FOUND",
		"找不到重新整理權杖",
		"",
	)

	ErrRefreshTokenExpired = NewBaseError(
		http.StatusUnauthorized,
		"REFRESH_TOKEN_EXPIRED",
		"重新整理權杖已過期",
		"",
	)

	ErrSessionLimitExceeded = NewBaseError(
		http.StatusTooManyRequests,
		"SESSION_LIMIT_EXCEEDED",
		"已達到最大同時登入裝置數量",
		"",
	)

	// Transaction-related errors
	ErrTransactionFailed = NewBaseError(
		http.StatusInternalServerError,
		"TRANSACTION_FAILED",
		"資料庫交易失敗",
		"",
	)

	// General errors
	ErrInternalError = NewBaseError(
		http.StatusInternalServerError,
		"INTERNAL_ERROR",
		"系統內部錯誤",
		"",
	)

	ErrForbidden = NewBaseError(
		http.StatusForbidden,
		"FORBIDDEN",
		"存取被拒絕",
		"",
	)

	ErrNotFound = NewBaseError(
		http.StatusNotFound,
		"NOT_FOUND",
		"找不到該資源",
		"",
	)

	ErrConflict = NewBaseError(
		http.StatusConflict,
		"CONFLICT",
		"資源衝突",
		"",
	)
)

// DatabaseExecuteError represents a database execution error, implementing the AppError interface
type DatabaseExecuteError struct {
	err     error
	details string
}

// NewDatabaseExecuteError creates a database-related error
func NewDatabaseExecuteError(err error, details string) AppError {
	return &DatabaseExecuteError{
		err:     err,
		details: details,
	}
}

// Error implements the error interface
func (e *DatabaseExecuteError) Error() string {
	return errors.Wrap(e.err, "database execution failed").Error()
}

// HTTPCode returns the HTTP status code
func (e *DatabaseExecuteError) HTTPCode() int {
	return http.StatusInternalServerError
}

// ErrorCode returns the business error code
func (e *DatabaseExecuteError) ErrorCode() string {
	return "DATABASE_EXECUTE_FAILED"
}

// Message returns the user-friendly error message
func (e *DatabaseExecuteError) Message() string {
	return "資料庫執行失敗"
}

// Details returns detailed error information
func (e *DatabaseExecuteError) Details() string {
	return e.details
}
