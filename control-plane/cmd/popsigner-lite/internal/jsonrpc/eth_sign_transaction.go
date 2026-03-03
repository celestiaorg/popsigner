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
// Fields mirror go-ethereum's internal/ethapi.TransactionArgs for compatibility.
type TransactionArgs struct {
	From                 *common.Address   `json:"from"`
	To                   *common.Address   `json:"to"`
	Gas                  *hexutil.Uint64   `json:"gas"`
	GasPrice             *hexutil.Big      `json:"gasPrice"`
	MaxFeePerGas         *hexutil.Big      `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.Big      `json:"maxPriorityFeePerGas"`
	Value                *hexutil.Big      `json:"value"`
	Nonce                *hexutil.Uint64   `json:"nonce"`
	Data                 *hexutil.Bytes    `json:"data"`
	Input                *hexutil.Bytes    `json:"input"`
	AccessList           *types.AccessList `json:"accessList,omitempty"`
	ChainID              *hexutil.Big      `json:"chainId"`
	Type                 *hexutil.Uint64   `json:"type,omitempty"`
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

	// Resolve access list (nil → empty slice for type inference)
	var accessList types.AccessList
	if txArgs.AccessList != nil {
		accessList = *txArgs.AccessList
	}

	// Determine transaction type.
	// Priority: explicit type > maxFeePerGas (EIP-1559) > accessList only (EIP-2930) > legacy.
	// Note: an EIP-1559 tx (type 2) may include an accessList — that doesn't make it type 1.
	// Type 1 (EIP-2930) is only inferred when accessList is set but maxFeePerGas is not.
	txType := uint64(0)
	if txArgs.Type != nil {
		txType = uint64(*txArgs.Type)
		switch txType {
		case types.DynamicFeeTxType:
			if txArgs.MaxFeePerGas == nil || txArgs.MaxPriorityFeePerGas == nil {
				return nil, ErrInvalidParams("type 0x2 requires maxFeePerGas and maxPriorityFeePerGas")
			}
		case types.AccessListTxType:
			if txArgs.GasPrice == nil {
				return nil, ErrInvalidParams("type 0x1 requires gasPrice")
			}
		case types.LegacyTxType:
			if txArgs.GasPrice == nil {
				return nil, ErrInvalidParams("type 0x0 requires gasPrice")
			}
		default:
			return nil, ErrInvalidParams(fmt.Sprintf("unsupported transaction type: 0x%x", txType))
		}
	} else if txArgs.MaxFeePerGas != nil {
		txType = types.DynamicFeeTxType
	} else if txArgs.AccessList != nil {
		txType = types.AccessListTxType
	}

	// Determine transaction type and build transaction
	var tx *types.Transaction
	switch txType {
	case types.DynamicFeeTxType:
		// EIP-1559 transaction. To==nil means contract creation, which go-ethereum handles correctly.
		maxFeePerGas := big.NewInt(0)
		if txArgs.MaxFeePerGas != nil {
			maxFeePerGas = txArgs.MaxFeePerGas.ToInt()
		}
		maxPriorityFeePerGas := big.NewInt(0)
		if txArgs.MaxPriorityFeePerGas != nil {
			maxPriorityFeePerGas = txArgs.MaxPriorityFeePerGas.ToInt()
		}
		tx = types.NewTx(&types.DynamicFeeTx{
			ChainID:    chainID,
			Nonce:      nonce,
			GasTipCap:  maxPriorityFeePerGas,
			GasFeeCap:  maxFeePerGas,
			Gas:        gasLimit,
			To:         txArgs.To, // nil == contract creation
			Value:      value,
			Data:       txData,
			AccessList: accessList,
		})

	case types.AccessListTxType:
		// EIP-2930 transaction. To==nil means contract creation, which go-ethereum handles correctly.
		gasPrice := big.NewInt(0)
		if txArgs.GasPrice != nil {
			gasPrice = txArgs.GasPrice.ToInt()
		}
		tx = types.NewTx(&types.AccessListTx{
			ChainID:    chainID,
			Nonce:      nonce,
			GasPrice:   gasPrice,
			Gas:        gasLimit,
			To:         txArgs.To, // nil == contract creation
			Value:      value,
			Data:       txData,
			AccessList: accessList,
		})

	default:
		// Legacy transaction (type 0)
		gasPrice := big.NewInt(0)
		if txArgs.GasPrice != nil {
			gasPrice = txArgs.GasPrice.ToInt()
		}
		if txArgs.To == nil {
			tx = types.NewContractCreation(nonce, value, gasLimit, gasPrice, txData)
		} else {
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
