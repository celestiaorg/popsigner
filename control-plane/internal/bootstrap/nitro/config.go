package nitro

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// ============================================================================
// Artifact Generator
// ============================================================================

// ArtifactGenerator creates Nitro-specific configuration files from deployment results.
type ArtifactGenerator struct {
	repo repository.Repository
}

// NewArtifactGenerator creates a new artifact generator.
func NewArtifactGenerator(repo repository.Repository) *ArtifactGenerator {
	return &ArtifactGenerator{repo: repo}
}

// GenerateArtifacts creates all Nitro config artifacts after deployment.
func (g *ArtifactGenerator) GenerateArtifacts(
	ctx context.Context,
	deploymentID uuid.UUID,
	config *DeployConfig,
	result *DeployResult,
) error {
	if result == nil || !result.Success {
		return fmt.Errorf("cannot generate artifacts without successful deployment")
	}

	// 1. Generate chain-info.json
	chainInfo, err := GenerateChainInfo(config, result)
	if err != nil {
		return fmt.Errorf("generate chain-info: %w", err)
	}
	if err := g.saveArtifact(ctx, deploymentID, "chain_info", chainInfo); err != nil {
		return err
	}

	// 2. Generate node-config.json
	nodeConfig, err := GenerateNodeConfig(config, result)
	if err != nil {
		return fmt.Errorf("generate node-config: %w", err)
	}
	if err := g.saveArtifact(ctx, deploymentID, "node_config", nodeConfig); err != nil {
		return err
	}

	// 3. Generate core-contracts.json
	coreContracts, err := GenerateCoreContractsArtifact(result)
	if err != nil {
		return fmt.Errorf("generate core-contracts: %w", err)
	}
	if err := g.saveArtifact(ctx, deploymentID, "core_contracts", coreContracts); err != nil {
		return err
	}

	// 4. Generate docker-compose.yaml
	dockerCompose := GenerateDockerCompose(config, result)
	if err := g.saveTextArtifact(ctx, deploymentID, "docker_compose", dockerCompose); err != nil {
		return err
	}

	// 5. Generate celestia-config.toml
	celestiaConfig := GenerateCelestiaConfig(config, result)
	if err := g.saveTextArtifact(ctx, deploymentID, "celestia_config", celestiaConfig); err != nil {
		return err
	}

	// 6. Generate .env.example
	envExample := GenerateEnvExample(config, result)
	if err := g.saveTextArtifact(ctx, deploymentID, "env_example", envExample); err != nil {
		return err
	}

	// 7. Generate README.md
	readme := GenerateReadme(config, result)
	if err := g.saveTextArtifact(ctx, deploymentID, "readme", readme); err != nil {
		return err
	}

	// 8. Save mTLS certificates (for L1 signing)
	// These are the certificates used during deployment and are included in the bundle
	// so users don't have to manually copy them
	if config.ClientCert != "" {
		if err := g.saveTextArtifact(ctx, deploymentID, "client_cert", config.ClientCert); err != nil {
			return err
		}
	}
	if config.ClientKey != "" {
		if err := g.saveTextArtifact(ctx, deploymentID, "client_key", config.ClientKey); err != nil {
			return err
		}
	}
	if config.CaCert != "" {
		if err := g.saveTextArtifact(ctx, deploymentID, "ca_cert", config.CaCert); err != nil {
			return err
		}
	}

	return nil
}

// saveTextArtifact saves a plain text artifact (not JSON).
// For non-JSON content (like docker-compose.yaml, certificates), wraps as base64 in a JSON object.
// This avoids PostgreSQL JSONB normalization issues with escape sequences.
func (g *ArtifactGenerator) saveTextArtifact(
	ctx context.Context,
	deploymentID uuid.UUID,
	artifactType string,
	content string,
) error {
	// Wrap non-JSON content as base64 in a JSON object.
	// This avoids PostgreSQL JSONB escape sequence normalization issues.
	wrapper := struct {
		Type string `json:"_type"`
		Data string `json:"data"`
	}{
		Type: "base64",
		Data: base64.StdEncoding.EncodeToString([]byte(content)),
	}
	encoded, err := json.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("marshal text content for %s: %w", artifactType, err)
	}

	artifact := &repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ArtifactType: artifactType,
		Content:      encoded,
		CreatedAt:    time.Now(),
	}

	return g.repo.SaveArtifact(ctx, artifact)
}

func (g *ArtifactGenerator) saveArtifact(
	ctx context.Context,
	deploymentID uuid.UUID,
	artifactType string,
	data interface{},
) error {
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", artifactType, err)
	}

	artifact := &repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ArtifactType: artifactType,
		Content:      content,
		CreatedAt:    time.Now(),
	}

	return g.repo.SaveArtifact(ctx, artifact)
}

// ============================================================================
// Chain Info (chain-info.json)
// ============================================================================

// ChainInfo is the structure expected by Nitro's --chain.info-files flag.
// This is an array with a single chain configuration.
type ChainInfo []ChainInfoEntry

// ChainInfoEntry represents a single chain's info in the array.
type ChainInfoEntry struct {
	ChainID       uint64                 `json:"chain-id"`
	ParentChainID uint64                 `json:"parent-chain-id"`
	ChainName     string                 `json:"chain-name"`
	ChainConfig   map[string]interface{} `json:"chain-config"`
	Rollup        RollupInfo             `json:"rollup"`
}

// RollupInfo contains rollup contract addresses.
type RollupInfo struct {
	Bridge                 string `json:"bridge"`
	Inbox                  string `json:"inbox"`
	SequencerInbox         string `json:"sequencer-inbox"`
	Rollup                 string `json:"rollup"`
	ValidatorWalletCreator string `json:"validator-wallet-creator,omitempty"`
	DeployedAt             uint64 `json:"deployed-at"`
}

// GenerateChainInfo creates the chain-info.json content.
func GenerateChainInfo(config *DeployConfig, result *DeployResult) (*ChainInfo, error) {
	if result == nil || !result.Success {
		return nil, fmt.Errorf("cannot generate chain info without successful deployment")
	}

	if result.CoreContracts == nil {
		return nil, fmt.Errorf("deployment result missing core contracts")
	}

	chainConfig := buildChainConfig(config)

	info := ChainInfo{
		{
			ChainID:       uint64(config.ChainID),
			ParentChainID: uint64(config.ParentChainID),
			ChainName:     config.ChainName,
			ChainConfig:   chainConfig,
			Rollup: RollupInfo{
				Bridge:                 result.CoreContracts.Bridge,
				Inbox:                  result.CoreContracts.Inbox,
				SequencerInbox:         result.CoreContracts.SequencerInbox,
				Rollup:                 result.CoreContracts.Rollup,
				ValidatorWalletCreator: result.CoreContracts.ValidatorWalletCreator,
				DeployedAt:             uint64(result.CoreContracts.DeployedAtBlockNumber),
			},
		},
	}

	return &info, nil
}

// buildChainConfig creates the chain-config object for Nitro.
func buildChainConfig(config *DeployConfig) map[string]interface{} {
	return map[string]interface{}{
		"chainId":             config.ChainID,
		"homesteadBlock":      0,
		"daoForkBlock":        nil,
		"daoForkSupport":      true,
		"eip150Block":         0,
		"eip155Block":         0,
		"eip158Block":         0,
		"byzantiumBlock":      0,
		"constantinopleBlock": 0,
		"petersburgBlock":     0,
		"istanbulBlock":       0,
		"muirGlacierBlock":    0,
		"berlinBlock":         0,
		"londonBlock":         0,
		"clique": map[string]interface{}{
			"period": 0,
			"epoch":  0,
		},
		"arbitrum": map[string]interface{}{
			"EnableArbOS":           true,
			"AllowDebugPrecompiles": false,
			// DataAvailabilityCommittee:
			//   false = Rollup mode OR External DA provider (Celestia)
			//   true  = AnyTrust DAC mode
			// For Celestia, we use external-provider flags with DAC=false
			"DataAvailabilityCommittee": config.DataAvailability == "anytrust",
			"InitialArbOSVersion":       20,
			"InitialChainOwner":         config.Owner,
			"GenesisBlockNum":           0,
		},
	}
}

// ============================================================================
// Node Config (node-config.json)
// ============================================================================

// NitroNodeConfig represents the full Nitro node configuration.
type NitroNodeConfig struct {
	ParentChain ParentChainConfig `json:"parent-chain"`
	Chain       ChainNodeConfig   `json:"chain"`
	HTTP        HTTPConfig        `json:"http"`
	WS          WSConfig          `json:"ws"`
	Node        NodeSettings      `json:"node"`
	Execution   *ExecutionConfig  `json:"execution,omitempty"`
	Metrics     MetricsConfig     `json:"metrics"`
}

// ParentChainConfig contains parent chain connection settings.
type ParentChainConfig struct {
	Connection ConnectionConfig `json:"connection"`
}

// ConnectionConfig contains RPC connection settings.
type ConnectionConfig struct {
	URL string `json:"url"`
}

// ChainNodeConfig contains chain-specific node settings.
type ChainNodeConfig struct {
	ID        uint64 `json:"id"`
	InfoFiles string `json:"info-files,omitempty"`
}

// HTTPConfig contains HTTP RPC settings.
type HTTPConfig struct {
	Addr       string   `json:"addr"`
	Port       int      `json:"port"`
	VHosts     string   `json:"vhosts"`
	Corsdomain string   `json:"corsdomain"`
	API        []string `json:"api"`
}

// WSConfig contains WebSocket RPC settings.
type WSConfig struct {
	Addr string   `json:"addr"`
	Port int      `json:"port"`
	API  []string `json:"api"`
}

// NodeSettings contains node-specific settings.
type NodeSettings struct {
	Sequencer        SequencerConfig   `json:"sequencer"`
	BatchPoster      BatchPosterConfig `json:"batch-poster"`
	Staker           StakerConfig      `json:"staker"`
	DataAvailability *DAConfig         `json:"data-availability,omitempty"`
	DelayedSequencer DelayedSeqConfig  `json:"delayed-sequencer"`
}

// SequencerConfig contains sequencer settings.
type SequencerConfig struct {
	Enable bool `json:"enable"`
}

// BatchPosterConfig contains batch poster settings.
type BatchPosterConfig struct {
	Enable     bool             `json:"enable"`
	DataPoster DataPosterConfig `json:"data-poster"`
}

// DataPosterConfig contains data poster settings.
type DataPosterConfig struct {
	ExternalSigner ExternalSignerConfig `json:"external-signer"`
}

// ExternalSignerConfig is the POPSigner mTLS configuration.
type ExternalSignerConfig struct {
	URL              string `json:"url"`
	Method           string `json:"method"`
	ClientCert       string `json:"client-cert"`
	ClientPrivateKey string `json:"client-private-key"`
}

// StakerConfig contains staker/validator settings.
type StakerConfig struct {
	Enable     bool             `json:"enable"`
	Strategy   string           `json:"strategy,omitempty"`
	DataPoster DataPosterConfig `json:"data-poster"`
}

// DAConfig contains data availability settings.
type DAConfig struct {
	Enable             bool            `json:"enable"`
	SequencerInboxAddr string          `json:"sequencer-inbox-address"`
	Celestia           *CelestiaConfig `json:"celestia,omitempty"`
}

// CelestiaConfig contains Celestia-specific DA settings.
type CelestiaConfig struct {
	Enable    bool   `json:"enable"`
	ServerURL string `json:"rpc-url"`
}

// DelayedSeqConfig contains delayed sequencer settings.
type DelayedSeqConfig struct {
	Enable bool `json:"enable"`
}

// ExecutionConfig contains execution settings.
type ExecutionConfig struct {
	ForwardingTarget string `json:"forwarding-target,omitempty"`
}

// MetricsConfig contains metrics settings.
type MetricsConfig struct {
	Server MetricsServerConfig `json:"server"`
}

// MetricsServerConfig contains metrics server settings.
type MetricsServerConfig struct {
	Addr string `json:"addr"`
	Port int    `json:"port"`
}

// GenerateNodeConfig creates the node-config.json content.
func GenerateNodeConfig(config *DeployConfig, result *DeployResult) (*NitroNodeConfig, error) {
	// Create POPSigner external signer config template
	externalSigner := ExternalSignerConfig{
		URL:              "${POPSIGNER_MTLS_URL}",
		Method:           "eth_signTransaction",
		ClientCert:       "/certs/client.crt",
		ClientPrivateKey: "/certs/client.key",
	}

	nodeCfg := &NitroNodeConfig{
		ParentChain: ParentChainConfig{
			Connection: ConnectionConfig{
				URL: "${L1_RPC_URL}", // Templated - filled by user
			},
		},
		Chain: ChainNodeConfig{
			ID:        uint64(config.ChainID),
			InfoFiles: "/config/chain-info.json",
		},
		HTTP: HTTPConfig{
			Addr:       "0.0.0.0",
			Port:       8547,
			VHosts:     "*",
			Corsdomain: "*",
			API:        []string{"eth", "net", "web3", "arb", "debug"},
		},
		WS: WSConfig{
			Addr: "0.0.0.0",
			Port: 8548,
			API:  []string{"eth", "net", "web3"},
		},
		Node: NodeSettings{
			Sequencer: SequencerConfig{
				Enable: true,
			},
			BatchPoster: BatchPosterConfig{
				Enable: true,
				DataPoster: DataPosterConfig{
					ExternalSigner: externalSigner,
				},
			},
			Staker: StakerConfig{
				Enable:   true,
				Strategy: "MakeNodes",
				DataPoster: DataPosterConfig{
					ExternalSigner: externalSigner,
				},
			},
			DelayedSequencer: DelayedSeqConfig{
				Enable: true,
			},
		},
		Metrics: MetricsConfig{
			Server: MetricsServerConfig{
				Addr: "0.0.0.0",
				Port: 9642,
			},
		},
	}

	// Add Celestia DA configuration by default
	// POPSigner deployments use Celestia DA unless explicitly set to "rollup"
	if config.DataAvailability != "rollup" {
		sequencerInbox := ""
		if result != nil && result.CoreContracts != nil {
			sequencerInbox = result.CoreContracts.SequencerInbox
		}

		nodeCfg.Node.DataAvailability = &DAConfig{
			Enable:             true,
			SequencerInboxAddr: sequencerInbox,
			Celestia: &CelestiaConfig{
				Enable:    true,
				ServerURL: "${CELESTIA_RPC_URL}",
			},
		}
	}

	return nodeCfg, nil
}

// GenerateValidatorNodeConfig creates a node-config.json for a validator-only node.
func GenerateValidatorNodeConfig(config *DeployConfig, result *DeployResult) (*NitroNodeConfig, error) {
	baseCfg, err := GenerateNodeConfig(config, result)
	if err != nil {
		return nil, err
	}

	// Disable sequencer for validator
	baseCfg.Node.Sequencer.Enable = false
	baseCfg.Node.BatchPoster.Enable = false

	// Add forwarding target for RPC
	baseCfg.Execution = &ExecutionConfig{
		ForwardingTarget: "${SEQUENCER_URL}",
	}

	return baseCfg, nil
}

// ============================================================================
// Core Contracts (core-contracts.json)
// ============================================================================

// CoreContractsArtifact is the formatted core contracts artifact for output.
type CoreContractsArtifact struct {
	Rollup                 string `json:"rollup"`
	Inbox                  string `json:"inbox"`
	Outbox                 string `json:"outbox"`
	Bridge                 string `json:"bridge"`
	SequencerInbox         string `json:"sequencerInbox"`
	RollupEventInbox       string `json:"rollupEventInbox,omitempty"`
	ChallengeManager       string `json:"challengeManager,omitempty"`
	AdminProxy             string `json:"adminProxy"`
	UpgradeExecutor        string `json:"upgradeExecutor,omitempty"`
	ValidatorWalletCreator string `json:"validatorWalletCreator,omitempty"`
	NativeToken            string `json:"nativeToken,omitempty"`
	DeployedAtBlockNumber  uint64 `json:"deployedAtBlockNumber"`
	TransactionHash        string `json:"transactionHash"`
}

// GenerateCoreContractsArtifact creates the core-contracts.json content.
func GenerateCoreContractsArtifact(result *DeployResult) (*CoreContractsArtifact, error) {
	if result == nil || !result.Success {
		return nil, fmt.Errorf("cannot generate core contracts without successful deployment")
	}

	if result.CoreContracts == nil {
		return nil, fmt.Errorf("deployment result missing core contracts")
	}

	return &CoreContractsArtifact{
		Rollup:                 result.CoreContracts.Rollup,
		Inbox:                  result.CoreContracts.Inbox,
		Outbox:                 result.CoreContracts.Outbox,
		Bridge:                 result.CoreContracts.Bridge,
		SequencerInbox:         result.CoreContracts.SequencerInbox,
		RollupEventInbox:       result.CoreContracts.RollupEventInbox,
		ChallengeManager:       result.CoreContracts.ChallengeManager,
		AdminProxy:             result.CoreContracts.AdminProxy,
		UpgradeExecutor:        result.CoreContracts.UpgradeExecutor,
		ValidatorWalletCreator: result.CoreContracts.ValidatorWalletCreator,
		NativeToken:            result.CoreContracts.NativeToken,
		DeployedAtBlockNumber:  uint64(result.CoreContracts.DeployedAtBlockNumber),
		TransactionHash:        result.TransactionHash,
	}, nil
}

// ============================================================================
// Helpers
// ============================================================================

// GenerateAllArtifacts is a convenience function that generates all artifacts
// without requiring a repository (returns raw JSON).
func GenerateAllArtifacts(config *DeployConfig, result *DeployResult) (map[string][]byte, error) {
	artifacts := make(map[string][]byte)

	// Chain info
	chainInfo, err := GenerateChainInfo(config, result)
	if err != nil {
		return nil, fmt.Errorf("generate chain-info: %w", err)
	}
	chainInfoJSON, err := json.MarshalIndent(chainInfo, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal chain-info: %w", err)
	}
	artifacts["chain-info.json"] = chainInfoJSON

	// Node config
	nodeConfig, err := GenerateNodeConfig(config, result)
	if err != nil {
		return nil, fmt.Errorf("generate node-config: %w", err)
	}
	nodeConfigJSON, err := json.MarshalIndent(nodeConfig, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal node-config: %w", err)
	}
	artifacts["node-config.json"] = nodeConfigJSON

	// Core contracts
	coreContracts, err := GenerateCoreContractsArtifact(result)
	if err != nil {
		return nil, fmt.Errorf("generate core-contracts: %w", err)
	}
	coreContractsJSON, err := json.MarshalIndent(coreContracts, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal core-contracts: %w", err)
	}
	artifacts["core-contracts.json"] = coreContractsJSON

	return artifacts, nil
}

// ============================================================================
// Docker Compose Generator
// ============================================================================

// GenerateDockerCompose creates a docker-compose.yaml for Nitro + Celestia DA.
func GenerateDockerCompose(config *DeployConfig, result *DeployResult) string {
	return fmt.Sprintf(`version: '3.8'

# =============================================================================
# Nitro + Celestia DA Docker Compose
# Chain: %s (ID: %d) on Parent Chain %d
# Uses ClientTX: Direct connection to Celestia infrastructure (no local node)
# =============================================================================

services:
  # ===========================================================================
  # Celestia DAS Server
  # Translates between Nitro DA provider protocol and Celestia
  # Uses ClientTX with remote signer (popsigner) for blob submission
  # ===========================================================================
  celestia-das-server:
    image: ${NITRO_DAS_IMAGE:-ghcr.io/celestiaorg/nitro-das-celestia:v0.7.0}
    container_name: celestia-das-server
    restart: unless-stopped
    command:
      - --config
      - /config/celestia-config.toml
    ports:
      - "9876:9876" # DA provider RPC (Nitro connects here)
      - "6060:6060" # Metrics (optional)
    volumes:
      - ./config/celestia-config.toml:/config/celestia-config.toml:ro
    environment:
      # Remote signer (popsigner) credentials
      - POPSIGNER_API_KEY=${POPSIGNER_CELESTIA_API_KEY}
      # Parent chain RPC for Blobstream validation (fraud proofs)
      - ETH_RPC_URL=${L1_RPC_URL}
    networks:
      - nitro-network

  # ===========================================================================
  # Nitro Sequencer Node (Batch Poster + Validator)
  # Uses official Offchain Labs image with DA API interface (PR #3949, #3237)
  #
  # Image options:
  # 1. Use latest release with DA support: offchainlabs/nitro-node:v3.9.0 (when released)
  # 2. Build from your local source (v3.9.0-rc.1): see README
  # 3. Use Celestia fork (legacy): ghcr.io/celestiaorg/nitro:v3.6.8
  # ===========================================================================
  nitro-sequencer:
    # TODO: Update to official release once v3.9.0 is published on Docker Hub
    # Your local repo is at v3.9.0-rc.1 - you may need to build this image locally
    # docker build -t nitro-node:local --target nitro-node .
    image: ${NITRO_IMAGE:-offchainlabs/nitro-node:v3.5.4-8de7ff5}
    container_name: nitro-sequencer
    restart: unless-stopped
    depends_on:
      celestia-das-server:
        condition: service_started
    ports:
      - "8547:8547" # HTTP RPC
      - "8548:8548" # WebSocket RPC
      - "9642:9642" # Metrics
      - "9644:9644" # Feed
    volumes:
      - ./config:/config:ro
      - ./certs:/certs:ro
      - nitro-data:/home/user/.arbitrum
      - nitro-keystore:/home/user/l1keystore
    environment:
      # L1 Connections - WSS is recommended for sequencers for real-time updates
      - L1_RPC_URL=${L1_RPC_URL} # Can be HTTP or WSS (e.g., wss://sepolia.infura.io/ws/v3/KEY)
      - L1_BEACON_URL=${L1_BEACON_URL} # Beacon Chain API (HTTP)
      - POPSIGNER_MTLS_URL=${POPSIGNER_MTLS_URL}
    command:
      # -------------------------------------------------------------------------
      # Core Chain Configuration
      # -------------------------------------------------------------------------
      - --chain.id=%d
      - --chain.name=%s
      - --chain.info-files=/config/chain-info.json
      # -------------------------------------------------------------------------
      # Parent Chain (L1) Connection
      # URL can be HTTP or WSS - WSS recommended for sequencers:
      #   HTTP: https://sepolia.infura.io/v3/YOUR_KEY
      #   WSS:  wss://sepolia.infura.io/ws/v3/YOUR_KEY
      # -------------------------------------------------------------------------
      - --parent-chain.connection.url=${L1_RPC_URL}
      # Beacon Chain API for EIP-4844 blob data (always HTTP)
      - --parent-chain.blob-client.beacon-url=${L1_BEACON_URL}
      # -------------------------------------------------------------------------
      # HTTP/WS RPC Configuration
      # -------------------------------------------------------------------------
      - --http.addr=0.0.0.0
      - --http.port=8547
      - --http.api=eth,net,web3,arb,debug
      - --http.vhosts=*
      - --http.corsdomain=*
      - --ws.addr=0.0.0.0
      - --ws.port=8548
      - --ws.api=eth,net,web3
      - --ws.origins=*
      # -------------------------------------------------------------------------
      # Sequencer Configuration
      # For single-sequencer setup, disable coordinator requirement
      # -------------------------------------------------------------------------
      - --node.sequencer=true
      - --execution.sequencer.enable=true
      - --node.delayed-sequencer.enable=true
      - --node.dangerous.no-sequencer-coordinator=true
      # -------------------------------------------------------------------------
      # Celestia DA Provider Configuration (PR #3949)
      # External DA provider connects to celestia-das-server
      # -------------------------------------------------------------------------
      - --node.da.external-provider.enable=true
      - --node.da.external-provider.with-writer=true
      - --node.da.external-provider.rpc.url=http://celestia-das-server:9876
      # -------------------------------------------------------------------------
      # Batch Poster Configuration (uses PopSigner for L1 tx signing)
      # -------------------------------------------------------------------------
      - --node.batch-poster.enable=true
      - --node.batch-poster.data-poster.external-signer.url=${POPSIGNER_MTLS_URL}
      - --node.batch-poster.data-poster.external-signer.method=eth_signTransaction
      - --node.batch-poster.data-poster.external-signer.client-cert=/certs/client.crt
      - --node.batch-poster.data-poster.external-signer.client-private-key=/certs/client.key
      # -------------------------------------------------------------------------
      # Staker/Validator Configuration (uses PopSigner for L1 tx signing)
      # -------------------------------------------------------------------------
      - --node.staker.enable=true
      - --node.staker.strategy=MakeNodes
      - --node.staker.data-poster.external-signer.url=${POPSIGNER_MTLS_URL}
      - --node.staker.data-poster.external-signer.method=eth_signTransaction
      - --node.staker.data-poster.external-signer.client-cert=/certs/client.crt
      - --node.staker.data-poster.external-signer.client-private-key=/certs/client.key
      # -------------------------------------------------------------------------
      # Feed Output (for full nodes to subscribe)
      # -------------------------------------------------------------------------
      - --node.feed.output.enable=true
      - --node.feed.output.addr=0.0.0.0
      - --node.feed.output.port=9644
      # -------------------------------------------------------------------------
      # Metrics
      # -------------------------------------------------------------------------
      - --metrics
      - --metrics-server.addr=0.0.0.0
      - --metrics-server.port=9642
    networks:
      - nitro-network

networks:
  nitro-network:
    driver: bridge

volumes:
  nitro-data:
  nitro-keystore:
`, config.ChainName, config.ChainID, config.ParentChainID, config.ChainID, config.ChainName)
}

// ============================================================================
// Celestia Config Generator
// ============================================================================

// GenerateCelestiaConfig creates a celestia-config.toml for the DAS server.
func GenerateCelestiaConfig(config *DeployConfig, result *DeployResult) string {
	// Determine network and blobstream address based on parent chain
	celestiaNetwork := "mocha-4"
	blobstreamAddr := "0xF0c6429ebAB2e7DC6e05DaFB61128bE21f13cb1e" // Sepolia Blobstream SP1
	if config.ParentChainID == 1 {
		celestiaNetwork = "celestia"
		blobstreamAddr = "0x7Cf3876F681Dbb6EdA8f6FfC45D66B996Df08fAe" // Mainnet Blobstream
	}

	return fmt.Sprintf(`# =============================================================================
# Celestia DAS Server Configuration
# Chain: %s (ID: %d) on Parent Chain %d
#
# Uses ClientTX architecture:
# - Reader: Connects to Celestia DA Bridge node (JSON-RPC) for blob reads
# - Writer: Connects to Celestia Core node (gRPC) for blob submission
# - No local Celestia node required!
# =============================================================================

[server]
rpc_addr = "0.0.0.0"
rpc_port = 9876
rpc_body_limit = 0
read_timeout = "30s"
read_header_timeout = "10s"
write_timeout = "30s"
idle_timeout = "120s"

[celestia]
# Namespace ID for blob operations (hex string)
# IMPORTANT: Use a unique namespace for your chain!
# Generate one at: https://docs.celestia.org/tutorials/node-tutorial#namespaces
namespace_id = "YOUR_UNIQUE_NAMESPACE_HEX"

# Gas settings for blob transactions
gas_price = 0.01
gas_multiplier = 1.01

# Celestia network: "celestia" for mainnet, "mocha-4" for testnet
network = "%s"

# Enable blob submission (writer mode) - required for batch poster
with_writer = true
noop_writer = false

# Cache cleanup interval
cache_time = "30m"

# -----------------------------------------------------------------------------
# Reader configuration
# Connects to Celestia DA Bridge node (JSON-RPC) for reading blobs
# Uses public Celestia infrastructure - no local node needed!
# -----------------------------------------------------------------------------
[celestia.reader]
# Public Mocha testnet DA Bridge node
# You can also use providers like QuickNode: https://www.quicknode.com/docs/celestia
rpc = "https://YOUR_CELESTIA_RPC_ENDPOINT"
# Auth token (if using a provider like QuickNode)
auth_token = ""
enable_tls = true

# -----------------------------------------------------------------------------
# Writer configuration
# Connects to Celestia Core node (gRPC) for blob submission
# Uses ClientTX - direct gRPC connection to consensus node
# -----------------------------------------------------------------------------
[celestia.writer]
# Public Mocha testnet consensus node gRPC
core_grpc = "YOUR_CELESTIA_GRPC_ENDPOINT:9090"
core_token = ""
enable_tls = true

# -----------------------------------------------------------------------------
# Signer configuration
# Uses remote signing via PopSigner for Celestia transaction signing
# This is separate from the L1 PopSigner used by Nitro!
# -----------------------------------------------------------------------------
[celestia.signer]
type = "remote"

[celestia.signer.remote]
# PopSigner API key for Celestia signing (from environment variable)
api_key = "${POPSIGNER_CELESTIA_API_KEY}"
# PopSigner Key ID for your Celestia key
key_id = "${POPSIGNER_CELESTIA_KEY_ID}"
# Custom PopSigner endpoint (optional, leave empty for default)
base_url = ""

# Alternative: Local signer (if not using PopSigner for Celestia)
# Uncomment below and comment out the remote section above
# [celestia.signer]
# type = "local"
# [celestia.signer.local]
# key_name = "%s-celestia-key"
# key_path = ""  # Uses default: ~/.celestia-light-mocha-4/keys
# backend = "test"

# -----------------------------------------------------------------------------
# Retry configuration for failed blob operations
# -----------------------------------------------------------------------------
[celestia.retry]
max_retries = 5
initial_backoff = "10s"
max_backoff = "120s"
backoff_factor = 2.0

# -----------------------------------------------------------------------------
# Validator configuration for Blobstream proof validation (FRAUD PROOFS)
# This is CRITICAL for validators - enables fraud proofs with Celestia DA
# PR #3237: Custom DA Complete Fraud Proof Support
# -----------------------------------------------------------------------------
[celestia.validator]
# Parent chain RPC endpoint
# This is used to query Blobstream contract for data root attestations
eth_rpc = "${ETH_RPC_URL}"

# Blobstream X contract address
# Check latest address: https://docs.celestia.org/how-to-guides/blobstream#deployed-contracts
blobstream_addr = "%s"

# Seconds between Blobstream event polling (for catching up proofs)
sleep_time = 3600

# -----------------------------------------------------------------------------
# Fallback to Arbitrum AnyTrust DAS (optional)
# Enable if you want to fall back to AnyTrust when Celestia fails
# -----------------------------------------------------------------------------
[fallback]
enabled = false
das_rpc = ""

# -----------------------------------------------------------------------------
# Logging configuration
# -----------------------------------------------------------------------------
[logging]
level = "INFO"
type = "plaintext"

# -----------------------------------------------------------------------------
# Metrics and profiling configuration
# -----------------------------------------------------------------------------
[metrics]
enabled = true
addr = "0.0.0.0"
port = 6060
pprof = false
pprof_addr = "127.0.0.1"
pprof_port = 6061
`, config.ChainName, config.ChainID, config.ParentChainID, celestiaNetwork, config.ChainName, blobstreamAddr)
}

// ============================================================================
// Environment Example Generator
// ============================================================================

// GenerateEnvExample creates a .env.example file.
func GenerateEnvExample(config *DeployConfig, result *DeployResult) string {
	// Determine parent chain name
	parentChainName := "Sepolia"
	beaconURL := "https://ethereum-sepolia-beacon-api.publicnode.com"
	rpcExample := "wss://sepolia.infura.io/ws/v3/YOUR_KEY"
	if config.ParentChainID == 1 {
		parentChainName = "Mainnet"
		beaconURL = "https://ethereum-mainnet-beacon-api.publicnode.com"
		rpcExample = "wss://mainnet.infura.io/ws/v3/YOUR_KEY"
	}

	return fmt.Sprintf(`# =============================================================================
# %s Nitro + Celestia DA Environment Configuration
# Chain ID: %d | Parent Chain: %s (%d)
# =============================================================================

# =============================================================================
# PARENT CHAIN (%s) - L1 CONNECTIONS
# =============================================================================

# Main L1 RPC endpoint
# WSS is RECOMMENDED for sequencers (real-time updates, lower latency)
# Examples:
#   Infura WSS:  wss://%s.infura.io/ws/v3/YOUR_KEY
#   Alchemy WSS: wss://eth-%s.g.alchemy.com/v2/YOUR_KEY
#   Infura HTTP: https://%s.infura.io/v3/YOUR_KEY
#
L1_RPC_URL=%s

# Beacon Chain API for EIP-4844 blob data (always HTTP)
# Public options:
#   %s
#
L1_BEACON_URL=%s

# =============================================================================
# POPSIGNER FOR L1 (%s) - Batch Poster & Validator Signing
# Used for signing L1 transactions (batch submissions, staking)
# =============================================================================

# PopSigner mTLS endpoint for transaction signing
POPSIGNER_MTLS_URL=https://rpc-mtls.popsigner.com

# Note: mTLS certificates are included in ./certs/
#   - ./certs/client.crt  (auto-generated during deployment)
#   - ./certs/client.key  (auto-generated during deployment)
#   - ./certs/ca.crt      (CA certificate for verification)

# =============================================================================
# POPSIGNER FOR CELESTIA - Blob Submission Signing
# Used for signing Celestia blob transactions (SEPARATE from L1 signer!)
# =============================================================================

# PopSigner API key for Celestia
POPSIGNER_CELESTIA_API_KEY=REPLACE_WITH_YOUR_CELESTIA_API_KEY

# PopSigner Key ID for your Celestia signing key
POPSIGNER_CELESTIA_KEY_ID=REPLACE_WITH_YOUR_CELESTIA_KEY_ID

# =============================================================================
# NITRO IMAGE
# =============================================================================

# After running 'make docker' in nitro repo, use:
NITRO_IMAGE=nitro-node-dev:latest
NITRO_DAS_IMAGE=ghcr.io/celestiaorg/nitro-das-celestia:v0.7.0

# Alternative options:
#   nitro-node:local     (if you tagged it with :local)
#   nitro-node-slim      (minimal image)
#   ghcr.io/celestiaorg/nitro:v3.6.8  (Celestia fork, legacy)

# =============================================================================
# OPTIONAL: Additional Configuration
# =============================================================================

# Uncomment if you need custom Celestia namespace
# CELESTIA_NAMESPACE_ID=YOUR_NAMESPACE_HEX

# Uncomment for custom log level
# LOG_LEVEL=INFO
`, config.ChainName, config.ChainID, parentChainName, config.ParentChainID,
		parentChainName,
		strings.ToLower(parentChainName), strings.ToLower(parentChainName), strings.ToLower(parentChainName),
		rpcExample, beaconURL, beaconURL, parentChainName)
}

// ============================================================================
// README Generator
// ============================================================================

// GenerateReadme creates a README.md with deployment instructions.
func GenerateReadme(config *DeployConfig, result *DeployResult) string {
	contracts := result.CoreContracts
	if contracts == nil {
		contracts = &CoreContracts{}
	}

	// Determine parent chain name
	parentChainName := "Sepolia"
	celestiaNetwork := "Mocha Testnet"
	if config.ParentChainID == 1 {
		parentChainName = "Mainnet"
		celestiaNetwork = "Mainnet"
	}

	return fmt.Sprintf(`# %s Nitro + Celestia DA Deployment

This bundle deploys an Arbitrum Orbit chain (%s) with Celestia DA on %s.

## Architecture

`+"`"+``+"`"+``+"`"+`
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              Docker Compose                                      │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────┐              ┌─────────────────────┐                       │
│  │  Nitro Sequencer │  ─────────► │  Celestia DAS Server │                       │
│  │  (offchainlabs/  │  daprovider │  (nitro-das-celestia)│                       │
│  │   nitro-node)    │  RPC        │  ClientTX            │                       │
│  └────────┬─────────┘              └──────────┬──────────┘                       │
│           │                                   │                                  │
└───────────┼───────────────────────────────────┼──────────────────────────────────┘
            │                                   │
            │ L1 RPC                            │ gRPC (write) + JSON-RPC (read)
            ▼                                   ▼
    ┌───────────────┐               ┌───────────────────────────┐
    │   %s          │               │   Celestia %s             │
    │   (L1)        │◄──────────────│   - Consensus (gRPC): writes  │
    │               │  Blobstream   │   - DA Bridge (RPC): reads    │
    │   Blobstream X│               └───────────────────────────┘
    │   Contract    │
    └───────────────┘
`+"`"+``+"`"+``+"`"+`

**Key Design: No Local Celestia Node Required!**

Uses ClientTX architecture:

- **Writes**: Direct gRPC to Celestia consensus nodes
- **Reads**: JSON-RPC to Celestia DA Bridge nodes
- **Signing**: Remote signing via PopSigner

## Chain Configuration

| Parameter       | Value                                      |
| --------------- | ------------------------------------------ |
| Chain ID        | %d                                         |
| Chain Name      | %s                                         |
| Parent Chain    | %s (%d)                                    |
| Rollup Contract | %s |
| Sequencer Inbox | %s |
| Bridge          | %s |

## Prerequisites

1. **Docker & Docker Compose** installed
2. **PopSigner credentials** (two sets):
   - **L1 (%s)**: For batch poster and validator transactions
   - **Celestia**: For blob submission transactions
3. **%s RPC** endpoint
4. **%s Beacon RPC** endpoint
5. **TIA tokens** on Celestia %s (for your PopSigner Celestia key)

## Directory Structure

`+"`"+``+"`"+``+"`"+`
%s-nitro-bundle/
├── config/
│   ├── chain-info.json          # Chain configuration (from SDK)
│   ├── core-contracts.json      # Deployed contract addresses
│   ├── node-config.json         # Original node config (reference only)
│   └── celestia-config.toml     # Celestia DAS server config
├── certs/                       # PopSigner mTLS certificates (for L1)
│   ├── client.crt
│   └── client.key
├── docker-compose.yaml          # Main compose file
├── .env.example                 # Environment template
└── README.md
`+"`"+``+"`"+``+"`"+`

## Quick Start

### 1. Set up environment variables

`+"`"+``+"`"+``+"`"+`bash
cp .env.example .env
# Edit .env with your values
`+"`"+``+"`"+``+"`"+`

### 2. Add PopSigner mTLS certificates (for L1 signing)

`+"`"+``+"`"+``+"`"+`bash
mkdir -p certs
cp /path/to/client.crt certs/
cp /path/to/client.key certs/
`+"`"+``+"`"+``+"`"+`

### 3. Configure Celestia namespace

Edit `+"`"+`config/celestia-config.toml`+"`"+`:

`+"`"+``+"`"+``+"`"+`toml
[celestia]
namespace_id = "YOUR_UNIQUE_NAMESPACE_HEX"
`+"`"+``+"`"+``+"`"+`

Generate a unique namespace: https://docs.celestia.org/tutorials/node-tutorial#namespaces

### 4. Fund your Celestia key with TIA

Your PopSigner Celestia key needs TIA for gas:

- Get testnet TIA from: https://faucet.celestia-mocha.com/

### 5. Start the stack

`+"`"+``+"`"+``+"`"+`bash
docker compose up -d
docker compose logs -f
`+"`"+``+"`"+``+"`"+`

## Blobstream Configuration (Fraud Proofs)

**This is critical for validators!**

Blobstream is a bridge that relays Celestia data root attestations to Ethereum. It enables:

- Fraud proofs for batches posted to Celestia
- Verification that batch data was actually available on Celestia

### How Blobstream Works

1. **Batch Poster** posts batch data to Celestia → gets `+"`"+`BlobPointer`+"`"+`
2. **Batch Poster** posts batch commitment to Sequencer Inbox
3. **Blobstream** relays Celestia block data roots to L1
4. **Validator** (during fraud proof) calls `+"`"+`GetProof`+"`"+` on celestia-das-server
5. **celestia-das-server** queries Blobstream contract for attestation
6. **celestia-das-server** returns proof data for on-chain verification

## Ports

| Service             | Port | Description           |
| ------------------- | ---- | --------------------- |
| Nitro Sequencer     | 8547 | HTTP RPC              |
| Nitro Sequencer     | 8548 | WebSocket RPC         |
| Nitro Sequencer     | 9642 | Metrics               |
| Nitro Sequencer     | 9644 | Feed (for full nodes) |
| Celestia DAS Server | 9876 | DA Provider RPC       |
| Celestia DAS Server | 6060 | Metrics               |

## Two PopSigner Keys Explained

This setup uses **two separate PopSigner keys**:

### 1. L1 (%s) PopSigner

- **Used by**: Nitro batch poster and validator
- **For**: Signing L1 transactions (batch submissions, staking)
- **Config**: `+"`"+`POPSIGNER_MTLS_URL`+"`"+` + mTLS certificates
- **Funds needed**: ETH on %s

### 2. Celestia PopSigner

- **Used by**: Celestia DAS server
- **For**: Signing Celestia blob transactions
- **Config**: `+"`"+`POPSIGNER_CELESTIA_API_KEY`+"`"+` + `+"`"+`POPSIGNER_CELESTIA_KEY_ID`+"`"+`
- **Funds needed**: TIA on Celestia %s

## Nitro Image Options

### Option 1: Build from Local Source (Recommended)

`+"`"+``+"`"+``+"`"+`bash
cd /path/to/nitro

# Build the Docker image
make docker

# Or build specific target
docker build -t nitro-node:local --target nitro-node .
`+"`"+``+"`"+``+"`"+`

Then update your `+"`"+`.env`+"`"+`:

`+"`"+``+"`"+``+"`"+`bash
echo "NITRO_IMAGE=nitro-node:local" >> .env
`+"`"+``+"`"+``+"`"+`

### Option 2: Use Official Release

When `+"`"+`offchainlabs/nitro-node:v3.9.0`+"`"+` is published on Docker Hub, update:

`+"`"+``+"`"+``+"`"+`bash
echo "NITRO_IMAGE=offchainlabs/nitro-node:v3.9.0" >> .env
`+"`"+``+"`"+``+"`"+`

## Troubleshooting

### Check service health

`+"`"+``+"`"+``+"`"+`bash
docker compose ps
docker compose logs celestia-das-server
docker compose logs nitro-sequencer
`+"`"+``+"`"+``+"`"+`

### Test Celestia DAS server

`+"`"+``+"`"+``+"`"+`bash
curl -X POST http://localhost:9876 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"daprovider_getSupportedHeaderBytes","params":[],"id":1}'
`+"`"+``+"`"+``+"`"+`

### Batch poster not submitting to Celestia

1. Check if Celestia PopSigner key is funded with TIA
2. Check celestia-das-server logs: `+"`"+`docker compose logs -f celestia-das-server`+"`"+`
3. Verify Celestia endpoint connectivity

### Blobstream proof failures

1. Verify `+"`"+`blobstream_addr`+"`"+` is correct for your parent chain
2. Check that `+"`"+`eth_rpc`+"`"+` can reach the parent chain
3. Blobstream may need time to relay attestations (~1 hour)

## Resources

- [Arbitrum Orbit Docs](https://docs.arbitrum.io/launch-orbit-chain/orbit-gentle-introduction)
- [Celestia DA Docs](https://docs.celestia.org/)
- [nitro-das-celestia](https://github.com/celestiaorg/nitro-das-celestia)
- [Blobstream Docs](https://docs.celestia.org/how-to-guides/blobstream)
- [Celestia Mocha Faucet](https://faucet.celestia-mocha.com/)
- [PopSigner](https://github.com/Bidon15/popsigner)
`, config.ChainName, config.ChainName, parentChainName,
		parentChainName, celestiaNetwork,
		config.ChainID, config.ChainName, parentChainName, config.ParentChainID,
		contracts.Rollup, contracts.SequencerInbox, contracts.Bridge,
		parentChainName, parentChainName, parentChainName, celestiaNetwork,
		config.ChainName,
		parentChainName, parentChainName, celestiaNetwork)
}
