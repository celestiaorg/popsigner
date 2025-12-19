// Package opstack provides OP Stack chain deployment infrastructure.
package opstack

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-chain-ops/addresses"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"
	openv "github.com/ethereum-optimism/optimism/op-deployer/pkg/env"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
)

// Artifact content hashes for different OP Stack contract versions
// These are pre-built artifacts hosted on Google Cloud Storage
const (
	// ArtifactHashV5 is the content hash for OP Stack contracts v5.0.0
	ArtifactHashV5 = "b112b16f8939fbb732c0693de3d3bd1e8e3e2f0771f91d5ab300a6c9b7b1af73"
	// ArtifactHashV4_1 is the content hash for OP Stack contracts v4.1.0
	ArtifactHashV4_1 = "579f43b5bbb43e74216b7ed33125280567df86eaf00f7621f354e4a68c07323e"
	// ArtifactHashV4 is the content hash for OP Stack contracts v4.0.0
	ArtifactHashV4 = "67966a2cb9945e1d9ab40e9c61f499e73cdb31d21b8d29a5a5c909b2b13ecd70"

	// DefaultArtifactHash is the default artifact hash to use (v5.0.0)
	DefaultArtifactHash = ArtifactHashV5
)

// OPDeployer wraps the op-deployer library for OP Stack contract deployment.
// It manages the deployment pipeline stages and integrates with POPSigner for
// transaction signing.
type OPDeployer struct {
	logger   *slog.Logger
	cacheDir string
}

// OPDeployerConfig contains configuration for the OPDeployer.
type OPDeployerConfig struct {
	Logger   *slog.Logger
	CacheDir string // Directory for caching downloaded artifacts
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
}

// Deploy executes a full OP Stack deployment using the op-deployer pipeline.
// It runs all pipeline stages: Init, DeploySuperchain, DeployImplementations,
// DeployOPChain, and GenerateL2Genesis.
func (d *OPDeployer) Deploy(ctx context.Context, cfg *DeploymentConfig, signerAdapter *POPSignerAdapter) (*DeployResult, error) {
	d.logger.Info("starting OP Stack deployment via op-deployer",
		slog.String("chain_name", cfg.ChainName),
		slog.Uint64("chain_id", cfg.ChainID),
		slog.Uint64("l1_chain_id", cfg.L1ChainID),
	)

	// 1. Build the Intent from our config
	intent, err := BuildIntent(cfg)
	if err != nil {
		return nil, fmt.Errorf("build intent: %w", err)
	}

	// 2. Initialize state
	st := &state.State{
		Version: 1,
	}

	// 3. Connect to L1
	rpcClient, err := rpc.DialContext(ctx, cfg.L1RPC)
	if err != nil {
		return nil, fmt.Errorf("dial L1 RPC: %w", err)
	}
	defer rpcClient.Close()

	l1Client := ethclient.NewClient(rpcClient)

	// 4. Download artifacts from HTTPS (pre-built artifacts on Google Cloud Storage)
	d.logger.Info("downloading contract artifacts from HTTPS",
		slog.String("artifact_hash", DefaultArtifactHash),
	)

	// Use HTTPS locator instead of embedded artifacts
	artifactURL := artifacts.CreateHttpLocator(DefaultArtifactHash)
	locator := artifacts.MustNewLocatorFromURL(artifactURL)

	l1Artifacts, err := artifacts.Download(ctx, locator, nil, d.cacheDir)
	if err != nil {
		return nil, fmt.Errorf("download L1 artifacts: %w", err)
	}

	// L2 artifacts use the same locator (same contract package)
	l2Artifacts, err := artifacts.Download(ctx, locator, nil, d.cacheDir)
	if err != nil {
		return nil, fmt.Errorf("download L2 artifacts: %w", err)
	}

	bundle := pipeline.ArtifactsBundle{
		L1: l1Artifacts,
		L2: l2Artifacts,
	}

	d.logger.Info("artifacts downloaded successfully")

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

	// 9. Run pipeline stages
	d.logger.Info("running pipeline: InitLiveStrategy")
	if err := pipeline.InitLiveStrategy(ctx, env, intent, st); err != nil {
		return nil, fmt.Errorf("init live strategy: %w", err)
	}

	// Broadcast any queued transactions and check for errors
	if _, err := bcaster.Broadcast(ctx); err != nil {
		return nil, fmt.Errorf("broadcast init transactions: %w", err)
	}

	d.logger.Info("running pipeline: DeploySuperchain")
	if err := pipeline.DeploySuperchain(env, intent, st); err != nil {
		return nil, fmt.Errorf("deploy superchain: %w", err)
	}
	if _, err := bcaster.Broadcast(ctx); err != nil {
		return nil, fmt.Errorf("broadcast superchain transactions: %w", err)
	}

	d.logger.Info("running pipeline: DeployImplementations")
	if err := pipeline.DeployImplementations(env, intent, st); err != nil {
		return nil, fmt.Errorf("deploy implementations: %w", err)
	}
	if _, err := bcaster.Broadcast(ctx); err != nil {
		return nil, fmt.Errorf("broadcast implementations transactions: %w", err)
	}

	// Deploy each chain
	for _, chainIntent := range intent.Chains {
		d.logger.Info("running pipeline: DeployOPChain", slog.String("chain_id", chainIntent.ID.Hex()))
		if err := pipeline.DeployOPChain(env, intent, st, chainIntent.ID); err != nil {
			return nil, fmt.Errorf("deploy OP chain %s: %w", chainIntent.ID.Hex(), err)
		}
		if _, err := bcaster.Broadcast(ctx); err != nil {
			return nil, fmt.Errorf("broadcast OP chain transactions: %w", err)
		}

		d.logger.Info("running pipeline: GenerateL2Genesis", slog.String("chain_id", chainIntent.ID.Hex()))
		if err := pipeline.GenerateL2Genesis(env, intent, bundle, st, chainIntent.ID); err != nil {
			return nil, fmt.Errorf("generate L2 genesis %s: %w", chainIntent.ID.Hex(), err)
		}
	}

	// 10. Store the applied intent
	st.AppliedIntent = intent

	d.logger.Info("OP Stack deployment completed successfully",
		slog.Int("chains_deployed", len(st.Chains)),
	)

	return &DeployResult{
		State:                    st,
		SuperchainContracts:      st.SuperchainDeployment,
		ImplementationsContracts: st.ImplementationsDeployment,
		ChainStates:              st.Chains,
	}, nil
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
