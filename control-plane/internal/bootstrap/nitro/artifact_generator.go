package nitro

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

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

	// 6b. Generate ready-to-use .env file
	envFile := GenerateEnv(config, result)
	if err := g.saveTextArtifact(ctx, deploymentID, "env_file", envFile); err != nil {
		return err
	}

	// 7. Generate README.md
	readme := GenerateReadme(config, result)
	if err := g.saveTextArtifact(ctx, deploymentID, "readme", readme); err != nil {
		return err
	}

	// 8. Save mTLS certificates
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

func (g *ArtifactGenerator) saveTextArtifact(ctx context.Context, deploymentID uuid.UUID, artifactType string, content string) error {
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

func (g *ArtifactGenerator) saveArtifact(ctx context.Context, deploymentID uuid.UUID, artifactType string, data interface{}) error {
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

// GenerateChainInfo creates the chain-info.json content.
func GenerateChainInfo(config *DeployConfig, result *DeployResult) (*ChainInfo, error) {
	if result == nil || !result.Success {
		return nil, fmt.Errorf("cannot generate chain info without successful deployment")
	}
	if result.CoreContracts == nil {
		return nil, fmt.Errorf("deployment result missing core contracts")
	}

	chainConfig := buildChainConfig(config)

	// Get stake token - BOLD protocol requires this
	stakeToken := config.StakeToken
	if stakeToken == "" {
		// Default to Sepolia WETH if not specified
		if config.ParentChainID == 11155111 {
			stakeToken = "0x7b79995e5f793A07Bc00c21412e50Ecae098E7f9"
		} else if config.ParentChainID == 1 {
			stakeToken = "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"
		}
	}

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
				StakeToken:             stakeToken,
				NativeToken:            result.CoreContracts.NativeToken,
			},
		},
	}

	return &info, nil
}

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
			"DataAvailabilityCommittee": config.DataAvailability == "anytrust",
			"InitialArbOSVersion":       51,
			"InitialChainOwner":         config.Owner,
			"GenesisBlockNum":           0,
		},
	}
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

// GenerateAllArtifacts generates all artifacts without requiring a repository.
func GenerateAllArtifacts(config *DeployConfig, result *DeployResult) (map[string][]byte, error) {
	artifacts := make(map[string][]byte)

	chainInfo, err := GenerateChainInfo(config, result)
	if err != nil {
		return nil, fmt.Errorf("generate chain-info: %w", err)
	}
	chainInfoJSON, err := json.MarshalIndent(chainInfo, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal chain-info: %w", err)
	}
	artifacts["chain-info.json"] = chainInfoJSON

	nodeConfig, err := GenerateNodeConfig(config, result)
	if err != nil {
		return nil, fmt.Errorf("generate node-config: %w", err)
	}
	nodeConfigJSON, err := json.MarshalIndent(nodeConfig, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal node-config: %w", err)
	}
	artifacts["node-config.json"] = nodeConfigJSON

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
