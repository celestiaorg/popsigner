// Package nitro provides Nitro chain deployment infrastructure using pre-built contracts.
package nitro

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// ArtifactVersion is the current version of the nitro-contracts artifacts.
// Update this when uploading new artifacts to S3.
const ArtifactVersion = "v3.2.0-beta.0"

// ArtifactChecksums contains SHA256 hashes for each artifact version.
// CRIT-020: These checksums are verified after download to ensure artifact integrity.
// To calculate: sha256sum /path/to/downloaded/v3.2.0-beta.0.zip
// Update this map when uploading new artifacts to S3.
var ArtifactChecksums = map[string]string{
	// Calculated: curl -skL "https://nitro-contracts.s3.nl-ams.scw.cloud/v3.2.0-beta.0.zip" | sha256sum
	"v3.2.0-beta.0": "sha256:10f1c0eade0e1d9c51ddc9df04d96bca108f6794dc132139c3a1ae1024608b9c",
	// Add previous versions for rollback support:
	// "v3.1.0": "sha256:...",
}

// SkipChecksumVerification can be set to true in test environments to skip checksum verification.
// WARNING: Never set this to true in production!
var SkipChecksumVerification = false

// ArtifactBaseURL is the base URL for Nitro contract artifact storage.
const ArtifactBaseURL = "https://nitro-contracts.s3.nl-ams.scw.cloud"

// ContractArtifactURL is the URL to nitro-contracts v3.2.0-beta.0 artifacts.
// v3.2 includes CUSTOM_DA_MESSAGE_HEADER_FLAG (0x01) for External DA support.
var ContractArtifactURL = ArtifactBaseURL + "/" + ArtifactVersion + ".zip"

// ContractArtifact represents a compiled Solidity contract with ABI and bytecode.
type ContractArtifact struct {
	ABI              json.RawMessage `json:"abi"`
	Bytecode         Bytecode        `json:"bytecode"`
	DeployedBytecode Bytecode        `json:"deployedBytecode,omitempty"`
	ContractName     string          `json:"contractName,omitempty"`
}

// Bytecode contains the contract bytecode.
// It handles both formats:
// - Simple string: "0x608060..."
// - Object with "object" field: {"object": "0x608060..."}
type Bytecode struct {
	hex string
}

// UnmarshalJSON handles both string and object bytecode formats.
func (b *Bytecode) UnmarshalJSON(data []byte) error {
	// Try as plain string first (Foundry/Hardhat output format)
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		b.hex = s
		return nil
	}

	// Try as object with "object" field (older formats)
	var obj struct {
		Object string `json:"object"`
	}
	if err := json.Unmarshal(data, &obj); err == nil {
		b.hex = obj.Object
		return nil
	}

	return fmt.Errorf("bytecode must be a string or object with 'object' field")
}

// MarshalJSON marshals the bytecode as a string.
func (b Bytecode) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.hex)
}

// String returns the bytecode hex string.
func (b Bytecode) String() string {
	return b.hex
}

// GetBytecode returns the bytecode as a hex string (with 0x prefix).
func (a *ContractArtifact) GetBytecode() string {
	return a.Bytecode.hex
}

// NitroArtifacts contains all compiled Nitro contract artifacts needed for deployment.
type NitroArtifacts struct {
	// Core deployment contracts
	RollupCreator *ContractArtifact
	BridgeCreator *ContractArtifact

	// Rollup infrastructure (ETH native token)
	SequencerInbox     *ContractArtifact
	Bridge             *ContractArtifact
	Inbox              *ContractArtifact
	Outbox             *ContractArtifact
	RollupEventInbox   *ContractArtifact // NEW: for BridgeCreator templates
	RollupCore         *ContractArtifact
	RollupAdminLogic   *ContractArtifact
	RollupUserLogic    *ContractArtifact

	// ERC20 native token variants (for custom gas tokens)
	ERC20Bridge *ContractArtifact
	ERC20Inbox  *ContractArtifact

	// Challenge/Fraud proof contracts (BOLD protocol in v3.2+)
	// Note: EdgeChallengeManager replaces the old ChallengeManager in BOLD
	EdgeChallengeManager *ContractArtifact
	OneStepProofEntry    *ContractArtifact
	OneStepProver0       *ContractArtifact
	OneStepProverMemory  *ContractArtifact
	OneStepProverMath    *ContractArtifact
	OneStepProverHostIo  *ContractArtifact

	// Upgrade infrastructure
	UpgradeExecutor *ContractArtifact

	// Validator infrastructure
	ValidatorWalletCreator *ContractArtifact

	// L2 Factory deployer
	DeployHelper *ContractArtifact

	// EIP-4844 blob reader (for L1 chains, wraps BLOBBASEFEE/BLOBHASH opcodes)
	Reader4844 *ContractArtifact

	// Version metadata
	Version   string
	LoadedAt  time.Time
	SourceURL string
}

// ContractArtifactDownloader handles downloading and parsing Nitro contract artifacts from S3.
type ContractArtifactDownloader struct {
	cacheDir string
	mu       sync.Mutex
}

// NewContractArtifactDownloader creates a new artifact downloader.
func NewContractArtifactDownloader(cacheDir string) *ContractArtifactDownloader {
	if cacheDir == "" {
		cacheDir = os.TempDir()
	}
	return &ContractArtifactDownloader{
		cacheDir: cacheDir,
	}
}

// Download downloads and parses Nitro contract artifacts from S3.
// Returns a NitroArtifacts struct with all contracts loaded.
// The version parameter is used for checksum verification.
func (d *ContractArtifactDownloader) Download(ctx context.Context, url string, version string) (*NitroArtifacts, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Create cache directory if needed
	if err := os.MkdirAll(d.cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	// Download the zip file
	zipPath := filepath.Join(d.cacheDir, fmt.Sprintf("nitro-contracts-%d.zip", time.Now().UnixNano()))
	if err := d.downloadFile(ctx, url, zipPath); err != nil {
		return nil, fmt.Errorf("download artifacts: %w", err)
	}
	defer os.Remove(zipPath)

	// CRIT-020: Verify checksum BEFORE parsing to prevent use of tampered artifacts
	if err := d.verifyChecksum(zipPath, version); err != nil {
		return nil, fmt.Errorf("artifact integrity check failed: %w", err)
	}

	// Extract and parse artifacts
	artifacts, err := d.parseZip(zipPath)
	if err != nil {
		return nil, fmt.Errorf("parse artifacts: %w", err)
	}

	artifacts.Version = version
	artifacts.LoadedAt = time.Now()
	artifacts.SourceURL = url

	return artifacts, nil
}

// DownloadDefault downloads artifacts from the default URL and version.
func (d *ContractArtifactDownloader) DownloadDefault(ctx context.Context) (*NitroArtifacts, error) {
	return d.Download(ctx, ContractArtifactURL, ArtifactVersion)
}

// downloadFile downloads a file from URL to the given path.
func (d *ContractArtifactDownloader) downloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d from %s", resp.StatusCode, url)
	}

	// Write to temp file first, then rename for atomicity
	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	_, err = io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("write file: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// verifyChecksum calculates SHA256 of the downloaded file and compares with expected.
// CRIT-020: This prevents use of tampered artifacts if S3 storage is compromised.
func (d *ContractArtifactDownloader) verifyChecksum(filePath, version string) error {
	// Allow skipping in test environments (but never in production)
	if SkipChecksumVerification {
		return nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file for checksum verification: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("calculate checksum: %w", err)
	}

	actualHash := fmt.Sprintf("sha256:%x", h.Sum(nil))
	expectedHash, ok := ArtifactChecksums[version]
	if !ok {
		return fmt.Errorf("SECURITY: no checksum registered for artifact version %s - refusing to use unverified artifacts", version)
	}

	if actualHash != expectedHash {
		return fmt.Errorf("SECURITY: checksum mismatch for %s - expected %s, got %s - artifacts may be tampered", version, expectedHash, actualHash)
	}

	return nil
}

// parseZip extracts and parses contract artifacts from a zip file.
func (d *ContractArtifactDownloader) parseZip(zipPath string) (*NitroArtifacts, error) {
	// Open the zip file
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	// Map to store loaded artifacts by contract name
	loaded := make(map[string]*ContractArtifact)

	// List of contracts we need (BOLD protocol for v3.2+)
	requiredContracts := []string{
		// Core deployment
		"RollupCreator",
		"BridgeCreator",
		// Rollup infrastructure (ETH native)
		"SequencerInbox",
		"Bridge",
		"Inbox",
		"Outbox",
		"RollupEventInbox",
		"RollupCore",
		"RollupAdminLogic",
		"RollupUserLogic",
		// ERC20 native token variants
		"ERC20Bridge",
		"ERC20Inbox",
		// Challenge/Fraud proofs (BOLD)
		"EdgeChallengeManager",
		"OneStepProofEntry",
		"OneStepProver0",
		"OneStepProverMemory",
		"OneStepProverMath",
		"OneStepProverHostIo",
		// Upgrade/Validator infrastructure
		"UpgradeExecutor",
		"ValidatorWalletCreator",
		"DeployHelper",
		// EIP-4844 blob reader
		"Reader4844",
	}

	// Build a set for quick lookup
	requiredSet := make(map[string]bool)
	for _, name := range requiredContracts {
		requiredSet[name] = true
	}

	// Extract and parse each JSON file
	for _, f := range r.File {
		// Skip directories
		if f.FileInfo().IsDir() {
			continue
		}

		// Only process JSON files
		if filepath.Ext(f.Name) != ".json" {
			continue
		}

		// Extract contract name from filename (e.g., "RollupCreator.json" -> "RollupCreator")
		baseName := filepath.Base(f.Name)
		contractName := baseName[:len(baseName)-5] // Remove .json

		// Only load contracts we need
		if !requiredSet[contractName] {
			continue
		}

		// Read and parse the file
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", f.Name, err)
		}

		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name, err)
		}

		var artifact ContractArtifact
		if err := json.Unmarshal(data, &artifact); err != nil {
			return nil, fmt.Errorf("parse %s: %w", f.Name, err)
		}

		loaded[contractName] = &artifact
	}

	// Verify we loaded all required contracts
	var missing []string
	for _, name := range requiredContracts {
		if loaded[name] == nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required contracts: %v", missing)
	}

	// Build the NitroArtifacts struct
	return &NitroArtifacts{
		// Core deployment
		RollupCreator: loaded["RollupCreator"],
		BridgeCreator: loaded["BridgeCreator"],
		// Rollup infrastructure (ETH native)
		SequencerInbox:   loaded["SequencerInbox"],
		Bridge:           loaded["Bridge"],
		Inbox:            loaded["Inbox"],
		Outbox:           loaded["Outbox"],
		RollupEventInbox: loaded["RollupEventInbox"],
		RollupCore:       loaded["RollupCore"],
		RollupAdminLogic: loaded["RollupAdminLogic"],
		RollupUserLogic:  loaded["RollupUserLogic"],
		// ERC20 native token variants
		ERC20Bridge: loaded["ERC20Bridge"],
		ERC20Inbox:  loaded["ERC20Inbox"],
		// Challenge/Fraud proofs (BOLD)
		EdgeChallengeManager: loaded["EdgeChallengeManager"],
		OneStepProofEntry:    loaded["OneStepProofEntry"],
		OneStepProver0:       loaded["OneStepProver0"],
		OneStepProverMemory:  loaded["OneStepProverMemory"],
		OneStepProverMath:    loaded["OneStepProverMath"],
		OneStepProverHostIo:  loaded["OneStepProverHostIo"],
		// Upgrade/Validator infrastructure
		UpgradeExecutor:        loaded["UpgradeExecutor"],
		ValidatorWalletCreator: loaded["ValidatorWalletCreator"],
		DeployHelper:           loaded["DeployHelper"],
		// EIP-4844 blob reader
		Reader4844: loaded["Reader4844"],
	}, nil
}

// LoadFromDirectory loads artifacts from a local directory (for testing or offline use).
func LoadFromDirectory(dir string) (*NitroArtifacts, error) {
	// List of contracts we need
	contracts := map[string]**ContractArtifact{}

	artifacts := &NitroArtifacts{}
	// Core deployment
	contracts["RollupCreator"] = &artifacts.RollupCreator
	contracts["BridgeCreator"] = &artifacts.BridgeCreator
	// Rollup infrastructure (ETH native)
	contracts["SequencerInbox"] = &artifacts.SequencerInbox
	contracts["Bridge"] = &artifacts.Bridge
	contracts["Inbox"] = &artifacts.Inbox
	contracts["Outbox"] = &artifacts.Outbox
	contracts["RollupEventInbox"] = &artifacts.RollupEventInbox
	contracts["RollupCore"] = &artifacts.RollupCore
	contracts["RollupAdminLogic"] = &artifacts.RollupAdminLogic
	contracts["RollupUserLogic"] = &artifacts.RollupUserLogic
	// ERC20 native token variants
	contracts["ERC20Bridge"] = &artifacts.ERC20Bridge
	contracts["ERC20Inbox"] = &artifacts.ERC20Inbox
	// Challenge/Fraud proofs (BOLD)
	contracts["EdgeChallengeManager"] = &artifacts.EdgeChallengeManager
	contracts["OneStepProofEntry"] = &artifacts.OneStepProofEntry
	contracts["OneStepProver0"] = &artifacts.OneStepProver0
	contracts["OneStepProverMemory"] = &artifacts.OneStepProverMemory
	contracts["OneStepProverMath"] = &artifacts.OneStepProverMath
	contracts["OneStepProverHostIo"] = &artifacts.OneStepProverHostIo
	// Upgrade/Validator infrastructure
	contracts["UpgradeExecutor"] = &artifacts.UpgradeExecutor
	contracts["ValidatorWalletCreator"] = &artifacts.ValidatorWalletCreator
	contracts["DeployHelper"] = &artifacts.DeployHelper
	// EIP-4844 blob reader
	contracts["Reader4844"] = &artifacts.Reader4844

	var missing []string
	for name, ptr := range contracts {
		path := filepath.Join(dir, name+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			missing = append(missing, name)
			continue
		}

		var artifact ContractArtifact
		if err := json.Unmarshal(data, &artifact); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		*ptr = &artifact
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required contracts in %s: %v", dir, missing)
	}

	artifacts.Version = "local"
	artifacts.LoadedAt = time.Now()
	artifacts.SourceURL = "file://" + dir

	return artifacts, nil
}

// EncodeConstructorArgs encodes constructor arguments using the contract's ABI.
// Returns the encoded args (without bytecode prefix) ready to append to bytecode.
func (a *ContractArtifact) EncodeConstructorArgs(args ...interface{}) ([]byte, error) {
	if len(args) == 0 {
		return nil, nil
	}

	// Parse the ABI
	parsedABI, err := abi.JSON(bytes.NewReader(a.ABI))
	if err != nil {
		return nil, fmt.Errorf("parse ABI: %w", err)
	}

	// Check if constructor exists
	if parsedABI.Constructor.Inputs == nil {
		return nil, fmt.Errorf("contract has no constructor")
	}

	// Pack the arguments
	packed, err := parsedABI.Constructor.Inputs.Pack(args...)
	if err != nil {
		return nil, fmt.Errorf("pack constructor args: %w", err)
	}

	return packed, nil
}

// EncodeFunctionCall encodes a function call using the contract's ABI.
func (a *ContractArtifact) EncodeFunctionCall(method string, args ...interface{}) ([]byte, error) {
	// Parse the ABI
	parsedABI, err := abi.JSON(bytes.NewReader(a.ABI))
	if err != nil {
		return nil, fmt.Errorf("parse ABI: %w", err)
	}

	// Pack the function call
	packed, err := parsedABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("pack function call: %w", err)
	}

	return packed, nil
}

// GetParsedABI returns the parsed ABI for direct access.
func (a *ContractArtifact) GetParsedABI() (abi.ABI, error) {
	return abi.JSON(bytes.NewReader(a.ABI))
}

// GetBytecodeBytes returns the bytecode as a byte slice.
func (a *ContractArtifact) GetBytecodeBytes() ([]byte, error) {
	bytecode := a.Bytecode.hex
	if len(bytecode) == 0 {
		return nil, fmt.Errorf("empty bytecode")
	}

	// Remove 0x prefix if present
	if len(bytecode) >= 2 && bytecode[:2] == "0x" {
		bytecode = bytecode[2:]
	}

	return hexDecode(bytecode)
}

// hexDecode decodes a hex string to bytes.
func hexDecode(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("odd length hex string")
	}

	result := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		var b byte
		_, err := fmt.Sscanf(s[i:i+2], "%02x", &b)
		if err != nil {
			return nil, fmt.Errorf("invalid hex at position %d: %w", i, err)
		}
		result[i/2] = b
	}
	return result, nil
}

