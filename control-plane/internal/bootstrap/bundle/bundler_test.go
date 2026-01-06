package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// extractTarGz extracts a .tar.gz and returns a map of filename -> content.
func extractTarGz(data []byte) (map[string][]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	files := make(map[string][]byte)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[hdr.Name] = content
	}

	return files, nil
}

// getFileMode extracts file mode from tar.gz for a specific file.
func getFileMode(data []byte, filename string) (int64, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}

		if hdr.Name == filename || strings.HasSuffix(hdr.Name, "/"+filename) {
			return hdr.Mode, nil
		}
	}

	return 0, nil
}

func TestCreateOPStackBundle(t *testing.T) {
	bundler := NewBundler(nil) // nil repo since we use CreateBundleFromConfig

	cfg := &BundleConfig{
		Stack:             StackOPStack,
		ChainID:           42069,
		ChainName:         "my-test-chain",
		DAType:            "celestia",
		POPSignerEndpoint: "https://rpc.popsigner.com",
		BatcherAddress:    "0x1111111111111111111111111111111111111111",
		ProposerAddress:   "0x2222222222222222222222222222222222222222",
		Contracts: map[string]string{
			"l2_output_oracle": "0x3333333333333333333333333333333333333333",
		},
		Artifacts: map[string][]byte{
			"genesis":       []byte(`{"config": {"chainId": 42069}}`),
			"rollup_config": []byte(`{"genesis": {"l1": {}}}`),
			"addresses":     []byte(`{"l2_output_oracle": "0x3333"}`),
			"deploy_config": []byte(`{"l2ChainId": 42069}`),
		},
	}

	result, err := bundler.CreateBundleFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateBundleFromConfig failed: %v", err)
	}

	// Verify result structure
	if result.Data == nil || len(result.Data) == 0 {
		t.Fatal("Bundle data is empty")
	}
	if result.Filename != "my-test-chain-opstack-artifacts.tar.gz" {
		t.Errorf("Unexpected filename: %s", result.Filename)
	}
	if result.Checksum == "" {
		t.Error("Checksum is empty")
	}
	if result.SizeBytes == 0 {
		t.Error("SizeBytes is 0")
	}

	// Extract and verify contents
	files, err := extractTarGz(result.Data)
	if err != nil {
		t.Fatalf("Failed to extract bundle: %v", err)
	}

	// Check required files exist
	requiredFiles := []string{
		"docker-compose.yml",
		".env.example",
		"config/rollup.json",
		"config/addresses.json",
		"genesis/genesis.json",
		"secrets/jwt.txt",
		"scripts/start.sh",
		"scripts/healthcheck.sh",
		"README.md",
		"manifest.json",
	}

	baseDir := "my-test-chain-opstack-artifacts"
	for _, file := range requiredFiles {
		fullPath := baseDir + "/" + file
		if _, ok := files[fullPath]; !ok {
			t.Errorf("Missing required file: %s", file)
		}
	}

	// Verify docker-compose.yml contains OP Stack services
	composeContent := string(files[baseDir+"/docker-compose.yml"])
	if !strings.Contains(composeContent, "op-node:") {
		t.Error("docker-compose.yml missing op-node service")
	}
	if !strings.Contains(composeContent, "op-geth:") {
		t.Error("docker-compose.yml missing op-geth service")
	}
	if !strings.Contains(composeContent, "op-batcher:") {
		t.Error("docker-compose.yml missing op-batcher service")
	}

	// Verify manifest
	manifestContent := files[baseDir+"/manifest.json"]
	var manifest BundleManifest
	if err := json.Unmarshal(manifestContent, &manifest); err != nil {
		t.Fatalf("Failed to unmarshal manifest: %v", err)
	}
	if manifest.Stack != StackOPStack {
		t.Errorf("Manifest stack = %s, want opstack", manifest.Stack)
	}
	if manifest.ChainID != 42069 {
		t.Errorf("Manifest chainID = %d, want 42069", manifest.ChainID)
	}
	if !manifest.POPSignerInfo.APIKeyConfigured {
		t.Error("Manifest should indicate API key is configured")
	}

	// Verify JWT was generated
	jwtContent := files[baseDir+"/secrets/jwt.txt"]
	if len(jwtContent) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("JWT length = %d, want 64", len(jwtContent))
	}
}

func TestCreateNitroBundle(t *testing.T) {
	bundler := NewBundler(nil)

	cfg := &BundleConfig{
		Stack:                 StackNitro,
		ChainID:               42170,
		ChainName:             "my-orbit-chain",
		DAType:                "celestia",
		POPSignerMTLSEndpoint: "https://rpc-mtls.popsigner.com",
		ValidatorAddress:      "0x4444444444444444444444444444444444444444",
		Artifacts: map[string][]byte{
			"chain_info":     []byte(`[{"chain-id": 42170}]`),
			"node_config":    []byte(`{"parent-chain": {}}`),
			"core_contracts": []byte(`{"rollup": "0x5555"}`),
		},
		ClientCert: []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"),
		ClientKey:  []byte("-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----"),
		CACert:     []byte("-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----"),
	}

	result, err := bundler.CreateBundleFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateBundleFromConfig failed: %v", err)
	}

	// Verify filename
	if result.Filename != "my-orbit-chain-nitro-artifacts.tar.gz" {
		t.Errorf("Unexpected filename: %s", result.Filename)
	}

	// Extract and verify contents
	files, err := extractTarGz(result.Data)
	if err != nil {
		t.Fatalf("Failed to extract bundle: %v", err)
	}

	// Check required files exist
	requiredFiles := []string{
		"docker-compose.yml",
		".env.example",
		"config/chain-info.json",
		"config/node-config.json",
		"config/core-contracts.json",
		"certs/client.crt",
		"certs/client.key",
		"certs/ca.crt",
		"scripts/start.sh",
		"scripts/healthcheck.sh",
		"README.md",
		"manifest.json",
	}

	baseDir := "my-orbit-chain-nitro-artifacts"
	for _, file := range requiredFiles {
		fullPath := baseDir + "/" + file
		if _, ok := files[fullPath]; !ok {
			t.Errorf("Missing required file: %s", file)
		}
	}

	// Verify docker-compose.yml contains Nitro services
	composeContent := string(files[baseDir+"/docker-compose.yml"])
	if !strings.Contains(composeContent, "nitro:") {
		t.Error("docker-compose.yml missing nitro service")
	}
	if !strings.Contains(composeContent, "batch-poster:") {
		t.Error("docker-compose.yml missing batch-poster service")
	}
	if !strings.Contains(composeContent, "validator:") {
		t.Error("docker-compose.yml missing validator service")
	}

	// Verify mTLS config is in compose
	if !strings.Contains(composeContent, "./certs:/certs:ro") {
		t.Error("docker-compose.yml missing certs volume mount")
	}
	if !strings.Contains(composeContent, "external-signer.client-cert") {
		t.Error("docker-compose.yml missing mTLS config")
	}

	// Verify manifest
	manifestContent := files[baseDir+"/manifest.json"]
	var manifest BundleManifest
	if err := json.Unmarshal(manifestContent, &manifest); err != nil {
		t.Fatalf("Failed to unmarshal manifest: %v", err)
	}
	if manifest.Stack != StackNitro {
		t.Errorf("Manifest stack = %s, want nitro", manifest.Stack)
	}
	if !manifest.POPSignerInfo.CertificateIncluded {
		t.Error("Manifest should indicate certificates are included")
	}
}

func TestNitroBundleWithoutCerts(t *testing.T) {
	bundler := NewBundler(nil)

	cfg := &BundleConfig{
		Stack:                 StackNitro,
		ChainID:               42170,
		ChainName:             "no-certs-chain",
		POPSignerMTLSEndpoint: "https://rpc-mtls.popsigner.com",
		Artifacts: map[string][]byte{
			"chain_info":  []byte(`[{"chain-id": 42170}]`),
			"node_config": []byte(`{"parent-chain": {}}`),
		},
		// No certificates
	}

	result, err := bundler.CreateBundleFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateBundleFromConfig failed: %v", err)
	}

	// Extract contents
	files, err := extractTarGz(result.Data)
	if err != nil {
		t.Fatalf("Failed to extract bundle: %v", err)
	}

	baseDir := "no-certs-chain-nitro-artifacts"

	// Should have .gitkeep placeholder instead of certs
	if _, ok := files[baseDir+"/certs/.gitkeep"]; !ok {
		t.Error("Missing certs/.gitkeep placeholder")
	}

	// README should mention missing certs
	readmeContent := string(files[baseDir+"/README.md"])
	if !strings.Contains(readmeContent, "mTLS certificates are not included") {
		t.Error("README should mention missing certificates")
	}

	// Manifest should indicate certs not included
	var manifest BundleManifest
	if err := json.Unmarshal(files[baseDir+"/manifest.json"], &manifest); err != nil {
		t.Fatalf("Failed to unmarshal manifest: %v", err)
	}
	if manifest.POPSignerInfo.CertificateIncluded {
		t.Error("Manifest should indicate certificates are NOT included")
	}
}

func TestClientKeyPermissions(t *testing.T) {
	bundler := NewBundler(nil)

	cfg := &BundleConfig{
		Stack:                 StackNitro,
		ChainID:               42170,
		ChainName:             "perms-test",
		POPSignerMTLSEndpoint: "https://rpc-mtls.popsigner.com",
		Artifacts:             map[string][]byte{},
		ClientCert:            []byte("cert"),
		ClientKey:             []byte("key"),
	}

	result, err := bundler.CreateBundleFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateBundleFromConfig failed: %v", err)
	}

	// Check client.key has 0600 permissions
	mode, err := getFileMode(result.Data, "certs/client.key")
	if err != nil {
		t.Fatalf("Failed to get file mode: %v", err)
	}

	if mode != 0600 {
		t.Errorf("client.key mode = %o, want 0600", mode)
	}
}

func TestJWTPermissions(t *testing.T) {
	bundler := NewBundler(nil)

	cfg := &BundleConfig{
		Stack:             StackOPStack,
		ChainID:           42069,
		ChainName:         "jwt-test",
		POPSignerEndpoint: "https://rpc.popsigner.com",
		Artifacts:         map[string][]byte{},
	}

	result, err := bundler.CreateBundleFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateBundleFromConfig failed: %v", err)
	}

	// Check jwt.txt has 0600 permissions
	mode, err := getFileMode(result.Data, "secrets/jwt.txt")
	if err != nil {
		t.Fatalf("Failed to get file mode: %v", err)
	}

	if mode != 0600 {
		t.Errorf("jwt.txt mode = %o, want 0600", mode)
	}
}

func TestScriptPermissions(t *testing.T) {
	bundler := NewBundler(nil)

	cfg := &BundleConfig{
		Stack:             StackOPStack,
		ChainID:           42069,
		ChainName:         "script-test",
		POPSignerEndpoint: "https://rpc.popsigner.com",
		Artifacts:         map[string][]byte{},
	}

	result, err := bundler.CreateBundleFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateBundleFromConfig failed: %v", err)
	}

	// Check start.sh has executable permissions
	mode, err := getFileMode(result.Data, "scripts/start.sh")
	if err != nil {
		t.Fatalf("Failed to get file mode: %v", err)
	}

	if mode != 0755 {
		t.Errorf("start.sh mode = %o, want 0755", mode)
	}

	// Check healthcheck.sh has executable permissions
	mode, err = getFileMode(result.Data, "scripts/healthcheck.sh")
	if err != nil {
		t.Fatalf("Failed to get file mode: %v", err)
	}

	if mode != 0755 {
		t.Errorf("healthcheck.sh mode = %o, want 0755", mode)
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-chain", "my-chain"},
		{"My Chain", "my-chain"},
		{"My_Chain_123", "my_chain_123"},
		{"UPPERCASE", "uppercase"},
		{"special!@#$chars", "specialchars"},
		{"", "rollup"},
		{"   ", "rollup"},
		{"chain-name-with-dashes", "chain-name-with-dashes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBundleChecksums(t *testing.T) {
	bundler := NewBundler(nil)

	cfg := &BundleConfig{
		Stack:             StackOPStack,
		ChainID:           42069,
		ChainName:         "checksum-test",
		POPSignerEndpoint: "https://rpc.popsigner.com",
		Artifacts: map[string][]byte{
			"genesis": []byte(`{"test": true}`),
		},
	}

	result, err := bundler.CreateBundleFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateBundleFromConfig failed: %v", err)
	}

	// Verify manifest has checksums
	if result.Manifest.Checksums == nil {
		t.Error("Manifest should have checksums")
	}

	// Verify checksums are non-empty
	for path, checksum := range result.Manifest.Checksums {
		if checksum == "" {
			t.Errorf("Empty checksum for %s", path)
		}
		if len(checksum) != 64 { // SHA256 = 64 hex chars
			t.Errorf("Checksum for %s has wrong length: %d", path, len(checksum))
		}
	}

	// Verify bundle-level checksum
	if result.Checksum == "" {
		t.Error("Bundle checksum is empty")
	}
	if len(result.Checksum) != 64 {
		t.Errorf("Bundle checksum has wrong length: %d", len(result.Checksum))
	}
}

func TestCelestiaDAIncluded(t *testing.T) {
	bundler := NewBundler(nil)

	// OP Stack with Celestia
	cfg := &BundleConfig{
		Stack:             StackOPStack,
		ChainID:           42069,
		ChainName:         "celestia-test",
		DAType:            "celestia",
		POPSignerEndpoint: "https://rpc.popsigner.com",
		Artifacts:         map[string][]byte{},
	}

	result, err := bundler.CreateBundleFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateBundleFromConfig failed: %v", err)
	}

	files, err := extractTarGz(result.Data)
	if err != nil {
		t.Fatalf("Failed to extract bundle: %v", err)
	}

	baseDir := "celestia-test-opstack-artifacts"
	composeContent := string(files[baseDir+"/docker-compose.yml"])

	// Should include op-alt-da service
	if !strings.Contains(composeContent, "op-alt-da:") {
		t.Error("docker-compose.yml should include op-alt-da service for Celestia")
	}

	// Should include altda config
	if !strings.Contains(composeContent, "altda.enabled=true") {
		t.Error("docker-compose.yml should include altda configuration")
	}

	// .env should have Celestia variables
	envContent := string(files[baseDir+"/.env.example"])
	if !strings.Contains(envContent, "CELESTIA_RPC_URL") {
		t.Error(".env.example should include CELESTIA_RPC_URL")
	}
}

func TestNitroCelestiaDAIncluded(t *testing.T) {
	bundler := NewBundler(nil)

	cfg := &BundleConfig{
		Stack:                 StackNitro,
		ChainID:               42170,
		ChainName:             "celestia-nitro",
		DAType:                "celestia",
		POPSignerMTLSEndpoint: "https://rpc-mtls.popsigner.com",
		Artifacts:             map[string][]byte{},
	}

	result, err := bundler.CreateBundleFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateBundleFromConfig failed: %v", err)
	}

	files, err := extractTarGz(result.Data)
	if err != nil {
		t.Fatalf("Failed to extract bundle: %v", err)
	}

	baseDir := "celestia-nitro-nitro-artifacts"
	composeContent := string(files[baseDir+"/docker-compose.yml"])

	// Should include celestia-server service
	if !strings.Contains(composeContent, "celestia-server:") {
		t.Error("docker-compose.yml should include celestia-server for Celestia")
	}
}

func TestEmptyArtifacts(t *testing.T) {
	bundler := NewBundler(nil)

	// Should work even with no artifacts
	cfg := &BundleConfig{
		Stack:             StackOPStack,
		ChainID:           42069,
		ChainName:         "empty-test",
		POPSignerEndpoint: "https://rpc.popsigner.com",
		Artifacts:         map[string][]byte{},
	}

	result, err := bundler.CreateBundleFromConfig(cfg)
	if err != nil {
		t.Fatalf("CreateBundleFromConfig failed: %v", err)
	}

	// Should still create a valid bundle
	if result.Data == nil {
		t.Error("Bundle data should not be nil")
	}

	files, err := extractTarGz(result.Data)
	if err != nil {
		t.Fatalf("Failed to extract bundle: %v", err)
	}

	// Should have docker-compose.yml even without artifacts
	baseDir := "empty-test-opstack-artifacts"
	if _, ok := files[baseDir+"/docker-compose.yml"]; !ok {
		t.Error("Should have docker-compose.yml even with empty artifacts")
	}
}

func TestInvalidStack(t *testing.T) {
	bundler := NewBundler(nil)

	cfg := &BundleConfig{
		Stack:     Stack("invalid"),
		ChainID:   42069,
		ChainName: "test",
		Artifacts: map[string][]byte{},
	}

	_, err := bundler.CreateBundleFromConfig(cfg)
	if err == nil {
		t.Error("Should return error for invalid stack")
	}
	// The error comes from compose generator which tries to find a template
	if err == nil || (!strings.Contains(err.Error(), "unsupported stack") && !strings.Contains(err.Error(), "file does not exist")) {
		t.Errorf("Error should mention unsupported stack or template not found: %v", err)
	}
}

