package jsonrpc

import "encoding/json"

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// Error represents a JSON-RPC 2.0 error.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 error codes
const (
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
)

// Application-specific error codes (from -32000 to -32099)
const (
	ErrCodeUnauthorized      = -32001
	ErrCodeResourceNotFound  = -32002
	ErrCodeSigningFailed     = -32003
	ErrCodeInvalidAddress    = -32004
	ErrCodeKeyNotFound       = -32005
)

// Helper functions to create standard errors
func NewError(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

func NewErrorWithData(code int, message string, data interface{}) *Error {
	return &Error{Code: code, Message: message, Data: data}
}

func ErrParseError(message string) *Error {
	return NewError(ErrCodeParse, message)
}

func ErrInvalidRequest(message string) *Error {
	return NewError(ErrCodeInvalidRequest, message)
}

func ErrMethodNotFound(method string) *Error {
	return NewErrorWithData(ErrCodeMethodNotFound, "method not found", method)
}

func ErrInvalidParams(message string) *Error {
	return NewError(ErrCodeInvalidParams, message)
}

func ErrInternal(message string) *Error {
	return NewError(ErrCodeInternal, message)
}

func ErrUnauthorized(message string) *Error {
	return NewError(ErrCodeUnauthorized, message)
}

func ErrResourceNotFound(message string) *Error {
	return NewError(ErrCodeResourceNotFound, message)
}

func ErrSigningFailed(message string) *Error {
	return NewError(ErrCodeSigningFailed, message)
}

func ErrInvalidAddress(message string) *Error {
	return NewError(ErrCodeInvalidAddress, message)
}

func ErrKeyNotFound(message string) *Error {
	return NewError(ErrCodeKeyNotFound, message)
}
