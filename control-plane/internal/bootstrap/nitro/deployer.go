// Package nitro provides Nitro chain deployment infrastructure.
package nitro

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Default deployment parameters
const (
	DefaultConfirmPeriodBlocks = 45818  // ~1 week on Ethereum
	DefaultMaxDataSize         = 117964 // ~115KB max batch size
)

// DataAvailabilityMode represents the data availability mode for the chain.
type DataAvailabilityMode string

const (
	DAModeCelestia DataAvailabilityMode = "celestia"
	DAModeRollup   DataAvailabilityMode = "rollup"
	DAModeAnytrust DataAvailabilityMode = "anytrust"
)

// RollupConfig contains all configuration for deploying a Nitro rollup.
type RollupConfig struct {
	// Chain configuration
	ChainID        int64  `json:"chainId"`
	ChainName      string `json:"chainName"`
	ParentChainID  int64  `json:"parentChainId"`
	ParentChainRPC string `json:"parentChainRpc"`

	// Ownership and operators
	Owner        common.Address   `json:"owner"`
	BatchPosters []common.Address `json:"batchPosters"`
	Validators   []common.Address `json:"validators"`

	// Staking
	StakeToken common.Address `json:"stakeToken"`
	BaseStake  *big.Int       `json:"baseStake"`

	// Optional parameters with defaults
	ConfirmPeriodBlocks int64                `json:"confirmPeriodBlocks,omitempty"`
	MaxDataSize         int64                `json:"maxDataSize,omitempty"`
	DataAvailability    DataAvailabilityMode `json:"dataAvailability,omitempty"`
	NativeToken         common.Address       `json:"nativeToken,omitempty"`
	DeployFactoriesToL2 bool                 `json:"deployFactoriesToL2,omitempty"`
}

// RollupContracts contains addresses of all deployed core contracts.
type RollupContracts struct {
	Rollup                 common.Address `json:"rollup"`
	Inbox                  common.Address `json:"inbox"`
	Outbox                 common.Address `json:"outbox"`
	Bridge                 common.Address `json:"bridge"`
	SequencerInbox         common.Address `json:"sequencerInbox"`
	RollupEventInbox       common.Address `json:"rollupEventInbox"`
	ChallengeManager       common.Address `json:"challengeManager"`
	AdminProxy             common.Address `json:"adminProxy"`
	UpgradeExecutor        common.Address `json:"upgradeExecutor"`
	ValidatorWalletCreator common.Address `json:"validatorWalletCreator"`
	NativeToken            common.Address `json:"nativeToken"`
	DeployedAtBlockNumber  uint64         `json:"deployedAtBlockNumber"`
}

// RollupDeployResult contains the result of a deployment operation.
type RollupDeployResult struct {
	Success         bool                   `json:"success"`
	Contracts       *RollupContracts       `json:"contracts,omitempty"`
	TransactionHash common.Hash            `json:"transactionHash,omitempty"`
	BlockNumber     uint64                 `json:"blockNumber,omitempty"`
	ChainConfig     map[string]interface{} `json:"chainConfig,omitempty"`
	Error           string                 `json:"error,omitempty"`
}

// RollupDeployer handles deployment of Nitro rollups using RollupCreator.
type RollupDeployer struct {
	artifacts *NitroArtifacts
	signer    TransactionSigner
	logger    *slog.Logger
	encoder   *RollupEncoder

	// Cached ABIs
	rollupCreatorABI   abi.ABI
	sequencerInboxABI  abi.ABI
	upgradeExecutorABI abi.ABI
}

// NewRollupDeployer creates a new rollup deployer.
// The signer can be either a NitroSigner (for production via POPSigner) or
// a LocalSigner (for testing on Anvil/local networks).
func NewRollupDeployer(
	artifacts *NitroArtifacts,
	signer TransactionSigner,
	logger *slog.Logger,
) (*RollupDeployer, error) {
	// Parse ABIs
	rollupCreatorABI, err := ParseContractABI(artifacts.RollupCreator.ABI)
	if err != nil {
		return nil, fmt.Errorf("parse RollupCreator ABI: %w", err)
	}

	sequencerInboxABI, err := ParseContractABI(artifacts.SequencerInbox.ABI)
	if err != nil {
		return nil, fmt.Errorf("parse SequencerInbox ABI: %w", err)
	}

	// UpgradeExecutor ABI - we only need executeCall
	upgradeExecutorABI, err := abi.JSON(strings.NewReader(`[{
		"inputs": [
			{"name": "upgrade", "type": "address"},
			{"name": "upgradeCallData", "type": "bytes"}
		],
		"name": "executeCall",
		"outputs": [],
		"stateMutability": "payable",
		"type": "function"
	}]`))
	if err != nil {
		return nil, fmt.Errorf("parse UpgradeExecutor ABI: %w", err)
	}

	return &RollupDeployer{
		artifacts:          artifacts,
		signer:             signer,
		logger:             logger,
		encoder:            NewRollupEncoder(rollupCreatorABI, logger),
		rollupCreatorABI:   rollupCreatorABI,
		sequencerInboxABI:  sequencerInboxABI,
		upgradeExecutorABI: upgradeExecutorABI,
	}, nil
}

// Deploy deploys a new Nitro rollup using the RollupCreator contract.
func (d *RollupDeployer) Deploy(
	ctx context.Context,
	cfg *RollupConfig,
	rollupCreatorAddr common.Address,
) (*RollupDeployResult, error) {
	d.logger.Info("starting Nitro rollup deployment",
		slog.Int64("chain_id", cfg.ChainID),
		slog.String("chain_name", cfg.ChainName),
		slog.Int64("parent_chain_id", cfg.ParentChainID),
	)

	// Apply defaults
	d.applyDefaults(cfg)

	// Connect to parent chain
	client, err := ethclient.DialContext(ctx, cfg.ParentChainRPC)
	if err != nil {
		return d.errorResult(fmt.Errorf("connect to parent chain: %w", err))
	}
	defer client.Close()

	// Verify chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return d.errorResult(fmt.Errorf("get chain ID: %w", err))
	}
	if chainID.Int64() != cfg.ParentChainID {
		return d.errorResult(fmt.Errorf("chain ID mismatch: expected %d, got %d", cfg.ParentChainID, chainID.Int64()))
	}

	// Check deployer balance
	balance, err := client.BalanceAt(ctx, d.signer.Address(), nil)
	if err != nil {
		return d.errorResult(fmt.Errorf("get balance: %w", err))
	}
	d.logger.Info("deployer balance",
		slog.String("address", d.signer.Address().Hex()),
		slog.String("balance_wei", balance.String()),
	)

	if balance.Sign() == 0 {
		return d.errorResult(fmt.Errorf("deployer address has no ETH balance"))
	}

	// Prepare chain config
	chainConfig := PrepareChainConfig(cfg)
	d.logger.Info("chain config prepared", slog.Any("config", chainConfig))

	// Encode createRollup call data
	callData, err := d.encoder.EncodeCreateRollup(cfg, chainConfig)
	if err != nil {
		return d.errorResult(fmt.Errorf("encode createRollup: %w", err))
	}

	// Get nonce
	nonce, err := client.PendingNonceAt(ctx, d.signer.Address())
	if err != nil {
		return d.errorResult(fmt.Errorf("get nonce: %w", err))
	}

	// Get gas price with boost
	gasPrice, err := d.getGasPrice(ctx, client)
	if err != nil {
		return d.errorResult(fmt.Errorf("get gas price: %w", err))
	}

	// Estimate gas
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:     d.signer.Address(),
		To:       &rollupCreatorAddr,
		Gas:      0,
		GasPrice: gasPrice,
		Value:    big.NewInt(0),
		Data:     callData,
	})
	if err != nil {
		// Use a high default for rollup creation
		gasLimit = 15_000_000
		d.logger.Warn("gas estimation failed, using default",
			slog.Uint64("gas_limit", gasLimit),
			slog.String("error", err.Error()),
		)
	}
	// Add 20% buffer
	gasLimit = gasLimit * 120 / 100

	// Cap at 15M to stay under block gas limit (Sepolia ~16.7M)
	const maxGasLimit = 15_000_000
	if gasLimit > maxGasLimit {
		d.logger.Warn("gas limit capped to max",
			slog.Uint64("original", gasLimit),
			slog.Uint64("capped", maxGasLimit),
		)
		gasLimit = maxGasLimit
	}

	d.logger.Info("sending createRollup transaction",
		slog.String("rollup_creator", rollupCreatorAddr.Hex()),
		slog.Uint64("gas_limit", gasLimit),
		slog.String("gas_price", gasPrice.String()),
	)

	// Create transaction
	tx := types.NewTransaction(
		nonce,
		rollupCreatorAddr,
		big.NewInt(0), // Value
		gasLimit,
		gasPrice,
		callData,
	)

	// Sign and send
	signedTx, err := d.signer.SignTransaction(ctx, tx)
	if err != nil {
		return d.errorResult(fmt.Errorf("sign transaction: %w", err))
	}

	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return d.errorResult(fmt.Errorf("send transaction: %w", err))
	}

	d.logger.Info("transaction submitted, waiting for confirmation",
		slog.String("tx_hash", signedTx.Hash().Hex()),
	)

	// Wait for receipt
	receipt, err := bind.WaitMined(ctx, client, signedTx)
	if err != nil {
		return &RollupDeployResult{
			Success:         false,
			TransactionHash: signedTx.Hash(),
			Error:           fmt.Sprintf("wait for receipt: %v", err),
		}, nil
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return &RollupDeployResult{
			Success:         false,
			TransactionHash: signedTx.Hash(),
			BlockNumber:     receipt.BlockNumber.Uint64(),
			Error:           "transaction reverted",
		}, nil
	}

	d.logger.Info("transaction confirmed",
		slog.Uint64("block_number", receipt.BlockNumber.Uint64()),
	)

	// Parse contract addresses from logs
	coreContracts, err := d.parseDeploymentLogs(receipt)
	if err != nil {
		return d.errorResult(fmt.Errorf("parse deployment logs: %w", err))
	}
	coreContracts.DeployedAtBlockNumber = receipt.BlockNumber.Uint64()

	d.logger.Info("rollup deployed successfully",
		slog.String("rollup", coreContracts.Rollup.Hex()),
		slog.String("sequencer_inbox", coreContracts.SequencerInbox.Hex()),
	)

	// Whitelist batch posters
	if len(cfg.BatchPosters) > 0 {
		if err := d.whitelistBatchPosters(ctx, client, coreContracts, cfg.BatchPosters); err != nil {
			d.logger.Warn("failed to whitelist batch posters",
				slog.String("error", err.Error()),
			)
			// Don't fail the deployment, just log the warning
		}
	}

	// Ensure validator has WETH for BOLD staking
	// This wraps ETH to WETH automatically so the user doesn't have to
	if cfg.StakeToken != (common.Address{}) {
		requiredWETH := big.NewInt(100000000000000000) // 0.1 WETH (enough for all stake levels)
		if err := d.ensureWETHBalance(ctx, client, cfg.StakeToken, requiredWETH); err != nil {
			d.logger.Warn("failed to ensure WETH balance for staking",
				slog.String("error", err.Error()),
			)
			// Don't fail - staker can wrap manually if needed
		}
	}

	return &RollupDeployResult{
		Success:         true,
		Contracts:       coreContracts,
		TransactionHash: signedTx.Hash(),
		BlockNumber:     receipt.BlockNumber.Uint64(),
		ChainConfig:     chainConfig,
	}, nil
}

// applyDefaults applies default values to config.
func (d *RollupDeployer) applyDefaults(cfg *RollupConfig) {
	if cfg.ConfirmPeriodBlocks == 0 {
		cfg.ConfirmPeriodBlocks = DefaultConfirmPeriodBlocks
	}
	if cfg.MaxDataSize == 0 {
		cfg.MaxDataSize = DefaultMaxDataSize
	}
	if cfg.DataAvailability == "" {
		cfg.DataAvailability = DAModeCelestia
	}
}

// getGasPrice returns a boosted gas price for faster inclusion.
func (d *RollupDeployer) getGasPrice(ctx context.Context, client *ethclient.Client) (*big.Int, error) {
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, err
	}

	// Boost by 50%
	boosted := new(big.Int).Mul(gasPrice, big.NewInt(150))
	boosted = boosted.Div(boosted, big.NewInt(100))

	// Ensure at least 2 Gwei
	minGwei := big.NewInt(2_000_000_000)
	if boosted.Cmp(minGwei) < 0 {
		boosted = minGwei
	}

	return boosted, nil
}

// parseDeploymentLogs parses the RollupCreated event from transaction logs.
func (d *RollupDeployer) parseDeploymentLogs(receipt *types.Receipt) (*RollupContracts, error) {
	// RollupCreated event signature for BOLD (v3.2.0):
	// event RollupCreated(
	//   address indexed rollupAddress,
	//   address indexed nativeToken,
	//   address inboxAddress,
	//   address outbox,
	//   address rollupEventInbox,
	//   address challengeManager,
	//   address adminProxy,
	//   address sequencerInbox,
	//   address bridge,
	//   address upgradeExecutor,
	//   address validatorWalletCreator
	// )

	// RollupCreated event topic hash
	// keccak256("RollupCreated(address,address,address,address,address,address,address,address,address,address,address)")
	rollupCreatedTopic := common.HexToHash("0xd9bfd3bb3012f0caa103d1ba172692464d2de5c7b75877ce255c72147086a79d")

	d.logger.Info("parsing deployment logs",
		slog.Int("num_logs", len(receipt.Logs)),
		slog.String("tx_hash", receipt.TxHash.Hex()),
	)

	for i, log := range receipt.Logs {
		d.logger.Debug("checking log",
			slog.Int("index", i),
			slog.Int("num_topics", len(log.Topics)),
			slog.Int("data_len", len(log.Data)),
			slog.String("address", log.Address.Hex()),
		)

		if len(log.Topics) == 0 {
			continue
		}

		// Match by event topic signature
		if log.Topics[0] != rollupCreatedTopic {
			continue
		}

		d.logger.Info("found RollupCreated event",
			slog.Int("index", i),
			slog.Int("data_len", len(log.Data)),
		)

		// Must have at least 3 topics (event sig + 2 indexed params)
		if len(log.Topics) < 3 {
			d.logger.Warn("RollupCreated event has insufficient topics",
				slog.Int("topics", len(log.Topics)),
			)
			continue
		}

		// Must have 9 addresses in data (9 * 32 = 288 bytes)
		if len(log.Data) < 32*9 {
			d.logger.Warn("RollupCreated event has insufficient data",
				slog.Int("data_len", len(log.Data)),
				slog.Int("expected", 32*9),
			)
			continue
		}

		contracts := &RollupContracts{}

		// Indexed parameters from topics
		contracts.Rollup = common.BytesToAddress(log.Topics[1].Bytes())
		contracts.NativeToken = common.BytesToAddress(log.Topics[2].Bytes())

		// Non-indexed parameters from data (each is a 32-byte ABI-encoded address)
		contracts.Inbox = common.BytesToAddress(log.Data[0:32])
		contracts.Outbox = common.BytesToAddress(log.Data[32:64])
		contracts.RollupEventInbox = common.BytesToAddress(log.Data[64:96])
		contracts.ChallengeManager = common.BytesToAddress(log.Data[96:128])
		contracts.AdminProxy = common.BytesToAddress(log.Data[128:160])
		contracts.SequencerInbox = common.BytesToAddress(log.Data[160:192])
		contracts.Bridge = common.BytesToAddress(log.Data[192:224])
		contracts.UpgradeExecutor = common.BytesToAddress(log.Data[224:256])
		contracts.ValidatorWalletCreator = common.BytesToAddress(log.Data[256:288])

		d.logger.Info("parsed RollupCreated event",
			slog.String("rollup", contracts.Rollup.Hex()),
			slog.String("inbox", contracts.Inbox.Hex()),
			slog.String("bridge", contracts.Bridge.Hex()),
			slog.String("sequencer_inbox", contracts.SequencerInbox.Hex()),
			slog.String("outbox", contracts.Outbox.Hex()),
			slog.String("challenge_manager", contracts.ChallengeManager.Hex()),
			slog.String("upgrade_executor", contracts.UpgradeExecutor.Hex()),
		)

		return contracts, nil
	}

	// If we didn't find the event, log all topics for debugging
	d.logger.Error("RollupCreated event not found",
		slog.String("expected_topic", rollupCreatedTopic.Hex()),
	)
	for i, log := range receipt.Logs {
		if len(log.Topics) > 0 {
			d.logger.Error("log topic",
				slog.Int("index", i),
				slog.String("topic0", log.Topics[0].Hex()),
				slog.String("address", log.Address.Hex()),
			)
		}
	}

	return nil, fmt.Errorf("RollupCreated event not found in logs (checked %d logs)", len(receipt.Logs))
}

// whitelistBatchPosters whitelists batch posters on the SequencerInbox via UpgradeExecutor.
func (d *RollupDeployer) whitelistBatchPosters(
	ctx context.Context,
	client *ethclient.Client,
	contracts *RollupContracts,
	batchPosters []common.Address,
) error {
	d.logger.Info("whitelisting batch posters via UpgradeExecutor",
		slog.Int("count", len(batchPosters)),
		slog.String("upgrade_executor", contracts.UpgradeExecutor.Hex()),
		slog.String("sequencer_inbox", contracts.SequencerInbox.Hex()),
	)

	for _, batchPoster := range batchPosters {
		// Check if already whitelisted
		isWhitelisted, err := d.isBatchPoster(ctx, client, contracts.SequencerInbox, batchPoster)
		if err != nil {
			d.logger.Warn("failed to check batch poster status",
				slog.String("batch_poster", batchPoster.Hex()),
				slog.String("error", err.Error()),
			)
			continue
		}

		if isWhitelisted {
			d.logger.Info("batch poster already whitelisted",
				slog.String("batch_poster", batchPoster.Hex()),
			)
			continue
		}

		// Encode setIsBatchPoster(batchPoster, true)
		innerCallData, err := d.sequencerInboxABI.Pack("setIsBatchPoster", batchPoster, true)
		if err != nil {
			return fmt.Errorf("encode setIsBatchPoster: %w", err)
		}

		// Encode executeCall(sequencerInbox, innerCallData)
		outerCallData, err := d.upgradeExecutorABI.Pack("executeCall", contracts.SequencerInbox, innerCallData)
		if err != nil {
			return fmt.Errorf("encode executeCall: %w", err)
		}

		// Get nonce
		nonce, err := client.PendingNonceAt(ctx, d.signer.Address())
		if err != nil {
			return fmt.Errorf("get nonce: %w", err)
		}

		// Get gas price
		gasPrice, err := d.getGasPrice(ctx, client)
		if err != nil {
			return fmt.Errorf("get gas price: %w", err)
		}

		// Estimate gas
		gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
			From:     d.signer.Address(),
			To:       &contracts.UpgradeExecutor,
			Gas:      0,
			GasPrice: gasPrice,
			Value:    big.NewInt(0),
			Data:     outerCallData,
		})
		if err != nil {
			gasLimit = 500_000 // Default
			d.logger.Warn("gas estimation failed for setIsBatchPoster",
				slog.String("error", err.Error()),
			)
		}
		gasLimit = gasLimit * 120 / 100 // 20% buffer

		// Create transaction
		tx := types.NewTransaction(
			nonce,
			contracts.UpgradeExecutor,
			big.NewInt(0),
			gasLimit,
			gasPrice,
			outerCallData,
		)

		// Sign and send
		signedTx, err := d.signer.SignTransaction(ctx, tx)
		if err != nil {
			return fmt.Errorf("sign transaction: %w", err)
		}

		if err := client.SendTransaction(ctx, signedTx); err != nil {
			return fmt.Errorf("send transaction: %w", err)
		}

		d.logger.Info("setIsBatchPoster transaction submitted",
			slog.String("batch_poster", batchPoster.Hex()),
			slog.String("tx_hash", signedTx.Hash().Hex()),
		)

		// Wait for confirmation
		receipt, err := bind.WaitMined(ctx, client, signedTx)
		if err != nil {
			return fmt.Errorf("wait for receipt: %w", err)
		}

		if receipt.Status != types.ReceiptStatusSuccessful {
			return fmt.Errorf("setIsBatchPoster reverted for %s", batchPoster.Hex())
		}

		// Verify
		isNowWhitelisted, err := d.isBatchPoster(ctx, client, contracts.SequencerInbox, batchPoster)
		if err != nil {
			d.logger.Warn("failed to verify batch poster whitelisting",
				slog.String("error", err.Error()),
			)
		} else if !isNowWhitelisted {
			return fmt.Errorf("batch poster %s not whitelisted after transaction", batchPoster.Hex())
		}

		d.logger.Info("batch poster whitelisted successfully",
			slog.String("batch_poster", batchPoster.Hex()),
		)
	}

	return nil
}

// isBatchPoster checks if an address is whitelisted as a batch poster.
func (d *RollupDeployer) isBatchPoster(
	ctx context.Context,
	client *ethclient.Client,
	sequencerInbox common.Address,
	addr common.Address,
) (bool, error) {
	callData, err := d.sequencerInboxABI.Pack("isBatchPoster", addr)
	if err != nil {
		return false, err
	}

	result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &sequencerInbox,
		Data: callData,
	}, nil)
	if err != nil {
		return false, err
	}

	var isWhitelisted bool
	if err := d.sequencerInboxABI.UnpackIntoInterface(&isWhitelisted, "isBatchPoster", result); err != nil {
		return false, err
	}

	return isWhitelisted, nil
}

// errorResult creates an error result.
func (d *RollupDeployer) errorResult(err error) (*RollupDeployResult, error) {
	return &RollupDeployResult{
		Success: false,
		Error:   err.Error(),
	}, nil
}
