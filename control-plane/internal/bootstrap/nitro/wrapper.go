// Package nitro provides Go wrapper for TypeScript Nitro deployment subprocess.
package nitro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// DeployConfig contains configuration for a Nitro chain deployment.
// This is serialized to JSON and passed to the TypeScript worker.
type DeployConfig struct {
	// Chain configuration
	ChainID   int64  `json:"chainId"`
	ChainName string `json:"chainName"`

	// Parent chain configuration
	ParentChainID  int64  `json:"parentChainId"`
	ParentChainRpc string `json:"parentChainRpc"`

	// Owner and operators
	Owner        string   `json:"owner"`
	BatchPosters []string `json:"batchPosters"`
	Validators   []string `json:"validators"`

	// Staking configuration
	StakeToken string `json:"stakeToken"`
	BaseStake  string `json:"baseStake"`

	// Data availability - defaults to "celestia" if empty
	// POPSigner deployments use Celestia DA by default
	DataAvailability string `json:"dataAvailability,omitempty"`

	// Optional: custom gas token
	NativeToken string `json:"nativeToken,omitempty"`

	// Optional: deployment parameters
	ConfirmPeriodBlocks      int  `json:"confirmPeriodBlocks,omitempty"`
	ExtraChallengeTimeBlocks int  `json:"extraChallengeTimeBlocks,omitempty"`
	MaxDataSize              int  `json:"maxDataSize,omitempty"`
	DeployFactoriesToL2      bool `json:"deployFactoriesToL2,omitempty"`

	// POPSigner mTLS configuration
	PopsignerEndpoint string `json:"popsignerEndpoint"`
	ClientCert        string `json:"clientCert"`
	ClientKey         string `json:"clientKey"`
	CaCert            string `json:"caCert,omitempty"`
}

// CoreContracts contains deployed contract addresses.
type CoreContracts struct {
	Rollup                 string `json:"rollup"`
	Inbox                  string `json:"inbox"`
	Outbox                 string `json:"outbox"`
	Bridge                 string `json:"bridge"`
	SequencerInbox         string `json:"sequencerInbox"`
	RollupEventInbox       string `json:"rollupEventInbox"`
	ChallengeManager       string `json:"challengeManager"`
	AdminProxy             string `json:"adminProxy"`
	UpgradeExecutor        string `json:"upgradeExecutor"`
	ValidatorWalletCreator string `json:"validatorWalletCreator"`
	NativeToken            string `json:"nativeToken"`
	DeployedAtBlockNumber  int64  `json:"deployedAtBlockNumber"`
}

// DeployResult is the result returned from the TypeScript deployment script.
type DeployResult struct {
	Success         bool                   `json:"success"`
	CoreContracts   *CoreContracts         `json:"coreContracts,omitempty"`
	TransactionHash string                 `json:"transactionHash,omitempty"`
	BlockNumber     int64                  `json:"blockNumber,omitempty"`
	ChainConfig     map[string]interface{} `json:"chainConfig,omitempty"`
	Error           string                 `json:"error,omitempty"`
}

// Deployer manages Nitro chain deployments via TypeScript subprocess.
type Deployer struct {
	workerPath string           // Path to the nitro-deployer-worker directory
	nodeCmd    string           // Node.js command (default: "node")
	logger     *slog.Logger     // Logger for deployment progress
	repo       repository.Repository // Repository for persisting state
}

// DeployerOption configures a Deployer.
type DeployerOption func(*Deployer)

// WithNodeCommand sets a custom Node.js command (e.g., for testing).
func WithNodeCommand(cmd string) DeployerOption {
	return func(d *Deployer) {
		d.nodeCmd = cmd
	}
}

// WithLogger sets a custom logger.
func WithLogger(logger *slog.Logger) DeployerOption {
	return func(d *Deployer) {
		d.logger = logger
	}
}

// WithRepository sets the repository for state persistence.
func WithRepository(repo repository.Repository) DeployerOption {
	return func(d *Deployer) {
		d.repo = repo
	}
}

// NewDeployer creates a new Nitro deployer.
func NewDeployer(workerPath string, opts ...DeployerOption) *Deployer {
	d := &Deployer{
		workerPath: workerPath,
		nodeCmd:    "node",
		logger:     slog.Default(),
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// Deploy executes a Nitro chain deployment.
// The deployment is atomic - it either fully succeeds or fails.
func (d *Deployer) Deploy(ctx context.Context, config *DeployConfig) (*DeployResult, error) {
	d.logger.Info("starting Nitro deployment",
		slog.Int64("chain_id", config.ChainID),
		slog.String("chain_name", config.ChainName),
		slog.Int64("parent_chain_id", config.ParentChainID),
	)

	startTime := time.Now()

	// Execute the TypeScript worker
	result, err := d.executeWorker(ctx, config)
	duration := time.Since(startTime)

	if err != nil {
		d.logger.Error("deployment failed",
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if !result.Success {
		d.logger.Error("deployment returned failure",
			slog.Duration("duration", duration),
			slog.String("error", result.Error),
		)
		return result, fmt.Errorf("deployment failed: %s", result.Error)
	}

	d.logger.Info("Nitro deployment successful",
		slog.Duration("duration", duration),
		slog.String("rollup", result.CoreContracts.Rollup),
		slog.String("tx_hash", result.TransactionHash),
		slog.Int64("block_number", result.BlockNumber),
	)

	return result, nil
}

// ProgressCallback reports deployment progress.
// stage: current deployment stage (e.g., "deploying", "generating_artifacts")
// progress: 0.0-1.0 progress within the stage
// message: human-readable status message
type ProgressCallback func(stage string, progress float64, message string)

// DeployWithPersistence deploys and persists state to the repository.
func (d *Deployer) DeployWithPersistence(ctx context.Context, deploymentID uuid.UUID, config *DeployConfig) (*DeployResult, error) {
	return d.DeployWithPersistenceAndProgress(ctx, deploymentID, config, nil)
}

// DeployWithPersistenceAndProgress deploys with persistence and optional progress reporting.
func (d *Deployer) DeployWithPersistenceAndProgress(
	ctx context.Context,
	deploymentID uuid.UUID,
	config *DeployConfig,
	onProgress ProgressCallback,
) (*DeployResult, error) {
	if d.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}

	// Helper to report progress
	reportProgress := func(stage string, progress float64, message string) {
		if onProgress != nil {
			onProgress(stage, progress, message)
		}
		d.logger.Info(message, slog.String("stage", stage), slog.Float64("progress", progress))
	}

	// Update status to running
	stage := "deploying"
	if err := d.repo.UpdateDeploymentStatus(ctx, deploymentID, repository.StatusRunning, &stage); err != nil {
		d.logger.Warn("failed to update deployment status", slog.String("error", err.Error()))
	}

	reportProgress("deploying", 0.1, "Starting Nitro chain deployment")

	// Execute deployment
	result, err := d.Deploy(ctx, config)
	if err != nil {
		// Record error
		if setErr := d.repo.SetDeploymentError(ctx, deploymentID, err.Error()); setErr != nil {
			d.logger.Warn("failed to set deployment error", slog.String("error", setErr.Error()))
		}
		if updateErr := d.repo.UpdateDeploymentStatus(ctx, deploymentID, repository.StatusFailed, nil); updateErr != nil {
			d.logger.Warn("failed to update deployment status", slog.String("error", updateErr.Error()))
		}
		return nil, err
	}

	reportProgress("deploying", 0.7, "Deployment transaction confirmed")

	// Record transaction
	if result.TransactionHash != "" {
		desc := "Nitro chain deployment"
		tx := &repository.Transaction{
			ID:           uuid.New(),
			DeploymentID: deploymentID,
			Stage:        "deployment",
			TxHash:       result.TransactionHash,
			Description:  &desc,
			CreatedAt:    time.Now(),
		}
		if err := d.repo.RecordTransaction(ctx, tx); err != nil {
			d.logger.Warn("failed to record transaction", slog.String("error", err.Error()))
		}
	}

	// Update stage to generating artifacts
	artifactStage := "generating_artifacts"
	if err := d.repo.UpdateDeploymentStatus(ctx, deploymentID, repository.StatusRunning, &artifactStage); err != nil {
		d.logger.Warn("failed to update deployment status", slog.String("error", err.Error()))
	}

	reportProgress("generating_artifacts", 0.8, "Generating Nitro node configuration artifacts")

	// Generate proper Nitro config artifacts (chain-info.json, node-config.json, core-contracts.json)
	if result.CoreContracts != nil {
		generator := NewArtifactGenerator(d.repo)
		if err := generator.GenerateArtifacts(ctx, deploymentID, config, result); err != nil {
			d.logger.Error("failed to generate config artifacts", slog.String("error", err.Error()))
			// Record error but don't fail the deployment - contracts are already deployed
			if setErr := d.repo.SetDeploymentError(ctx, deploymentID, fmt.Sprintf("artifact generation failed: %s", err.Error())); setErr != nil {
				d.logger.Warn("failed to set deployment error", slog.String("error", setErr.Error()))
			}
			// Mark as completed with warning (deployment succeeded, artifacts failed)
			if updateErr := d.repo.UpdateDeploymentStatus(ctx, deploymentID, repository.StatusCompleted, nil); updateErr != nil {
				d.logger.Warn("failed to update deployment status", slog.String("error", updateErr.Error()))
			}
			return result, fmt.Errorf("deployment succeeded but artifact generation failed: %w", err)
		}
	}

	reportProgress("generating_artifacts", 1.0, "Artifacts generated successfully")

	// Mark as completed
	if err := d.repo.UpdateDeploymentStatus(ctx, deploymentID, repository.StatusCompleted, nil); err != nil {
		d.logger.Warn("failed to update deployment status", slog.String("error", err.Error()))
	}

	return result, nil
}

// DeployWithRetry attempts deployment with exponential backoff.
func (d *Deployer) DeployWithRetry(ctx context.Context, config *DeployConfig, maxAttempts int) (*DeployResult, error) {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := d.Deploy(ctx, config)
		if err == nil && result.Success {
			return result, nil
		}

		lastErr = err
		if result != nil {
			lastErr = fmt.Errorf("deployment failed: %s", result.Error)
		}

		d.logger.Warn("deployment attempt failed",
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", maxAttempts),
			slog.String("error", lastErr.Error()),
		)

		if attempt < maxAttempts {
			// Exponential backoff: 1s, 4s, 9s, 16s, ...
			backoff := time.Duration(attempt*attempt) * time.Second
			d.logger.Info("retrying deployment",
				slog.Duration("backoff", backoff),
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// Continue to next attempt
			}
		}
	}

	return nil, fmt.Errorf("all %d deployment attempts failed: %w", maxAttempts, lastErr)
}

// executeWorker runs the TypeScript deployment worker as a subprocess.
func (d *Deployer) executeWorker(ctx context.Context, config *DeployConfig) (*DeployResult, error) {
	// Check if TypeScript worker has been built
	cliPath := filepath.Join(d.workerPath, "dist", "cli.js")
	if _, err := os.Stat(cliPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("TypeScript worker not built: %s does not exist. Run 'npm run build' in %s",
			cliPath, d.workerPath)
	}

	// Serialize config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	// Create temp directory for certificates
	tmpDir, err := os.MkdirTemp("", "nitro-deploy-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write config to temp file
	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, configJSON, 0600); err != nil {
		return nil, fmt.Errorf("write config file: %w", err)
	}

	// Construct command
	cmd := exec.CommandContext(ctx, d.nodeCmd, cliPath, configPath)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	d.logger.Debug("executing TypeScript worker",
		slog.String("command", d.nodeCmd),
		slog.String("cli_path", cliPath),
		slog.String("config_path", configPath),
	)

	// Run the command
	startTime := time.Now()
	runErr := cmd.Run()
	duration := time.Since(startTime)

	// Log stderr output (contains worker logs)
	if stderr.Len() > 0 {
		d.logger.Debug("TypeScript worker stderr",
			slog.String("output", stderr.String()),
		)
	}

	d.logger.Debug("TypeScript worker completed",
		slog.Duration("duration", duration),
		slog.Bool("error", runErr != nil),
	)

	// Parse result from stdout
	var result DeployResult
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			return nil, fmt.Errorf("parse result JSON: %w (stdout: %s, stderr: %s)",
				err, stdout.String(), stderr.String())
		}
	} else {
		// No output - check if there was a run error
		if runErr != nil {
			return nil, fmt.Errorf("worker failed with no output: %w (stderr: %s)",
				runErr, stderr.String())
		}
		return nil, fmt.Errorf("worker produced no output (stderr: %s)", stderr.String())
	}

	// If the command failed but we got a result, return it with context
	if runErr != nil && !result.Success && result.Error == "" {
		result.Error = fmt.Sprintf("worker process failed: %v", runErr)
	}

	return &result, nil
}

// saveArtifacts persists raw deployment artifacts to the repository.
// Deprecated: Use ArtifactGenerator.GenerateArtifacts() instead, which generates
// properly formatted Nitro node config artifacts (chain-info.json, node-config.json).
// This method only saves raw TypeScript output and is kept for backwards compatibility.
func (d *Deployer) saveArtifacts(ctx context.Context, deploymentID uuid.UUID, result *DeployResult) error {
	// Save core contracts as artifact
	if result.CoreContracts != nil {
		contractsJSON, err := json.Marshal(result.CoreContracts)
		if err != nil {
			return fmt.Errorf("marshal core contracts: %w", err)
		}

		artifact := &repository.Artifact{
			ID:           uuid.New(),
			DeploymentID: deploymentID,
			ArtifactType: "core_contracts",
			Content:      contractsJSON,
			CreatedAt:    time.Now(),
		}

		if err := d.repo.SaveArtifact(ctx, artifact); err != nil {
			return fmt.Errorf("save core contracts artifact: %w", err)
		}
	}

	// Save chain config as artifact
	if result.ChainConfig != nil {
		chainConfigJSON, err := json.Marshal(result.ChainConfig)
		if err != nil {
			return fmt.Errorf("marshal chain config: %w", err)
		}

		artifact := &repository.Artifact{
			ID:           uuid.New(),
			DeploymentID: deploymentID,
			ArtifactType: "chain_config",
			Content:      chainConfigJSON,
			CreatedAt:    time.Now(),
		}

		if err := d.repo.SaveArtifact(ctx, artifact); err != nil {
			return fmt.Errorf("save chain config artifact: %w", err)
		}
	}

	return nil
}

// BuildConfigFromDeployment creates a DeployConfig from a repository Deployment.
func BuildConfigFromDeployment(deployment *repository.Deployment, certs CertificateBundle) (*DeployConfig, error) {
	var config DeployConfig
	if err := json.Unmarshal(deployment.Config, &config); err != nil {
		return nil, fmt.Errorf("unmarshal deployment config: %w", err)
	}

	// Add certificate bundle
	config.ClientCert = certs.ClientCert
	config.ClientKey = certs.ClientKey
	config.CaCert = certs.CaCert

	return &config, nil
}

// CertificateBundle contains mTLS certificates for POPSigner authentication.
type CertificateBundle struct {
	ClientCert string // PEM-encoded client certificate
	ClientKey  string // PEM-encoded client private key
	CaCert     string // PEM-encoded CA certificate (optional)
}

// ReadCertificateBundle reads certificates from files.
func ReadCertificateBundle(certPath, keyPath, caPath string) (*CertificateBundle, error) {
	cert, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read client cert: %w", err)
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read client key: %w", err)
	}

	bundle := &CertificateBundle{
		ClientCert: string(cert),
		ClientKey:  string(key),
	}

	if caPath != "" {
		ca, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("read ca cert: %w", err)
		}
		bundle.CaCert = string(ca)
	}

	return bundle, nil
}

// WriteCertificatesToDir writes certificates to a directory for subprocess access.
func WriteCertificatesToDir(dir string, bundle *CertificateBundle) (certPath, keyPath, caPath string, err error) {
	certPath = filepath.Join(dir, "client.crt")
	if err := os.WriteFile(certPath, []byte(bundle.ClientCert), 0600); err != nil {
		return "", "", "", fmt.Errorf("write client cert: %w", err)
	}

	keyPath = filepath.Join(dir, "client.key")
	if err := os.WriteFile(keyPath, []byte(bundle.ClientKey), 0600); err != nil {
		return "", "", "", fmt.Errorf("write client key: %w", err)
	}

	if bundle.CaCert != "" {
		caPath = filepath.Join(dir, "ca.crt")
		if err := os.WriteFile(caPath, []byte(bundle.CaCert), 0600); err != nil {
			return "", "", "", fmt.Errorf("write ca cert: %w", err)
		}
	}

	return certPath, keyPath, caPath, nil
}

// Ensure io.Reader is used to avoid import error
var _ io.Reader = (*bytes.Buffer)(nil)

