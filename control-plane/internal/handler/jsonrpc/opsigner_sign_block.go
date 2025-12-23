package jsonrpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Bidon15/popsigner/control-plane/internal/middleware"
	"github.com/Bidon15/popsigner/control-plane/internal/openbao"
	"github.com/Bidon15/popsigner/control-plane/internal/repository"
)

// SignBlockPayloadHandler handles opsigner_signBlockPayload and opsigner_signBlockPayloadV2.
// These methods are used by op-node for P2P block gossip signing.
type SignBlockPayloadHandler struct {
	keyRepo   repository.KeyRepository
	baoClient *openbao.Client
}

// NewSignBlockPayloadHandler creates a new block payload signing handler.
func NewSignBlockPayloadHandler(keyRepo repository.KeyRepository, baoClient *openbao.Client) *SignBlockPayloadHandler {
	return &SignBlockPayloadHandler{
		keyRepo:   keyRepo,
		baoClient: baoClient,
	}
}

// BlockPayloadArgs represents the arguments for opsigner_signBlockPayload.
type BlockPayloadArgs struct {
	Domain        hexutil.Bytes  `json:"domain"`        // 32 bytes, always zeros for V1
	ChainID       *hexutil.Big   `json:"chainId"`       // L2 chain ID
	PayloadHash   hexutil.Bytes  `json:"payloadHash"`   // 32 bytes - keccak256 of block payload
	SenderAddress *common.Address `json:"senderAddress"` // Sequencer address (identifies which key)
}

// BlockPayloadArgsV2 represents the arguments for opsigner_signBlockPayloadV2.
// Same as V1 but chainId is encoded as 32-byte hex instead of big.Int.
type BlockPayloadArgsV2 struct {
	Domain        hexutil.Bytes   `json:"domain"`        // 32 bytes
	ChainID       hexutil.Bytes   `json:"chainId"`       // 32 bytes (padded big-endian)
	PayloadHash   hexutil.Bytes   `json:"payloadHash"`   // 32 bytes
	SenderAddress *common.Address `json:"senderAddress"` // Optional
}

// Handle implements the opsigner_signBlockPayload JSON-RPC method.
//
// Request:
//
//	{
//	  "jsonrpc": "2.0",
//	  "method": "opsigner_signBlockPayload",
//	  "params": [{
//	    "domain": "0x0000000000000000000000000000000000000000000000000000000000000000",
//	    "chainId": "0x1a4",
//	    "payloadHash": "0x...",
//	    "senderAddress": "0x..."
//	  }],
//	  "id": 1
//	}
//
// Response:
//
//	{
//	  "jsonrpc": "2.0",
//	  "result": "0x...",  // 65-byte signature (r, s, v)
//	  "id": 1
//	}
func (h *SignBlockPayloadHandler) Handle(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
	orgID := middleware.GetOrgIDFromContext(ctx)
	if orgID.String() == "00000000-0000-0000-0000-000000000000" {
		return nil, ErrUnauthorized("missing organization context")
	}

	// Parse arguments
	var args []BlockPayloadArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, ErrInvalidParams(err.Error())
	}
	if len(args) == 0 {
		return nil, ErrInvalidParams("block payload arguments required")
	}

	arg := args[0]

	// Validate arguments
	if len(arg.Domain) != 32 {
		return nil, ErrInvalidParams("domain must be 32 bytes")
	}
	if len(arg.PayloadHash) != 32 {
		return nil, ErrInvalidParams("payloadHash must be 32 bytes")
	}
	if arg.ChainID == nil {
		return nil, ErrInvalidParams("chainId is required")
	}
	if arg.SenderAddress == nil {
		return nil, ErrInvalidParams("senderAddress is required")
	}

	// Convert chainID to 32-byte big-endian
	chainIDBytes := make([]byte, 32)
	arg.ChainID.ToInt().FillBytes(chainIDBytes)

	// Compute signing hash: keccak256(domain || chainId_bytes32 || payloadHash)
	signingInput := make([]byte, 0, 96)
	signingInput = append(signingInput, arg.Domain...)
	signingInput = append(signingInput, chainIDBytes...)
	signingInput = append(signingInput, arg.PayloadHash...)
	signingHash := crypto.Keccak256(signingInput)

	// Lookup key by sender address
	senderAddr := arg.SenderAddress.Hex()
	key, err := h.keyRepo.GetByEthAddress(ctx, orgID, senderAddr)
	if err != nil {
		return nil, ErrInternal(fmt.Sprintf("failed to lookup key: %v", err))
	}
	if key == nil {
		return nil, ErrResourceNotFound(fmt.Sprintf("no key found for address %s", senderAddr))
	}

	// Sign via OpenBao (use chainID=0 for raw yParity)
	hashB64 := base64.StdEncoding.EncodeToString(signingHash)
	signResp, err := h.baoClient.SignEVM(key.BaoKeyPath, hashB64, 0)
	if err != nil {
		return nil, ErrSigningFailed(err.Error())
	}

	// Build 65-byte signature: r (32) + s (32) + v (1)
	signature, err := buildSignature65(signResp)
	if err != nil {
		return nil, ErrInternal(fmt.Sprintf("failed to build signature: %v", err))
	}

	return hexutil.Encode(signature), nil
}

// HandleV2 implements the opsigner_signBlockPayloadV2 JSON-RPC method.
// Same as V1 but chainId is encoded as 32-byte hex.
func (h *SignBlockPayloadHandler) HandleV2(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
	orgID := middleware.GetOrgIDFromContext(ctx)
	if orgID.String() == "00000000-0000-0000-0000-000000000000" {
		return nil, ErrUnauthorized("missing organization context")
	}

	// Parse V2 arguments
	var args []BlockPayloadArgsV2
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, ErrInvalidParams(err.Error())
	}
	if len(args) == 0 {
		return nil, ErrInvalidParams("block payload arguments required")
	}

	arg := args[0]

	// Validate arguments
	if len(arg.Domain) != 32 {
		return nil, ErrInvalidParams("domain must be 32 bytes")
	}
	if len(arg.ChainID) != 32 {
		return nil, ErrInvalidParams("chainId must be 32 bytes for V2")
	}
	if len(arg.PayloadHash) != 32 {
		return nil, ErrInvalidParams("payloadHash must be 32 bytes")
	}
	if arg.SenderAddress == nil {
		return nil, ErrInvalidParams("senderAddress is required")
	}

	// Compute signing hash: keccak256(domain || chainId || payloadHash)
	signingInput := make([]byte, 0, 96)
	signingInput = append(signingInput, arg.Domain...)
	signingInput = append(signingInput, arg.ChainID...)
	signingInput = append(signingInput, arg.PayloadHash...)
	signingHash := crypto.Keccak256(signingInput)

	// Lookup key by sender address
	senderAddr := arg.SenderAddress.Hex()
	key, err := h.keyRepo.GetByEthAddress(ctx, orgID, senderAddr)
	if err != nil {
		return nil, ErrInternal(fmt.Sprintf("failed to lookup key: %v", err))
	}
	if key == nil {
		return nil, ErrResourceNotFound(fmt.Sprintf("no key found for address %s", senderAddr))
	}

	// Sign via OpenBao
	hashB64 := base64.StdEncoding.EncodeToString(signingHash)
	signResp, err := h.baoClient.SignEVM(key.BaoKeyPath, hashB64, 0)
	if err != nil {
		return nil, ErrSigningFailed(err.Error())
	}

	// Build 65-byte signature
	signature, err := buildSignature65(signResp)
	if err != nil {
		return nil, ErrInternal(fmt.Sprintf("failed to build signature: %v", err))
	}

	return hexutil.Encode(signature), nil
}

// buildSignature65 constructs a 65-byte signature from OpenBao response.
// Format: r (32 bytes) + s (32 bytes) + v (1 byte)
func buildSignature65(signResp *openbao.SignEVMResponse) ([]byte, error) {
	// Parse r and s from hex strings
	rBytes, err := hexutil.Decode("0x" + signResp.R)
	if err != nil {
		return nil, fmt.Errorf("decode r: %w", err)
	}
	sBytes, err := hexutil.Decode("0x" + signResp.S)
	if err != nil {
		return nil, fmt.Errorf("decode s: %w", err)
	}

	// Convert v to single byte (should be 0 or 1 for yParity)
	v := signResp.VInt
	if v >= 27 {
		v -= 27 // Convert from Ethereum legacy v to yParity
	}

	// Pad r and s to 32 bytes each
	rPadded := make([]byte, 32)
	sPadded := make([]byte, 32)
	copy(rPadded[32-len(rBytes):], rBytes)
	copy(sPadded[32-len(sBytes):], sBytes)

	// Build 65-byte signature: r || s || v
	signature := make([]byte, 65)
	copy(signature[0:32], rPadded)
	copy(signature[32:64], sPadded)
	signature[64] = byte(v)

	return signature, nil
}

// chainIDToBytes32 converts a big.Int chain ID to 32-byte big-endian.
func chainIDToBytes32(chainID *big.Int) []byte {
	bytes := make([]byte, 32)
	chainID.FillBytes(bytes)
	return bytes
}

