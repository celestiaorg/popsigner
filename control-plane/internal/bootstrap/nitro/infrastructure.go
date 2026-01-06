// Package nitro provides Nitro chain deployment infrastructure.
package nitro

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/repository"
)

// InfrastructureDeployer handles deployment of shared Nitro infrastructure.
// The RollupCreator and associated contracts are deployed once per parent chain
// and reused for all customer rollups on that chain.
type InfrastructureDeployer struct {
	artifacts *NitroArtifacts
	signer    *NitroSigner
	repo      repository.NitroInfrastructureRepository
	logger    *slog.Logger
}

// InfrastructureConfig contains configuration for infrastructure deployment.
type InfrastructureConfig struct {
	// ParentChainID is the chain ID of the parent chain
	ParentChainID int64
	// ParentRPC is the RPC URL for the parent chain
	ParentRPC string
	// DeployerID is the ID of the user deploying (for audit)
	DeployerID *uuid.UUID
}

// InfrastructureResult contains the result of infrastructure deployment.
type InfrastructureResult struct {
	RollupCreatorAddress   common.Address
	BridgeCreatorAddress   common.Address
	Version                string
	DeploymentTxHash       common.Hash
	AlreadyDeployed        bool
}

// NewInfrastructureDeployer creates a new infrastructure deployer.
func NewInfrastructureDeployer(
	artifacts *NitroArtifacts,
	signer *NitroSigner,
	repo repository.NitroInfrastructureRepository,
	logger *slog.Logger,
) *InfrastructureDeployer {
	return &InfrastructureDeployer{
		artifacts: artifacts,
		signer:    signer,
		repo:      repo,
		logger:    logger,
	}
}

// EnsureInfrastructure ensures that Nitro infrastructure is deployed on the parent chain.
// If infrastructure already exists and is the correct version, it returns the existing addresses.
// If infrastructure is missing or outdated, it deploys new infrastructure.
func (d *InfrastructureDeployer) EnsureInfrastructure(
	ctx context.Context,
	cfg *InfrastructureConfig,
) (*InfrastructureResult, error) {
	d.logger.Info("checking for existing Nitro infrastructure",
		slog.Int64("parent_chain_id", cfg.ParentChainID),
	)

	// Check if infrastructure already exists (only if repo is available)
	if d.repo != nil {
		existing, err := d.repo.Get(ctx, cfg.ParentChainID)
		if err != nil {
			d.logger.Warn("failed to check existing infrastructure, will deploy new",
				slog.String("error", err.Error()),
			)
		} else if existing != nil && existing.Version == d.artifacts.Version {
			d.logger.Info("using existing Nitro infrastructure",
				slog.String("rollup_creator", existing.RollupCreatorAddress),
				slog.String("version", existing.Version),
			)
			return &InfrastructureResult{
				RollupCreatorAddress: common.HexToAddress(existing.RollupCreatorAddress),
				BridgeCreatorAddress: common.HexToAddress(ptrToString(existing.BridgeCreatorAddress)),
				Version:              existing.Version,
				AlreadyDeployed:      true,
			}, nil
		}
	} else {
		d.logger.Info("no infrastructure repository configured, will deploy new infrastructure")
	}

	// Connect to parent chain
	client, err := ethclient.DialContext(ctx, cfg.ParentRPC)
	if err != nil {
		return nil, fmt.Errorf("connect to parent chain: %w", err)
	}
	defer client.Close()

	// Verify chain ID matches
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("get chain ID: %w", err)
	}
	if chainID.Int64() != cfg.ParentChainID {
		return nil, fmt.Errorf("chain ID mismatch: expected %d, got %d", cfg.ParentChainID, chainID.Int64())
	}

	d.logger.Info("deploying Nitro infrastructure",
		slog.Int64("parent_chain_id", cfg.ParentChainID),
		slog.String("version", d.artifacts.Version),
	)

	// Deploy infrastructure contracts
	result, err := d.deployInfrastructure(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("deploy infrastructure: %w", err)
	}

	// Save to database
	infraRecord := &repository.NitroInfrastructure{
		ParentChainID:        cfg.ParentChainID,
		RollupCreatorAddress: result.RollupCreatorAddress.Hex(),
		Version:              d.artifacts.Version,
		DeployedBy:           cfg.DeployerID,
	}
	if result.BridgeCreatorAddress != (common.Address{}) {
		addr := result.BridgeCreatorAddress.Hex()
		infraRecord.BridgeCreatorAddress = &addr
	}
	if result.DeploymentTxHash != (common.Hash{}) {
		hash := result.DeploymentTxHash.Hex()
		infraRecord.DeploymentTxHash = &hash
	}

	// Save to database if repo is available
	if d.repo != nil {
		if err := d.repo.Upsert(ctx, infraRecord); err != nil {
			d.logger.Warn("failed to save infrastructure to database, but deployment succeeded",
				slog.String("error", err.Error()),
			)
		}
	} else {
		d.logger.Info("no infrastructure repository configured, skipping database save")
	}

	d.logger.Info("Nitro infrastructure deployed successfully",
		slog.String("rollup_creator", result.RollupCreatorAddress.Hex()),
		slog.String("version", d.artifacts.Version),
	)

	return result, nil
}

// deployInfrastructure deploys all required infrastructure contracts.
// This is a complex multi-step process:
// 1. Deploy OneStepProvers (OSP0, OSPMemory, OSPMath, OSPHostIo)
// 2. Deploy OneStepProofEntry (references provers)
// 3. Deploy ChallengeManager template
// 4. Deploy Rollup contracts (Bridge, Inbox, Outbox, SequencerInbox)
// 5. Deploy BridgeCreator
// 6. Deploy RollupCreator (the main entry point)
func (d *InfrastructureDeployer) deployInfrastructure(
	ctx context.Context,
	client *ethclient.Client,
) (*InfrastructureResult, error) {
	// For now, we use a simplified deployment that assumes:
	// 1. Official Arbitrum RollupCreator is already deployed on testnets/mainnet
	// 2. For local Anvil, we need to deploy everything
	//
	// In production, we'd check for existing deployments first.

	// Get the deployer's nonce
	nonce, err := client.PendingNonceAt(ctx, d.signer.Address())
	if err != nil {
		return nil, fmt.Errorf("get nonce: %w", err)
	}

	// Get gas price
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("get gas price: %w", err)
	}
	// Boost gas price by 50%
	gasPrice = new(big.Int).Mul(gasPrice, big.NewInt(150))
	gasPrice = new(big.Int).Div(gasPrice, big.NewInt(100))

	chainID := d.signer.ChainID()

	// Deploy RollupCreator
	// Note: In a full implementation, we'd deploy all dependencies first.
	// For now, we deploy RollupCreator which is the main entry point.
	
	rollupCreatorAddr, txHash, err := d.deployContract(
		ctx,
		client,
		d.artifacts.RollupCreator,
		nonce,
		gasPrice,
		chainID,
		nil, // No constructor args for now
	)
	if err != nil {
		return nil, fmt.Errorf("deploy RollupCreator: %w", err)
	}

	d.logger.Info("RollupCreator deployed",
		slog.String("address", rollupCreatorAddr.Hex()),
		slog.String("tx_hash", txHash.Hex()),
	)

	return &InfrastructureResult{
		RollupCreatorAddress: rollupCreatorAddr,
		Version:              d.artifacts.Version,
		DeploymentTxHash:     txHash,
		AlreadyDeployed:      false,
	}, nil
}

// deployContract deploys a single contract and waits for confirmation.
func (d *InfrastructureDeployer) deployContract(
	ctx context.Context,
	client *ethclient.Client,
	artifact *ContractArtifact,
	nonce uint64,
	gasPrice *big.Int,
	chainID *big.Int,
	constructorArgs []byte,
) (common.Address, common.Hash, error) {
	// Get bytecode
	bytecode, err := artifact.GetBytecodeBytes()
	if err != nil {
		return common.Address{}, common.Hash{}, fmt.Errorf("get bytecode: %w", err)
	}

	// Append constructor args if any
	data := bytecode
	if len(constructorArgs) > 0 {
		data = append(data, constructorArgs...)
	}

	// Estimate gas
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:     d.signer.Address(),
		To:       nil, // Contract creation
		Gas:      0,
		GasPrice: gasPrice,
		Value:    big.NewInt(0),
		Data:     data,
	})
	if err != nil {
		// Use a high default if estimation fails (common for contract deployment)
		gasLimit = 10_000_000
		d.logger.Warn("gas estimation failed, using default",
			slog.Uint64("gas_limit", gasLimit),
			slog.String("error", err.Error()),
		)
	}

	// Add 20% buffer to gas limit
	gasLimit = gasLimit * 120 / 100

	// Create transaction
	tx := types.NewContractCreation(
		nonce,
		big.NewInt(0), // Value
		gasLimit,
		gasPrice,
		data,
	)

	// Sign transaction
	signedTx, err := d.signer.SignTransaction(ctx, tx)
	if err != nil {
		return common.Address{}, common.Hash{}, fmt.Errorf("sign transaction: %w", err)
	}

	// Send transaction
	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return common.Address{}, common.Hash{}, fmt.Errorf("send transaction: %w", err)
	}

	// Wait for receipt
	receipt, err := bind.WaitMined(ctx, client, signedTx)
	if err != nil {
		return common.Address{}, signedTx.Hash(), fmt.Errorf("wait for receipt: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return common.Address{}, signedTx.Hash(), fmt.Errorf("contract deployment reverted")
	}

	return receipt.ContractAddress, signedTx.Hash(), nil
}

// GetRollupCreator returns the RollupCreator address for a parent chain, if deployed.
func (d *InfrastructureDeployer) GetRollupCreator(ctx context.Context, parentChainID int64) (common.Address, error) {
	infra, err := d.repo.Get(ctx, parentChainID)
	if err != nil {
		return common.Address{}, fmt.Errorf("get infrastructure: %w", err)
	}
	if infra == nil {
		return common.Address{}, fmt.Errorf("no infrastructure deployed on chain %d", parentChainID)
	}
	return common.HexToAddress(infra.RollupCreatorAddress), nil
}

// ptrToString safely dereferences a string pointer.
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// WellKnownInfrastructure contains information about official/known RollupCreator deployments.
type WellKnownInfrastructure struct {
	Address common.Address
	Version string // Semantic version (e.g., "v3.1.0", "v3.2.0")
}

// WellKnownRollupCreators contains addresses of official Arbitrum RollupCreator contracts
// along with their version information. This allows version-aware infrastructure selection.
var WellKnownRollupCreators = map[int64]WellKnownInfrastructure{
	// Ethereum Mainnet - Official Arbitrum deployment
	1: {
		Address: common.HexToAddress("0x90D68B056c411015eaE3EC0b98AD94E2C91419F1"),
		Version: "v3.1.0", // Does NOT support External DA (0x01 header)
	},
	// Sepolia - Official Arbitrum deployment
	11155111: {
		Address: common.HexToAddress("0xfb774ea8A92ae528A596c8D90CBCF1BdBc4Cee79"),
		Version: "v3.1.0", // Does NOT support External DA (0x01 header)
	},
	// Arbitrum One
	42161: {
		Address: common.HexToAddress("0x79607f00e61E6d7C0E6330bd7451f73136042a5C"),
		Version: "v3.1.0",
	},
	// Arbitrum Sepolia
	421614: {
		Address: common.HexToAddress("0xd2Ec8376B1dF436fAb18120E416d3F2BeC61275b"),
		Version: "v3.1.0",
	},
}

// TargetContractVersion is the version we require for full feature support.
// Features like External DA Provider (Celestia) require v3.2.0+.
const TargetContractVersion = "v3.2.0"

// GetWellKnownRollupCreator returns the official RollupCreator for a chain, if it exists
// and matches the target version. Returns false if version doesn't match our requirements.
func GetWellKnownRollupCreator(chainID int64, targetVersion string) (common.Address, bool) {
	infra, ok := WellKnownRollupCreators[chainID]
	if !ok {
		return common.Address{}, false
	}

	// Check if the well-known version meets our requirements
	if !isVersionCompatible(infra.Version, targetVersion) {
		return common.Address{}, false
	}

	return infra.Address, true
}

// GetWellKnownRollupCreatorAnyVersion returns the official RollupCreator regardless of version.
// Use this when version compatibility is not required.
func GetWellKnownRollupCreatorAnyVersion(chainID int64) (common.Address, string, bool) {
	infra, ok := WellKnownRollupCreators[chainID]
	if !ok {
		return common.Address{}, "", false
	}
	return infra.Address, infra.Version, true
}

// isVersionCompatible checks if the available version meets the target version requirements.
// Uses semantic versioning comparison (major.minor.patch).
func isVersionCompatible(available, target string) bool {
	// Parse versions (strip "v" prefix if present)
	availMajor, availMinor, availPatch := parseVersion(available)
	targMajor, targMinor, targPatch := parseVersion(target)

	// Compare major.minor.patch
	if availMajor > targMajor {
		return true
	}
	if availMajor < targMajor {
		return false
	}
	// Major versions equal
	if availMinor > targMinor {
		return true
	}
	if availMinor < targMinor {
		return false
	}
	// Minor versions equal
	return availPatch >= targPatch
}

// parseVersion extracts major, minor, patch from a version string like "v3.2.0" or "3.2.0".
func parseVersion(v string) (major, minor, patch int) {
	// Strip "v" prefix
	if len(v) > 0 && v[0] == 'v' {
		v = v[1:]
	}
	// Parse major.minor.patch (ignore -beta, -rc suffixes)
	var suffix string
	fmt.Sscanf(v, "%d.%d.%d%s", &major, &minor, &patch, &suffix)
	return
}
