package signer

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// TransactionSigner handles signing Ethereum transactions (legacy and EIP-1559).
type TransactionSigner struct {
	signer *EthereumSigner
}

// NewTransactionSigner creates a new transaction signer.
func NewTransactionSigner() *TransactionSigner {
	return &TransactionSigner{
		signer: NewEthereumSigner(),
	}
}

// SignTransaction signs an Ethereum transaction with the given private key and chain ID.
// Supports both legacy and EIP-1559 (type 2) transactions.
// Returns the RLP-encoded signed transaction bytes.
func (s *TransactionSigner) SignTransaction(tx *types.Transaction, privateKey *ecdsa.PrivateKey, chainID *big.Int) ([]byte, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is nil")
	}
	if privateKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	if chainID == nil {
		return nil, fmt.Errorf("chain ID is nil")
	}

	// Create the appropriate signer for the chain ID
	signer := types.LatestSignerForChainID(chainID)

	// Sign the transaction
	signedTx, err := types.SignTx(tx, signer, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Encode the signed transaction to RLP format
	encodedTx, err := signedTx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to encode signed transaction: %w", err)
	}

	return encodedTx, nil
}

// SignLegacyTransaction creates and signs a legacy (pre-EIP-1559) transaction.
func (s *TransactionSigner) SignLegacyTransaction(
	nonce uint64,
	to *common.Address,
	value *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	data []byte,
	privateKey *ecdsa.PrivateKey,
	chainID *big.Int,
) ([]byte, error) {
	tx := types.NewTransaction(nonce, *to, value, gasLimit, gasPrice, data)
	return s.SignTransaction(tx, privateKey, chainID)
}

// SignEIP1559Transaction creates and signs an EIP-1559 (type 2) transaction.
func (s *TransactionSigner) SignEIP1559Transaction(
	nonce uint64,
	to *common.Address,
	value *big.Int,
	gasLimit uint64,
	gasTipCap *big.Int,
	gasFeeCap *big.Int,
	data []byte,
	privateKey *ecdsa.PrivateKey,
	chainID *big.Int,
) ([]byte, error) {
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       gasLimit,
		To:        to,
		Value:     value,
		Data:      data,
	})
	return s.SignTransaction(tx, privateKey, chainID)
}

// SignContractDeployment signs a transaction that deploys a contract.
func (s *TransactionSigner) SignContractDeployment(
	nonce uint64,
	value *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	bytecode []byte,
	privateKey *ecdsa.PrivateKey,
	chainID *big.Int,
) ([]byte, error) {
	tx := types.NewContractCreation(nonce, value, gasLimit, gasPrice, bytecode)
	return s.SignTransaction(tx, privateKey, chainID)
}

// SignEIP1559ContractDeployment signs an EIP-1559 transaction that deploys a contract.
func (s *TransactionSigner) SignEIP1559ContractDeployment(
	nonce uint64,
	value *big.Int,
	gasLimit uint64,
	gasTipCap *big.Int,
	gasFeeCap *big.Int,
	bytecode []byte,
	privateKey *ecdsa.PrivateKey,
	chainID *big.Int,
) ([]byte, error) {
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       gasLimit,
		To:        nil, // nil means contract deployment
		Value:     value,
		Data:      bytecode,
	})
	return s.SignTransaction(tx, privateKey, chainID)
}

// GetTransactionSigner returns the appropriate signer for a chain ID.
// This is useful for manual transaction signing operations.
func (s *TransactionSigner) GetTransactionSigner(chainID *big.Int) types.Signer {
	return types.LatestSignerForChainID(chainID)
}

// ExtractSignature extracts the V, R, S values from a signed transaction.
func (s *TransactionSigner) ExtractSignature(signedTx *types.Transaction) (v, r, sVal *big.Int, err error) {
	v, r, sVal = signedTx.RawSignatureValues()
	if v == nil || r == nil || sVal == nil {
		return nil, nil, nil, fmt.Errorf("transaction is not signed")
	}
	return v, r, sVal, nil
}
