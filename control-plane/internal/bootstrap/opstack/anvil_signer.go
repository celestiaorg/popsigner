// Package opstack provides OP Stack chain deployment infrastructure.
package opstack

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	opcrypto "github.com/ethereum-optimism/optimism/op-service/crypto"
)

// AnvilPrivateKeys contains Anvil's 10 deterministic private keys.
// These are derived from the default mnemonic:
// "test test test test test test test test test test test junk"
//
// ⚠️  SECURITY WARNING - DO NOT USE IN PRODUCTION ⚠️
// These keys are PUBLICLY KNOWN and included in Foundry's Anvil tool.
// Any funds sent to these addresses on real networks WILL BE STOLEN.
// Use ONLY for local development and testing with Anvil.
//
// NewAnvilSigner enforces protection by refusing to sign for production networks.
var AnvilPrivateKeys = []string{
	"ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80", // anvil-0: 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266
	"59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d", // anvil-1: 0x70997970C51812dc3A010C7d01b50e0d17dc79C8
	"5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a", // anvil-2: 0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC
	"7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6", // anvil-3: 0x90F79bf6EB2c4f870365E785982E1f101E93b906
	"47e179ec197488593b187f80a00eb0da91f1b9d0b13f8733639f19c30a34926a", // anvil-4: 0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65
	"8b3a350cf5c34c9194ca85829a2df0ec3153be0318b5e2d3348e872092edffba", // anvil-5: 0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc
	"92db14e403b83dfe3df233f83dfa3a0d7096f21ca9b0d6d6b8d88b2b4ec1564e", // anvil-6: 0x976EA74026E726554dB657fA54763abd0C3a0aa9
	"4bbbf85ce3377467afe5d46f804f221813b2bb87f24d81f60f1fcdbf7cbf4356", // anvil-7: 0x14dC79964da2C08b23698B3D3cc7Ca32193d9955
	"dbda1821b80551c9d65939329250298aa3472ba22feea921c0cf5d620ea67b97", // anvil-8: 0x23618e81E3f5cdF7f54C3d65f7FBc0aBf5B21E8f
	"2a871d0798f97d79848a013d4936a73bf4cc922c825d33c1cf7073dff6d409c6", // anvil-9: 0xa0Ee7A142d267C1f36714E4a8F75612F20a79720
}

// AnvilSigner signs transactions using Anvil's well-known private keys.
//
// ⚠️  WARNING: Use ONLY for local Anvil deployments on test networks.
// These keys are publicly known and must never be used in production.
//
// Safe for concurrent use after construction.
type AnvilSigner struct {
	keys    map[common.Address]*ecdsa.PrivateKey
	chainID *big.Int
}

// NewAnvilSigner creates a signer pre-loaded with all Anvil deterministic keys.
//
// Returns error if chainID corresponds to a production network (mainnet or major L2s).
func NewAnvilSigner(chainID *big.Int) (*AnvilSigner, error) {
	// Prevent accidental use on production networks
	productionChainIDs := map[int64]string{
		1:     "Ethereum Mainnet",
		10:    "Optimism",
		42161: "Arbitrum One",
		137:   "Polygon",
		8453:  "Base",
	}

	if chainName, isProduction := productionChainIDs[chainID.Int64()]; isProduction {
		return nil, fmt.Errorf("anvil signer cannot be used on %s (chain_id=%s): keys are publicly known", chainName, chainID)
	}

	keys := make(map[common.Address]*ecdsa.PrivateKey, len(AnvilPrivateKeys))

	for _, hexKey := range AnvilPrivateKeys {
		privateKey, err := crypto.HexToECDSA(hexKey)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		address := crypto.PubkeyToAddress(privateKey.PublicKey)
		keys[address] = privateKey
	}

	return &AnvilSigner{keys: keys, chainID: chainID}, nil
}

// SignerFn returns an opcrypto.SignerFn compatible with op-deployer.
// This is the main entry point for transaction signing in the op-deployer pipeline.
func (s *AnvilSigner) SignerFn() opcrypto.SignerFn {
	return func(ctx context.Context, addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
		privateKey, ok := s.keys[addr]
		if !ok {
			return nil, fmt.Errorf("no private key for address %s (not an Anvil account)", addr.Hex())
		}

		signer := types.LatestSignerForChainID(s.chainID)
		signedTx, err := types.SignTx(tx, signer, privateKey)
		if err != nil {
			return nil, fmt.Errorf("sign transaction: %w", err)
		}

		return signedTx, nil
	}
}

// HasKey returns true if the signer has a private key for the given address.
func (s *AnvilSigner) HasKey(addr common.Address) bool {
	_, ok := s.keys[addr]
	return ok
}

// Addresses returns all addresses this signer can sign for.
func (s *AnvilSigner) Addresses() []common.Address {
	addrs := make([]common.Address, 0, len(s.keys))
	for addr := range s.keys {
		addrs = append(addrs, addr)
	}
	return addrs
}
