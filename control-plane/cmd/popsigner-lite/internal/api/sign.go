package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/keystore"
	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/signer"
)

// SignHandler handles signing operations.
type SignHandler struct {
	keystore *keystore.Keystore
	signer   *signer.EthereumSigner
}

// NewSignHandler creates a new sign handler.
func NewSignHandler(ks *keystore.Keystore) *SignHandler {
	return &SignHandler{
		keystore: ks,
		signer:   signer.NewEthereumSigner(),
	}
}

// Sign handles POST /v1/keys/:id/sign - Signs data with a specific key.
func (h *SignHandler) Sign(c *gin.Context) {
	keyID := c.Param("id")
	if keyID == "" {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "key ID is required")
		return
	}

	var req SignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid_request", fmt.Sprintf("failed to parse request: %v", err))
		return
	}

	if req.Data == "" {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "data is required")
		return
	}

	// Get the key
	key, err := h.keystore.GetKeyByID(keyID)
	if err != nil {
		// Try by address
		key, err = h.keystore.GetKey(keyID)
		if err != nil {
			errorResponse(c, http.StatusNotFound, "not_found", fmt.Sprintf("key with ID %s not found", keyID))
			return
		}
	}

	// Decode the data (base64)
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid_request", fmt.Sprintf("invalid data base64: %v", err))
		return
	}

	// Hash the data based on prehashed parameter
	var hash []byte
	if req.Prehashed {
		if len(data) != 32 {
			errorResponse(c, http.StatusBadRequest, "invalid_request", fmt.Sprintf("prehashed data must be 32 bytes, got %d", len(data)))
			return
		}
		hash = data
	} else {
		hashSum := sha256.Sum256(data)
		hash = hashSum[:]
	}

	// Sign the hash
	signature, err := h.signer.SignHash(hash, key.PrivateKey)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "signing_failed", fmt.Sprintf("failed to sign data: %v", err))
		return
	}

	dataResponse(c, http.StatusOK, SignResponse{
		Signature:  base64.StdEncoding.EncodeToString(signature),
		PublicKey:  hex.EncodeToString(key.PublicKey),
		KeyVersion: key.Version,
	})
}

// BatchSign handles POST /v1/sign/batch - Signs multiple messages.
func (h *SignHandler) BatchSign(c *gin.Context) {
	var req BatchSignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid_request", fmt.Sprintf("failed to parse request: %v", err))
		return
	}

	if len(req.Requests) == 0 {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "at least one request is required")
		return
	}

	// Process each item
	results := make([]BatchSignResult, len(req.Requests))
	for i, item := range req.Requests {
		results[i] = h.signSingleItem(item)
	}

	dataResponse(c, http.StatusOK, BatchSignResponse{
		Signatures: results,
		Count:      len(results),
	})
}

// signSingleItem signs a single item in a batch request.
func (h *SignHandler) signSingleItem(item BatchSignItem) BatchSignResult {
	result := BatchSignResult{
		KeyID: item.KeyID,
	}

	// Get the key
	key, err := h.keystore.GetKeyByID(item.KeyID)
	if err != nil {
		// Try by address
		key, err = h.keystore.GetKey(item.KeyID)
		if err != nil {
			errMsg := fmt.Sprintf("key not found: %v", err)
			result.Error = &errMsg
			return result
		}
	}

	// Decode the data (base64)
	data, err := base64.StdEncoding.DecodeString(item.Data)
	if err != nil {
		errMsg := fmt.Sprintf("invalid data base64: %v", err)
		result.Error = &errMsg
		return result
	}

	// Hash the data based on prehashed parameter
	var hash []byte
	if item.Prehashed {
		if len(data) != 32 {
			errMsg := fmt.Sprintf("prehashed data must be 32 bytes, got %d", len(data))
			result.Error = &errMsg
			return result
		}
		hash = data
	} else {
		hashSum := sha256.Sum256(data)
		hash = hashSum[:]
	}

	// Sign the hash
	signature, err := h.signer.SignHash(hash, key.PrivateKey)
	if err != nil {
		errMsg := fmt.Sprintf("signing failed: %v", err)
		result.Error = &errMsg
		return result
	}

	// Success
	sig := base64.StdEncoding.EncodeToString(signature)
	result.Signature = &sig
	result.PublicKey = hex.EncodeToString(key.PublicKey)
	result.KeyVersion = key.Version
	return result
}
