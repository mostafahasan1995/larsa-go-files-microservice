package utils

import "net/http"

// AppError represents an application-specific error with an HTTP status code
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	return e.Message
}

// NewApiError creates a new AppError
func NewApiError(code int, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// GetStatusCode returns the HTTP status code for an error
// If the error is an AppError, it returns its code, otherwise it returns 500
func GetStatusCode(err error) int {
	if ae, ok := err.(*AppError); ok {
		return ae.Code
	}
	return http.StatusInternalServerError
}
