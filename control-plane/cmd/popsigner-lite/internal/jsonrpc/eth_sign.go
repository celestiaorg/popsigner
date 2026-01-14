package jsonrpc

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/keystore"
	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/signer"
)

// EthSignHandler handles eth_sign and personal_sign requests.
type EthSignHandler struct {
	keystore *keystore.Keystore
	signer   *signer.EthereumSigner
}

// NewEthSignHandler creates a new eth_sign handler.
func NewEthSignHandler(ks *keystore.Keystore, s *signer.TransactionSigner) *EthSignHandler {
	return &EthSignHandler{
		keystore: ks,
		signer:   signer.NewEthereumSigner(),
	}
}

// HandleEthSign implements the eth_sign JSON-RPC method.
// eth_sign(address, message)
// Signs data with the given address.
// IMPORTANT: eth_sign calculates an Ethereum specific signature with:
// sign(keccak256("\x19Ethereum Signed Message:\n" + len(message) + message)))
func (h *EthSignHandler) HandleEthSign(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
	// Parse params: [address, data]
	var args []string
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, ErrInvalidParams(fmt.Sprintf("failed to parse params: %v", err))
	}
	if len(args) < 2 {
		return nil, ErrInvalidParams("eth_sign requires address and data parameters")
	}

	address := args[0]
	dataHex := args[1]

	// Decode the data
	data, err := hexutil.Decode(dataHex)
	if err != nil {
		return nil, ErrInvalidParams(fmt.Sprintf("invalid data hex: %v", err))
	}

	// Get the key
	key, rpcErr := h.getKey(address)
	if rpcErr != nil {
		return nil, rpcErr
	}

	// eth_sign adds the Ethereum prefix and hashes the message
	hash := h.ethSignHash(data)

	// Sign the hash
	signature, err := h.signer.SignHash(hash, key.PrivateKey)
	if err != nil {
		return nil, ErrSigningFailed(fmt.Sprintf("failed to sign: %v", err))
	}

	return hexutil.Encode(signature), nil
}

// HandlePersonalSign implements the personal_sign JSON-RPC method.
// personal_sign(data, address)
// Note: personal_sign has parameters in reversed order compared to eth_sign
func (h *EthSignHandler) HandlePersonalSign(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
	// Parse params: [data, address]
	var args []string
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, ErrInvalidParams(fmt.Sprintf("failed to parse params: %v", err))
	}
	if len(args) < 2 {
		return nil, ErrInvalidParams("personal_sign requires data and address parameters")
	}

	dataHex := args[0]
	address := args[1]

	// Decode the data
	data, err := hexutil.Decode(dataHex)
	if err != nil {
		return nil, ErrInvalidParams(fmt.Sprintf("invalid data hex: %v", err))
	}

	// Get the key
	key, rpcErr := h.getKey(address)
	if rpcErr != nil {
		return nil, rpcErr
	}

	// personal_sign adds the Ethereum prefix and hashes the message
	hash := h.ethSignHash(data)

	// Sign the hash
	signature, err := h.signer.SignHash(hash, key.PrivateKey)
	if err != nil {
		return nil, ErrSigningFailed(fmt.Sprintf("failed to sign: %v", err))
	}

	return hexutil.Encode(signature), nil
}

// getKey retrieves a key from the keystore by address (case-insensitive).
func (h *EthSignHandler) getKey(address string) (*keystore.Key, *Error) {
	// Ensure address has 0x prefix
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}

	// Normalize to lowercase
	address = strings.ToLower(address)

	// Validate address format
	if !common.IsHexAddress(address) {
		return nil, ErrInvalidAddress(fmt.Sprintf("invalid Ethereum address: %s", address))
	}

	// Try to get key directly
	key, err := h.keystore.GetKey(address)
	if err != nil {
		// Try case-insensitive search
		keys := h.keystore.ListKeys()
		for _, k := range keys {
			if strings.EqualFold(k.Address, address) {
				return k, nil
			}
		}
		return nil, ErrKeyNotFound(fmt.Sprintf("no key found for address %s", address))
	}

	return key, nil
}

// ethSignHash calculates the Ethereum signed message hash.
// This adds the prefix "\x19Ethereum Signed Message:\n" + len(message) and hashes it.
func (h *EthSignHandler) ethSignHash(data []byte) []byte {
	// Use go-ethereum's accounts.TextHash which implements the standard Ethereum signed message hash
	return crypto.Keccak256(
		[]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(data))),
		data,
	)
}

// signWithKey is a helper that signs data with a private key.
func (h *EthSignHandler) signWithKey(hash []byte, privateKey *ecdsa.PrivateKey) ([]byte, error) {
	return h.signer.SignHash(hash, privateKey)
}
