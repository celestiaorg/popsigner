package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/opstack"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
)

const (
	// L1 Configuration (Anvil)
	l1ChainID = 31337
	l1RPC     = "http://localhost:8545"

	// L2 Configuration
	l2ChainID   = 42069
	l2ChainName = "local-opstack-devnet"

	// POPSigner-Lite
	popSignerRPC    = "http://localhost:8555"
	popSignerAPIKey = "psk_local_dev_00000000000000000000000000000000"

	// Anvil Accounts (deterministic)
	deployerAddress = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266" // anvil-0
	batcherAddress  = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"  // anvil-1
	proposerAddress = "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"  // anvil-2

	// Chain Parameters
	blockTime = 2
	gasLimit  = 30000000
)

func main() {
	// Parse command-line flags
	bundleDirFlag := flag.String("bundle-dir", filepath.Join(os.TempDir(), "pop-deployer-bundle"),
		"Directory to write bundle files (default: /tmp/pop-deployer-bundle)")
	flag.Parse()

	// Dereference the flag to get the actual string value
	bundleDir := *bundleDirFlag

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Track processes for cleanup
	var anvilCmd *exec.Cmd
	var popSignerCmd *exec.Cmd

	// Cleanup handler
	cleanup := func() {
		logger.Info("Cleaning up processes...")
		if popSignerCmd != nil && popSignerCmd.Process != nil {
			popSignerCmd.Process.Kill()
		}
		if anvilCmd != nil && anvilCmd.Process != nil {
			anvilCmd.Process.Kill()
		}
	}
	defer cleanup()

	// Handle interrupts
	go func() {
		<-sigChan
		logger.Info("Received interrupt signal, cleaning up...")
		cleanup()
		os.Exit(1)
	}()

	log.Println()
	log.Println("🚀 POPKins Bundle Builder")
	log.Println("Creating pre-deployed local devnet bundle...")
	log.Printf("Bundle directory: %s\n", bundleDir)
	log.Println()

	// Create bundle directory (remove if exists to start fresh)
	if err := os.RemoveAll(bundleDir); err != nil {
		logger.Error("failed to remove old bundle directory", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		logger.Error("failed to create bundle directory", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Step 1: Start ephemeral Anvil
	logger.Info("1️⃣  Starting ephemeral Anvil...")
	stateFile := filepath.Join(bundleDir, "anvil-state.json")
	anvilCmd = exec.CommandContext(ctx, "anvil",
		"--chain-id", fmt.Sprintf("%d", l1ChainID),
		"--accounts", "10",
		"--balance", "10000",
		"--gas-limit", fmt.Sprintf("%d", gasLimit),
		"--block-time", fmt.Sprintf("%d", blockTime),
		"--port", "8545",
		"--host", "0.0.0.0",
		"--state", stateFile,
	)
	anvilCmd.Stdout = os.Stdout
	anvilCmd.Stderr = os.Stderr

	if err := anvilCmd.Start(); err != nil {
		logger.Error("failed to start Anvil", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Wait for Anvil to be ready
	if !waitForHTTP("http://localhost:8545", 30*time.Second) {
		logger.Error("Anvil failed to start")
		os.Exit(1)
	}
	logger.Info("Anvil is ready", slog.String("rpc", l1RPC))

	// Step 2: Start popsigner-lite
	logger.Info("2️⃣  Starting popsigner-lite...")

	// Build popsigner-lite if not already built
	// Path is relative to pop-deployer directory: ../popsigner-lite
	buildCmd := exec.Command("go", "build", "-o", "popsigner-lite", ".")
	buildCmd.Dir = "../popsigner-lite"
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		logger.Error("failed to build popsigner-lite", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Run popsigner-lite from its directory
	popSignerCmd = exec.CommandContext(ctx, "../popsigner-lite/popsigner-lite")
	popSignerCmd.Env = append(os.Environ(),
		"JSONRPC_PORT=8555",
		"REST_API_PORT=3000",
	)
	popSignerCmd.Stdout = os.Stdout
	popSignerCmd.Stderr = os.Stderr

	if err := popSignerCmd.Start(); err != nil {
		logger.Error("failed to start popsigner-lite", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Wait for popsigner-lite to be ready
	if !waitForHTTP("http://localhost:3000/health", 30*time.Second) {
		logger.Error("popsigner-lite failed to start")
		os.Exit(1)
	}
	logger.Info("popsigner-lite is ready", slog.String("rpc", popSignerRPC))

	// Step 3: Deploy OP Stack contracts
	logger.Info("3️⃣  Deploying OP Stack contracts...")

	cfg := &opstack.DeploymentConfig{
		ChainID:           l2ChainID,
		ChainName:         l2ChainName,
		L1ChainID:         l1ChainID,
		L1RPC:             l1RPC,
		POPSignerEndpoint: popSignerRPC,
		POPSignerAPIKey:   popSignerAPIKey,
		DeployerAddress:   deployerAddress,
		BatcherAddress:    batcherAddress,
		ProposerAddress:   proposerAddress,
		BlockTime:         blockTime,
		GasLimit:          gasLimit,
	}

	deployer := opstack.NewOPDeployer(opstack.OPDeployerConfig{
		Logger:   logger,
		CacheDir: filepath.Join(os.TempDir(), "pop-deployer-cache"),
	})

	chainIDBigInt := new(big.Int).SetUint64(l1ChainID)
	adapter := opstack.NewPOPSignerAdapter(
		popSignerRPC,
		popSignerAPIKey,
		chainIDBigInt,
	)

	var result *opstack.DeployResult
	var deployErr error

	progressCallback := func(stage string, progress float64, message string) {
		logger.Info(message,
			slog.String("stage", stage),
			slog.Float64("progress", progress*100),
		)
	}

	// Run deployment with timeout
	deployCtx, deployCancel := context.WithTimeout(ctx, 30*time.Minute)
	defer deployCancel()

	result, deployErr = deployer.Deploy(deployCtx, cfg, adapter, progressCallback)
	if deployErr != nil {
		logger.Error("deployment failed", slog.String("error", deployErr.Error()))
		os.Exit(1)
	}

	logger.Info("Deployment completed successfully",
		slog.Int("chains", len(result.ChainStates)),
		slog.String("create2_salt", result.Create2Salt.Hex()),
	)

	// Populate StartBlock (required for genesis generation)
	// This follows the pattern from orchestrator.go lines 319-334
	if len(result.ChainStates) == 0 {
		logger.Error("no chain states returned from deployment")
		os.Exit(1)
	}

	chainState := result.ChainStates[0]

	// Ensure StartBlock is populated (required for GenesisAndRollup)
	if chainState.StartBlock == nil {
		logger.Info("populating StartBlock from L1 (was nil)")

		// Connect to L1 to fetch latest block
		l1Client, err := ethclient.Dial(l1RPC)
		if err != nil {
			logger.Error("failed to connect to L1 for StartBlock", slog.String("error", err.Error()))
			os.Exit(1)
		}
		defer l1Client.Close()

		header, err := l1Client.HeaderByNumber(ctx, nil)
		if err != nil {
			logger.Error("failed to get L1 header for StartBlock", slog.String("error", err.Error()))
			os.Exit(1)
		}

		chainState.StartBlock = state.BlockRefJsonFromHeader(header)
		logger.Info("StartBlock populated",
			slog.Uint64("block_number", header.Number.Uint64()),
			slog.String("block_hash", header.Hash().Hex()),
		)

		// Also update the state's chain
		for i, c := range result.State.Chains {
			if c.ID == chainState.ID {
				result.State.Chains[i].StartBlock = chainState.StartBlock
				break
			}
		}
	} else {
		logger.Info("StartBlock already populated",
			slog.Uint64("block_number", uint64(chainState.StartBlock.Number)),
		)
	}

	// Step 4: Shutdown Anvil gracefully to trigger state dump
	logger.Info("4️⃣  Shutting down Anvil to dump state...")

	// Send SIGTERM to Anvil for graceful shutdown
	if err := anvilCmd.Process.Signal(syscall.SIGTERM); err != nil {
		logger.Error("failed to send SIGTERM to Anvil", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Wait for Anvil to finish (with timeout)
	anvilDone := make(chan error, 1)
	go func() {
		anvilDone <- anvilCmd.Wait()
	}()

	select {
	case err := <-anvilDone:
		if err != nil && err.Error() != "signal: terminated" {
			logger.Warn("Anvil exited with error", slog.String("error", err.Error()))
		}
	case <-time.After(5 * time.Second):
		logger.Warn("Anvil shutdown timeout, forcing kill")
		anvilCmd.Process.Kill()
	}

	// Verify state file was created
	if info, err := os.Stat(stateFile); err == nil {
		logger.Info("Anvil state dumped successfully",
			slog.String("file", stateFile),
			slog.Int64("size_kb", info.Size()/1024),
		)
	} else {
		logger.Error("Anvil state file not created", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Step 5: Get Celestia key from popsigner-lite
	// We use Anvil account #9 as the Celestia key (0xa0Ee7A142d267C1f36714E4a8F75612F20a79720)
	// This key is deterministic and exists in both the ephemeral popsigner-lite (during deployment)
	// and the Docker Compose popsigner-lite (at runtime)
	logger.Info("5️⃣  Getting Celestia key from popsigner-lite...")

	celestiaAddress := "0xa0Ee7A142d267C1f36714E4a8F75612F20a79720" // anvil-9
	celestiaKeyID, err := getKeyID("http://localhost:3000", celestiaAddress)
	if err != nil {
		logger.Error("failed to get Celestia key", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("Celestia key found",
		slog.String("key_id", celestiaKeyID),
		slog.String("address", celestiaAddress))

	// Step 6: Write all configs
	logger.Info("6️⃣  Writing bundle configs...")

	writer := &ConfigWriter{
		logger:        logger,
		bundleDir:     bundleDir,
		result:        result,
		config:        cfg,
		celestiaKeyID: celestiaKeyID,
	}

	if err := writer.WriteAll(); err != nil {
		logger.Error("failed to write configs", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("All configs written successfully")

	// Step 7: Package bundle
	logger.Info("7️⃣  Creating bundle archive...")

	archiveName := "opstack-local-devnet-bundle.tar.gz"
	tarCmd := exec.Command("tar", "czf", archiveName, "-C", bundleDir, ".")
	if err := tarCmd.Run(); err != nil {
		logger.Error("failed to create bundle archive", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("Bundle archive created", slog.String("file", archiveName))

	// Get file size
	stat, err := os.Stat(archiveName)
	if err == nil {
		logger.Info("Bundle size", slog.Int64("bytes", stat.Size()), slog.Int64("mb", stat.Size()/(1024*1024)))
	}

	log.Println()
	log.Println("✅ Bundle created successfully!")
	log.Printf("📦 File: %s\n", archiveName)
	log.Println()
	log.Println("Next steps:")
	log.Println("  1. Extract the bundle: tar xzf " + archiveName)
	log.Println("  2. Review configs in ./bundle/")
	log.Println("  3. docker compose up -d")
	log.Println()

	// Handle shutdown signal
	go func() {
		<-sigChan
		logger.Info("Received shutdown signal, cleaning up...")
		cancel()
	}()
}

// waitForHTTP polls an HTTP endpoint until it responds or timeout expires.
func waitForHTTP(url string, timeout time.Duration) bool {
	start := time.Now()
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	for time.Since(start) < timeout {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode < 500 {
			resp.Body.Close()
			return true
		}
		time.Sleep(1 * time.Second)
	}

	return false
}

// getKeyID retrieves the key ID for a given address from popsigner-lite via REST API
func getKeyID(baseURL string, address string) (string, error) {
	// Send GET request to retrieve key by address
	resp, err := http.Get(fmt.Sprintf("%s/v1/keys/%s", baseURL, address))
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get key (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Address   string `json:"address"`
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.ID, nil
}
