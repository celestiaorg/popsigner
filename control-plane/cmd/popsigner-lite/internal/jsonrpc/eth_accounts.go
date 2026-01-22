package jsonrpc

import (
	"context"
	"encoding/json"

	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/keystore"
)

// EthAccountsHandler handles eth_accounts requests.
type EthAccountsHandler struct {
	keystore *keystore.Keystore
}

// NewEthAccountsHandler creates a new eth_accounts handler.
func NewEthAccountsHandler(ks *keystore.Keystore) *EthAccountsHandler {
	return &EthAccountsHandler{
		keystore: ks,
	}
}

// Handle implements the eth_accounts JSON-RPC method.
// Returns a list of all Ethereum addresses available for signing.
func (h *EthAccountsHandler) Handle(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
	// Get all keys from keystore
	keys := h.keystore.ListKeys()

	// Extract addresses
	addresses := make([]string, len(keys))
	for i, key := range keys {
		addresses[i] = key.Address
	}

	return addresses, nil
}
