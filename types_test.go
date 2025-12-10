package banhbaoring

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_WithDefaults(t *testing.T) {
	cfg := Config{BaoAddr: "http://localhost:8200"}
	cfg = cfg.WithDefaults()

	assert.Equal(t, DefaultSecp256k1Path, cfg.Secp256k1Path)
	assert.Equal(t, DefaultHTTPTimeout, cfg.HTTPTimeout)
}

func TestConfig_WithDefaults_PreservesExisting(t *testing.T) {
	cfg := Config{
		Secp256k1Path: "custom-path",
		HTTPTimeout:   60 * time.Second,
	}
	cfg = cfg.WithDefaults()

	assert.Equal(t, "custom-path", cfg.Secp256k1Path)
	assert.Equal(t, 60*time.Second, cfg.HTTPTimeout)
}

func TestConfig_WithDefaults_PreservesAllFields(t *testing.T) {
	tlsCfg := &tls.Config{InsecureSkipVerify: true}
	cfg := Config{
		BaoAddr:       "https://vault.example.com:8200",
		BaoToken:      "hvs.test-token",
		BaoNamespace:  "my-namespace",
		Secp256k1Path: "custom-secp",
		StorePath:     "/tmp/store.json",
		HTTPTimeout:   45 * time.Second,
		TLSConfig:     tlsCfg,
		SkipTLSVerify: true,
	}
	result := cfg.WithDefaults()

	// Verify all fields are preserved
	assert.Equal(t, "https://vault.example.com:8200", result.BaoAddr)
	assert.Equal(t, "hvs.test-token", result.BaoToken)
	assert.Equal(t, "my-namespace", result.BaoNamespace)
	assert.Equal(t, "custom-secp", result.Secp256k1Path)
	assert.Equal(t, "/tmp/store.json", result.StorePath)
	assert.Equal(t, 45*time.Second, result.HTTPTimeout)
	assert.Same(t, tlsCfg, result.TLSConfig)
	assert.True(t, result.SkipTLSVerify)
}

func TestConfig_WithDefaults_PartialDefaults(t *testing.T) {
	// Test with only Secp256k1Path set (HTTPTimeout should get default)
	cfg := Config{
		BaoAddr:       "http://localhost:8200",
		Secp256k1Path: "my-path",
	}
	result := cfg.WithDefaults()

	assert.Equal(t, "my-path", result.Secp256k1Path)
	assert.Equal(t, DefaultHTTPTimeout, result.HTTPTimeout)

	// Test with only HTTPTimeout set (Secp256k1Path should get default)
	cfg2 := Config{
		BaoAddr:     "http://localhost:8200",
		HTTPTimeout: 10 * time.Second,
	}
	result2 := cfg2.WithDefaults()

	assert.Equal(t, DefaultSecp256k1Path, result2.Secp256k1Path)
	assert.Equal(t, 10*time.Second, result2.HTTPTimeout)
}

func TestConstants(t *testing.T) {
	// Verify algorithm constant
	assert.Equal(t, "secp256k1", AlgorithmSecp256k1)

	// Verify default constants
	assert.Equal(t, "secp256k1", DefaultSecp256k1Path)
	assert.Equal(t, 30*time.Second, DefaultHTTPTimeout)
	assert.Equal(t, 1, DefaultStoreVersion)

	// Verify source constants
	assert.Equal(t, "generated", SourceGenerated)
	assert.Equal(t, "imported", SourceImported)
	assert.Equal(t, "synced", SourceSynced)
}

func TestKeyMetadata_JSONTags(t *testing.T) {
	// Verify KeyMetadata can be created with all fields
	km := KeyMetadata{
		UID:         "test-uid",
		Name:        "test-key",
		PubKeyBytes: []byte{0x02, 0x03, 0x04},
		PubKeyType:  "secp256k1",
		Address:     "cosmos1abc...",
		BaoKeyPath:  "secp256k1/keys/test-key",
		Algorithm:   AlgorithmSecp256k1,
		Exportable:  true,
		CreatedAt:   time.Now(),
		Source:      SourceGenerated,
	}

	require.NotEmpty(t, km.UID)
	require.NotEmpty(t, km.Name)
	require.NotEmpty(t, km.PubKeyBytes)
}

func TestKeyInfo_Fields(t *testing.T) {
	now := time.Now()
	ki := KeyInfo{
		Name:       "test-key",
		PublicKey:  "base64-encoded-pubkey",
		Address:    "cosmos1xyz...",
		Exportable: true,
		CreatedAt:  now,
	}

	assert.Equal(t, "test-key", ki.Name)
	assert.Equal(t, "base64-encoded-pubkey", ki.PublicKey)
	assert.Equal(t, "cosmos1xyz...", ki.Address)
	assert.True(t, ki.Exportable)
	assert.Equal(t, now, ki.CreatedAt)
}

func TestKeyOptions_Defaults(t *testing.T) {
	// Default KeyOptions should have Exportable as false
	opts := KeyOptions{}
	assert.False(t, opts.Exportable)

	// Explicit setting
	opts2 := KeyOptions{Exportable: true}
	assert.True(t, opts2.Exportable)
}

func TestSignRequest_Fields(t *testing.T) {
	req := SignRequest{
		Input:        "base64-encoded-data",
		Prehashed:    true,
		HashAlgo:     "sha256",
		OutputFormat: "raw",
	}

	assert.Equal(t, "base64-encoded-data", req.Input)
	assert.True(t, req.Prehashed)
	assert.Equal(t, "sha256", req.HashAlgo)
	assert.Equal(t, "raw", req.OutputFormat)
}

func TestSignResponse_Fields(t *testing.T) {
	resp := SignResponse{
		Signature:  "base64-encoded-signature",
		PublicKey:  "base64-encoded-pubkey",
		KeyVersion: 1,
	}

	assert.Equal(t, "base64-encoded-signature", resp.Signature)
	assert.Equal(t, "base64-encoded-pubkey", resp.PublicKey)
	assert.Equal(t, 1, resp.KeyVersion)
}

func TestStoreData_Init(t *testing.T) {
	store := StoreData{
		Version: DefaultStoreVersion,
		Keys:    make(map[string]*KeyMetadata),
	}

	assert.Equal(t, 1, store.Version)
	assert.NotNil(t, store.Keys)
	assert.Len(t, store.Keys, 0)

	// Add a key
	store.Keys["test"] = &KeyMetadata{
		UID:  "test-uid",
		Name: "test",
	}
	assert.Len(t, store.Keys, 1)
}

