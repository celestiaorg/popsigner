package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/nitro"
	"github.com/ethereum/go-ethereum/common"
)

// =============================================================================
// waitForHTTP tests
// =============================================================================

func TestWaitForHTTP_Success(t *testing.T) {
	// Create a test server that responds immediately
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	result := waitForHTTP(ctx, server.URL, 5*time.Second)

	if !result {
		t.Error("expected waitForHTTP to return true for responsive server")
	}
}

func TestWaitForHTTP_Timeout(t *testing.T) {
	// Create a test server that never responds (closes connection)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow server by sleeping longer than timeout
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	ctx := context.Background()
	// Use very short timeout
	result := waitForHTTP(ctx, server.URL, 100*time.Millisecond)

	if result {
		t.Error("expected waitForHTTP to return false on timeout")
	}
}

func TestWaitForHTTP_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel context immediately
	cancel()

	result := waitForHTTP(ctx, server.URL, 10*time.Second)

	if result {
		t.Error("expected waitForHTTP to return false when context is cancelled")
	}
}

func TestWaitForHTTP_ServerError(t *testing.T) {
	// Server returns 500, should keep polling until timeout
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	result := waitForHTTP(ctx, server.URL, 5*time.Second)

	if !result {
		t.Error("expected waitForHTTP to return true after server recovers")
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 calls, got %d", callCount)
	}
}

// =============================================================================
// getKeyID tests
// =============================================================================

func TestGetKeyID_Success(t *testing.T) {
	expectedID := "test-key-uuid-12345"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/keys/0xTestAddress" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Address   string `json:"address"`
			PublicKey string `json:"public_key"`
		}{
			ID:        expectedID,
			Name:      "test-key",
			Address:   "0xTestAddress",
			PublicKey: "0x04...",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx := context.Background()
	keyID, err := getKeyID(ctx, server.URL, "0xTestAddress")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if keyID != expectedID {
		t.Errorf("expected keyID %q, got %q", expectedID, keyID)
	}
}

func TestGetKeyID_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("key not found"))
	}))
	defer server.Close()

	ctx := context.Background()
	_, err := getKeyID(ctx, server.URL, "0xUnknownAddress")

	if err == nil {
		t.Error("expected error for 404 response")
	}
}

func TestGetKeyID_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	ctx := context.Background()
	_, err := getKeyID(ctx, server.URL, "0xTestAddress")

	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

// =============================================================================
// NitroConfigWriter tests
// =============================================================================

func TestNitroConfigWriter_WriteAll(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "nitro-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mock deploy result
	result := &nitroDeployResult{
		contracts: &nitro.RollupContracts{
			Rollup:                common.HexToAddress("0x1111111111111111111111111111111111111111"),
			Inbox:                 common.HexToAddress("0x2222222222222222222222222222222222222222"),
			Outbox:                common.HexToAddress("0x3333333333333333333333333333333333333333"),
			Bridge:                common.HexToAddress("0x4444444444444444444444444444444444444444"),
			SequencerInbox:        common.HexToAddress("0x5555555555555555555555555555555555555555"),
			RollupEventInbox:      common.HexToAddress("0x6666666666666666666666666666666666666666"),
			ChallengeManager:      common.HexToAddress("0x7777777777777777777777777777777777777777"),
			AdminProxy:            common.HexToAddress("0x8888888888888888888888888888888888888888"),
			UpgradeExecutor:       common.HexToAddress("0x9999999999999999999999999999999999999999"),
			ValidatorWalletCreator: common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			NativeToken:           common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		},
		chainConfig: map[string]interface{}{
			"chainId": 42069,
		},
		deploymentBlock: 100,
		stakeToken:      common.HexToAddress("0xcccccccccccccccccccccccccccccccccccccccc"),
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	writer := &NitroConfigWriter{
		logger:        logger,
		bundleDir:     tmpDir,
		result:        result,
		celestiaKeyID: "test-celestia-key",
	}

	err = writer.WriteAll()
	if err != nil {
		t.Fatalf("WriteAll failed: %v", err)
	}

	// Verify expected files were created
	expectedFiles := []string{
		"config/chain-info.json",
		"config/celestia-config.toml",
		"config/addresses.json",
		"config/jwt.txt",
		"docker-compose.yml",
		".env",
		"scripts/start.sh",
		"scripts/stop.sh",
		"scripts/reset.sh",
		"scripts/test.sh",
		"README.md",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file not created: %s", file)
		}
	}
}

func TestNitroConfigWriter_ChainInfo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nitro-chaininfo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result := &nitroDeployResult{
		contracts: &nitro.RollupContracts{
			Rollup:                common.HexToAddress("0x1111111111111111111111111111111111111111"),
			Inbox:                 common.HexToAddress("0x2222222222222222222222222222222222222222"),
			Bridge:                common.HexToAddress("0x4444444444444444444444444444444444444444"),
			SequencerInbox:        common.HexToAddress("0x5555555555555555555555555555555555555555"),
			ValidatorWalletCreator: common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			NativeToken:           common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
			UpgradeExecutor:       common.HexToAddress("0x9999999999999999999999999999999999999999"),
		},
		chainConfig: map[string]interface{}{
			"chainId": 42069,
		},
		deploymentBlock: 100,
		stakeToken:      common.HexToAddress("0xcccccccccccccccccccccccccccccccccccccccc"),
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	writer := &NitroConfigWriter{
		logger:        logger,
		bundleDir:     tmpDir,
		result:        result,
		celestiaKeyID: "test-key",
	}

	// Create config directory
	os.MkdirAll(filepath.Join(tmpDir, "config"), 0755)

	err = writer.writeChainInfo()
	if err != nil {
		t.Fatalf("writeChainInfo failed: %v", err)
	}

	// Read and verify chain-info.json
	data, err := os.ReadFile(filepath.Join(tmpDir, "config", "chain-info.json"))
	if err != nil {
		t.Fatalf("failed to read chain-info.json: %v", err)
	}

	var chainInfo []map[string]interface{}
	if err := json.Unmarshal(data, &chainInfo); err != nil {
		t.Fatalf("failed to unmarshal chain-info.json: %v", err)
	}

	if len(chainInfo) != 1 {
		t.Fatalf("expected 1 chain info entry, got %d", len(chainInfo))
	}

	// Verify key fields
	entry := chainInfo[0]
	if entry["chain-id"] != float64(42069) {
		t.Errorf("expected chain-id 42069, got %v", entry["chain-id"])
	}
	if entry["parent-chain-id"] != float64(31337) {
		t.Errorf("expected parent-chain-id 31337, got %v", entry["parent-chain-id"])
	}
}

func TestNitroConfigWriter_JWT(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nitro-jwt-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result := &nitroDeployResult{
		contracts: &nitro.RollupContracts{},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	writer := &NitroConfigWriter{
		logger:        logger,
		bundleDir:     tmpDir,
		result:        result,
		celestiaKeyID: "test-key",
	}

	// Create config directory
	os.MkdirAll(filepath.Join(tmpDir, "config"), 0755)

	err = writer.writeJWT()
	if err != nil {
		t.Fatalf("writeJWT failed: %v", err)
	}

	// Read and verify jwt.txt
	data, err := os.ReadFile(filepath.Join(tmpDir, "config", "jwt.txt"))
	if err != nil {
		t.Fatalf("failed to read jwt.txt: %v", err)
	}

	// JWT should be 64 hex characters (32 bytes)
	if len(data) != 64 {
		t.Errorf("expected 64 character hex string, got %d characters", len(data))
	}

	// Verify it's valid hex
	for _, c := range data {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("invalid hex character in JWT: %c", c)
		}
	}
}

// =============================================================================
// bundleBuilder tests
// =============================================================================

func TestNewBundleBuilder(t *testing.T) {
	bundleDir := "/tmp/test-bundle"
	stackType := StackNitro

	builder := newBundleBuilder(bundleDir, stackType)

	if builder.bundleDir != bundleDir {
		t.Errorf("expected bundleDir %q, got %q", bundleDir, builder.bundleDir)
	}
	if builder.stackType != stackType {
		t.Errorf("expected stackType %q, got %q", stackType, builder.stackType)
	}
	if builder.ctx == nil {
		t.Error("expected context to be set")
	}
	if builder.cancel == nil {
		t.Error("expected cancel func to be set")
	}
	if builder.logger == nil {
		t.Error("expected logger to be set")
	}
}

func TestBundleBuilder_PrepareBundleDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bundle-prep-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	bundleDir := filepath.Join(tmpDir, "new-bundle")

	// Create pre-existing file to verify it gets removed
	os.MkdirAll(bundleDir, 0755)
	os.WriteFile(filepath.Join(bundleDir, "old-file.txt"), []byte("old content"), 0644)

	builder := newBundleBuilder(bundleDir, StackOPStack)

	err = builder.prepareBundleDirectory()
	if err != nil {
		t.Fatalf("prepareBundleDirectory failed: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(bundleDir)
	if err != nil {
		t.Fatalf("bundle directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected bundleDir to be a directory")
	}

	// Verify old file was removed
	if _, err := os.Stat(filepath.Join(bundleDir, "old-file.txt")); !os.IsNotExist(err) {
		t.Error("expected old file to be removed")
	}
}

func TestBundleBuilder_Run_UnknownStack(t *testing.T) {
	builder := newBundleBuilder("/tmp/test", StackType("unknown"))

	err := builder.run()

	if err == nil {
		t.Error("expected error for unknown stack type")
	}
}

// =============================================================================
// waitForReceipt tests
// =============================================================================

func TestWaitForReceipt_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Pass nil client - should fail on context check before using client
	_, err := waitForReceipt(ctx, nil, common.Hash{})

	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}
}

// =============================================================================
// StackType tests
// =============================================================================

func TestStackType_Constants(t *testing.T) {
	if StackOPStack != "opstack" {
		t.Errorf("expected StackOPStack to be 'opstack', got %q", StackOPStack)
	}
	if StackNitro != "nitro" {
		t.Errorf("expected StackNitro to be 'nitro', got %q", StackNitro)
	}
}

// =============================================================================
// createContractCreation tests
// =============================================================================

func TestCreateContractCreation(t *testing.T) {
	nonce := uint64(5)
	data := []byte{0x60, 0x60, 0x60, 0x40}
	gasLimit := uint64(1000000)
	gasPrice := big.NewInt(1000000000)

	tx := createContractCreation(nonce, data, gasLimit, gasPrice)

	if tx == nil {
		t.Fatal("expected transaction to be created")
	}
	if tx.Nonce() != nonce {
		t.Errorf("expected nonce %d, got %d", nonce, tx.Nonce())
	}
	if tx.Gas() != gasLimit {
		t.Errorf("expected gas %d, got %d", gasLimit, tx.Gas())
	}
	if tx.To() != nil {
		t.Error("expected To() to be nil for contract creation")
	}
}

// =============================================================================
// Integration-style tests (with mocks)
// =============================================================================

func TestNitroConfigWriter_Env(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nitro-env-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result := &nitroDeployResult{
		contracts: &nitro.RollupContracts{
			Rollup:                common.HexToAddress("0x1111111111111111111111111111111111111111"),
			Inbox:                 common.HexToAddress("0x2222222222222222222222222222222222222222"),
			Bridge:                common.HexToAddress("0x4444444444444444444444444444444444444444"),
			SequencerInbox:        common.HexToAddress("0x5555555555555555555555555555555555555555"),
			ValidatorWalletCreator: common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		},
		stakeToken: common.HexToAddress("0xcccccccccccccccccccccccccccccccccccccccc"),
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	writer := &NitroConfigWriter{
		logger:        logger,
		bundleDir:     tmpDir,
		result:        result,
		celestiaKeyID: "test-key",
	}

	err = writer.writeEnv()
	if err != nil {
		t.Fatalf("writeEnv failed: %v", err)
	}

	// Read and verify .env
	data, err := os.ReadFile(filepath.Join(tmpDir, ".env"))
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}

	content := string(data)

	// Verify key values are present
	expectedStrings := []string{
		"L1_CHAIN_ID=31337",
		"L2_CHAIN_ID=42069",
		"NITRO_IMAGE=rg.nl-ams.scw.cloud/banhbao/nitro-node-dev:v3.10.0",
		"NITRO_DAS_IMAGE=rg.nl-ams.scw.cloud/banhbao/nitro-das-server:v0.8.2",
		"ROLLUP_ADDRESS=0x1111111111111111111111111111111111111111",
	}

	for _, expected := range expectedStrings {
		if !contains(content, expected) {
			t.Errorf("expected .env to contain %q", expected)
		}
	}
}

func TestNitroConfigWriter_DockerCompose(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nitro-compose-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result := &nitroDeployResult{
		contracts: &nitro.RollupContracts{},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	writer := &NitroConfigWriter{
		logger:        logger,
		bundleDir:     tmpDir,
		result:        result,
		celestiaKeyID: "test-key",
	}

	err = writer.writeDockerCompose()
	if err != nil {
		t.Fatalf("writeDockerCompose failed: %v", err)
	}

	// Read and verify docker-compose.yml
	data, err := os.ReadFile(filepath.Join(tmpDir, "docker-compose.yml"))
	if err != nil {
		t.Fatalf("failed to read docker-compose.yml: %v", err)
	}

	content := string(data)

	// Verify key services are defined
	expectedStrings := []string{
		"anvil:",
		"platform: linux/amd64",
		"popsigner-lite:",
		"localestia:",
		"celestia-das-server:",
		"nitro-sequencer:",
		"popsigner-lite:v0.1.2",
		"localestia:v0.1.5",
	}

	for _, expected := range expectedStrings {
		if !contains(content, expected) {
			t.Errorf("expected docker-compose.yml to contain %q", expected)
		}
	}
}

func TestNitroConfigWriter_StartScript(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nitro-script-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result := &nitroDeployResult{
		contracts: &nitro.RollupContracts{},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	writer := &NitroConfigWriter{
		logger:        logger,
		bundleDir:     tmpDir,
		result:        result,
		celestiaKeyID: "test-key",
	}

	// Create scripts directory
	os.MkdirAll(filepath.Join(tmpDir, "scripts"), 0755)

	err = writer.writeStartScript()
	if err != nil {
		t.Fatalf("writeStartScript failed: %v", err)
	}

	// Read and verify start.sh
	data, err := os.ReadFile(filepath.Join(tmpDir, "scripts", "start.sh"))
	if err != nil {
		t.Fatalf("failed to read start.sh: %v", err)
	}

	content := string(data)

	// Verify restart detection logic is present
	expectedStrings := []string{
		"IS_RESTART=false",
		"docker volume inspect",
		"two-phase",
		"BATCH_POSTER_ENABLE",
	}

	for _, expected := range expectedStrings {
		if !contains(content, expected) {
			t.Errorf("expected start.sh to contain %q", expected)
		}
	}

	// Verify file is executable
	info, err := os.Stat(filepath.Join(tmpDir, "scripts", "start.sh"))
	if err != nil {
		t.Fatalf("failed to stat start.sh: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("expected start.sh to be executable")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
