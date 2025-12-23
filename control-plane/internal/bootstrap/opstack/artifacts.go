package opstack

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
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
	AltDAConfig   string            `json:"altda_config"`       // op-alt-da config.toml (Celestia)
	Readme        string            `json:"readme"`             // Bundle README.md
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
	L1           GenesisBlockRef `json:"l1"`
	L2           GenesisBlockRef `json:"l2"`
	L2Time       uint64          `json:"l2_time"`
	SystemConfig SystemConfig    `json:"system_config"`
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

	// 2. Extract rollup.json - prefer saved artifact from op-deployer, fallback to building from config
	rollupJSON, err := e.extractRollupConfig(ctx, deploymentID, cfg)
	if err != nil {
		return nil, fmt.Errorf("extract rollup config: %w", err)
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

	// 8. Generate op-alt-da config.toml (Celestia DA - always enabled for POPKins)
	altDAConfig, err := GenerateAltDAConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("generate altda config: %w", err)
	}
	artifacts.AltDAConfig = altDAConfig

	// 9. Generate README (POPKins always uses Celestia DA)
	artifacts.Readme = GenerateBundleReadme(cfg.ChainName, true)

	// 10. Save all artifacts to database
	if err := e.saveAllArtifacts(ctx, deploymentID, artifacts); err != nil {
		return nil, fmt.Errorf("save artifacts: %w", err)
	}

	return artifacts, nil
}

// extractGenesis retrieves the genesis.json from the database.
func (e *ArtifactExtractor) extractGenesis(ctx context.Context, deploymentID uuid.UUID) (json.RawMessage, error) {
	// Try new artifact name first (genesis.json)
	artifact, err := e.repo.GetArtifact(ctx, deploymentID, "genesis.json")
	if err != nil {
		return nil, fmt.Errorf("get genesis artifact: %w", err)
	}
	if artifact != nil && len(artifact.Content) > 0 {
		return artifact.Content, nil
	}

	// Fallback to old name (genesis) for backwards compatibility
	artifact, err = e.repo.GetArtifact(ctx, deploymentID, "genesis")
	if err != nil {
		return nil, fmt.Errorf("get genesis artifact (legacy): %w", err)
	}
	if artifact == nil {
		return nil, fmt.Errorf("genesis artifact not found")
	}
	return artifact.Content, nil
}

// extractRollupConfig retrieves the rollup.json from the database.
// It prefers the saved artifact from op-deployer (which uses inspect.GenesisAndRollup)
// and falls back to building from deployment config if not found.
func (e *ArtifactExtractor) extractRollupConfig(ctx context.Context, deploymentID uuid.UUID, cfg *DeploymentConfig) (json.RawMessage, error) {
	// Try new artifact name first (rollup.json)
	artifact, err := e.repo.GetArtifact(ctx, deploymentID, "rollup.json")
	if err != nil {
		return nil, fmt.Errorf("get rollup config artifact: %w", err)
	}
	if artifact != nil && len(artifact.Content) > 0 {
		return artifact.Content, nil
	}

	// Fallback to old name (rollup_config)
	artifact, err = e.repo.GetArtifact(ctx, deploymentID, "rollup_config")
	if err != nil {
		return nil, fmt.Errorf("get rollup config artifact (legacy): %w", err)
	}
	if artifact != nil && len(artifact.Content) > 0 {
		// Return the saved rollup config directly (it's already in the correct format)
		return artifact.Content, nil
	}

	// Fallback: build from deployment config (legacy path)
	rollup, err := e.buildRollupConfig(ctx, deploymentID, cfg)
	if err != nil {
		return nil, fmt.Errorf("build rollup config: %w", err)
	}
	return json.MarshalIndent(rollup, "", "  ")
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

	// Extract chain_state for contract addresses (uses camelCase from Go struct serialization)
	chainState, _ := state["chain_state"].(map[string]interface{})
	if chainState == nil {
		chainState = state // Fallback to top level
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
				BatcherAddr: cfg.BatcherAddress,
				Overhead:    "0x0000000000000000000000000000000000000000000000000000000000000834",
				Scalar:      "0x00000000000000000000000000000000000000000000000000000000000f4240",
				GasLimit:    cfg.GasLimit,
			},
		},
		BlockTime:           cfg.BlockTime,
		MaxSequencerDrift:   cfg.MaxSequencerDrift,
		SequencerWindowSize: cfg.SequencerWindowSize,
		ChannelTimeout:      300,
		L1ChainID:           cfg.L1ChainID,
		L2ChainID:           cfg.ChainID,
		BatchInboxAddress:   calculateBatchInboxAddress(cfg.ChainID),
		DepositContractAddr: getAddressFromState(chainState, "OptimismPortalProxy"),
		L1SystemConfigAddr:  getAddressFromState(chainState, "SystemConfigProxy"),
	}

	// Add hardfork timestamps (set at genesis for new chains)
	zero := uint64(0)
	rollup.RegolithTime = &zero
	rollup.CanyonTime = &zero
	rollup.DeltaTime = &zero
	rollup.EcotoneTime = &zero
	rollup.FjordTime = &zero
	rollup.GraniteTime = &zero

	// Alt-DA configuration - always enabled for Celestia DA
	// POPKins exclusively uses Celestia as the DA layer
	rollup.AltDAEnabled = true

	return rollup, nil
}

// extractContractAddresses retrieves deployed contract addresses from state.
// The state structure from op-deployer is:
//
//	{
//	  "chain_state": {
//	    "OptimismPortalProxy": "0x...",      // camelCase from Go struct
//	    "L1CrossDomainMessengerProxy": "0x...",
//	    ...
//	  },
//	  "superchain_deployment": { ... },
//	  "implementations_deployment": { ... }
//	}
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

		// The addresses are nested in chain_state (from op-deployer's ChainState struct)
		chainState, _ := state["chain_state"].(map[string]interface{})
		if chainState == nil {
			// Fallback: try top-level (older format)
			chainState = state
		}

		// Extract addresses using camelCase keys (Go struct JSON serialization)
		// OpChainCoreContracts
		addrs.OptimismPortalProxy = getAddressFromState(chainState, "OptimismPortalProxy")
		addrs.L1CrossDomainMessengerProxy = getAddressFromState(chainState, "L1CrossDomainMessengerProxy")
		addrs.L1StandardBridgeProxy = getAddressFromState(chainState, "L1StandardBridgeProxy")
		addrs.L1ERC721BridgeProxy = getAddressFromState(chainState, "L1Erc721BridgeProxy")
		addrs.SystemConfigProxy = getAddressFromState(chainState, "SystemConfigProxy")
		addrs.OptimismMintableERC20Factory = getAddressFromState(chainState, "OptimismMintableErc20FactoryProxy")
		addrs.AddressManager = getAddressFromState(chainState, "AddressManagerImpl")

		// OpChainFaultProofsContracts
		addrs.DisputeGameFactoryProxy = getAddressFromState(chainState, "DisputeGameFactoryProxy")
		addrs.AnchorStateRegistryProxy = getAddressFromState(chainState, "AnchorStateRegistryProxy")
		addrs.DelayedWETHProxy = getAddressFromState(chainState, "DelayedWethPermissionedGameProxy")

		// SuperchainContracts (from superchain_deployment)
		superchain, _ := state["superchain_deployment"].(map[string]interface{})
		if superchain != nil {
			addrs.SuperchainConfig = getAddressFromState(superchain, "SuperchainConfigProxy")
			addrs.ProtocolVersions = getAddressFromState(superchain, "ProtocolVersionsProxy")
		}
	}

	// Get batch inbox address from chain ID
	deployment, err := e.repo.GetDeployment(ctx, deploymentID)
	if err == nil && deployment != nil {
		addrs.BatchInbox = calculateBatchInboxAddress(uint64(deployment.ChainID))
	}

	return addrs, nil
}

// getAddressFromState extracts an Ethereum address from the state map.
// It handles both string addresses and common.Address types (which serialize as hex strings).
func getAddressFromState(state map[string]interface{}, key string) string {
	if state == nil {
		return ""
	}
	if v, ok := state[key].(string); ok {
		return v
	}
	return ""
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

	// Save op-alt-da config.toml (Celestia)
	if arts.AltDAConfig != "" {
		if err := e.saveArtifact(ctx, deploymentID, "config.toml", []byte(arts.AltDAConfig)); err != nil {
			return err
		}
	}

	// Save README.md
	if arts.Readme != "" {
		if err := e.saveArtifact(ctx, deploymentID, "README.md", []byte(arts.Readme)); err != nil {
			return err
		}
	}

	return nil
}

// saveArtifact saves a single artifact to the database.
// For non-JSON content (like docker-compose.yml, jwt.txt), wraps as base64 in a JSON object.
// This avoids PostgreSQL JSONB normalization issues with escape sequences.
func (e *ArtifactExtractor) saveArtifact(ctx context.Context, deploymentID uuid.UUID, name string, content []byte) error {
	var jsonContent json.RawMessage

	// Check if content is already valid JSON
	if json.Valid(content) {
		jsonContent = content
	} else {
		// Wrap non-JSON content as base64 in a JSON object.
		// This avoids PostgreSQL JSONB escape sequence normalization issues.
		wrapper := struct {
			Type string `json:"_type"`
			Data string `json:"data"`
		}{
			Type: "base64",
			Data: base64.StdEncoding.EncodeToString(content),
		}
		encoded, err := json.Marshal(wrapper)
		if err != nil {
			return fmt.Errorf("marshal non-JSON content for %s: %w", name, err)
		}
		jsonContent = encoded
	}

	artifact := &repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ArtifactType: name,
		Content:      jsonContent,
		CreatedAt:    time.Now(),
	}
	return e.repo.SaveArtifact(ctx, artifact)
}

// CreateBundle packages all artifacts into a ZIP bundle.
func (e *ArtifactExtractor) CreateBundle(ctx context.Context, deploymentID uuid.UUID, chainName string) ([]byte, error) {
	artifacts, err := e.repo.GetAllArtifacts(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("get artifacts: %w", err)
	}

	if len(artifacts) == 0 {
		return nil, fmt.Errorf("no artifacts found for deployment %s", deploymentID)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Create bundle directory structure
	bundlePrefix := fmt.Sprintf("%s-opstack-bundle/", sanitizeChainName(chainName))

	// Organize artifacts into the bundle
	for _, a := range artifacts {
		var path string
		isPlainText := false // Non-JSON files stored as JSON strings need unwrapping
		switch a.ArtifactType {
		case "genesis.json":
			path = bundlePrefix + "genesis.json"
		case "rollup.json":
			path = bundlePrefix + "rollup.json"
		case "addresses.json":
			path = bundlePrefix + "addresses.json"
		case "deploy-config.json":
			path = bundlePrefix + "deploy-config.json"
		case "docker-compose.yml":
			path = bundlePrefix + "docker-compose.yml"
			isPlainText = true
		case ".env.example":
			path = bundlePrefix + ".env.example"
			isPlainText = true
		case "jwt.txt":
			path = bundlePrefix + "jwt.txt"
			isPlainText = true
		case "config.toml":
			path = bundlePrefix + "config.toml"
			isPlainText = true
		case "README.md":
			path = bundlePrefix + "README.md"
			isPlainText = true
		default:
			// Skip internal artifacts like deployment_state
			continue
		}

		// Get content, unwrapping JSON string if necessary
		content := a.Content
		if isPlainText {
			content = unwrapJSONString(a.Content)
		}

		if err := addToZip(zw, path, content); err != nil {
			return nil, fmt.Errorf("add %s to zip: %w", a.ArtifactType, err)
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// unwrapJSONString unwraps content that was stored for JSONB column.
// Supports two formats:
// 1. NEW: base64 wrapper {"_type":"base64","data":"..."}
// 2. LEGACY: JSON string "content..." (with PostgreSQL normalization issues)
func unwrapJSONString(data []byte) []byte {
	// Try new base64 wrapper format first
	var wrapper struct {
		Type string `json:"_type"`
		Data string `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil && wrapper.Type == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(wrapper.Data)
		if err == nil {
			return decoded
		}
	}

	// Try legacy JSON string format
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		return []byte(s)
	}

	// Legacy fallback: PostgreSQL JSONB normalized \n to real newlines, breaking JSON.
	// Manually unwrap: check for outer quotes and unescape remaining sequences.
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		inner := data[1 : len(data)-1]
		result := make([]byte, 0, len(inner))
		for i := 0; i < len(inner); i++ {
			if inner[i] == '\\' && i+1 < len(inner) {
				switch inner[i+1] {
				case '"':
					result = append(result, '"')
					i++
				case '\\':
					result = append(result, '\\')
					i++
				case 't':
					result = append(result, '\t')
					i++
				case 'r':
					result = append(result, '\r')
					i++
				case 'n':
					result = append(result, '\n')
					i++
				default:
					result = append(result, inner[i])
				}
			} else {
				result = append(result, inner[i])
			}
		}
		return result
	}

	// Not a recognized format, return as-is
	return data
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

// addToZip adds a file to the ZIP archive.
func addToZip(zw *zip.Writer, name string, content []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(content)
	return err
}

// GenerateEnvExample generates the .env.example file content.
// Simplified: just POPSigner API key and L1 RPC - everything else is pre-configured.
func GenerateEnvExample(cfg *DeploymentConfig, addrs *ContractAddresses) string {
	// POPSigner endpoint
	popsignerRPC := cfg.POPSignerEndpoint
	if popsignerRPC == "" {
		popsignerRPC = "https://rpc.popsigner.com"
	}

	// Get dispute game factory address
	disputeGameFactory := addrs.DisputeGameFactoryProxy
	if disputeGameFactory == "" {
		disputeGameFactory = "<set_after_deployment>"
	}

	// Determine L1 beacon URL based on chain ID
	l1BeaconURL := "https://ethereum-sepolia-beacon-api.publicnode.com"
	if cfg.L1ChainID == 1 {
		l1BeaconURL = "https://ethereum-beacon-api.publicnode.com"
	} else if cfg.L1ChainID == 17000 {
		l1BeaconURL = "https://ethereum-holesky-beacon-api.publicnode.com"
	}

	return fmt.Sprintf(`################################################################################
#                         %s - OP Stack Configuration
################################################################################
# Generated by POPKins - https://popkins.popsigner.com
# 
# Copy this file to .env and fill in POPSIGNER_API_KEY.
# All other values are pre-configured for your chain.

################################################################################
# REQUIRED - Fill these in
################################################################################

# Your POPSigner API key (get from dashboard.popsigner.com)
POPSIGNER_API_KEY=<REQUIRED>

# L1 RPC endpoint (Alchemy, Infura, QuickNode, or self-hosted)
L1_RPC_URL=%s

# L1 Beacon API endpoint (required for op-node)
# Public nodes: https://ethereum-sepolia-beacon-api.publicnode.com (Sepolia)
#               https://ethereum-beacon-api.publicnode.com (Mainnet)
#               https://ethereum-holesky-beacon-api.publicnode.com (Holesky)
L1_BEACON_URL=%s

################################################################################
# PRE-CONFIGURED - Usually no changes needed
################################################################################

# L1 RPC type (basic, quicknode, erigon, nethermind, geth)
L1_RPC_KIND=basic

# Your L2 chain ID
CHAIN_ID=%d

# POPSigner RPC endpoint
POPSIGNER_RPC_URL=%s

# Role addresses (managed by POPSigner)
SEQUENCER_ADDRESS=%s
BATCHER_ADDRESS=%s
PROPOSER_ADDRESS=%s

# Contract addresses
DISPUTE_GAME_FACTORY_ADDRESS=%s
`,
		cfg.ChainName,
		cfg.L1RPC,
		l1BeaconURL,
		cfg.ChainID,
		popsignerRPC,
		cfg.SequencerAddress,
		cfg.BatcherAddress,
		cfg.ProposerAddress,
		disputeGameFactory,
	)
}

// GenerateBundleReadme generates the README.md for the artifact bundle.
func GenerateBundleReadme(chainName string, useAltDA bool) string {
	daDescription := "Ethereum calldata"
	if useAltDA {
		daDescription = "Celestia DA"
	}

	readme := fmt.Sprintf(`# %s OP Stack Bundle

This bundle contains everything needed to run your OP Stack rollup with %s.

## Quick Start

1. **Configure environment variables:**
`+"```bash"+`
cp .env.example .env
# Edit .env - fill in your POPSigner API key
`+"```"+`

2. **Start all services:**
`+"```bash"+`
docker compose up -d
`+"```"+`

That's it! The `+"`op-geth-init`"+` service automatically initializes the genesis state on first run.

3. **Check service health:**
`+"```bash"+`
# Check op-geth (should return block number)
curl -s http://localhost:8545 -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# Check op-node (should return sync status)
curl -s http://localhost:9545 -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"optimism_syncStatus","params":[],"id":1}'
`+"```"+`

## Bundle Contents

| File | Description |
|------|-------------|
| `+"`genesis.json`"+` | L2 genesis state |
| `+"`rollup.json`"+` | Rollup configuration |
| `+"`addresses.json`"+` | Deployed L1 contract addresses |
| `+"`docker-compose.yml`"+` | Docker Compose configuration |
| `+"`jwt.txt`"+` | Engine API JWT secret |
| `+"`.env.example`"+` | Environment variable template |
`, chainName, daDescription)

	if useAltDA {
		readme += `
## Celestia DA

Your chain uses **Celestia** as the Data Availability layer via op-alt-da v0.10.0.

POPSigner manages your Celestia signing keys - no local keyring setup required!

### Funding Your Celestia Account

**Testnet (Mocha-4):**
- Your Celestia address is shown in the POPSigner dashboard
- Use the faucet: https://faucet.celestia-mocha.com/

**Mainnet:**
- Transfer TIA tokens to your Celestia address
- Ensure sufficient balance for blob fees (~0.01 TIA per blob)

### Configuration

The ` + "`CELESTIA_NAMESPACE`" + ` in your ` + "`.env`" + ` is auto-generated from your chain ID.
POPSigner handles all Celestia transaction signing via the ` + "`POPSIGNER_CELESTIA_ENDPOINT`" + `.
`
	}

	readme += `
## POPSigner Integration

This bundle uses **POPSigner** for secure transaction signing. Your keys never leave the secure enclave.

All signing operations are handled automatically:
- **Sequencer signing** - L2 block proposals
- **Batcher signing** - L1 batch submissions
- **Proposer signing** - L1 state root proposals
`
	if useAltDA {
		readme += `- **Celestia signing** - DA blob submissions
`
	}

	readme += `
### Setup

1. Log into [POPSigner Dashboard](https://dashboard.popsigner.com)
2. Navigate to your organization's keys
3. Copy the API key to your ` + "`.env`" + ` file (` + "`POPSIGNER_API_KEY`" + `)
4. Your role addresses are pre-configured in the bundle

## Data Directories

Service data is stored in bundle-relative directories:
- ` + "`./op-geth/data/`" + ` - L2 execution layer state
- Logs and metrics available on exposed ports

## Service Ports

| Service | Port | Description |
|---------|------|-------------|
| op-geth | 8545 | JSON-RPC |
| op-geth | 8546 | WebSocket |
| op-geth | 8551 | Engine API (internal) |
| op-node | 9545 | Rollup RPC |
| op-batcher | 8548 | Admin RPC |
| op-proposer | 8560 | Admin RPC |
`
	if useAltDA {
		readme += `| op-alt-da | 3100 | DA Server |
`
	}

	readme += `
## Troubleshooting

**Services not starting:**
` + "```bash" + `
docker compose logs -f op-geth
docker compose logs -f op-node
` + "```" + `

**Sequencer not producing blocks:**
- Check L1 RPC connectivity
- Verify POPSigner API key is correct
- Ensure sequencer address has L1 ETH for gas

**Batcher errors:**
- Check L1 balance for batcher address
- Verify DA layer connectivity

## Support

- [OP Stack Documentation](https://docs.optimism.io)
- [POPSigner Documentation](https://docs.popsigner.io)
- [Celestia Documentation](https://docs.celestia.org)

---
Generated by [POPKins](https://popkins.popsigner.com) Chain Bootstrapping Service
`

	return readme
}
