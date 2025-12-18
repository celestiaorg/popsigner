package opstack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// ArtifactExtractor extracts and bundles deployment artifacts from OP Stack state.
type ArtifactExtractor struct {
	repo repository.Repository
}

// NewArtifactExtractor creates a new artifact extractor.
func NewArtifactExtractor(repo repository.Repository) *ArtifactExtractor {
	return &ArtifactExtractor{repo: repo}
}

// OPStackArtifacts contains all artifacts from an OP Stack deployment.
type OPStackArtifacts struct {
	Genesis       json.RawMessage   `json:"genesis"`            // L2 genesis.json
	Rollup        json.RawMessage   `json:"rollup"`             // rollup.json configuration
	Addresses     ContractAddresses `json:"contract_addresses"` // Deployed contract addresses
	DeployConfig  json.RawMessage   `json:"deploy_config"`      // Original deployment config
	JWTSecret     string            `json:"jwt_secret"`         // Engine API JWT secret
	DockerCompose string            `json:"docker_compose"`     // Generated docker-compose.yml
	EnvExample    string            `json:"env_example"`        // .env.example template
}

// ContractAddresses contains all deployed OP Stack contract addresses.
type ContractAddresses struct {
	// Superchain contracts
	SuperchainConfig string `json:"superchain_config,omitempty"`
	ProtocolVersions string `json:"protocol_versions,omitempty"`

	// Proxy contracts
	OptimismPortalProxy         string `json:"optimism_portal_proxy"`
	L1CrossDomainMessengerProxy string `json:"l1_cross_domain_messenger_proxy"`
	L1StandardBridgeProxy       string `json:"l1_standard_bridge_proxy"`
	L1ERC721BridgeProxy         string `json:"l1_erc721_bridge_proxy,omitempty"`
	SystemConfigProxy           string `json:"system_config_proxy"`
	DisputeGameFactoryProxy     string `json:"dispute_game_factory_proxy,omitempty"`
	AnchorStateRegistryProxy    string `json:"anchor_state_registry_proxy,omitempty"`
	DelayedWETHProxy            string `json:"delayed_weth_proxy,omitempty"`

	// Other contracts
	OptimismMintableERC20Factory string `json:"optimism_mintable_erc20_factory,omitempty"`
	AddressManager               string `json:"address_manager,omitempty"`
	BatchInbox                   string `json:"batch_inbox"`
	L2OutputOracle               string `json:"l2_output_oracle,omitempty"` // Legacy, pre-fault proofs
}

// RollupConfig represents the rollup.json configuration structure.
type RollupConfig struct {
	Genesis              RollupGenesisConfig `json:"genesis"`
	BlockTime            uint64              `json:"block_time"`
	MaxSequencerDrift    uint64              `json:"max_sequencer_drift"`
	SequencerWindowSize  uint64              `json:"sequencer_window_size"`
	ChannelTimeout       uint64              `json:"channel_timeout"`
	L1ChainID            uint64              `json:"l1_chain_id"`
	L2ChainID            uint64              `json:"l2_chain_id"`
	RegolithTime         *uint64             `json:"regolith_time,omitempty"`
	CanyonTime           *uint64             `json:"canyon_time,omitempty"`
	DeltaTime            *uint64             `json:"delta_time,omitempty"`
	EcotoneTime          *uint64             `json:"ecotone_time,omitempty"`
	FjordTime            *uint64             `json:"fjord_time,omitempty"`
	GraniteTime          *uint64             `json:"granite_time,omitempty"`
	HoloceneTime         *uint64             `json:"holocene_time,omitempty"`
	BatchInboxAddress    string              `json:"batch_inbox_address"`
	DepositContractAddr  string              `json:"deposit_contract_address"`
	L1SystemConfigAddr   string              `json:"l1_system_config_address"`
	ProtocolVersionsAddr string              `json:"protocol_versions_address,omitempty"`

	// Alt-DA configuration
	AltDAEnabled    bool   `json:"alt_da_enabled,omitempty"`
	DAChallengeAddr string `json:"da_challenge_address,omitempty"`
}

// RollupGenesisConfig represents the genesis portion of rollup.json.
type RollupGenesisConfig struct {
	L1        GenesisBlockRef `json:"l1"`
	L2        GenesisBlockRef `json:"l2"`
	L2Time    uint64          `json:"l2_time"`
	SystemConfig SystemConfig  `json:"system_config"`
}

// GenesisBlockRef represents a block reference in rollup genesis.
type GenesisBlockRef struct {
	Hash   string `json:"hash"`
	Number uint64 `json:"number"`
}

// SystemConfig represents the system configuration in rollup.json.
type SystemConfig struct {
	BatcherAddr       string `json:"batcherAddr"`
	Overhead          string `json:"overhead"`
	Scalar            string `json:"scalar"`
	GasLimit          uint64 `json:"gasLimit"`
	BaseFeeScalar     uint64 `json:"baseFeeScalar,omitempty"`
	BlobBaseFeeScalar uint64 `json:"blobBaseFeeScalar,omitempty"`
}

// ExtractArtifacts extracts all deployment artifacts from the deployment state.
func (e *ArtifactExtractor) ExtractArtifacts(
	ctx context.Context,
	deploymentID uuid.UUID,
	cfg *DeploymentConfig,
) (*OPStackArtifacts, error) {
	artifacts := &OPStackArtifacts{}

	// 1. Extract genesis.json from saved artifact
	genesis, err := e.extractGenesis(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("extract genesis: %w", err)
	}
	artifacts.Genesis = genesis

	// 2. Build rollup.json from state and config
	rollup, err := e.buildRollupConfig(ctx, deploymentID, cfg)
	if err != nil {
		return nil, fmt.Errorf("build rollup config: %w", err)
	}
	rollupJSON, err := json.MarshalIndent(rollup, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal rollup config: %w", err)
	}
	artifacts.Rollup = rollupJSON

	// 3. Extract contract addresses from state
	addrs, err := e.extractContractAddresses(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("extract addresses: %w", err)
	}
	artifacts.Addresses = addrs

	// 4. Get original deployment config
	cfgJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	artifacts.DeployConfig = cfgJSON

	// 5. Generate JWT secret for Engine API
	artifacts.JWTSecret = generateJWTSecret()

	// 6. Generate Docker Compose
	compose, err := GenerateDockerCompose(cfg, artifacts)
	if err != nil {
		return nil, fmt.Errorf("generate docker-compose: %w", err)
	}
	artifacts.DockerCompose = compose

	// 7. Generate .env.example
	artifacts.EnvExample = GenerateEnvExample(cfg, &addrs)

	// 8. Save all artifacts to database
	if err := e.saveAllArtifacts(ctx, deploymentID, artifacts); err != nil {
		return nil, fmt.Errorf("save artifacts: %w", err)
	}

	return artifacts, nil
}

// extractGenesis retrieves the genesis.json from the database.
func (e *ArtifactExtractor) extractGenesis(ctx context.Context, deploymentID uuid.UUID) (json.RawMessage, error) {
	artifact, err := e.repo.GetArtifact(ctx, deploymentID, "genesis")
	if err != nil {
		return nil, fmt.Errorf("get genesis artifact: %w", err)
	}
	if artifact == nil {
		return nil, fmt.Errorf("genesis artifact not found")
	}
	return artifact.Content, nil
}

// buildRollupConfig constructs the rollup.json from deployment state and config.
func (e *ArtifactExtractor) buildRollupConfig(
	ctx context.Context,
	deploymentID uuid.UUID,
	cfg *DeploymentConfig,
) (*RollupConfig, error) {
	// Get deployment state
	artifact, err := e.repo.GetArtifact(ctx, deploymentID, "deployment_state")
	if err != nil {
		return nil, fmt.Errorf("get state artifact: %w", err)
	}

	var state map[string]interface{}
	if artifact != nil {
		if err := json.Unmarshal(artifact.Content, &state); err != nil {
			return nil, fmt.Errorf("unmarshal state: %w", err)
		}
	}

	// Get rollup config artifact if already generated
	rollupArtifact, _ := e.repo.GetArtifact(ctx, deploymentID, "rollup_config")
	if rollupArtifact != nil {
		var rollup RollupConfig
		if err := json.Unmarshal(rollupArtifact.Content, &rollup); err == nil {
			return &rollup, nil
		}
	}

	// Build rollup config from deployment config
	// L2 genesis time should come from state, default to now
	l2Time := uint64(time.Now().Unix())
	if ts, ok := state["l2_genesis_time"].(float64); ok {
		l2Time = uint64(ts)
	}

	rollup := &RollupConfig{
		Genesis: RollupGenesisConfig{
			L1: GenesisBlockRef{
				Hash:   getStringFromState(state, "l1_genesis_hash", "0x0000000000000000000000000000000000000000000000000000000000000000"),
				Number: getUint64FromState(state, "l1_genesis_number", 0),
			},
			L2: GenesisBlockRef{
				Hash:   getStringFromState(state, "l2_genesis_hash", "0x0000000000000000000000000000000000000000000000000000000000000000"),
				Number: 0,
			},
			L2Time: l2Time,
			SystemConfig: SystemConfig{
				BatcherAddr:   cfg.BatcherAddress,
				Overhead:      "0x0000000000000000000000000000000000000000000000000000000000000834",
				Scalar:        "0x00000000000000000000000000000000000000000000000000000000000f4240",
				GasLimit:      cfg.GasLimit,
			},
		},
		BlockTime:           cfg.BlockTime,
		MaxSequencerDrift:   cfg.MaxSequencerDrift,
		SequencerWindowSize: cfg.SequencerWindowSize,
		ChannelTimeout:      300,
		L1ChainID:           cfg.L1ChainID,
		L2ChainID:           cfg.ChainID,
		BatchInboxAddress:   calculateBatchInboxAddress(cfg.ChainID),
		DepositContractAddr: getStringFromState(state, "optimism_portal_proxy", ""),
		L1SystemConfigAddr:  getStringFromState(state, "system_config_proxy", ""),
	}

	// Add hardfork timestamps (set at genesis for new chains)
	zero := uint64(0)
	rollup.RegolithTime = &zero
	rollup.CanyonTime = &zero
	rollup.DeltaTime = &zero
	rollup.EcotoneTime = &zero
	rollup.FjordTime = &zero
	rollup.GraniteTime = &zero

	// Alt-DA configuration
	if cfg.UseAltDA {
		rollup.AltDAEnabled = true
	}

	return rollup, nil
}

// extractContractAddresses retrieves deployed contract addresses from state.
func (e *ArtifactExtractor) extractContractAddresses(ctx context.Context, deploymentID uuid.UUID) (ContractAddresses, error) {
	artifact, err := e.repo.GetArtifact(ctx, deploymentID, "deployment_state")
	if err != nil {
		return ContractAddresses{}, fmt.Errorf("get state artifact: %w", err)
	}

	addrs := ContractAddresses{}

	if artifact != nil {
		var state map[string]interface{}
		if err := json.Unmarshal(artifact.Content, &state); err != nil {
			return addrs, fmt.Errorf("unmarshal state: %w", err)
		}

		// Extract addresses from state
		addrs.OptimismPortalProxy = getStringFromState(state, "optimism_portal_proxy", "")
		addrs.L1CrossDomainMessengerProxy = getStringFromState(state, "l1_cross_domain_messenger_proxy", "")
		addrs.L1StandardBridgeProxy = getStringFromState(state, "l1_standard_bridge_proxy", "")
		addrs.L1ERC721BridgeProxy = getStringFromState(state, "l1_erc721_bridge_proxy", "")
		addrs.SystemConfigProxy = getStringFromState(state, "system_config_proxy", "")
		addrs.DisputeGameFactoryProxy = getStringFromState(state, "dispute_game_factory_proxy", "")
		addrs.AnchorStateRegistryProxy = getStringFromState(state, "anchor_state_registry_proxy", "")
		addrs.DelayedWETHProxy = getStringFromState(state, "delayed_weth_proxy", "")
		addrs.OptimismMintableERC20Factory = getStringFromState(state, "optimism_mintable_erc20_factory", "")
		addrs.AddressManager = getStringFromState(state, "address_manager", "")
		addrs.SuperchainConfig = getStringFromState(state, "superchain_config", "")
		addrs.ProtocolVersions = getStringFromState(state, "protocol_versions", "")
	}

	// Get batch inbox address from chain ID
	deployment, err := e.repo.GetDeployment(ctx, deploymentID)
	if err == nil && deployment != nil {
		addrs.BatchInbox = calculateBatchInboxAddress(uint64(deployment.ChainID))
	}

	return addrs, nil
}

// saveAllArtifacts saves all artifacts to the database.
func (e *ArtifactExtractor) saveAllArtifacts(ctx context.Context, deploymentID uuid.UUID, arts *OPStackArtifacts) error {
	// Save genesis.json (already saved during deployment, but update if needed)
	if len(arts.Genesis) > 0 {
		if err := e.saveArtifact(ctx, deploymentID, "genesis.json", arts.Genesis); err != nil {
			return err
		}
	}

	// Save rollup.json
	if len(arts.Rollup) > 0 {
		if err := e.saveArtifact(ctx, deploymentID, "rollup.json", arts.Rollup); err != nil {
			return err
		}
	}

	// Save addresses.json
	addrsBytes, err := json.MarshalIndent(arts.Addresses, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal addresses: %w", err)
	}
	if err := e.saveArtifact(ctx, deploymentID, "addresses.json", addrsBytes); err != nil {
		return err
	}

	// Save deploy-config.json
	if len(arts.DeployConfig) > 0 {
		if err := e.saveArtifact(ctx, deploymentID, "deploy-config.json", arts.DeployConfig); err != nil {
			return err
		}
	}

	// Save docker-compose.yml
	if arts.DockerCompose != "" {
		if err := e.saveArtifact(ctx, deploymentID, "docker-compose.yml", []byte(arts.DockerCompose)); err != nil {
			return err
		}
	}

	// Save .env.example
	if arts.EnvExample != "" {
		if err := e.saveArtifact(ctx, deploymentID, ".env.example", []byte(arts.EnvExample)); err != nil {
			return err
		}
	}

	// Save JWT secret
	if arts.JWTSecret != "" {
		if err := e.saveArtifact(ctx, deploymentID, "jwt.txt", []byte(arts.JWTSecret)); err != nil {
			return err
		}
	}

	return nil
}

// saveArtifact saves a single artifact to the database.
func (e *ArtifactExtractor) saveArtifact(ctx context.Context, deploymentID uuid.UUID, name string, content []byte) error {
	artifact := &repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ArtifactType: name,
		Content:      content,
		CreatedAt:    time.Now(),
	}
	return e.repo.SaveArtifact(ctx, artifact)
}

// CreateBundle packages all artifacts into a tar.gz bundle.
func (e *ArtifactExtractor) CreateBundle(ctx context.Context, deploymentID uuid.UUID, chainName string) ([]byte, error) {
	artifacts, err := e.repo.GetAllArtifacts(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("get artifacts: %w", err)
	}

	if len(artifacts) == 0 {
		return nil, fmt.Errorf("no artifacts found for deployment %s", deploymentID)
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Create bundle directory structure
	bundlePrefix := fmt.Sprintf("%s-opstack-artifacts/", chainName)

	// Add README.md
	readme := generateBundleReadme(chainName)
	if err := addToTar(tw, bundlePrefix+"README.md", []byte(readme)); err != nil {
		return nil, err
	}

	// Organize artifacts into directories
	for _, a := range artifacts {
		var path string
		switch a.ArtifactType {
		case "genesis.json":
			path = bundlePrefix + "genesis/genesis.json"
		case "rollup.json", "deploy-config.json", "addresses.json":
			path = bundlePrefix + "config/" + a.ArtifactType
		case "docker-compose.yml", ".env.example":
			path = bundlePrefix + a.ArtifactType
		case "jwt.txt":
			path = bundlePrefix + "secrets/jwt.txt"
		default:
			// Skip internal artifacts like deployment_state
			continue
		}

		if err := addToTar(tw, path, a.Content); err != nil {
			return nil, fmt.Errorf("add %s to tar: %w", a.ArtifactType, err)
		}
	}

	// Add helper scripts
	startScript := generateStartScript()
	if err := addToTar(tw, bundlePrefix+"scripts/start.sh", []byte(startScript)); err != nil {
		return nil, err
	}

	healthcheckScript := generateHealthcheckScript()
	if err := addToTar(tw, bundlePrefix+"scripts/healthcheck.sh", []byte(healthcheckScript)); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// GetArtifact retrieves a specific artifact by type.
func (e *ArtifactExtractor) GetArtifact(ctx context.Context, deploymentID uuid.UUID, artifactType string) ([]byte, error) {
	artifact, err := e.repo.GetArtifact(ctx, deploymentID, artifactType)
	if err != nil {
		return nil, fmt.Errorf("get artifact: %w", err)
	}
	if artifact == nil {
		return nil, fmt.Errorf("artifact %s not found", artifactType)
	}
	return artifact.Content, nil
}

// ListArtifacts returns all available artifact types for a deployment.
func (e *ArtifactExtractor) ListArtifacts(ctx context.Context, deploymentID uuid.UUID) ([]string, error) {
	artifacts, err := e.repo.GetAllArtifacts(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("get artifacts: %w", err)
	}

	types := make([]string, 0, len(artifacts))
	for _, a := range artifacts {
		// Skip internal artifacts
		if a.ArtifactType == "deployment_state" {
			continue
		}
		types = append(types, a.ArtifactType)
	}
	return types, nil
}

// Helper functions

// generateJWTSecret generates a random JWT secret for Engine API authentication.
func generateJWTSecret() string {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		// Fallback to a static secret (should not happen in practice)
		return "0x" + "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	}
	return "0x" + hex.EncodeToString(secret)
}

// calculateBatchInboxAddress calculates the deterministic batch inbox address for a chain.
func calculateBatchInboxAddress(chainID uint64) string {
	// Standard batch inbox format: 0xff00...{chainID in 4 bytes}
	return fmt.Sprintf("0xff00000000000000000000000000000000%08x", chainID)
}

// getStringFromState safely extracts a string value from state map.
func getStringFromState(state map[string]interface{}, key string, defaultVal string) string {
	if v, ok := state[key].(string); ok {
		return v
	}
	return defaultVal
}

// getUint64FromState safely extracts a uint64 value from state map.
func getUint64FromState(state map[string]interface{}, key string, defaultVal uint64) uint64 {
	if v, ok := state[key].(float64); ok {
		return uint64(v)
	}
	return defaultVal
}

// addToTar adds a file to the tar archive.
func addToTar(tw *tar.Writer, name string, content []byte) error {
	hdr := &tar.Header{
		Name:    name,
		Size:    int64(len(content)),
		Mode:    0644,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(content)
	return err
}

// generateBundleReadme generates the README.md for the artifact bundle.
func generateBundleReadme(chainName string) string {
	return fmt.Sprintf(`# %s OP Stack Artifacts

This bundle contains everything needed to run your OP Stack rollup.

## Quick Start

1. Copy .env.example to .env and configure:
   cp .env.example .env
   vim .env

2. Start your rollup:
   ./scripts/start.sh

3. Verify health:
   ./scripts/healthcheck.sh

## Contents

- genesis/genesis.json     - L2 genesis state
- config/rollup.json       - Rollup configuration
- config/addresses.json    - Deployed contract addresses
- config/deploy-config.json - Original deployment configuration
- secrets/jwt.txt          - Engine API JWT secret
- docker-compose.yml       - Ready-to-run Docker configuration
- .env.example             - Environment variable template

## Required Environment Variables

- L1_RPC_URL: Your L1 RPC endpoint (Alchemy, Infura, etc.)
- POPSIGNER_RPC_URL: POPSigner signing service URL
- POPSIGNER_API_KEY: Your POPSigner API key

## Documentation

- OP Stack Docs: https://docs.optimism.io
- POPSigner Docs: https://docs.popsigner.io

Generated by POPKins Chain Bootstrapping Service
`, chainName)
}

// generateStartScript generates the start.sh helper script.
func generateStartScript() string {
	return `#!/bin/bash
set -e

echo "Starting OP Stack services..."

# Check for .env file
if [ ! -f .env ]; then
    echo "Error: .env file not found. Copy .env.example to .env and configure it."
    exit 1
fi

# Start services
docker compose up -d

echo "Services started! Run ./scripts/healthcheck.sh to verify."
`
}

// generateHealthcheckScript generates the healthcheck.sh script.
func generateHealthcheckScript() string {
	return `#!/bin/bash

echo "Checking OP Stack service health..."

# Check op-geth
echo -n "op-geth: "
if curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://localhost:8545 | grep -q result; then
    echo "✓ OK"
else
    echo "✗ FAILED"
fi

# Check op-node
echo -n "op-node: "
if curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"optimism_syncStatus","params":[],"id":1}' \
    http://localhost:9545 | grep -q result; then
    echo "✓ OK"
else
    echo "✗ FAILED"
fi

# Check container status
echo ""
echo "Container status:"
docker compose ps
`
}

// GenerateEnvExample generates the .env.example file content.
func GenerateEnvExample(cfg *DeploymentConfig, addrs *ContractAddresses) string {
	return fmt.Sprintf(`# L1 Configuration
L1_RPC_URL=%s

# POPSigner Configuration (API Key Auth)
POPSIGNER_RPC_URL=%s
POPSIGNER_API_KEY=your_api_key_here

# Role Addresses (from deployment)
BATCHER_ADDRESS=%s
PROPOSER_ADDRESS=%s
L2_OUTPUT_ORACLE_ADDRESS=%s

# Chain Configuration
L2_CHAIN_ID=%d
L2_CHAIN_NAME=%s

# Celestia DA (if enabled)
# CELESTIA_RPC_URL=%s
# CELESTIA_AUTH_TOKEN=your_token_here
# CELESTIA_NAMESPACE=your_namespace_hex
`,
		cfg.L1RPC,
		cfg.POPSignerEndpoint,
		cfg.BatcherAddress,
		cfg.ProposerAddress,
		addrs.L2OutputOracle,
		cfg.ChainID,
		cfg.ChainName,
		cfg.CelestiaRPC,
	)
}

