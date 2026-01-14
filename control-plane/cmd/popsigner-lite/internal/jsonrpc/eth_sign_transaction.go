package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/keystore"
	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/signer"
)

// EthSignTransactionHandler handles eth_signTransaction requests.
type EthSignTransactionHandler struct {
	keystore *keystore.Keystore
	signer   *signer.TransactionSigner
}

// NewEthSignTransactionHandler creates a new eth_signTransaction handler.
func NewEthSignTransactionHandler(ks *keystore.Keystore, s *signer.TransactionSigner) *EthSignTransactionHandler {
	return &EthSignTransactionHandler{
		keystore: ks,
		signer:   s,
	}
}

// TransactionArgs represents the arguments for an Ethereum transaction.
type TransactionArgs struct {
	From                 *common.Address `json:"from"`
	To                   *common.Address `json:"to"`
	Gas                  *hexutil.Uint64 `json:"gas"`
	GasPrice             *hexutil.Big    `json:"gasPrice"`
	MaxFeePerGas         *hexutil.Big    `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.Big    `json:"maxPriorityFeePerGas"`
	Value                *hexutil.Big    `json:"value"`
	Nonce                *hexutil.Uint64 `json:"nonce"`
	Data                 *hexutil.Bytes  `json:"data"`
	Input                *hexutil.Bytes  `json:"input"`
	ChainID              *hexutil.Big    `json:"chainId"`
}

// Handle implements the eth_signTransaction JSON-RPC method.
// Signs an Ethereum transaction and returns the RLP-encoded signed transaction.
func (h *EthSignTransactionHandler) Handle(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
	// Parse transaction arguments
	var args []TransactionArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, ErrInvalidParams(fmt.Sprintf("failed to parse params: %v", err))
	}
	if len(args) == 0 {
		return nil, ErrInvalidParams("transaction arguments required")
	}

	txArgs := args[0]

	// Validate required fields
	if txArgs.From == nil {
		return nil, ErrInvalidParams("from address is required")
	}
	if txArgs.ChainID == nil {
		return nil, ErrInvalidParams("chainId is required")
	}

	// Normalize from address to lowercase with 0x prefix
	fromAddr := strings.ToLower(txArgs.From.Hex())

	// Lookup key by from address
	key, err := h.keystore.GetKey(fromAddr)
	if err != nil {
		// Try without case sensitivity
		keys := h.keystore.ListKeys()
		for _, k := range keys {
			if strings.EqualFold(k.Address, fromAddr) {
				key = k
				break
			}
		}
		if key == nil {
			return nil, ErrKeyNotFound(fmt.Sprintf("no key found for address %s", fromAddr))
		}
	}

	// Get transaction data (prefer input over data)
	var txData []byte
	if txArgs.Input != nil {
		txData = *txArgs.Input
	} else if txArgs.Data != nil {
		txData = *txArgs.Data
	}

	// Get value (default to 0)
	value := big.NewInt(0)
	if txArgs.Value != nil {
		value = txArgs.Value.ToInt()
	}

	// Get nonce (default to 0 if not provided)
	nonce := uint64(0)
	if txArgs.Nonce != nil {
		nonce = uint64(*txArgs.Nonce)
	}

	// Get gas limit (default to 21000 if not provided)
	gasLimit := uint64(21000)
	if txArgs.Gas != nil {
		gasLimit = uint64(*txArgs.Gas)
	}

	chainID := txArgs.ChainID.ToInt()

	// Determine transaction type and build transaction
	var tx *types.Transaction
	if txArgs.MaxFeePerGas != nil {
		// EIP-1559 transaction
		maxFeePerGas := txArgs.MaxFeePerGas.ToInt()
		maxPriorityFeePerGas := big.NewInt(0)
		if txArgs.MaxPriorityFeePerGas != nil {
			maxPriorityFeePerGas = txArgs.MaxPriorityFeePerGas.ToInt()
		}

		if txArgs.To == nil {
			// Contract deployment
			tx = types.NewTx(&types.DynamicFeeTx{
				ChainID:   chainID,
				Nonce:     nonce,
				GasTipCap: maxPriorityFeePerGas,
				GasFeeCap: maxFeePerGas,
				Gas:       gasLimit,
				To:        nil,
				Value:     value,
				Data:      txData,
			})
		} else {
			// Regular transaction
			tx = types.NewTx(&types.DynamicFeeTx{
				ChainID:   chainID,
				Nonce:     nonce,
				GasTipCap: maxPriorityFeePerGas,
				GasFeeCap: maxFeePerGas,
				Gas:       gasLimit,
				To:        txArgs.To,
				Value:     value,
				Data:      txData,
			})
		}
	} else {
		// Legacy transaction
		gasPrice := big.NewInt(0)
		if txArgs.GasPrice != nil {
			gasPrice = txArgs.GasPrice.ToInt()
		}

		if txArgs.To == nil {
			// Contract deployment
			tx = types.NewContractCreation(nonce, value, gasLimit, gasPrice, txData)
		} else {
			// Regular transaction
			tx = types.NewTransaction(nonce, *txArgs.To, value, gasLimit, gasPrice, txData)
		}
	}

	// Sign the transaction
	signedTxBytes, err := h.signer.SignTransaction(tx, key.PrivateKey, chainID)
	if err != nil {
		return nil, ErrSigningFailed(fmt.Sprintf("failed to sign transaction: %v", err))
	}

	// Return hex-encoded signed transaction
	return hexutil.Encode(signedTxBytes), nil
}
