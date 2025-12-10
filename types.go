// Package banhbaoring provides a Cosmos SDK keyring implementation
// backed by OpenBao for secure secp256k1 signing.
package banhbaoring

import (
	"crypto/tls"
	"time"
)

// Algorithm constants
const (
	AlgorithmSecp256k1   = "secp256k1"
	DefaultSecp256k1Path = "secp256k1"
	DefaultHTTPTimeout   = 30 * time.Second
	DefaultStoreVersion  = 1
)

// Source constants
const (
	SourceGenerated = "generated"
	SourceImported  = "imported"
	SourceSynced    = "synced"
)

// Config holds configuration for BaoKeyring initialization.
type Config struct {
	BaoAddr       string        // OpenBao server address
	BaoToken      string        // OpenBao authentication token
	BaoNamespace  string        // Optional: OpenBao namespace
	Secp256k1Path string        // Plugin mount path (default: "secp256k1")
	StorePath     string        // Path to local metadata store
	HTTPTimeout   time.Duration // HTTP request timeout
	TLSConfig     *tls.Config   // Optional: custom TLS config
	SkipTLSVerify bool          // INSECURE: skip TLS verification
}

// WithDefaults returns Config with default values applied.
func (c Config) WithDefaults() Config {
	if c.Secp256k1Path == "" {
		c.Secp256k1Path = DefaultSecp256k1Path
	}
	if c.HTTPTimeout == 0 {
		c.HTTPTimeout = DefaultHTTPTimeout
	}
	return c
}

// Validate checks required configuration fields.
func (c *Config) Validate() error {
	if c.BaoAddr == "" {
		return ErrMissingBaoAddr
	}
	if c.BaoToken == "" {
		return ErrMissingBaoToken
	}
	if c.StorePath == "" {
		return ErrMissingStorePath
	}
	return nil
}

// KeyMetadata contains locally stored key information.
type KeyMetadata struct {
	UID         string    `json:"uid"`
	Name        string    `json:"name"`
	PubKeyBytes []byte    `json:"pub_key"`
	PubKeyType  string    `json:"pub_key_type"`
	Address     string    `json:"address"`
	BaoKeyPath  string    `json:"bao_key_path"`
	Algorithm   string    `json:"algorithm"`
	Exportable  bool      `json:"exportable"`
	CreatedAt   time.Time `json:"created_at"`
	Source      string    `json:"source"`
}

// KeyInfo represents public key information from OpenBao.
type KeyInfo struct {
	Name       string    `json:"name"`
	PublicKey  string    `json:"public_key"`
	Address    string    `json:"address"`
	Exportable bool      `json:"exportable"`
	CreatedAt  time.Time `json:"created_at"`
}

// KeyOptions configures key creation.
type KeyOptions struct {
	Exportable bool
}

// SignRequest for OpenBao signing.
type SignRequest struct {
	Input        string `json:"input"`
	Prehashed    bool   `json:"prehashed"`
	HashAlgo     string `json:"hash_algorithm,omitempty"`
	OutputFormat string `json:"output_format,omitempty"`
}

// SignResponse from OpenBao signing.
type SignResponse struct {
	Signature  string `json:"signature"`
	PublicKey  string `json:"public_key"`
	KeyVersion int    `json:"key_version"`
}

// StoreData is the persisted store format.
type StoreData struct {
	Version int                     `json:"version"`
	Keys    map[string]*KeyMetadata `json:"keys"`
}

// CreateBatchOptions configures batch key creation.
// This is optimized for the Celestia parallel worker pattern where
// multiple worker accounts sign blobs concurrently.
type CreateBatchOptions struct {
	Prefix     string // Key name prefix (e.g., "blob-worker")
	Count      int    // Number of keys to create (e.g., 4)
	Namespace  string // Optional namespace
	Exportable bool   // Whether keys are exportable
}

// CreateBatchResult contains results of batch key creation.
type CreateBatchResult struct {
	Keys   []*KeyRecord // Created key records
	Errors []error      // Per-key errors (nil if successful)
}

// KeyRecord holds minimal key information for batch results.
// This avoids importing keyring package in types.go.
type KeyRecord struct {
	Name      string // Key name/UID
	PubKey    []byte // 33-byte compressed secp256k1 public key
	Address   string // Cosmos address
	Algorithm string // Algorithm (always "secp256k1")
}

// BatchSignRequest represents a single signing request in a batch.
// Each request can use a different key - perfect for parallel workers.
type BatchSignRequest struct {
	UID  string // Key UID
	Msg  []byte // Message to sign
}

// BatchSignResult represents a single signing result.
type BatchSignResult struct {
	UID       string // Key UID
	Signature []byte // 64-byte Cosmos signature (R||S format)
	PubKey    []byte // 33-byte compressed secp256k1 public key
	Error     error  // nil if successful
}

