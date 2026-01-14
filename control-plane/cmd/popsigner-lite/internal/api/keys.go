package api

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
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

// ListKeys handles GET /v1/keys - Returns all keys.
func (h *KeysHandler) ListKeys(c *gin.Context) {
	keys := h.keystore.ListKeys()

	response := make([]KeyResponse, len(keys))
	for i, key := range keys {
		response[i] = KeyResponse{
			ID:        key.ID,
			Name:      key.Name,
			Address:   key.Address,
			PublicKey: hexutil.Encode(key.PublicKey),
			CreatedAt: key.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetKey handles GET /v1/keys/:id - Returns a specific key.
func (h *KeysHandler) GetKey(c *gin.Context) {
	keyID := c.Param("id")
	if keyID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "key ID is required",
		})
		return
	}

	// Try to get by ID first
	key, err := h.keystore.GetKeyByID(keyID)
	if err != nil {
		// Try to get by address
		key, err = h.keystore.GetKey(keyID)
		if err != nil {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: fmt.Sprintf("key with ID %s not found", keyID),
			})
			return
		}
	}

	response := KeyResponse{
		ID:        key.ID,
		Name:      key.Name,
		Address:   key.Address,
		PublicKey: hexutil.Encode(key.PublicKey),
		CreatedAt: key.CreatedAt,
	}

	c.JSON(http.StatusOK, response)
}

// CreateKey handles POST /v1/keys - Creates a new key.
func (h *KeysHandler) CreateKey(c *gin.Context) {
	var req CreateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: fmt.Sprintf("failed to parse request: %v", err),
		})
		return
	}

	// Generate new private key
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: fmt.Sprintf("failed to generate key: %v", err),
		})
		return
	}

	// Derive public key and address
	publicKey := crypto.FromECDSAPub(&privateKey.PublicKey)
	address := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	// Create key object
	key := &keystore.Key{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Address:    address,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		CreatedAt:  time.Now(),
	}

	// Add to keystore
	if err := h.keystore.AddKey(key); err != nil {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "conflict",
			Message: fmt.Sprintf("failed to add key: %v", err),
		})
		return
	}

	response := KeyResponse{
		ID:        key.ID,
		Name:      key.Name,
		Address:   key.Address,
		PublicKey: hexutil.Encode(key.PublicKey),
		CreatedAt: key.CreatedAt,
	}

	c.JSON(http.StatusCreated, response)
}

// DeleteKey handles DELETE /v1/keys/:id - Deletes a key.
func (h *KeysHandler) DeleteKey(c *gin.Context) {
	keyID := c.Param("id")
	if keyID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "key ID is required",
		})
		return
	}

	// Try to get key to find its address
	key, err := h.keystore.GetKeyByID(keyID)
	if err != nil {
		// Try by address
		key, err = h.keystore.GetKey(keyID)
		if err != nil {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: fmt.Sprintf("key with ID %s not found", keyID),
			})
			return
		}
	}

	// Delete by address
	if err := h.keystore.DeleteKey(key.Address); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: fmt.Sprintf("failed to delete key: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("key %s deleted successfully", keyID),
	})
}

// generateRandomBytes generates cryptographically secure random bytes.
func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
