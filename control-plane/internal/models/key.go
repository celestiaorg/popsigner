package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Algorithm represents a cryptographic algorithm.
type Algorithm string

const (
	AlgorithmSecp256k1 Algorithm = "secp256k1"
	AlgorithmEd25519   Algorithm = "ed25519"
)

// NetworkType represents the primary network for a key
type NetworkType string

const (
	NetworkTypeCelestia NetworkType = "celestia"
	NetworkTypeEVM      NetworkType = "evm"
	NetworkTypeAll      NetworkType = "all" // Shows both (default)
)

// Valid returns true if the network type is valid
func (n NetworkType) Valid() bool {
	switch n {
	case NetworkTypeCelestia, NetworkTypeEVM, NetworkTypeAll:
		return true
	default:
		return false
	}
}

// String returns the string representation
func (n NetworkType) String() string {
	return string(n)
}

// Key represents a cryptographic key's metadata.
// The actual key material is stored in OpenBao.
type Key struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	OrgID       uuid.UUID       `json:"org_id" db:"org_id"`
	NamespaceID uuid.UUID       `json:"namespace_id" db:"namespace_id"`
	Name        string          `json:"name" db:"name"`
	PublicKey   []byte          `json:"public_key" db:"public_key"`
	Address     string          `json:"address" db:"address"`                    // Cosmos address
	EthAddress  *string         `json:"eth_address,omitempty" db:"eth_address"` // Ethereum address (EIP-55)
	NetworkType NetworkType     `json:"network_type" db:"network_type"`
	Algorithm   Algorithm       `json:"algorithm" db:"algorithm"`
	BaoKeyPath  string          `json:"-" db:"bao_key_path"` // Internal path in OpenBao
	Exportable  bool            `json:"exportable" db:"exportable"`
	Metadata    json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	Version     int             `json:"version" db:"version"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// IsCelestia returns true if key is primarily for Celestia
func (k *Key) IsCelestia() bool {
	return k.NetworkType == NetworkTypeCelestia || k.NetworkType == NetworkTypeAll
}

// IsEVM returns true if key is primarily for EVM
func (k *Key) IsEVM() bool {
	return k.NetworkType == NetworkTypeEVM || k.NetworkType == NetworkTypeAll
}

// GetEthAddress returns the Ethereum address or empty string if not set
func (k *Key) GetEthAddress() string {
	if k.EthAddress != nil {
		return *k.EthAddress
	}
	return ""
}

// KeyResponse is the API response format for keys.
type KeyResponse struct {
	ID          uuid.UUID              `json:"id"`
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	PublicKey   string                 `json:"public_key"`            // Hex encoded
	Address     string                 `json:"address"`               // Cosmos address
	EthAddress  string                 `json:"eth_address,omitempty"` // Ethereum address
	NetworkType NetworkType            `json:"network_type"`
	Algorithm   Algorithm              `json:"algorithm"`
	Exportable  bool                   `json:"exportable"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Version     int                    `json:"version"`
	CreatedAt   time.Time              `json:"created_at"`
}

// SignRequest represents a signing request.
type SignRequest struct {
	Data      string `json:"data" validate:"required"`      // Base64 encoded data to sign
	Encoding  string `json:"encoding" validate:"omitempty"` // base64 or hex, default base64
	Prehashed bool   `json:"prehashed"`                     // If true, data is already hashed
}

// SignResponse represents a signing response.
type SignResponse struct {
	Signature  string `json:"signature"`   // Base64 encoded signature
	PublicKey  string `json:"public_key"`  // Hex encoded public key
	KeyVersion int    `json:"key_version"` // Version of the key used
}

// BatchSignRequest represents a batch signing request.
type BatchSignRequest struct {
	Requests  []BatchSignItem `json:"requests" validate:"required,min=1,max=100"`
	Encoding  string          `json:"encoding" validate:"omitempty"`
	Prehashed bool            `json:"prehashed"`
}

// BatchSignItem represents a single item in a batch sign request.
type BatchSignItem struct {
	KeyID string `json:"key_id" validate:"required"`
	Data  string `json:"data" validate:"required"`
}

// BatchSignResponse represents a batch signing response.
type BatchSignResponse struct {
	Signatures []BatchSignResult `json:"signatures"`
}

// BatchSignResult represents a single result in a batch sign response.
type BatchSignResult struct {
	KeyID      string  `json:"key_id"`
	Signature  string  `json:"signature,omitempty"`
	PublicKey  string  `json:"public_key,omitempty"`
	KeyVersion int     `json:"key_version,omitempty"`
	Error      *string `json:"error,omitempty"`
}

