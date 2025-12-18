package nitro

import (
	"context"
	"encoding/json"
	"fmt"
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

	return nil
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
			"EnableArbOS":               true,
			"AllowDebugPrecompiles":     false,
			// DataAvailabilityCommittee is true for external DA (Celestia/AnyTrust)
			// POPSigner deployments default to Celestia DA
			"DataAvailabilityCommittee": config.DataAvailability != "rollup",
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

