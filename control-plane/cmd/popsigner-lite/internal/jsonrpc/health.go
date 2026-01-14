package jsonrpc

import (
	"context"
	"encoding/json"
)

// HealthStatusHandler handles health_status requests.
type HealthStatusHandler struct{}

// NewHealthStatusHandler creates a new health status handler.
func NewHealthStatusHandler() *HealthStatusHandler {
	return &HealthStatusHandler{}
}

// HealthStatusResponse represents the response for health_status method.
type HealthStatusResponse struct {
	Status string `json:"status"`
}

// Handle implements the health_status JSON-RPC method.
// This is required for OP Stack signer client initialization.
// The OP Stack signer client expects a string "ok", not an object.
func (h *HealthStatusHandler) Handle(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
	return "ok", nil
}
