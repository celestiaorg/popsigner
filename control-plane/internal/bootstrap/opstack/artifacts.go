package opstack

import (
	"archive/zip"
	"bytes"
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
// For non-JSON content (like docker-compose.yml, jwt.txt), wraps as JSON string.
func (e *ArtifactExtractor) saveArtifact(ctx context.Context, deploymentID uuid.UUID, name string, content []byte) error {
	var jsonContent json.RawMessage

	// Check if content is already valid JSON
	if json.Valid(content) {
		jsonContent = content
	} else {
		// Wrap non-JSON content as a JSON string (for YAML, plain text, etc.)
		// This allows storing docker-compose.yml, jwt.txt, README.md, etc.
		escaped, err := json.Marshal(string(content))
		if err != nil {
			return fmt.Errorf("marshal non-JSON content for %s: %w", name, err)
		}
		jsonContent = escaped
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

// unwrapJSONString unwraps a JSON-encoded string back to plain text.
// Used for non-JSON artifacts (docker-compose.yml, jwt.txt, etc.) that were
// stored as JSON strings to satisfy the JSONB column requirement.
//
// IMPORTANT: PostgreSQL JSONB normalizes \n escape sequences to actual newline
// bytes (0x0a), but keeps other escapes (\", \\) intact. This creates malformed
// JSON that json.Unmarshal can't parse. We handle this by manually unescaping
// when json.Unmarshal fails.
func unwrapJSONString(data []byte) []byte {
	// Try to unmarshal as a JSON string (works if PostgreSQL didn't normalize)
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		return []byte(s)
	}

	// PostgreSQL JSONB normalized \n to real newlines, breaking JSON parsing.
	// Manually unwrap: check for outer quotes and unescape remaining sequences.
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		// Strip outer quotes
		inner := data[1 : len(data)-1]

		// Unescape remaining JSON escape sequences that PostgreSQL didn't normalize:
		// - \" -> "
		// - \\ -> \
		// - \t -> tab (in case)
		// - \r -> carriage return (in case)
		result := make([]byte, 0, len(inner))
		for i := 0; i < len(inner); i++ {
			if inner[i] == '\\' && i+1 < len(inner) {
				switch inner[i+1] {
				case '"':
					result = append(result, '"')
					i++ // skip the next char
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
					// Shouldn't happen (PostgreSQL already normalized), but handle it
					result = append(result, '\n')
					i++
				default:
					// Unknown escape, keep as-is
					result = append(result, inner[i])
				}
			} else {
				result = append(result, inner[i])
			}
		}
		return result
	}

	// Not a JSON string, return as-is
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

// GenerateEnvExample generates the .env.example file content with Celestia configuration.
// POPKins only supports Celestia as the DA layer, so Celestia config is always included.
func GenerateEnvExample(cfg *DeploymentConfig, addrs *ContractAddresses) string {
	envContent := fmt.Sprintf(`# =============================================================
# %s OP Stack Configuration
# =============================================================
# Generated by POPKins - https://popkins.popsigner.com

# L1 Configuration
# ---------------------------------------------------------
# Your L1 RPC endpoint (Alchemy, Infura, or self-hosted)
L1_RPC_URL=%s

# POPSigner Configuration
# ---------------------------------------------------------
# POPSigner signing service for secure key management
POPSIGNER_ENDPOINT=%s
POPSIGNER_API_KEY=your_api_key_here

# Role Addresses (from your POPSigner keys)
# ---------------------------------------------------------
SEQUENCER_ADDRESS=%s
BATCHER_ADDRESS=%s
PROPOSER_ADDRESS=%s

# Contract Addresses (from deployment)
# ---------------------------------------------------------
L2_OUTPUT_ORACLE_ADDRESS=%s

# Chain Configuration
# ---------------------------------------------------------
CHAIN_ID=%d
`,
		cfg.ChainName,
		cfg.L1RPC,
		cfg.POPSignerEndpoint,
		cfg.SequencerAddress,
		cfg.BatcherAddress,
		cfg.ProposerAddress,
		addrs.L2OutputOracle,
		cfg.ChainID,
	)

	// POPKins always uses Celestia DA - add Celestia configuration
	envContent += `
# =============================================================
# Celestia DA Configuration (REQUIRED for op-alt-da v0.10.0)
# =============================================================
# See README.md for detailed setup instructions

# Celestia Namespace (hex-encoded, 10 bytes)
# Generate with: openssl rand -hex 10
CELESTIA_NAMESPACE=00000000000000000000000000000000000000acfe

# Core gRPC endpoint for blob submission
# Testnet (mocha-4): consensus-full-mocha-4.celestia-mocha.com:9090
# Mainnet: Use your own consensus node or a public endpoint
CELESTIA_CORE_GRPC=consensus-full-mocha-4.celestia-mocha.com:9090

# Celestia keyring configuration
# The key name in your local Celestia keyring (created with celestia-appd keys add)
CELESTIA_KEY_NAME=my_celes_key

# Celestia network identifier
# Options: mocha-4 (testnet), celestia (mainnet)
CELESTIA_NETWORK=mocha-4
`

	return envContent
}

// GenerateBundleReadme generates the README.md for the artifact bundle.
func GenerateBundleReadme(chainName string, useAltDA bool) string {
	readme := fmt.Sprintf(`# %s OP Stack Bundle

This bundle contains everything needed to run your OP Stack rollup with Celestia DA.

## Quick Start

1. **Configure environment variables:**
   `+"`"+`bash
   cp .env.example .env
   # Edit .env with your configuration
   `+"`"+`

2. **Initialize op-geth with genesis:**
   `+"`"+`bash
   docker compose run --rm op-geth geth init --datadir=/data /config/genesis.json
   `+"`"+`

3. **Start all services:**
   `+"`"+`bash
   docker compose up -d
   `+"`"+`

4. **Check service health:**
   `+"`"+`bash
   # Check op-geth
   curl -s -X POST -H "Content-Type: application/json" \
     --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
     http://localhost:8545

   # Check op-node
   curl -s -X POST -H "Content-Type: application/json" \
     --data '{"jsonrpc":"2.0","method":"optimism_syncStatus","params":[],"id":1}' \
     http://localhost:9545
   `+"`"+`

## Bundle Contents

| File | Description |
|------|-------------|
| `+"`genesis.json`"+` | L2 genesis state |
| `+"`rollup.json`"+` | Rollup configuration |
| `+"`addresses.json`"+` | Deployed L1 contract addresses |
| `+"`docker-compose.yml`"+` | Docker Compose configuration |
| `+"`jwt.txt`"+` | Engine API JWT secret |
| `+"`.env.example`"+` | Environment variable template |
`, chainName)

	if useAltDA {
		readme += `| ` + "`config.toml`" + ` | op-alt-da configuration for Celestia |

## Celestia DA Setup

Your chain uses **Celestia** as the Data Availability layer via op-alt-da v0.10.0.

### Prerequisites

- celestia-appd v6.4.0+ (for keyring management)
- celestia-node v0.28.4+ (optional, for running your own node)

### Step 1: Install Celestia CLI

` + "```bash" + `
# Install celestia-appd for key management
git clone https://github.com/celestiaorg/celestia-app.git
cd celestia-app
git checkout v6.4.0
make install

# Verify installation
celestia-appd version
` + "```" + `

### Step 2: Create Celestia Keyring

` + "```bash" + `
# Create a new key for blob submission
celestia-appd keys add my_celes_key --keyring-backend test

# IMPORTANT: Save the mnemonic phrase securely!
# The address shown is your Celestia address for funding
` + "```" + `

### Step 3: Fund Your Celestia Account

**Testnet (Mocha-4):**
- Use the faucet: https://faucet.celestia-mocha.com/
- Request TIA tokens for your address

**Mainnet:**
- Transfer TIA tokens to your Celestia address
- Ensure sufficient balance for blob fees

### Step 4: Export Keyring to Volume

` + "```bash" + `
# Create the celestia-keys volume directory
mkdir -p ./celestia-keys

# Copy your keyring files (adjust path for your OS)
# Linux/Mac:
cp -r ~/.celestia-app/keyring-test/* ./celestia-keys/

# The docker-compose.yml mounts this directory to /keys in op-alt-da
` + "```" + `

### Step 5: Configure Environment

Edit your ` + "`.env`" + ` file:

` + "```bash" + `
# Generate a unique namespace for your chain
CELESTIA_NAMESPACE=$(openssl rand -hex 10)

# Set Core gRPC endpoint
# Testnet: consensus-full-mocha-4.celestia-mocha.com:9090
CELESTIA_CORE_GRPC=consensus-full-mocha-4.celestia-mocha.com:9090

# Your key name from step 2
CELESTIA_KEY_NAME=my_celes_key

# Network: mocha-4 (testnet) or celestia (mainnet)
CELESTIA_NETWORK=mocha-4
` + "```" + `

### Troubleshooting

**"key not found" error:**
- Ensure the keyring directory is correctly mounted
- Verify the key name matches exactly
- Check file permissions (readable by container)

**"insufficient funds" error:**
- Fund your Celestia address with TIA tokens
- For testnet, use the faucet
- For mainnet, ensure adequate TIA balance

**Connection errors:**
- Verify Core gRPC endpoint is reachable
- Check network configuration (mocha-4 vs celestia)
- Ensure TLS is enabled (core_grpc_tls_enabled = true)
`
	}

	readme += `
## POPSigner Integration

This bundle uses POPSigner for secure transaction signing. Your keys never leave the secure enclave.

1. Log into POPSigner Dashboard: https://dashboard.popsigner.com
2. Navigate to your organization's keys
3. Copy the API key to your ` + "`.env`" + ` file
4. Verify your sequencer, batcher, and proposer addresses match the keys

## Support

- OP Stack Documentation: https://docs.optimism.io
- POPSigner Documentation: https://docs.popsigner.io
- Celestia Documentation: https://docs.celestia.org

---
Generated by POPKins Chain Bootstrapping Service
`

	return readme
}

