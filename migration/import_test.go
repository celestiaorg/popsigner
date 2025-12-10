package migration

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Bidon15/banhbaoring"
	"github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================
// Mock Source Keyring (implements keyring.Keyring)
// ============================================

// mockSourceKeyring implements keyring.Keyring for testing import from local keyrings.
type mockSourceKeyring struct {
	keys        map[string]*secp256k1.PrivKey
	records     map[string]*keyring.Record
	exportError error
	listError   error
	keyError    error
	deleteError error
	deletedKeys []string
}

func newMockSourceKeyring() *mockSourceKeyring {
	return &mockSourceKeyring{
		keys:        make(map[string]*secp256k1.PrivKey),
		records:     make(map[string]*keyring.Record),
		deletedKeys: make([]string, 0),
	}
}

func (m *mockSourceKeyring) addKey(name string, privKeyBytes []byte) error {
	privKey := &secp256k1.PrivKey{Key: privKeyBytes}
	m.keys[name] = privKey

	record, err := keyring.NewLocalRecord(name, privKey, privKey.PubKey())
	if err != nil {
		return err
	}
	m.records[name] = record
	return nil
}

func (m *mockSourceKeyring) Backend() string { return "test" }

func (m *mockSourceKeyring) SupportedAlgorithms() (keyring.SigningAlgoList, keyring.SigningAlgoList) {
	return nil, nil
}

func (m *mockSourceKeyring) List() ([]*keyring.Record, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	records := make([]*keyring.Record, 0, len(m.records))
	for _, r := range m.records {
		records = append(records, r)
	}
	return records, nil
}

func (m *mockSourceKeyring) Key(uid string) (*keyring.Record, error) {
	if m.keyError != nil {
		return nil, m.keyError
	}
	r, ok := m.records[uid]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", uid)
	}
	return r, nil
}

func (m *mockSourceKeyring) KeyByAddress(address sdk.Address) (*keyring.Record, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) Delete(uid string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	m.deletedKeys = append(m.deletedKeys, uid)
	delete(m.keys, uid)
	delete(m.records, uid)
	return nil
}

func (m *mockSourceKeyring) DeleteByAddress(address sdk.Address) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) Rename(from, to string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) NewMnemonic(uid string, language keyring.Language, hdPath, bip39Passphrase string, algo keyring.SignatureAlgo) (*keyring.Record, string, error) {
	return nil, "", fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) NewAccount(uid, mnemonic, bip39Passphrase, hdPath string, algo keyring.SignatureAlgo) (*keyring.Record, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) SaveLedgerKey(uid string, algo keyring.SignatureAlgo, hrp string, coinType, account, index uint32) (*keyring.Record, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) SaveMultisig(uid string, pubkey cryptotypes.PubKey) (*keyring.Record, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) SaveOfflineKey(uid string, pubkey cryptotypes.PubKey) (*keyring.Record, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) Sign(uid string, msg []byte, signMode signing.SignMode) ([]byte, cryptotypes.PubKey, error) {
	privKey, ok := m.keys[uid]
	if !ok {
		return nil, nil, fmt.Errorf("key not found: %s", uid)
	}
	sig, err := privKey.Sign(msg)
	if err != nil {
		return nil, nil, err
	}
	return sig, privKey.PubKey(), nil
}

func (m *mockSourceKeyring) SignByAddress(address sdk.Address, msg []byte, signMode signing.SignMode) ([]byte, cryptotypes.PubKey, error) {
	return nil, nil, fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) ImportPrivKey(uid, armor, passphrase string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) ImportPrivKeyHex(uid, privKey, algoStr string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) ImportPubKey(uid, armor string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) ExportPubKeyArmor(uid string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) ExportPubKeyArmorByAddress(address sdk.Address) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) ExportPrivKeyArmor(uid, encryptPassphrase string) (armor string, err error) {
	if m.exportError != nil {
		return "", m.exportError
	}
	privKey, ok := m.keys[uid]
	if !ok {
		return "", fmt.Errorf("key not found: %s", uid)
	}
	// Use the SDK's proper armor function
	return crypto.EncryptArmorPrivKey(privKey, encryptPassphrase, privKey.Type()), nil
}

func (m *mockSourceKeyring) ExportPrivKeyArmorByAddress(address sdk.Address, encryptPassphrase string) (armor string, err error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockSourceKeyring) MigrateAll() ([]*keyring.Record, error) {
	return nil, fmt.Errorf("not implemented")
}

var _ keyring.Keyring = (*mockSourceKeyring)(nil)

// ============================================
// Test Helpers
// ============================================

// generateTestKey generates a valid secp256k1 private key for testing.
func generateTestKey() []byte {
	privKey := secp256k1.GenPrivKey()
	return privKey.Key
}

// setupImportTestKeyring creates a BaoKeyring for testing with import/sign handlers.
// ============================================
// Import Tests
// ============================================

func TestImport_Success(t *testing.T) {
	privKeyBytes := generateTestKey()
	privKey := &secp256k1.PrivKey{Key: privKeyBytes}
	pubKeyHex := hex.EncodeToString(privKey.PubKey().Bytes())
	addr := privKey.PubKey().Address()
	addrHex := hex.EncodeToString(addr.Bytes())

	importHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/secp256k1/keys/testkey/import" {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name":       "testkey",
					"public_key": pubKeyHex,
					"address":    addrHex,
					"exportable": true,
					"imported":   true,
					"created_at": time.Now().Format(time.RFC3339),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	signHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/secp256k1/sign/testkey" {
			sig := make([]byte, 64)
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"signature":   base64.StdEncoding.EncodeToString(sig),
					"public_key":  pubKeyHex,
					"key_version": 1,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	// Create handler that routes both
	combinedHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/v1/secp256k1/keys/testkey/import" {
			importHandler(w, r)
			return
		}
		if r.URL.Path == "/v1/secp256k1/sign/testkey" {
			signHandler(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(combinedHandler))
	defer server.Close()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	cfg := banhbaoring.Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	destKr, err := banhbaoring.New(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = destKr.Close() }()

	// Create source keyring with a test key
	sourceKr := newMockSourceKeyring()
	err = sourceKr.addKey("testkey", privKeyBytes)
	require.NoError(t, err)

	// Perform import
	ctx := context.Background()
	importCfg := ImportConfig{
		SourceKeyring:     sourceKr,
		DestKeyring:       destKr,
		KeyName:           "testkey",
		Exportable:        true,
		VerifyAfterImport: true,
	}

	result, err := Import(ctx, importCfg)
	require.NoError(t, err)

	assert.Equal(t, "testkey", result.KeyName)
	assert.NotEmpty(t, result.Address)
	assert.NotEmpty(t, result.PubKey)
	assert.True(t, result.Verified)
}

func TestImport_WithNewKeyName(t *testing.T) {
	privKeyBytes := generateTestKey()
	privKey := &secp256k1.PrivKey{Key: privKeyBytes}
	pubKeyHex := hex.EncodeToString(privKey.PubKey().Bytes())
	addrHex := hex.EncodeToString(privKey.PubKey().Address().Bytes())

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/v1/secp256k1/keys/newname/import" {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name":       "newname",
					"public_key": pubKeyHex,
					"address":    addrHex,
					"exportable": false,
					"imported":   true,
					"created_at": time.Now().Format(time.RFC3339),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	cfg := banhbaoring.Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	destKr, err := banhbaoring.New(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = destKr.Close() }()

	sourceKr := newMockSourceKeyring()
	_ = sourceKr.addKey("oldname", privKeyBytes)

	ctx := context.Background()
	importCfg := ImportConfig{
		SourceKeyring: sourceKr,
		DestKeyring:   destKr,
		KeyName:       "oldname",
		NewKeyName:    "newname",
		Exportable:    false,
	}

	result, err := Import(ctx, importCfg)
	require.NoError(t, err)

	assert.Equal(t, "newname", result.KeyName)
}

func TestImport_DeleteAfterImport(t *testing.T) {
	privKeyBytes := generateTestKey()
	privKey := &secp256k1.PrivKey{Key: privKeyBytes}
	pubKeyHex := hex.EncodeToString(privKey.PubKey().Bytes())
	addrHex := hex.EncodeToString(privKey.PubKey().Address().Bytes())

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/v1/secp256k1/keys/deletekey/import" {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name":       "deletekey",
					"public_key": pubKeyHex,
					"address":    addrHex,
					"exportable": false,
					"imported":   true,
					"created_at": time.Now().Format(time.RFC3339),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	cfg := banhbaoring.Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	destKr, err := banhbaoring.New(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = destKr.Close() }()

	sourceKr := newMockSourceKeyring()
	_ = sourceKr.addKey("deletekey", privKeyBytes)

	ctx := context.Background()
	importCfg := ImportConfig{
		SourceKeyring:     sourceKr,
		DestKeyring:       destKr,
		KeyName:           "deletekey",
		DeleteAfterImport: true,
	}

	_, err = Import(ctx, importCfg)
	require.NoError(t, err)

	// Verify key was deleted from source
	assert.Contains(t, sourceKr.deletedKeys, "deletekey")
}

func TestImport_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ImportConfig
		wantErr string
	}{
		{
			name:    "nil source keyring",
			cfg:     ImportConfig{DestKeyring: &banhbaoring.BaoKeyring{}, KeyName: "test"},
			wantErr: "source keyring is required",
		},
		{
			name:    "nil dest keyring",
			cfg:     ImportConfig{SourceKeyring: newMockSourceKeyring(), KeyName: "test"},
			wantErr: "destination keyring is required",
		},
		{
			name:    "empty key name",
			cfg:     ImportConfig{SourceKeyring: newMockSourceKeyring(), DestKeyring: &banhbaoring.BaoKeyring{}},
			wantErr: "key name is required",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Import(ctx, tt.cfg)
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}
}

func TestImport_ExportError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	cfg := banhbaoring.Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	destKr, err := banhbaoring.New(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = destKr.Close() }()

	sourceKr := newMockSourceKeyring()
	privKeyBytes := generateTestKey()
	_ = sourceKr.addKey("testkey", privKeyBytes)
	sourceKr.exportError = fmt.Errorf("export failed")

	ctx := context.Background()
	importCfg := ImportConfig{
		SourceKeyring: sourceKr,
		DestKeyring:   destKr,
		KeyName:       "testkey",
	}

	_, err = Import(ctx, importCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "export failed")
}

func TestImport_ImportError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/v1/secp256k1/keys/testkey/import" {
			w.WriteHeader(http.StatusBadRequest)
			resp := map[string]interface{}{
				"errors": []string{"import failed"},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	cfg := banhbaoring.Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	destKr, err := banhbaoring.New(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = destKr.Close() }()

	sourceKr := newMockSourceKeyring()
	privKeyBytes := generateTestKey()
	_ = sourceKr.addKey("testkey", privKeyBytes)

	ctx := context.Background()
	importCfg := ImportConfig{
		SourceKeyring: sourceKr,
		DestKeyring:   destKr,
		KeyName:       "testkey",
	}

	_, err = Import(ctx, importCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to import key to destination")
}

// ============================================
// BatchImport Tests
// ============================================

func TestBatchImport_Success(t *testing.T) {
	keys := make(map[string][]byte)
	keyInfos := make(map[string]map[string]string)

	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("key%d", i)
		privKeyBytes := generateTestKey()
		keys[name] = privKeyBytes
		privKey := &secp256k1.PrivKey{Key: privKeyBytes}
		keyInfos[name] = map[string]string{
			"public_key": hex.EncodeToString(privKey.PubKey().Bytes()),
			"address":    hex.EncodeToString(privKey.PubKey().Address().Bytes()),
		}
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		for name, info := range keyInfos {
			if r.URL.Path == fmt.Sprintf("/v1/secp256k1/keys/%s/import", name) {
				resp := map[string]interface{}{
					"data": map[string]interface{}{
						"name":       name,
						"public_key": info["public_key"],
						"address":    info["address"],
						"exportable": true,
						"imported":   true,
						"created_at": time.Now().Format(time.RFC3339),
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	cfg := banhbaoring.Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	destKr, err := banhbaoring.New(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = destKr.Close() }()

	sourceKr := newMockSourceKeyring()
	for name, privKeyBytes := range keys {
		_ = sourceKr.addKey(name, privKeyBytes)
	}

	ctx := context.Background()
	batchCfg := BatchImportConfig{
		SourceKeyring: sourceKr,
		DestKeyring:   destKr,
		Exportable:    true,
	}

	result, err := BatchImport(ctx, batchCfg)
	require.NoError(t, err)

	assert.Len(t, result.Successful, 3)
	assert.Len(t, result.Failed, 0)
}

func TestBatchImport_PartialFailure(t *testing.T) {
	keys := make(map[string][]byte)
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("key%d", i)
		keys[name] = generateTestKey()
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Fail key2
		if r.URL.Path == "/v1/secp256k1/keys/key2/import" {
			w.WriteHeader(http.StatusBadRequest)
			resp := map[string]interface{}{
				"errors": []string{"key2 import failed"},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		// Succeed for key1 and key3
		for name, privKeyBytes := range keys {
			if name == "key2" {
				continue
			}
			if r.URL.Path == fmt.Sprintf("/v1/secp256k1/keys/%s/import", name) {
				privKey := &secp256k1.PrivKey{Key: privKeyBytes}
				resp := map[string]interface{}{
					"data": map[string]interface{}{
						"name":       name,
						"public_key": hex.EncodeToString(privKey.PubKey().Bytes()),
						"address":    hex.EncodeToString(privKey.PubKey().Address().Bytes()),
						"exportable": true,
						"imported":   true,
						"created_at": time.Now().Format(time.RFC3339),
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	cfg := banhbaoring.Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	destKr, err := banhbaoring.New(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = destKr.Close() }()

	sourceKr := newMockSourceKeyring()
	for name, privKeyBytes := range keys {
		_ = sourceKr.addKey(name, privKeyBytes)
	}

	ctx := context.Background()
	batchCfg := BatchImportConfig{
		SourceKeyring: sourceKr,
		DestKeyring:   destKr,
		KeyNames:      []string{"key1", "key2", "key3"},
	}

	result, err := BatchImport(ctx, batchCfg)
	require.NoError(t, err)

	assert.Len(t, result.Successful, 2)
	assert.Len(t, result.Failed, 1)
	assert.Equal(t, "key2", result.Failed[0].KeyName)
}

func TestBatchImport_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     BatchImportConfig
		wantErr string
	}{
		{
			name:    "nil source keyring",
			cfg:     BatchImportConfig{DestKeyring: &banhbaoring.BaoKeyring{}},
			wantErr: "source keyring is required",
		},
		{
			name:    "nil dest keyring",
			cfg:     BatchImportConfig{SourceKeyring: newMockSourceKeyring()},
			wantErr: "destination keyring is required",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BatchImport(ctx, tt.cfg)
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}
}

func TestBatchImport_ContextCancellation(t *testing.T) {
	sourceKr := newMockSourceKeyring()
	for i := 1; i <= 5; i++ {
		_ = sourceKr.addKey(fmt.Sprintf("key%d", i), generateTestKey())
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	cfg := banhbaoring.Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	destKr, err := banhbaoring.New(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = destKr.Close() }()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	batchCfg := BatchImportConfig{
		SourceKeyring: sourceKr,
		DestKeyring:   destKr,
		KeyNames:      []string{"key1", "key2", "key3", "key4", "key5"},
	}

	result, err := BatchImport(ctx, batchCfg)
	// Either error or partial failure due to cancellation
	if err == nil {
		// Should have failures due to context cancellation
		assert.NotEmpty(t, result.Failed)
	}
}

// ============================================
// Helper Function Tests
// ============================================

func TestListSourceKeys_Success(t *testing.T) {
	sourceKr := newMockSourceKeyring()
	for i := 1; i <= 3; i++ {
		_ = sourceKr.addKey(fmt.Sprintf("key%d", i), generateTestKey())
	}

	names, err := ListSourceKeys(sourceKr)
	require.NoError(t, err)
	assert.Len(t, names, 3)
}

func TestListSourceKeys_Error(t *testing.T) {
	sourceKr := newMockSourceKeyring()
	sourceKr.listError = fmt.Errorf("list failed")

	_, err := ListSourceKeys(sourceKr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list failed")
}

func TestListSourceKeys_NilKeyring(t *testing.T) {
	_, err := ListSourceKeys(nil)
	require.Error(t, err)
	assert.Equal(t, "keyring is required", err.Error())
}

func TestValidateSourceKey_Success(t *testing.T) {
	sourceKr := newMockSourceKeyring()
	_ = sourceKr.addKey("validkey", generateTestKey())

	err := ValidateSourceKey(sourceKr, "validkey")
	require.NoError(t, err)
}

func TestValidateSourceKey_NotFound(t *testing.T) {
	sourceKr := newMockSourceKeyring()

	err := ValidateSourceKey(sourceKr, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
}

func TestValidateSourceKey_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		kr      keyring.Keyring
		keyName string
		wantErr string
	}{
		{
			name:    "nil keyring",
			kr:      nil,
			keyName: "test",
			wantErr: "keyring is required",
		},
		{
			name:    "empty key name",
			kr:      newMockSourceKeyring(),
			keyName: "",
			wantErr: "key name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSourceKey(tt.kr, tt.keyName)
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}
}

func TestValidateSourceKey_ExportError(t *testing.T) {
	sourceKr := newMockSourceKeyring()
	_ = sourceKr.addKey("testkey", generateTestKey())
	sourceKr.exportError = fmt.Errorf("cannot export")

	err := ValidateSourceKey(sourceKr, "testkey")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot export")
}

// ============================================
// VerifyKey Tests
// ============================================

func TestImport_VerificationFails(t *testing.T) {
	privKeyBytes := generateTestKey()
	privKey := &secp256k1.PrivKey{Key: privKeyBytes}
	pubKeyHex := hex.EncodeToString(privKey.PubKey().Bytes())
	addrHex := hex.EncodeToString(privKey.PubKey().Address().Bytes())

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/v1/secp256k1/keys/testkey/import" {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name":       "testkey",
					"public_key": pubKeyHex,
					"address":    addrHex,
					"exportable": true,
					"imported":   true,
					"created_at": time.Now().Format(time.RFC3339),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		// Sign endpoint fails
		if r.URL.Path == "/v1/secp256k1/sign/testkey" {
			w.WriteHeader(http.StatusInternalServerError)
			resp := map[string]interface{}{
				"errors": []string{"signing failed"},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	cfg := banhbaoring.Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	destKr, err := banhbaoring.New(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = destKr.Close() }()

	sourceKr := newMockSourceKeyring()
	_ = sourceKr.addKey("testkey", privKeyBytes)

	ctx := context.Background()
	importCfg := ImportConfig{
		SourceKeyring:     sourceKr,
		DestKeyring:       destKr,
		KeyName:           "testkey",
		VerifyAfterImport: true,
	}

	result, err := Import(ctx, importCfg)
	require.NoError(t, err) // Import should succeed even if verification fails

	assert.Equal(t, "testkey", result.KeyName)
	assert.False(t, result.Verified) // Verification should fail
}

func TestImport_DeleteSkippedOnVerificationFailure(t *testing.T) {
	privKeyBytes := generateTestKey()
	privKey := &secp256k1.PrivKey{Key: privKeyBytes}
	pubKeyHex := hex.EncodeToString(privKey.PubKey().Bytes())
	addrHex := hex.EncodeToString(privKey.PubKey().Address().Bytes())

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/v1/secp256k1/keys/testkey/import" {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name":       "testkey",
					"public_key": pubKeyHex,
					"address":    addrHex,
					"exportable": true,
					"imported":   true,
					"created_at": time.Now().Format(time.RFC3339),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		// Sign endpoint fails
		if r.URL.Path == "/v1/secp256k1/sign/testkey" {
			w.WriteHeader(http.StatusInternalServerError)
			resp := map[string]interface{}{
				"errors": []string{"signing failed"},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	cfg := banhbaoring.Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	destKr, err := banhbaoring.New(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = destKr.Close() }()

	sourceKr := newMockSourceKeyring()
	_ = sourceKr.addKey("testkey", privKeyBytes)

	ctx := context.Background()
	importCfg := ImportConfig{
		SourceKeyring:     sourceKr,
		DestKeyring:       destKr,
		KeyName:           "testkey",
		VerifyAfterImport: true,
		DeleteAfterImport: true, // Should NOT delete since verification fails
	}

	result, err := Import(ctx, importCfg)
	require.NoError(t, err)

	assert.False(t, result.Verified)
	// Key should NOT be deleted from source since verification failed
	assert.Empty(t, sourceKr.deletedKeys)
}

// ============================================
// Public Key Matching Tests
// ============================================

func TestImport_PublicKeyMatch(t *testing.T) {
	privKeyBytes := generateTestKey()
	privKey := &secp256k1.PrivKey{Key: privKeyBytes}
	expectedPubKey := privKey.PubKey().Bytes()
	pubKeyHex := hex.EncodeToString(expectedPubKey)
	addrHex := hex.EncodeToString(privKey.PubKey().Address().Bytes())

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/v1/secp256k1/keys/testkey/import" {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"name":       "testkey",
					"public_key": pubKeyHex,
					"address":    addrHex,
					"exportable": true,
					"imported":   true,
					"created_at": time.Now().Format(time.RFC3339),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer server.Close()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "keyring.json")

	cfg := banhbaoring.Config{
		BaoAddr:       server.URL,
		BaoToken:      "test-token",
		StorePath:     storePath,
		SkipTLSVerify: true,
	}

	destKr, err := banhbaoring.New(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = destKr.Close() }()

	sourceKr := newMockSourceKeyring()
	_ = sourceKr.addKey("testkey", privKeyBytes)

	ctx := context.Background()
	importCfg := ImportConfig{
		SourceKeyring: sourceKr,
		DestKeyring:   destKr,
		KeyName:       "testkey",
	}

	result, err := Import(ctx, importCfg)
	require.NoError(t, err)

	// Verify public keys match
	assert.Equal(t, hex.EncodeToString(result.PubKey), hex.EncodeToString(expectedPubKey))
}
