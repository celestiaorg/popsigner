// Package openbao provides a client for interacting with OpenBao.
package openbao

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/Bidon15/popsigner/control-plane/internal/config"
	"github.com/Bidon15/popsigner/control-plane/internal/service"
	"golang.org/x/crypto/sha3"
)

// deriveEthAddressFromPubKey derives an Ethereum address from a compressed secp256k1 public key.
// Returns the address as a 0x-prefixed hex string.
func deriveEthAddressFromPubKey(compressedPubKey []byte) (string, error) {
	if len(compressedPubKey) != 33 {
		return "", fmt.Errorf("invalid compressed public key length: got %d, expected 33", len(compressedPubKey))
	}

	// Parse the compressed public key
	// First byte: 0x02 (even y) or 0x03 (odd y)
	// Remaining 32 bytes: x coordinate
	prefix := compressedPubKey[0]
	if prefix != 0x02 && prefix != 0x03 {
		return "", fmt.Errorf("invalid public key prefix: %x", prefix)
	}

	// For Ethereum address derivation, we need to decompress the public key
	// This requires secp256k1 curve operations. For simplicity, we'll use a pure Go approach.
	// The uncompressed key is 65 bytes: 0x04 + x (32 bytes) + y (32 bytes)
	
	// Decompress the public key using the secp256k1 curve
	uncompressedPubKey, err := decompressSecp256k1PubKey(compressedPubKey)
	if err != nil {
		return "", fmt.Errorf("failed to decompress public key: %w", err)
	}

	// Ethereum address = last 20 bytes of Keccak256(uncompressed_pubkey[1:])
	// Skip the 0x04 prefix (first byte)
	hash := sha3.NewLegacyKeccak256()
	hash.Write(uncompressedPubKey[1:]) // Skip the 0x04 prefix
	fullHash := hash.Sum(nil)
	
	// Take last 20 bytes
	address := fullHash[12:]
	
	return "0x" + hex.EncodeToString(address), nil
}

// decompressSecp256k1PubKey decompresses a 33-byte compressed secp256k1 public key to 65-byte uncompressed.
func decompressSecp256k1PubKey(compressed []byte) ([]byte, error) {
	// Use crypto/elliptic with secp256k1 curve parameters
	// secp256k1 parameters
	curve := secp256k1Curve()
	
	x, y := curve.UnmarshalCompressed(compressed)
	if x == nil {
		return nil, fmt.Errorf("failed to unmarshal compressed public key")
	}
	
	// Create uncompressed format: 0x04 + x + y
	uncompressed := make([]byte, 65)
	uncompressed[0] = 0x04
	xBytes := x.Bytes()
	yBytes := y.Bytes()
	
	// Pad to 32 bytes each
	copy(uncompressed[1+32-len(xBytes):33], xBytes)
	copy(uncompressed[33+32-len(yBytes):65], yBytes)
	
	return uncompressed, nil
}

// secp256k1Curve returns the secp256k1 elliptic curve.
// We use a custom implementation since Go's crypto/elliptic doesn't include secp256k1.
func secp256k1Curve() *secp256k1 {
	return &secp256k1{}
}

// secp256k1 implements elliptic.Curve for secp256k1.
// This is a minimal implementation for decompressing public keys.
type secp256k1 struct{}

func (secp256k1) UnmarshalCompressed(data []byte) (*big.Int, *big.Int) {
	if len(data) != 33 || (data[0] != 0x02 && data[0] != 0x03) {
		return nil, nil
	}

	// secp256k1 curve parameters
	p, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F", 16)
	
	x := new(big.Int).SetBytes(data[1:33])
	
	// y² = x³ + 7 (mod p)
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)
	x3.Mod(x3, p)
	
	y2 := new(big.Int).Add(x3, big.NewInt(7))
	y2.Mod(y2, p)
	
	// Compute y = sqrt(y²) mod p using Tonelli-Shanks
	y := new(big.Int).ModSqrt(y2, p)
	if y == nil {
		return nil, nil
	}
	
	// Choose the correct y based on the prefix (0x02 = even, 0x03 = odd)
	if data[0] == 0x02 {
		if y.Bit(0) == 1 { // y is odd, we need even
			y.Sub(p, y)
		}
	} else {
		if y.Bit(0) == 0 { // y is even, we need odd
			y.Sub(p, y)
		}
	}
	
	return x, y
}

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
		Address    string `json:"address"`     // Hex address (RIPEMD160(SHA256(pubkey)))
		EthAddress string `json:"eth_address"` // Ethereum address (0x + Keccak256(pubkey)[12:])
		PublicKey  string `json:"public_key"`  // Compressed secp256k1 public key (hex)
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
func (c *Client) NewAccountWithOptions(uid string, opts service.KeyOptions) (pubKey []byte, address string, ethAddress string, err error) {
	url := fmt.Sprintf("%s/v1/%s/keys/%s", c.address, c.mountPath, uid)

	body := map[string]interface{}{
		"exportable": opts.Exportable,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, "", "", fmt.Errorf("OpenBao error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var keyResp keyResponse
	if err := json.Unmarshal(respBody, &keyResp); err != nil {
		return nil, "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(keyResp.Errors) > 0 {
		return nil, "", "", fmt.Errorf("OpenBao error: %v", keyResp.Errors)
	}

	// Decode the hex public key
	pubKeyBytes, err := hex.DecodeString(keyResp.Data.PublicKey)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to decode public key: %w", err)
	}

	// Address is already in hex format from the plugin (RIPEMD160(SHA256(pubkey)))
	// EthAddress is in 0x-prefixed hex format (Keccak256(pubkey)[12:])
	ethAddr := keyResp.Data.EthAddress
	
	// If OpenBao didn't return eth_address, derive it from the public key
	if ethAddr == "" && len(pubKeyBytes) == 33 {
		derivedAddr, err := deriveEthAddressFromPubKey(pubKeyBytes)
		if err == nil {
			ethAddr = derivedAddr
		}
	}
	
	return pubKeyBytes, keyResp.Data.Address, ethAddr, nil
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

	// Treat 404 as success - key is already gone (e.g., legacy keys not in OpenBao)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
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

	ethAddr := keyResp.Data.EthAddress
	
	// If OpenBao didn't return eth_address, derive it from the public key
	if ethAddr == "" && len(pubKeyBytes) == 33 {
		derivedAddr, err := deriveEthAddressFromPubKey(pubKeyBytes)
		if err == nil {
			ethAddr = derivedAddr
		}
	}

	return &service.KeyMetadata{
		UID:         uid,
		Name:        keyResp.Data.Name,
		PubKeyBytes: pubKeyBytes,
		Address:     keyResp.Data.Address,
		EthAddress:  ethAddr,
	}, nil
}

// ImportKey imports a key into OpenBao.
func (c *Client) ImportKey(uid string, ciphertext string, exportable bool) (pubKey []byte, address string, ethAddress string, err error) {
	url := fmt.Sprintf("%s/v1/%s/import/%s", c.address, c.mountPath, uid)

	body := map[string]interface{}{
		"private_key": ciphertext,
		"exportable":  exportable,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("OpenBao error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var keyResp keyResponse
	if err := json.Unmarshal(respBody, &keyResp); err != nil {
		return nil, "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	pubKeyBytes, err := hex.DecodeString(keyResp.Data.PublicKey)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to decode public key: %w", err)
	}

	ethAddr := keyResp.Data.EthAddress
	
	// If OpenBao didn't return eth_address, derive it from the public key
	if ethAddr == "" && len(pubKeyBytes) == 33 {
		derivedAddr, err := deriveEthAddressFromPubKey(pubKeyBytes)
		if err == nil {
			ethAddr = derivedAddr
		}
	}

	return pubKeyBytes, keyResp.Data.Address, ethAddr, nil
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

// SignEVMResponse represents the response from the sign-evm endpoint.
type SignEVMResponse struct {
	V          string `json:"v"`
	R          string `json:"r"`
	S          string `json:"s"`
	VInt       int64  `json:"v_int"`
	PublicKey  string `json:"public_key"`
	EthAddress string `json:"eth_address"`
}

// SignEVM signs a hash with EIP-155 format via the plugin.
// The hash should be base64-encoded.
// chainID of 0 means legacy signing (v = 27/28).
func (c *Client) SignEVM(keyName, hashB64 string, chainID int64) (*SignEVMResponse, error) {
	url := fmt.Sprintf("%s/v1/%s/sign-evm/%s", c.address, c.mountPath, keyName)

	body := map[string]interface{}{
		"hash":     hashB64,
		"chain_id": chainID,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenBao error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var signResp struct {
		Data SignEVMResponse `json:"data"`
	}
	if err := json.Unmarshal(respBody, &signResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &signResp.Data, nil
}

// PKI returns the PKI client for certificate authority operations.
func (c *Client) PKI() *PKIClient {
	return NewPKIClient(c)
}

// HealthCheck checks the health of the OpenBao server.
func (c *Client) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1/sys/health", c.address)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Health check doesn't require auth
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// OpenBao returns various status codes for health:
	// 200 - initialized, unsealed, active
	// 429 - unsealed and standby
	// 472 - disaster recovery secondary, active
	// 473 - performance standby
	// 501 - not initialized
	// 503 - sealed
	if resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusTooManyRequests ||
		resp.StatusCode == 472 ||
		resp.StatusCode == 473 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("OpenBao unhealthy (status %d): %s", resp.StatusCode, string(body))
}

// ===============================================
// KV v2 Secret Store Methods
// ===============================================

// kvSecretResponse represents the response from KV v2 read operations.
type kvSecretResponse struct {
	RequestID string `json:"request_id"`
	Data      struct {
		Data     map[string]interface{} `json:"data"`
		Metadata struct {
			CreatedTime  string `json:"created_time"`
			Version      int    `json:"version"`
			Destroyed    bool   `json:"destroyed"`
			DeletionTime string `json:"deletion_time"`
		} `json:"metadata"`
	} `json:"data"`
	Errors []string `json:"errors"`
}

// ReadKVSecret reads a secret from the KV v2 secret engine.
// Path should be the logical path without the "data/" prefix (e.g., "orgs/123/api-key").
func (c *Client) ReadKVSecret(path string) (map[string]interface{}, error) {
	// KV v2 uses the "data/" prefix for read/write operations
	url := fmt.Sprintf("%s/v1/secret/data/%s", c.address, path)

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
		return nil, nil // Secret not found
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenBao error (status %d): %s", resp.StatusCode, string(body))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var kvResp kvSecretResponse
	if err := json.Unmarshal(respBody, &kvResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(kvResp.Errors) > 0 {
		return nil, fmt.Errorf("OpenBao error: %v", kvResp.Errors)
	}

	return kvResp.Data.Data, nil
}

// WriteKVSecret writes a secret to the KV v2 secret engine.
// Path should be the logical path without the "data/" prefix (e.g., "orgs/123/api-key").
func (c *Client) WriteKVSecret(path string, data map[string]interface{}) error {
	// KV v2 uses the "data/" prefix and wraps data in a "data" object
	url := fmt.Sprintf("%s/v1/secret/data/%s", c.address, path)

	body := map[string]interface{}{
		"data": data,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenBao error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// DeleteKVSecret deletes a secret from the KV v2 secret engine.
// Path should be the logical path without the "data/" prefix.
func (c *Client) DeleteKVSecret(path string) error {
	// KV v2 uses the "data/" prefix for soft delete
	url := fmt.Sprintf("%s/v1/secret/data/%s", c.address, path)

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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenBao error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Compile-time check
var _ service.BaoKeyringInterface = (*Client)(nil)

