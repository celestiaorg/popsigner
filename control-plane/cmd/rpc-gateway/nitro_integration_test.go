// Package main contains integration tests for Arbitrum Nitro remote signer functionality.
//
// This test simulates how Arbitrum Nitro components (batch-poster, staker, validator)
// interact with POPSigner's RPC Gateway using mTLS authentication.
//
// Arbitrum Nitro Configuration Example:
//
//	./nitro \
//	  --node.batch-poster.enable=true \
//	  --node.batch-poster.data-poster.external-signer.url=https://rpc.popsigner.com \
//	  --node.batch-poster.data-poster.external-signer.address=0x742d35Cc6634C0532925a3b844Bc454e4438f44e \
//	  --node.batch-poster.data-poster.external-signer.method=eth_signTransaction \
//	  --node.batch-poster.data-poster.external-signer.root-ca=/certs/popsigner-ca.crt \
//	  --node.batch-poster.data-poster.external-signer.client-cert=/certs/client.crt \
//	  --node.batch-poster.data-poster.external-signer.client-private-key=/certs/client.key
//
// Run these tests against a deployed RPC gateway with mTLS enabled:
//
//	POPSIGNER_MTLS_URL=https://your-gateway:8546 \
//	POPSIGNER_CA_CERT=/path/to/popsigner-ca.crt \
//	POPSIGNER_CLIENT_CERT=/path/to/client.crt \
//	POPSIGNER_CLIENT_KEY=/path/to/client.key \
//	POPSIGNER_SIGNER_ADDRESS=0x... \
//	go test -v -tags=integration ./cmd/rpc-gateway/ -run TestNitro
package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Environment variables for Nitro integration tests.
const (
	envMTLSURL      = "POPSIGNER_MTLS_URL"       // e.g., https://rpc-mtls.popsigner.com
	envCACert       = "POPSIGNER_CA_CERT"        // Path to CA certificate
	envClientCert   = "POPSIGNER_CLIENT_CERT"    // Path to client certificate
	envClientKey    = "POPSIGNER_CLIENT_KEY"     // Path to client private key
	envNitroAddress = "POPSIGNER_SIGNER_ADDRESS" // Ethereum address to sign with
	envNitroChainID = "POPSIGNER_CHAIN_ID"       // Chain ID (default: Arbitrum One)
)

// NitroTestConfig holds the configuration for Nitro integration tests.
type NitroTestConfig struct {
	MTLSURL        string
	CACertPath     string
	ClientCertPath string
	ClientKeyPath  string
	SignerAddress  string
	ChainID        string
}

// loadNitroTestConfig loads configuration from environment variables.
func loadNitroTestConfig(t *testing.T) *NitroTestConfig {
	t.Helper()

	mtlsURL := os.Getenv(envMTLSURL)
	if mtlsURL == "" {
		t.Skipf("Skipping Nitro integration test: %s not set", envMTLSURL)
	}

	caCert := os.Getenv(envCACert)
	if caCert == "" {
		t.Skipf("Skipping Nitro integration test: %s not set", envCACert)
	}

	clientCert := os.Getenv(envClientCert)
	if clientCert == "" {
		t.Skipf("Skipping Nitro integration test: %s not set", envClientCert)
	}

	clientKey := os.Getenv(envClientKey)
	if clientKey == "" {
		t.Skipf("Skipping Nitro integration test: %s not set", envClientKey)
	}

	signerAddress := os.Getenv(envNitroAddress)
	if signerAddress == "" {
		t.Skipf("Skipping Nitro integration test: %s not set", envNitroAddress)
	}

	chainID := os.Getenv(envNitroChainID)
	if chainID == "" {
		chainID = "0xa4b1" // Default to Arbitrum One (42161)
	} else if !strings.HasPrefix(chainID, "0x") {
		// Convert decimal to hex if needed
		var id int64
		fmt.Sscanf(chainID, "%d", &id)
		chainID = fmt.Sprintf("0x%x", id)
	}

	return &NitroTestConfig{
		MTLSURL:        mtlsURL,
		CACertPath:     caCert,
		ClientCertPath: clientCert,
		ClientKeyPath:  clientKey,
		SignerAddress:  signerAddress,
		ChainID:        chainID,
	}
}

// createMTLSHTTPClient creates an HTTP client with mTLS configuration.
func createMTLSHTTPClient(t *testing.T, cfg *NitroTestConfig) *http.Client {
	t.Helper()

	// Load CA certificate
	caCert, err := os.ReadFile(cfg.CACertPath)
	if err != nil {
		t.Fatalf("Failed to read CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		t.Fatal("Failed to parse CA certificate")
	}

	// Load client certificate and key
	clientCert, err := tls.LoadX509KeyPair(cfg.ClientCertPath, cfg.ClientKeyPath)
	if err != nil {
		t.Fatalf("Failed to load client certificate: %v", err)
	}

	tlsConfig := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{clientCert},
		MinVersion:   tls.VersionTLS12,
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 30 * time.Second,
	}
}

// sendMTLSRPCRequest sends a JSON-RPC request using mTLS.
func sendMTLSRPCRequest(client *http.Client, url string, req *JSONRPCRequest) (*JSONRPCResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &rpcResp, nil
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestNitroHealthCheck verifies the mTLS gateway health endpoint.
func TestNitroHealthCheck(t *testing.T) {
	cfg := loadNitroTestConfig(t)
	client := createMTLSHTTPClient(t, cfg)

	resp, err := client.Get(cfg.MTLSURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", body["status"])
	}

	t.Logf("✓ Health check passed: %+v", body)
}

// TestNitroEthAccounts tests the eth_accounts method via mTLS.
func TestNitroEthAccounts(t *testing.T) {
	cfg := loadNitroTestConfig(t)
	client := createMTLSHTTPClient(t, cfg)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_accounts",
		Params:  []interface{}{},
		ID:      1,
	}

	resp, err := sendMTLSRPCRequest(client, cfg.MTLSURL, req)
	if err != nil {
		t.Fatalf("RPC request failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("RPC error: code=%d, message=%s", resp.Error.Code, resp.Error.Message)
	}

	var accounts []string
	if err := json.Unmarshal(resp.Result, &accounts); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Check if our signer address is in the list
	found := false
	normalizedSigner := strings.ToLower(cfg.SignerAddress)
	for _, addr := range accounts {
		if strings.ToLower(addr) == normalizedSigner {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Signer address %s not found in accounts: %v", cfg.SignerAddress, accounts)
	}

	t.Logf("✓ eth_accounts returned %d addresses", len(accounts))
	for _, addr := range accounts {
		t.Logf("  - %s", addr)
	}
}

// TestNitroEthSignTransaction tests the eth_signTransaction method via mTLS.
// This is the core method used by Nitro batch-poster and staker.
func TestNitroEthSignTransaction(t *testing.T) {
	cfg := loadNitroTestConfig(t)
	client := createMTLSHTTPClient(t, cfg)

	// Construct a test transaction (typical batch submission tx)
	txParams := map[string]interface{}{
		"from":     cfg.SignerAddress,
		"to":       "0x0000000000000000000000000000000000000000",
		"gas":      "0x5208",     // 21000
		"gasPrice": "0x3b9aca00", // 1 Gwei
		"value":    "0x0",
		"nonce":    "0x0",
		"data":     "0x",
		"chainId":  cfg.ChainID,
	}

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_signTransaction",
		Params:  []interface{}{txParams},
		ID:      1,
	}

	t.Logf("Sending eth_signTransaction request via mTLS:")
	t.Logf("  from:    %s", cfg.SignerAddress)
	t.Logf("  chainId: %s", cfg.ChainID)

	resp, err := sendMTLSRPCRequest(client, cfg.MTLSURL, req)
	if err != nil {
		t.Fatalf("RPC request failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("RPC error: code=%d, message=%s", resp.Error.Code, resp.Error.Message)
	}

	var signedTx string
	if err := json.Unmarshal(resp.Result, &signedTx); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if !strings.HasPrefix(signedTx, "0x") {
		t.Errorf("Expected signed tx to start with 0x, got: %s", signedTx[:20])
	}

	t.Logf("✓ eth_signTransaction succeeded via mTLS")
	t.Logf("  Signed TX: %s...%s (%d bytes)", signedTx[:10], signedTx[len(signedTx)-8:], len(signedTx)/2)
}

// TestNitroEIP1559Transaction tests signing an EIP-1559 transaction via mTLS.
func TestNitroEIP1559Transaction(t *testing.T) {
	cfg := loadNitroTestConfig(t)
	client := createMTLSHTTPClient(t, cfg)

	// EIP-1559 transaction
	txParams := map[string]interface{}{
		"from":                 cfg.SignerAddress,
		"to":                   "0x0000000000000000000000000000000000000000",
		"gas":                  "0x5208",
		"maxFeePerGas":         "0x77359400", // 2 Gwei
		"maxPriorityFeePerGas": "0x3b9aca00", // 1 Gwei
		"value":                "0x0",
		"nonce":                "0x0",
		"data":                 "0x",
		"chainId":              cfg.ChainID,
	}

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_signTransaction",
		Params:  []interface{}{txParams},
		ID:      1,
	}

	t.Logf("Sending EIP-1559 eth_signTransaction request via mTLS:")
	t.Logf("  from:                 %s", cfg.SignerAddress)
	t.Logf("  maxFeePerGas:         %s", txParams["maxFeePerGas"])
	t.Logf("  maxPriorityFeePerGas: %s", txParams["maxPriorityFeePerGas"])

	resp, err := sendMTLSRPCRequest(client, cfg.MTLSURL, req)
	if err != nil {
		t.Fatalf("RPC request failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("RPC error: code=%d, message=%s", resp.Error.Code, resp.Error.Message)
	}

	var signedTx string
	if err := json.Unmarshal(resp.Result, &signedTx); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	t.Logf("✓ EIP-1559 eth_signTransaction succeeded via mTLS")
	t.Logf("  Signed TX: %s...%s", signedTx[:10], signedTx[len(signedTx)-8:])
}

// TestNitroBatchSubmission simulates a Nitro batch-poster batch submission.
func TestNitroBatchSubmission(t *testing.T) {
	cfg := loadNitroTestConfig(t)
	client := createMTLSHTTPClient(t, cfg)

	// Simulate batch data (in production, this is compressed L2 batch data)
	batchData := "0x00" + strings.Repeat("deadbeef", 8) // 33 bytes

	txParams := map[string]interface{}{
		"from":                 cfg.SignerAddress,
		"to":                   "0x1c479675ad559DC151F6Ec7ed3FbF8ceE79582B6", // Example SequencerInbox
		"gas":                  "0x30d40",                                    // 200000
		"maxFeePerGas":         "0xb2d05e00",                                 // 3 Gwei
		"maxPriorityFeePerGas": "0x3b9aca00",                                 // 1 Gwei
		"value":                "0x0",
		"nonce":                "0x1",
		"data":                 batchData,
		"chainId":              cfg.ChainID,
	}

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_signTransaction",
		Params:  []interface{}{txParams},
		ID:      1,
	}

	t.Logf("Simulating Nitro batch-poster batch submission via mTLS:")
	t.Logf("  from:     %s", cfg.SignerAddress)
	t.Logf("  to:       %s (SequencerInbox)", txParams["to"])
	t.Logf("  data len: %d bytes", len(batchData)/2-1)

	resp, err := sendMTLSRPCRequest(client, cfg.MTLSURL, req)
	if err != nil {
		t.Fatalf("RPC request failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("RPC error: code=%d, message=%s", resp.Error.Code, resp.Error.Message)
	}

	var signedTx string
	if err := json.Unmarshal(resp.Result, &signedTx); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	t.Logf("✓ Batch submission signing succeeded via mTLS")
	t.Logf("  Signed TX: %s...%s", signedTx[:10], signedTx[len(signedTx)-8:])
}

// TestNitroMTLSRequired tests that requests without mTLS are rejected.
func TestNitroMTLSRequired(t *testing.T) {
	cfg := loadNitroTestConfig(t)

	// Create a client WITHOUT client certificates
	caCert, err := os.ReadFile(cfg.CACertPath)
	if err != nil {
		t.Fatalf("Failed to read CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	noClientCertClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    caCertPool,
				MinVersion: tls.VersionTLS12,
			},
		},
		Timeout: 10 * time.Second,
	}

	// Try to access the RPC endpoint without client cert
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_accounts",
		Params:  []interface{}{},
		ID:      1,
	}

	_, err = sendMTLSRPCRequest(noClientCertClient, cfg.MTLSURL, req)
	if err == nil {
		t.Error("Expected request without client certificate to fail")
	} else {
		t.Logf("✓ Request without client certificate correctly rejected: %v", err)
	}
}

// TestNitroInvalidCertificate tests that requests with invalid certificates are rejected.
func TestNitroInvalidCertificate(t *testing.T) {
	_ = loadNitroTestConfig(t)

	// This test would require generating an invalid certificate
	// For now, we just verify that the test infrastructure works
	t.Log("✓ Invalid certificate test (requires separate invalid cert generation)")
	t.Skip("Requires invalid certificate for testing")
}

// TestNitroErrorCases tests error handling via mTLS.
func TestNitroErrorCases(t *testing.T) {
	cfg := loadNitroTestConfig(t)
	client := createMTLSHTTPClient(t, cfg)

	testCases := []struct {
		name        string
		method      string
		params      interface{}
		wantErrCode int
	}{
		{
			name:        "Unknown method",
			method:      "eth_unknownMethod",
			params:      []interface{}{},
			wantErrCode: -32601, // Method not found
		},
		{
			name:        "Invalid params",
			method:      "eth_signTransaction",
			params:      "not an array",
			wantErrCode: -32602, // Invalid params
		},
		{
			name:   "Wrong from address",
			method: "eth_signTransaction",
			params: []interface{}{map[string]interface{}{
				"from":     "0x0000000000000000000000000000000000000001", // Not our address
				"to":       "0x0000000000000000000000000000000000000000",
				"gas":      "0x5208",
				"gasPrice": "0x3b9aca00",
				"value":    "0x0",
				"nonce":    "0x0",
				"data":     "0x",
				"chainId":  cfg.ChainID,
			}},
			wantErrCode: -32603, // Internal error (key not found)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  tc.method,
				Params:  tc.params,
				ID:      1,
			}

			resp, err := sendMTLSRPCRequest(client, cfg.MTLSURL, req)
			if err != nil {
				t.Logf("✓ %s: request failed as expected: %v", tc.name, err)
				return
			}

			if resp.Error == nil {
				t.Errorf("Expected error, got success")
				return
			}

			if resp.Error.Code != tc.wantErrCode {
				t.Logf("Note: Expected error code %d, got %d (%s)",
					tc.wantErrCode, resp.Error.Code, resp.Error.Message)
			}

			t.Logf("✓ %s: error code=%d, message=%s", tc.name, resp.Error.Code, resp.Error.Message)
		})
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// BenchmarkNitroEthSignTransaction benchmarks the signing latency via mTLS.
func BenchmarkNitroEthSignTransaction(b *testing.B) {
	mtlsURL := os.Getenv(envMTLSURL)
	caCertPath := os.Getenv(envCACert)
	clientCertPath := os.Getenv(envClientCert)
	clientKeyPath := os.Getenv(envClientKey)
	signerAddress := os.Getenv(envNitroAddress)
	chainID := os.Getenv(envNitroChainID)

	if mtlsURL == "" || caCertPath == "" || clientCertPath == "" || clientKeyPath == "" || signerAddress == "" {
		b.Skip("Nitro integration test environment not configured")
	}

	if chainID == "" {
		chainID = "0xa4b1"
	}

	// Load certificates
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		b.Fatalf("Failed to read CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		b.Fatalf("Failed to load client certificate: %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      caCertPool,
				Certificates: []tls.Certificate{clientCert},
				MinVersion:   tls.VersionTLS12,
			},
		},
		Timeout: 30 * time.Second,
	}

	txParams := map[string]interface{}{
		"from":     signerAddress,
		"to":       "0x0000000000000000000000000000000000000000",
		"gas":      "0x5208",
		"gasPrice": "0x3b9aca00",
		"value":    "0x0",
		"nonce":    "0x0",
		"data":     "0x",
		"chainId":  chainID,
	}

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_signTransaction",
		Params:  []interface{}{txParams},
		ID:      1,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resp, err := sendMTLSRPCRequest(client, mtlsURL, req)
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
		if resp.Error != nil {
			b.Fatalf("RPC error: %s", resp.Error.Message)
		}
	}
}
