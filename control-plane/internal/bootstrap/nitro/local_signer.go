// Package nitro provides Nitro chain deployment infrastructure.
package nitro

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// LocalSigner implements TransactionSigner for local testing with a private key.
// Use this for testing on Anvil or other local development networks.
// DO NOT use in production - private keys should be managed by POPSigner.
type LocalSigner struct {
	privateKey *ecdsa.PrivateKey
	address    common.Address
	chainID    *big.Int
}

// NewLocalSigner creates a new LocalSigner from a hex-encoded private key.
// The key should NOT have a "0x" prefix.
func NewLocalSigner(hexKey string, chainID int64) (*LocalSigner, error) {
	privateKey, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	publicKey, ok := privateKey.Public().(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("failed to get public key")
	}
	address := crypto.PubkeyToAddress(*publicKey)

	return &LocalSigner{
		privateKey: privateKey,
		address:    address,
		chainID:    big.NewInt(chainID),
	}, nil
}

// Address returns the signer's Ethereum address.
func (s *LocalSigner) Address() common.Address {
	return s.address
}

// ChainID returns the chain ID for transaction signing.
func (s *LocalSigner) ChainID() *big.Int {
	return s.chainID
}

// SignTransaction signs a transaction using the local private key.
func (s *LocalSigner) SignTransaction(ctx context.Context, tx *types.Transaction) (*types.Transaction, error) {
	signer := types.LatestSignerForChainID(s.chainID)
	signedTx, err := types.SignTx(tx, signer, s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}
	return signedTx, nil
}

// Ensure LocalSigner implements TransactionSigner.
var _ TransactionSigner = (*LocalSigner)(nil)
