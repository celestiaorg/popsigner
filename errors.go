package banhbaoring

import (
	"errors"
	"fmt"
)

// Sentinel errors - Configuration
var (
	ErrMissingBaoAddr   = errors.New("banhbaoring: BaoAddr is required")
	ErrMissingBaoToken  = errors.New("banhbaoring: BaoToken is required")
	ErrMissingStorePath = errors.New("banhbaoring: StorePath is required")
)

// Sentinel errors - Keys
var (
	ErrKeyNotFound      = errors.New("banhbaoring: key not found")
	ErrKeyExists        = errors.New("banhbaoring: key already exists")
	ErrKeyNotExportable = errors.New("banhbaoring: key is not exportable")
)

// Sentinel errors - OpenBao
var (
	ErrBaoConnection  = errors.New("banhbaoring: failed to connect to OpenBao")
	ErrBaoAuth        = errors.New("banhbaoring: authentication failed")
	ErrBaoSealed      = errors.New("banhbaoring: OpenBao is sealed")
	ErrBaoUnavailable = errors.New("banhbaoring: OpenBao is unavailable")
)

// Sentinel errors - Operations
var (
	ErrSigningFailed    = errors.New("banhbaoring: signing failed")
	ErrInvalidSignature = errors.New("banhbaoring: invalid signature")
	ErrUnsupportedAlgo  = errors.New("banhbaoring: unsupported algorithm")
	ErrStorePersist     = errors.New("banhbaoring: failed to persist")
	ErrStoreCorrupted   = errors.New("banhbaoring: store corrupted")
)

// BaoError represents an OpenBao API error.
type BaoError struct {
	StatusCode int
	Errors     []string
	RequestID  string
}

// Error implements the error interface.
func (e *BaoError) Error() string {
	if len(e.Errors) == 0 {
		return fmt.Sprintf("OpenBao error (HTTP %d)", e.StatusCode)
	}
	return fmt.Sprintf("OpenBao error (HTTP %d): %s", e.StatusCode, e.Errors[0])
}

// Is implements the errors.Is interface for HTTP status code mapping.
// This allows checking BaoError against sentinel errors based on status codes.
func (e *BaoError) Is(target error) bool {
	switch e.StatusCode {
	case 403:
		return errors.Is(target, ErrBaoAuth)
	case 404:
		return errors.Is(target, ErrKeyNotFound)
	case 503:
		return errors.Is(target, ErrBaoSealed)
	default:
		return false
	}
}

// NewBaoError creates a new BaoError with the given parameters.
func NewBaoError(statusCode int, errs []string, requestID string) *BaoError {
	return &BaoError{
		StatusCode: statusCode,
		Errors:     errs,
		RequestID:  requestID,
	}
}

// KeyError wraps an error with key context.
type KeyError struct {
	KeyName string
	Op      string
	Err     error
}

// Error implements the error interface.
func (e *KeyError) Error() string {
	return fmt.Sprintf("%s key %q: %v", e.Op, e.KeyName, e.Err)
}

// Unwrap implements the errors.Unwrap interface for error chaining.
func (e *KeyError) Unwrap() error {
	return e.Err
}

// WrapKeyError wraps an error with key operation context.
// Returns nil if the provided error is nil.
func WrapKeyError(op, keyName string, err error) error {
	if err == nil {
		return nil
	}
	return &KeyError{
		KeyName: keyName,
		Op:      op,
		Err:     err,
	}
}

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

// NewValidationError creates a new ValidationError with the given field and message.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
