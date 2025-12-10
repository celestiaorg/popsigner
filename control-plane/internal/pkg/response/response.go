// Package response provides JSON response helpers for API handlers.
package response

import (
	"encoding/json"
	"net/http"

	apierrors "github.com/Bidon15/banhbaoring/control-plane/internal/pkg/errors"
)

// Response represents a standard API response envelope.
type Response struct {
	Data  any   `json:"data,omitempty"`
	Error any   `json:"error,omitempty"`
	Meta  *Meta `json:"meta,omitempty"`
}

// Meta contains pagination metadata.
type Meta struct {
	Page       int    `json:"page,omitempty"`
	PerPage    int    `json:"per_page,omitempty"`
	Total      int64  `json:"total,omitempty"`
	TotalPages int    `json:"total_pages,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
	PrevCursor string `json:"prev_cursor,omitempty"`
}

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(Response{Data: data}); err != nil {
		// Log error but can't do much else at this point
		http.Error(w, `{"error":{"code":"internal_error","message":"Failed to encode response"}}`, http.StatusInternalServerError)
	}
}

// JSONWithMeta writes a JSON response with pagination metadata.
func JSONWithMeta(w http.ResponseWriter, status int, data any, meta *Meta) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(Response{Data: data, Meta: meta}); err != nil {
		http.Error(w, `{"error":{"code":"internal_error","message":"Failed to encode response"}}`, http.StatusInternalServerError)
	}
}

// Error writes an error response.
func Error(w http.ResponseWriter, err error) {
	apiErr := apierrors.AsAPIError(err)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.StatusCode)
	json.NewEncoder(w).Encode(Response{Error: apiErr})
}

// ErrorWithStatus writes an error response with a custom status code.
func ErrorWithStatus(w http.ResponseWriter, status int, err error) {
	apiErr := apierrors.AsAPIError(err)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{Error: apiErr})
}

// Created writes a 201 Created response.
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, data)
}

// OK writes a 200 OK response.
func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, data)
}

// NoContent writes a 204 No Content response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Accepted writes a 202 Accepted response.
func Accepted(w http.ResponseWriter, data any) {
	JSON(w, http.StatusAccepted, data)
}

// BadRequest writes a 400 Bad Request error response.
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, apierrors.ErrBadRequest.WithMessage(message))
}

// Unauthorized writes a 401 Unauthorized error response.
func Unauthorized(w http.ResponseWriter) {
	Error(w, apierrors.ErrUnauthorized)
}

// Forbidden writes a 403 Forbidden error response.
func Forbidden(w http.ResponseWriter) {
	Error(w, apierrors.ErrForbidden)
}

// NotFound writes a 404 Not Found error response.
func NotFound(w http.ResponseWriter, resource string) {
	Error(w, apierrors.NewNotFoundError(resource))
}

// InternalError writes a 500 Internal Server Error response.
func InternalError(w http.ResponseWriter) {
	Error(w, apierrors.ErrInternal)
}

// ValidationError writes a 400 validation error response.
func ValidationError(w http.ResponseWriter, field, message string) {
	Error(w, apierrors.NewValidationError(field, message))
}

// ValidationErrors writes a 400 validation error response with multiple field errors.
func ValidationErrors(w http.ResponseWriter, errors map[string]string) {
	Error(w, apierrors.NewValidationErrors(errors))
}

