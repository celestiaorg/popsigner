package banhbaoring

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
)

// SignService handles signing operations.
type SignService struct {
	client *Client
}

// SignRequest is the request for signing data.
type SignRequest struct {
	// KeyID is the ID of the key to sign with.
	KeyID uuid.UUID `json:"key_id"`
	// Data is the data to sign (will be base64-encoded for transmission).
	Data []byte `json:"-"`
	// Prehashed indicates if the data is already hashed.
	Prehashed bool `json:"prehashed,omitempty"`
}

// SignResponse is the response from a sign operation.
type SignResponse struct {
	// KeyID is the ID of the key that signed.
	KeyID uuid.UUID `json:"key_id"`
	// Signature is the raw signature bytes.
	Signature []byte `json:"-"`
	// PublicKey is the hex-encoded public key.
	PublicKey string `json:"public_key"`
	// KeyVersion is the version of the key that signed.
	KeyVersion int `json:"key_version"`
}

// BatchSignRequest signs multiple messages in parallel.
type BatchSignRequest struct {
	// Requests is the list of sign requests.
	Requests []SignRequest `json:"requests"`
}

// BatchSignResult is a single result from a batch sign operation.
type BatchSignResult struct {
	// KeyID is the ID of the key that signed.
	KeyID uuid.UUID `json:"key_id"`
	// Signature is the raw signature bytes, or nil if there was an error.
	Signature []byte `json:"-"`
	// PublicKey is the hex-encoded public key.
	PublicKey string `json:"public_key,omitempty"`
	// KeyVersion is the version of the key that signed.
	KeyVersion int `json:"key_version,omitempty"`
	// Error is the error message if signing failed for this request.
	Error string `json:"error,omitempty"`
}

// Sign signs data with a key.
//
// Example:
//
//	result, err := client.Sign.Sign(ctx, keyID, []byte("message to sign"), false)
//	signature := result.Signature
func (s *SignService) Sign(ctx context.Context, keyID uuid.UUID, data []byte, prehashed bool) (*SignResponse, error) {
	req := map[string]interface{}{
		"data":      base64.StdEncoding.EncodeToString(data),
		"prehashed": prehashed,
	}

	var resp struct {
		Signature  string `json:"signature"`
		PublicKey  string `json:"public_key"`
		KeyVersion int    `json:"key_version"`
	}

	if err := s.client.post(ctx, fmt.Sprintf("/v1/keys/%s/sign", keyID), req, &resp); err != nil {
		return nil, err
	}

	sig, err := base64.StdEncoding.DecodeString(resp.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding: %w", err)
	}

	return &SignResponse{
		KeyID:      keyID,
		Signature:  sig,
		PublicKey:  resp.PublicKey,
		KeyVersion: resp.KeyVersion,
	}, nil
}

// SignBatch signs multiple messages in parallel.
// This is critical for Celestia's parallel blob submission pattern.
//
// Example:
//
//	results, err := client.Sign.SignBatch(ctx, banhbaoring.BatchSignRequest{
//	    Requests: []banhbaoring.SignRequest{
//	        {KeyID: worker1, Data: tx1},
//	        {KeyID: worker2, Data: tx2},
//	        {KeyID: worker3, Data: tx3},
//	        {KeyID: worker4, Data: tx4},
//	    },
//	})
//	// All 4 sign in parallel - completes in ~200ms, not 800ms!
func (s *SignService) SignBatch(ctx context.Context, req BatchSignRequest) ([]*BatchSignResult, error) {
	// Convert to API format
	requests := make([]map[string]interface{}, len(req.Requests))
	for i, r := range req.Requests {
		requests[i] = map[string]interface{}{
			"key_id":    r.KeyID.String(),
			"data":      base64.StdEncoding.EncodeToString(r.Data),
			"prehashed": r.Prehashed,
		}
	}

	apiReq := map[string]interface{}{
		"requests": requests,
	}

	var resp struct {
		Signatures []struct {
			KeyID      string `json:"key_id"`
			Signature  string `json:"signature,omitempty"`
			PublicKey  string `json:"public_key,omitempty"`
			KeyVersion int    `json:"key_version,omitempty"`
			Error      string `json:"error,omitempty"`
		} `json:"signatures"`
		Count int `json:"count"`
	}

	if err := s.client.post(ctx, "/v1/sign/batch", apiReq, &resp); err != nil {
		return nil, err
	}

	results := make([]*BatchSignResult, len(resp.Signatures))
	for i, sig := range resp.Signatures {
		keyID, _ := uuid.Parse(sig.KeyID)
		result := &BatchSignResult{
			KeyID:      keyID,
			PublicKey:  sig.PublicKey,
			KeyVersion: sig.KeyVersion,
			Error:      sig.Error,
		}

		if sig.Error == "" && sig.Signature != "" {
			sigBytes, err := base64.StdEncoding.DecodeString(sig.Signature)
			if err == nil {
				result.Signature = sigBytes
			}
		}

		results[i] = result
	}

	return results, nil
}

// SignWithOptions provides additional signing options.
type SignWithOptions struct {
	// Prehashed indicates if the data is already hashed.
	Prehashed bool
}

