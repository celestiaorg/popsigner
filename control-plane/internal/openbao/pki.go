// Package openbao provides PKI certificate authority operations via OpenBao PKI secrets engine.
package openbao

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// PKIMountPath is the mount path for the PKI secrets engine.
	PKIMountPath = "pki"

	// CACommonName is the Common Name for the POPSigner CA.
	CACommonName = "POPSigner CA"

	// CAOrganization is the Organization for the POPSigner CA.
	CAOrganization = "POPSigner"

	// DefaultCATTL is the default TTL for the CA certificate (10 years).
	DefaultCATTL = "87600h"

	// DefaultCertTTL is the default TTL for issued certificates (1 year).
	DefaultCertTTL = "8760h"

	// MaxCertTTL is the maximum TTL for issued certificates (5 years).
	MaxCertTTL = "43800h"

	// ClientCertRoleName is the role name for issuing client certificates.
	ClientCertRoleName = "client-cert"
)

// PKIClient provides certificate authority operations via OpenBao PKI secrets engine.
type PKIClient struct {
	client *Client
	mount  string
}

// NewPKIClient creates a new PKI client.
func NewPKIClient(client *Client) *PKIClient {
	return &PKIClient{
		client: client,
		mount:  PKIMountPath,
	}
}

// SetMount sets a custom mount path for the PKI engine.
func (p *PKIClient) SetMount(mount string) {
	p.mount = mount
}

// CACertificate represents the CA certificate.
type CACertificate struct {
	CertificatePEM string    `json:"certificate_pem"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// IssueCertRequest represents a request to issue a certificate.
type IssueCertRequest struct {
	CommonName string `json:"common_name"` // Format: org_{org_id}
	TTL        string `json:"ttl,omitempty"`
}

// IssuedCertificate represents an issued certificate.
type IssuedCertificate struct {
	CertificatePEM string    `json:"certificate_pem"`
	PrivateKeyPEM  string    `json:"private_key_pem"`
	CACertPEM      string    `json:"ca_cert_pem"`
	SerialNumber   string    `json:"serial_number"`
	IssuedAt       time.Time `json:"issued_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// pkiResponse represents a generic response from OpenBao PKI operations.
type pkiResponse struct {
	RequestID string                 `json:"request_id"`
	Data      map[string]interface{} `json:"data"`
	Errors    []string               `json:"errors"`
}

// EnsurePKIEnabled ensures the PKI secrets engine is enabled at the mount path.
func (p *PKIClient) EnsurePKIEnabled(ctx context.Context) error {
	// Check if PKI is already mounted by trying to read the mount
	checkPath := "/v1/sys/mounts"
	resp, err := p.doRequest(ctx, "GET", checkPath, nil)
	if err != nil {
		return fmt.Errorf("checking mounts: %w", err)
	}

	// Check if pki/ is in the mounts
	mountKey := p.mount + "/"
	if resp.Data != nil {
		if _, exists := resp.Data[mountKey]; exists {
			// Already mounted
			return nil
		}
	}

	// Enable PKI secrets engine
	enablePath := fmt.Sprintf("/v1/sys/mounts/%s", p.mount)
	data := map[string]interface{}{
		"type":        "pki",
		"description": "POPSigner PKI CA for mTLS client certificates",
		"config": map[string]interface{}{
			"default_lease_ttl": DefaultCertTTL,
			"max_lease_ttl":     MaxCertTTL,
		},
	}

	_, err = p.doRequest(ctx, "POST", enablePath, data)
	if err != nil {
		return fmt.Errorf("enabling PKI secrets engine: %w", err)
	}

	return nil
}

// InitializeCA initializes the Certificate Authority.
// This generates a new root CA if one doesn't exist.
func (p *PKIClient) InitializeCA(ctx context.Context) (*CACertificate, error) {
	// Check if CA already exists
	existing, err := p.GetCACertificate(ctx)
	if err == nil && existing != nil {
		return existing, nil
	}

	// Generate root CA with EC P-256 key
	path := fmt.Sprintf("/v1/%s/root/generate/internal", p.mount)
	data := map[string]interface{}{
		"common_name":          CACommonName,
		"organization":         CAOrganization,
		"ttl":                  DefaultCATTL,
		"key_type":             "ec",
		"key_bits":             256,
		"exclude_cn_from_sans": true,
	}

	resp, err := p.doRequest(ctx, "POST", path, data)
	if err != nil {
		return nil, fmt.Errorf("generating CA: %w", err)
	}

	certPEM, ok := resp.Data["certificate"].(string)
	if !ok {
		return nil, fmt.Errorf("no certificate in response")
	}

	// Configure URLs
	if err := p.configureURLs(ctx); err != nil {
		return nil, fmt.Errorf("configuring URLs: %w", err)
	}

	// Create a role for issuing client certificates
	if err := p.createClientCertRole(ctx); err != nil {
		return nil, fmt.Errorf("creating client cert role: %w", err)
	}

	return &CACertificate{
		CertificatePEM: certPEM,
		ExpiresAt:      time.Now().Add(10 * 365 * 24 * time.Hour),
	}, nil
}

// configureURLs configures the issuing and CRL URLs.
func (p *PKIClient) configureURLs(ctx context.Context) error {
	path := fmt.Sprintf("/v1/%s/config/urls", p.mount)
	data := map[string]interface{}{
		"issuing_certificates":    []string{},
		"crl_distribution_points": []string{},
	}

	_, err := p.doRequest(ctx, "POST", path, data)
	return err
}

// createClientCertRole creates a role for issuing client certificates.
func (p *PKIClient) createClientCertRole(ctx context.Context) error {
	path := fmt.Sprintf("/v1/%s/roles/%s", p.mount, ClientCertRoleName)
	data := map[string]interface{}{
		"allow_any_name":    true,
		"enforce_hostnames": false,
		"allow_ip_sans":     false,
		"server_flag":       false,
		"client_flag":       true,
		"code_signing_flag": false,
		"key_type":          "ec",
		"key_bits":          256,
		"key_usage":         []string{"DigitalSignature", "KeyEncipherment"},
		"ext_key_usage":     []string{"ClientAuth"},
		"ttl":               DefaultCertTTL,
		"max_ttl":           MaxCertTTL,
		"generate_lease":    false,
		"no_store":          false,
	}

	_, err := p.doRequest(ctx, "POST", path, data)
	return err
}

// GetCACertificate retrieves the current CA certificate.
func (p *PKIClient) GetCACertificate(ctx context.Context) (*CACertificate, error) {
	path := fmt.Sprintf("/v1/%s/cert/ca", p.mount)

	resp, err := p.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("reading CA certificate: %w", err)
	}

	if resp.Data == nil {
		return nil, fmt.Errorf("CA not initialized")
	}

	certPEM, ok := resp.Data["certificate"].(string)
	if !ok || certPEM == "" {
		return nil, fmt.Errorf("no certificate in response")
	}

	// Parse to get expiration
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing certificate: %w", err)
	}

	return &CACertificate{
		CertificatePEM: certPEM,
		ExpiresAt:      cert.NotAfter,
	}, nil
}

// IssueCertificate issues a new client certificate.
func (p *PKIClient) IssueCertificate(ctx context.Context, req *IssueCertRequest) (*IssuedCertificate, error) {
	path := fmt.Sprintf("/v1/%s/issue/%s", p.mount, ClientCertRoleName)

	ttl := DefaultCertTTL
	if req.TTL != "" {
		ttl = req.TTL
	}

	data := map[string]interface{}{
		"common_name": req.CommonName,
		"ttl":         ttl,
	}

	resp, err := p.doRequest(ctx, "POST", path, data)
	if err != nil {
		return nil, fmt.Errorf("issuing certificate: %w", err)
	}

	certPEM, ok := resp.Data["certificate"].(string)
	if !ok {
		return nil, fmt.Errorf("no certificate in response")
	}

	privateKeyPEM, ok := resp.Data["private_key"].(string)
	if !ok {
		return nil, fmt.Errorf("no private key in response")
	}

	caPEM, ok := resp.Data["issuing_ca"].(string)
	if !ok {
		return nil, fmt.Errorf("no issuing CA in response")
	}

	serialNumber, ok := resp.Data["serial_number"].(string)
	if !ok {
		return nil, fmt.Errorf("no serial number in response")
	}

	// Parse certificate to get details
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing certificate: %w", err)
	}

	return &IssuedCertificate{
		CertificatePEM: certPEM,
		PrivateKeyPEM:  privateKeyPEM,
		CACertPEM:      caPEM,
		SerialNumber:   serialNumber,
		IssuedAt:       cert.NotBefore,
		ExpiresAt:      cert.NotAfter,
	}, nil
}

// RevokeCertificate revokes a certificate by serial number.
func (p *PKIClient) RevokeCertificate(ctx context.Context, serialNumber string) error {
	path := fmt.Sprintf("/v1/%s/revoke", p.mount)
	data := map[string]interface{}{
		"serial_number": serialNumber,
	}

	_, err := p.doRequest(ctx, "POST", path, data)
	if err != nil {
		return fmt.Errorf("revoking certificate: %w", err)
	}

	return nil
}

// doRequest performs an HTTP request to the OpenBao PKI endpoint.
func (p *PKIClient) doRequest(ctx context.Context, method, path string, data map[string]interface{}) (*pkiResponse, error) {
	url := p.client.address + path

	var body io.Reader
	if data != nil {
		jsonBody, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Vault-Token", p.client.token)
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.client.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle non-success status codes
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("OpenBao error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Handle empty response (204 No Content)
	if resp.StatusCode == http.StatusNoContent || len(respBody) == 0 {
		return &pkiResponse{}, nil
	}

	var pkiResp pkiResponse
	if err := json.Unmarshal(respBody, &pkiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(pkiResp.Errors) > 0 {
		return nil, fmt.Errorf("OpenBao error: %v", pkiResp.Errors)
	}

	return &pkiResp, nil
}

