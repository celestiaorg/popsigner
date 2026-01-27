package popdeployer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/nitro"
	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/opstack"
	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
)

const (
	// deploymentTimeout is the maximum time allowed for OP Stack deployment.
	// Typical deployments take 5-15 minutes; 30 minutes provides adequate buffer
	// for slow networks or resource-constrained environments.
	deploymentTimeout = 30 * time.Minute

	// anvilShutdownTimeout is how long to wait for Anvil to gracefully shut down
	// before forcing a kill signal.
	anvilShutdownTimeout = 5 * time.Second
)

// Stage represents the deployment stage for progress tracking.
type Stage string

const (
	// Common stages
	StageStartingAnvil     Stage = "starting_anvil"
	StageCapturingState    Stage = "capturing_state"
	StageGeneratingConfigs Stage = "generating_configs"
	StageComplete          Stage = "complete"

	// OP Stack stages
	StageDeployingContracts Stage = "deploying_contracts"

	// Nitro stages
	StageDownloadingArtifacts     Stage = "downloading_artifacts"
	StageDeployingInfrastructure  Stage = "deploying_infrastructure"
	StageDeployingWETH            Stage = "deploying_weth"
	StageCreatingRollup           Stage = "creating_rollup"
)

// String returns the string representation of the stage.
func (s Stage) String() string {
	return string(s)
}

// ProgressCallback is called during deployment to report progress.
type ProgressCallback func(stage Stage, progress float64, message string)

// OrchestratorConfig contains configuration for the orchestrator.
type OrchestratorConfig struct {
	// Logger for structured logging
	Logger *slog.Logger

	// CacheDir for op-deployer artifacts
	CacheDir string

	// WorkDir for temporary files (Anvil state, etc.)
	WorkDir string
}

// Orchestrator coordinates POPKins devnet bundle deployments.
type Orchestrator struct {
	repo   repository.Repository
	config OrchestratorConfig
	logger *slog.Logger
}

// New Orchestrator creates a new POPKins bundle deployment orchestrator.
func NewOrchestrator(
	repo repository.Repository,
	config OrchestratorConfig,
) *Orchestrator {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if config.CacheDir == "" {
		config.CacheDir = filepath.Join(os.TempDir(), "popdeployer-cache")
	}

	if config.WorkDir == "" {
		config.WorkDir = filepath.Join(os.TempDir(), "popdeployer-work")
	}

	return &Orchestrator{
		repo:   repo,
		config: config,
		logger: logger,
	}
}

// Deploy executes a POPKins devnet bundle deployment.
// It runs ephemeral Anvil, deploys contracts (OP Stack or Nitro based on bundle_stack),
// and saves all artifacts for bundle generation.
func (o *Orchestrator) Deploy(ctx context.Context, deploymentID uuid.UUID, onProgress ProgressCallback) error {
	o.logger.Info("starting POPKins devnet bundle deployment",
		slog.String("deployment_id", deploymentID.String()),
	)

	// 1. Load deployment from database
	deployment, err := o.repo.GetDeployment(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("load deployment: %w", err)
	}
	if deployment == nil {
		return fmt.Errorf("deployment not found: %s", deploymentID)
	}

	// 2. Parse deployment config
	var cfg DeploymentConfig
	if err := json.Unmarshal(deployment.Config, &cfg); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	// 3. Populate hardcoded values
	cfg = o.populateDefaults(cfg)

	// 4. Create work directory for this deployment
	workDir := filepath.Join(o.config.WorkDir, deploymentID.String())
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}
	defer os.RemoveAll(workDir) // Clean up after deployment

	// 5. Create deployment context
	deployCtx := &DeploymentContext{
		DeploymentID: deploymentID,
		Config:       &cfg,
		WorkDir:      workDir,
		OnProgress:   onProgress,
	}

	// 6. Dispatch based on bundle_stack
	stageWriter := &StageWriter{repo: o.repo, deploymentID: deploymentID}

	// Default to "opstack" if bundle_stack is empty
	bundleStack := cfg.BundleStack
	if bundleStack == "" {
		bundleStack = "opstack"
	}

	o.logger.Info("deploying bundle",
		slog.String("bundle_stack", bundleStack),
		slog.String("deployment_id", deploymentID.String()),
	)

	var deployErr error
	switch bundleStack {
	case "nitro":
		deployErr = o.deployNitroBundle(ctx, deployCtx, stageWriter)
	default:
		// OP Stack (default)
		deployErr = o.deployOPStackBundle(ctx, deployCtx, stageWriter)
	}

	if deployErr != nil {
		return deployErr
	}

	// Mark as complete
	if onProgress != nil {
		onProgress(StageComplete, 1.0, "Bundle deployment complete")
	}

	// Update deployment status to completed (not just the stage)
	stageStr := StageComplete.String()
	if err := o.repo.UpdateDeploymentStatus(ctx, deploymentID, repository.StatusCompleted, &stageStr); err != nil {
		o.logger.Warn("failed to mark deployment as completed",
			slog.String("error", err.Error()),
		)
	}

	o.logger.Info("POPKins devnet bundle deployment completed successfully",
		slog.String("deployment_id", deploymentID.String()),
		slog.String("bundle_stack", bundleStack),
	)

	return nil
}

// deployOPStackBundle deploys an OP Stack devnet bundle.
func (o *Orchestrator) deployOPStackBundle(ctx context.Context, deployCtx *DeploymentContext, stageWriter *StageWriter) error {
	// Stage 1: Start Anvil
	if err := o.startAnvil(ctx, deployCtx, stageWriter); err != nil {
		return fmt.Errorf("start anvil: %w", err)
	}

	// Stage 2: Deploy OP Stack contracts (Anvil handles signing directly)
	result, err := o.deployOPStack(ctx, deployCtx, stageWriter)
	if err != nil {
		return fmt.Errorf("deploy opstack: %w", err)
	}

	// Stage 3: Capture Anvil state
	if err := o.captureAnvilState(ctx, deployCtx, stageWriter); err != nil {
		return fmt.Errorf("capture anvil state: %w", err)
	}

	// Stage 4: Generate and save all config artifacts
	if err := o.generateConfigs(ctx, deployCtx, result, stageWriter); err != nil {
		return fmt.Errorf("generate configs: %w", err)
	}

	return nil
}

// nitroDeployResult holds the results of Nitro deployment for config generation.
type nitroDeployResult struct {
	contracts       *nitro.RollupContracts
	chainConfig     map[string]interface{}
	deploymentBlock uint64
	stakeToken      common.Address
}

// deployNitroBundle deploys a Nitro devnet bundle.
// Pipeline: Anvil → Download Artifacts → Infrastructure → WETH → Rollup → Capture State → Generate Configs
func (o *Orchestrator) deployNitroBundle(ctx context.Context, deployCtx *DeploymentContext, stageWriter *StageWriter) error {
	o.logger.Info("deploying Nitro bundle",
		slog.String("deployment_id", deployCtx.DeploymentID.String()),
		slog.Uint64("chain_id", deployCtx.Config.ChainID),
		slog.String("chain_name", deployCtx.Config.ChainName),
	)

	// Stage 1: Start Anvil
	if err := o.startAnvil(ctx, deployCtx, stageWriter); err != nil {
		return fmt.Errorf("start anvil: %w", err)
	}

	// Stage 2: Download Nitro contract artifacts
	artifacts, err := o.downloadNitroArtifacts(ctx, deployCtx, stageWriter)
	if err != nil {
		return fmt.Errorf("download nitro artifacts: %w", err)
	}

	// Stage 3: Create LocalSigner using Anvil's deployer key
	signer, err := o.createNitroLocalSigner(deployCtx)
	if err != nil {
		return fmt.Errorf("create local signer: %w", err)
	}

	// Stage 4: Deploy infrastructure (RollupCreator + templates)
	infraResult, err := o.deployNitroInfrastructure(ctx, deployCtx, stageWriter, artifacts, signer)
	if err != nil {
		return fmt.Errorf("deploy infrastructure: %w", err)
	}

	// Stage 5: Deploy WETH for BOLD staking
	stakeToken, err := o.deployWETH(ctx, deployCtx, stageWriter, signer)
	if err != nil {
		return fmt.Errorf("deploy WETH: %w", err)
	}

	// Stage 6: Create Rollup
	rollupResult, err := o.deployNitroRollup(ctx, deployCtx, stageWriter, artifacts, signer, infraResult.RollupCreatorAddress, stakeToken)
	if err != nil {
		return fmt.Errorf("deploy rollup: %w", err)
	}

	// Stage 7: Capture Anvil state
	if err := o.captureAnvilState(ctx, deployCtx, stageWriter); err != nil {
		return fmt.Errorf("capture anvil state: %w", err)
	}

	// Stage 8: Generate and save all config artifacts
	nitroResult := &nitroDeployResult{
		contracts:       rollupResult.Contracts,
		chainConfig:     rollupResult.ChainConfig,
		deploymentBlock: rollupResult.BlockNumber,
		stakeToken:      stakeToken,
	}
	if err := o.generateNitroConfigs(ctx, deployCtx, nitroResult, stageWriter); err != nil {
		return fmt.Errorf("generate nitro configs: %w", err)
	}

	return nil
}

// downloadNitroArtifacts downloads Nitro contract artifacts from S3.
func (o *Orchestrator) downloadNitroArtifacts(ctx context.Context, deployCtx *DeploymentContext, sw *StageWriter) (*nitro.NitroArtifacts, error) {
	if deployCtx.OnProgress != nil {
		deployCtx.OnProgress(StageDownloadingArtifacts, 0.15, "Downloading Nitro contract artifacts...")
	}
	if err := sw.UpdateStage(ctx, StageDownloadingArtifacts); err != nil {
		o.logger.Warn("failed to update stage", slog.String("error", err.Error()))
	}

	cacheDir := filepath.Join(o.config.CacheDir, "nitro-artifacts")
	downloader := nitro.NewContractArtifactDownloader(cacheDir)

	artifacts, err := downloader.DownloadDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("download artifacts: %w", err)
	}

	o.logger.Info("Nitro artifacts downloaded",
		slog.String("version", artifacts.Version),
		slog.String("source", artifacts.SourceURL),
	)

	return artifacts, nil
}

// createNitroLocalSigner creates a LocalSigner using Anvil's deterministic deployer key.
func (o *Orchestrator) createNitroLocalSigner(deployCtx *DeploymentContext) (*nitro.LocalSigner, error) {
	// Anvil's deterministic deployer private key (anvil-0)
	const anvilDeployerKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

	signer, err := nitro.NewLocalSigner(anvilDeployerKey, int64(deployCtx.Config.L1ChainID))
	if err != nil {
		return nil, fmt.Errorf("create local signer: %w", err)
	}

	o.logger.Info("Created local signer",
		slog.String("address", signer.Address().Hex()),
	)

	return signer, nil
}

// deployNitroInfrastructure deploys RollupCreator and template contracts.
func (o *Orchestrator) deployNitroInfrastructure(
	ctx context.Context,
	deployCtx *DeploymentContext,
	sw *StageWriter,
	artifacts *nitro.NitroArtifacts,
	signer *nitro.LocalSigner,
) (*nitro.InfrastructureResult, error) {
	if deployCtx.OnProgress != nil {
		deployCtx.OnProgress(StageDeployingInfrastructure, 0.25, "Deploying Nitro infrastructure...")
	}
	if err := sw.UpdateStage(ctx, StageDeployingInfrastructure); err != nil {
		o.logger.Warn("failed to update stage", slog.String("error", err.Error()))
	}

	infraDeployer := nitro.NewInfrastructureDeployer(artifacts, signer, nil, o.logger)

	cfg := &nitro.InfrastructureConfig{
		ParentChainID: int64(deployCtx.Config.L1ChainID),
		ParentRPC:     deployCtx.Config.L1RPC,
	}

	result, err := infraDeployer.EnsureInfrastructure(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("ensure infrastructure: %w", err)
	}

	o.logger.Info("Nitro infrastructure deployed",
		slog.String("rollup_creator", result.RollupCreatorAddress.Hex()),
		slog.Int("contracts_deployed", len(result.DeployedContracts)),
	)

	return result, nil
}

// WETH9 bytecode (standard Wrapped Ether contract)
const weth9Bytecode = "0x60606040526040805190810160405280600d81526020017f57726170706564204574686572000000000000000000000000000000000000008152506000908051906020019061004f9291906100c8565b506040805190810160405280600481526020017f57455448000000000000000000000000000000000000000000000000000000008152506001908051906020019061009b9291906100c8565b506012600260006101000a81548160ff021916908360ff16021790555034156100c357600080fd5b61016d565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f1061010957805160ff1916838001178555610137565b82800160010185558215610137579182015b8281111561013657825182559160200191906001019061011b565b5b5090506101449190610148565b5090565b61016a91905b8082111561016657600081600090555060010161014e565b5090565b90565b6106598061017c6000396000f30060606040526004361061008e576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063095ea7b31461009357806318160ddd146100ed57806323b872dd14610116578063313ce5671461018f5780636361c39d146101be57806370a0823114610205578063a9059cbb14610252578063dd62ed3e146102ac575b600080fd5b341561009e57600080fd5b6100d3600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091908035906020019091905050610318565b604051808215151515815260200191505060405180910390f35b34156100f857600080fd5b61010061040a565b6040518082815260200191505060405180910390f35b341561012157600080fd5b610175600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803573ffffffffffffffffffffffffffffffffffffffff16906020019091908035906020019091905050610410565b604051808215151515815260200191505060405180910390f35b341561019a57600080fd5b6101a261062e565b604051808260ff1660ff16815260200191505060405180910390f35b6101eb600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050610641565b604051808215151515815260200191505060405180910390f35b341561021057600080fd5b61023c600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190505061069b565b6040518082815260200191505060405180910390f35b341561025d57600080fd5b610292600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919080359060200190919050506106b3565b604051808215151515815260200191505060405180910390f35b34156102b757600080fd5b610302600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506106c8565b6040518082815260200191505060405180910390f35b600081600460003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925846040518082815260200191505060405180910390a36001905092915050565b60035481565b600081600360008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054101580156104fd575081600460008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205410155b801561050a575060008210155b151561051557600080fd5b81600360008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254039250508190555081600360008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254019250508190555081600460008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825403925050819055506001905093915050565b600260009054906101000a900460ff1681565b60003073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561067d57600080fd5b81600360008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055506001905092915050565b60036020528060005260406000206000915090505481565b60006106c0338484610410565b905092915050565b60046020528160005260406000206020528060005260406000206000915091505054815600a165627a7a72305820e7e9c87b51c5bb35f82f2f1f7bb1823c41cfcd8f3ab8c2a5b58baf35e9cbdd4f0029"

// deployWETH deploys a WETH contract for BOLD staking.
func (o *Orchestrator) deployWETH(
	ctx context.Context,
	deployCtx *DeploymentContext,
	sw *StageWriter,
	signer *nitro.LocalSigner,
) (common.Address, error) {
	if deployCtx.OnProgress != nil {
		deployCtx.OnProgress(StageDeployingWETH, 0.35, "Deploying WETH for BOLD staking...")
	}
	if err := sw.UpdateStage(ctx, StageDeployingWETH); err != nil {
		o.logger.Warn("failed to update stage", slog.String("error", err.Error()))
	}

	client, err := ethclient.DialContext(ctx, deployCtx.Config.L1RPC)
	if err != nil {
		return common.Address{}, fmt.Errorf("connect to L1: %w", err)
	}
	defer client.Close()

	wethBytecode := common.FromHex(weth9Bytecode)

	nonce, err := client.PendingNonceAt(ctx, signer.Address())
	if err != nil {
		return common.Address{}, fmt.Errorf("get nonce: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return common.Address{}, fmt.Errorf("get gas price: %w", err)
	}

	// Create contract creation transaction
	tx := types.NewContractCreation(nonce, big.NewInt(0), 1_000_000, gasPrice, wethBytecode)

	signedTx, err := signer.SignTransaction(ctx, tx)
	if err != nil {
		return common.Address{}, fmt.Errorf("sign transaction: %w", err)
	}

	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return common.Address{}, fmt.Errorf("send transaction: %w", err)
	}

	// Wait for receipt
	receipt, err := waitForReceipt(ctx, client, signedTx.Hash())
	if err != nil {
		return common.Address{}, fmt.Errorf("wait for receipt: %w", err)
	}

	if receipt.Status == 0 {
		return common.Address{}, fmt.Errorf("WETH deployment reverted")
	}

	o.logger.Info("WETH deployed",
		slog.String("address", receipt.ContractAddress.Hex()),
	)

	return receipt.ContractAddress, nil
}

// waitForReceipt waits for a transaction receipt.
func waitForReceipt(ctx context.Context, client *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			receipt, err := client.TransactionReceipt(ctx, txHash)
			if err == nil {
				return receipt, nil
			}
			// Continue polling if receipt not found
		}
	}
}

// deployNitroRollup deploys the Nitro rollup using RollupCreator.
func (o *Orchestrator) deployNitroRollup(
	ctx context.Context,
	deployCtx *DeploymentContext,
	sw *StageWriter,
	artifacts *nitro.NitroArtifacts,
	signer *nitro.LocalSigner,
	rollupCreatorAddr common.Address,
	stakeToken common.Address,
) (*nitro.RollupDeployResult, error) {
	if deployCtx.OnProgress != nil {
		deployCtx.OnProgress(StageCreatingRollup, 0.45, "Creating Nitro rollup...")
	}
	if err := sw.UpdateStage(ctx, StageCreatingRollup); err != nil {
		o.logger.Warn("failed to update stage", slog.String("error", err.Error()))
	}

	deployer, err := nitro.NewRollupDeployer(artifacts, signer, o.logger)
	if err != nil {
		return nil, fmt.Errorf("create rollup deployer: %w", err)
	}

	// Use hardcoded Anvil addresses for batch poster and validator
	batchPosterAddr := common.HexToAddress(deployCtx.Config.BatcherAddress)
	validatorAddr := common.HexToAddress(deployCtx.Config.ProposerAddress)

	cfg := &nitro.RollupConfig{
		ChainID:          int64(deployCtx.Config.ChainID),
		ChainName:        deployCtx.Config.ChainName,
		ParentChainID:    int64(deployCtx.Config.L1ChainID),
		ParentChainRPC:   deployCtx.Config.L1RPC,
		Owner:            signer.Address(),
		BatchPosters:     []common.Address{batchPosterAddr},
		Validators:       []common.Address{validatorAddr},
		StakeToken:       stakeToken,
		BaseStake:        big.NewInt(100000000000000000), // 0.1 ETH
		DataAvailability: nitro.DAModeCelestia,
	}

	result, err := deployer.Deploy(ctx, cfg, rollupCreatorAddr)
	if err != nil {
		return nil, fmt.Errorf("deploy rollup: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("rollup deployment failed: %s", result.Error)
	}

	o.logger.Info("Nitro rollup deployed",
		slog.String("rollup", result.Contracts.Rollup.Hex()),
		slog.String("inbox", result.Contracts.Inbox.Hex()),
		slog.String("bridge", result.Contracts.Bridge.Hex()),
		slog.Uint64("block", result.BlockNumber),
	)

	return result, nil
}

// generateNitroConfigs generates all Nitro configuration files and saves them as artifacts.
func (o *Orchestrator) generateNitroConfigs(ctx context.Context, deployCtx *DeploymentContext, result *nitroDeployResult, sw *StageWriter) error {
	if deployCtx.OnProgress != nil {
		deployCtx.OnProgress(StageGeneratingConfigs, 0.8, "Generating Nitro configuration files...")
	}
	if err := sw.UpdateStage(ctx, StageGeneratingConfigs); err != nil {
		o.logger.Warn("failed to update stage", slog.String("error", err.Error()))
	}

	// Celestia key ID is hardcoded - popsigner-lite uses deterministic IDs for Anvil accounts
	celestiaKeyID := "anvil-9"

	o.logger.Info("Using Celestia key",
		slog.String("key_id", celestiaKeyID),
	)

	// Create Nitro config writer
	writer := &NitroConfigWriter{
		logger:        o.logger,
		result:        result,
		config:        deployCtx.Config,
		celestiaKeyID: celestiaKeyID,
	}

	// Generate all configs
	artifacts, err := writer.GenerateAll()
	if err != nil {
		return fmt.Errorf("generate configs: %w", err)
	}

	// Save all artifacts to database
	for artifactType, content := range artifacts {
		jsonContent, err := wrapContentForStorage([]byte(content))
		if err != nil {
			return fmt.Errorf("wrap artifact %s: %w", artifactType, err)
		}

		artifact := &repository.Artifact{
			ID:           uuid.New(),
			DeploymentID: deployCtx.DeploymentID,
			ArtifactType: artifactType,
			Content:      jsonContent,
			CreatedAt:    time.Now(),
		}

		if err := o.repo.SaveArtifact(ctx, artifact); err != nil {
			return fmt.Errorf("save artifact %s: %w", artifactType, err)
		}

		o.logger.Info("Saved artifact",
			slog.String("type", artifactType),
			slog.Int("size_bytes", len(content)),
		)
	}

	// Also save anvil-state.json
	stateFile := filepath.Join(deployCtx.WorkDir, "anvil-state.json")
	stateData, err := os.ReadFile(stateFile)
	if err != nil {
		return fmt.Errorf("read anvil state: %w", err)
	}

	stateArtifact := &repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: deployCtx.DeploymentID,
		ArtifactType: "anvil-state.json",
		Content:      json.RawMessage(stateData),
		CreatedAt:    time.Now(),
	}

	if err := o.repo.SaveArtifact(ctx, stateArtifact); err != nil {
		return fmt.Errorf("save anvil state: %w", err)
	}

	o.logger.Info("All Nitro artifacts saved to database",
		slog.Int("count", len(artifacts)+1),
	)

	return nil
}

// DeploymentContext holds runtime context for a deployment.
type DeploymentContext struct {
	DeploymentID uuid.UUID
	Config       *DeploymentConfig
	WorkDir      string
	OnProgress   ProgressCallback

	// Process handles for cleanup
	AnvilCmd *exec.Cmd
	AnvilIPC string // IPC socket path for Anvil (unique per deployment)
}

// Cleanup terminates any running processes.
func (dc *DeploymentContext) Cleanup() {
	if dc.AnvilCmd != nil && dc.AnvilCmd.Process != nil {
		dc.AnvilCmd.Process.Kill()
	}
}

// StageWriter updates deployment stage in the database.
type StageWriter struct {
	repo         repository.Repository
	deploymentID uuid.UUID
}

// UpdateStage updates the current stage in the database.
func (sw *StageWriter) UpdateStage(ctx context.Context, stage Stage) error {
	stageStr := stage.String()
	return sw.repo.UpdateDeploymentStatus(ctx, sw.deploymentID, repository.StatusRunning, &stageStr)
}

// populateDefaults fills in hardcoded configuration values.
func (o *Orchestrator) populateDefaults(cfg DeploymentConfig) DeploymentConfig {
	// Set hardcoded L1 configuration (Anvil)
	cfg.L1ChainID = 31337
	cfg.L1RPC = "http://localhost:8545"

	// Set hardcoded Anvil accounts
	cfg.DeployerAddress = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266" // anvil-0
	cfg.BatcherAddress = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"  // anvil-1
	cfg.ProposerAddress = "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"  // anvil-2

	// Set hardcoded chain parameters
	if cfg.BlockTime == 0 {
		cfg.BlockTime = 2
	}
	if cfg.GasLimit == 0 {
		cfg.GasLimit = 30000000
	}

	// Note: POPSigner-Lite is NOT needed during bundle build phase
	// We use AnvilSigner for direct ECDSA signing with Anvil's well-known keys
	// POPSigner-Lite is only needed at runtime (op-batcher, op-proposer in docker-compose)

	return cfg
}

// startAnvil starts an ephemeral Anvil L1 node using IPC for isolation.
// Using IPC instead of HTTP ports allows multiple concurrent deployments
// without port conflicts.
func (o *Orchestrator) startAnvil(ctx context.Context, dc *DeploymentContext, sw *StageWriter) error {
	if dc.OnProgress != nil {
		dc.OnProgress(StageStartingAnvil, 0.1, "Starting ephemeral Anvil L1...")
	}
	if err := sw.UpdateStage(ctx, StageStartingAnvil); err != nil {
		o.logger.Warn("failed to update stage", slog.String("error", err.Error()))
	}

	stateFile := filepath.Join(dc.WorkDir, "anvil-state.json")

	// Use IPC socket in the work directory - unique per deployment
	// This avoids port conflicts when multiple deployments run concurrently
	ipcPath := filepath.Join(dc.WorkDir, "anvil.ipc")
	dc.AnvilIPC = ipcPath

	// Update L1RPC to use IPC path instead of HTTP
	// go-ethereum's rpc.DialContext supports IPC paths directly
	dc.Config.L1RPC = ipcPath

	dc.AnvilCmd = exec.CommandContext(ctx, "anvil",
		"--chain-id", fmt.Sprintf("%d", dc.Config.L1ChainID),
		"--accounts", "10",
		"--balance", "10000",
		"--gas-limit", fmt.Sprintf("%d", dc.Config.GasLimit),
		"--block-time", fmt.Sprintf("%d", dc.Config.BlockTime),
		"--ipc", ipcPath,
		"--state", stateFile,
	)

	// Redirect output to log files in work directory
	anvilLog := filepath.Join(dc.WorkDir, "anvil.log")
	logFile, err := os.Create(anvilLog)
	if err != nil {
		return fmt.Errorf("create anvil log: %w", err)
	}
	defer logFile.Close()

	dc.AnvilCmd.Stdout = logFile
	dc.AnvilCmd.Stderr = logFile

	if err := dc.AnvilCmd.Start(); err != nil {
		return fmt.Errorf("start anvil process: %w", err)
	}

	// Wait for Anvil IPC socket to be ready
	if !waitForIPC(ipcPath, 30*time.Second) {
		return fmt.Errorf("anvil IPC socket failed to appear within 30 seconds")
	}

	o.logger.Info("Anvil is ready (IPC mode)",
		slog.String("ipc", ipcPath),
		slog.String("state_file", stateFile),
	)

	return nil
}

// waitForIPC polls for an IPC socket to be ready by attempting actual RPC connections.
// Simply checking file existence is insufficient - the socket file can exist before
// anvil is ready to accept connections (especially on macOS).
func waitForIPC(path string, timeout time.Duration) bool {
	start := time.Now()
	for time.Since(start) < timeout {
		// First check if file exists
		if _, err := os.Stat(path); err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Try to actually connect and make an RPC call
		client, err := ethclient.Dial(path)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Try a simple RPC call to verify the connection works
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err = client.ChainID(ctx)
		cancel()
		client.Close()

		if err == nil {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// deployOPStack deploys OP Stack contracts using the op-deployer.
func (o *Orchestrator) deployOPStack(ctx context.Context, dc *DeploymentContext, sw *StageWriter) (*opstack.DeployResult, error) {
	if dc.OnProgress != nil {
		dc.OnProgress(StageDeployingContracts, 0.3, "Deploying OP Stack contracts...")
	}
	if err := sw.UpdateStage(ctx, StageDeployingContracts); err != nil {
		o.logger.Warn("failed to update stage", slog.String("error", err.Error()))
	}

	// Create opstack deployment config
	// UseLocalSigning=true skips POPSigner validation - we use AnvilSigner instead
	// FundDevAccounts=true pre-funds Anvil's accounts on L2 for testing
	opstackCfg := &opstack.DeploymentConfig{
		ChainID:         dc.Config.ChainID,
		ChainName:       dc.Config.ChainName,
		L1ChainID:       dc.Config.L1ChainID,
		L1RPC:           dc.Config.L1RPC,
		DeployerAddress: dc.Config.DeployerAddress,
		BatcherAddress:  dc.Config.BatcherAddress,
		ProposerAddress: dc.Config.ProposerAddress,
		BlockTime:       dc.Config.BlockTime,
		GasLimit:        dc.Config.GasLimit,
		UseLocalSigning: true, // Use AnvilSigner for Anvil's well-known keys
		FundDevAccounts: true, // Pre-fund Anvil accounts on L2 for local testing
	}

	// Create deployer
	deployer := opstack.NewOPDeployer(opstack.OPDeployerConfig{
		Logger:   o.logger,
		CacheDir: o.config.CacheDir,
	})

	// Create AnvilSigner for direct local ECDSA signing
	// This is much faster than HTTP-based signing and works with IPC
	chainIDBigInt := new(big.Int).SetUint64(dc.Config.L1ChainID)
	adapter, err := opstack.NewAnvilSigner(chainIDBigInt)
	if err != nil {
		return nil, fmt.Errorf("create anvil signer: %w", err)
	}

	// Progress callback
	progressCallback := func(stage string, progress float64, message string) {
		o.logger.Info(message,
			slog.String("stage", stage),
			slog.Float64("progress", progress*100),
		)
		if dc.OnProgress != nil {
			// Map opstack progress (0.0-1.0) to our range (0.3-0.6)
			adjustedProgress := 0.3 + (progress * 0.3)
			dc.OnProgress(StageDeployingContracts, adjustedProgress, message)
		}
	}

	// Deploy with timeout
	deployCtx, deployCancel := context.WithTimeout(ctx, deploymentTimeout)
	defer deployCancel()

	result, err := deployer.Deploy(deployCtx, opstackCfg, adapter, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("deploy: %w", err)
	}

	o.logger.Info("OP Stack deployment completed",
		slog.Int("chains", len(result.ChainStates)),
	)

	// Populate StartBlock if needed (following orchestrator.go pattern)
	if len(result.ChainStates) == 0 {
		return nil, fmt.Errorf("no chain states returned from deployment")
	}

	chainState := result.ChainStates[0]
	if chainState.StartBlock == nil {
		o.logger.Info("populating StartBlock from L1")

		l1Client, err := ethclient.Dial(dc.Config.L1RPC)
		if err != nil {
			return nil, fmt.Errorf("connect to L1: %w", err)
		}
		defer l1Client.Close()

		header, err := l1Client.HeaderByNumber(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("get L1 header: %w", err)
		}

		chainState.StartBlock = state.BlockRefJsonFromHeader(header)

		// Update in state as well
		for i, c := range result.State.Chains {
			if c.ID == chainState.ID {
				result.State.Chains[i].StartBlock = chainState.StartBlock
				break
			}
		}

		o.logger.Info("StartBlock populated",
			slog.Uint64("block_number", header.Number.Uint64()),
		)
	}

	return result, nil
}

// captureAnvilState gracefully shuts down Anvil to trigger state dump.
func (o *Orchestrator) captureAnvilState(ctx context.Context, dc *DeploymentContext, sw *StageWriter) error {
	if dc.OnProgress != nil {
		dc.OnProgress(StageCapturingState, 0.7, "Capturing Anvil state...")
	}
	if err := sw.UpdateStage(ctx, StageCapturingState); err != nil {
		o.logger.Warn("failed to update stage", slog.String("error", err.Error()))
	}

	// Send SIGTERM for graceful shutdown
	if err := dc.AnvilCmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to anvil: %w", err)
	}

	// Wait for Anvil to finish (with timeout)
	anvilDone := make(chan error, 1)
	go func() {
		anvilDone <- dc.AnvilCmd.Wait()
	}()

	select {
	case err := <-anvilDone:
		// SIGTERM exit is expected, other errors are warnings
		if err != nil {
			// Check if it's a signal termination (expected)
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
					// Normal SIGTERM exit - no warning needed
					break
				}
			}
			// Unexpected error
			o.logger.Warn("Anvil exited with error", slog.String("error", err.Error()))
		}
	case <-time.After(anvilShutdownTimeout):
		o.logger.Warn("Anvil shutdown timeout, forcing kill")
		dc.AnvilCmd.Process.Kill()
	}

	// Verify state file was created
	stateFile := filepath.Join(dc.WorkDir, "anvil-state.json")
	if info, err := os.Stat(stateFile); err == nil {
		o.logger.Info("Anvil state dumped successfully",
			slog.String("file", stateFile),
			slog.Int64("size_kb", info.Size()/1024),
		)
	} else {
		return fmt.Errorf("anvil state file not created: %w", err)
	}

	return nil
}

// generateConfigs generates all configuration files and saves them as artifacts.
func (o *Orchestrator) generateConfigs(ctx context.Context, dc *DeploymentContext, result *opstack.DeployResult, sw *StageWriter) error {
	if dc.OnProgress != nil {
		dc.OnProgress(StageGeneratingConfigs, 0.8, "Generating configuration files...")
	}
	if err := sw.UpdateStage(ctx, StageGeneratingConfigs); err != nil {
		o.logger.Warn("failed to update stage", slog.String("error", err.Error()))
	}

	// Celestia key ID is hardcoded - popsigner-lite uses deterministic IDs for Anvil accounts
	// anvil-9 = 0xa0Ee7A142d267C1f36714E4a8F75612F20a79720
	celestiaKeyID := "anvil-9"

	o.logger.Info("Using Celestia key",
		slog.String("key_id", celestiaKeyID),
	)

	// Create config writer
	writer := &ConfigWriter{
		logger:        o.logger,
		result:        result,
		config:        dc.Config,
		celestiaKeyID: celestiaKeyID,
	}

	// Generate all configs
	artifacts, err := writer.GenerateAll()
	if err != nil {
		return fmt.Errorf("generate configs: %w", err)
	}

	// Save all artifacts to database
	for artifactType, content := range artifacts {
		// Wrap content for storage (base64 for non-JSON, as-is for JSON)
		jsonContent, err := wrapContentForStorage([]byte(content))
		if err != nil {
			return fmt.Errorf("wrap artifact %s: %w", artifactType, err)
		}

		artifact := &repository.Artifact{
			ID:           uuid.New(),
			DeploymentID: dc.DeploymentID,
			ArtifactType: artifactType,
			Content:      jsonContent,
			CreatedAt:    time.Now(),
		}

		if err := o.repo.SaveArtifact(ctx, artifact); err != nil {
			return fmt.Errorf("save artifact %s: %w", artifactType, err)
		}

		o.logger.Info("Saved artifact",
			slog.String("type", artifactType),
			slog.Int("size_bytes", len(content)),
		)
	}

	// Also save anvil-state.json
	stateFile := filepath.Join(dc.WorkDir, "anvil-state.json")
	stateData, err := os.ReadFile(stateFile)
	if err != nil {
		return fmt.Errorf("read anvil state: %w", err)
	}

	stateArtifact := &repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: dc.DeploymentID,
		ArtifactType: "anvil-state.json",
		Content:      json.RawMessage(stateData),
		CreatedAt:    time.Now(),
	}

	if err := o.repo.SaveArtifact(ctx, stateArtifact); err != nil {
		return fmt.Errorf("save anvil state: %w", err)
	}

	o.logger.Info("All artifacts saved to database",
		slog.Int("count", len(artifacts)+1),
	)

	return nil
}

// wrapContentForStorage wraps content for PostgreSQL JSONB storage.
// For non-JSON content (like config.toml, docker-compose.yml, jwt.txt), wraps as base64 in a JSON object.
// This avoids PostgreSQL JSONB normalization issues with escape sequences.
func wrapContentForStorage(content []byte) (json.RawMessage, error) {
	// Check if content is already valid JSON
	if json.Valid(content) {
		return content, nil
	}

	// Wrap non-JSON content as base64 in a JSON object
	wrapper := struct {
		Type string `json:"_type"`
		Data string `json:"data"`
	}{
		Type: "base64",
		Data: base64.StdEncoding.EncodeToString(content),
	}
	encoded, err := json.Marshal(wrapper)
	if err != nil {
		return nil, fmt.Errorf("marshal non-JSON content: %w", err)
	}
	return encoded, nil
}
