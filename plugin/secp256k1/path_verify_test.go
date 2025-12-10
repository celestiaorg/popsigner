package secp256k1

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/openbao/openbao/sdk/v2/logical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathVerify(t *testing.T) {
	t.Run("returns correct path configuration", func(t *testing.T) {
		b, _ := getTestBackend(t)

		paths := pathVerify(b)
		require.Len(t, paths, 1)
		assert.Contains(t, paths[0].Pattern, "verify/")
		assert.Contains(t, paths[0].Fields, "name")
		assert.Contains(t, paths[0].Fields, "input")
		assert.Contains(t, paths[0].Fields, "signature")
		assert.Contains(t, paths[0].Fields, "prehashed")
		assert.Contains(t, paths[0].Fields, "hash_algorithm")
	})
}

func TestPathVerifyWrite(t *testing.T) {
	ctx := context.Background()

	t.Run("verifies valid signature", func(t *testing.T) {
		b, storage := getTestBackend(t)
		entry := createTestKey(t, b, storage, "testkey", false)

		// Create message and sign it
		message := []byte("test message")
		hash := hashSHA256(message)
		privKey, _ := ParsePrivateKey(entry.PrivateKey)
		sigBytes, err := SignMessage(privKey, hash)
		require.NoError(t, err)

		inputB64 := base64.StdEncoding.EncodeToString(message)
		sigB64 := base64.StdEncoding.EncodeToString(sigBytes)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     inputB64,
				"signature": sigB64,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError(), "unexpected error: %v", resp.Error())

		valid, ok := resp.Data["valid"].(bool)
		require.True(t, ok)
		assert.True(t, valid)
	})

	t.Run("rejects invalid signature", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		message := []byte("test message")
		inputB64 := base64.StdEncoding.EncodeToString(message)

		// Create an invalid signature (wrong content)
		invalidSig := make([]byte, 64)
		for i := range invalidSig {
			invalidSig[i] = byte(i)
		}
		sigB64 := base64.StdEncoding.EncodeToString(invalidSig)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     inputB64,
				"signature": sigB64,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError())

		valid := resp.Data["valid"].(bool)
		assert.False(t, valid)
	})

	t.Run("rejects signature with wrong message", func(t *testing.T) {
		b, storage := getTestBackend(t)
		entry := createTestKey(t, b, storage, "testkey", false)

		// Sign original message
		originalMessage := []byte("original message")
		hash := hashSHA256(originalMessage)
		privKey, _ := ParsePrivateKey(entry.PrivateKey)
		sigBytes, err := SignMessage(privKey, hash)
		require.NoError(t, err)

		// Try to verify with different message
		differentMessage := []byte("different message")
		inputB64 := base64.StdEncoding.EncodeToString(differentMessage)
		sigB64 := base64.StdEncoding.EncodeToString(sigBytes)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     inputB64,
				"signature": sigB64,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError())

		valid := resp.Data["valid"].(bool)
		assert.False(t, valid)
	})

	t.Run("verifies with keccak256 hash", func(t *testing.T) {
		b, storage := getTestBackend(t)
		entry := createTestKey(t, b, storage, "testkey", false)

		message := []byte("ethereum message")
		hash := hashKeccak256(message)
		privKey, _ := ParsePrivateKey(entry.PrivateKey)
		sigBytes, err := SignMessage(privKey, hash)
		require.NoError(t, err)

		inputB64 := base64.StdEncoding.EncodeToString(message)
		sigB64 := base64.StdEncoding.EncodeToString(sigBytes)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":           "testkey",
				"input":          inputB64,
				"signature":      sigB64,
				"hash_algorithm": "keccak256",
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError())

		valid := resp.Data["valid"].(bool)
		assert.True(t, valid)
	})

	t.Run("verifies prehashed message", func(t *testing.T) {
		b, storage := getTestBackend(t)
		entry := createTestKey(t, b, storage, "testkey", false)

		// Create a 32-byte hash directly
		hash := hashSHA256([]byte("original message"))
		privKey, _ := ParsePrivateKey(entry.PrivateKey)
		sigBytes, err := SignMessage(privKey, hash)
		require.NoError(t, err)

		inputB64 := base64.StdEncoding.EncodeToString(hash)
		sigB64 := base64.StdEncoding.EncodeToString(sigBytes)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     inputB64,
				"signature": sigB64,
				"prehashed": true,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError())

		valid := resp.Data["valid"].(bool)
		assert.True(t, valid)
	})

	t.Run("error on nonexistent key", func(t *testing.T) {
		b, storage := getTestBackend(t)

		// Test that a key that doesn't exist returns an appropriate error
		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/nonexistent-key",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "nonexistent-key",
				"input":     "dGVzdA==",
				"signature": base64.StdEncoding.EncodeToString(make([]byte, 64)),
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsError())
		assert.Contains(t, resp.Error().Error(), "not found")
	})

	t.Run("error on missing input", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     "",
				"signature": base64.StdEncoding.EncodeToString(make([]byte, 64)),
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsError())
		assert.Contains(t, resp.Error().Error(), "input")
	})

	t.Run("error on missing signature", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     "dGVzdA==",
				"signature": "",
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsError())
		assert.Contains(t, resp.Error().Error(), "signature")
	})

	t.Run("error on invalid base64 input", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     "not-valid-base64!!!",
				"signature": base64.StdEncoding.EncodeToString(make([]byte, 64)),
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsError())
		assert.Contains(t, resp.Error().Error(), "base64")
	})

	t.Run("error on invalid base64 signature", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     "dGVzdA==",
				"signature": "not-valid-base64!!!",
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsError())
		assert.Contains(t, resp.Error().Error(), "base64")
	})

	t.Run("error on key not found", func(t *testing.T) {
		b, storage := getTestBackend(t)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/nonexistent",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "nonexistent",
				"input":     "dGVzdA==",
				"signature": base64.StdEncoding.EncodeToString(make([]byte, 64)),
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsError())
		assert.Contains(t, resp.Error().Error(), "not found")
	})

	t.Run("error on unsupported hash algorithm", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":           "testkey",
				"input":          "dGVzdA==",
				"signature":      base64.StdEncoding.EncodeToString(make([]byte, 64)),
				"hash_algorithm": "md5",
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsError())
		assert.Contains(t, resp.Error().Error(), "unsupported hash algorithm")
	})

	t.Run("error on prehashed input wrong length", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		// 20 bytes instead of 32
		shortHash := make([]byte, 20)
		inputB64 := base64.StdEncoding.EncodeToString(shortHash)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     inputB64,
				"signature": base64.StdEncoding.EncodeToString(make([]byte, 64)),
				"prehashed": true,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsError())
		assert.Contains(t, resp.Error().Error(), "32 bytes")
	})

	t.Run("returns false for wrong signature length", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		// 32 bytes instead of 64
		shortSig := make([]byte, 32)
		sigB64 := base64.StdEncoding.EncodeToString(shortSig)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     "dGVzdA==",
				"signature": sigB64,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError())

		valid := resp.Data["valid"].(bool)
		assert.False(t, valid)

		// Error message is optional but if present should mention the issue
		if errMsg, ok := resp.Data["error"].(string); ok {
			assert.Contains(t, errMsg, "64 bytes")
		}
	})

	t.Run("returns false for signature with zero R", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		// Signature with R=0 (first 32 bytes are zero)
		zeroRSig := make([]byte, 64)
		for i := 32; i < 64; i++ {
			zeroRSig[i] = byte(i)
		}
		sigB64 := base64.StdEncoding.EncodeToString(zeroRSig)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     "dGVzdA==",
				"signature": sigB64,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError())

		valid := resp.Data["valid"].(bool)
		assert.False(t, valid)
	})

	t.Run("verifies signature from different key correctly fails", func(t *testing.T) {
		b, storage := getTestBackend(t)
		entry1 := createTestKey(t, b, storage, "key1", false)
		createTestKey(t, b, storage, "key2", false)

		// Sign with key1
		message := []byte("test message")
		hash := hashSHA256(message)
		privKey, _ := ParsePrivateKey(entry1.PrivateKey)
		sigBytes, err := SignMessage(privKey, hash)
		require.NoError(t, err)

		inputB64 := base64.StdEncoding.EncodeToString(message)
		sigB64 := base64.StdEncoding.EncodeToString(sigBytes)

		// Try to verify with key2
		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/key2",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "key2",
				"input":     inputB64,
				"signature": sigB64,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError())

		valid := resp.Data["valid"].(bool)
		assert.False(t, valid)
	})

	t.Run("verifies single byte message", func(t *testing.T) {
		b, storage := getTestBackend(t)
		entry := createTestKey(t, b, storage, "testkey", false)

		// Sign single byte message (empty base64 encoding "" is treated as missing input)
		message := []byte{0x00}
		hash := hashSHA256(message)
		privKey, _ := ParsePrivateKey(entry.PrivateKey)
		sigBytes, err := SignMessage(privKey, hash)
		require.NoError(t, err)

		inputB64 := base64.StdEncoding.EncodeToString(message)
		sigB64 := base64.StdEncoding.EncodeToString(sigBytes)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "verify/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     inputB64,
				"signature": sigB64,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError())

		valid := resp.Data["valid"].(bool)
		assert.True(t, valid)
	})
}

func TestVerifyWithWrongHashAlgorithm(t *testing.T) {
	ctx := context.Background()
	b, storage := getTestBackend(t)
	entry := createTestKey(t, b, storage, "testkey", false)

	message := []byte("test message")

	// Sign with SHA-256
	hash := hashSHA256(message)
	privKey, _ := ParsePrivateKey(entry.PrivateKey)
	sigBytes, err := SignMessage(privKey, hash)
	require.NoError(t, err)

	inputB64 := base64.StdEncoding.EncodeToString(message)
	sigB64 := base64.StdEncoding.EncodeToString(sigBytes)

	// Verify with keccak256 (should fail)
	req := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "verify/testkey",
		Storage:   storage,
		Data: map[string]interface{}{
			"name":           "testkey",
			"input":          inputB64,
			"signature":      sigB64,
			"hash_algorithm": "keccak256",
		},
	}

	resp, err := b.HandleRequest(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.False(t, resp.IsError())

	valid := resp.Data["valid"].(bool)
	assert.False(t, valid, "signature should not verify with wrong hash algorithm")
}

func BenchmarkPathVerifyWrite(b *testing.B) {
	ctx := context.Background()
	backend, storage := getTestBackendBench(b)
	entry := createTestKeyBench(b, backend, storage, "benchmark-key", false)

	// Pre-create signature
	message := []byte("benchmark message")
	hash := hashSHA256(message)
	privKey, _ := ParsePrivateKey(entry.PrivateKey)
	sigBytes, _ := SignMessage(privKey, hash)

	inputB64 := base64.StdEncoding.EncodeToString(message)
	sigB64 := base64.StdEncoding.EncodeToString(sigBytes)

	req := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "verify/benchmark-key",
		Storage:   storage,
		Data: map[string]interface{}{
			"name":      "benchmark-key",
			"input":     inputB64,
			"signature": sigB64,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = backend.HandleRequest(ctx, req)
	}
}

func TestSignatureFormatCompatibility(t *testing.T) {
	ctx := context.Background()
	b, storage := getTestBackend(t)
	createTestKey(t, b, storage, "testkey", false)

	message := []byte("test message")
	inputB64 := base64.StdEncoding.EncodeToString(message)

	// Sign through the path
	signReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "sign/testkey",
		Storage:   storage,
		Data: map[string]interface{}{
			"name":  "testkey",
			"input": inputB64,
		},
	}

	signResp, err := b.HandleRequest(ctx, signReq)
	require.NoError(t, err)
	require.NotNil(t, signResp)
	require.False(t, signResp.IsError())

	sigB64 := signResp.Data["signature"].(string)
	sigBytes, _ := base64.StdEncoding.DecodeString(sigB64)

	// Verify signature format
	t.Run("signature is exactly 64 bytes", func(t *testing.T) {
		assert.Len(t, sigBytes, 64)
	})

	t.Run("signature has low-S", func(t *testing.T) {
		s := new(btcec.ModNScalar)
		s.SetByteSlice(sigBytes[32:])
		assert.False(t, s.IsOverHalfOrder())
	})

	t.Run("R and S are non-zero", func(t *testing.T) {
		r := new(btcec.ModNScalar)
		s := new(btcec.ModNScalar)
		r.SetByteSlice(sigBytes[:32])
		s.SetByteSlice(sigBytes[32:])

		assert.False(t, r.IsZero(), "R should not be zero")
		assert.False(t, s.IsZero(), "S should not be zero")
	})
}

