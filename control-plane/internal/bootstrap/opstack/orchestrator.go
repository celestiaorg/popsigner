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
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/inspect"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum-optimism/optimism/op-node/rollup"

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
// It manages the deployment lifecycle using the op-deployer pipeline,
// integrates with POPSigner for transaction signing and StateWriter
// for state persistence, enabling resumable deployments.
type Orchestrator struct {
	repo          repository.Repository
	signerFactory SignerFactory
	l1Factory     L1ClientFactory
	opDeployer    *OPDeployer
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

	// Initialize the op-deployer wrapper
	opDeployer := NewOPDeployer(OPDeployerConfig{
		Logger:   logger,
		CacheDir: config.CacheDir,
	})

	return &Orchestrator{
		repo:          repo,
		signerFactory: signerFactory,
		l1Factory:     l1Factory,
		opDeployer:    opDeployer,
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

// Deploy executes an OP Stack deployment using the real op-deployer pipeline.
// It loads the deployment configuration, runs the full op-deployer pipeline,
// and saves all artifacts for bundle generation.
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

	// 4. Update status to running
	if err := stateWriter.UpdateStage(ctx, StageInit); err != nil {
		return fmt.Errorf("update stage: %w", err)
	}

	// 5. Report progress - starting deployment
	if onProgress != nil {
		onProgress(StageInit, 0.1, "Initializing OP Stack deployment...")
	}

	// 6. Run the full op-deployer pipeline
	if err := o.deployWithOPDeployer(ctx, deploymentID, cfg, stateWriter, onProgress); err != nil {
		// Mark as failed with error
		if markErr := stateWriter.MarkFailed(ctx, err.Error()); markErr != nil {
			o.logger.Error("failed to mark deployment as failed",
				slog.String("error", markErr.Error()),
			)
		}
		return err
	}

	// 7. Mark complete - real contracts deployed
	if err := stateWriter.MarkComplete(ctx); err != nil {
		return fmt.Errorf("mark complete: %w", err)
	}

	o.logger.Info("OP Stack deployment completed successfully",
		slog.String("deployment_id", deploymentID.String()),
	)

	if onProgress != nil {
		onProgress(StageCompleted, 1.0, "Deployment completed - OP Stack contracts deployed on L1!")
	}

	return nil
}

// deployWithOPDeployer runs the full op-deployer pipeline.
// This replaces the individual stage handlers with real contract deployments.
func (o *Orchestrator) deployWithOPDeployer(
	ctx context.Context,
	deploymentID uuid.UUID,
	cfg *DeploymentConfig,
	stateWriter *StateWriter,
	onProgress ProgressCallback,
) error {
	// 1. Preflight check - verify deployer has sufficient balance
	if onProgress != nil {
		onProgress(StageInit, 0.1, "Checking deployer balance...")
	}

	if err := o.checkDeployerBalance(ctx, cfg); err != nil {
		return err
	}

	// Get L1 client for later use (StartBlock population)
	l1Client, err := o.l1Factory.Dial(ctx, cfg.L1RPC)
	if err != nil {
		return fmt.Errorf("connect to L1: %w", err)
	}
	defer l1Client.Close()

	// Create POPSigner adapter for op-deployer
	adapter := NewPOPSignerAdapter(
		cfg.POPSignerEndpoint,
		cfg.POPSignerAPIKey,
		new(big.Int).SetUint64(cfg.L1ChainID),
	)

	// Report progress - deploying contracts
	if onProgress != nil {
		onProgress(StageSuperchain, 0.2, "Deploying superchain contracts...")
	}

	// Run the full op-deployer pipeline
	o.logger.Info("starting op-deployer pipeline",
		slog.String("chain_name", cfg.ChainName),
		slog.Uint64("chain_id", cfg.ChainID),
	)

	// Create progress callback wrapper for deployer - updates both DB stage and onProgress callback
	deployerProgress := func(stage string, progress float64, message string) {
		// Map deployer stages to orchestrator stages
		var orchStage Stage
		switch stage {
		case "init":
			orchStage = StageInit
		case "deploy-superchain":
			orchStage = StageSuperchain
		case "deploy-implementations":
			orchStage = StageImplementations
		case "deploy-opchain":
			orchStage = StageOPChain
		case "generate-genesis":
			orchStage = StageGenesis
		case "completed":
			orchStage = StageCompleted
		default:
			orchStage = Stage(stage)
		}

		// CRITICAL: Update stage in database so UI can see progress
		if err := stateWriter.UpdateStage(ctx, orchStage); err != nil {
			o.logger.Warn("failed to update stage in database",
				slog.String("stage", string(orchStage)),
			slog.String("error", err.Error()),
		)
	}

		// Also call onProgress if provided
		if onProgress != nil {
			onProgress(orchStage, progress, message)
		}
	}

	result, err := o.opDeployer.Deploy(ctx, cfg, adapter, deployerProgress)
	if err != nil {
		return fmt.Errorf("op-deployer pipeline failed: %w", err)
	}

	// Report progress - saving artifacts
	if onProgress != nil {
		onProgress(StageGenesis, 0.8, "Saving deployment artifacts...")
	}

	// Save the op-deployer state as artifact for bundle extraction
	stateJSON, err := json.Marshal(result.State)
	if err != nil {
		return fmt.Errorf("marshal op-deployer state: %w", err)
	}

	artifact := &repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ArtifactType: "opdeployer_state",
		Content:      stateJSON,
		CreatedAt:    time.Now(),
	}
	if err := o.repo.SaveArtifact(ctx, artifact); err != nil {
		return fmt.Errorf("save op-deployer state: %w", err)
	}

	// Extract proper genesis.json and rollup.json from op-deployer state
	// Using inspect.GenesisAndRollup to generate the correct formats
	// CRITICAL: genesis.json is REQUIRED - deployment MUST fail without it
	if len(result.ChainStates) == 0 || result.State == nil {
		return fmt.Errorf("no chain states returned from deployment - cannot generate genesis.json")
	}

	chainState := result.ChainStates[0]

	// Ensure StartBlock is populated (required for GenesisAndRollup)
	if chainState.StartBlock == nil {
		o.logger.Info("populating StartBlock from L1 (was nil)")
		header, err := l1Client.HeaderByNumber(ctx, nil)
	if err != nil {
			return fmt.Errorf("failed to get L1 header for StartBlock: %w", err)
		}
		chainState.StartBlock = state.BlockRefJsonFromHeader(header)
		// Also update the state's chain
		for i, c := range result.State.Chains {
			if c.ID == chainState.ID {
				result.State.Chains[i].StartBlock = chainState.StartBlock
				break
			}
		}
	}

	// Generate proper genesis.json and rollup.json using op-deployer's inspect package
	// Wrap in recover() because GenesisAndRollup can panic if L1 block refs are nil
	var l2Genesis *core.Genesis
	var rollupCfg *rollup.Config
	var genErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				genErr = fmt.Errorf("GenesisAndRollup panicked: %v", r)
			}
		}()
		l2Genesis, rollupCfg, genErr = inspect.GenesisAndRollup(result.State, chainState.ID)
	}()

	// CRITICAL: genesis.json is REQUIRED for the chain to work
	if genErr != nil {
		return fmt.Errorf("failed to generate genesis.json (REQUIRED): %w", genErr)
	}
	if l2Genesis == nil {
		return fmt.Errorf("genesis generation returned nil - cannot create OP Stack chain without genesis.json")
	}
	if rollupCfg == nil {
		return fmt.Errorf("rollup config generation returned nil - cannot create OP Stack chain without rollup.json")
	}

	// Save genesis.json (REQUIRED)
	genesisJSON, err := json.MarshalIndent(l2Genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis.json: %w", err)
	}
	genesisArtifact := &repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ArtifactType: "genesis.json",
		Content:      genesisJSON,
		CreatedAt:    time.Now(),
	}
	if err := o.repo.SaveArtifact(ctx, genesisArtifact); err != nil {
		return fmt.Errorf("failed to save genesis.json artifact: %w", err)
	}
	o.logger.Info("saved genesis.json",
		slog.Int("size_bytes", len(genesisJSON)),
		slog.String("chain_id", chainState.ID.Hex()),
	)

	// Save rollup.json (REQUIRED)
	rollupJSON, err := json.MarshalIndent(rollupCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rollup.json: %w", err)
	}
	rollupArtifact := &repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ArtifactType: "rollup.json",
		Content:      rollupJSON,
		CreatedAt:    time.Now(),
	}
	if err := o.repo.SaveArtifact(ctx, rollupArtifact); err != nil {
		return fmt.Errorf("failed to save rollup.json artifact: %w", err)
	}
	o.logger.Info("saved rollup.json",
		slog.Int("size_bytes", len(rollupJSON)),
		slog.String("chain_id", chainState.ID.Hex()),
	)

	// Save contract addresses as deployment_state artifact
	// Extract addresses from the state for the bundle
	addressData := map[string]interface{}{
		"deployment_complete": true,
		"deployed_at":         time.Now().UTC().Format(time.RFC3339),
	}

	if result.SuperchainContracts != nil {
		addressData["superchain_deployment"] = result.SuperchainContracts
	}

	if result.ImplementationsContracts != nil {
		addressData["implementations_deployment"] = result.ImplementationsContracts
	}

	if len(result.ChainStates) > 0 {
		addressData["chain_state"] = result.ChainStates[0]
	}

	addressesJSON, err := json.MarshalIndent(addressData, "", "  ")
	if err != nil {
		o.logger.Warn("failed to marshal addresses", slog.String("error", err.Error()))
	} else {
		addressesArtifact := &repository.Artifact{
			ID:           uuid.New(),
			DeploymentID: deploymentID,
			ArtifactType: "deployment_state",
			Content:      addressesJSON,
			CreatedAt:    time.Now(),
		}
		if err := o.repo.SaveArtifact(ctx, addressesArtifact); err != nil {
			o.logger.Warn("failed to save addresses artifact", slog.String("error", err.Error()))
		}
	}

	o.logger.Info("op-deployer pipeline completed, generating bundle artifacts",
		slog.Int("chains_deployed", len(result.ChainStates)),
	)

	// Generate bundle artifacts (docker-compose, .env, README, etc.)
	if onProgress != nil {
		onProgress(StageGenesis, 0.9, "Generating deployment bundle...")
	}

	extractor := NewArtifactExtractor(o.repo)
	if _, err := extractor.ExtractArtifacts(ctx, deploymentID, cfg); err != nil {
		o.logger.Warn("failed to generate bundle artifacts (deployment still succeeded)",
			slog.String("error", err.Error()),
		)
		// Don't fail the deployment - the raw artifacts are saved
	}

	o.logger.Info("OP Stack deployment and bundle generation completed")

	return nil
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

// checkDeployerBalance verifies the deployer has sufficient ETH for deployment.
// This is called as a preflight check before running the op-deployer pipeline.
func (o *Orchestrator) checkDeployerBalance(ctx context.Context, cfg *DeploymentConfig) error {
	// Connect to L1 to check balance
	l1Client, err := o.l1Factory.Dial(ctx, cfg.L1RPC)
	if err != nil {
		return fmt.Errorf("connect to L1: %w", err)
	}
	defer l1Client.Close()

	// Verify chain ID matches
	actualChainID, err := l1Client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("get L1 chain ID: %w", err)
	}
	expectedChainID := new(big.Int).SetUint64(cfg.L1ChainID)
	if actualChainID.Cmp(expectedChainID) != 0 {
		return fmt.Errorf("L1 chain ID mismatch: expected %d, got %d", cfg.L1ChainID, actualChainID)
	}

	// Check deployer balance
	deployerAddr := common.HexToAddress(cfg.DeployerAddress)
	balance, err := l1Client.BalanceAt(ctx, deployerAddr, nil)
	if err != nil {
		return fmt.Errorf("get deployer balance: %w", err)
	}

	// Apply defaults to get required funding
	cfg.ApplyDefaults()
	requiredFunding := cfg.RequiredFundingWei

	o.logger.Info("preflight balance check",
		slog.String("deployer", deployerAddr.Hex()),
		slog.String("balance_eth", weiToETH(balance)),
		slog.String("required_eth", weiToETH(requiredFunding)),
	)

	if balance.Cmp(requiredFunding) < 0 {
		return fmt.Errorf("insufficient deployer balance for %s: have %s ETH, need %s ETH. Fund this address on L1 and click 'Resume Deployment'",
			deployerAddr.Hex(), weiToETH(balance), weiToETH(requiredFunding))
	}

	o.logger.Info("deployer balance check passed",
		slog.String("deployer", deployerAddr.Hex()),
		slog.String("balance_eth", weiToETH(balance)),
	)

	return nil
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


