package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/nitro"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Nitro-specific constants
const (
	nitroL2ChainID   = 42069
	nitroL2ChainName = "nitro-local"

	// Anvil deterministic private key for deployer (without 0x prefix)
	anvilDeployerKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80" // anvil-0
)

// nitroDeployResult contains deployment results needed for bundle generation.
type nitroDeployResult struct {
	contracts       *nitro.RollupContracts
	chainConfig     map[string]interface{}
	deploymentBlock uint64
	stakeToken      common.Address
}

// runNitro executes the 8-step Nitro bundle creation pipeline.
func (b *bundleBuilder) runNitro() error {
	b.printBanner("Nitro")

	if err := b.prepareBundleDirectory(); err != nil {
		return fmt.Errorf("prepare bundle directory: %w", err)
	}

	stateFile, err := b.startAnvil()
	if err != nil {
		return fmt.Errorf("start anvil: %w", err)
	}

	if err := b.startPOPSignerLite(); err != nil {
		return fmt.Errorf("start popsigner-lite: %w", err)
	}

	result, err := b.deployNitro()
	if err != nil {
		return fmt.Errorf("deploy nitro: %w", err)
	}

	celestiaKeyID, err := b.getCelestiaKeyID()
	if err != nil {
		return fmt.Errorf("get celestia key: %w", err)
	}

	if err := b.shutdownAnvilAndDumpState(stateFile); err != nil {
		return fmt.Errorf("shutdown anvil: %w", err)
	}

	if err := b.moveAnvilStateForNitro(stateFile); err != nil {
		return fmt.Errorf("move anvil state: %w", err)
	}

	if err := b.writeNitroConfigs(result, celestiaKeyID); err != nil {
		return fmt.Errorf("write nitro configs: %w", err)
	}

	archivePath, err := b.createNitroArchive()
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}

	b.printNitroSuccess(archivePath)
	return nil
}

// deployNitro handles the full Nitro deployment process.
func (b *bundleBuilder) deployNitro() (*nitroDeployResult, error) {
	b.logger.Info("3️⃣  Deploying Nitro rollup...")

	artifacts, err := b.downloadNitroArtifacts()
	if err != nil {
		return nil, fmt.Errorf("download artifacts: %w", err)
	}

	signer, err := b.createLocalSigner()
	if err != nil {
		return nil, fmt.Errorf("create local signer: %w", err)
	}

	infraResult, err := b.deployNitroInfrastructure(artifacts, signer)
	if err != nil {
		return nil, fmt.Errorf("deploy infrastructure: %w", err)
	}

	stakeToken, err := b.deployWETH(signer)
	if err != nil {
		return nil, fmt.Errorf("deploy WETH: %w", err)
	}

	rollupResult, err := b.deployNitroRollup(artifacts, signer, infraResult.RollupCreatorAddress, stakeToken)
	if err != nil {
		return nil, fmt.Errorf("deploy rollup: %w", err)
	}

	return &nitroDeployResult{
		contracts:       rollupResult.Contracts,
		chainConfig:     rollupResult.ChainConfig,
		deploymentBlock: rollupResult.BlockNumber,
		stakeToken:      stakeToken,
	}, nil
}

// downloadNitroArtifacts downloads contract artifacts from S3.
func (b *bundleBuilder) downloadNitroArtifacts() (*nitro.NitroArtifacts, error) {
	b.logger.Info("Downloading Nitro contract artifacts...")

	cacheDir := filepath.Join(os.TempDir(), "pop-deployer-nitro-cache")
	downloader := nitro.NewContractArtifactDownloader(cacheDir)

	artifacts, err := downloader.DownloadDefault(b.ctx)
	if err != nil {
		return nil, fmt.Errorf("download default artifacts: %w", err)
	}

	b.logger.Info("Artifacts downloaded",
		slog.String("version", artifacts.Version),
		slog.String("source", artifacts.SourceURL),
	)

	return artifacts, nil
}

// createLocalSigner creates a signer using Anvil's deterministic keys.
func (b *bundleBuilder) createLocalSigner() (*nitro.LocalSigner, error) {
	signer, err := nitro.NewLocalSigner(anvilDeployerKey, l1ChainID)
	if err != nil {
		return nil, fmt.Errorf("create local signer: %w", err)
	}

	b.logger.Info("Created local signer",
		slog.String("address", signer.Address().Hex()),
	)

	return signer, nil
}

// deployNitroInfrastructure deploys the RollupCreator and template contracts.
func (b *bundleBuilder) deployNitroInfrastructure(
	artifacts *nitro.NitroArtifacts,
	signer *nitro.LocalSigner,
) (*nitro.InfrastructureResult, error) {
	b.logger.Info("Deploying Nitro infrastructure (RollupCreator + templates)...")

	infraDeployer := nitro.NewInfrastructureDeployer(artifacts, signer, nil, b.logger)

	cfg := &nitro.InfrastructureConfig{
		ParentChainID: l1ChainID,
		ParentRPC:     l1RPC,
	}

	result, err := infraDeployer.EnsureInfrastructure(b.ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("ensure infrastructure: %w", err)
	}

	b.logger.Info("Infrastructure deployed",
		slog.String("rollup_creator", result.RollupCreatorAddress.Hex()),
		slog.Int("contracts_deployed", len(result.DeployedContracts)),
	)

	return result, nil
}

// deployWETH deploys a WETH contract for BOLD staking.
func (b *bundleBuilder) deployWETH(signer *nitro.LocalSigner) (common.Address, error) {
	b.logger.Info("Deploying WETH for BOLD staking...")

	client, err := ethclient.DialContext(b.ctx, l1RPC)
	if err != nil {
		return common.Address{}, fmt.Errorf("connect to L1: %w", err)
	}
	defer client.Close()

	// WETH9 bytecode (standard)
	wethBytecode := common.FromHex(weth9Bytecode)

	nonce, err := client.PendingNonceAt(b.ctx, signer.Address())
	if err != nil {
		return common.Address{}, fmt.Errorf("get nonce: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(b.ctx)
	if err != nil {
		return common.Address{}, fmt.Errorf("get gas price: %w", err)
	}

	// Create contract creation transaction
	tx := createContractCreation(nonce, wethBytecode, 1_000_000, gasPrice)

	signedTx, err := signer.SignTransaction(b.ctx, tx)
	if err != nil {
		return common.Address{}, fmt.Errorf("sign transaction: %w", err)
	}

	if err := client.SendTransaction(b.ctx, signedTx); err != nil {
		return common.Address{}, fmt.Errorf("send transaction: %w", err)
	}

	receipt, err := waitForReceipt(b.ctx, client, signedTx.Hash())
	if err != nil {
		return common.Address{}, fmt.Errorf("wait for receipt: %w", err)
	}

	if receipt.Status == 0 {
		return common.Address{}, fmt.Errorf("WETH deployment reverted")
	}

	b.logger.Info("WETH deployed",
		slog.String("address", receipt.ContractAddress.Hex()),
	)

	return receipt.ContractAddress, nil
}

// deployNitroRollup creates the Nitro rollup using RollupCreator.
func (b *bundleBuilder) deployNitroRollup(
	artifacts *nitro.NitroArtifacts,
	signer *nitro.LocalSigner,
	rollupCreatorAddr common.Address,
	stakeToken common.Address,
) (*nitro.RollupDeployResult, error) {
	b.logger.Info("Deploying Nitro rollup...",
		slog.String("rollup_creator", rollupCreatorAddr.Hex()),
	)

	deployer, err := nitro.NewRollupDeployer(artifacts, signer, b.logger)
	if err != nil {
		return nil, fmt.Errorf("create rollup deployer: %w", err)
	}

	batchPosterAddr := common.HexToAddress(batcherAddress)
	validatorAddr := common.HexToAddress(proposerAddress)

	cfg := &nitro.RollupConfig{
		ChainID:          nitroL2ChainID,
		ChainName:        nitroL2ChainName,
		ParentChainID:    l1ChainID,
		ParentChainRPC:   l1RPC,
		Owner:            signer.Address(),
		BatchPosters:     []common.Address{batchPosterAddr},
		Validators:       []common.Address{validatorAddr},
		StakeToken:       stakeToken,
		BaseStake:        big.NewInt(100000000000000000), // 0.1 ETH
		DataAvailability: nitro.DAModeCelestia,
	}

	result, err := deployer.Deploy(b.ctx, cfg, rollupCreatorAddr)
	if err != nil {
		return nil, fmt.Errorf("deploy rollup: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("rollup deployment failed: %s", result.Error)
	}

	b.logger.Info("Nitro rollup deployed",
		slog.String("rollup", result.Contracts.Rollup.Hex()),
		slog.String("inbox", result.Contracts.Inbox.Hex()),
		slog.String("bridge", result.Contracts.Bridge.Hex()),
		slog.Uint64("block", result.BlockNumber),
	)

	return result, nil
}

// writeNitroConfigs writes all Nitro bundle configuration files.
func (b *bundleBuilder) writeNitroConfigs(result *nitroDeployResult, celestiaKeyID string) error {
	b.logger.Info("7️⃣  Writing Nitro bundle configs...")

	writer := &NitroConfigWriter{
		logger:        b.logger,
		bundleDir:     b.bundleDir,
		result:        result,
		celestiaKeyID: celestiaKeyID,
	}

	if err := writer.WriteAll(); err != nil {
		return err
	}

	b.logger.Info("All Nitro configs written successfully")
	return nil
}

// moveAnvilStateForNitro moves the anvil state file to the state/ subdirectory.
func (b *bundleBuilder) moveAnvilStateForNitro(stateFile string) error {
	b.logger.Info("5️⃣  Moving anvil state to state/ directory...")

	stateDir := filepath.Join(b.bundleDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	destFile := filepath.Join(stateDir, "anvil-state.json")

	// Read and write instead of rename (may be cross-device)
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return fmt.Errorf("read state file: %w", err)
	}

	if err := os.WriteFile(destFile, data, 0644); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}

	// Remove original
	if err := os.Remove(stateFile); err != nil {
		b.logger.Warn("failed to remove original state file", slog.String("error", err.Error()))
	}

	b.logger.Info("Anvil state moved", slog.String("path", destFile))
	return nil
}

// createNitroArchive creates the Nitro bundle archive.
func (b *bundleBuilder) createNitroArchive() (string, error) {
	b.logger.Info("8️⃣  Creating bundle archive...")
	return b.createArchive("nitro-local-devnet-bundle.tar.gz")
}

// printNitroSuccess prints success message for Nitro bundle.
func (b *bundleBuilder) printNitroSuccess(archivePath string) {
	log.Println()
	log.Println("✅ Nitro bundle created successfully!")
	log.Printf("📦 File: %s\n", archivePath)
	log.Println()
	log.Println("Next steps:")
	log.Println("  1. Extract the bundle: tar xzf " + archivePath)
	log.Println("  2. Review configs in ./bundle/")
	log.Println("  3. ./scripts/start.sh")
	log.Println("  4. ./scripts/test.sh")
	log.Println()
	log.Println("Note: Nitro uses two-phase startup (Issue #4208 workaround)")
	log.Println()
}

// WETH9 bytecode (standard Wrapped Ether contract)
// This is the canonical WETH9 deployed on Ethereum mainnet
const weth9Bytecode = "0x60606040526040805190810160405280600d81526020017f57726170706564204574686572000000000000000000000000000000000000008152506000908051906020019061004f9291906100c8565b506040805190810160405280600481526020017f57455448000000000000000000000000000000000000000000000000000000008152506001908051906020019061009b9291906100c8565b506012600260006101000a81548160ff021916908360ff16021790555034156100c357600080fd5b61016d565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f1061010957805160ff1916838001178555610137565b82800160010185558215610137579182015b8281111561013657825182559160200191906001019061011b565b5b5090506101449190610148565b5090565b61016a91905b8082111561016657600081600090555060010161014e565b5090565b90565b6106598061017c6000396000f30060606040526004361061008e576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063095ea7b31461009357806318160ddd146100ed57806323b872dd14610116578063313ce5671461018f5780636361c39d146101be57806370a0823114610205578063a9059cbb14610252578063dd62ed3e146102ac575b600080fd5b341561009e57600080fd5b6100d3600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091908035906020019091905050610318565b604051808215151515815260200191505060405180910390f35b34156100f857600080fd5b61010061040a565b6040518082815260200191505060405180910390f35b341561012157600080fd5b610175600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803573ffffffffffffffffffffffffffffffffffffffff16906020019091908035906020019091905050610410565b604051808215151515815260200191505060405180910390f35b341561019a57600080fd5b6101a261062e565b604051808260ff1660ff16815260200191505060405180910390f35b6101eb600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050610641565b604051808215151515815260200191505060405180910390f35b341561021057600080fd5b61023c600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190505061069b565b6040518082815260200191505060405180910390f35b341561025d57600080fd5b610292600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919080359060200190919050506106b3565b604051808215151515815260200191505060405180910390f35b34156102b757600080fd5b610302600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506106c8565b6040518082815260200191505060405180910390f35b600081600460003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925846040518082815260200191505060405180910390a36001905092915050565b60035481565b600081600360008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054101580156104fd575081600460008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205410155b801561050a575060008210155b151561051557600080fd5b81600360008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254039250508190555081600360008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254019250508190555081600460008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825403925050819055506001905093915050565b600260009054906101000a900460ff1681565b60003073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561067d57600080fd5b81600360008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055506001905092915050565b60036020528060005260406000206000915090505481565b60006106c0338484610410565b905092915050565b60046020528160005260406000206020528060005260406000206000915091505054815600a165627a7a72305820e7e9c87b51c5bb35f82f2f1f7bb1823c41cfcd8f3ab8c2a5b58baf35e9cbdd4f0029"

// createContractCreation creates a contract creation transaction.
func createContractCreation(nonce uint64, data []byte, gasLimit uint64, gasPrice *big.Int) *types.Transaction {
	return types.NewContractCreation(nonce, big.NewInt(0), gasLimit, gasPrice, data)
}

// waitForReceipt waits for a transaction receipt.
func waitForReceipt(ctx context.Context, client *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	for {
		receipt, err := client.TransactionReceipt(ctx, txHash)
		if err == nil {
			return receipt, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Small sleep to avoid hammering the RPC
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// Continue polling
		}
	}
}
