package errors

// ErrorInfo contains detailed error information
type ErrorInfo struct {
	Code    string `json:"code"`              // Business error code, e.g., "USER_NOT_FOUND"
	Message string `json:"message"`           // User-friendly error message
	Details any    `json:"details,omitempty"` // Detailed error information (optional)
}

// MetaInfo represents response metadata
type MetaInfo struct {
	RequestID string `json:"request_id"` // Request tracking ID
}

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
