// Package opstack provides OP Stack chain deployment infrastructure.
package opstack

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-chain-ops/addresses"
	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	openv "github.com/ethereum-optimism/optimism/op-deployer/pkg/env"
	opcrypto "github.com/ethereum-optimism/optimism/op-service/crypto"
)

// SignerAdapter is an interface for transaction signing adapters.
// Both POPSignerAdapter (HTTP-based) and AnvilSigner (local ECDSA) implement this.
type SignerAdapter interface {
	SignerFn() opcrypto.SignerFn
}

// OPDeployer wraps the op-deployer library for OP Stack contract deployment.
// It manages the deployment pipeline stages and integrates with POPSigner for
// transaction signing.
type OPDeployer struct {
	logger      *slog.Logger
	cacheDir    string
	infraMgr    *InfrastructureManager
}

// OPDeployerConfig contains configuration for the OPDeployer.
type OPDeployerConfig struct {
	Logger                 *slog.Logger
	CacheDir               string                    // Directory for caching downloaded artifacts
	InfrastructureManager  *InfrastructureManager    // Optional: for infrastructure reuse
}

// NewOPDeployer creates a new OPDeployer instance.
func NewOPDeployer(cfg OPDeployerConfig) *OPDeployer {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		cacheDir = os.TempDir()
	}

	return &OPDeployer{
		logger:   logger,
		cacheDir: cacheDir,
		infraMgr: cfg.InfrastructureManager,
	}
}

// DeployResult contains the result of an OP Stack deployment.
type DeployResult struct {
	// State is the complete deployment state from op-deployer
	State *state.State

	// SuperchainContracts contains addresses of superchain contracts
	SuperchainContracts *addresses.SuperchainContracts

	// ImplementationsContracts contains addresses of implementation contracts
	ImplementationsContracts *addresses.ImplementationsContracts

	// ChainStates contains state for each deployed chain
	ChainStates []*state.ChainState

	// InfrastructureReused indicates if existing infrastructure was used
	InfrastructureReused bool

	// Create2Salt used for this deployment
	Create2Salt common.Hash
}

// DeployerProgressCallback reports deployment progress from the deployer.
// Uses string stage names which are mapped to Stage constants by the orchestrator.
type DeployerProgressCallback func(stage string, progress float64, message string)

// Deploy executes a full OP Stack deployment using the op-deployer pipeline.
// It runs all pipeline stages: Init, DeploySuperchain, DeployImplementations,
// DeployOPChain, and GenerateL2Genesis.
//
// Behavior depends on ReuseInfrastructure in config:
// - false: ISOLATED deployment with fresh OPCM, blueprints, infrastructure
// - true: Reuses existing infrastructure (~10x cheaper, faster)
//
// For isolated deployments, CREATE2 salt = hash(chainName + chainID + artifactVersion).
func (d *OPDeployer) Deploy(ctx context.Context, cfg *DeploymentConfig, signerAdapter SignerAdapter, onProgress DeployerProgressCallback) (*DeployResult, error) {
	// Calculate salt upfront for logging
	salt := GetDeploymentSalt(cfg.ChainName, cfg.ChainID)
	infraReused := false

	// Check for infrastructure reuse
	if cfg.ReuseInfrastructure && d.infraMgr != nil {
		existingInfra, err := d.infraMgr.GetExistingInfrastructure(ctx, cfg.L1ChainID, ArtifactVersion)
		if err != nil {
			d.logger.Warn("failed to check for existing infrastructure, proceeding with fresh deployment",
				slog.String("error", err.Error()),
			)
		} else if existingInfra != nil {
			// Populate config with existing infrastructure addresses
			d.infraMgr.PopulateConfigFromInfra(cfg, existingInfra)
			// Use the same salt as the existing infrastructure for consistency
			salt = existingInfra.Create2Salt
			infraReused = true
			d.logger.Info("reusing existing OP Stack infrastructure",
				slog.Uint64("l1_chain_id", cfg.L1ChainID),
				slog.String("opcm_address", cfg.ExistingOPCMAddress),
				slog.String("version", existingInfra.Version),
			)
		}
	}

	deployMode := "ISOLATED"
	if infraReused {
		deployMode = "REUSE"
	}

	d.logger.Info(fmt.Sprintf("starting %s OP Stack deployment via op-deployer", deployMode),
		slog.String("chain_name", cfg.ChainName),
		slog.Uint64("chain_id", cfg.ChainID),
		slog.Uint64("l1_chain_id", cfg.L1ChainID),
		slog.String("artifact_version", ArtifactVersion),
		slog.String("create2_salt", salt.Hex()),
		slog.String("deployer", cfg.DeployerAddress),
		slog.Bool("reusing_infrastructure", infraReused),
	)

	// 1. Build the Intent from our config (optionally with OPCM address for reuse)
	intent, err := BuildIntent(cfg)
	if err != nil {
		return nil, fmt.Errorf("build intent: %w", err)
	}

	// 2. Initialize state with the appropriate salt
	var st *state.State
	if infraReused {
		// For reuse, use a unique salt for this chain (not the infra salt)
		chainSalt := GetDeploymentSalt(cfg.ChainName, cfg.ChainID)
		st = &state.State{
			Version:     1,
			Create2Salt: chainSalt,
		}
	} else {
		st = BuildState(cfg.ChainName, cfg.ChainID)
	}

	d.logger.Info("intent and state built",
		slog.String("create2_salt", st.Create2Salt.Hex()),
		slog.Bool("opcmAddress_is_nil", intent.OPCMAddress == nil),
		slog.Bool("reusing_infrastructure", infraReused),
	)

	// 3. Connect to L1
	rpcClient, err := rpc.DialContext(ctx, cfg.L1RPC)
	if err != nil {
		return nil, fmt.Errorf("dial L1 RPC: %w", err)
	}
	defer rpcClient.Close()

	l1Client := ethclient.NewClient(rpcClient)

	// 4. Download and extract artifacts ourselves to avoid op-deployer's
	// finicky directory structure expectations
	d.logger.Info("downloading contract artifacts",
		slog.String("url", ContractArtifactURL),
	)

	// Clean any cached artifacts from op-deployer's cache to force fresh download
	// This ensures we always use the latest artifacts from S3
	if err := d.cleanArtifactCache(); err != nil {
		d.logger.Warn("failed to clean artifact cache", slog.String("error", err.Error()))
	}

	artifactDownloader := NewContractArtifactDownloader(d.cacheDir)
	artifactDir, err := artifactDownloader.Download(ctx, ContractArtifactURL)
	if err != nil {
		return nil, fmt.Errorf("download artifacts: %w", err)
	}

	d.logger.Info("artifacts downloaded and extracted",
		slog.String("path", artifactDir),
	)

	// Create file:// locator pointing to our extracted artifacts
	// op-deployer's file handler correctly looks for forge-artifacts/ subdirectory
	fileLocator, err := artifacts.NewFileLocator(artifactDir)
	if err != nil {
		return nil, fmt.Errorf("create file locator: %w", err)
	}

	// Update intent to use our local artifacts
	intent.L1ContractsLocator = fileLocator
	intent.L2ContractsLocator = fileLocator

	// Now use op-deployer's Download which will just use os.DirFS for file:// locators
	l1Artifacts, err := artifacts.Download(ctx, intent.L1ContractsLocator, nil, d.cacheDir)
	if err != nil {
		return nil, fmt.Errorf("load L1 artifacts: %w", err)
	}

	// L2 uses same artifacts
	l2Artifacts := l1Artifacts

	bundle := pipeline.ArtifactsBundle{
		L1: l1Artifacts,
		L2: l2Artifacts,
	}

	d.logger.Info("artifacts downloaded successfully")

	// Debug: Log bytecode sizes from our downloaded artifacts
	d.logger.Info("checking bytecode sizes from downloaded artifacts", slog.String("path", artifactDir))
	d.logBytecodeSizes(artifactDir)

	// 5. Create broadcaster with our signer
	deployerAddr := common.HexToAddress(cfg.DeployerAddress)
	chainID := new(big.Int).SetUint64(cfg.L1ChainID)

	bcaster, err := broadcaster.NewKeyedBroadcaster(broadcaster.KeyedBroadcasterOpts{
		Logger:  d.gethLogger(),
		ChainID: chainID,
		Client:  l1Client,
		Signer:  signerAdapter.SignerFn(),
		From:    deployerAddr,
	})
	if err != nil {
		return nil, fmt.Errorf("create broadcaster: %w", err)
	}

	// 6. Create script host for L1
	l1Host, err := openv.DefaultForkedScriptHost(
		ctx,
		bcaster,
		d.gethLogger(),
		deployerAddr,
		l1Artifacts,
		rpcClient,
	)
	if err != nil {
		return nil, fmt.Errorf("create L1 script host: %w", err)
	}

	// 7. Load all deployment scripts
	scripts, err := opcm.NewScripts(l1Host)
	if err != nil {
		return nil, fmt.Errorf("load deployment scripts: %w", err)
	}

	// Debug: Verify scripts are loaded correctly
	d.logger.Info("scripts loaded successfully",
		slog.Bool("hasDeployImplementations", scripts.DeployImplementations != nil),
		slog.Bool("hasDeployOPChain", scripts.DeployOPChain != nil),
		slog.Bool("hasDeploySuperchain", scripts.DeploySuperchain != nil),
	)

	// 8. Create pipeline environment
	env := &pipeline.Env{
		StateWriter:  d.stateWriter(st),
		L1ScriptHost: l1Host,
		L1Client:     l1Client,
		Broadcaster:  bcaster,
		Deployer:     deployerAddr,
		Logger:       d.gethLogger(),
		Scripts:      scripts,
	}

	// Helper to refresh fork after broadcast to pick up newly confirmed contracts
	refreshFork := func(stage string) error {
		latest, err := l1Client.HeaderByNumber(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to get latest block for %s: %w", stage, err)
		}
		if _, err := l1Host.CreateSelectFork(
			script.ForkWithURLOrAlias("main"),
			script.ForkWithBlockNumberU256(latest.Number),
		); err != nil {
			return fmt.Errorf("failed to refresh fork for %s: %w", stage, err)
		}
		d.logger.Info("fork refreshed after broadcast",
			slog.String("stage", stage),
			slog.Uint64("block", latest.Number.Uint64()),
		)
		return nil
	}

	// Helper to report progress
	reportProgress := func(stage string, progress float64, message string) {
		d.logger.Info(message, slog.String("stage", stage), slog.Float64("progress", progress))
		if onProgress != nil {
			onProgress(stage, progress, message)
		}
	}

	// 9. Run pipeline stages
	reportProgress("init", 0.1, "Initializing deployment strategy...")
	d.logger.Info("running pipeline: InitLiveStrategy")
	if err := pipeline.InitLiveStrategy(ctx, env, intent, st); err != nil {
		return nil, fmt.Errorf("init live strategy: %w", err)
	}

	// Broadcast any queued transactions and check for errors
	if _, err := bcaster.Broadcast(ctx); err != nil {
		return nil, fmt.Errorf("broadcast init transactions: %w", err)
	}
	// Refresh fork to pick up init transactions
	if err := refreshFork("init"); err != nil {
		return nil, err
	}

	reportProgress("deploy-superchain", 0.2, "Deploying Superchain contracts (ProtocolVersions, SuperchainConfig)...")
	d.logger.Info("running pipeline: DeploySuperchain")
	if err := pipeline.DeploySuperchain(env, intent, st); err != nil {
		return nil, fmt.Errorf("deploy superchain: %w", err)
	}
	if _, err := bcaster.Broadcast(ctx); err != nil {
		return nil, fmt.Errorf("broadcast superchain transactions: %w", err)
	}
	// Refresh fork to pick up superchain contracts
	if err := refreshFork("deploy-superchain"); err != nil {
		return nil, err
	}

	reportProgress("deploy-implementations", 0.35, "Deploying Implementation contracts (OPCM, blueprints, dispute games)...")
	d.logger.Info("running pipeline: DeployImplementations")
	if err := pipeline.DeployImplementations(env, intent, st); err != nil {
		return nil, fmt.Errorf("deploy implementations: %w", err)
	}
	if _, err := bcaster.Broadcast(ctx); err != nil {
		return nil, fmt.Errorf("broadcast implementations transactions: %w", err)
	}
	// Refresh fork to pick up implementation contracts (CRITICAL for blueprints!)
	if err := refreshFork("deploy-implementations"); err != nil {
		return nil, err
	}

	// Debug: Log implementations deployment result
	if st.ImplementationsDeployment != nil {
		impl := st.ImplementationsDeployment
		d.logger.Info("implementations deployed successfully",
			slog.String("opcmImpl", impl.OpcmImpl.Hex()),
			slog.String("opcmContractsContainerImpl", impl.OpcmContractsContainerImpl.Hex()), // LEGACY - stores blueprints, MUST be non-zero!
			slog.String("opcmDeployerImpl", impl.OpcmDeployerImpl.Hex()),
			slog.String("opcmV2Impl", impl.OpcmV2Impl.Hex()),               // V2 - zero if V2 disabled
			slog.String("opcmContainerImpl", impl.OpcmContainerImpl.Hex()), // V2 - zero if V2 disabled
			slog.String("delayedWETHImpl", impl.DelayedWethImpl.Hex()),
			slog.String("optimismPortalImpl", impl.OptimismPortalImpl.Hex()),
			slog.String("systemConfigImpl", impl.SystemConfigImpl.Hex()),
			slog.String("mipsImpl", impl.MipsImpl.Hex()),
			slog.String("preimageOracleImpl", impl.PreimageOracleImpl.Hex()),
		)

		// Critical check: OpcmContractsContainerImpl (LEGACY) MUST be non-zero
		if impl.OpcmContractsContainerImpl == (common.Address{}) {
			d.logger.Error("CRITICAL: OpcmContractsContainerImpl is ZERO - blueprints not deployed correctly!")
		} else {
			d.logger.Info("OpcmContractsContainerImpl is NON-ZERO - blueprints should be available",
				slog.String("address", impl.OpcmContractsContainerImpl.Hex()),
			)
		}

		// Log dispute game implementation addresses
		d.logger.Info("dispute game implementations from state",
			slog.String("faultDisputeGameV2Impl", impl.FaultDisputeGameV2Impl.Hex()),
			slog.String("permissionedDisputeGameV2Impl", impl.PermissionedDisputeGameV2Impl.Hex()),
			slog.String("disputeGameFactoryImpl", impl.DisputeGameFactoryImpl.Hex()),
		)

		// Verify OPCM-related contract code exists in simulation state
		d.logger.Info("verifying OPCM-related contracts in simulation state")
		d.verifyBlueprintCode(l1Host, "OpcmImpl", impl.OpcmImpl)
		d.verifyBlueprintCode(l1Host, "OpcmDeployerImpl", impl.OpcmDeployerImpl)
		d.verifyBlueprintCode(l1Host, "OpcmContainerImpl", impl.OpcmContainerImpl)
		d.verifyBlueprintCode(l1Host, "FaultDisputeGameV2Impl", impl.FaultDisputeGameV2Impl)
		d.verifyBlueprintCode(l1Host, "PermissionedDisputeGameV2Impl", impl.PermissionedDisputeGameV2Impl)

		// Check if these addresses have code on L1 (not just simulation)
		d.logger.Info("checking if OPCM contracts exist on L1 (outside simulation)")
		d.checkL1Code(ctx, l1Client, "OpcmDeployerImpl", impl.OpcmDeployerImpl)
		d.checkL1Code(ctx, l1Client, "OpcmContainerImpl", impl.OpcmContainerImpl)
	} else {
		d.logger.Warn("implementations deployment returned nil state")
	}

	// Deploy each chain
	for i, chainIntent := range intent.Chains {
		chainProgress := 0.5 + float64(i)*0.3/float64(len(intent.Chains))
		reportProgress("deploy-opchain", chainProgress, fmt.Sprintf("Deploying OP Chain contracts (proxies, bridges, system config)..."))
		d.logger.Info("running pipeline: DeployOPChain", slog.String("chain_id", chainIntent.ID.Hex()))

		// Debug: Re-verify contracts before DeployOPChain (this is where NotABlueprint occurs)
		if st.ImplementationsDeployment != nil {
			impl := st.ImplementationsDeployment
			d.logger.Info("=== PRE-DeployOPChain VERIFICATION ===",
				slog.String("chainID", chainIntent.ID.Hex()),
			)

			// CRITICAL: The LEGACY container stores blueprint addresses
			d.logger.Info("verifying OpcmContractsContainerImpl (LEGACY - stores blueprint addresses)")
			d.verifyBlueprintCode(l1Host, "OpcmContractsContainerImpl", impl.OpcmContractsContainerImpl)
			d.checkL1Code(ctx, l1Client, "OpcmContractsContainerImpl-L1", impl.OpcmContractsContainerImpl)

			// Query blueprints from container on L1
			d.queryBlueprintsFromContainer(ctx, l1Client, impl.OpcmContractsContainerImpl)

			// The deployer that calls into the container
			d.logger.Info("verifying OPContractsManagerDeployer (where error occurs)")
			d.verifyBlueprintCode(l1Host, "OpcmDeployerImpl", impl.OpcmDeployerImpl)
			d.checkL1Code(ctx, l1Client, "OpcmDeployerImpl-L1", impl.OpcmDeployerImpl)

			// V2 contracts (expected to be zero if V2 disabled)
			d.logger.Info("verifying V2 contracts (expected zero if V2 disabled)")
			d.verifyBlueprintCode(l1Host, "OpcmContainerImpl-V2", impl.OpcmContainerImpl)
		}

		if err := pipeline.DeployOPChain(env, intent, st, chainIntent.ID); err != nil {
			return nil, fmt.Errorf("deploy OP chain %s: %w", chainIntent.ID.Hex(), err)
		}
		if _, err := bcaster.Broadcast(ctx); err != nil {
			return nil, fmt.Errorf("broadcast OP chain transactions: %w", err)
		}

		genesisProgress := 0.7 + float64(i)*0.1/float64(len(intent.Chains))
		reportProgress("generate-genesis", genesisProgress, "Generating L2 genesis and rollup configuration...")
		d.logger.Info("running pipeline: GenerateL2Genesis", slog.String("chain_id", chainIntent.ID.Hex()))
		if err := pipeline.GenerateL2Genesis(env, intent, bundle, st, chainIntent.ID); err != nil {
			return nil, fmt.Errorf("generate L2 genesis %s: %w", chainIntent.ID.Hex(), err)
		}
	}

	reportProgress("completed", 0.85, "Contract deployment completed, preparing artifacts...")

	// 10. Store the applied intent
	st.AppliedIntent = intent

	result := &DeployResult{
		State:                    st,
		SuperchainContracts:      st.SuperchainDeployment,
		ImplementationsContracts: st.ImplementationsDeployment,
		ChainStates:              st.Chains,
		InfrastructureReused:     infraReused,
		Create2Salt:              st.Create2Salt,
	}

	// 11. Save infrastructure for future reuse (only on first deployment)
	if !infraReused && d.infraMgr != nil {
		if err := d.infraMgr.SaveInfrastructure(ctx, cfg.L1ChainID, result, st.Create2Salt, nil); err != nil {
			d.logger.Warn("failed to save infrastructure for reuse",
				slog.String("error", err.Error()),
			)
			// Don't fail the deployment, just log the warning
		} else {
			d.logger.Info("infrastructure saved for future reuse",
				slog.Uint64("l1_chain_id", cfg.L1ChainID),
				slog.String("version", ArtifactVersion),
			)
		}
	}

	d.logger.Info("OP Stack deployment completed successfully",
		slog.Int("chains_deployed", len(st.Chains)),
		slog.Bool("infrastructure_reused", infraReused),
	)

	return result, nil
}

// stateWriter returns a pipeline.StateWriter that updates the given state.
func (d *OPDeployer) stateWriter(st *state.State) pipeline.StateWriter {
	return stateWriterFunc(func(newState *state.State) error {
		*st = *newState
		return nil
	})
}

// stateWriterFunc is a function adapter for pipeline.StateWriter.
type stateWriterFunc func(st *state.State) error

func (f stateWriterFunc) WriteState(st *state.State) error {
	return f(st)
}

// gethLogger creates a go-ethereum compatible logger from slog.
func (d *OPDeployer) gethLogger() log.Logger {
	return log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stderr, log.LevelInfo, true))
}

// cleanArtifactCache removes all cached artifacts to force fresh downloads.
// This ensures we always use the latest artifacts from S3.
func (d *OPDeployer) cleanArtifactCache() error {
	// Clean our custom download cache (artifacts-* directories and *.tzst files)
	entries, err := os.ReadDir(d.cacheDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		// Remove artifact directories and tzst files
		if strings.HasPrefix(name, "artifacts-") || strings.HasSuffix(name, ".tzst") {
			path := filepath.Join(d.cacheDir, name)
			d.logger.Info("cleaning cached artifact", slog.String("path", path))
			if err := os.RemoveAll(path); err != nil {
				d.logger.Warn("failed to remove cached artifact",
					slog.String("path", path),
					slog.String("error", err.Error()))
			}
		}
	}
	return nil
}

