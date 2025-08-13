package errors

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
