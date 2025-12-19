// Package opstack provides OP Stack chain deployment infrastructure.
package opstack

import (
	"context"
	"log/slog"
	"os"

	"github.com/ethereum-optimism/optimism/op-chain-ops/addresses"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
)

// OPDeployer wraps the op-deployer library for OP Stack contract deployment.
// It manages the deployment pipeline stages and integrates with POPSigner for
// transaction signing.
//
// NOTE: Full op-deployer integration requires embedded artifacts that are only
// available when building from within the optimism monorepo. When importing
// the library as a Go module, the embedded artifacts are not available.
// This implementation provides placeholder deployment results that enable
// bundle generation (docker-compose, .env, README), while actual contract
// deployment should be done using the op-deployer CLI.
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
//
// NOTE: Currently returns a placeholder result because embedded artifacts are
// not available when importing op-deployer as a Go module. The generated bundle
// (docker-compose, .env, README) is still useful for users to run op-deployer CLI.
func (d *OPDeployer) Deploy(ctx context.Context, cfg *DeploymentConfig, signerAdapter *POPSignerAdapter) (*DeployResult, error) {
	d.logger.Info("starting OP Stack deployment via op-deployer",
		slog.String("chain_name", cfg.ChainName),
		slog.Uint64("chain_id", cfg.ChainID),
		slog.Uint64("l1_chain_id", cfg.L1ChainID),
	)

	// NOTE: The embedded artifacts from op-deployer are not available when importing as a Go module.
	// This is a known limitation - the artifacts are only embedded when building from within the
	// optimism monorepo. For now, we skip the full op-deployer pipeline and return a result
	// that allows bundle generation with placeholder genesis/rollup configs.
	d.logger.Warn("op-deployer embedded artifacts not available - using placeholder deployment",
		slog.String("note", "Full OP Stack contract deployment requires op-deployer CLI or building from optimism monorepo"),
	)

	// Return a placeholder result that allows bundle generation
	// The user will need to run op-deployer CLI separately to deploy actual contracts
	return d.createPlaceholderResult(cfg)
}

// createPlaceholderResult creates a deployment result with placeholder data.
// This allows bundle generation (docker-compose, .env, README) while actual contract
// deployment is deferred to op-deployer CLI.
func (d *OPDeployer) createPlaceholderResult(cfg *DeploymentConfig) (*DeployResult, error) {
	d.logger.Info("creating placeholder deployment result for bundle generation")

	// Create a minimal chain state for bundle generation
	// Note: The actual genesis and rollup config will be generated when user runs op-deployer CLI
	chainState := &state.ChainState{}

	return &DeployResult{
		State:       &state.State{Version: 1},
		ChainStates: []*state.ChainState{chainState},
	}, nil
}
