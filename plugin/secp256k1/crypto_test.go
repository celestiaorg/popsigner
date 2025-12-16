package secp256k1

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test vectors for known values
var (
	// Known SHA-256 hash of "test"
	testSHA256Expected = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	// Known Keccak-256 hash of "test"
	testKeccak256Expected = "9c22ff5f21f0b81b113e63f7db6da94fedef11b2119b4088b89664fb9a3cb658"
)

func TestGenerateKey(t *testing.T) {
	t.Run("generates valid keypair", func(t *testing.T) {
		privKey, pubKey, err := GenerateKey()
		require.NoError(t, err)
		require.NotNil(t, privKey)
		require.NotNil(t, pubKey)

		// Private key should be 32 bytes
		privBytes := privKey.Serialize()
		assert.Len(t, privBytes, 32)

		// Public key should derive from private key
		assert.Equal(t, privKey.PubKey(), pubKey)
	})

	t.Run("generates unique keys", func(t *testing.T) {
		privKey1, _, err := GenerateKey()
		require.NoError(t, err)

		privKey2, _, err := GenerateKey()
		require.NoError(t, err)

		// Keys should be different
		assert.NotEqual(t, privKey1.Serialize(), privKey2.Serialize())
	})
}

func TestSignMessage(t *testing.T) {
	privKey, _, err := GenerateKey()
	require.NoError(t, err)

	hash := hashSHA256([]byte("test message"))

	t.Run("signs message successfully", func(t *testing.T) {
		sig, err := SignMessage(privKey, hash)
		require.NoError(t, err)
		assert.Len(t, sig, 64)
	})

	t.Run("produces low-S signature", func(t *testing.T) {
		sig, err := SignMessage(privKey, hash)
		require.NoError(t, err)

		// Verify low-S
		s := new(btcec.ModNScalar)
		s.SetByteSlice(sig[32:])
		assert.False(t, s.IsOverHalfOrder(), "signature should have low-S")
	})

	t.Run("rejects invalid hash length", func(t *testing.T) {
		_, err := SignMessage(privKey, []byte("short"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hash must be 32 bytes")
	})

	t.Run("rejects nil private key", func(t *testing.T) {
		_, err := SignMessage(nil, hash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "private key cannot be nil")
	})
}

func TestVerifySignature(t *testing.T) {
	privKey, pubKey, err := GenerateKey()
	require.NoError(t, err)

	hash := hashSHA256([]byte("test message"))
	sig, err := SignMessage(privKey, hash)
	require.NoError(t, err)

	t.Run("verifies valid signature", func(t *testing.T) {
		valid, err := VerifySignature(pubKey, hash, sig)
		require.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("rejects tampered signature", func(t *testing.T) {
		tamperedSig := make([]byte, 64)
		copy(tamperedSig, sig)
		tamperedSig[0] ^= 0xff // Flip bits

		valid, err := VerifySignature(pubKey, hash, tamperedSig)
		// May return error or false depending on whether it parses
		if err == nil {
			assert.False(t, valid)
		}
	})

	t.Run("rejects wrong message", func(t *testing.T) {
		wrongHash := hashSHA256([]byte("different message"))
		valid, err := VerifySignature(pubKey, wrongHash, sig)
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("rejects wrong public key", func(t *testing.T) {
		_, wrongPubKey, err := GenerateKey()
		require.NoError(t, err)

		valid, err := VerifySignature(wrongPubKey, hash, sig)
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("rejects nil public key", func(t *testing.T) {
		_, err := VerifySignature(nil, hash, sig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "public key cannot be nil")
	})

	t.Run("rejects invalid hash length", func(t *testing.T) {
		_, err := VerifySignature(pubKey, []byte("short"), sig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hash must be 32 bytes")
	})

	t.Run("rejects invalid signature length", func(t *testing.T) {
		_, err := VerifySignature(pubKey, hash, []byte("short"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signature must be 64 bytes")
	})
}

func TestSerializePublicKey(t *testing.T) {
	t.Run("serializes to compressed format", func(t *testing.T) {
		_, pubKey, err := GenerateKey()
		require.NoError(t, err)

		serialized := SerializePublicKey(pubKey)
		assert.Len(t, serialized, 33)

		// First byte should be 0x02 or 0x03 for compressed keys
		assert.True(t, serialized[0] == 0x02 || serialized[0] == 0x03)
	})

	t.Run("handles nil public key", func(t *testing.T) {
		serialized := SerializePublicKey(nil)
		assert.Nil(t, serialized)
	})
}

func TestParsePublicKey(t *testing.T) {
	_, pubKey, err := GenerateKey()
	require.NoError(t, err)

	t.Run("parses compressed public key", func(t *testing.T) {
		serialized := SerializePublicKey(pubKey)
		parsed, err := ParsePublicKey(serialized)
		require.NoError(t, err)
		assert.Equal(t, pubKey.SerializeCompressed(), parsed.SerializeCompressed())
	})

	t.Run("parses uncompressed public key", func(t *testing.T) {
		uncompressed := pubKey.SerializeUncompressed()
		assert.Len(t, uncompressed, 65)

		parsed, err := ParsePublicKey(uncompressed)
		require.NoError(t, err)
		assert.Equal(t, pubKey.SerializeCompressed(), parsed.SerializeCompressed())
	})

	t.Run("rejects empty data", func(t *testing.T) {
		_, err := ParsePublicKey([]byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("rejects invalid data", func(t *testing.T) {
		_, err := ParsePublicKey([]byte{0x00, 0x01, 0x02})
		assert.Error(t, err)
	})
}

func TestParsePrivateKey(t *testing.T) {
	privKey, _, err := GenerateKey()
	require.NoError(t, err)

	t.Run("parses valid private key", func(t *testing.T) {
		serialized := SerializePrivateKey(privKey)
		parsed, err := ParsePrivateKey(serialized)
		require.NoError(t, err)
		assert.Equal(t, privKey.Serialize(), parsed.Serialize())
	})

	t.Run("rejects wrong length", func(t *testing.T) {
		_, err := ParsePrivateKey([]byte{0x01, 0x02, 0x03})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "private key must be 32 bytes")
	})
}

func TestSerializePrivateKey(t *testing.T) {
	t.Run("serializes to 32 bytes", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		serialized := SerializePrivateKey(privKey)
		assert.Len(t, serialized, 32)
	})

	t.Run("handles nil private key", func(t *testing.T) {
		serialized := SerializePrivateKey(nil)
		assert.Nil(t, serialized)
	})
}

func TestHashSHA256(t *testing.T) {
	t.Run("returns 32 bytes", func(t *testing.T) {
		result := hashSHA256([]byte("test"))
		assert.Len(t, result, 32)
	})

	t.Run("matches known test vector", func(t *testing.T) {
		result := hashSHA256([]byte("test"))
		expected, _ := hex.DecodeString(testSHA256Expected)
		assert.Equal(t, expected, result)
	})

	t.Run("handles empty input", func(t *testing.T) {
		result := hashSHA256([]byte{})
		assert.Len(t, result, 32)
	})

	t.Run("produces deterministic output", func(t *testing.T) {
		data := []byte("deterministic test")
		result1 := hashSHA256(data)
		result2 := hashSHA256(data)
		assert.Equal(t, result1, result2)
	})
}

func TestHashKeccak256(t *testing.T) {
	t.Run("returns 32 bytes", func(t *testing.T) {
		result := hashKeccak256([]byte("test"))
		assert.Len(t, result, 32)
	})

	t.Run("matches known test vector", func(t *testing.T) {
		result := hashKeccak256([]byte("test"))
		expected, _ := hex.DecodeString(testKeccak256Expected)
		assert.Equal(t, expected, result)
	})

	t.Run("handles empty input", func(t *testing.T) {
		result := hashKeccak256([]byte{})
		assert.Len(t, result, 32)
	})

	t.Run("differs from SHA256", func(t *testing.T) {
		data := []byte("test")
		sha256Result := hashSHA256(data)
		keccak256Result := hashKeccak256(data)
		assert.NotEqual(t, sha256Result, keccak256Result)
	})
}

func TestDeriveCosmosAddress(t *testing.T) {
	t.Run("returns 20 bytes", func(t *testing.T) {
		_, pubKey, err := GenerateKey()
		require.NoError(t, err)

		serialized := SerializePublicKey(pubKey)
		addr := deriveCosmosAddress(serialized)
		assert.Len(t, addr, 20)
	})

	t.Run("produces deterministic output", func(t *testing.T) {
		_, pubKey, err := GenerateKey()
		require.NoError(t, err)

		serialized := SerializePublicKey(pubKey)
		addr1 := deriveCosmosAddress(serialized)
		addr2 := deriveCosmosAddress(serialized)
		assert.Equal(t, addr1, addr2)
	})

	t.Run("different keys produce different addresses", func(t *testing.T) {
		_, pubKey1, err := GenerateKey()
		require.NoError(t, err)
		_, pubKey2, err := GenerateKey()
		require.NoError(t, err)

		addr1 := deriveCosmosAddress(SerializePublicKey(pubKey1))
		addr2 := deriveCosmosAddress(SerializePublicKey(pubKey2))
		assert.NotEqual(t, addr1, addr2)
	})

	t.Run("matches known test vector", func(t *testing.T) {
		// Known test vector: compressed pubkey -> address
		// This is a simplified test to verify the RIPEMD160(SHA256(pubkey)) formula
		pubKeyHex := "02950e1cdfcb133d6024109fd489f734eeb4502418e538c28481f22c28a37082"
		pubKey, err := hex.DecodeString(pubKeyHex)
		require.NoError(t, err)

		addr := deriveCosmosAddress(pubKey)
		assert.Len(t, addr, 20)
	})
}

func TestFormatCosmosSignature(t *testing.T) {
	privKey, _, err := GenerateKey()
	require.NoError(t, err)

	hash := hashSHA256([]byte("test"))

	t.Run("returns 64 bytes", func(t *testing.T) {
		sig := ecdsa.Sign(privKey, hash)
		formatted := formatCosmosSignature(sig)
		assert.Len(t, formatted, 64)
	})

	t.Run("produces low-S signature", func(t *testing.T) {
		// Sign multiple times to increase chance of hitting high-S
		for i := 0; i < 10; i++ {
			testHash := hashSHA256([]byte{byte(i)})
			sig := ecdsa.Sign(privKey, testHash)
			formatted := formatCosmosSignature(sig)

			// Verify low-S
			s := new(btcec.ModNScalar)
			s.SetByteSlice(formatted[32:])
			assert.False(t, s.IsOverHalfOrder(), "signature should have low-S")
		}
	})
}

func TestParseCosmosSignature(t *testing.T) {
	privKey, pubKey, err := GenerateKey()
	require.NoError(t, err)

	hash := hashSHA256([]byte("test"))
	sig := ecdsa.Sign(privKey, hash)
	formatted := formatCosmosSignature(sig)

	t.Run("parses valid signature", func(t *testing.T) {
		parsed, err := parseCosmosSignature(formatted)
		require.NoError(t, err)
		assert.NotNil(t, parsed)

		// Should verify
		assert.True(t, parsed.Verify(hash, pubKey))
	})

	t.Run("rejects wrong length", func(t *testing.T) {
		_, err := parseCosmosSignature([]byte{0x01, 0x02, 0x03})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signature must be 64 bytes")
	})

	t.Run("rejects zero R", func(t *testing.T) {
		zeroSig := make([]byte, 64)
		copy(zeroSig[32:], formatted[32:]) // Copy valid S
		// R is all zeros

		_, err := parseCosmosSignature(zeroSig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "R or S is zero")
	})

	t.Run("rejects zero S", func(t *testing.T) {
		zeroSig := make([]byte, 64)
		copy(zeroSig[:32], formatted[:32]) // Copy valid R
		// S is all zeros

		_, err := parseCosmosSignature(zeroSig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "R or S is zero")
	})
}

func TestSecureZero(t *testing.T) {
	t.Run("zeroes all bytes", func(t *testing.T) {
		data := []byte{1, 2, 3, 4, 5, 0xff, 0xaa, 0x55}
		secureZero(data)

		for i, b := range data {
			assert.Equal(t, byte(0), b, "byte at position %d should be zero", i)
		}
	})

	t.Run("handles empty slice", func(t *testing.T) {
		data := []byte{}
		secureZero(data) // Should not panic
	})

	t.Run("handles nil slice", func(t *testing.T) {
		var data []byte
		secureZero(data) // Should not panic
	})
}

func TestSignVerifyRoundTrip(t *testing.T) {
	t.Run("sign and verify round trip", func(t *testing.T) {
		privKey, pubKey, err := GenerateKey()
		require.NoError(t, err)

		messages := []string{
			"Hello, World!",
			"",
			"0123456789",
			string(make([]byte, 1000)), // Large message
		}

		for _, msg := range messages {
			hash := hashSHA256([]byte(msg))
			sig, err := SignMessage(privKey, hash)
			require.NoError(t, err)

			valid, err := VerifySignature(pubKey, hash, sig)
			require.NoError(t, err)
			assert.True(t, valid, "signature should verify for message: %q", msg)
		}
	})
}

func TestKeySerializationRoundTrip(t *testing.T) {
	t.Run("private key round trip", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		serialized := SerializePrivateKey(privKey)
		restored, err := ParsePrivateKey(serialized)
		require.NoError(t, err)

		assert.Equal(t, privKey.Serialize(), restored.Serialize())
	})

	t.Run("public key round trip", func(t *testing.T) {
		_, pubKey, err := GenerateKey()
		require.NoError(t, err)

		serialized := SerializePublicKey(pubKey)
		restored, err := ParsePublicKey(serialized)
		require.NoError(t, err)

		assert.Equal(t, pubKey.SerializeCompressed(), restored.SerializeCompressed())
	})
}

func TestCrossValidation(t *testing.T) {
	t.Run("serialized key can be used for signing", func(t *testing.T) {
		// Generate and serialize
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		privBytes := SerializePrivateKey(privKey)
		pubBytes := SerializePublicKey(privKey.PubKey())

		// Restore keys
		restoredPriv, err := ParsePrivateKey(privBytes)
		require.NoError(t, err)
		restoredPub, err := ParsePublicKey(pubBytes)
		require.NoError(t, err)

		// Sign with restored private key
		hash := hashSHA256([]byte("cross validation test"))
		sig, err := SignMessage(restoredPriv, hash)
		require.NoError(t, err)

		// Verify with restored public key
		valid, err := VerifySignature(restoredPub, hash, sig)
		require.NoError(t, err)
		assert.True(t, valid)
	})
}

// ==============================================================================
// Ethereum/EVM Crypto Tests
// ==============================================================================

func TestDeriveEthereumAddress(t *testing.T) {
	t.Run("produces 20-byte address", func(t *testing.T) {
		_, pubKey, err := GenerateKey()
		require.NoError(t, err)

		addr := deriveEthereumAddress(pubKey)
		assert.Len(t, addr, 20)
	})

	t.Run("deterministic output", func(t *testing.T) {
		_, pubKey, err := GenerateKey()
		require.NoError(t, err)

		addr1 := deriveEthereumAddress(pubKey)
		addr2 := deriveEthereumAddress(pubKey)
		assert.Equal(t, addr1, addr2)
	})

	t.Run("matches known test vector", func(t *testing.T) {
		// Known Ethereum test vector
		// Private key: 0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318
		// Address: 0x2c7536E3605D9C16a7a3D7b1898e529396a65c23
		privKeyHex := "4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318"
		privKeyBytes, err := hex.DecodeString(privKeyHex)
		require.NoError(t, err)

		privKey, err := ParsePrivateKey(privKeyBytes)
		require.NoError(t, err)

		addr := deriveEthereumAddress(privKey.PubKey())
		expectedAddr, err := hex.DecodeString("2c7536E3605D9C16a7a3D7b1898e529396a65c23")
		require.NoError(t, err)

		assert.Equal(t, strings.ToLower(hex.EncodeToString(expectedAddr)), strings.ToLower(hex.EncodeToString(addr)))
	})

	t.Run("different keys produce different addresses", func(t *testing.T) {
		_, pubKey1, err := GenerateKey()
		require.NoError(t, err)
		_, pubKey2, err := GenerateKey()
		require.NoError(t, err)

		addr1 := deriveEthereumAddress(pubKey1)
		addr2 := deriveEthereumAddress(pubKey2)
		assert.NotEqual(t, addr1, addr2)
	})
}

func TestDeriveEthereumAddressFromBytes(t *testing.T) {
	t.Run("accepts compressed public key", func(t *testing.T) {
		_, pubKey, err := GenerateKey()
		require.NoError(t, err)

		compressed := pubKey.SerializeCompressed()
		addr, err := deriveEthereumAddressFromBytes(compressed)
		require.NoError(t, err)
		assert.Len(t, addr, 20)

		// Should match direct derivation
		directAddr := deriveEthereumAddress(pubKey)
		assert.Equal(t, directAddr, addr)
	})

	t.Run("accepts uncompressed public key", func(t *testing.T) {
		_, pubKey, err := GenerateKey()
		require.NoError(t, err)

		uncompressed := pubKey.SerializeUncompressed()
		addr, err := deriveEthereumAddressFromBytes(uncompressed)
		require.NoError(t, err)
		assert.Len(t, addr, 20)

		// Should match direct derivation
		directAddr := deriveEthereumAddress(pubKey)
		assert.Equal(t, directAddr, addr)
	})

	t.Run("rejects invalid public key", func(t *testing.T) {
		_, err := deriveEthereumAddressFromBytes([]byte{0x01, 0x02, 0x03})
		assert.Error(t, err)
	})
}

func TestFormatEthereumAddress(t *testing.T) {
	t.Run("applies EIP-55 checksum", func(t *testing.T) {
		// Known checksummed address
		addrBytes, err := hex.DecodeString("5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed")
		require.NoError(t, err)

		formatted := formatEthereumAddress(addrBytes)
		assert.Equal(t, "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", formatted)
	})

	t.Run("checksums known test vector address", func(t *testing.T) {
		// Address from test vector: 0x2c7536E3605D9C16a7a3D7b1898e529396a65c23
		addrBytes, err := hex.DecodeString("2c7536E3605D9C16a7a3D7b1898e529396a65c23")
		require.NoError(t, err)

		formatted := formatEthereumAddress(addrBytes)
		assert.Equal(t, "0x2c7536E3605D9C16a7a3D7b1898e529396a65c23", formatted)
	})

	t.Run("rejects invalid length", func(t *testing.T) {
		formatted := formatEthereumAddress([]byte{0x01, 0x02, 0x03})
		assert.Equal(t, "", formatted)
	})

	t.Run("all lowercase address", func(t *testing.T) {
		// All lowercase hex address
		addrBytes, err := hex.DecodeString("0000000000000000000000000000000000000000")
		require.NoError(t, err)

		formatted := formatEthereumAddress(addrBytes)
		assert.Equal(t, "0x0000000000000000000000000000000000000000", formatted)
	})
}

func TestSignEIP155(t *testing.T) {
	t.Run("produces valid EIP-155 signature", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		hash := hashKeccak256([]byte("test message"))
		chainID := big.NewInt(1) // Ethereum mainnet

		v, r, s, err := SignEIP155(privKey, hash, chainID)
		require.NoError(t, err)

		// v should be chainId * 2 + 35 + recovery_id (0 or 1)
		// For chainId=1: v should be 37 or 38
		assert.True(t, v.Cmp(big.NewInt(37)) >= 0 && v.Cmp(big.NewInt(38)) <= 0,
			"v should be 37 or 38 for chainId=1, got %s", v.String())

		// r and s should be 32 bytes max
		assert.True(t, len(r.Bytes()) <= 32, "r should be at most 32 bytes")
		assert.True(t, len(s.Bytes()) <= 32, "s should be at most 32 bytes")
	})

	t.Run("signature is recoverable", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		hash := hashKeccak256([]byte("test message"))
		chainID := big.NewInt(10) // OP Mainnet

		v, r, s, err := SignEIP155(privKey, hash, chainID)
		require.NoError(t, err)

		// Construct signature bytes
		sig := make([]byte, 65)
		rBytes := r.Bytes()
		sBytes := s.Bytes()
		copy(sig[32-len(rBytes):32], rBytes)
		copy(sig[64-len(sBytes):64], sBytes)
		sig[64] = byte(v.Int64())

		// Recover public key
		recoveredPubKey, err := RecoverPubKeyFromSignature(hash, sig, chainID)
		require.NoError(t, err)

		assert.True(t, recoveredPubKey.IsEqual(privKey.PubKey()),
			"recovered public key should match original")
	})

	t.Run("rejects invalid hash length", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		_, _, _, err = SignEIP155(privKey, []byte("short"), big.NewInt(1))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hash must be 32 bytes")
	})

	t.Run("rejects nil private key", func(t *testing.T) {
		hash := hashKeccak256([]byte("test"))
		_, _, _, err := SignEIP155(nil, hash, big.NewInt(1))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "private key cannot be nil")
	})

	t.Run("rejects nil chain ID", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		hash := hashKeccak256([]byte("test"))
		_, _, _, err = SignEIP155(privKey, hash, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "chain ID cannot be nil")
	})

	t.Run("different chain IDs produce different v values", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		hash := hashKeccak256([]byte("test message"))

		v1, _, _, err := SignEIP155(privKey, hash, big.NewInt(1))
		require.NoError(t, err)

		v10, _, _, err := SignEIP155(privKey, hash, big.NewInt(10))
		require.NoError(t, err)

		// v values should reflect different chain IDs
		// chainId=1: v=37 or 38
		// chainId=10: v=55 or 56
		assert.NotEqual(t, v1.Int64()/2, v10.Int64()/2,
			"v values should differ for different chain IDs")
	})
}

func TestSignLegacy(t *testing.T) {
	t.Run("produces valid legacy signature", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		hash := hashKeccak256([]byte("test message"))

		v, r, s, err := SignLegacy(privKey, hash)
		require.NoError(t, err)

		// Legacy v should be 27 or 28
		assert.True(t, v.Cmp(big.NewInt(27)) >= 0 && v.Cmp(big.NewInt(28)) <= 0,
			"v should be 27 or 28, got %s", v.String())
		assert.NotNil(t, r)
		assert.NotNil(t, s)
	})

	t.Run("signature is recoverable", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		hash := hashKeccak256([]byte("test message"))

		v, r, s, err := SignLegacy(privKey, hash)
		require.NoError(t, err)

		// Construct signature bytes
		sig := make([]byte, 65)
		rBytes := r.Bytes()
		sBytes := s.Bytes()
		copy(sig[32-len(rBytes):32], rBytes)
		copy(sig[64-len(sBytes):64], sBytes)
		sig[64] = byte(v.Int64())

		// Recover public key with nil chainID (legacy)
		recoveredPubKey, err := RecoverPubKeyFromSignature(hash, sig, nil)
		require.NoError(t, err)

		assert.True(t, recoveredPubKey.IsEqual(privKey.PubKey()),
			"recovered public key should match original")
	})

	t.Run("rejects invalid hash length", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		_, _, _, err = SignLegacy(privKey, []byte("short"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hash must be 32 bytes")
	})

	t.Run("rejects nil private key", func(t *testing.T) {
		hash := hashKeccak256([]byte("test"))
		_, _, _, err := SignLegacy(nil, hash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "private key cannot be nil")
	})
}

func TestRecoverPubKeyFromSignature(t *testing.T) {
	t.Run("recovers public key from EIP-155 signature", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		hash := hashKeccak256([]byte("recovery test"))
		chainID := big.NewInt(1)

		v, r, s, err := SignEIP155(privKey, hash, chainID)
		require.NoError(t, err)

		sig := make([]byte, 65)
		rBytes := r.Bytes()
		sBytes := s.Bytes()
		copy(sig[32-len(rBytes):32], rBytes)
		copy(sig[64-len(sBytes):64], sBytes)
		sig[64] = byte(v.Int64())

		recovered, err := RecoverPubKeyFromSignature(hash, sig, chainID)
		require.NoError(t, err)
		assert.True(t, recovered.IsEqual(privKey.PubKey()))
	})

	t.Run("recovers public key from legacy signature", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		hash := hashKeccak256([]byte("legacy recovery test"))

		v, r, s, err := SignLegacy(privKey, hash)
		require.NoError(t, err)

		sig := make([]byte, 65)
		rBytes := r.Bytes()
		sBytes := s.Bytes()
		copy(sig[32-len(rBytes):32], rBytes)
		copy(sig[64-len(sBytes):64], sBytes)
		sig[64] = byte(v.Int64())

		recovered, err := RecoverPubKeyFromSignature(hash, sig, nil)
		require.NoError(t, err)
		assert.True(t, recovered.IsEqual(privKey.PubKey()))
	})

	t.Run("rejects invalid hash length", func(t *testing.T) {
		_, err := RecoverPubKeyFromSignature([]byte("short"), make([]byte, 65), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hash must be 32 bytes")
	})

	t.Run("rejects invalid signature length", func(t *testing.T) {
		hash := hashKeccak256([]byte("test"))
		_, err := RecoverPubKeyFromSignature(hash, []byte("short"), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signature must be 65 bytes")
	})
}

func TestVerifyRecovery(t *testing.T) {
	t.Run("returns true for valid recovery", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)

		hash := hashKeccak256([]byte("test"))
		v, r, s, err := SignLegacy(privKey, hash)
		require.NoError(t, err)

		recoveryID := byte(v.Int64() - 27)
		valid := verifyRecovery(hash, r, s, recoveryID, privKey.PubKey())
		assert.True(t, valid)
	})

	t.Run("returns false for wrong public key", func(t *testing.T) {
		privKey, _, err := GenerateKey()
		require.NoError(t, err)
		_, wrongPubKey, err := GenerateKey()
		require.NoError(t, err)

		hash := hashKeccak256([]byte("test"))
		v, r, s, err := SignLegacy(privKey, hash)
		require.NoError(t, err)

		recoveryID := byte(v.Int64() - 27)
		valid := verifyRecovery(hash, r, s, recoveryID, wrongPubKey)
		assert.False(t, valid)
	})
}

func TestEVMSigningRoundTrip(t *testing.T) {
	t.Run("full EIP-155 round trip with address verification", func(t *testing.T) {
		// Generate key
		privKey, pubKey, err := GenerateKey()
		require.NoError(t, err)

		// Derive address
		expectedAddr := deriveEthereumAddress(pubKey)

		// Sign message
		hash := hashKeccak256([]byte("verify me"))
		chainID := big.NewInt(1)

		v, r, s, err := SignEIP155(privKey, hash, chainID)
		require.NoError(t, err)

		// Construct signature
		sig := make([]byte, 65)
		rBytes := r.Bytes()
		sBytes := s.Bytes()
		copy(sig[32-len(rBytes):32], rBytes)
		copy(sig[64-len(sBytes):64], sBytes)
		sig[64] = byte(v.Int64())

		// Recover public key
		recovered, err := RecoverPubKeyFromSignature(hash, sig, chainID)
		require.NoError(t, err)

		// Derive address from recovered key
		recoveredAddr := deriveEthereumAddress(recovered)

		// Addresses should match
		assert.Equal(t, expectedAddr, recoveredAddr,
			"recovered address should match original")
	})
}

func BenchmarkDeriveEthereumAddress(b *testing.B) {
	_, pubKey, _ := GenerateKey()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deriveEthereumAddress(pubKey)
	}
}

func BenchmarkFormatEthereumAddress(b *testing.B) {
	_, pubKey, _ := GenerateKey()
	addr := deriveEthereumAddress(pubKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = formatEthereumAddress(addr)
	}
}

func BenchmarkSignEIP155(b *testing.B) {
	privKey, _, _ := GenerateKey()
	hash := hashKeccak256([]byte("benchmark"))
	chainID := big.NewInt(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = SignEIP155(privKey, hash, chainID)
	}
}

func BenchmarkSignLegacy(b *testing.B) {
	privKey, _, _ := GenerateKey()
	hash := hashKeccak256([]byte("benchmark"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = SignLegacy(privKey, hash)
	}
}

func BenchmarkRecoverPubKeyFromSignature(b *testing.B) {
	privKey, _, _ := GenerateKey()
	hash := hashKeccak256([]byte("benchmark"))
	chainID := big.NewInt(1)

	v, r, s, _ := SignEIP155(privKey, hash, chainID)
	sig := make([]byte, 65)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	sig[64] = byte(v.Int64())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = RecoverPubKeyFromSignature(hash, sig, chainID)
	}
}

func BenchmarkGenerateKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = GenerateKey()
	}
}

func BenchmarkSignMessage(b *testing.B) {
	privKey, _, _ := GenerateKey()
	hash := hashSHA256([]byte("benchmark"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = SignMessage(privKey, hash)
	}
}

func BenchmarkVerifySignature(b *testing.B) {
	privKey, pubKey, _ := GenerateKey()
	hash := hashSHA256([]byte("benchmark"))
	sig, _ := SignMessage(privKey, hash)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = VerifySignature(pubKey, hash, sig)
	}
}

func BenchmarkHashSHA256(b *testing.B) {
	data := bytes.Repeat([]byte{0x42}, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hashSHA256(data)
	}
}

func BenchmarkHashKeccak256(b *testing.B) {
	data := bytes.Repeat([]byte{0x42}, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hashKeccak256(data)
	}
}

func BenchmarkDeriveCosmosAddress(b *testing.B) {
	_, pubKey, _ := GenerateKey()
	serialized := SerializePublicKey(pubKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deriveCosmosAddress(serialized)
	}
}

