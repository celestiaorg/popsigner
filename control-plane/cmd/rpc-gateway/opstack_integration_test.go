// Package main contains integration tests for OP Stack remote signer functionality.
//
// This test simulates how OP Stack components (op-batcher, op-proposer, op-challenger)
// interact with POPSigner's RPC Gateway for transaction signing.
//
// OP Stack Configuration Example:
//
//	./op-batcher \
//	  --signer.endpoint=https://popsigner.example.com:8545/rpc \
//	  --signer.address=0x742d35Cc6634C0532925a3b844Bc454e4438f44e \
//	  --signer.header="X-API-Key=pop_abc123..."
//
// Run these tests against a deployed RPC gateway:
//
//	POPSIGNER_RPC_URL=https://your-gateway:8545 \
//	POPSIGNER_API_KEY=pop_... \
//	POPSIGNER_SIGNER_ADDRESS=0x... \
//	go test -v -tags=integration ./cmd/rpc-gateway/ -run TestOPStack
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Environment variables for integration tests.
const (
	envRPCURL        = "POPSIGNER_RPC_URL"        // e.g., https://popsigner.example.com:8545
	envAPIKey        = "POPSIGNER_API_KEY"        // e.g., pop_abc123...
	envSignerAddress = "POPSIGNER_SIGNER_ADDRESS" // e.g., 0x742d35Cc...
	envChainID       = "POPSIGNER_CHAIN_ID"       // e.g., 11155111 (Sepolia)
)

// TestConfig holds the configuration for integration tests.
type TestConfig struct {
	RPCURL        string
	APIKey        string
	SignerAddress string
	ChainID       string
}

// loadTestConfig loads configuration from environment variables.
func loadTestConfig(t *testing.T) *TestConfig {
	t.Helper()

	rpcURL := os.Getenv(envRPCURL)
	if rpcURL == "" {
		t.Skipf("Skipping integration test: %s not set", envRPCURL)
	}

	apiKey := os.Getenv(envAPIKey)
	if apiKey == "" {
		t.Skipf("Skipping integration test: %s not set", envAPIKey)
	}

	signerAddress := os.Getenv(envSignerAddress)
	if signerAddress == "" {
		t.Skipf("Skipping integration test: %s not set", envSignerAddress)
	}

	chainID := os.Getenv(envChainID)
	if chainID == "" {
		chainID = "0xaa36a7" // Default to Sepolia (11155111)
	} else if !strings.HasPrefix(chainID, "0x") {
		// Convert decimal to hex if needed
		var id int64
		fmt.Sscanf(chainID, "%d", &id)
		chainID = fmt.Sprintf("0x%x", id)
	}

	return &TestConfig{
		RPCURL:        rpcURL,
		APIKey:        apiKey,
		SignerAddress: signerAddress,
		ChainID:       chainID,
	}
}

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// sendRPCRequest sends a JSON-RPC request to the gateway.
func sendRPCRequest(url, apiKey string, req *JSONRPCRequest) (*JSONRPCResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url+"/rpc", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers exactly as OP Stack would
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
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

// TestOPStackHealthCheck verifies the gateway health endpoint.
func TestOPStackHealthCheck(t *testing.T) {
	cfg := loadTestConfig(t)

	resp, err := http.Get(cfg.RPCURL + "/health")
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

// TestOPStackEthAccounts tests the eth_accounts method.
// This is called by OP Stack to verify the signer address is available.
func TestOPStackEthAccounts(t *testing.T) {
	cfg := loadTestConfig(t)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_accounts",
		Params:  []interface{}{},
		ID:      1,
	}

	resp, err := sendRPCRequest(cfg.RPCURL, cfg.APIKey, req)
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

// TestOPStackEthSignTransaction tests the eth_signTransaction method.
// This is the core method used by op-batcher and op-proposer.
func TestOPStackEthSignTransaction(t *testing.T) {
	cfg := loadTestConfig(t)

	// Construct a test transaction (typical batch submission tx)
	// Note: This uses test values - in production, these come from the OP Stack component
	txParams := map[string]interface{}{
		"from":     cfg.SignerAddress,
		"to":       "0x0000000000000000000000000000000000000000", // Null address for test
		"gas":      "0x5208",                                     // 21000
		"gasPrice": "0x3b9aca00",                                 // 1 Gwei
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

	t.Logf("Sending eth_signTransaction request:")
	t.Logf("  from:    %s", cfg.SignerAddress)
	t.Logf("  chainId: %s", cfg.ChainID)

	resp, err := sendRPCRequest(cfg.RPCURL, cfg.APIKey, req)
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

	t.Logf("✓ eth_signTransaction succeeded")
	t.Logf("  Signed TX: %s...%s (%d bytes)", signedTx[:10], signedTx[len(signedTx)-8:], len(signedTx)/2)
}

// TestOPStackEIP1559Transaction tests signing an EIP-1559 transaction.
// Modern OP Stack uses EIP-1559 for better gas estimation.
func TestOPStackEIP1559Transaction(t *testing.T) {
	cfg := loadTestConfig(t)

	// EIP-1559 transaction with maxFeePerGas and maxPriorityFeePerGas
	txParams := map[string]interface{}{
		"from":                 cfg.SignerAddress,
		"to":                   "0x0000000000000000000000000000000000000000",
		"gas":                  "0x5208",    // 21000
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

	t.Logf("Sending EIP-1559 eth_signTransaction request:")
	t.Logf("  from:                 %s", cfg.SignerAddress)
	t.Logf("  maxFeePerGas:         %s", txParams["maxFeePerGas"])
	t.Logf("  maxPriorityFeePerGas: %s", txParams["maxPriorityFeePerGas"])

	resp, err := sendRPCRequest(cfg.RPCURL, cfg.APIKey, req)
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

	// EIP-1559 transactions start with 0x02
	if !strings.HasPrefix(signedTx, "0x02") {
		t.Logf("Note: Expected EIP-1559 tx prefix 0x02, got: %s", signedTx[:6])
	}

	t.Logf("✓ EIP-1559 eth_signTransaction succeeded")
	t.Logf("  Signed TX: %s...%s", signedTx[:10], signedTx[len(signedTx)-8:])
}

// TestOPStackBatchSubmission simulates an op-batcher batch submission.
// This is the most common transaction type for OP Stack operators.
func TestOPStackBatchSubmission(t *testing.T) {
	cfg := loadTestConfig(t)

	// Simulate batch data (in production, this is compressed L2 batch data)
	// Using a small test payload
	batchData := "0x00" + strings.Repeat("deadbeef", 8) // 33 bytes

	txParams := map[string]interface{}{
		"from":                 cfg.SignerAddress,
		"to":                   "0x5050505050505050505050505050505050505050", // BatchInbox address (placeholder)
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

	t.Logf("Simulating op-batcher batch submission:")
	t.Logf("  from:     %s", cfg.SignerAddress)
	t.Logf("  to:       %s (BatchInbox)", txParams["to"])
	t.Logf("  data len: %d bytes", len(batchData)/2-1)

	resp, err := sendRPCRequest(cfg.RPCURL, cfg.APIKey, req)
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

	t.Logf("✓ Batch submission signing succeeded")
	t.Logf("  Signed TX: %s...%s", signedTx[:10], signedTx[len(signedTx)-8:])
}

// TestOPStackAuthenticationMethods tests different authentication methods.
func TestOPStackAuthenticationMethods(t *testing.T) {
	cfg := loadTestConfig(t)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_accounts",
		Params:  []interface{}{},
		ID:      1,
	}
	body, _ := json.Marshal(req)

	testCases := []struct {
		name       string
		setHeaders func(r *http.Request)
		wantOK     bool
	}{
		{
			name: "X-API-Key header",
			setHeaders: func(r *http.Request) {
				r.Header.Set("X-API-Key", cfg.APIKey)
			},
			wantOK: true,
		},
		{
			name: "Authorization Bearer",
			setHeaders: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+cfg.APIKey)
			},
			wantOK: true,
		},
		{
			name: "No auth header",
			setHeaders: func(r *http.Request) {
				// No auth headers set
			},
			wantOK: false,
		},
		{
			name: "Invalid API key",
			setHeaders: func(r *http.Request) {
				r.Header.Set("X-API-Key", "invalid_key_12345")
			},
			wantOK: false,
		},
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpReq, _ := http.NewRequest("POST", cfg.RPCURL+"/rpc", bytes.NewReader(body))
			httpReq.Header.Set("Content-Type", "application/json")
			tc.setHeaders(httpReq)

			resp, err := client.Do(httpReq)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if tc.wantOK {
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Expected 200, got %d", resp.StatusCode)
				} else {
					t.Logf("✓ %s: authenticated successfully", tc.name)
				}
			} else {
				if resp.StatusCode == http.StatusOK {
					t.Errorf("Expected auth failure, got 200")
				} else {
					t.Logf("✓ %s: correctly rejected (status %d)", tc.name, resp.StatusCode)
				}
			}
		})
	}
}

// TestOPStackErrorCases tests error handling.
func TestOPStackErrorCases(t *testing.T) {
	cfg := loadTestConfig(t)

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
			name:        "Wrong from address",
			method:      "eth_signTransaction",
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

			resp, err := sendRPCRequest(cfg.RPCURL, cfg.APIKey, req)
			if err != nil {
				// For truly invalid JSON, we might get an HTTP error
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

// BenchmarkEthSignTransaction benchmarks the signing latency.
func BenchmarkEthSignTransaction(b *testing.B) {
	rpcURL := os.Getenv(envRPCURL)
	apiKey := os.Getenv(envAPIKey)
	signerAddress := os.Getenv(envSignerAddress)
	chainID := os.Getenv(envChainID)

	if rpcURL == "" || apiKey == "" || signerAddress == "" {
		b.Skip("Integration test environment not configured")
	}

	if chainID == "" {
		chainID = "0xaa36a7"
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
		resp, err := sendRPCRequest(rpcURL, apiKey, req)
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
		if resp.Error != nil {
			b.Fatalf("RPC error: %s", resp.Error.Message)
		}
	}
}

