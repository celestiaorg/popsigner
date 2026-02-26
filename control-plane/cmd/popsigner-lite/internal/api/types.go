package api

import (
	"time"

	"github.com/gin-gonic/gin"
)

// KeyResponse represents a key in API responses.
// Matches the SDK's keyResponse struct.
type KeyResponse struct {
	ID          string            `json:"id"`                    // UUID format
	NamespaceID string            `json:"namespace_id"`          // UUID format
	Name        string            `json:"name"`
	PublicKey   string            `json:"public_key"`            // Hex encoded, no 0x prefix
	Address     string            `json:"address"`
	Algorithm   string            `json:"algorithm"`             // "secp256k1"
	Exportable  bool              `json:"exportable"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Version     int               `json:"version"`
	CreatedAt   time.Time         `json:"created_at"`
}

// CreateKeyRequest represents a request to create a new key.
type CreateKeyRequest struct {
	Name        string            `json:"name"`
	NamespaceID string            `json:"namespace_id,omitempty"` // Defaults to dev namespace
	Algorithm   string            `json:"algorithm,omitempty"`    // Defaults to secp256k1
	Exportable  bool              `json:"exportable,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// BatchCreateKeyRequest represents a request to create multiple keys.
type BatchCreateKeyRequest struct {
	Prefix      string `json:"prefix"`
	Count       int    `json:"count"`
	NamespaceID string `json:"namespace_id,omitempty"`
	Exportable  bool   `json:"exportable,omitempty"`
}

// ImportKeyRequest represents a request to import a private key.
type ImportKeyRequest struct {
	Name        string `json:"name"`
	NamespaceID string `json:"namespace_id,omitempty"`
	PrivateKey  string `json:"private_key"` // Base64-encoded
	Exportable  bool   `json:"exportable,omitempty"`
}

// ExportKeyResponse represents an exported key.
type ExportKeyResponse struct {
	PrivateKey string `json:"private_key"` // Base64-encoded
	Warning    string `json:"warning"`
}

// SignRequest represents a request to sign data.
type SignRequest struct {
	Data      string `json:"data"`                // Base64-encoded data to sign
	Prehashed bool   `json:"prehashed,omitempty"` // true if data is already hashed (skip SHA-256)
}

// SignResponse represents a signing response.
type SignResponse struct {
	Signature  string `json:"signature"`   // Base64-encoded signature
	PublicKey  string `json:"public_key"`  // Hex-encoded, no 0x prefix
	KeyVersion int    `json:"key_version"`
}

// BatchSignRequest represents a batch signing request.
type BatchSignRequest struct {
	Requests []BatchSignItem `json:"requests"`
}

// BatchSignItem represents a single item in a batch sign request.
type BatchSignItem struct {
	KeyID     string `json:"key_id"`
	Data      string `json:"data"`                // Base64-encoded data to sign
	Prehashed bool   `json:"prehashed,omitempty"` // true if data is already hashed (skip SHA-256)
}

// BatchSignResponse represents a batch signing response.
type BatchSignResponse struct {
	Signatures []BatchSignResult `json:"signatures"`
	Count      int               `json:"count"`
}

// BatchSignResult represents a single result in a batch sign response.
type BatchSignResult struct {
	KeyID      string  `json:"key_id"`
	Signature  *string `json:"signature,omitempty"`   // Base64-encoded signature (nil if error)
	PublicKey  *string `json:"public_key,omitempty"`  // Hex-encoded, no 0x prefix (nil if error)
	KeyVersion *int    `json:"key_version,omitempty"` // nil if error
	Error      *string `json:"error,omitempty"`       // Error message (nil if success)
}

// HealthResponse represents a health check response.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// dataResponse wraps a response in the {"data": ...} envelope the SDK expects.
func dataResponse(c *gin.Context, status int, data interface{}) {
	c.JSON(status, gin.H{"data": data})
}

// errorResponse returns an error in the {"error": {"code": ..., "message": ...}} format
// that the SDK's parseError understands.
func errorResponse(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}
