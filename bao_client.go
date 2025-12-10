package banhbaoring

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// BaoClient handles HTTP communication with OpenBao.
type BaoClient struct {
	httpClient    *http.Client
	baseURL       string
	token         string
	namespace     string
	secp256k1Path string
}

// NewBaoClient creates a new client instance.
func NewBaoClient(cfg Config) (*BaoClient, error) {
	cfg = cfg.WithDefaults()

	// Build TLS config
	tlsConfig := cfg.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if cfg.SkipTLSVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
	}

	return &BaoClient{
		httpClient:    &http.Client{Timeout: cfg.HTTPTimeout, Transport: transport},
		baseURL:       strings.TrimSuffix(cfg.BaoAddr, "/"),
		token:         cfg.BaoToken,
		namespace:     cfg.BaoNamespace,
		secp256k1Path: cfg.Secp256k1Path,
	}, nil
}

// CreateKey creates a new secp256k1 key.
func (c *BaoClient) CreateKey(ctx context.Context, name string, opts KeyOptions) (*KeyInfo, error) {
	path := fmt.Sprintf("/v1/%s/keys/%s", c.secp256k1Path, name)
	body := map[string]interface{}{"exportable": opts.Exportable}

	resp, err := c.post(ctx, path, body)
	if err != nil {
		return nil, WrapKeyError("create", name, err)
	}

	var result struct {
		Data KeyInfo `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, WrapKeyError("create", name, err)
	}
	return &result.Data, nil
}

// GetKey retrieves key info.
func (c *BaoClient) GetKey(ctx context.Context, name string) (*KeyInfo, error) {
	path := fmt.Sprintf("/v1/%s/keys/%s", c.secp256k1Path, name)
	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, WrapKeyError("get", name, err)
	}

	var result struct {
		Data KeyInfo `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, WrapKeyError("get", name, err)
	}
	return &result.Data, nil
}

// ListKeys lists all keys.
func (c *BaoClient) ListKeys(ctx context.Context) ([]string, error) {
	path := fmt.Sprintf("/v1/%s/keys", c.secp256k1Path)
	resp, err := c.list(ctx, path)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return result.Data.Keys, nil
}

// DeleteKey deletes a key.
func (c *BaoClient) DeleteKey(ctx context.Context, name string) error {
	// Enable deletion first
	configPath := fmt.Sprintf("/v1/%s/keys/%s/config", c.secp256k1Path, name)
	_, _ = c.post(ctx, configPath, map[string]interface{}{"deletion_allowed": true})

	path := fmt.Sprintf("/v1/%s/keys/%s", c.secp256k1Path, name)
	return c.delete(ctx, path)
}

// Sign signs data and returns 64-byte Cosmos signature.
func (c *BaoClient) Sign(ctx context.Context, keyName string, data []byte, prehashed bool) ([]byte, error) {
	path := fmt.Sprintf("/v1/%s/sign/%s", c.secp256k1Path, keyName)
	body := map[string]interface{}{
		"input":         base64.StdEncoding.EncodeToString(data),
		"prehashed":     prehashed,
		"output_format": "cosmos",
	}
	if !prehashed {
		body["hash_algorithm"] = "sha256"
	}

	resp, err := c.post(ctx, path, body)
	if err != nil {
		return nil, WrapKeyError("sign", keyName, err)
	}

	var result struct {
		Data SignResponse `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, WrapKeyError("sign", keyName, err)
	}

	sig, err := base64.StdEncoding.DecodeString(result.Data.Signature)
	if err != nil {
		return nil, WrapKeyError("sign", keyName, err)
	}
	if len(sig) != 64 {
		return nil, WrapKeyError("sign", keyName, ErrInvalidSignature)
	}
	return sig, nil
}

// ImportKey imports a key into OpenBao.
// The ciphertext should be base64-encoded raw private key bytes.
func (c *BaoClient) ImportKey(ctx context.Context, name string, ciphertext string, exportable bool) (*KeyInfo, error) {
	path := fmt.Sprintf("/v1/%s/keys/%s/import", c.secp256k1Path, name)
	body := map[string]interface{}{
		"ciphertext": ciphertext,
		"exportable": exportable,
	}

	resp, err := c.post(ctx, path, body)
	if err != nil {
		return nil, WrapKeyError("import", name, err)
	}

	var result struct {
		Data KeyInfo `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, WrapKeyError("import", name, err)
	}
	return &result.Data, nil
}

// ExportKey exports a key from OpenBao.
// Returns the base64-encoded private key if the key was created with exportable=true.
func (c *BaoClient) ExportKey(ctx context.Context, name string) (string, *KeyInfo, error) {
	path := fmt.Sprintf("/v1/%s/export/%s", c.secp256k1Path, name)
	resp, err := c.get(ctx, path)
	if err != nil {
		return "", nil, WrapKeyError("export", name, err)
	}

	var result struct {
		Data struct {
			Name      string            `json:"name"`
			PublicKey string            `json:"public_key"`
			Address   string            `json:"address"`
			Keys      map[string]string `json:"keys"`
			CreatedAt time.Time         `json:"created_at"`
			Imported  bool              `json:"imported"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", nil, WrapKeyError("export", name, err)
	}

	// Get the latest version of the key (version "1" for imported keys)
	keyData, ok := result.Data.Keys["1"]
	if !ok {
		return "", nil, WrapKeyError("export", name, ErrKeyNotFound)
	}

	info := &KeyInfo{
		Name:      result.Data.Name,
		PublicKey: result.Data.PublicKey,
		Address:   result.Data.Address,
		CreatedAt: result.Data.CreatedAt,
	}

	return keyData, info, nil
}

// Health checks OpenBao status.
func (c *BaoClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/sys/health", nil)
	if err != nil {
		return ErrBaoConnection
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ErrBaoConnection
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case 200:
		return nil
	case 503:
		return ErrBaoSealed
	default:
		return ErrBaoUnavailable
	}
}

// HTTP helpers
func (c *BaoClient) get(ctx context.Context, path string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil)
}

func (c *BaoClient) post(ctx context.Context, path string, body interface{}) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPost, path, body)
}

func (c *BaoClient) delete(ctx context.Context, path string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}

func (c *BaoClient) list(ctx context.Context, path string) ([]byte, error) {
	return c.doRequest(ctx, "LIST", path, nil)
}

func (c *BaoClient) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, ErrBaoConnection
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, ErrBaoConnection
	}

	req.Header.Set("X-Vault-Token", c.token)
	req.Header.Set("Content-Type", "application/json")
	if c.namespace != "" {
		req.Header.Set("X-Vault-Namespace", c.namespace)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ErrBaoConnection
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ErrBaoConnection
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Errors []string `json:"errors"`
		}
		_ = json.Unmarshal(respBody, &errResp)
		return nil, NewBaoError(resp.StatusCode, errResp.Errors, "")
	}

	return respBody, nil
}
