package secp256k1

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/openbao/openbao/sdk/v2/logical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestKey creates a key in storage for testing.
func createTestKey(t *testing.T, b *backend, storage logical.Storage, name string, exportable bool) *keyEntry {
	t.Helper()

	privKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	entry := &keyEntry{
		PrivateKey: privKey.Serialize(),
		PublicKey:  privKey.PubKey().SerializeCompressed(),
		Exportable: exportable,
	}

	storageEntry, err := logical.StorageEntryJSON("keys/"+name, entry)
	require.NoError(t, err)
	require.NoError(t, storage.Put(context.Background(), storageEntry))

	return entry
}

func TestPathSign(t *testing.T) {
	t.Run("returns correct path configuration", func(t *testing.T) {
		b, _ := getTestBackend(t)

		paths := pathSign(b)
		require.Len(t, paths, 1)
		assert.Contains(t, paths[0].Pattern, "sign/")
		assert.Contains(t, paths[0].Fields, "name")
		assert.Contains(t, paths[0].Fields, "input")
		assert.Contains(t, paths[0].Fields, "prehashed")
		assert.Contains(t, paths[0].Fields, "hash_algorithm")
		assert.Contains(t, paths[0].Fields, "output_format")
	})
}

func TestPathSignWrite(t *testing.T) {
	ctx := context.Background()

	t.Run("signs message with sha256 hash", func(t *testing.T) {
		b, storage := getTestBackend(t)
		entry := createTestKey(t, b, storage, "testkey", false)

		// Prepare message
		message := []byte("Hello, World!")
		inputB64 := base64.StdEncoding.EncodeToString(message)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "sign/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":  "testkey",
				"input": inputB64,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError(), "unexpected error: %v", resp.Error())

		// Verify response
		sigB64, ok := resp.Data["signature"].(string)
		require.True(t, ok)
		sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
		require.NoError(t, err)
		assert.Len(t, sigBytes, 64, "signature should be 64 bytes (Cosmos format)")

		// Verify public key in response
		pubKeyHex, ok := resp.Data["public_key"].(string)
		require.True(t, ok)
		assert.Equal(t, hex.EncodeToString(entry.PublicKey), pubKeyHex)

		// Verify key_version
		keyVersion, ok := resp.Data["key_version"].(int)
		require.True(t, ok)
		assert.Equal(t, 1, keyVersion)
	})

	t.Run("signs message with keccak256 hash", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		message := []byte("Ethereum message")
		inputB64 := base64.StdEncoding.EncodeToString(message)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "sign/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":           "testkey",
				"input":          inputB64,
				"hash_algorithm": "keccak256",
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError())

		sigB64 := resp.Data["signature"].(string)
		sigBytes, _ := base64.StdEncoding.DecodeString(sigB64)
		assert.Len(t, sigBytes, 64)
	})

	t.Run("signs prehashed message", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		// Create a 32-byte hash
		hash := hashSHA256([]byte("test message"))
		inputB64 := base64.StdEncoding.EncodeToString(hash)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "sign/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     inputB64,
				"prehashed": true,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError())

		sigB64 := resp.Data["signature"].(string)
		sigBytes, _ := base64.StdEncoding.DecodeString(sigB64)
		assert.Len(t, sigBytes, 64)
	})

	t.Run("returns der format when requested", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		message := []byte("test")
		inputB64 := base64.StdEncoding.EncodeToString(message)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "sign/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":          "testkey",
				"input":         inputB64,
				"output_format": "der",
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError())

		sigB64 := resp.Data["signature"].(string)
		sigBytes, _ := base64.StdEncoding.DecodeString(sigB64)
		// DER signatures start with 0x30 (sequence tag)
		assert.Equal(t, byte(0x30), sigBytes[0], "DER signature should start with sequence tag")
	})

	t.Run("returns cosmos format by default", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		message := []byte("test")
		inputB64 := base64.StdEncoding.EncodeToString(message)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "sign/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":  "testkey",
				"input": inputB64,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError())

		sigB64 := resp.Data["signature"].(string)
		sigBytes, _ := base64.StdEncoding.DecodeString(sigB64)
		assert.Len(t, sigBytes, 64, "Cosmos format should be exactly 64 bytes")
	})

	t.Run("produces low-S signature", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		// Sign multiple messages to increase chance of hitting high-S
		for i := 0; i < 10; i++ {
			message := []byte{byte(i), 0x01, 0x02, 0x03}
			inputB64 := base64.StdEncoding.EncodeToString(message)

			req := &logical.Request{
				Operation: logical.UpdateOperation,
				Path:      "sign/testkey",
				Storage:   storage,
				Data: map[string]interface{}{
					"name":  "testkey",
					"input": inputB64,
				},
			}

			resp, err := b.HandleRequest(ctx, req)
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.False(t, resp.IsError())

			sigB64 := resp.Data["signature"].(string)
			sigBytes, _ := base64.StdEncoding.DecodeString(sigB64)

			// Check low-S
			s := new(btcec.ModNScalar)
			s.SetByteSlice(sigBytes[32:])
			assert.False(t, s.IsOverHalfOrder(), "signature should have low-S")
		}
	})

	t.Run("error on nonexistent key", func(t *testing.T) {
		b, storage := getTestBackend(t)

		// Test that a key that doesn't exist returns an appropriate error
		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "sign/nonexistent-key",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":  "nonexistent-key",
				"input": "dGVzdA==",
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
			Path:      "sign/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":  "testkey",
				"input": "",
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsError())
		assert.Contains(t, resp.Error().Error(), "input")
	})

	t.Run("error on invalid base64 input", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "sign/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":  "testkey",
				"input": "not-valid-base64!!!",
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
			Path:      "sign/nonexistent",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":  "nonexistent",
				"input": "dGVzdA==",
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
			Path:      "sign/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":           "testkey",
				"input":          "dGVzdA==",
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
			Path:      "sign/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":      "testkey",
				"input":     inputB64,
				"prehashed": true,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.IsError())
		assert.Contains(t, resp.Error().Error(), "32 bytes")
	})

	t.Run("signature is verifiable", func(t *testing.T) {
		b, storage := getTestBackend(t)
		entry := createTestKey(t, b, storage, "testkey", false)

		message := []byte("verify me")
		inputB64 := base64.StdEncoding.EncodeToString(message)

		// Sign
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

		// Verify manually
		hash := hashSHA256(message)
		pubKey, _ := ParsePublicKey(entry.PublicKey)
		valid, err := VerifySignature(pubKey, hash, sigBytes)
		require.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("handles empty message", func(t *testing.T) {
		b, storage := getTestBackend(t)
		createTestKey(t, b, storage, "testkey", false)

		// Empty message encoded as base64 - note that base64 of empty bytes is ""
		// which would be treated as missing input. Use a single byte instead.
		inputB64 := base64.StdEncoding.EncodeToString([]byte{0x00})

		req := &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "sign/testkey",
			Storage:   storage,
			Data: map[string]interface{}{
				"name":  "testkey",
				"input": inputB64,
			},
		}

		resp, err := b.HandleRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.IsError())

		sigB64 := resp.Data["signature"].(string)
		sigBytes, _ := base64.StdEncoding.DecodeString(sigB64)
		assert.Len(t, sigBytes, 64)
	})
}

func TestSignVerifyRoundTripThroughPaths(t *testing.T) {
	ctx := context.Background()
	b, storage := getTestBackend(t)
	createTestKey(t, b, storage, "roundtrip-key", false)

	testCases := []struct {
		name      string
		message   string
		hashAlgo  string
		prehashed bool
	}{
		{"simple message sha256", "Hello, World!", "sha256", false},
		{"simple message keccak256", "Hello, Ethereum!", "keccak256", false},
		{"single byte message", string([]byte{0x00}), "sha256", false},
		{"prehashed sha256", string(make([]byte, 32)), "sha256", true},
		{"large message", string(make([]byte, 10000)), "sha256", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var inputB64 string
			if tc.prehashed {
				inputB64 = base64.StdEncoding.EncodeToString([]byte(tc.message))
			} else {
				inputB64 = base64.StdEncoding.EncodeToString([]byte(tc.message))
			}

			// Sign
			signReq := &logical.Request{
				Operation: logical.UpdateOperation,
				Path:      "sign/roundtrip-key",
				Storage:   storage,
				Data: map[string]interface{}{
					"name":           "roundtrip-key",
					"input":          inputB64,
					"hash_algorithm": tc.hashAlgo,
					"prehashed":      tc.prehashed,
				},
			}

			signResp, err := b.HandleRequest(ctx, signReq)
			require.NoError(t, err)
			require.NotNil(t, signResp)
			require.False(t, signResp.IsError(), "sign error: %v", signResp.Error())

			sigB64 := signResp.Data["signature"].(string)

			// Verify
			verifyReq := &logical.Request{
				Operation: logical.UpdateOperation,
				Path:      "verify/roundtrip-key",
				Storage:   storage,
				Data: map[string]interface{}{
					"name":           "roundtrip-key",
					"input":          inputB64,
					"signature":      sigB64,
					"hash_algorithm": tc.hashAlgo,
					"prehashed":      tc.prehashed,
				},
			}

			verifyResp, err := b.HandleRequest(ctx, verifyReq)
			require.NoError(t, err)
			require.NotNil(t, verifyResp)
			require.False(t, verifyResp.IsError(), "verify error: %v", verifyResp.Error())

			valid := verifyResp.Data["valid"].(bool)
			assert.True(t, valid, "signature should be valid for %s", tc.name)
		})
	}
}

func BenchmarkPathSignWrite(b *testing.B) {
	ctx := context.Background()
	backend, storage := getTestBackendBench(b)
	createTestKeyBench(b, backend, storage, "benchmark-key", false)

	message := []byte("benchmark message")
	inputB64 := base64.StdEncoding.EncodeToString(message)

	req := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "sign/benchmark-key",
		Storage:   storage,
		Data: map[string]interface{}{
			"name":  "benchmark-key",
			"input": inputB64,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = backend.HandleRequest(ctx, req)
	}
}

// Helper functions for benchmarks
func getTestBackendBench(b *testing.B) (*backend, logical.Storage) {
	b.Helper()

	config := logical.TestBackendConfig()
	config.StorageView = &logical.InmemStorage{}

	back, err := Factory(context.Background(), config)
	if err != nil {
		b.Fatal(err)
	}

	return back.(*backend), config.StorageView
}

func createTestKeyBench(b *testing.B, backend *backend, storage logical.Storage, name string, exportable bool) *keyEntry {
	b.Helper()

	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		b.Fatal(err)
	}

	entry := &keyEntry{
		PrivateKey: privKey.Serialize(),
		PublicKey:  privKey.PubKey().SerializeCompressed(),
		Exportable: exportable,
	}

	storageEntry, err := logical.StorageEntryJSON("keys/"+name, entry)
	if err != nil {
		b.Fatal(err)
	}
	if err := storage.Put(context.Background(), storageEntry); err != nil {
		b.Fatal(err)
	}

	return entry
}

