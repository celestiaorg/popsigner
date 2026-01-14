package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/keystore"
	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/signer"
)

// SignBlockPayloadHandler handles opsigner_signBlockPayload and opsigner_signBlockPayloadV2 requests.
// These methods are used by op-node for P2P sequencer block signing.
type SignBlockPayloadHandler struct {
	keystore *keystore.Keystore
	signer   *signer.EthereumSigner
}

// NewSignBlockPayloadHandler creates a new block payload signing handler.
func NewSignBlockPayloadHandler(ks *keystore.Keystore, s *signer.TransactionSigner) *SignBlockPayloadHandler {
	return &SignBlockPayloadHandler{
		keystore: ks,
		signer:   signer.NewEthereumSigner(),
	}
}

// BlockPayloadArgs represents the arguments for block payload signing.
type BlockPayloadArgs struct {
	Address string `json:"address"`
	Data    string `json:"data"`
}

// BlockPayloadV2Args represents the arguments for block payload signing v2.
type BlockPayloadV2Args struct {
	Address   string `json:"address"`
	ChainID   string `json:"chainId"`
	BlockHash string `json:"blockHash"`
	// Additional fields may be added as needed
}

// SignatureResponse represents the response for block signing.
type SignatureResponse struct {
	Signature string `json:"signature"`
}

// Handle implements the opsigner_signBlockPayload JSON-RPC method (v1).
// This signs a block payload for OP Stack P2P sequencer consensus.
func (h *SignBlockPayloadHandler) Handle(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
	// Parse arguments
	var args []BlockPayloadArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, ErrInvalidParams(fmt.Sprintf("failed to parse params: %v", err))
	}
	if len(args) == 0 {
		return nil, ErrInvalidParams("block payload arguments required")
	}

	payload := args[0]

	// Validate address
	if payload.Address == "" {
		return nil, ErrInvalidParams("address is required")
	}
	if payload.Data == "" {
		return nil, ErrInvalidParams("data is required")
	}

	// Get the key
	key, rpcErr := h.getKey(payload.Address)
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Decode the data to sign
	data, err := hexutil.Decode(payload.Data)
	if err != nil {
		return nil, ErrInvalidParams(fmt.Sprintf("invalid data hex: %v", err))
	}

	// Hash the data (Keccak256)
	hash := crypto.Keccak256(data)

	// Sign the hash
	signature, err := h.signer.SignHash(hash, key.PrivateKey)
	if err != nil {
		return nil, ErrSigningFailed(fmt.Sprintf("failed to sign block payload: %v", err))
	}

	return SignatureResponse{
		Signature: hexutil.Encode(signature),
	}, nil
}

// HandleV2 implements the opsigner_signBlockPayloadV2 JSON-RPC method (v2).
// This is an enhanced version with additional fields like chainID and blockHash.
func (h *SignBlockPayloadHandler) HandleV2(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
	// Parse arguments
	var args []BlockPayloadV2Args
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, ErrInvalidParams(fmt.Sprintf("failed to parse params: %v", err))
	}
	if len(args) == 0 {
		return nil, ErrInvalidParams("block payload v2 arguments required")
	}

	payload := args[0]

	// Validate address
	if payload.Address == "" {
		return nil, ErrInvalidParams("address is required")
	}
	if payload.BlockHash == "" {
		return nil, ErrInvalidParams("blockHash is required")
	}

	// Get the key
	key, rpcErr := h.getKey(payload.Address)
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Decode the block hash
	blockHash, err := hexutil.Decode(payload.BlockHash)
	if err != nil {
		return nil, ErrInvalidParams(fmt.Sprintf("invalid blockHash hex: %v", err))
	}

	// For v2, we may need to construct a signing payload with chainID
	// For now, we'll sign the block hash directly
	// This can be enhanced based on actual OP Stack requirements
	var dataToSign []byte
	if payload.ChainID != "" {
		// Include chain ID in the signing data if provided
		chainIDBytes, err := hexutil.Decode(payload.ChainID)
		if err != nil {
			return nil, ErrInvalidParams(fmt.Sprintf("invalid chainId hex: %v", err))
		}
		// Concatenate chainID and blockHash
		dataToSign = append(chainIDBytes, blockHash...)
	} else {
		dataToSign = blockHash
	}

	// Hash the data
	hash := crypto.Keccak256(dataToSign)

	// Sign the hash
	signature, err := h.signer.SignHash(hash, key.PrivateKey)
	if err != nil {
		return nil, ErrSigningFailed(fmt.Sprintf("failed to sign block payload v2: %v", err))
	}

	return SignatureResponse{
		Signature: hexutil.Encode(signature),
	}, nil
}

// getKey retrieves a key from the keystore by address (case-insensitive).
func (h *SignBlockPayloadHandler) getKey(address string) (*keystore.Key, *Error) {
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
