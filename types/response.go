package types

import "time"

// Response is the unified API response format.
type Response struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Message   string      `json:"message"`
	Code      int         `json:"code"`
	Timestamp string      `json:"timestamp"`
}

// Error code constants
const (
	ERR_INVALID_PARAMETER  = 400
	ERR_UNAUTHORIZED       = 401
	ERR_NOT_FOUND          = 404
	ERR_INTERNAL           = 500
	ERR_SDK_ERROR          = 5001
	ERR_NETWORK_ERROR      = 5002
	ERR_TRANSACTION_FAILED = 5003
)

// SuccessResponse builds a successful Response.
func SuccessResponse(data interface{}, message string) Response {
	return Response{
		Success:   true,
		Data:      data,
		Message:   message,
		Code:      0,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// ErrorResponse builds an error Response.
func ErrorResponse(code int, message string, details interface{}) Response {
	return Response{
		Success:   false,
		Data:      details,
		Message:   message,
		Code:      code,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}
