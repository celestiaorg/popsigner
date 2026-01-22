// Package bundle provides artifact bundling for rollup deployments.
// It creates downloadable .tar.gz packages containing all necessary
// configuration files for both OP Stack and Nitro rollup deployments.
package bundle

import (
	"time"
)

// Stack represents the rollup stack type.
type Stack string

const (
	// StackOPStack represents the OP Stack rollup framework.
	StackOPStack Stack = "opstack"
	// StackNitro represents the Arbitrum Nitro rollup framework.
	StackNitro Stack = "nitro"
	// StackPopBundle represents the POPKins Devnet Bundle (OP Stack + Celestia DA).
	StackPopBundle Stack = "pop-bundle"
)

// BundleManifest describes the bundle contents and metadata.
type BundleManifest struct {
	// Version is the bundle format version.
	Version string `json:"version"`
	// Stack indicates whether this is an opstack or nitro bundle.
	Stack Stack `json:"stack"`
	// ChainID is the L2/Orbit chain ID.
	ChainID uint64 `json:"chain_id"`
	// ChainName is the human-readable chain name.
	ChainName string `json:"chain_name"`
	// GeneratedAt is when this bundle was created.
	GeneratedAt time.Time `json:"generated_at"`
	// Files lists all files in the bundle with descriptions.
	Files []FileEntry `json:"files"`
	// POPSignerInfo contains POPSigner connection details.
	POPSignerInfo POPSignerInfo `json:"popsigner"`
	// Checksums contains SHA256 checksums of key files.
	Checksums map[string]string `json:"checksums,omitempty"`
}

// FileEntry describes a file in the bundle.
type FileEntry struct {
	// Path is the relative path within the bundle.
	Path string `json:"path"`
	// Description explains what this file is for.
	Description string `json:"description"`
	// Required indicates if this file is necessary for operation.
	Required bool `json:"required"`
	// SizeBytes is the file size (for display purposes).
	SizeBytes int64 `json:"size_bytes,omitempty"`
	// Checksum is the SHA256 hash of the file contents.
	Checksum string `json:"checksum,omitempty"`
}

// POPSignerInfo contains POPSigner connection details.
type POPSignerInfo struct {
	// Common fields
	// Endpoint is the POPSigner RPC endpoint (for OP Stack API key auth).
	Endpoint string `json:"endpoint,omitempty"`

	// OP Stack specific (API Key auth)
	// APIKeyConfigured indicates an API key placeholder is in .env.
	APIKeyConfigured bool `json:"api_key_configured,omitempty"`
	// BatcherAddr is the Ethereum address for the batcher role.
	BatcherAddr string `json:"batcher_address,omitempty"`
	// ProposerAddr is the Ethereum address for the proposer role.
	ProposerAddr string `json:"proposer_address,omitempty"`

	// Nitro specific (mTLS auth)
	// MTLSEndpoint is the POPSigner mTLS endpoint.
	MTLSEndpoint string `json:"mtls_endpoint,omitempty"`
	// CertificateIncluded indicates if client certs are in the bundle.
	CertificateIncluded bool `json:"certificate_included,omitempty"`
	// BatchPosterAddr is the Ethereum address for batch posting.
	BatchPosterAddr string `json:"batch_poster_address,omitempty"`
	// ValidatorAddr is the Ethereum address for the validator/staker.
	ValidatorAddr string `json:"validator_address,omitempty"`
}

// BundleConfig contains all information needed to create a bundle.
type BundleConfig struct {
	// Stack type (opstack or nitro)
	Stack Stack
	// ChainID is the L2/Orbit chain ID
	ChainID uint64
	// ChainName is the human-readable chain name
	ChainName string

	// DAType specifies the data availability layer ("celestia", "anytrust", or "")
	DAType string

	// Artifacts from deployment (artifact_type -> content)
	Artifacts map[string][]byte

	// POPSigner connection info
	POPSignerEndpoint     string // OP Stack (API key auth)
	POPSignerMTLSEndpoint string // Nitro (mTLS auth)

	// Role addresses
	BatcherAddress   string // OP Stack batcher
	ProposerAddress  string // OP Stack proposer
	ValidatorAddress string // Nitro validator/staker

	// Contract addresses (for reference in manifest)
	Contracts map[string]string

	// Credentials (Nitro mTLS)
	ClientCert []byte // PEM-encoded client certificate
	ClientKey  []byte // PEM-encoded client private key
	CACert     []byte // PEM-encoded CA certificate (optional)

	// Generated files (from compose generator)
	DockerCompose string
	EnvExample    string
	EnvFile       string // Ready-to-use .env file with actual values
}

// BundleResult contains the generated bundle and metadata.
type BundleResult struct {
	// Data is the raw .tar.gz bundle content.
	Data []byte
	// Filename is the suggested download filename.
	Filename string
	// Manifest is the bundle manifest (also included in the bundle).
	Manifest *BundleManifest
	// SizeBytes is the total bundle size.
	SizeBytes int64
	// Checksum is the SHA256 hash of the entire bundle.
	Checksum string
}

