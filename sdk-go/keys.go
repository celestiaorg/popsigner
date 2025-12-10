package banhbaoring

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// KeysService handles key management operations.
type KeysService struct {
	client *Client
}

// CreateKeyRequest is the request for creating a key.
type CreateKeyRequest struct {
	// Name is the key name (required).
	Name string `json:"name"`
	// NamespaceID is the namespace to create the key in (required).
	NamespaceID uuid.UUID `json:"namespace_id"`
	// Algorithm is the key algorithm (default: secp256k1).
	Algorithm string `json:"algorithm,omitempty"`
	// Exportable indicates if the private key can be exported.
	Exportable bool `json:"exportable,omitempty"`
	// Metadata is optional key-value metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CreateBatchRequest creates multiple keys at once.
type CreateBatchRequest struct {
	// Prefix is the key name prefix. Keys will be named "prefix-1", "prefix-2", etc.
	Prefix string `json:"prefix"`
	// Count is the number of keys to create (1-100).
	Count int `json:"count"`
	// NamespaceID is the namespace to create keys in (required).
	NamespaceID uuid.UUID `json:"namespace_id"`
	// Exportable indicates if private keys can be exported.
	Exportable bool `json:"exportable,omitempty"`
}

// UpdateKeyRequest is the request for updating a key.
type UpdateKeyRequest struct {
	// Name is the new key name.
	Name string `json:"name,omitempty"`
	// Metadata is the new key metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ImportKeyRequest is the request for importing a key.
type ImportKeyRequest struct {
	// Name is the key name (required).
	Name string `json:"name"`
	// NamespaceID is the namespace to import the key into (required).
	NamespaceID uuid.UUID `json:"namespace_id"`
	// PrivateKey is the base64-encoded private key (required).
	PrivateKey string `json:"private_key"`
	// Exportable indicates if the private key can be exported.
	Exportable bool `json:"exportable,omitempty"`
}

// ExportKeyResponse is the response from exporting a key.
type ExportKeyResponse struct {
	// PrivateKey is the base64-encoded private key.
	PrivateKey string `json:"private_key"`
	// Warning is a security warning message.
	Warning string `json:"warning"`
}

// keyResponse is the internal API response format for keys.
type keyResponse struct {
	ID          uuid.UUID              `json:"id"`
	NamespaceID uuid.UUID              `json:"namespace_id"`
	Name        string                 `json:"name"`
	PublicKey   string                 `json:"public_key"`
	Address     string                 `json:"address"`
	Algorithm   Algorithm              `json:"algorithm"`
	Exportable  bool                   `json:"exportable"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Version     int                    `json:"version"`
	CreatedAt   string                 `json:"created_at"`
}

// toKey converts a keyResponse to a Key.
func (r *keyResponse) toKey() *Key {
	createdAt, _ := time.Parse("2006-01-02T15:04:05Z", r.CreatedAt)

	metadata := make(map[string]string)
	for k, v := range r.Metadata {
		if s, ok := v.(string); ok {
			metadata[k] = s
		}
	}

	return &Key{
		ID:          r.ID,
		NamespaceID: r.NamespaceID,
		Name:        r.Name,
		PublicKey:   r.PublicKey,
		Address:     r.Address,
		Algorithm:   r.Algorithm,
		Exportable:  r.Exportable,
		Metadata:    metadata,
		Version:     r.Version,
		CreatedAt:   createdAt,
	}
}

// Create creates a new key.
//
// Example:
//
//	key, err := client.Keys.Create(ctx, banhbaoring.CreateKeyRequest{
//	    Name:        "sequencer-main",
//	    NamespaceID: namespaceID,
//	    Algorithm:   "secp256k1",
//	    Exportable:  true,
//	})
func (s *KeysService) Create(ctx context.Context, req CreateKeyRequest) (*Key, error) {
	// Convert to API format
	apiReq := map[string]interface{}{
		"name":         req.Name,
		"namespace_id": req.NamespaceID.String(),
		"exportable":   req.Exportable,
	}
	if req.Algorithm != "" {
		apiReq["algorithm"] = req.Algorithm
	}
	if len(req.Metadata) > 0 {
		apiReq["metadata"] = req.Metadata
	}

	var resp keyResponse
	if err := s.client.post(ctx, "/v1/keys", apiReq, &resp); err != nil {
		return nil, err
	}
	return resp.toKey(), nil
}

// CreateBatch creates multiple keys in parallel.
// This is optimized for Celestia's parallel worker pattern.
//
// Example:
//
//	keys, err := client.Keys.CreateBatch(ctx, banhbaoring.CreateBatchRequest{
//	    Prefix:      "blob-worker",
//	    Count:       4,
//	    NamespaceID: prodNamespace,
//	})
//	// Creates: blob-worker-1, blob-worker-2, blob-worker-3, blob-worker-4
func (s *KeysService) CreateBatch(ctx context.Context, req CreateBatchRequest) ([]*Key, error) {
	apiReq := map[string]interface{}{
		"prefix":       req.Prefix,
		"count":        req.Count,
		"namespace_id": req.NamespaceID.String(),
		"exportable":   req.Exportable,
	}

	var resp struct {
		Keys  []*keyResponse `json:"keys"`
		Count int            `json:"count"`
	}
	if err := s.client.post(ctx, "/v1/keys/batch", apiReq, &resp); err != nil {
		return nil, err
	}

	keys := make([]*Key, len(resp.Keys))
	for i, k := range resp.Keys {
		keys[i] = k.toKey()
	}
	return keys, nil
}

// Get retrieves a key by ID.
//
// Example:
//
//	key, err := client.Keys.Get(ctx, keyID)
func (s *KeysService) Get(ctx context.Context, keyID uuid.UUID) (*Key, error) {
	var resp keyResponse
	if err := s.client.get(ctx, fmt.Sprintf("/v1/keys/%s", keyID), &resp); err != nil {
		return nil, err
	}
	return resp.toKey(), nil
}

// ListOptions are options for listing keys.
type ListOptions struct {
	// NamespaceID filters keys by namespace.
	NamespaceID *uuid.UUID
}

// List returns all keys, optionally filtered by namespace.
//
// Example:
//
//	// List all keys
//	keys, err := client.Keys.List(ctx, nil)
//
//	// List keys in a specific namespace
//	keys, err := client.Keys.List(ctx, &banhbaoring.ListOptions{
//	    NamespaceID: &namespaceID,
//	})
func (s *KeysService) List(ctx context.Context, opts *ListOptions) ([]*Key, error) {
	path := "/v1/keys"
	if opts != nil && opts.NamespaceID != nil {
		path = fmt.Sprintf("/v1/keys?namespace_id=%s", opts.NamespaceID)
	}

	var resp []*keyResponse
	if err := s.client.get(ctx, path, &resp); err != nil {
		return nil, err
	}

	keys := make([]*Key, len(resp))
	for i, k := range resp {
		keys[i] = k.toKey()
	}
	return keys, nil
}

// Delete deletes a key.
//
// Example:
//
//	err := client.Keys.Delete(ctx, keyID)
func (s *KeysService) Delete(ctx context.Context, keyID uuid.UUID) error {
	return s.client.delete(ctx, fmt.Sprintf("/v1/keys/%s", keyID))
}

// Import imports a private key.
//
// Example:
//
//	key, err := client.Keys.Import(ctx, banhbaoring.ImportKeyRequest{
//	    Name:        "imported-key",
//	    NamespaceID: namespaceID,
//	    PrivateKey:  base64PrivateKey,
//	    Exportable:  true,
//	})
func (s *KeysService) Import(ctx context.Context, req ImportKeyRequest) (*Key, error) {
	apiReq := map[string]interface{}{
		"name":         req.Name,
		"namespace_id": req.NamespaceID.String(),
		"private_key":  req.PrivateKey,
		"exportable":   req.Exportable,
	}

	var resp keyResponse
	if err := s.client.post(ctx, "/v1/keys/import", apiReq, &resp); err != nil {
		return nil, err
	}
	return resp.toKey(), nil
}

// Export exports a key's private key material.
// The key must have been created with Exportable: true.
//
// Example:
//
//	result, err := client.Keys.Export(ctx, keyID)
//	privateKey := result.PrivateKey // base64-encoded
func (s *KeysService) Export(ctx context.Context, keyID uuid.UUID) (*ExportKeyResponse, error) {
	var resp ExportKeyResponse
	if err := s.client.post(ctx, fmt.Sprintf("/v1/keys/%s/export", keyID), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

