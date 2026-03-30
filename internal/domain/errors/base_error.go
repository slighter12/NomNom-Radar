package errors

import "fmt"

// BaseError is the shared concrete implementation used by all sentinels.
type BaseError struct {
	httpCode  int
	errorCode string
	message   string
	details   string
}

// NewBaseError creates a new base error.
func NewBaseError(httpCode int, errorCode, message, details string) *BaseError {
	return &BaseError{
		httpCode:  httpCode,
		errorCode: errorCode,
		message:   message,
		details:   details,
	}
}

// Error implements the error interface.
func (e *BaseError) Error() string {
	return e.message
}

// Is matches AppError values by canonical business error identity.
func (e *BaseError) Is(target error) bool {
	targetErr, ok := target.(AppError)
	if !ok {
		return false
	}

	return e.errorCode == targetErr.ErrorCode()
}

// WrapMessage wraps the error with an additional context message.
func (e *BaseError) WrapMessage(message string) error {
	return fmt.Errorf("%s: %w", message, e)
}

// HTTPCode returns the HTTP status code associated with the error.
func (e *BaseError) HTTPCode() int {
	return e.httpCode
}

// ErrorCode returns the machine-readable business error code.
func (e *BaseError) ErrorCode() string {
	return e.errorCode
}

// Message returns the user-facing message.
func (e *BaseError) Message() string {
	return e.message
}

// Details returns optional details for downstream consumers.
func (e *BaseError) Details() string {
	return e.details
}
