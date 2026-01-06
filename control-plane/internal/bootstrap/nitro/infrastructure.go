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

// TransactionSigner is the interface for signing transactions.
// This abstraction allows using either POPSigner (production) or local keys (testing).
type TransactionSigner interface {
	// Address returns the signer's Ethereum address.
	Address() common.Address
	// ChainID returns the chain ID for transaction signing.
	ChainID() *big.Int
	// SignTransaction signs a transaction and returns the signed version.
	SignTransaction(ctx context.Context, tx *types.Transaction) (*types.Transaction, error)
}

// InfrastructureDeployer handles deployment of shared Nitro infrastructure.
// The RollupCreator and associated contracts are deployed once per parent chain
// and reused for all customer rollups on that chain.
type InfrastructureDeployer struct {
	artifacts *NitroArtifacts
	signer    TransactionSigner
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

	// All deployed infrastructure addresses (for transparency)
	DeployedContracts map[string]common.Address
}

// NewInfrastructureDeployer creates a new infrastructure deployer.
func NewInfrastructureDeployer(
	artifacts *NitroArtifacts,
	signer TransactionSigner,
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
// This is a complex multi-step process following the dependency graph:
//
// Phase 1: Simple contracts (no constructor args)
//   - OneStepProver0, OneStepProverMemory, OneStepProverMath, OneStepProverHostIo
//   - Bridge, SequencerInbox, Inbox, Outbox, RollupEventInbox
//   - ERC20Bridge, ERC20Inbox (for custom gas tokens)
//   - EdgeChallengeManager, RollupAdminLogic, RollupUserLogic, UpgradeExecutor
//   - ValidatorWalletCreator, DeployHelper
//
// Phase 2: Contracts with dependencies
//   - OneStepProofEntry (needs 4 provers)
//   - BridgeCreator (needs bridge template addresses)
//
// Phase 3: Main entry point
//   - RollupCreator (needs all above)
func (d *InfrastructureDeployer) deployInfrastructure(
	ctx context.Context,
	client *ethclient.Client,
) (*InfrastructureResult, error) {
	deployed := make(map[string]common.Address)

	// Get gas price (boosted 50%)
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("get gas price: %w", err)
	}
	gasPrice = new(big.Int).Mul(gasPrice, big.NewInt(150))
	gasPrice = new(big.Int).Div(gasPrice, big.NewInt(100))

	chainID := d.signer.ChainID()

	// Helper to deploy a contract and track it
	deploy := func(name string, artifact *ContractArtifact, constructorArgs []byte) error {
		nonce, err := client.PendingNonceAt(ctx, d.signer.Address())
		if err != nil {
			return fmt.Errorf("get nonce for %s: %w", name, err)
		}

		addr, txHash, err := d.deployContract(ctx, client, artifact, nonce, gasPrice, chainID, constructorArgs)
		if err != nil {
			return fmt.Errorf("deploy %s: %w", name, err)
		}

		deployed[name] = addr
		d.logger.Info("deployed contract",
			slog.String("name", name),
			slog.String("address", addr.Hex()),
			slog.String("tx_hash", txHash.Hex()),
		)
		return nil
	}

	// Constants for constructor args
	const maxDataSize = 117964 // MAX_DATA_SIZE from Arbitrum contracts
	zeroAddress := common.Address{}

	// Check if we're on an Arbitrum chain (then Reader4844 should be zero)
	// For non-Arbitrum chains, we need to deploy a Reader4844 mock
	var reader4844Addr common.Address
	isArbitrum := d.isArbitrumChain(ctx, client)
	if isArbitrum {
		reader4844Addr = zeroAddress
		d.logger.Info("Detected Arbitrum chain, using zero address for Reader4844")
	} else {
		// Deploy Reader4844 for L1/local chains (requires Cancun hardfork for EIP-4844)
		d.logger.Info("Deploying Reader4844 for L1/local chain (requires Cancun EVM)...")
		reader4844Addr, err = d.deployReader4844(ctx, client, gasPrice, chainID)
		if err != nil {
			return nil, fmt.Errorf("deploy Reader4844: %w", err)
		}
		deployed["Reader4844"] = reader4844Addr
		d.logger.Info("deployed Reader4844", slog.String("address", reader4844Addr.Hex()))
	}

	// ============================================
	// Phase 1: Simple contracts (no constructor args)
	// ============================================
	d.logger.Info("Phase 1: Deploying simple contracts...")

	simpleContracts := []struct {
		name     string
		artifact *ContractArtifact
	}{
		// OneStepProvers (except HostIo which needs _customDAValidator)
		{"OneStepProver0", d.artifacts.OneStepProver0},
		{"OneStepProverMemory", d.artifacts.OneStepProverMemory},
		{"OneStepProverMath", d.artifacts.OneStepProverMath},
		// Bridge templates (only Bridge and Outbox have no constructor args)
		{"Bridge", d.artifacts.Bridge},
		{"Outbox", d.artifacts.Outbox},
		{"RollupEventInbox", d.artifacts.RollupEventInbox},
		// ERC20 variants (ERC20Bridge has no constructor)
		{"ERC20Bridge", d.artifacts.ERC20Bridge},
		// Rollup logic
		{"EdgeChallengeManager", d.artifacts.EdgeChallengeManager},
		{"RollupAdminLogic", d.artifacts.RollupAdminLogic},
		{"RollupUserLogic", d.artifacts.RollupUserLogic},
		{"UpgradeExecutor", d.artifacts.UpgradeExecutor},
		// Validator/Deploy helpers
		{"ValidatorWalletCreator", d.artifacts.ValidatorWalletCreator},
		{"DeployHelper", d.artifacts.DeployHelper},
	}

	for _, c := range simpleContracts {
		if err := deploy(c.name, c.artifact, nil); err != nil {
			return nil, err
		}
	}

	// ============================================
	// Phase 1b: Contracts with simple constructor args
	// ============================================
	d.logger.Info("Phase 1b: Deploying contracts with constructor args...")

	// OneStepProverHostIo(_customDAValidator)
	ospHostIoArgs, err := d.artifacts.OneStepProverHostIo.EncodeConstructorArgs(zeroAddress)
	if err != nil {
		return nil, fmt.Errorf("encode OneStepProverHostIo args: %w", err)
	}
	if err := deploy("OneStepProverHostIo", d.artifacts.OneStepProverHostIo, ospHostIoArgs); err != nil {
		return nil, err
	}

	// SequencerInbox(_maxDataSize, reader4844_, _isUsingFeeToken, _isDelayBufferable)
	// Deploy non-delay-bufferable variant for ETH
	seqInboxArgs, err := d.artifacts.SequencerInbox.EncodeConstructorArgs(
		big.NewInt(maxDataSize),
		reader4844Addr, // reader4844_ (zero for Arbitrum, deployed mock for L1)
		false,          // _isUsingFeeToken
		false,          // _isDelayBufferable
	)
	if err != nil {
		return nil, fmt.Errorf("encode SequencerInbox args: %w", err)
	}
	if err := deploy("SequencerInbox", d.artifacts.SequencerInbox, seqInboxArgs); err != nil {
		return nil, err
	}

	// Deploy delay-bufferable variant (same params but isDelayBufferable=true)
	seqInboxDelayArgs, err := d.artifacts.SequencerInbox.EncodeConstructorArgs(
		big.NewInt(maxDataSize),
		reader4844Addr, // reader4844_
		false,          // _isUsingFeeToken
		true,           // _isDelayBufferable = true
	)
	if err != nil {
		return nil, fmt.Errorf("encode SequencerInboxDelay args: %w", err)
	}
	if err := deploy("SequencerInboxDelay", d.artifacts.SequencerInbox, seqInboxDelayArgs); err != nil {
		return nil, err
	}

	// Inbox(_maxDataSize)
	inboxArgs, err := d.artifacts.Inbox.EncodeConstructorArgs(big.NewInt(maxDataSize))
	if err != nil {
		return nil, fmt.Errorf("encode Inbox args: %w", err)
	}
	if err := deploy("Inbox", d.artifacts.Inbox, inboxArgs); err != nil {
		return nil, err
	}

	// ERC20Inbox(_maxDataSize)
	erc20InboxArgs, err := d.artifacts.ERC20Inbox.EncodeConstructorArgs(big.NewInt(maxDataSize))
	if err != nil {
		return nil, fmt.Errorf("encode ERC20Inbox args: %w", err)
	}
	if err := deploy("ERC20Inbox", d.artifacts.ERC20Inbox, erc20InboxArgs); err != nil {
		return nil, err
	}

	// ============================================
	// Phase 2: Contracts with dependencies
	// ============================================
	d.logger.Info("Phase 2: Deploying contracts with dependencies...")

	// OneStepProofEntry(prover0, proverMem, proverMath, proverHostIo)
	ospArgs, err := d.artifacts.OneStepProofEntry.EncodeConstructorArgs(
		deployed["OneStepProver0"],
		deployed["OneStepProverMemory"],
		deployed["OneStepProverMath"],
		deployed["OneStepProverHostIo"],
	)
	if err != nil {
		return nil, fmt.Errorf("encode OneStepProofEntry args: %w", err)
	}
	if err := deploy("OneStepProofEntry", d.artifacts.OneStepProofEntry, ospArgs); err != nil {
		return nil, err
	}

	// BridgeCreator(ethTemplates, erc20Templates)
	// BridgeTemplates struct: (bridge, sequencerInbox, delayBufferableSequencerInbox, inbox, rollupEventInbox, outbox)
	ethTemplates := struct {
		Bridge                        common.Address
		SequencerInbox                common.Address
		DelayBufferableSequencerInbox common.Address
		Inbox                         common.Address
		RollupEventInbox              common.Address
		Outbox                        common.Address
	}{
		Bridge:                        deployed["Bridge"],
		SequencerInbox:                deployed["SequencerInbox"],
		DelayBufferableSequencerInbox: deployed["SequencerInboxDelay"], // Delay bufferable variant
		Inbox:                         deployed["Inbox"],
		RollupEventInbox:              deployed["RollupEventInbox"],
		Outbox:                        deployed["Outbox"],
	}

	erc20Templates := struct {
		Bridge                        common.Address
		SequencerInbox                common.Address
		DelayBufferableSequencerInbox common.Address
		Inbox                         common.Address
		RollupEventInbox              common.Address
		Outbox                        common.Address
	}{
		Bridge:                        deployed["ERC20Bridge"],
		SequencerInbox:                deployed["SequencerInbox"],      // Same SequencerInbox (ETH based)
		DelayBufferableSequencerInbox: deployed["SequencerInboxDelay"], // Delay bufferable
		Inbox:                         deployed["ERC20Inbox"],
		RollupEventInbox:              deployed["RollupEventInbox"],
		Outbox:                        deployed["Outbox"],
	}

	bridgeCreatorArgs, err := d.artifacts.BridgeCreator.EncodeConstructorArgs(
		ethTemplates,
		erc20Templates,
	)
	if err != nil {
		return nil, fmt.Errorf("encode BridgeCreator args: %w", err)
	}
	if err := deploy("BridgeCreator", d.artifacts.BridgeCreator, bridgeCreatorArgs); err != nil {
		return nil, err
	}

	// ============================================
	// Phase 3: RollupCreator (main entry point)
	// ============================================
	d.logger.Info("Phase 3: Deploying RollupCreator...")

	// RollupCreator(
	//   initialOwner,
	//   bridgeCreator,
	//   osp (OneStepProofEntry),
	//   challengeManagerLogic,
	//   rollupAdminLogic,
	//   rollupUserLogic,
	//   upgradeExecutorLogic,
	//   validatorWalletCreator,
	//   l2FactoriesDeployer (DeployHelper)
	// )
	rollupCreatorArgs, err := d.artifacts.RollupCreator.EncodeConstructorArgs(
		d.signer.Address(),                  // initialOwner (deployer)
		deployed["BridgeCreator"],           // bridgeCreator
		deployed["OneStepProofEntry"],       // osp
		deployed["EdgeChallengeManager"],    // challengeManagerLogic
		deployed["RollupAdminLogic"],        // rollupAdminLogic
		deployed["RollupUserLogic"],         // rollupUserLogic
		deployed["UpgradeExecutor"],         // upgradeExecutorLogic
		deployed["ValidatorWalletCreator"],  // validatorWalletCreator
		deployed["DeployHelper"],            // l2FactoriesDeployer
	)
	if err != nil {
		return nil, fmt.Errorf("encode RollupCreator args: %w", err)
	}

	nonce, err := client.PendingNonceAt(ctx, d.signer.Address())
	if err != nil {
		return nil, fmt.Errorf("get nonce for RollupCreator: %w", err)
	}
	rollupCreatorAddr, txHash, err := d.deployContract(
		ctx, client,
		d.artifacts.RollupCreator,
		nonce, gasPrice, chainID,
		rollupCreatorArgs,
	)
	if err != nil {
		return nil, fmt.Errorf("deploy RollupCreator: %w", err)
	}

	deployed["RollupCreator"] = rollupCreatorAddr

	d.logger.Info("Infrastructure deployment complete!",
		slog.String("rollup_creator", rollupCreatorAddr.Hex()),
		slog.Int("total_contracts", len(deployed)),
	)

	return &InfrastructureResult{
		RollupCreatorAddress: rollupCreatorAddr,
		BridgeCreatorAddress: deployed["BridgeCreator"],
		Version:              d.artifacts.Version,
		DeploymentTxHash:     txHash,
		AlreadyDeployed:      false,
		DeployedContracts:    deployed,
	}, nil
}

// isArbitrumChain checks if the connected chain is an Arbitrum chain.
// Arbitrum chains have the ArbSys precompile at 0x64.
func (d *InfrastructureDeployer) isArbitrumChain(ctx context.Context, client *ethclient.Client) bool {
	// ArbSys precompile address on Arbitrum chains
	arbSysAddr := common.HexToAddress("0x0000000000000000000000000000000000000064")
	code, err := client.CodeAt(ctx, arbSysAddr, nil)
	if err != nil {
		return false
	}
	return len(code) > 0
}

// deployReader4844 deploys the Reader4844 contract from artifacts.
// Reader4844 is a Yul contract that wraps EIP-4844 opcodes (BLOBBASEFEE, BLOBHASH).
// Requires the chain to support Cancun hardfork (EIP-4844).
func (d *InfrastructureDeployer) deployReader4844(
	ctx context.Context,
	client *ethclient.Client,
	gasPrice *big.Int,
	chainID *big.Int,
) (common.Address, error) {
	// Use Reader4844 bytecode from S3 artifacts
	reader4844Bytecode, err := d.artifacts.Reader4844.GetBytecodeBytes()
	if err != nil {
		return common.Address{}, fmt.Errorf("get Reader4844 bytecode: %w", err)
	}

	nonce, err := client.PendingNonceAt(ctx, d.signer.Address())
	if err != nil {
		return common.Address{}, fmt.Errorf("get nonce: %w", err)
	}

	// Estimate gas
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:     d.signer.Address(),
		To:       nil,
		Gas:      0,
		GasPrice: gasPrice,
		Value:    big.NewInt(0),
		Data:     reader4844Bytecode,
	})
	if err != nil {
		gasLimit = 200000 // Default for small contract
	}
	gasLimit = gasLimit * 120 / 100

	tx := types.NewContractCreation(nonce, big.NewInt(0), gasLimit, gasPrice, reader4844Bytecode)
	signedTx, err := d.signer.SignTransaction(ctx, tx)
	if err != nil {
		return common.Address{}, fmt.Errorf("sign: %w", err)
	}

	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return common.Address{}, fmt.Errorf("send: %w", err)
	}

	receipt, err := bind.WaitMined(ctx, client, signedTx)
	if err != nil {
		return common.Address{}, fmt.Errorf("wait: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return common.Address{}, fmt.Errorf("deployment reverted")
	}

	return receipt.ContractAddress, nil
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
