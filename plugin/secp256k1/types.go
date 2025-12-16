package secp256k1

import "time"

// keyEntry represents a stored secp256k1 key in OpenBao.
// Private keys are automatically encrypted at rest using OpenBao's seal wrap.
type keyEntry struct {
	// PrivateKey is the raw 32-byte secp256k1 private key.
	PrivateKey []byte `json:"private_key"`

	// PublicKey is the compressed 33-byte secp256k1 public key.
	PublicKey []byte `json:"public_key"`

	// PublicKeyUncompressed is the uncompressed 65-byte public key (for EVM).
	// Stored to avoid recomputation during Ethereum address derivation.
	PublicKeyUncompressed []byte `json:"public_key_uncompressed,omitempty"`

	// Exportable indicates whether the private key can be exported.
	// Once set to false, this cannot be changed.
	Exportable bool `json:"exportable"`

	// CreatedAt is the timestamp when the key was created.
	CreatedAt time.Time `json:"created_at"`

	// Imported indicates whether the key was imported (vs generated).
	Imported bool `json:"imported"`
}
