package popdeployer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/opstack"
	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
)

// Stage represents the deployment stage for progress tracking.
type Stage string

const (
	StageStartingAnvil      Stage = "starting_anvil"
	StageDeployingContracts Stage = "deploying_contracts"
	StageCapturingState     Stage = "capturing_state"
	StageGeneratingConfigs  Stage = "generating_configs"
	StageComplete           Stage = "complete"
)

// String returns the string representation of the stage.
func (s Stage) String() string {
	return string(s)
}

// ProgressCallback is called during deployment to report progress.
type ProgressCallback func(stage Stage, progress float64, message string)

// OrchestratorConfig contains configuration for the orchestrator.
type OrchestratorConfig struct {
	// Logger for structured logging
	Logger *slog.Logger

	// CacheDir for op-deployer artifacts
	CacheDir string

	// WorkDir for temporary files (Anvil state, etc.)
	WorkDir string
}

// Orchestrator coordinates POPKins devnet bundle deployments.
type Orchestrator struct {
	repo   repository.Repository
	config OrchestratorConfig
	logger *slog.Logger
}

// New Orchestrator creates a new POPKins bundle deployment orchestrator.
func NewOrchestrator(
	repo repository.Repository,
	config OrchestratorConfig,
) *Orchestrator {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if config.CacheDir == "" {
		config.CacheDir = filepath.Join(os.TempDir(), "popdeployer-cache")
	}

	if config.WorkDir == "" {
		config.WorkDir = filepath.Join(os.TempDir(), "popdeployer-work")
	}

	return &Orchestrator{
		repo:   repo,
		config: config,
		logger: logger,
	}
}

// Deploy executes a POPKins devnet bundle deployment.
// It runs ephemeral Anvil and popsigner-lite, deploys OP Stack contracts,
// and saves all artifacts for bundle generation.
func (o *Orchestrator) Deploy(ctx context.Context, deploymentID uuid.UUID, onProgress ProgressCallback) error {
	o.logger.Info("starting POPKins devnet bundle deployment",
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

	// 2. Parse deployment config
	var cfg DeploymentConfig
	if err := json.Unmarshal(deployment.Config, &cfg); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	// 3. Populate hardcoded values
	cfg = o.populateDefaults(cfg)

	// 4. Create work directory for this deployment
	workDir := filepath.Join(o.config.WorkDir, deploymentID.String())
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}
	defer os.RemoveAll(workDir) // Clean up after deployment

	// 5. Create deployment context
	deployCtx := &DeploymentContext{
		DeploymentID: deploymentID,
		Config:       &cfg,
		WorkDir:      workDir,
		OnProgress:   onProgress,
	}

	// 6. Execute deployment stages
	stageWriter := &StageWriter{repo: o.repo, deploymentID: deploymentID}

	// Stage 1: Start Anvil
	if err := o.startAnvil(ctx, deployCtx, stageWriter); err != nil {
		return fmt.Errorf("start anvil: %w", err)
	}

	// Stage 2: Deploy OP Stack contracts (Anvil handles signing directly)
	result, err := o.deployOPStack(ctx, deployCtx, stageWriter)
	if err != nil {
		return fmt.Errorf("deploy opstack: %w", err)
	}

	// Stage 3: Capture Anvil state
	if err := o.captureAnvilState(ctx, deployCtx, stageWriter); err != nil {
		return fmt.Errorf("capture anvil state: %w", err)
	}

	// Stage 4: Generate and save all config artifacts
	if err := o.generateConfigs(ctx, deployCtx, result, stageWriter); err != nil {
		return fmt.Errorf("generate configs: %w", err)
	}

	// Stage 5: Mark as complete
	if onProgress != nil {
		onProgress(StageComplete, 1.0, "Bundle deployment complete")
	}

	// Update deployment status to completed (not just the stage)
	stageStr := StageComplete.String()
	if err := o.repo.UpdateDeploymentStatus(ctx, deploymentID, repository.StatusCompleted, &stageStr); err != nil {
		o.logger.Warn("failed to mark deployment as completed",
			slog.String("error", err.Error()),
		)
	}

	o.logger.Info("POPKins devnet bundle deployment completed successfully",
		slog.String("deployment_id", deploymentID.String()),
	)

	return nil
}

// DeploymentContext holds runtime context for a deployment.
type DeploymentContext struct {
	DeploymentID uuid.UUID
	Config       *DeploymentConfig
	WorkDir      string
	OnProgress   ProgressCallback

	// Process handles for cleanup
	AnvilCmd *exec.Cmd
	AnvilIPC string // IPC socket path for Anvil (unique per deployment)
}

// Cleanup terminates any running processes.
func (dc *DeploymentContext) Cleanup() {
	if dc.AnvilCmd != nil && dc.AnvilCmd.Process != nil {
		dc.AnvilCmd.Process.Kill()
	}
}

// StageWriter updates deployment stage in the database.
type StageWriter struct {
	repo         repository.Repository
	deploymentID uuid.UUID
}

// UpdateStage updates the current stage in the database.
func (sw *StageWriter) UpdateStage(ctx context.Context, stage Stage) error {
	stageStr := stage.String()
	return sw.repo.UpdateDeploymentStatus(ctx, sw.deploymentID, repository.StatusRunning, &stageStr)
}

// populateDefaults fills in hardcoded configuration values.
func (o *Orchestrator) populateDefaults(cfg DeploymentConfig) DeploymentConfig {
	// Set hardcoded L1 configuration (Anvil)
	cfg.L1ChainID = 31337
	cfg.L1RPC = "http://localhost:8545"

	// Set hardcoded Anvil accounts
	cfg.DeployerAddress = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266" // anvil-0
	cfg.BatcherAddress = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"  // anvil-1
	cfg.ProposerAddress = "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"  // anvil-2

	// Set hardcoded chain parameters
	if cfg.BlockTime == 0 {
		cfg.BlockTime = 2
	}
	if cfg.GasLimit == 0 {
		cfg.GasLimit = 30000000
	}

	// Note: POPSigner-Lite is NOT needed during bundle build phase
	// We use AnvilSigner for direct ECDSA signing with Anvil's well-known keys
	// POPSigner-Lite is only needed at runtime (op-batcher, op-proposer in docker-compose)

	return cfg
}

// startAnvil starts an ephemeral Anvil L1 node using IPC for isolation.
// Using IPC instead of HTTP ports allows multiple concurrent deployments
// without port conflicts.
func (o *Orchestrator) startAnvil(ctx context.Context, dc *DeploymentContext, sw *StageWriter) error {
	if dc.OnProgress != nil {
		dc.OnProgress(StageStartingAnvil, 0.1, "Starting ephemeral Anvil L1...")
	}
	if err := sw.UpdateStage(ctx, StageStartingAnvil); err != nil {
		o.logger.Warn("failed to update stage", slog.String("error", err.Error()))
	}

	stateFile := filepath.Join(dc.WorkDir, "anvil-state.json")

	// Use IPC socket in the work directory - unique per deployment
	// This avoids port conflicts when multiple deployments run concurrently
	ipcPath := filepath.Join(dc.WorkDir, "anvil.ipc")
	dc.AnvilIPC = ipcPath

	// Update L1RPC to use IPC path instead of HTTP
	// go-ethereum's rpc.DialContext supports IPC paths directly
	dc.Config.L1RPC = ipcPath

	dc.AnvilCmd = exec.CommandContext(ctx, "anvil",
		"--chain-id", fmt.Sprintf("%d", dc.Config.L1ChainID),
		"--accounts", "10",
		"--balance", "10000",
		"--gas-limit", fmt.Sprintf("%d", dc.Config.GasLimit),
		"--block-time", fmt.Sprintf("%d", dc.Config.BlockTime),
		"--ipc", ipcPath,
		"--state", stateFile,
	)

	// Redirect output to log files in work directory
	anvilLog := filepath.Join(dc.WorkDir, "anvil.log")
	logFile, err := os.Create(anvilLog)
	if err != nil {
		return fmt.Errorf("create anvil log: %w", err)
	}
	defer logFile.Close()

	dc.AnvilCmd.Stdout = logFile
	dc.AnvilCmd.Stderr = logFile

	if err := dc.AnvilCmd.Start(); err != nil {
		return fmt.Errorf("start anvil process: %w", err)
	}

	// Wait for Anvil IPC socket to be ready
	if !waitForIPC(ipcPath, 30*time.Second) {
		return fmt.Errorf("anvil IPC socket failed to appear within 30 seconds")
	}

	o.logger.Info("Anvil is ready (IPC mode)",
		slog.String("ipc", ipcPath),
		slog.String("state_file", stateFile),
	)

	return nil
}

// waitForIPC polls for an IPC socket file to exist.
func waitForIPC(path string, timeout time.Duration) bool {
	start := time.Now()
	for time.Since(start) < timeout {
		if _, err := os.Stat(path); err == nil {
			// Socket exists, give it a moment to be ready
			time.Sleep(500 * time.Millisecond)
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// deployOPStack deploys OP Stack contracts using the op-deployer.
func (o *Orchestrator) deployOPStack(ctx context.Context, dc *DeploymentContext, sw *StageWriter) (*opstack.DeployResult, error) {
	if dc.OnProgress != nil {
		dc.OnProgress(StageDeployingContracts, 0.3, "Deploying OP Stack contracts...")
	}
	if err := sw.UpdateStage(ctx, StageDeployingContracts); err != nil {
		o.logger.Warn("failed to update stage", slog.String("error", err.Error()))
	}

	// Create opstack deployment config
	// UseLocalSigning=true skips POPSigner validation - we use AnvilSigner instead
	// FundDevAccounts=true pre-funds Anvil's accounts on L2 for testing
	opstackCfg := &opstack.DeploymentConfig{
		ChainID:         dc.Config.ChainID,
		ChainName:       dc.Config.ChainName,
		L1ChainID:       dc.Config.L1ChainID,
		L1RPC:           dc.Config.L1RPC,
		DeployerAddress: dc.Config.DeployerAddress,
		BatcherAddress:  dc.Config.BatcherAddress,
		ProposerAddress: dc.Config.ProposerAddress,
		BlockTime:       dc.Config.BlockTime,
		GasLimit:        dc.Config.GasLimit,
		UseLocalSigning: true, // Use AnvilSigner for Anvil's well-known keys
		FundDevAccounts: true, // Pre-fund Anvil accounts on L2 for local testing
	}

	// Create deployer
	deployer := opstack.NewOPDeployer(opstack.OPDeployerConfig{
		Logger:   o.logger,
		CacheDir: o.config.CacheDir,
	})

	// Create AnvilSigner for direct local ECDSA signing
	// This is much faster than HTTP-based signing and works with IPC
	chainIDBigInt := new(big.Int).SetUint64(dc.Config.L1ChainID)
	adapter, err := opstack.NewAnvilSigner(chainIDBigInt)
	if err != nil {
		return nil, fmt.Errorf("create anvil signer: %w", err)
	}

	// Progress callback
	progressCallback := func(stage string, progress float64, message string) {
		o.logger.Info(message,
			slog.String("stage", stage),
			slog.Float64("progress", progress*100),
		)
		if dc.OnProgress != nil {
			// Map opstack progress (0.0-1.0) to our range (0.3-0.6)
			adjustedProgress := 0.3 + (progress * 0.3)
			dc.OnProgress(StageDeployingContracts, adjustedProgress, message)
		}
	}

	// Deploy with timeout
	deployCtx, deployCancel := context.WithTimeout(ctx, 30*time.Minute)
	defer deployCancel()

	result, err := deployer.Deploy(deployCtx, opstackCfg, adapter, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("deploy: %w", err)
	}

	o.logger.Info("OP Stack deployment completed",
		slog.Int("chains", len(result.ChainStates)),
	)

	// Populate StartBlock if needed (following orchestrator.go pattern)
	if len(result.ChainStates) == 0 {
		return nil, fmt.Errorf("no chain states returned from deployment")
	}

	chainState := result.ChainStates[0]
	if chainState.StartBlock == nil {
		o.logger.Info("populating StartBlock from L1")

		l1Client, err := ethclient.Dial(dc.Config.L1RPC)
		if err != nil {
			return nil, fmt.Errorf("connect to L1: %w", err)
		}
		defer l1Client.Close()

		header, err := l1Client.HeaderByNumber(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("get L1 header: %w", err)
		}

		chainState.StartBlock = state.BlockRefJsonFromHeader(header)

		// Update in state as well
		for i, c := range result.State.Chains {
			if c.ID == chainState.ID {
				result.State.Chains[i].StartBlock = chainState.StartBlock
				break
			}
		}

		o.logger.Info("StartBlock populated",
			slog.Uint64("block_number", header.Number.Uint64()),
		)
	}

	return result, nil
}

// captureAnvilState gracefully shuts down Anvil to trigger state dump.
func (o *Orchestrator) captureAnvilState(ctx context.Context, dc *DeploymentContext, sw *StageWriter) error {
	if dc.OnProgress != nil {
		dc.OnProgress(StageCapturingState, 0.7, "Capturing Anvil state...")
	}
	if err := sw.UpdateStage(ctx, StageCapturingState); err != nil {
		o.logger.Warn("failed to update stage", slog.String("error", err.Error()))
	}

	// Send SIGTERM for graceful shutdown
	if err := dc.AnvilCmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to anvil: %w", err)
	}

	// Wait for Anvil to finish (with timeout)
	anvilDone := make(chan error, 1)
	go func() {
		anvilDone <- dc.AnvilCmd.Wait()
	}()

	select {
	case err := <-anvilDone:
		if err != nil && err.Error() != "signal: terminated" {
			o.logger.Warn("Anvil exited with error", slog.String("error", err.Error()))
		}
	case <-time.After(5 * time.Second):
		o.logger.Warn("Anvil shutdown timeout, forcing kill")
		dc.AnvilCmd.Process.Kill()
	}

	// Verify state file was created
	stateFile := filepath.Join(dc.WorkDir, "anvil-state.json")
	if info, err := os.Stat(stateFile); err == nil {
		o.logger.Info("Anvil state dumped successfully",
			slog.String("file", stateFile),
			slog.Int64("size_kb", info.Size()/1024),
		)
	} else {
		return fmt.Errorf("anvil state file not created: %w", err)
	}

	return nil
}

// generateConfigs generates all configuration files and saves them as artifacts.
func (o *Orchestrator) generateConfigs(ctx context.Context, dc *DeploymentContext, result *opstack.DeployResult, sw *StageWriter) error {
	if dc.OnProgress != nil {
		dc.OnProgress(StageGeneratingConfigs, 0.8, "Generating configuration files...")
	}
	if err := sw.UpdateStage(ctx, StageGeneratingConfigs); err != nil {
		o.logger.Warn("failed to update stage", slog.String("error", err.Error()))
	}

	// Celestia key ID is hardcoded - popsigner-lite uses deterministic IDs for Anvil accounts
	// anvil-9 = 0xa0Ee7A142d267C1f36714E4a8F75612F20a79720
	celestiaKeyID := "anvil-9"

	o.logger.Info("Using Celestia key",
		slog.String("key_id", celestiaKeyID),
	)

	// Create config writer
	writer := &ConfigWriter{
		logger:        o.logger,
		result:        result,
		config:        dc.Config,
		celestiaKeyID: celestiaKeyID,
	}

	// Generate all configs
	artifacts, err := writer.GenerateAll()
	if err != nil {
		return fmt.Errorf("generate configs: %w", err)
	}

	// Save all artifacts to database
	for artifactType, content := range artifacts {
		// Wrap content for storage (base64 for non-JSON, as-is for JSON)
		jsonContent, err := wrapContentForStorage([]byte(content))
		if err != nil {
			return fmt.Errorf("wrap artifact %s: %w", artifactType, err)
		}

		artifact := &repository.Artifact{
			ID:           uuid.New(),
			DeploymentID: dc.DeploymentID,
			ArtifactType: artifactType,
			Content:      jsonContent,
			CreatedAt:    time.Now(),
		}

		if err := o.repo.SaveArtifact(ctx, artifact); err != nil {
			return fmt.Errorf("save artifact %s: %w", artifactType, err)
		}

		o.logger.Info("Saved artifact",
			slog.String("type", artifactType),
			slog.Int("size_bytes", len(content)),
		)
	}

	// Also save anvil-state.json
	stateFile := filepath.Join(dc.WorkDir, "anvil-state.json")
	stateData, err := os.ReadFile(stateFile)
	if err != nil {
		return fmt.Errorf("read anvil state: %w", err)
	}

	stateArtifact := &repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: dc.DeploymentID,
		ArtifactType: "anvil-state.json",
		Content:      json.RawMessage(stateData),
		CreatedAt:    time.Now(),
	}

	if err := o.repo.SaveArtifact(ctx, stateArtifact); err != nil {
		return fmt.Errorf("save anvil state: %w", err)
	}

	o.logger.Info("All artifacts saved to database",
		slog.Int("count", len(artifacts)+1),
	)

	return nil
}

// wrapContentForStorage wraps content for PostgreSQL JSONB storage.
// For non-JSON content (like config.toml, docker-compose.yml, jwt.txt), wraps as base64 in a JSON object.
// This avoids PostgreSQL JSONB normalization issues with escape sequences.
func wrapContentForStorage(content []byte) (json.RawMessage, error) {
	// Check if content is already valid JSON
	if json.Valid(content) {
		return content, nil
	}

	// Wrap non-JSON content as base64 in a JSON object
	wrapper := struct {
		Type string `json:"_type"`
		Data string `json:"data"`
	}{
		Type: "base64",
		Data: base64.StdEncoding.EncodeToString(content),
	}
	encoded, err := json.Marshal(wrapper)
	if err != nil {
		return nil, fmt.Errorf("marshal non-JSON content: %w", err)
	}
	return encoded, nil
}
