package jsonrpc

import (
	"context"
	"encoding/json"
)

// Version is the POPSigner version reported to OP Stack clients.
const Version = "popsigner-v1.0.0"

// HealthStatusHandler handles the health_status JSON-RPC method.
// This method is required by op-service/signer/client.go for client initialization.
type HealthStatusHandler struct{}

// NewHealthStatusHandler creates a new health_status handler.
func NewHealthStatusHandler() *HealthStatusHandler {
	return &HealthStatusHandler{}
}

// Handle implements the health_status JSON-RPC method.
// Returns the POPSigner version string.
//
// Request:
//
//	{
//	  "jsonrpc": "2.0",
//	  "method": "health_status",
//	  "params": [],
//	  "id": 1
//	}
//
// Response:
//
//	{
//	  "jsonrpc": "2.0",
//	  "result": "popsigner-v1.0.0",
//	  "id": 1
//	}
func (h *HealthStatusHandler) Handle(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
	// health_status takes no parameters and returns a version string
	// This is used by op-node, op-batcher, op-proposer to verify the signer is reachable
	return Version, nil
}

