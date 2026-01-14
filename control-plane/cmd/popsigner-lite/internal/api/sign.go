package api

import (
	"crypto/sha256"
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/common/hexutil"
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
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "key ID is required",
		})
		return
	}

	var req SignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: fmt.Sprintf("failed to parse request: %v", err),
		})
		return
	}

	if req.Data == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "data is required",
		})
		return
	}

	// Get the key
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

	// Decode the data
	data, err := hexutil.Decode(req.Data)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: fmt.Sprintf("invalid data hex: %v", err),
		})
		return
	}

	// Hash the data based on prehashed parameter
	// - If prehashed=true: data is already a 32-byte hash, sign it directly
	// - If prehashed=false: apply SHA-256 hash (Celestia/Cosmos SDK style)
	// This matches the behavior of the original BaoKeyring.Sign() method
	var hash []byte
	if req.Prehashed {
		// Data is already hashed, use it directly
		if len(data) != 32 {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_request",
				Message: fmt.Sprintf("prehashed data must be 32 bytes, got %d", len(data)),
			})
			return
		}
		hash = data
	} else {
		// Apply SHA-256 hash (Celestia/Cosmos SDK style)
		// This matches the behavior of the original BaoKeyring.Sign() method
		h := sha256.Sum256(data)
		hash = h[:]
	}

	// Sign the hash
	signature, err := h.signer.SignHash(hash, key.PrivateKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "signing_failed",
			Message: fmt.Sprintf("failed to sign data: %v", err),
		})
		return
	}

	response := SignResponse{
		Signature: hexutil.Encode(signature),
	}

	c.JSON(http.StatusOK, response)
}

// BatchSign handles POST /v1/sign/batch - Signs multiple messages.
func (h *SignHandler) BatchSign(c *gin.Context) {
	var req BatchSignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: fmt.Sprintf("failed to parse request: %v", err),
		})
		return
	}

	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "at least one item is required",
		})
		return
	}

	// Process each item
	results := make([]BatchSignResult, len(req.Items))
	for i, item := range req.Items {
		result := h.signSingleItem(item)
		results[i] = result
	}

	response := BatchSignResponse{
		Results: results,
	}

	c.JSON(http.StatusOK, response)
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

	// Decode the data
	data, err := hexutil.Decode(item.Data)
	if err != nil {
		errMsg := fmt.Sprintf("invalid data hex: %v", err)
		result.Error = &errMsg
		return result
	}

	// Hash the data based on prehashed parameter
	var hash []byte
	if item.Prehashed {
		// Data is already hashed, use it directly
		if len(data) != 32 {
			errMsg := fmt.Sprintf("prehashed data must be 32 bytes, got %d", len(data))
			result.Error = &errMsg
			return result
		}
		hash = data
	} else {
		// Apply SHA-256 hash (Celestia/Cosmos SDK style)
		h := sha256.Sum256(data)
		hash = h[:]
	}

	// Sign the hash
	signature, err := h.signer.SignHash(hash, key.PrivateKey)
	if err != nil {
		errMsg := fmt.Sprintf("signing failed: %v", err)
		result.Error = &errMsg
		return result
	}

	// Success
	sig := hexutil.Encode(signature)
	result.Signature = &sig
	return result
}
