// Package openbao provides a client for interacting with OpenBao.
package openbao

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Bidon15/banhbaoring/control-plane/internal/config"
	"github.com/Bidon15/banhbaoring/control-plane/internal/service"
)

// Client implements service.BaoKeyringInterface for the secp256k1 plugin.
type Client struct {
	address   string
	token     string
	mountPath string
	client    *http.Client
}

// NewClient creates a new OpenBao client.
func NewClient(cfg *config.OpenBaoConfig) *Client {
	// Create HTTP client that skips TLS verification for self-signed certs
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	
	mountPath := cfg.Secp256k1Path
	if mountPath == "" {
		mountPath = "transit" // Use standard transit engine
	}

	return &Client{
		address:   cfg.Address,
		token:     cfg.Token,
		mountPath: mountPath,
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

// keyResponse represents the response from OpenBao secp256k1 plugin.
type keyResponse struct {
	RequestID string `json:"request_id"`
	Data      struct {
		Name       string `json:"name"`
		Address    string `json:"address"`    // Hex address (RIPEMD160(SHA256(pubkey)))
		PublicKey  string `json:"public_key"` // Compressed secp256k1 public key (hex)
		Exportable bool   `json:"exportable"`
		Imported   bool   `json:"imported"`
		CreatedAt  string `json:"created_at"`
	} `json:"data"`
	Errors []string `json:"errors"`
}

// signResponse represents the response from OpenBao sign operations.
type signResponse struct {
	RequestID string `json:"request_id"`
	Data      struct {
		Signature string `json:"signature"`
		PublicKey string `json:"public_key"`
	} `json:"data"`
	Errors []string `json:"errors"`
}

// NewAccountWithOptions creates a new secp256k1 key in OpenBao.
func (c *Client) NewAccountWithOptions(uid string, opts service.KeyOptions) (pubKey []byte, address string, err error) {
	url := fmt.Sprintf("%s/v1/%s/keys/%s", c.address, c.mountPath, uid)
	
	body := map[string]interface{}{
		"exportable": opts.Exportable,
	}
	
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("X-Vault-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, "", fmt.Errorf("OpenBao error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var keyResp keyResponse
	if err := json.Unmarshal(respBody, &keyResp); err != nil {
		return nil, "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(keyResp.Errors) > 0 {
		return nil, "", fmt.Errorf("OpenBao error: %v", keyResp.Errors)
	}

	// Decode the hex public key
	pubKeyBytes, err := hex.DecodeString(keyResp.Data.PublicKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode public key: %w", err)
	}

	// Address is already in hex format from the plugin (RIPEMD160(SHA256(pubkey)))
	return pubKeyBytes, keyResp.Data.Address, nil
}

// Sign signs a message with the given key.
func (c *Client) Sign(uid string, msg []byte) (signature []byte, pubKey []byte, err error) {
	url := fmt.Sprintf("%s/v1/%s/sign/%s", c.address, c.mountPath, uid)
	
	body := map[string]interface{}{
		"input":     base64.StdEncoding.EncodeToString(msg),
		"prehashed": false,
	}
	
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("X-Vault-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("OpenBao error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var signResp signResponse
	if err := json.Unmarshal(respBody, &signResp); err != nil {
		return nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(signResp.Data.Signature) == 0 {
		return nil, nil, fmt.Errorf("empty signature returned")
	}

	sig, err := base64.StdEncoding.DecodeString(signResp.Data.Signature)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	pubKeyBytes, err := hex.DecodeString(signResp.Data.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	return sig, pubKeyBytes, nil
}

// Delete removes a key from OpenBao.
func (c *Client) Delete(uid string) error {
	url := fmt.Sprintf("%s/v1/%s/keys/%s", c.address, c.mountPath, uid)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("X-Vault-Token", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenBao error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetMetadata returns the metadata for a key.
func (c *Client) GetMetadata(uid string) (*service.KeyMetadata, error) {
	url := fmt.Sprintf("%s/v1/%s/keys/%s", c.address, c.mountPath, uid)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("X-Vault-Token", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenBao error (status %d): %s", resp.StatusCode, string(body))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var keyResp keyResponse
	if err := json.Unmarshal(respBody, &keyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	pubKeyBytes, err := hex.DecodeString(keyResp.Data.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	return &service.KeyMetadata{
		UID:         uid,
		Name:        keyResp.Data.Name,
		PubKeyBytes: pubKeyBytes,
		Address:     keyResp.Data.Address,
	}, nil
}

// ImportKey imports a key into OpenBao.
func (c *Client) ImportKey(uid string, ciphertext string, exportable bool) (pubKey []byte, address string, err error) {
	url := fmt.Sprintf("%s/v1/%s/import/%s", c.address, c.mountPath, uid)
	
	body := map[string]interface{}{
		"private_key": ciphertext,
		"exportable":  exportable,
	}
	
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("X-Vault-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("OpenBao error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var keyResp keyResponse
	if err := json.Unmarshal(respBody, &keyResp); err != nil {
		return nil, "", fmt.Errorf("failed to parse response: %w", err)
	}

	pubKeyBytes, err := hex.DecodeString(keyResp.Data.PublicKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode public key: %w", err)
	}

	return pubKeyBytes, keyResp.Data.Address, nil
}

// ExportKey exports a key from OpenBao.
func (c *Client) ExportKey(uid string) (string, error) {
	url := fmt.Sprintf("%s/v1/%s/export/%s", c.address, c.mountPath, uid)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("X-Vault-Token", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenBao error (status %d): %s", resp.StatusCode, string(body))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var exportResp struct {
		Data struct {
			PrivateKey string `json:"private_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &exportResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return exportResp.Data.PrivateKey, nil
}

// Compile-time check
var _ service.BaoKeyringInterface = (*Client)(nil)

