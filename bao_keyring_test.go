package banhbaoring

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================
// Test Helpers
// ============================================

// setupTestKeyring creates a BaoKeyring for testing with a mock server.
// The server handler can be customized for each test.
func setupTestKeyring(t *testing.T, handler http.HandlerFunc) (*BaoKeyring, *httptest.Server) {
	t.Helper()

	server := httptest.NewTLSServer(handler)

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	// Create client directly (bypass health check for testing)
	client, err := NewBaoClient(Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		SkipTLSVerify: true,
	})
	require.NoError(t, err)

	store, err := NewBaoStore(storePath)
	require.NoError(t, err)

	// Use the newBaoKeyringForTesting helper
	kr := newBaoKeyringForTesting(client, store)

	return kr, server
}

// setupTestKeyringWithKey creates a keyring with a pre-populated key.
func setupTestKeyringWithKey(t *testing.T, uid, address string, pubKeyBytes []byte, handler http.HandlerFunc) (*BaoKeyring, *httptest.Server) {
	t.Helper()

	kr, server := setupTestKeyring(t, handler)

	// Add test key to store
	meta := &KeyMetadata{
		UID:         uid,
		Name:        uid,
		PubKeyBytes: pubKeyBytes,
		PubKeyType:  "secp256k1",
		Address:     address,
		BaoKeyPath:  "secp256k1/keys/" + uid,
		Algorithm:   AlgorithmSecp256k1,
		Exportable:  true,
		CreatedAt:   time.Now(),
		Source:      SourceGenerated,
	}
	err := kr.store.Save(meta)
	require.NoError(t, err)

	return kr, server
}

// validSignatureResponse creates a valid 64-byte signature for testing.
func validSignatureResponse() []byte {
	sig := make([]byte, 64)
	for i := range sig {
		sig[i] = byte(i)
	}
	return sig
}

// testPubKeyBytes returns a valid 33-byte compressed secp256k1 public key for testing.
func testPubKeyBytes() []byte {
	// A valid compressed secp256k1 public key starts with 0x02 or 0x03
	pubKey := make([]byte, 33)
	pubKey[0] = 0x02 // Compressed format prefix
	for i := 1; i < 33; i++ {
		pubKey[i] = byte(i)
	}
	return pubKey
}

// ============================================
// Sign Tests
// ============================================

func TestBaoKeyring_Sign_Success(t *testing.T) {
	validSig := validSignatureResponse()
	pubKeyBytes := testPubKeyBytes()

	handler := func(w http.ResponseWriter, r *http.Request) {
		// Handle health check
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Handle sign request
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/secp256k1/sign/sign-key")
		assert.Equal(t, "test-token", r.Header.Get("X-Vault-Token"))

		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)

		// Verify prehashed is true
		assert.True(t, body["prehashed"].(bool))
		assert.Equal(t, "cosmos", body["output_format"])

		// Verify the input is base64 encoded SHA-256 hash (32 bytes)
		inputB64 := body["input"].(string)
		inputBytes, err := base64.StdEncoding.DecodeString(inputB64)
		require.NoError(t, err)
		assert.Len(t, inputBytes, 32, "input should be 32-byte SHA-256 hash")

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(validSig),
				PublicKey: "02abcdef",
			},
		})
	}

	kr, server := setupTestKeyringWithKey(t, "sign-key", "cosmos1abc", pubKeyBytes, handler)
	defer server.Close()

	msg := []byte("test message to sign")
	sig, pubKey, err := kr.Sign("sign-key", msg, signing.SignMode_SIGN_MODE_DIRECT)

	require.NoError(t, err)
	assert.Len(t, sig, 64, "signature should be 64 bytes (R||S format)")
	assert.NotNil(t, pubKey)
	assert.Equal(t, pubKeyBytes, pubKey.Bytes())
}

func TestBaoKeyring_Sign_HashesMessageWithSHA256(t *testing.T) {
	validSig := validSignatureResponse()
	pubKeyBytes := testPubKeyBytes()
	msg := []byte("test message")
	expectedHash := sha256.Sum256(msg)

	var receivedInput []byte
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)

		inputB64 := body["input"].(string)
		receivedInput, _ = base64.StdEncoding.DecodeString(inputB64)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(validSig),
			},
		})
	}

	kr, server := setupTestKeyringWithKey(t, "hash-test", "cosmos1xyz", pubKeyBytes, handler)
	defer server.Close()

	_, _, err := kr.Sign("hash-test", msg, signing.SignMode_SIGN_MODE_DIRECT)
	require.NoError(t, err)

	// Verify the received input matches SHA-256 hash of the message
	assert.Equal(t, expectedHash[:], receivedInput, "OpenBao should receive SHA-256 hash of message")
}

func TestBaoKeyring_Sign_KeyNotFound(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Try to sign with non-existent key
	_, _, err := kr.Sign("nonexistent-key", []byte("test"), signing.SignMode_SIGN_MODE_DIRECT)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestBaoKeyring_Sign_OpenBaoError(t *testing.T) {
	pubKeyBytes := testPubKeyBytes()

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Simulate OpenBao error
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"permission denied"},
		})
	}

	kr, server := setupTestKeyringWithKey(t, "error-key", "cosmos1err", pubKeyBytes, handler)
	defer server.Close()

	_, _, err := kr.Sign("error-key", []byte("test"), signing.SignMode_SIGN_MODE_DIRECT)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBaoAuth)
}

func TestBaoKeyring_Sign_InvalidSignatureLength(t *testing.T) {
	pubKeyBytes := testPubKeyBytes()

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Return signature with wrong length
		shortSig := make([]byte, 32) // Should be 64 bytes
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(shortSig),
			},
		})
	}

	kr, server := setupTestKeyringWithKey(t, "short-sig", "cosmos1short", pubKeyBytes, handler)
	defer server.Close()

	_, _, err := kr.Sign("short-sig", []byte("test"), signing.SignMode_SIGN_MODE_DIRECT)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSignature)
}

func TestBaoKeyring_Sign_EmptyMessage(t *testing.T) {
	validSig := validSignatureResponse()
	pubKeyBytes := testPubKeyBytes()

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(validSig),
			},
		})
	}

	kr, server := setupTestKeyringWithKey(t, "empty-msg", "cosmos1empty", pubKeyBytes, handler)
	defer server.Close()

	// Empty message should still work (SHA-256 of empty produces valid hash)
	sig, pubKey, err := kr.Sign("empty-msg", []byte{}, signing.SignMode_SIGN_MODE_DIRECT)

	require.NoError(t, err)
	assert.Len(t, sig, 64)
	assert.NotNil(t, pubKey)
}

func TestBaoKeyring_Sign_LargeMessage(t *testing.T) {
	validSig := validSignatureResponse()
	pubKeyBytes := testPubKeyBytes()

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Verify input is still 32 bytes (SHA-256 hash) regardless of message size
		inputB64 := body["input"].(string)
		inputBytes, _ := base64.StdEncoding.DecodeString(inputB64)
		if len(inputBytes) != 32 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(validSig),
			},
		})
	}

	kr, server := setupTestKeyringWithKey(t, "large-msg", "cosmos1large", pubKeyBytes, handler)
	defer server.Close()

	// Create a large message (1MB)
	largeMsg := make([]byte, 1024*1024)
	for i := range largeMsg {
		largeMsg[i] = byte(i % 256)
	}

	sig, pubKey, err := kr.Sign("large-msg", largeMsg, signing.SignMode_SIGN_MODE_DIRECT)

	require.NoError(t, err)
	assert.Len(t, sig, 64)
	assert.NotNil(t, pubKey)
}

func TestBaoKeyring_Sign_DifferentSignModes(t *testing.T) {
	validSig := validSignatureResponse()
	pubKeyBytes := testPubKeyBytes()

	signModes := []signing.SignMode{
		signing.SignMode_SIGN_MODE_DIRECT,
		signing.SignMode_SIGN_MODE_TEXTUAL,
		signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
	}

	for _, mode := range signModes {
		t.Run(mode.String(), func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/sys/health" {
					w.WriteHeader(http.StatusOK)
					return
				}

				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": SignResponse{
						Signature: base64.StdEncoding.EncodeToString(validSig),
					},
				})
			}

			kr, server := setupTestKeyringWithKey(t, "mode-test", "cosmos1mode", pubKeyBytes, handler)
			defer server.Close()

			sig, pubKey, err := kr.Sign("mode-test", []byte("test"), mode)

			require.NoError(t, err)
			assert.Len(t, sig, 64)
			assert.NotNil(t, pubKey)
		})
	}
}

func TestBaoKeyring_Sign_ReturnsCorrectPubKey(t *testing.T) {
	validSig := validSignatureResponse()
	pubKeyBytes := testPubKeyBytes()

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(validSig),
			},
		})
	}

	kr, server := setupTestKeyringWithKey(t, "pubkey-test", "cosmos1pub", pubKeyBytes, handler)
	defer server.Close()

	_, pubKey, err := kr.Sign("pubkey-test", []byte("test"), signing.SignMode_SIGN_MODE_DIRECT)

	require.NoError(t, err)
	assert.Equal(t, "secp256k1", pubKey.Type())
	assert.Equal(t, pubKeyBytes, pubKey.Bytes())
}

// ============================================
// SignByAddress Tests
// ============================================

func TestBaoKeyring_SignByAddress_Success(t *testing.T) {
	validSig := validSignatureResponse()
	pubKeyBytes := testPubKeyBytes()
	testAddress := "cosmos1testaddr123"

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(validSig),
			},
		})
	}

	kr, server := setupTestKeyringWithKey(t, "addr-key", testAddress, pubKeyBytes, handler)
	defer server.Close()

	// Create a mock address that returns our test address string
	addr := mockAddress(testAddress)

	sig, pubKey, err := kr.SignByAddress(addr, []byte("test message"), signing.SignMode_SIGN_MODE_DIRECT)

	require.NoError(t, err)
	assert.Len(t, sig, 64)
	assert.NotNil(t, pubKey)
	assert.Equal(t, pubKeyBytes, pubKey.Bytes())
}

func TestBaoKeyring_SignByAddress_AddressNotFound(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Try to sign with non-existent address
	addr := mockAddress("cosmos1nonexistent")

	_, _, err := kr.SignByAddress(addr, []byte("test"), signing.SignMode_SIGN_MODE_DIRECT)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestBaoKeyring_SignByAddress_MultipleKeys(t *testing.T) {
	validSig := validSignatureResponse()
	pubKeyBytes1 := testPubKeyBytes()
	pubKeyBytes2 := make([]byte, 33)
	pubKeyBytes2[0] = 0x03
	for i := 1; i < 33; i++ {
		pubKeyBytes2[i] = byte(i + 100)
	}

	var signedKeyName string
	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sys/health":
			w.WriteHeader(http.StatusOK)
			return
		case "/v1/secp256k1/sign/key1":
			if r.Method == "POST" {
				signedKeyName = "key1"
			}
		case "/v1/secp256k1/sign/key2":
			if r.Method == "POST" {
				signedKeyName = "key2"
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(validSig),
			},
		})
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Add two keys with different addresses
	err := kr.store.Save(&KeyMetadata{
		UID:         "key1",
		Name:        "key1",
		PubKeyBytes: pubKeyBytes1,
		Address:     "cosmos1addr1",
		Algorithm:   AlgorithmSecp256k1,
	})
	require.NoError(t, err)

	err = kr.store.Save(&KeyMetadata{
		UID:         "key2",
		Name:        "key2",
		PubKeyBytes: pubKeyBytes2,
		Address:     "cosmos1addr2",
		Algorithm:   AlgorithmSecp256k1,
	})
	require.NoError(t, err)

	// Sign with second address - should use key2
	addr := mockAddress("cosmos1addr2")
	sig, pubKey, err := kr.SignByAddress(addr, []byte("test"), signing.SignMode_SIGN_MODE_DIRECT)

	require.NoError(t, err)
	assert.Len(t, sig, 64)
	assert.Equal(t, "key2", signedKeyName, "should have signed with key2")
	assert.Equal(t, pubKeyBytes2, pubKey.Bytes())
}

func TestBaoKeyring_SignByAddress_OpenBaoError(t *testing.T) {
	pubKeyBytes := testPubKeyBytes()

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"internal error"},
		})
	}

	kr, server := setupTestKeyringWithKey(t, "err-key", "cosmos1erraddr", pubKeyBytes, handler)
	defer server.Close()

	addr := mockAddress("cosmos1erraddr")
	_, _, err := kr.SignByAddress(addr, []byte("test"), signing.SignMode_SIGN_MODE_DIRECT)

	require.Error(t, err)
}

// ============================================
// Integration-style Tests
// ============================================

func TestBaoKeyring_Sign_RoundTrip(t *testing.T) {
	// This test verifies the complete signing flow
	validSig := validSignatureResponse()
	pubKeyBytes := testPubKeyBytes()

	var requestCount int
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		requestCount++

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(validSig),
			},
		})
	}

	kr, server := setupTestKeyringWithKey(t, "roundtrip", "cosmos1round", pubKeyBytes, handler)
	defer server.Close()

	// Sign multiple messages
	messages := [][]byte{
		[]byte("message 1"),
		[]byte("message 2"),
		[]byte("different message"),
	}

	for _, msg := range messages {
		sig, pubKey, err := kr.Sign("roundtrip", msg, signing.SignMode_SIGN_MODE_DIRECT)
		require.NoError(t, err)
		assert.Len(t, sig, 64)
		assert.NotNil(t, pubKey)
	}

	// Verify all requests went through
	assert.Equal(t, len(messages), requestCount)
}

func TestBaoKeyring_SignByAddress_ConsistentWithSign(t *testing.T) {
	// SignByAddress should produce the same result as Sign for the same key
	validSig := validSignatureResponse()
	pubKeyBytes := testPubKeyBytes()

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": SignResponse{
				Signature: base64.StdEncoding.EncodeToString(validSig),
			},
		})
	}

	kr, server := setupTestKeyringWithKey(t, "consistent", "cosmos1cons", pubKeyBytes, handler)
	defer server.Close()

	msg := []byte("test message")

	// Sign using UID
	sig1, pubKey1, err := kr.Sign("consistent", msg, signing.SignMode_SIGN_MODE_DIRECT)
	require.NoError(t, err)

	// Sign using address
	addr := mockAddress("cosmos1cons")
	sig2, pubKey2, err := kr.SignByAddress(addr, msg, signing.SignMode_SIGN_MODE_DIRECT)
	require.NoError(t, err)

	// Both should produce the same signature and public key
	assert.Equal(t, sig1, sig2, "signatures should match")
	assert.Equal(t, pubKey1.Bytes(), pubKey2.Bytes(), "public keys should match")
}

// ============================================
// Backend and SupportedAlgorithms Tests
// ============================================

func TestBaoKeyring_Backend(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	assert.Equal(t, BackendType, kr.Backend())
	assert.Equal(t, "openbao", kr.Backend())
}

func TestBaoKeyring_SupportedAlgorithms(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	supported, defaults := kr.SupportedAlgorithms()

	// Both should contain only secp256k1
	require.Len(t, supported, 1)
	require.Len(t, defaults, 1)

	// Name() returns hd.PubKeyType, compare using string conversion
	assert.Equal(t, "secp256k1", string(supported[0].Name()))
	assert.Equal(t, "secp256k1", string(defaults[0].Name()))

	// Both lists should be the same
	assert.Equal(t, supported[0], defaults[0])
}

func TestBaoKeyring_SupportedAlgorithms_ContainsSecp256k1(t *testing.T) {
	kr := &BaoKeyring{}

	supported, defaults := kr.SupportedAlgorithms()

	// Verify secp256k1 can be found in the lists
	found := false
	for _, algo := range supported {
		if algo.Name() == "secp256k1" {
			found = true
			break
		}
	}
	assert.True(t, found, "secp256k1 should be in supported algorithms")

	found = false
	for _, algo := range defaults {
		if algo.Name() == "secp256k1" {
			found = true
			break
		}
	}
	assert.True(t, found, "secp256k1 should be in default algorithms")
}

func TestBaoKeyring_Close(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// Close should succeed
	err := kr.Close()
	assert.NoError(t, err)

	// Close again should still succeed (idempotent)
	err = kr.Close()
	assert.NoError(t, err)
}

func TestBaoKeyring_Close_NilStore(t *testing.T) {
	// Test Close with nil store
	kr := &BaoKeyring{store: nil}
	err := kr.Close()
	assert.NoError(t, err)
}

func TestBaoKeyring_MigrateAll(t *testing.T) {
	pubKeyBytes := testPubKeyBytes()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	kr, server := setupTestKeyringWithKey(t, "migrate-key", "cosmos1migrate", pubKeyBytes, handler)
	defer server.Close()

	// MigrateAll should return all existing keys
	records, err := kr.MigrateAll()
	require.NoError(t, err)

	// Should have exactly one key
	assert.Len(t, records, 1)
	assert.Equal(t, "migrate-key", records[0].Name)
}

func TestBaoKeyring_MigrateAll_EmptyStore(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	kr, server := setupTestKeyring(t, handler)
	defer server.Close()

	// MigrateAll on empty store should return empty list, not error
	records, err := kr.MigrateAll()
	require.NoError(t, err)
	assert.Empty(t, records)
}

// ============================================
// New Function Tests
// ============================================

func TestNew_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewTLSServer(handler)
	defer server.Close()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	cfg := Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	kr, err := New(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, kr)
	assert.NotNil(t, kr.client)
	assert.NotNil(t, kr.store)
}

func TestNew_MissingConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr error
	}{
		{
			name:    "missing BaoAddr",
			cfg:     Config{BaoToken: "token", StorePath: "/tmp/test"},
			wantErr: ErrMissingBaoAddr,
		},
		{
			name:    "missing BaoToken",
			cfg:     Config{BaoAddr: "https://localhost:8200", StorePath: "/tmp/test"},
			wantErr: ErrMissingBaoToken,
		},
		{
			name:    "missing StorePath",
			cfg:     Config{BaoAddr: "https://localhost:8200", BaoToken: "token"},
			wantErr: ErrMissingStorePath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kr, err := New(context.Background(), tt.cfg)
			require.Error(t, err)
			assert.Nil(t, kr)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestNew_HealthCheckFailure(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusServiceUnavailable) // Sealed
			return
		}
	})

	server := httptest.NewTLSServer(handler)
	defer server.Close()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	cfg := Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	kr, err := New(context.Background(), cfg)
	require.Error(t, err)
	assert.Nil(t, kr)
	assert.Contains(t, err.Error(), "health check")
}

// ============================================
// Mock Helpers
// ============================================

// mockAddress implements sdk.Address for testing.
type mockAddress string

func (a mockAddress) String() string {
	return string(a)
}

func (a mockAddress) Bytes() []byte {
	return []byte(a)
}

func (a mockAddress) Equals(other sdk.Address) bool {
	if other == nil {
		return false
	}
	return a.String() == other.String()
}

func (a mockAddress) Empty() bool {
	return len(a) == 0
}

func (a mockAddress) Marshal() ([]byte, error) {
	return []byte(a), nil
}

func (a mockAddress) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(a))
}

func (a mockAddress) MarshalYAML() (interface{}, error) {
	return string(a), nil
}

func (a mockAddress) Unmarshal(data []byte) error {
	return nil
}

func (a mockAddress) UnmarshalJSON(data []byte) error {
	return nil
}

func (a mockAddress) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return nil
}

func (a mockAddress) Format(s fmt.State, verb rune) {
	_, _ = fmt.Fprintf(s, "%s", string(a))
}
