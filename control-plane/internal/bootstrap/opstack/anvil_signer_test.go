package opstack

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnvilSigner(t *testing.T) {
	signer, err := NewAnvilSigner(big.NewInt(31337))
	require.NoError(t, err)
	require.NotNil(t, signer)

	// Should have all 10 Anvil accounts
	addrs := signer.Addresses()
	assert.Len(t, addrs, 10)

	// Verify anvil-0 address is present
	anvil0 := common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	assert.True(t, signer.HasKey(anvil0), "should have anvil-0 key")
}

func TestAnvilSignerSignTransaction(t *testing.T) {
	chainID := big.NewInt(31337)
	signer, err := NewAnvilSigner(chainID)
	require.NoError(t, err)

	// Create a simple transaction
	anvil0 := common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	to := common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     0,
		GasTipCap: big.NewInt(1e9),
		GasFeeCap: big.NewInt(10e9),
		Gas:       21000,
		To:        &to,
		Value:     big.NewInt(1e18),
	})

	// Sign the transaction
	signerFn := signer.SignerFn()
	signedTx, err := signerFn(context.Background(), anvil0, tx)
	require.NoError(t, err)
	require.NotNil(t, signedTx)

	// Verify the signature recovers to the correct address
	ethSigner := types.LatestSignerForChainID(chainID)
	from, err := types.Sender(ethSigner, signedTx)
	require.NoError(t, err)
	assert.Equal(t, anvil0, from, "recovered address should match signer address")
}

func TestAnvilSignerUnknownAddress(t *testing.T) {
	signer, err := NewAnvilSigner(big.NewInt(31337))
	require.NoError(t, err)

	// Try to sign with an unknown address
	unknownAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1e9),
		Gas:      21000,
		Value:    big.NewInt(1e18),
	})

	signerFn := signer.SignerFn()
	_, err = signerFn(context.Background(), unknownAddr, tx)
	assert.Error(t, err, "should error for unknown address")
	assert.Contains(t, err.Error(), "no private key for address")
}

func TestAnvilSignerAllAccounts(t *testing.T) {
	signer, err := NewAnvilSigner(big.NewInt(31337))
	require.NoError(t, err)

	// Expected Anvil addresses (derived from mnemonic)
	expectedAddresses := []string{
		"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", // anvil-0
		"0x70997970C51812dc3A010C7d01b50e0d17dc79C8", // anvil-1
		"0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC", // anvil-2
		"0x90F79bf6EB2c4f870365E785982E1f101E93b906", // anvil-3
		"0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65", // anvil-4
		"0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc", // anvil-5
		"0x976EA74026E726554dB657fA54763abd0C3a0aa9", // anvil-6
		"0x14dC79964da2C08b23698B3D3cc7Ca32193d9955", // anvil-7
		"0x23618e81E3f5cdF7f54C3d65f7FBc0aBf5B21E8f", // anvil-8
		"0xa0Ee7A142d267C1f36714E4a8F75612F20a79720", // anvil-9
	}

	for i, addrStr := range expectedAddresses {
		addr := common.HexToAddress(addrStr)
		assert.True(t, signer.HasKey(addr), "should have anvil-%d key: %s", i, addrStr)
	}
}
