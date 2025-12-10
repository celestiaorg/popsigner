// Package errors provides standardized API error types.
package errors

import (
	"fmt"
	"net/http"
)

// APIError represents a standardized API error response.
type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
	Details    any    `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return e.Message
}

// WithDetails returns a copy of the error with additional details.
func (e *APIError) WithDetails(details any) *APIError {
	return &APIError{
		Code:       e.Code,
		Message:    e.Message,
		StatusCode: e.StatusCode,
		Details:    details,
	}
}

// WithMessage returns a copy of the error with a custom message.
func (e *APIError) WithMessage(message string) *APIError {
	return &APIError{
		Code:       e.Code,
		Message:    message,
		StatusCode: e.StatusCode,
		Details:    e.Details,
	}
}

// Standard error definitions
var (
	// ErrUnauthorized is returned when authentication is required but missing or invalid.
	ErrUnauthorized = &APIError{
		Code:       "unauthorized",
		Message:    "Authentication required",
		StatusCode: http.StatusUnauthorized,
	}

	// ErrForbidden is returned when the user lacks permission for an action.
	ErrForbidden = &APIError{
		Code:       "forbidden",
		Message:    "You don't have permission to perform this action",
		StatusCode: http.StatusForbidden,
	}

	// ErrNotFound is returned when a resource is not found.
	ErrNotFound = &APIError{
		Code:       "not_found",
		Message:    "Resource not found",
		StatusCode: http.StatusNotFound,
	}

	// ErrBadRequest is returned when the request is malformed.
	ErrBadRequest = &APIError{
		Code:       "bad_request",
		Message:    "Invalid request",
		StatusCode: http.StatusBadRequest,
	}

	// ErrRateLimited is returned when rate limits are exceeded.
	ErrRateLimited = &APIError{
		Code:       "rate_limited",
		Message:    "Too many requests. Please try again later.",
		StatusCode: http.StatusTooManyRequests,
	}

	// ErrQuotaExceeded is returned when plan limits are exceeded.
	ErrQuotaExceeded = &APIError{
		Code:       "quota_exceeded",
		Message:    "You've exceeded your plan limits",
		StatusCode: http.StatusPaymentRequired,
	}

	// ErrInternal is returned for unexpected server errors.
	ErrInternal = &APIError{
		Code:       "internal_error",
		Message:    "An internal error occurred",
		StatusCode: http.StatusInternalServerError,
	}

	// ErrConflict is returned when a resource already exists.
	ErrConflict = &APIError{
		Code:       "conflict",
		Message:    "Resource already exists",
		StatusCode: http.StatusConflict,
	}

	// ErrServiceUnavailable is returned when a dependent service is unavailable.
	ErrServiceUnavailable = &APIError{
		Code:       "service_unavailable",
		Message:    "Service temporarily unavailable",
		StatusCode: http.StatusServiceUnavailable,
	}
)

// NewValidationError creates a validation error for a specific field.
func NewValidationError(field, message string) *APIError {
	return &APIError{
		Code:       "validation_error",
		Message:    fmt.Sprintf("Validation failed: %s", message),
		StatusCode: http.StatusBadRequest,
		Details: map[string]string{
			"field": field,
			"error": message,
		},
	}
}

// NewValidationErrors creates a validation error with multiple field errors.
func NewValidationErrors(errors map[string]string) *APIError {
	return &APIError{
		Code:       "validation_error",
		Message:    "One or more fields failed validation",
		StatusCode: http.StatusBadRequest,
		Details:    errors,
	}
}

// NewNotFoundError creates a not found error for a specific resource type.
func NewNotFoundError(resource string) *APIError {
	return &APIError{
		Code:       "not_found",
		Message:    fmt.Sprintf("%s not found", resource),
		StatusCode: http.StatusNotFound,
	}
}

// NewConflictError creates a conflict error with a custom message.
func NewConflictError(message string) *APIError {
	return &APIError{
		Code:       "conflict",
		Message:    message,
		StatusCode: http.StatusConflict,
	}
}

// NewInternalError creates an internal error with a custom message.
// This should only be used in development; in production, use ErrInternal.
func NewInternalError(message string) *APIError {
	return &APIError{
		Code:       "internal_error",
		Message:    message,
		StatusCode: http.StatusInternalServerError,
	}
}

// IsAPIError checks if an error is an APIError.
func IsAPIError(err error) bool {
	_, ok := err.(*APIError)
	return ok
}

// AsAPIError converts an error to an APIError if possible.
// Returns ErrInternal if the error is not an APIError.
func AsAPIError(err error) *APIError {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr
	}
	return ErrInternal
}

