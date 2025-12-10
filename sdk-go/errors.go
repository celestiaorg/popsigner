package banhbaoring

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Error represents an API error response.
type Error struct {
	// StatusCode is the HTTP status code.
	StatusCode int `json:"-"`
	// Code is the error code (e.g., "unauthorized", "not_found").
	Code string `json:"code"`
	// Message is a human-readable error message.
	Message string `json:"message"`
	// Details contains additional error details.
	Details map[string]interface{} `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Message
}

// IsNotFound returns true if the error is a not found error.
func (e *Error) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound || e.Code == "not_found"
}

// IsUnauthorized returns true if the error is an authorization error.
func (e *Error) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized || e.Code == "unauthorized"
}

// IsForbidden returns true if the error is a permission error.
func (e *Error) IsForbidden() bool {
	return e.StatusCode == http.StatusForbidden || e.Code == "forbidden"
}

// IsRateLimited returns true if the error is a rate limit error.
func (e *Error) IsRateLimited() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.Code == "rate_limited"
}

// IsValidationError returns true if the error is a validation error.
func (e *Error) IsValidationError() bool {
	return e.StatusCode == http.StatusBadRequest || e.Code == "validation_error"
}

// Common error codes.
var (
	// ErrUnauthorized is returned when the API key is invalid or missing.
	ErrUnauthorized = &Error{
		StatusCode: http.StatusUnauthorized,
		Code:       "unauthorized",
		Message:    "Invalid or missing API key",
	}

	// ErrForbidden is returned when the API key lacks required permissions.
	ErrForbidden = &Error{
		StatusCode: http.StatusForbidden,
		Code:       "forbidden",
		Message:    "Insufficient permissions",
	}

	// ErrNotFound is returned when a resource is not found.
	ErrNotFound = &Error{
		StatusCode: http.StatusNotFound,
		Code:       "not_found",
		Message:    "Resource not found",
	}

	// ErrRateLimited is returned when rate limits are exceeded.
	ErrRateLimited = &Error{
		StatusCode: http.StatusTooManyRequests,
		Code:       "rate_limited",
		Message:    "Rate limit exceeded",
	}
)

// parseError parses an error response from the API.
func parseError(statusCode int, body []byte) error {
	// Try to parse structured error
	var apiError struct {
		Error struct {
			Code    string                 `json:"code"`
			Message string                 `json:"message"`
			Details map[string]interface{} `json:"details,omitempty"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &apiError); err == nil && apiError.Error.Code != "" {
		return &Error{
			StatusCode: statusCode,
			Code:       apiError.Error.Code,
			Message:    apiError.Error.Message,
			Details:    apiError.Error.Details,
		}
	}

	// Try alternative format
	var simpleError struct {
		Code    string                 `json:"code"`
		Message string                 `json:"message"`
		Details map[string]interface{} `json:"details,omitempty"`
	}

	if err := json.Unmarshal(body, &simpleError); err == nil && simpleError.Message != "" {
		return &Error{
			StatusCode: statusCode,
			Code:       simpleError.Code,
			Message:    simpleError.Message,
			Details:    simpleError.Details,
		}
	}

	// Fallback to generic error
	return &Error{
		StatusCode: statusCode,
		Code:       http.StatusText(statusCode),
		Message:    string(body),
	}
}

// IsAPIError checks if an error is an API error and returns it.
func IsAPIError(err error) (*Error, bool) {
	if apiErr, ok := err.(*Error); ok {
		return apiErr, true
	}
	return nil, false
}

