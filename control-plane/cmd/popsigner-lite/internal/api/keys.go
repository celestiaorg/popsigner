package api

import (
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/keystore"
)

// KeysHandler handles key management operations.
type KeysHandler struct {
	keystore *keystore.Keystore
}

// NewKeysHandler creates a new keys handler.
func NewKeysHandler(ks *keystore.Keystore) *KeysHandler {
	return &KeysHandler{
		keystore: ks,
	}
}

// keyToResponse converts a keystore.Key to a KeyResponse with proper encoding.
func keyToResponse(key *keystore.Key) KeyResponse {
	return KeyResponse{
		ID:          key.ID,
		NamespaceID: key.NamespaceID,
		Name:        key.Name,
		PublicKey:   hex.EncodeToString(key.PublicKey),
		Address:     key.Address,
		Algorithm:   key.Algorithm,
		Exportable:  key.Exportable,
		Metadata:    key.Metadata,
		Version:     key.Version,
		CreatedAt:   key.CreatedAt,
	}
}

// ListKeys handles GET /v1/keys - Returns all keys.
func (h *KeysHandler) ListKeys(c *gin.Context) {
	keys := h.keystore.ListKeys()

	response := make([]KeyResponse, len(keys))
	for i, key := range keys {
		response[i] = keyToResponse(key)
	}

	dataResponse(c, http.StatusOK, response)
}

// GetKey handles GET /v1/keys/:id - Returns a specific key.
func (h *KeysHandler) GetKey(c *gin.Context) {
	keyID := c.Param("id")
	if keyID == "" {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "key ID is required")
		return
	}

	// Try to get by ID first
	key, err := h.keystore.GetKeyByID(keyID)
	if err != nil {
		// Try to get by address
		key, err = h.keystore.GetKeyInsensitive(keyID)
		if err != nil {
			errorResponse(c, http.StatusNotFound, "not_found", fmt.Sprintf("key with ID %s not found", keyID))
			return
		}
	}

	dataResponse(c, http.StatusOK, keyToResponse(key))
}

// CreateKey handles POST /v1/keys - Creates a new key.
func (h *KeysHandler) CreateKey(c *gin.Context) {
	var req CreateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid_request", fmt.Sprintf("failed to parse request: %v", err))
		return
	}

	if req.Name == "" {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}

	// Default namespace and algorithm
	nsID := req.NamespaceID
	if nsID == "" {
		nsID = keystore.DevNamespaceID
	} else if _, err := uuid.Parse(nsID); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "namespace_id must be a valid UUID")
		return
	}

	algo := req.Algorithm
	if algo == "" {
		algo = "secp256k1"
	} else if algo != "secp256k1" {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "only secp256k1 algorithm is supported")
		return
	}

	// Generate new private key
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "internal_error", fmt.Sprintf("failed to generate key: %v", err))
		return
	}

	// Derive compressed public key and address
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	address := crypto.PubkeyToAddress(*publicKey).Hex()

	// Create key object
	key := &keystore.Key{
		ID:          uuid.New().String(),
		NamespaceID: nsID,
		Name:        req.Name,
		Address:     address,
		PrivateKey:  privateKey,
		PublicKey:   crypto.CompressPubkey(publicKey),
		Algorithm:   algo,
		Exportable:  req.Exportable,
		Metadata:    req.Metadata,
		Version:     1,
		CreatedAt:   time.Now(),
	}

	// Add to keystore
	if err := h.keystore.AddKey(key); err != nil {
		errorResponse(c, http.StatusConflict, "conflict", fmt.Sprintf("failed to add key: %v", err))
		return
	}

	dataResponse(c, http.StatusCreated, keyToResponse(key))
}

// CreateBatchKeys handles POST /v1/keys/batch - Creates multiple keys at once.
func (h *KeysHandler) CreateBatchKeys(c *gin.Context) {
	var req BatchCreateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid_request", fmt.Sprintf("failed to parse request: %v", err))
		return
	}

	if req.Prefix == "" {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "prefix is required")
		return
	}
	if req.Count < 1 || req.Count > 100 {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "count must be between 1 and 100")
		return
	}

	nsID := req.NamespaceID
	if nsID == "" {
		nsID = keystore.DevNamespaceID
	} else if _, err := uuid.Parse(nsID); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "namespace_id must be a valid UUID")
		return
	}

	// Generate all keys first so that a generation failure doesn't result in a
	// partial batch being added to the keystore.
	batch := make([]*keystore.Key, req.Count)
	for i := 0; i < req.Count; i++ {
		privateKey, err := crypto.GenerateKey()
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "internal_error", fmt.Sprintf("failed to generate key %d: %v", i+1, err))
			return
		}
		publicKey := privateKey.Public().(*ecdsa.PublicKey)
		batch[i] = &keystore.Key{
			ID:          uuid.New().String(),
			NamespaceID: nsID,
			Name:        fmt.Sprintf("%s-%d", req.Prefix, i+1),
			Address:     crypto.PubkeyToAddress(*publicKey).Hex(),
			PrivateKey:  privateKey,
			PublicKey:   crypto.CompressPubkey(publicKey),
			Algorithm:   "secp256k1",
			Exportable:  req.Exportable,
			Version:     1,
			CreatedAt:   time.Now(),
		}
	}

	// Ensure no generated key conflicts with an existing address before adding.
	for _, key := range batch {
		if _, err := h.keystore.GetKeyInsensitive(key.Address); err == nil {
			errorResponse(c, http.StatusConflict, "conflict", fmt.Sprintf("failed to add key %s: key with address %s already exists", key.Name, key.Address))
			return
		}
	}

	// All keys generated — now add them to the keystore.
	// If any add fails unexpectedly, roll back previously added keys from this batch.
	keys := make([]KeyResponse, 0, req.Count)
	addedAddresses := make([]string, 0, req.Count)
	for _, key := range batch {
		if err := h.keystore.AddKey(key); err != nil {
			for _, addr := range addedAddresses {
				_ = h.keystore.DeleteKey(addr)
			}
			errorResponse(c, http.StatusConflict, "conflict", fmt.Sprintf("failed to add key %s: %v", key.Name, err))
			return
		}
		addedAddresses = append(addedAddresses, key.Address)
		keys = append(keys, keyToResponse(key))
	}

	dataResponse(c, http.StatusCreated, gin.H{
		"keys":  keys,
		"count": len(keys),
	})
}

// ImportKey handles POST /v1/keys/import - Imports a private key.
func (h *KeysHandler) ImportKey(c *gin.Context) {
	var req ImportKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid_request", fmt.Sprintf("failed to parse request: %v", err))
		return
	}

	if req.Name == "" {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}

	// Decode base64 private key
	privKeyBytes, err := base64.StdEncoding.DecodeString(req.PrivateKey)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid_request", fmt.Sprintf("invalid private key encoding: %v", err))
		return
	}

	privateKey, err := crypto.ToECDSA(privKeyBytes)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid_request", fmt.Sprintf("invalid private key: %v", err))
		return
	}

	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	address := crypto.PubkeyToAddress(*publicKey).Hex()

	nsID := req.NamespaceID
	if nsID == "" {
		nsID = keystore.DevNamespaceID
	} else if _, err := uuid.Parse(nsID); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "namespace_id must be a valid UUID")
		return
	}

	key := &keystore.Key{
		ID:          uuid.New().String(),
		NamespaceID: nsID,
		Name:        req.Name,
		Address:     address,
		PrivateKey:  privateKey,
		PublicKey:   crypto.CompressPubkey(publicKey),
		Algorithm:   "secp256k1",
		Exportable:  req.Exportable,
		Version:     1,
		CreatedAt:   time.Now(),
	}

	if err := h.keystore.AddKey(key); err != nil {
		errorResponse(c, http.StatusConflict, "conflict", fmt.Sprintf("failed to add key: %v", err))
		return
	}

	dataResponse(c, http.StatusCreated, keyToResponse(key))
}

// ExportKey handles POST /v1/keys/:id/export - Exports a key's private key.
func (h *KeysHandler) ExportKey(c *gin.Context) {
	keyID := c.Param("id")
	if keyID == "" {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "key ID is required")
		return
	}

	key, err := h.keystore.GetKeyByID(keyID)
	if err != nil {
		key, err = h.keystore.GetKeyInsensitive(keyID)
		if err != nil {
			errorResponse(c, http.StatusNotFound, "not_found", fmt.Sprintf("key with ID %s not found", keyID))
			return
		}
	}

	if !key.Exportable {
		errorResponse(c, http.StatusForbidden, "forbidden", "key is not exportable")
		return
	}

	privKeyBytes := crypto.FromECDSA(key.PrivateKey)

	dataResponse(c, http.StatusOK, ExportKeyResponse{
		PrivateKey: base64.StdEncoding.EncodeToString(privKeyBytes),
		Warning:    "Handle private key material with extreme care. Never share or log this value.",
	})
}

// DeleteKey handles DELETE /v1/keys/:id - Deletes a key.
func (h *KeysHandler) DeleteKey(c *gin.Context) {
	keyID := c.Param("id")
	if keyID == "" {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "key ID is required")
		return
	}

	// Try to get key to find its address
	key, err := h.keystore.GetKeyByID(keyID)
	if err != nil {
		// Try by address
		key, err = h.keystore.GetKeyInsensitive(keyID)
		if err != nil {
			errorResponse(c, http.StatusNotFound, "not_found", fmt.Sprintf("key with ID %s not found", keyID))
			return
		}
	}

	// Delete by address
	if err := h.keystore.DeleteKey(key.Address); err != nil {
		errorResponse(c, http.StatusInternalServerError, "internal_error", fmt.Sprintf("failed to delete key: %v", err))
		return
	}

	dataResponse(c, http.StatusOK, gin.H{
		"message": fmt.Sprintf("key %s deleted successfully", keyID),
	})
}
