// Package nitro provides Nitro/Orbit chain deployment orchestration.
package nitro

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// CertificateProvider provides mTLS certificates for POPSigner authentication.
type CertificateProvider interface {
	// GetCertificates returns the mTLS certificates for the given organization.
	// Returns clientCert, clientKey, caCert (PEM-encoded).
	GetCertificates(ctx context.Context, orgID uuid.UUID) (*CertificateBundle, error)
}

// OrchestratorConfig contains configuration for the Nitro orchestrator.
type OrchestratorConfig struct {
	// Logger for structured logging
	Logger *slog.Logger

	// WorkerPath is the path to the TypeScript nitro-deployer-worker directory
	WorkerPath string

	// POPSignerMTLSEndpoint is the mTLS endpoint for POPSigner (e.g., "https://rpc-mtls.popsigner.com")
	POPSignerMTLSEndpoint string

	// RetryAttempts for transient failures
	RetryAttempts int

	// RetryDelay between retry attempts
	RetryDelay time.Duration
}

// Orchestrator coordinates Nitro/Orbit chain deployments.
// It manages the deployment lifecycle, integrating with POPSigner for
// transaction signing via mTLS and generating proper Nitro node config files.
type Orchestrator struct {
	repo            repository.Repository
	certProvider    CertificateProvider
	deployer        *Deployer
	config          OrchestratorConfig
	logger          *slog.Logger
}

// NewOrchestrator creates a new Nitro deployment orchestrator.
func NewOrchestrator(
	repo repository.Repository,
	certProvider CertificateProvider,
	config OrchestratorConfig,
) *Orchestrator {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if config.RetryAttempts <= 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 5 * time.Second
	}
	if config.POPSignerMTLSEndpoint == "" {
		config.POPSignerMTLSEndpoint = "https://rpc-mtls.popsigner.com"
	}

	// Initialize the deployer wrapper
	deployer := NewDeployer(
		config.WorkerPath,
		WithRepository(repo),
		WithLogger(logger),
	)

	return &Orchestrator{
		repo:         repo,
		certProvider: certProvider,
		deployer:     deployer,
		config:       config,
		logger:       logger,
	}
}

// Deploy executes a Nitro/Orbit chain deployment.
// It loads the deployment configuration, runs the TypeScript deployment script,
// and generates all necessary Nitro node configuration artifacts.
func (o *Orchestrator) Deploy(ctx context.Context, deploymentID uuid.UUID, onProgress ProgressCallback) error {
	o.logger.Info("starting Nitro deployment",
		slog.String("deployment_id", deploymentID.String()),
	)

	// Helper to report progress
	reportProgress := func(stage string, progress float64, message string) {
		if onProgress != nil {
			onProgress(stage, progress, message)
		}
		o.logger.Info(message,
			slog.String("deployment_id", deploymentID.String()),
			slog.String("stage", stage),
			slog.Float64("progress", progress),
		)
	}

	reportProgress("init", 0.0, "Loading deployment configuration")

	// 1. Load deployment from database
	deployment, err := o.repo.GetDeployment(ctx, deploymentID)
	if err != nil {
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("load deployment: %w", err))
	}
	if deployment == nil {
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("deployment not found: %s", deploymentID))
	}

	// 2. Parse deployment config
	var config NitroDeploymentConfigRaw
	if err := json.Unmarshal(deployment.Config, &config); err != nil {
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("parse deployment config: %w", err))
	}

	reportProgress("init", 0.1, "Validating deployment configuration")

	// 3. Get mTLS certificates for POPSigner
	var certs *CertificateBundle
	if o.certProvider != nil {
		o.logger.Info("using CertificateProvider to issue deployment certificate",
			slog.String("deployment_id", deploymentID.String()),
			slog.String("org_id", config.OrgID),
		)

		orgID, err := uuid.Parse(config.OrgID)
		if err != nil {
			return o.failDeployment(ctx, deploymentID, fmt.Errorf("invalid org_id: %w", err))
		}

		certs, err = o.certProvider.GetCertificates(ctx, orgID)
		if err != nil {
			o.logger.Error("failed to issue deployment certificate",
				slog.String("deployment_id", deploymentID.String()),
				slog.String("error", err.Error()),
			)
			return o.failDeployment(ctx, deploymentID, fmt.Errorf("get mTLS certificates: %w", err))
		}

		o.logger.Info("deployment certificate issued successfully",
			slog.String("deployment_id", deploymentID.String()),
			slog.Bool("has_cert", certs.ClientCert != ""),
			slog.Bool("has_key", certs.ClientKey != ""),
		)
	} else {
		o.logger.Warn("no CertificateProvider configured, using certs from config",
			slog.String("deployment_id", deploymentID.String()),
		)
		// For testing or when certs are already in config
		certs = &CertificateBundle{
			ClientCert: config.ClientCert,
			ClientKey:  config.ClientKey,
			CaCert:     config.CaCert,
		}
	}

	if certs.ClientCert == "" || certs.ClientKey == "" {
		o.logger.Error("mTLS certificates missing",
			slog.String("deployment_id", deploymentID.String()),
			slog.Bool("has_cert", certs.ClientCert != ""),
			slog.Bool("has_key", certs.ClientKey != ""),
			slog.Bool("has_provider", o.certProvider != nil),
		)
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("mTLS certificates not available"))
	}

	reportProgress("init", 0.2, "Building deployment configuration")

	// 4. Build DeployConfig for the TypeScript worker
	// Use getter methods to handle both POPKins form names (l1_*) and explicit names (parent_chain_*)
	deployConfig := &DeployConfig{
		ChainID:              config.ChainID,
		ChainName:            config.ChainName,
		ParentChainID:        config.GetParentChainID(),
		ParentChainRpc:       config.GetParentChainRpc(),
		Owner:                config.DeployerAddress,
		BatchPosters:         config.BatchPosters,
		Validators:           config.Validators,
		StakeToken:           config.StakeToken,
		BaseStake:            config.BaseStake,
		DataAvailability:     config.GetDataAvailability(),
		NativeToken:          config.NativeToken,
		ConfirmPeriodBlocks:  config.ConfirmPeriodBlocks,
		ExtraChallengeTimeBlocks: config.ExtraChallengeTimeBlocks,
		MaxDataSize:          config.MaxDataSize,
		DeployFactoriesToL2:  config.DeployFactoriesToL2,
		PopsignerEndpoint:    o.config.POPSignerMTLSEndpoint,
		ClientCert:           certs.ClientCert,
		ClientKey:            certs.ClientKey,
		CaCert:               certs.CaCert,
	}

	// Validate required fields
	if deployConfig.ParentChainRpc == "" {
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("parent chain RPC URL is required (l1_rpc or parent_chain_rpc)"))
	}
	if deployConfig.ParentChainID == 0 {
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("parent chain ID is required (l1_chain_id or parent_chain_id)"))
	}
	if deployConfig.Owner == "" {
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("deployer_address is required"))
	}
	if len(deployConfig.BatchPosters) == 0 {
		// Default to deployer address
		deployConfig.BatchPosters = []string{deployConfig.Owner}
	}
	if len(deployConfig.Validators) == 0 {
		// Default to deployer address
		deployConfig.Validators = []string{deployConfig.Owner}
	}
	if deployConfig.StakeToken == "" {
		// Default to native ETH
		deployConfig.StakeToken = "0x0000000000000000000000000000000000000000"
	}
	if deployConfig.BaseStake == "" {
		// Default base stake (0.1 ETH)
		deployConfig.BaseStake = "100000000000000000"
	}
	if deployConfig.DataAvailability == "" {
		// Default to Celestia
		deployConfig.DataAvailability = "celestia"
	}

	reportProgress("deploying", 0.3, "Executing Nitro chain deployment")

	// 5. Execute deployment with progress reporting
	result, err := o.deployer.DeployWithPersistenceAndProgress(ctx, deploymentID, deployConfig, func(stage string, progress float64, message string) {
		// Map internal progress (0-1) to overall progress (0.3-1.0)
		overallProgress := 0.3 + (progress * 0.7)
		reportProgress(stage, overallProgress, message)
	})

	if err != nil {
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("deployment failed: %w", err))
	}

	if !result.Success {
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("deployment failed: %s", result.Error))
	}

	reportProgress("completed", 1.0, fmt.Sprintf("Deployment successful! Rollup: %s", result.CoreContracts.Rollup))

	o.logger.Info("Nitro deployment completed successfully",
		slog.String("deployment_id", deploymentID.String()),
		slog.String("rollup", result.CoreContracts.Rollup),
		slog.String("tx_hash", result.TransactionHash),
	)

	return nil
}

// failDeployment marks a deployment as failed and returns the error.
func (o *Orchestrator) failDeployment(ctx context.Context, deploymentID uuid.UUID, err error) error {
	o.logger.Error("deployment failed",
		slog.String("deployment_id", deploymentID.String()),
		slog.String("error", err.Error()),
	)

	if setErr := o.repo.SetDeploymentError(ctx, deploymentID, err.Error()); setErr != nil {
		o.logger.Warn("failed to set deployment error", slog.String("error", setErr.Error()))
	}

	if updateErr := o.repo.UpdateDeploymentStatus(ctx, deploymentID, repository.StatusFailed, nil); updateErr != nil {
		o.logger.Warn("failed to update deployment status", slog.String("error", updateErr.Error()))
	}

	return err
}

// NitroDeploymentConfigRaw is the raw config format stored in the database.
// This maps to the API request format with key UUIDs that get resolved to addresses.
// Supports both POPKins form field names (l1_*) and explicit Nitro names (parent_chain_*).
type NitroDeploymentConfigRaw struct {
	// Organization ID (for cert/key lookup)
	OrgID string `json:"org_id"`

	// Chain configuration
	ChainID   int64  `json:"chain_id"`
	ChainName string `json:"chain_name"`

	// Parent chain configuration - supports both naming conventions
	// POPKins form uses l1_chain_id/l1_rpc, but we also accept parent_chain_*
	L1ChainID      int64  `json:"l1_chain_id"`      // From POPKins form
	L1RPC          string `json:"l1_rpc"`           // From POPKins form
	ParentChainID  int64  `json:"parent_chain_id"`  // Explicit Nitro config
	ParentChainRpc string `json:"parent_chain_rpc"` // Explicit Nitro config

	// Deployer address (resolved from deployer_key by unified orchestrator)
	DeployerAddress string `json:"deployer_address"`

	// Operator addresses
	BatchPosters []string `json:"batch_posters"`
	Validators   []string `json:"validators"`

	// Staking configuration
	StakeToken string `json:"stake_token"`
	BaseStake  string `json:"base_stake"`

	// Data availability (celestia, rollup, anytrust) - also accepts "da" from form
	DataAvailability string `json:"data_availability"`
	DA               string `json:"da"` // From POPKins form

	// Optional custom native token
	NativeToken string `json:"native_token,omitempty"`

	// Optional deployment parameters
	ConfirmPeriodBlocks      int  `json:"confirm_period_blocks,omitempty"`
	ExtraChallengeTimeBlocks int  `json:"extra_challenge_time_blocks,omitempty"`
	MaxDataSize              int  `json:"max_data_size,omitempty"`
	DeployFactoriesToL2      bool `json:"deploy_factories_to_l2,omitempty"`

	// POPSigner mTLS certs (may be provided directly or looked up via certProvider)
	ClientCert string `json:"client_cert,omitempty"`
	ClientKey  string `json:"client_key,omitempty"`
	CaCert     string `json:"ca_cert,omitempty"`
}

// GetParentChainID returns the parent chain ID, preferring l1_chain_id if set.
func (c *NitroDeploymentConfigRaw) GetParentChainID() int64 {
	if c.L1ChainID != 0 {
		return c.L1ChainID
	}
	return c.ParentChainID
}

// GetParentChainRpc returns the parent chain RPC URL, preferring l1_rpc if set.
func (c *NitroDeploymentConfigRaw) GetParentChainRpc() string {
	if c.L1RPC != "" {
		return c.L1RPC
	}
	return c.ParentChainRpc
}

// GetDataAvailability returns the DA type, preferring da field if set.
func (c *NitroDeploymentConfigRaw) GetDataAvailability() string {
	if c.DA != "" {
		return c.DA
	}
	if c.DataAvailability != "" {
		return c.DataAvailability
	}
	return "celestia" // Default
}

