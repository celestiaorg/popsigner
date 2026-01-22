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
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	// L1 Configuration (Anvil)
	l1ChainID = 31337
	l1RPC     = "http://localhost:8545"

	// L2 Configuration
	l2ChainID   = 42069
	l2ChainName = "local-opstack-devnet"

	// POPSigner-Lite
	popSignerRPCPort  = "8555"
	popSignerRestPort = "3000"
	popSignerRPC      = "http://localhost:" + popSignerRPCPort
	popSignerRestURL  = "http://localhost:" + popSignerRestPort
	popSignerAPIKey   = "psk_local_dev_00000000000000000000000000000000"

	// Anvil Accounts (deterministic)
	deployerAddress = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266" // anvil-0
	batcherAddress  = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8" // anvil-1
	proposerAddress = "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC" // anvil-2
	celestiaAddress = "0xa0Ee7A142d267C1f36714E4a8F75612F20a79720" // anvil-9

	// Chain Parameters
	blockTime = 2
	gasLimit  = 30000000

	// Timeouts
	httpReadyTimeout     = 30 * time.Second
	deploymentTimeout    = 30 * time.Minute
	anvilShutdownTimeout = 5 * time.Second
	httpClientTimeout    = 2 * time.Second
	httpPollInterval     = 1 * time.Second
)

// bundleBuilder orchestrates the creation of a pre-deployed OP Stack devnet bundle.
type bundleBuilder struct {
	ctx       context.Context
	cancel    context.CancelFunc
	logger    *slog.Logger
	bundleDir string

	// Managed processes
	anvilCmd     *exec.Cmd
	popSignerCmd *exec.Cmd
}

func main() {
	bundleDir := parseFlags()

	builder := newBundleBuilder(bundleDir)
	defer builder.cleanup()
	builder.setupSignalHandler()

	if err := builder.run(); err != nil {
		builder.logger.Error("bundle creation failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func parseFlags() string {
	bundleDirFlag := flag.String("bundle-dir", filepath.Join(os.TempDir(), "pop-deployer-bundle"),
		"Directory to write bundle files (default: /tmp/pop-deployer-bundle)")
	flag.Parse()
	return *bundleDirFlag
}

func newBundleBuilder(bundleDir string) *bundleBuilder {
	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return &bundleBuilder{
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger,
		bundleDir: bundleDir,
	}
}

func (b *bundleBuilder) cleanup() {
	b.logger.Info("Cleaning up processes...")
	if b.popSignerCmd != nil && b.popSignerCmd.Process != nil {
		b.popSignerCmd.Process.Kill()
	}
	if b.anvilCmd != nil && b.anvilCmd.Process != nil {
		b.anvilCmd.Process.Kill()
	}
}

func (b *bundleBuilder) setupSignalHandler() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		b.logger.Info("Received interrupt signal, cleaning up...")
		b.cleanup()
		os.Exit(1)
	}()
}

// run executes the 7-step bundle creation pipeline.
func (b *bundleBuilder) run() error {
	b.printBanner()

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

	result, cfg, err := b.deployOPStack()
	if err != nil {
		return fmt.Errorf("deploy OP stack: %w", err)
	}

	if err := b.populateStartBlock(result); err != nil {
		return fmt.Errorf("populate start block: %w", err)
	}

	if err := b.shutdownAnvilAndDumpState(stateFile); err != nil {
		return fmt.Errorf("shutdown anvil: %w", err)
	}

	celestiaKeyID, err := b.getCelestiaKeyID()
	if err != nil {
		return fmt.Errorf("get celestia key: %w", err)
	}

	if err := b.writeConfigs(result, cfg, celestiaKeyID); err != nil {
		return fmt.Errorf("write configs: %w", err)
	}

	archivePath, err := b.createArchive()
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}

	b.printSuccess(archivePath)
	return nil
}

func (b *bundleBuilder) printBanner() {
	log.Println()
	log.Println("🚀 POPKins Bundle Builder")
	log.Println("Creating pre-deployed local devnet bundle...")
	log.Printf("Bundle directory: %s\n", b.bundleDir)
	log.Println()
}

func (b *bundleBuilder) prepareBundleDirectory() error {
	if err := os.RemoveAll(b.bundleDir); err != nil {
		return fmt.Errorf("remove old directory: %w", err)
	}
	if err := os.MkdirAll(b.bundleDir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	return nil
}

func (b *bundleBuilder) startAnvil() (stateFile string, err error) {
	b.logger.Info("1️⃣  Starting ephemeral Anvil...")

	stateFile = filepath.Join(b.bundleDir, "anvil-state.json")
	b.anvilCmd = exec.CommandContext(b.ctx, "anvil",
		"--chain-id", fmt.Sprintf("%d", l1ChainID),
		"--accounts", "10",
		"--balance", "10000",
		"--gas-limit", fmt.Sprintf("%d", gasLimit),
		"--block-time", fmt.Sprintf("%d", blockTime),
		"--port", "8545",
		"--host", "0.0.0.0",
		"--state", stateFile,
	)
	b.anvilCmd.Stdout = os.Stdout
	b.anvilCmd.Stderr = os.Stderr

	if err := b.anvilCmd.Start(); err != nil {
		return "", fmt.Errorf("start process: %w", err)
	}

	if !waitForHTTP(b.ctx, l1RPC, httpReadyTimeout) {
		return "", fmt.Errorf("anvil failed to become ready")
	}

	b.logger.Info("Anvil is ready", slog.String("rpc", l1RPC))
	return stateFile, nil
}

func (b *bundleBuilder) startPOPSignerLite() error {
	b.logger.Info("2️⃣  Starting popsigner-lite...")

	// Build popsigner-lite
	buildCmd := exec.Command("go", "build", "-o", "popsigner-lite", ".")
	buildCmd.Dir = "../popsigner-lite"
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build: %w", err)
	}

	// Run popsigner-lite
	b.popSignerCmd = exec.CommandContext(b.ctx, "../popsigner-lite/popsigner-lite")
	b.popSignerCmd.Env = append(os.Environ(),
		"JSONRPC_PORT="+popSignerRPCPort,
		"REST_API_PORT="+popSignerRestPort,
	)
	b.popSignerCmd.Stdout = os.Stdout
	b.popSignerCmd.Stderr = os.Stderr

	if err := b.popSignerCmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}

	if !waitForHTTP(b.ctx, popSignerRestURL+"/health", httpReadyTimeout) {
		return fmt.Errorf("popsigner-lite failed to become ready")
	}

	b.logger.Info("popsigner-lite is ready", slog.String("rpc", popSignerRPC))
	return nil
}

func (b *bundleBuilder) deployOPStack() (*opstack.DeployResult, *opstack.DeploymentConfig, error) {
	b.logger.Info("3️⃣  Deploying OP Stack contracts...")

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
		Logger:   b.logger,
		CacheDir: filepath.Join(os.TempDir(), "pop-deployer-cache"),
	})

	chainIDBigInt := new(big.Int).SetUint64(l1ChainID)
	adapter := opstack.NewPOPSignerAdapter(l1RPC, popSignerAPIKey, chainIDBigInt)

	progressCallback := func(stage string, progress float64, message string) {
		b.logger.Info(message,
			slog.String("stage", stage),
			slog.Float64("progress", progress*100),
		)
	}

	deployCtx, deployCancel := context.WithTimeout(b.ctx, deploymentTimeout)
	defer deployCancel()

	result, err := deployer.Deploy(deployCtx, cfg, adapter, progressCallback)
	if err != nil {
		return nil, nil, fmt.Errorf("deploy: %w", err)
	}

	b.logger.Info("Deployment completed successfully",
		slog.Int("chains", len(result.ChainStates)),
		slog.String("create2_salt", result.Create2Salt.Hex()),
	)

	return result, cfg, nil
}

func (b *bundleBuilder) populateStartBlock(result *opstack.DeployResult) error {
	if len(result.ChainStates) == 0 {
		return fmt.Errorf("no chain states returned from deployment")
	}

	chainState := result.ChainStates[0]
	if chainState.StartBlock != nil {
		b.logger.Info("StartBlock already populated",
			slog.Uint64("block_number", uint64(chainState.StartBlock.Number)),
		)
		return nil
	}

	b.logger.Info("populating StartBlock from L1 (was nil)")

	l1Client, err := ethclient.Dial(l1RPC)
	if err != nil {
		return fmt.Errorf("connect to L1: %w", err)
	}
	defer l1Client.Close()

	header, err := l1Client.HeaderByNumber(b.ctx, nil)
	if err != nil {
		return fmt.Errorf("get L1 header: %w", err)
	}

	chainState.StartBlock = state.BlockRefJsonFromHeader(header)
	b.logger.Info("StartBlock populated",
		slog.Uint64("block_number", header.Number.Uint64()),
		slog.String("block_hash", header.Hash().Hex()),
	)

	// Update the state's chain reference
	for i, c := range result.State.Chains {
		if c.ID == chainState.ID {
			result.State.Chains[i].StartBlock = chainState.StartBlock
			break
		}
	}

	return nil
}

func (b *bundleBuilder) shutdownAnvilAndDumpState(stateFile string) error {
	b.logger.Info("4️⃣  Shutting down Anvil to dump state...")

	if err := b.anvilCmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM: %w", err)
	}

	// Wait for Anvil to finish with timeout
	anvilDone := make(chan error, 1)
	go func() {
		anvilDone <- b.anvilCmd.Wait()
	}()

	select {
	case err := <-anvilDone:
		if err != nil && err.Error() != "signal: terminated" {
			b.logger.Warn("Anvil exited with error", slog.String("error", err.Error()))
		}
	case <-time.After(anvilShutdownTimeout):
		b.logger.Warn("Anvil shutdown timeout, forcing kill")
		b.anvilCmd.Process.Kill()
	}

	// Verify state file was created
	info, err := os.Stat(stateFile)
	if err != nil {
		return fmt.Errorf("state file not created: %w", err)
	}

	b.logger.Info("Anvil state dumped successfully",
		slog.String("file", stateFile),
		slog.Int64("size_kb", info.Size()/1024),
	)
	return nil
}

func (b *bundleBuilder) getCelestiaKeyID() (string, error) {
	b.logger.Info("5️⃣  Getting Celestia key from popsigner-lite...")

	keyID, err := getKeyID(b.ctx, popSignerRestURL, celestiaAddress)
	if err != nil {
		return "", err
	}

	b.logger.Info("Celestia key found",
		slog.String("key_id", keyID),
		slog.String("address", celestiaAddress),
	)
	return keyID, nil
}

func (b *bundleBuilder) writeConfigs(result *opstack.DeployResult, cfg *opstack.DeploymentConfig, celestiaKeyID string) error {
	b.logger.Info("6️⃣  Writing bundle configs...")

	writer := &ConfigWriter{
		logger:        b.logger,
		bundleDir:     b.bundleDir,
		result:        result,
		config:        cfg,
		celestiaKeyID: celestiaKeyID,
	}

	if err := writer.WriteAll(); err != nil {
		return err
	}

	b.logger.Info("All configs written successfully")
	return nil
}

func (b *bundleBuilder) createArchive() (string, error) {
	b.logger.Info("7️⃣  Creating bundle archive...")

	archiveName := "opstack-local-devnet-bundle.tar.gz"
	tarCmd := exec.Command("tar", "czf", archiveName, "-C", b.bundleDir, ".")
	if err := tarCmd.Run(); err != nil {
		return "", fmt.Errorf("tar: %w", err)
	}

	b.logger.Info("Bundle archive created", slog.String("file", archiveName))

	if stat, err := os.Stat(archiveName); err == nil {
		b.logger.Info("Bundle size",
			slog.Int64("bytes", stat.Size()),
			slog.Int64("mb", stat.Size()/(1024*1024)),
		)
	}

	return archiveName, nil
}

func (b *bundleBuilder) printSuccess(archivePath string) {
	log.Println()
	log.Println("✅ Bundle created successfully!")
	log.Printf("📦 File: %s\n", archivePath)
	log.Println()
	log.Println("Next steps:")
	log.Println("  1. Extract the bundle: tar xzf " + archivePath)
	log.Println("  2. Review configs in ./bundle/")
	log.Println("  3. docker compose up -d")
	log.Println()
}

// waitForHTTP polls an HTTP endpoint until it responds, timeout expires, or context is cancelled.
func waitForHTTP(ctx context.Context, url string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := &http.Client{
		Timeout: httpClientTimeout,
	}

	ticker := time.NewTicker(httpPollInterval)
	defer ticker.Stop()

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return false
		}

		resp, err := client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			resp.Body.Close()
			return true
		}
		if resp != nil {
			resp.Body.Close()
		}

		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			// continue polling
		}
	}
}

// getKeyID retrieves the key ID for a given address from popsigner-lite via REST API.
func getKeyID(ctx context.Context, baseURL, address string) (string, error) {
	url := fmt.Sprintf("%s/v1/keys/%s", baseURL, address)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get key (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Address   string `json:"address"`
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return result.ID, nil
}
