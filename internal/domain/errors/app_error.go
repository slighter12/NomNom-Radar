// Package errors defines shared application errors for the HTTP API.
// This package intentionally carries HTTP status on AppError to keep response
// handling straightforward in the delivery layer.
package errors

// AppError defines the canonical application-facing error contract.
type AppError interface {
	error
	HTTPCode() int
	ErrorCode() string
	Message() string
	Details() string
}
