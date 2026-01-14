package api

import "time"

// KeyResponse represents a key in API responses.
type KeyResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	PublicKey string    `json:"public_key"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateKeyRequest represents a request to create a new key.
type CreateKeyRequest struct {
	Name string `json:"name"`
}

// SignRequest represents a request to sign data.
type SignRequest struct {
	Data      string `json:"data"`                // Hex-encoded data to sign
	Prehashed bool   `json:"prehashed,omitempty"` // true if data is already hashed (skip SHA-256)
}

// SignResponse represents a signing response.
type SignResponse struct {
	Signature string `json:"signature"` // Hex-encoded signature
}

// BatchSignRequest represents a batch signing request.
type BatchSignRequest struct {
	Items []BatchSignItem `json:"items"`
}

// BatchSignItem represents a single item in a batch sign request.
type BatchSignItem struct {
	KeyID     string `json:"key_id"`
	Data      string `json:"data"`                // Hex-encoded data to sign
	Prehashed bool   `json:"prehashed,omitempty"` // true if data is already hashed (skip SHA-256)
}

// BatchSignResponse represents a batch signing response.
type BatchSignResponse struct {
	Results []BatchSignResult `json:"results"`
}

// BatchSignResult represents a single result in a batch sign response.
type BatchSignResult struct {
	KeyID     string  `json:"key_id"`
	Signature *string `json:"signature,omitempty"` // Hex-encoded signature (null if error)
	Error     *string `json:"error,omitempty"`     // Error message (null if success)
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// HealthResponse represents a health check response.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}
