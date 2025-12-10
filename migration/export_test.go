package migration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Bidon15/banhbaoring"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================
// Test Helpers
// ============================================

// testPrivKeyBytes returns a valid 32-byte secp256k1 private key for testing.
func testPrivKeyBytes() []byte {
	privKey := make([]byte, 32)
	for i := 0; i < 32; i++ {
		privKey[i] = byte(i + 1)
	}
	return privKey
}

// setupTestBaoKeyring creates a BaoKeyring for testing with a mock server.
func setupTestBaoKeyring(t *testing.T, handler http.HandlerFunc) (*banhbaoring.BaoKeyring, *httptest.Server, string) {
	t.Helper()

	server := httptest.NewTLSServer(handler)

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	// Create client directly
	cfg := banhbaoring.Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	// We need to use the newBaoKeyringForTesting helper which is in the main package
	// Instead, let's create the keyring through the exported New function with a healthy server
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		handler(w, r)
	}

	server.Config.Handler = http.HandlerFunc(healthHandler)

	kr, err := banhbaoring.New(context.Background(), cfg)
	require.NoError(t, err)

	return kr, server, tmpDir
}

// setupTestBaoKeyringWithExportableKey creates a keyring with a pre-populated exportable key.
func setupTestBaoKeyringWithExportableKey(t *testing.T, uid string, exportable bool, handler http.HandlerFunc) (*banhbaoring.BaoKeyring, *httptest.Server, string) {
	t.Helper()

	kr, server, tmpDir := setupTestBaoKeyring(t, handler)

	// Create a wrapper handler that responds to health checks
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		handler(w, r)
	})

	pubKeyHex := "02" + "0102030405060708091011121314151617181920212223242526272829303132"
	expectedAddr := "cosmos1test123456789"

	// Create the key in OpenBao via the API (mocked)
	createHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/v1/secp256k1/keys/"+uid {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name":       uid,
					"public_key": pubKeyHex,
					"address":    expectedAddr,
					"exportable": exportable,
					"created_at": time.Now().Format(time.RFC3339),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		handler(w, r)
	}

	server.Config.Handler = http.HandlerFunc(createHandler)

	_, err := kr.NewAccountWithOptions(uid, banhbaoring.KeyOptions{Exportable: exportable})
	require.NoError(t, err)

	// Now switch back to the original handler for subsequent operations
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		handler(w, r)
	})

	return kr, server, tmpDir
}

// mockExportDestKeyring is a mock implementation of keyring.Keyring for export testing.
type mockExportDestKeyring struct {
	keys              map[string][]byte
	importErr         error
	signErr           error
	deleteErr         error
	importedKeys      []string
	importedPasswords []string
}

func newMockExportDestKeyring() *mockExportDestKeyring {
	return &mockExportDestKeyring{
		keys:              make(map[string][]byte),
		importedKeys:      make([]string, 0),
		importedPasswords: make([]string, 0),
	}
}

func (m *mockExportDestKeyring) ImportPrivKey(uid, armor, passphrase string) error {
	if m.importErr != nil {
		return m.importErr
	}
	m.keys[uid] = []byte(armor)
	m.importedKeys = append(m.importedKeys, uid)
	m.importedPasswords = append(m.importedPasswords, passphrase)
	return nil
}

func (m *mockExportDestKeyring) Sign(uid string, msg []byte, signMode signing.SignMode) ([]byte, cryptotypes.PubKey, error) {
	if m.signErr != nil {
		return nil, nil, m.signErr
	}
	if _, ok := m.keys[uid]; !ok {
		return nil, nil, banhbaoring.ErrKeyNotFound
	}
	// Return a mock 64-byte signature
	sig := make([]byte, 64)
	for i := range sig {
		sig[i] = byte(i)
	}
	return sig, nil, nil
}

func (m *mockExportDestKeyring) Delete(uid string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.keys, uid)
	return nil
}

// Implement the rest of keyring.Keyring interface with no-ops
func (m *mockExportDestKeyring) Backend() string { return "mock" }
func (m *mockExportDestKeyring) List() ([]*keyring.Record, error) {
	return nil, nil
}
func (m *mockExportDestKeyring) SupportedAlgorithms() (keyring.SigningAlgoList, keyring.SigningAlgoList) {
	return nil, nil
}
func (m *mockExportDestKeyring) Key(uid string) (*keyring.Record, error) {
	return nil, nil
}
func (m *mockExportDestKeyring) KeyByAddress(address sdk.Address) (*keyring.Record, error) {
	return nil, nil
}
func (m *mockExportDestKeyring) DeleteByAddress(address sdk.Address) error {
	return nil
}
func (m *mockExportDestKeyring) Rename(from, to string) error {
	return nil
}
func (m *mockExportDestKeyring) NewMnemonic(uid string, language keyring.Language, hdPath, bip39Passphrase string, algo keyring.SignatureAlgo) (*keyring.Record, string, error) {
	return nil, "", nil
}
func (m *mockExportDestKeyring) NewAccount(uid, mnemonic, bip39Passphrase, hdPath string, algo keyring.SignatureAlgo) (*keyring.Record, error) {
	return nil, nil
}
func (m *mockExportDestKeyring) SaveLedgerKey(uid string, algo keyring.SignatureAlgo, hrp string, coinType, account, index uint32) (*keyring.Record, error) {
	return nil, nil
}
func (m *mockExportDestKeyring) SaveOfflineKey(uid string, pubkey cryptotypes.PubKey) (*keyring.Record, error) {
	return nil, nil
}
func (m *mockExportDestKeyring) SaveMultisig(uid string, pubkey cryptotypes.PubKey) (*keyring.Record, error) {
	return nil, nil
}
func (m *mockExportDestKeyring) SignByAddress(address sdk.Address, msg []byte, signMode signing.SignMode) ([]byte, cryptotypes.PubKey, error) {
	return nil, nil, nil
}
func (m *mockExportDestKeyring) ImportPrivKeyHex(uid, privKey, algoStr string) error {
	return nil
}
func (m *mockExportDestKeyring) ImportPubKey(uid, armor string) error {
	return nil
}
func (m *mockExportDestKeyring) ExportPubKeyArmor(uid string) (string, error) {
	return "", nil
}
func (m *mockExportDestKeyring) ExportPubKeyArmorByAddress(address sdk.Address) (string, error) {
	return "", nil
}
func (m *mockExportDestKeyring) ExportPrivKeyArmor(uid, encryptPassphrase string) (armor string, err error) {
	return "", nil
}
func (m *mockExportDestKeyring) ExportPrivKeyArmorByAddress(address sdk.Address, encryptPassphrase string) (armor string, err error) {
	return "", nil
}
func (m *mockExportDestKeyring) MigrateAll() ([]*keyring.Record, error) {
	return nil, nil
}

var _ keyring.Keyring = (*mockExportDestKeyring)(nil)

// ============================================
// Export Tests
// ============================================

func TestExport_Success(t *testing.T) {
	privKeyBytes := testPrivKeyBytes()
	privKeyB64 := base64.StdEncoding.EncodeToString(privKeyBytes)

	handler := func(w http.ResponseWriter, r *http.Request) {
		// Handle export request
		if r.Method == http.MethodGet && r.URL.Path == "/v1/secp256k1/export/test-key" {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name":       "test-key",
					"public_key": "02" + "0102030405060708091011121314151617181920212223242526272829303132",
					"address":    "cosmos1test123456789",
					"keys": map[string]string{
						"1": privKeyB64,
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	kr, server, _ := setupTestBaoKeyringWithExportableKey(t, "test-key", true, handler)
	defer server.Close()
	defer func() { _ = kr.Close() }()

	destKeyring := newMockExportDestKeyring()

	cfg := ExportConfig{
		SourceKeyring: kr,
		DestKeyring:   destKeyring,
		KeyName:       "test-key",
		Confirmed:     true,
	}

	result, err := Export(context.Background(), cfg)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "test-key", result.KeyName)
	assert.NotEmpty(t, result.Address)

	// Verify key was imported to destination
	assert.Contains(t, destKeyring.importedKeys, "test-key")
}

func TestExport_WithNewKeyName(t *testing.T) {
	privKeyBytes := testPrivKeyBytes()
	privKeyB64 := base64.StdEncoding.EncodeToString(privKeyBytes)

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/secp256k1/export/test-key" {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name": "test-key",
					"keys": map[string]string{
						"1": privKeyB64,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	kr, server, _ := setupTestBaoKeyringWithExportableKey(t, "test-key", true, handler)
	defer server.Close()
	defer func() { _ = kr.Close() }()

	destKeyring := newMockExportDestKeyring()

	cfg := ExportConfig{
		SourceKeyring: kr,
		DestKeyring:   destKeyring,
		KeyName:       "test-key",
		NewKeyName:    "renamed-key",
		Confirmed:     true,
	}

	result, err := Export(context.Background(), cfg)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "renamed-key", result.KeyName)
	assert.Contains(t, destKeyring.importedKeys, "renamed-key")
}

func TestExport_NotConfirmed(t *testing.T) {
	kr, server, _ := setupTestBaoKeyring(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()
	defer func() { _ = kr.Close() }()

	destKeyring := newMockExportDestKeyring()

	cfg := ExportConfig{
		SourceKeyring: kr,
		DestKeyring:   destKeyring,
		KeyName:       "test-key",
		Confirmed:     false, // Not confirmed
	}

	result, err := Export(context.Background(), cfg)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrExportNotConfirmed)
}

func TestExport_MissingSourceKeyring(t *testing.T) {
	destKeyring := newMockExportDestKeyring()

	cfg := ExportConfig{
		SourceKeyring: nil, // Missing
		DestKeyring:   destKeyring,
		KeyName:       "test-key",
		Confirmed:     true,
	}

	result, err := Export(context.Background(), cfg)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "source keyring is required")
}

func TestExport_MissingDestKeyring(t *testing.T) {
	kr, server, _ := setupTestBaoKeyring(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()
	defer func() { _ = kr.Close() }()

	cfg := ExportConfig{
		SourceKeyring: kr,
		DestKeyring:   nil, // Missing
		KeyName:       "test-key",
		Confirmed:     true,
	}

	result, err := Export(context.Background(), cfg)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "destination keyring is required")
}

func TestExport_MissingKeyName(t *testing.T) {
	kr, server, _ := setupTestBaoKeyring(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()
	defer func() { _ = kr.Close() }()

	destKeyring := newMockExportDestKeyring()

	cfg := ExportConfig{
		SourceKeyring: kr,
		DestKeyring:   destKeyring,
		KeyName:       "", // Missing
		Confirmed:     true,
	}

	result, err := Export(context.Background(), cfg)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "key name is required")
}

func TestExport_KeyNotExportable(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}

	// Create a non-exportable key
	kr, server, _ := setupTestBaoKeyringWithExportableKey(t, "non-exportable-key", false, handler)
	defer server.Close()
	defer func() { _ = kr.Close() }()

	destKeyring := newMockExportDestKeyring()

	cfg := ExportConfig{
		SourceKeyring: kr,
		DestKeyring:   destKeyring,
		KeyName:       "non-exportable-key",
		Confirmed:     true,
	}

	result, err := Export(context.Background(), cfg)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, banhbaoring.ErrKeyNotExportable)
}

func TestExport_KeyNotFound(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}

	kr, server, _ := setupTestBaoKeyring(t, handler)
	defer server.Close()
	defer func() { _ = kr.Close() }()

	destKeyring := newMockExportDestKeyring()

	cfg := ExportConfig{
		SourceKeyring: kr,
		DestKeyring:   destKeyring,
		KeyName:       "nonexistent-key",
		Confirmed:     true,
	}

	result, err := Export(context.Background(), cfg)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, banhbaoring.ErrKeyNotFound)
}

func TestExport_WithVerification(t *testing.T) {
	privKeyBytes := testPrivKeyBytes()
	privKeyB64 := base64.StdEncoding.EncodeToString(privKeyBytes)

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/secp256k1/export/test-key" {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name": "test-key",
					"keys": map[string]string{
						"1": privKeyB64,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	kr, server, _ := setupTestBaoKeyringWithExportableKey(t, "test-key", true, handler)
	defer server.Close()
	defer func() { _ = kr.Close() }()

	destKeyring := newMockExportDestKeyring()

	cfg := ExportConfig{
		SourceKeyring:     kr,
		DestKeyring:       destKeyring,
		KeyName:           "test-key",
		VerifyAfterExport: true,
		Confirmed:         true,
	}

	result, err := Export(context.Background(), cfg)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Verified)
}

func TestExport_WithDeleteAfterExport(t *testing.T) {
	privKeyBytes := testPrivKeyBytes()
	privKeyB64 := base64.StdEncoding.EncodeToString(privKeyBytes)
	deleted := false

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/secp256k1/export/test-key" {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name": "test-key",
					"keys": map[string]string{
						"1": privKeyB64,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		// Handle delete (config and actual delete)
		if r.Method == http.MethodPost || r.Method == http.MethodDelete {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	kr, server, _ := setupTestBaoKeyringWithExportableKey(t, "test-key", true, handler)
	defer server.Close()
	defer func() { _ = kr.Close() }()

	destKeyring := newMockExportDestKeyring()

	cfg := ExportConfig{
		SourceKeyring:     kr,
		DestKeyring:       destKeyring,
		KeyName:           "test-key",
		DeleteAfterExport: true,
		Confirmed:         true,
	}

	result, err := Export(context.Background(), cfg)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, deleted, "key should have been deleted from source")
}

func TestExport_DeleteFailsOnVerificationFailure(t *testing.T) {
	privKeyBytes := testPrivKeyBytes()
	privKeyB64 := base64.StdEncoding.EncodeToString(privKeyBytes)

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/secp256k1/export/test-key" {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name": "test-key",
					"keys": map[string]string{
						"1": privKeyB64,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	kr, server, _ := setupTestBaoKeyringWithExportableKey(t, "test-key", true, handler)
	defer server.Close()
	defer func() { _ = kr.Close() }()

	destKeyring := newMockExportDestKeyring()
	destKeyring.signErr = banhbaoring.ErrSigningFailed // Make verification fail

	cfg := ExportConfig{
		SourceKeyring:     kr,
		DestKeyring:       destKeyring,
		KeyName:           "test-key",
		DeleteAfterExport: true,
		VerifyAfterExport: true,
		Confirmed:         true,
	}

	result, err := Export(context.Background(), cfg)

	// Should return an error about verification failure
	assert.Error(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Verified)
	assert.Contains(t, err.Error(), "verification failed")
}

// ============================================
// ValidateExport Tests
// ============================================

func TestValidateExport_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	kr, server, _ := setupTestBaoKeyringWithExportableKey(t, "test-key", true, handler)
	defer server.Close()
	defer func() { _ = kr.Close() }()

	cfg := ExportConfig{
		SourceKeyring: kr,
		KeyName:       "test-key",
	}

	err := ValidateExport(context.Background(), cfg)

	assert.NoError(t, err)
}

func TestValidateExport_KeyNotExportable(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	kr, server, _ := setupTestBaoKeyringWithExportableKey(t, "non-exportable", false, handler)
	defer server.Close()
	defer func() { _ = kr.Close() }()

	cfg := ExportConfig{
		SourceKeyring: kr,
		KeyName:       "non-exportable",
	}

	err := ValidateExport(context.Background(), cfg)

	assert.Error(t, err)
	assert.ErrorIs(t, err, banhbaoring.ErrKeyNotExportable)
}

func TestValidateExport_KeyNotFound(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}

	kr, server, _ := setupTestBaoKeyring(t, handler)
	defer server.Close()
	defer func() { _ = kr.Close() }()

	cfg := ExportConfig{
		SourceKeyring: kr,
		KeyName:       "nonexistent",
	}

	err := ValidateExport(context.Background(), cfg)

	assert.Error(t, err)
	assert.ErrorIs(t, err, banhbaoring.ErrKeyNotFound)
}

func TestValidateExport_MissingSourceKeyring(t *testing.T) {
	cfg := ExportConfig{
		SourceKeyring: nil,
		KeyName:       "test-key",
	}

	err := ValidateExport(context.Background(), cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source keyring is required")
}

func TestValidateExport_MissingKeyName(t *testing.T) {
	kr, server, _ := setupTestBaoKeyring(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()
	defer func() { _ = kr.Close() }()

	cfg := ExportConfig{
		SourceKeyring: kr,
		KeyName:       "",
	}

	err := ValidateExport(context.Background(), cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key name is required")
}

// ============================================
// SecurityWarning Tests
// ============================================

func TestSecurityWarning_Format(t *testing.T) {
	warning := SecurityWarning("my-key", "cosmos1abc123", "/path/to/keyring")

	assert.Contains(t, warning, "SECURITY WARNING")
	assert.Contains(t, warning, "my-key")
	assert.Contains(t, warning, "cosmos1abc123")
	assert.Contains(t, warning, "/path/to/keyring")
	assert.Contains(t, warning, "EXPORT")
	assert.Contains(t, warning, "OpenBao")
}

func TestSecurityWarning_EmptyValues(t *testing.T) {
	warning := SecurityWarning("", "", "")

	// Should still produce a valid warning even with empty values
	assert.Contains(t, warning, "SECURITY WARNING")
	assert.Contains(t, warning, "Key:")
	assert.Contains(t, warning, "Address:")
	assert.Contains(t, warning, "Destination:")
}

// ============================================
// Helper Function Tests
// ============================================

func TestSecureZero(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}

	secureZero(data)

	for i, b := range data {
		assert.Equal(t, byte(0), b, "byte at index %d should be zero", i)
	}
}

func TestSecureZero_EmptySlice(t *testing.T) {
	data := []byte{}

	// Should not panic
	secureZero(data)
	assert.Empty(t, data)
}

func TestSecureZeroString(t *testing.T) {
	s := "secret data"

	secureZeroString(&s)

	assert.Empty(t, s)
}

func TestSecureZeroString_EmptyString(t *testing.T) {
	s := ""

	// Should not panic
	secureZeroString(&s)
	assert.Empty(t, s)
}

func TestSecureZeroString_NilPointer(t *testing.T) {
	// Should not panic
	secureZeroString(nil)
}

func TestVerifyLocalKey_Success(t *testing.T) {
	destKeyring := newMockExportDestKeyring()
	destKeyring.keys["test-key"] = []byte("test")

	result := verifyLocalKey(destKeyring, "test-key")

	assert.True(t, result)
}

func TestVerifyLocalKey_KeyNotFound(t *testing.T) {
	destKeyring := newMockExportDestKeyring()

	result := verifyLocalKey(destKeyring, "nonexistent")

	assert.False(t, result)
}

func TestVerifyLocalKey_SignError(t *testing.T) {
	destKeyring := newMockExportDestKeyring()
	destKeyring.keys["test-key"] = []byte("test")
	destKeyring.signErr = banhbaoring.ErrSigningFailed

	result := verifyLocalKey(destKeyring, "test-key")

	assert.False(t, result)
}

// ============================================
// ErrExportNotConfirmed Tests
// ============================================

func TestErrExportNotConfirmed(t *testing.T) {
	assert.Equal(t, "export requires user confirmation", ErrExportNotConfirmed.Error())
}

// ============================================
// Import Error Tests for Export
// ============================================

func TestExport_ImportError(t *testing.T) {
	privKeyBytes := testPrivKeyBytes()
	privKeyB64 := base64.StdEncoding.EncodeToString(privKeyBytes)

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/secp256k1/export/test-key" {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name": "test-key",
					"keys": map[string]string{
						"1": privKeyB64,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	kr, server, _ := setupTestBaoKeyringWithExportableKey(t, "test-key", true, handler)
	defer server.Close()
	defer func() { _ = kr.Close() }()

	destKeyring := newMockExportDestKeyring()
	destKeyring.importErr = banhbaoring.ErrKeyExists // Simulate import failure

	cfg := ExportConfig{
		SourceKeyring: kr,
		DestKeyring:   destKeyring,
		KeyName:       "test-key",
		Confirmed:     true,
	}

	result, err := Export(context.Background(), cfg)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "import to local keyring")
}

func TestExport_OpenBaoExportError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/secp256k1/export/test-key" {
			// Return an error from OpenBao
			w.WriteHeader(http.StatusForbidden)
			resp := map[string]interface{}{
				"errors": []string{"permission denied"},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	kr, server, _ := setupTestBaoKeyringWithExportableKey(t, "test-key", true, handler)
	defer server.Close()
	defer func() { _ = kr.Close() }()

	destKeyring := newMockExportDestKeyring()

	cfg := ExportConfig{
		SourceKeyring: kr,
		DestKeyring:   destKeyring,
		KeyName:       "test-key",
		Confirmed:     true,
	}

	result, err := Export(context.Background(), cfg)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "export from OpenBao")
}
