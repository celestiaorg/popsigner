package signer

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
)

// EthereumSigner provides ECDSA signing functionality for Ethereum.
type EthereumSigner struct{}

// NewEthereumSigner creates a new Ethereum signer.
func NewEthereumSigner() *EthereumSigner {
	return &EthereumSigner{}
}

// SignHash signs a 32-byte hash with the given private key.
// Returns a 65-byte signature with v value adjusted to 27/28 for Ethereum compatibility.
func (s *EthereumSigner) SignHash(hash []byte, privateKey *ecdsa.PrivateKey) ([]byte, error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("hash must be 32 bytes, got %d", len(hash))
	}

	// Sign the hash using go-ethereum's crypto library
	// This returns a [R || S || V] signature where V is 0 or 1
	signature, err := crypto.Sign(hash, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign hash: %w", err)
	}

	// Ethereum expects V to be 27 or 28 (not 0 or 1)
	// The signature from crypto.Sign has V at index 64
	signature[64] += 27

	return signature, nil
}

// SignHashRaw signs a 32-byte hash and returns the raw signature with V as 0 or 1.
// This is useful for EIP-1559 transactions which use yParity instead of V.
func (s *EthereumSigner) SignHashRaw(hash []byte, privateKey *ecdsa.PrivateKey) ([]byte, error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("hash must be 32 bytes, got %d", len(hash))
	}

	// Sign the hash using go-ethereum's crypto library
	// Returns [R || S || V] where V is 0 or 1 (yParity)
	signature, err := crypto.Sign(hash, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign hash: %w", err)
	}

	return signature, nil
}

// RecoverPublicKey recovers the public key from a signature and hash.
// This is useful for verifying signatures.
func (s *EthereumSigner) RecoverPublicKey(hash []byte, signature []byte) (*ecdsa.PublicKey, error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("hash must be 32 bytes, got %d", len(hash))
	}
	if len(signature) != 65 {
		return nil, fmt.Errorf("signature must be 65 bytes, got %d", len(signature))
	}

	// Create a copy of the signature to avoid modifying the original
	sig := make([]byte, 65)
	copy(sig, signature)

	// Normalize V value to 0 or 1 for recovery
	if sig[64] >= 27 {
		sig[64] -= 27
	}

	pubKey, err := crypto.SigToPub(hash, sig)
	if err != nil {
		return nil, fmt.Errorf("failed to recover public key: %w", err)
	}

	return pubKey, nil
}

// VerifySignature verifies that a signature was created by the given address.
func (s *EthereumSigner) VerifySignature(hash []byte, signature []byte, expectedAddress string) (bool, error) {
	pubKey, err := s.RecoverPublicKey(hash, signature)
	if err != nil {
		return false, err
	}

	recoveredAddr := crypto.PubkeyToAddress(*pubKey).Hex()
	return recoveredAddr == expectedAddress, nil
}
