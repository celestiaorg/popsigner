package jsonrpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/ethereum"
	"github.com/Bidon15/popsigner/control-plane/internal/middleware"
	"github.com/Bidon15/popsigner/control-plane/internal/models"
	"github.com/Bidon15/popsigner/control-plane/internal/openbao"
	"github.com/Bidon15/popsigner/control-plane/internal/repository"
)

// EthSignTransactionHandler handles eth_signTransaction requests.
type EthSignTransactionHandler struct {
	keyRepo   repository.KeyRepository
	baoClient *openbao.Client
	auditRepo repository.AuditRepository
	usageRepo repository.UsageRepository
}

// NewEthSignTransactionHandler creates a new eth_signTransaction handler.
func NewEthSignTransactionHandler(keyRepo repository.KeyRepository, baoClient *openbao.Client, auditRepo repository.AuditRepository, usageRepo repository.UsageRepository) *EthSignTransactionHandler {
	return &EthSignTransactionHandler{
		keyRepo:   keyRepo,
		baoClient: baoClient,
		auditRepo: auditRepo,
		usageRepo: usageRepo,
	}
}

// Handle implements the eth_signTransaction JSON-RPC method.
// Signs an Ethereum transaction and returns the RLP-encoded signed transaction.
func (h *EthSignTransactionHandler) Handle(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
	// Get org ID from context
	orgID := middleware.GetOrgIDFromContext(ctx)
	if orgID.String() == "00000000-0000-0000-0000-000000000000" {
		return nil, ErrUnauthorized("missing organization context")
	}

	// Parse transaction arguments
	var args []ethereum.TransactionArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, ErrInvalidParams(err.Error())
	}
	if len(args) == 0 {
		return nil, ErrInvalidParams("transaction arguments required")
	}

	txArgs := args[0]

	// Validate transaction
	if err := txArgs.Validate(); err != nil {
		return nil, ErrInvalidParams(err.Error())
	}

	// Lookup key by from address
	fromAddr := ethereum.EncodeAddress(*txArgs.From)
	key, err := h.keyRepo.GetByEthAddress(ctx, orgID, fromAddr)
	if err != nil {
		return nil, ErrInternal(fmt.Sprintf("failed to lookup key: %v", err))
	}
	if key == nil {
		return nil, ErrResourceNotFound(fmt.Sprintf("no key found for address %s", fromAddr))
	}

	// Determine transaction type and construct unsigned transaction
	var unsignedTx *ethereum.UnsignedTransaction
	if txArgs.MaxFeePerGas != nil {
		// EIP-1559 transaction
		unsignedTx = ethereum.NewEIP1559Transaction(&txArgs)
	} else {
		// Legacy transaction
		unsignedTx = ethereum.NewLegacyTransaction(&txArgs)
	}

	// Compute transaction hash for signing
	chainID := txArgs.ChainID.ToBig()
	txHash := unsignedTx.SigningHash(chainID)

	// Sign via OpenBao
	// For EIP-1559 (type 2) transactions, v should be just 0 or 1 (yParity)
	// Pass chainID=0 to get raw recovery ID, not EIP-155 encoded v
	hashB64 := base64.StdEncoding.EncodeToString(txHash)
	signChainID := chainID.Int64()
	if unsignedTx.Type == ethereum.EIP1559TxType {
		// EIP-1559 uses raw yParity (0 or 1), not EIP-155 encoded v
		signChainID = 0
	}
	signResp, err := h.baoClient.SignEVM(key.BaoKeyPath, hashB64, signChainID)
	if err != nil {
		return nil, ErrSigningFailed(err.Error())
	}

	// Parse v, r, s from response
	v, r, s, parseErr := parseSignatureResponse(signResp)
	if parseErr != nil {
		return nil, ErrInternal(fmt.Sprintf("failed to parse signature: %v", parseErr))
	}

	// For legacy (chainID=0) signing, v is 27 or 28
	// Convert to yParity (0 or 1) for EIP-1559
	if unsignedTx.Type == ethereum.EIP1559TxType && v.Int64() >= 27 {
		v = big.NewInt(v.Int64() - 27)
	}

	// Construct signed transaction
	signedTx := unsignedTx.WithSignature(v, r, s)

	// RLP encode
	encodedTx, encodeErr := signedTx.EncodeRLP()
	if encodeErr != nil {
		return nil, ErrInternal(fmt.Sprintf("failed to encode transaction: %v", encodeErr))
	}

	// Log audit and increment usage asynchronously
	go h.recordSignature(orgID, key.ID)

	// Return hex-encoded signed transaction
	return ethereum.EncodeBytes(encodedTx), nil
}

// recordSignature logs the signing operation and increments usage counters.
func (h *EthSignTransactionHandler) recordSignature(orgID, keyID uuid.UUID) {
	ctx := context.Background()

	// Create audit log
	if h.auditRepo != nil {
		resourceType := models.ResourceTypeKey
		_ = h.auditRepo.Create(ctx, &models.AuditLog{
			ID:           uuid.New(),
			OrgID:        orgID,
			Event:        models.AuditEventKeySigned,
			ActorType:    models.ActorTypeAPIKey,
			ResourceType: &resourceType,
			ResourceID:   &keyID,
		})
	}

	// Increment signature usage
	if h.usageRepo != nil {
		_ = h.usageRepo.Increment(ctx, orgID, "signatures", 1)
	}
}

// parseSignatureResponse parses v, r, s from OpenBao sign-evm response.
func parseSignatureResponse(resp *openbao.SignEVMResponse) (*big.Int, *big.Int, *big.Int, error) {
	v := new(big.Int)
	v.SetInt64(resp.VInt)

	r := new(big.Int)
	rBytes, err := ethereum.DecodeBytes("0x" + resp.R)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode r: %w", err)
	}
	r.SetBytes(rBytes)

	s := new(big.Int)
	sBytes, err := ethereum.DecodeBytes("0x" + resp.S)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode s: %w", err)
	}
	s.SetBytes(sBytes)

	return v, r, s, nil
}

