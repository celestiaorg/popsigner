package api

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Sign signs data with a key.
func (c *Client) Sign(ctx context.Context, keyID uuid.UUID, data string, prehashed bool) (*SignResponse, error) {
	body := map[string]interface{}{
		"data":      data,
		"prehashed": prehashed,
	}

	var resp signResponse
	if err := c.Post(ctx, fmt.Sprintf("/v1/keys/%s/sign", keyID), body, &resp); err != nil {
		return nil, err
	}

	resp.Data.KeyID = keyID
	return &resp.Data, nil
}

// SignBatch signs multiple messages in parallel.
func (c *Client) SignBatch(ctx context.Context, requests []SignRequest) (*BatchSignResponse, error) {
	// Convert to API format
	reqs := make([]map[string]interface{}, len(requests))
	for i, r := range requests {
		reqs[i] = map[string]interface{}{
			"key_id":    r.KeyID.String(),
			"data":      r.Data,
			"prehashed": r.Prehashed,
		}
	}

	body := map[string]interface{}{
		"requests": reqs,
	}

	var resp batchSignResponseWrapper
	if err := c.Post(ctx, "/v1/sign/batch", body, &resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}

// SignEVM signs an EVM transaction hash with the specified key.
// The txHash should be the hash of the unsigned transaction (32 bytes).
// Returns the signature in Ethereum format (v, r, s concatenated, 65 bytes).
func (c *Client) SignEVM(ctx context.Context, keyID uuid.UUID, txHash []byte, chainID uint64) (*SignEVMResponse, error) {
	body := map[string]interface{}{
		"tx_hash":  fmt.Sprintf("0x%x", txHash),
		"chain_id": chainID,
	}

	var resp signEVMResponseWrapper
	if err := c.Post(ctx, fmt.Sprintf("/v1/keys/%s/sign-evm", keyID), body, &resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}
