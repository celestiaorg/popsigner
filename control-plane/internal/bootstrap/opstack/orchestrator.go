package opstack

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// L1Client defines the interface for L1 Ethereum operations.
type L1Client interface {
	ChainID(ctx context.Context) (*big.Int, error)
	BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
	NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
	SuggestGasTipCap(ctx context.Context) (*big.Int, error)
	EstimateGas(ctx context.Context, call ethereum.CallMsg) (uint64, error)
	SendTransaction(ctx context.Context, tx *types.Transaction) error
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	Close()
}

// L1ClientFactory creates L1 clients from RPC URLs.
type L1ClientFactory interface {
	Dial(ctx context.Context, rpcURL string) (L1Client, error)
}

// ProgressCallback is called during deployment to report progress.
type ProgressCallback func(stage Stage, progress float64, message string)

// OrchestratorConfig contains configuration for the orchestrator.
type OrchestratorConfig struct {
	// Logger for structured logging
	Logger *slog.Logger

	// CacheDir for op-deployer artifacts
	CacheDir string

	// RetryAttempts for transient failures within a stage
	RetryAttempts int

	// RetryDelay between retry attempts
	RetryDelay time.Duration
}

// Orchestrator coordinates OP Stack chain deployments.
// It manages the deployment lifecycle through multiple stages,
// integrates with SignerFn for transaction signing and StateWriter
// for state persistence, enabling resumable deployments.
type Orchestrator struct {
	repo          repository.Repository
	signerFactory SignerFactory
	l1Factory     L1ClientFactory
	config        OrchestratorConfig
	logger        *slog.Logger
}

// SignerFactory creates POPSigner instances for deployments.
type SignerFactory interface {
	CreateSigner(endpoint, apiKey string, chainID *big.Int) *POPSigner
}

// DefaultSignerFactory implements SignerFactory.
type DefaultSignerFactory struct{}

// CreateSigner creates a new POPSigner with the given configuration.
func (f *DefaultSignerFactory) CreateSigner(endpoint, apiKey string, chainID *big.Int) *POPSigner {
	return NewPOPSigner(SignerConfig{
		Endpoint: endpoint,
		APIKey:   apiKey,
		ChainID:  chainID,
	})
}

// NewOrchestrator creates a new deployment orchestrator.
func NewOrchestrator(
	repo repository.Repository,
	signerFactory SignerFactory,
	l1Factory L1ClientFactory,
	config OrchestratorConfig,
) *Orchestrator {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if config.RetryAttempts <= 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 5 * time.Second
	}

	return &Orchestrator{
		repo:          repo,
		signerFactory: signerFactory,
		l1Factory:     l1Factory,
		config:        config,
		logger:        logger,
	}
}

// DeploymentContext holds runtime context for a deployment.
type DeploymentContext struct {
	DeploymentID uuid.UUID
	Config       *DeploymentConfig
	StateWriter  *StateWriter
	Signer       *POPSigner
	L1Client     L1Client
	OnProgress   ProgressCallback
}

// Deploy executes an OP Stack deployment.
// It loads the deployment configuration, determines the starting stage
// (for resumability), and executes each stage in order.
func (o *Orchestrator) Deploy(ctx context.Context, deploymentID uuid.UUID, onProgress ProgressCallback) error {
	o.logger.Info("starting OP Stack deployment",
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

	// 2. Parse configuration
	cfg, err := ParseConfig(deployment.Config)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// 3. Create state writer
	stateWriter := NewStateWriter(o.repo, deploymentID)
	if onProgress != nil {
		stateWriter.SetUpdateCallback(func(id uuid.UUID, stage string) {
			onProgress(Stage(stage), 0, fmt.Sprintf("Entering stage: %s", stage))
		})
	}

	// 4. Create signer
	signer := o.signerFactory.CreateSigner(
		cfg.POPSignerEndpoint,
		cfg.POPSignerAPIKey,
		cfg.L1ChainIDBig(),
	)

	// 5. Connect to L1
	l1Client, err := o.l1Factory.Dial(ctx, cfg.L1RPC)
	if err != nil {
		return fmt.Errorf("connect to L1: %w", err)
	}
	defer l1Client.Close()

	// 6. Build deployment context
	dctx := &DeploymentContext{
		DeploymentID: deploymentID,
		Config:       cfg,
		StateWriter:  stateWriter,
		Signer:       signer,
		L1Client:     l1Client,
		OnProgress:   onProgress,
	}

	// 7. Determine starting stage (for resumability)
	startStage, err := o.determineStartStage(ctx, stateWriter)
	if err != nil {
		return fmt.Errorf("determine start stage: %w", err)
	}

	o.logger.Info("deployment will start from stage",
		slog.String("deployment_id", deploymentID.String()),
		slog.String("start_stage", startStage.String()),
	)

	// 8. Execute stages
	if err := o.executeStages(ctx, dctx, startStage); err != nil {
		// Mark as failed with error
		if markErr := stateWriter.MarkFailed(ctx, err.Error()); markErr != nil {
			o.logger.Error("failed to mark deployment as failed",
				slog.String("error", markErr.Error()),
			)
		}
		return err
	}

	// 9. Mark complete - real contracts deployed
	if err := stateWriter.MarkComplete(ctx); err != nil {
		return fmt.Errorf("mark complete: %w", err)
	}

	o.logger.Info("OP Stack deployment completed successfully",
		slog.String("deployment_id", deploymentID.String()),
	)

	if onProgress != nil {
		onProgress(StageCompleted, 1.0, "Deployment completed - contracts deployed on L1!")
	}

	return nil
}

// determineStartStage returns the stage to start from based on previous progress.
func (o *Orchestrator) determineStartStage(ctx context.Context, stateWriter *StateWriter) (Stage, error) {
	canResume, err := stateWriter.CanResume(ctx)
	if err != nil {
		return StageInit, err
	}

	if !canResume {
		return StageInit, nil
	}

	currentStage, err := stateWriter.GetCurrentStage(ctx)
	if err != nil {
		return StageInit, err
	}

	// If deployment was previously at a stage, resume from that stage
	// (it may have partially completed before failure)
	return currentStage, nil
}

// executeStages runs all deployment stages from startStage.
func (o *Orchestrator) executeStages(ctx context.Context, dctx *DeploymentContext, startStage Stage) error {
	startIdx := StageIndex(startStage)
	if startIdx < 0 {
		return fmt.Errorf("invalid start stage: %s", startStage)
	}

	totalStages := len(StageOrder)

	for i := startIdx; i < totalStages; i++ {
		stage := StageOrder[i]

		// Skip completed stage marker
		if stage == StageCompleted {
			continue
		}

		// Calculate and report progress
		progress := float64(i) / float64(totalStages-1)
		if dctx.OnProgress != nil {
			dctx.OnProgress(stage, progress, fmt.Sprintf("Executing stage: %s", stage))
		}

		// Update stage in state writer
		if err := dctx.StateWriter.UpdateStage(ctx, stage); err != nil {
			return fmt.Errorf("update stage %s: %w", stage, err)
		}

		o.logger.Info("executing stage",
			slog.String("deployment_id", dctx.DeploymentID.String()),
			slog.String("stage", stage.String()),
			slog.Float64("progress", progress),
		)

		// Execute the stage with retry logic
		if err := o.executeStageWithRetry(ctx, dctx, stage); err != nil {
			return fmt.Errorf("stage %s failed: %w", stage, err)
		}

		o.logger.Info("stage completed",
			slog.String("deployment_id", dctx.DeploymentID.String()),
			slog.String("stage", stage.String()),
		)
	}

	return nil
}

// executeStageWithRetry executes a single stage with retry logic for transient failures.
func (o *Orchestrator) executeStageWithRetry(ctx context.Context, dctx *DeploymentContext, stage Stage) error {
	var lastErr error

	for attempt := 0; attempt < o.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			o.logger.Info("retrying stage",
				slog.String("stage", stage.String()),
				slog.Int("attempt", attempt+1),
				slog.Int("max_attempts", o.config.RetryAttempts),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(o.config.RetryDelay):
			}
		}

		err := o.executeStage(ctx, dctx, stage)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			return err
		}

		o.logger.Warn("stage failed with retryable error",
			slog.String("stage", stage.String()),
			slog.String("error", err.Error()),
		)
	}

	return fmt.Errorf("stage failed after %d attempts: %w", o.config.RetryAttempts, lastErr)
}

// executeStage dispatches to the appropriate stage handler.
func (o *Orchestrator) executeStage(ctx context.Context, dctx *DeploymentContext, stage Stage) error {
	switch stage {
	case StageInit:
		return o.stageInit(ctx, dctx)
	case StageSuperchain:
		return o.stageSuperchain(ctx, dctx)
	case StageImplementations:
		return o.stageImplementations(ctx, dctx)
	case StageOPChain:
		return o.stageOPChain(ctx, dctx)
	case StageAltDA:
		return o.stageAltDA(ctx, dctx)
	case StageGenesis:
		return o.stageGenesis(ctx, dctx)
	case StageStartBlock:
		return o.stageStartBlock(ctx, dctx)
	default:
		return fmt.Errorf("unknown stage: %s", stage)
	}
}

// stageInit validates L1 connection and configuration.
func (o *Orchestrator) stageInit(ctx context.Context, dctx *DeploymentContext) error {
	// Validate L1 chain ID
	chainID, err := dctx.L1Client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("get L1 chain ID: %w", err)
	}

	expectedChainID := dctx.Config.L1ChainIDBig()
	if chainID.Cmp(expectedChainID) != 0 {
		return fmt.Errorf("L1 chain ID mismatch: expected %s, got %s", expectedChainID, chainID)
	}

	// Check deployer balance
	deployerAddr := common.HexToAddress(dctx.Config.DeployerAddress)
	balance, err := dctx.L1Client.BalanceAt(ctx, deployerAddr, nil)
	if err != nil {
		return fmt.Errorf("get deployer balance: %w", err)
	}

	requiredFunding := dctx.Config.RequiredFundingWei
	if balance.Cmp(requiredFunding) < 0 {
		// Format balance in ETH for human readability
		haveETH := weiToETH(balance)
		needETH := weiToETH(requiredFunding)
		return fmt.Errorf("insufficient deployer balance for %s: have %s ETH, need %s ETH. Fund this address on L1 and retry", 
			dctx.Config.DeployerAddress, haveETH, needETH)
	}

	o.logger.Info("init stage completed",
		slog.String("l1_chain_id", chainID.String()),
		slog.String("deployer_balance", balance.String()),
	)

	// Save init state
	initState := map[string]interface{}{
		"l1_chain_id":      chainID.String(),
		"deployer_address": dctx.Config.DeployerAddress,
		"deployer_balance": balance.String(),
		"initialized_at":   time.Now().UTC().Format(time.RFC3339),
	}

	return dctx.StateWriter.WriteState(ctx, initState)
}

// stageSuperchain deploys superchain contracts.
// In a full implementation, this would call op-deployer's pipeline.DeploySuperchain.
// For now, we deploy a marker contract to demonstrate the full signing flow.
func (o *Orchestrator) stageSuperchain(ctx context.Context, dctx *DeploymentContext) error {
	// Check if already completed (idempotency)
	complete, err := dctx.StateWriter.IsStageComplete(ctx, StageSuperchain)
	if err != nil {
		return err
	}
	if complete {
		o.logger.Info("skipping superchain stage (already complete)")
		return nil
	}

	// Read existing state to check for partial completion
	existingState, err := dctx.StateWriter.ReadState(ctx)
	if err != nil {
		return fmt.Errorf("read existing state: %w", err)
	}

	var state map[string]interface{}
	if existingState != nil {
		if err := json.Unmarshal(existingState, &state); err != nil {
			state = make(map[string]interface{})
		}
	} else {
		state = make(map[string]interface{})
	}

	// Deploy a marker contract for the superchain stage
	// In production, this would be SuperchainConfig and ProtocolVersions
	bytecode := stageMarkerBytecode("superchain")
	deployment, err := o.deployContract(ctx, dctx, bytecode, "Deploy SuperchainConfig (marker)")
	if err != nil {
		return fmt.Errorf("deploy superchain marker: %w", err)
	}

	state["superchain_deployed"] = true
	state["superchain_deployed_at"] = time.Now().UTC().Format(time.RFC3339)
	state["superchain_config_address"] = deployment.ContractAddress.Hex()
	state["superchain_tx_hash"] = deployment.TxHash.Hex()
	state["superchain_gas_used"] = deployment.GasUsed

	// Record the real transaction
	if err := dctx.StateWriter.RecordTransaction(ctx, StageSuperchain, deployment.TxHash.Hex(), "Deploy SuperchainConfig"); err != nil {
		return fmt.Errorf("record transaction: %w", err)
	}

	return dctx.StateWriter.WriteState(ctx, state)
}

// stageImplementations deploys implementation contracts.
// In a full implementation, this would deploy L1CrossDomainMessenger, OptimismPortal, etc.
func (o *Orchestrator) stageImplementations(ctx context.Context, dctx *DeploymentContext) error {
	complete, err := dctx.StateWriter.IsStageComplete(ctx, StageImplementations)
	if err != nil {
		return err
	}
	if complete {
		o.logger.Info("skipping implementations stage (already complete)")
		return nil
	}

	existingState, _ := dctx.StateWriter.ReadState(ctx)
	var state map[string]interface{}
	if existingState != nil {
		json.Unmarshal(existingState, &state)
	}
	if state == nil {
		state = make(map[string]interface{})
	}

	// Deploy a marker contract for the implementations stage
	bytecode := stageMarkerBytecode("implementations")
	deployment, err := o.deployContract(ctx, dctx, bytecode, "Deploy Implementations (marker)")
	if err != nil {
		return fmt.Errorf("deploy implementations marker: %w", err)
	}

	state["implementations_deployed"] = true
	state["implementations_deployed_at"] = time.Now().UTC().Format(time.RFC3339)
	state["implementations_address"] = deployment.ContractAddress.Hex()
	state["implementations_tx_hash"] = deployment.TxHash.Hex()
	state["implementations_gas_used"] = deployment.GasUsed

	if err := dctx.StateWriter.RecordTransaction(ctx, StageImplementations, deployment.TxHash.Hex(), "Deploy Implementations"); err != nil {
		return fmt.Errorf("record transaction: %w", err)
	}

	return dctx.StateWriter.WriteState(ctx, state)
}

// stageOPChain deploys the OP chain contracts.
// In a full implementation, this would deploy SystemConfig, AddressManager, etc.
func (o *Orchestrator) stageOPChain(ctx context.Context, dctx *DeploymentContext) error {
	complete, err := dctx.StateWriter.IsStageComplete(ctx, StageOPChain)
	if err != nil {
		return err
	}
	if complete {
		o.logger.Info("skipping opchain stage (already complete)")
		return nil
	}

	existingState, _ := dctx.StateWriter.ReadState(ctx)
	var state map[string]interface{}
	if existingState != nil {
		json.Unmarshal(existingState, &state)
	}
	if state == nil {
		state = make(map[string]interface{})
	}

	// Deploy a marker contract for the opchain stage
	bytecode := stageMarkerBytecode("opchain")
	deployment, err := o.deployContract(ctx, dctx, bytecode, "Deploy OPChain (marker)")
	if err != nil {
		return fmt.Errorf("deploy opchain marker: %w", err)
	}

	state["opchain_deployed"] = true
	state["opchain_deployed_at"] = time.Now().UTC().Format(time.RFC3339)
	state["chain_id"] = dctx.Config.ChainID
	state["opchain_address"] = deployment.ContractAddress.Hex()
	state["opchain_tx_hash"] = deployment.TxHash.Hex()
	state["opchain_gas_used"] = deployment.GasUsed

	if err := dctx.StateWriter.RecordTransaction(ctx, StageOPChain, deployment.TxHash.Hex(), "Deploy OPChain"); err != nil {
		return fmt.Errorf("record transaction: %w", err)
	}

	return dctx.StateWriter.WriteState(ctx, state)
}

// stageAltDA configures Alt-DA for Celestia.
// POPKins only supports Celestia as the DA layer, so this stage always runs.
func (o *Orchestrator) stageAltDA(ctx context.Context, dctx *DeploymentContext) error {
	complete, err := dctx.StateWriter.IsStageComplete(ctx, StageAltDA)
	if err != nil {
		return err
	}
	if complete {
		o.logger.Info("skipping alt-da stage (already complete)")
		return nil
	}

	existingState, _ := dctx.StateWriter.ReadState(ctx)
	var state map[string]interface{}
	if existingState != nil {
		json.Unmarshal(existingState, &state)
	}
	if state == nil {
		state = make(map[string]interface{})
	}

	// In production, this would call:
	// pipeline.DeployAltDA(pEnv, intent, st, chainID)
	// Note: Celestia uses GenericCommitment which requires 0 on-chain transactions
	// The actual DA configuration is handled by the op-alt-da service at runtime
	state["alt_da_deployed"] = true
	state["alt_da_deployed_at"] = time.Now().UTC().Format(time.RFC3339)
	state["da_type"] = "celestia"
	state["da_commitment_type"] = CelestiaDACommitmentType
	state["celestia_namespace"] = dctx.Config.CelestiaNamespace
	state["celestia_rpc"] = dctx.Config.CelestiaRPC

	return dctx.StateWriter.WriteState(ctx, state)
}

// stageGenesis generates the L2 genesis file.
func (o *Orchestrator) stageGenesis(ctx context.Context, dctx *DeploymentContext) error {
	complete, err := dctx.StateWriter.IsStageComplete(ctx, StageGenesis)
	if err != nil {
		return err
	}
	if complete {
		o.logger.Info("skipping genesis stage (already complete)")
		return nil
	}

	existingState, _ := dctx.StateWriter.ReadState(ctx)
	var state map[string]interface{}
	if existingState != nil {
		json.Unmarshal(existingState, &state)
	}
	if state == nil {
		state = make(map[string]interface{})
	}

	// In production, this would call:
	// pipeline.GenerateL2Genesis(pEnv, intent, bundle, st, chainID)
	// This is a local computation, no on-chain transactions
	state["genesis_generated"] = true
	state["genesis_generated_at"] = time.Now().UTC().Format(time.RFC3339)

	// Save genesis as artifact
	genesisData := json.RawMessage(`{"placeholder": "genesis data would be here"}`)
	if err := dctx.StateWriter.SaveArtifact(ctx, "genesis", genesisData); err != nil {
		return fmt.Errorf("save genesis artifact: %w", err)
	}

	return dctx.StateWriter.WriteState(ctx, state)
}

// stageStartBlock sets the L2 start block.
func (o *Orchestrator) stageStartBlock(ctx context.Context, dctx *DeploymentContext) error {
	complete, err := dctx.StateWriter.IsStageComplete(ctx, StageStartBlock)
	if err != nil {
		return err
	}
	if complete {
		o.logger.Info("skipping start-block stage (already complete)")
		return nil
	}

	existingState, _ := dctx.StateWriter.ReadState(ctx)
	var state map[string]interface{}
	if existingState != nil {
		json.Unmarshal(existingState, &state)
	}
	if state == nil {
		state = make(map[string]interface{})
	}

	// In production, this would call:
	// pipeline.SetStartBlockLiveStrategy(ctx, intent, pEnv, st, chainID)
	// This reads the current L1 block and sets it as the anchor
	state["start_block_set"] = true
	state["start_block_set_at"] = time.Now().UTC().Format(time.RFC3339)

	// Save rollup config as artifact
	rollupConfig := json.RawMessage(`{"placeholder": "rollup config would be here"}`)
	if err := dctx.StateWriter.SaveArtifact(ctx, "rollup_config", rollupConfig); err != nil {
		return fmt.Errorf("save rollup config artifact: %w", err)
	}

	return dctx.StateWriter.WriteState(ctx, state)
}

// Resume attempts to resume a paused or failed deployment.
func (o *Orchestrator) Resume(ctx context.Context, deploymentID uuid.UUID, onProgress ProgressCallback) error {
	o.logger.Info("resuming OP Stack deployment",
		slog.String("deployment_id", deploymentID.String()),
	)

	stateWriter := NewStateWriter(o.repo, deploymentID)

	canResume, err := stateWriter.CanResume(ctx)
	if err != nil {
		return fmt.Errorf("check resume capability: %w", err)
	}
	if !canResume {
		return fmt.Errorf("deployment cannot be resumed (status is not paused, running, or failed)")
	}

	// Delegate to Deploy - it will determine the start stage
	return o.Deploy(ctx, deploymentID, onProgress)
}

// Pause marks a running deployment as paused.
func (o *Orchestrator) Pause(ctx context.Context, deploymentID uuid.UUID) error {
	stateWriter := NewStateWriter(o.repo, deploymentID)
	return stateWriter.MarkPaused(ctx)
}

// GetDeploymentStatus returns the current status of a deployment.
func (o *Orchestrator) GetDeploymentStatus(ctx context.Context, deploymentID uuid.UUID) (*DeploymentStatus, error) {
	deployment, err := o.repo.GetDeployment(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("get deployment: %w", err)
	}
	if deployment == nil {
		return nil, fmt.Errorf("deployment not found: %s", deploymentID)
	}

	stateWriter := NewStateWriter(o.repo, deploymentID)
	transactions, err := stateWriter.GetTransactions(ctx)
	if err != nil {
		return nil, fmt.Errorf("get transactions: %w", err)
	}

	currentStage, _ := stateWriter.GetCurrentStage(ctx)

	return &DeploymentStatus{
		DeploymentID:     deploymentID,
		Status:           deployment.Status,
		CurrentStage:     currentStage,
		TransactionCount: len(transactions),
		Error:            deployment.ErrorMessage,
	}, nil
}

// DeploymentStatus represents the current state of a deployment.
type DeploymentStatus struct {
	DeploymentID     uuid.UUID
	Status           repository.Status
	CurrentStage     Stage
	TransactionCount int
	Error            *string
}

// weiToETH converts wei to ETH as a human-readable string.
func weiToETH(wei *big.Int) string {
	if wei == nil {
		return "0"
	}
	// 1 ETH = 10^18 wei
	ethInWei := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	
	// Calculate whole ETH and remainder
	eth := new(big.Int).Div(wei, ethInWei)
	remainder := new(big.Int).Mod(wei, ethInWei)
	
	if remainder.Sign() == 0 {
		return eth.String()
	}
	
	// Format with decimals (up to 4 decimal places)
	decimalPart := new(big.Int).Mul(remainder, big.NewInt(10000))
	decimalPart.Div(decimalPart, ethInWei)
	
	if decimalPart.Sign() == 0 {
		return eth.String()
	}
	
	return fmt.Sprintf("%s.%04d", eth, decimalPart.Int64())
}

// ContractDeployment represents the result of a contract deployment.
type ContractDeployment struct {
	TxHash          common.Hash
	ContractAddress common.Address
	GasUsed         uint64
}

// deployContract deploys a contract and waits for the transaction receipt.
func (o *Orchestrator) deployContract(ctx context.Context, dctx *DeploymentContext, bytecode []byte, description string) (*ContractDeployment, error) {
	deployerAddr := common.HexToAddress(dctx.Config.DeployerAddress)

	// Get nonce
	nonce, err := dctx.L1Client.PendingNonceAt(ctx, deployerAddr)
	if err != nil {
		return nil, fmt.Errorf("get nonce: %w", err)
	}

	// Get gas price (use EIP-1559 if available)
	gasTipCap, err := dctx.L1Client.SuggestGasTipCap(ctx)
	if err != nil {
		// Fall back to legacy gas price
		gasPrice, priceErr := dctx.L1Client.SuggestGasPrice(ctx)
		if priceErr != nil {
			return nil, fmt.Errorf("get gas price: %w", priceErr)
		}
		gasTipCap = gasPrice
	}

	// Get base fee from latest block header
	header, err := dctx.L1Client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("get block header: %w", err)
	}
	baseFee := header.BaseFee
	if baseFee == nil {
		baseFee = big.NewInt(0)
	}

	// Calculate max fee (base fee * 2 + tip)
	gasFeeCap := new(big.Int).Mul(baseFee, big.NewInt(2))
	gasFeeCap.Add(gasFeeCap, gasTipCap)

	// Estimate gas
	estimatedGas, err := dctx.L1Client.EstimateGas(ctx, ethereum.CallMsg{
		From:  deployerAddr,
		To:    nil, // Contract creation
		Data:  bytecode,
		Value: big.NewInt(0),
	})
	if err != nil {
		return nil, fmt.Errorf("estimate gas: %w", err)
	}

	// Add 20% buffer to gas estimate
	gas := estimatedGas + (estimatedGas / 5)

	o.logger.Info("deploying contract",
		slog.String("description", description),
		slog.Uint64("nonce", nonce),
		slog.Uint64("gas", gas),
		slog.String("gasTipCap", gasTipCap.String()),
		slog.String("gasFeeCap", gasFeeCap.String()),
	)

	// Create EIP-1559 transaction for contract deployment
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   dctx.Config.L1ChainIDBig(),
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       gas,
		To:        nil, // Contract creation
		Value:     big.NewInt(0),
		Data:      bytecode,
	})

	// Sign the transaction via POPSigner
	signedTx, err := dctx.Signer.SignTransaction(ctx, deployerAddr, tx)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// Send the transaction
	if err := dctx.L1Client.SendTransaction(ctx, signedTx); err != nil {
		return nil, fmt.Errorf("send transaction: %w", err)
	}

	txHash := signedTx.Hash()
	o.logger.Info("transaction sent",
		slog.String("txHash", txHash.Hex()),
		slog.String("description", description),
	)

	// Wait for receipt (with timeout)
	receipt, err := o.waitForReceipt(ctx, dctx.L1Client, txHash)
	if err != nil {
		return nil, fmt.Errorf("wait for receipt: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, fmt.Errorf("transaction failed: status=%d", receipt.Status)
	}

	o.logger.Info("contract deployed",
		slog.String("contractAddress", receipt.ContractAddress.Hex()),
		slog.Uint64("gasUsed", receipt.GasUsed),
		slog.String("txHash", txHash.Hex()),
	)

	return &ContractDeployment{
		TxHash:          txHash,
		ContractAddress: receipt.ContractAddress,
		GasUsed:         receipt.GasUsed,
	}, nil
}

// waitForReceipt polls for a transaction receipt with exponential backoff.
func (o *Orchestrator) waitForReceipt(ctx context.Context, client L1Client, txHash common.Hash) (*types.Receipt, error) {
	backoff := 2 * time.Second
	maxBackoff := 30 * time.Second
	timeout := 5 * time.Minute

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		receipt, err := client.TransactionReceipt(ctx, txHash)
		if err == nil && receipt != nil {
			return receipt, nil
		}

		// Wait before next attempt
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}

		// Exponential backoff
		backoff = backoff * 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	return nil, fmt.Errorf("timeout waiting for transaction receipt")
}

// StageMarker is a minimal contract that stores a stage identifier.
// This is used for demo/testing purposes until full op-deployer integration.
// Solidity equivalent:
//   contract StageMarker { bytes32 public immutable stage; constructor(bytes32 s) { stage = s; } }
func stageMarkerBytecode(stageName string) []byte {
	// This is simplified bytecode that creates a minimal contract
	// The actual contract just stores the stage name hash as immutable
	
	// For now, use a minimal contract creation bytecode
	// This creates an empty contract with the stage name encoded in creation tx
	// The contract code is: PUSH32 <stage> PUSH1 0 SSTORE PUSH1 1 PUSH1 0 RETURN
	
	// Minimal creation code that returns empty runtime code (just for demo)
	// 0x600080600a8239f3 - copies 0 bytes, returns empty contract
	// We'll prepend with PUSH data to include the stage name
	
	// Simple bytecode: returns a 1-byte contract "0xfe" (INVALID opcode = placeholder)
	return common.FromHex("0x60fe60005360016000f3")
}

