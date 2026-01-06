// Package nitro provides Nitro/Orbit chain deployment orchestration.
package nitro

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"

	boostrepo "github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
	"github.com/Bidon15/popsigner/control-plane/internal/repository"
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

	// UseGoDeployer enables the Go-based deployer instead of TypeScript worker.
	// Default is false (uses TypeScript worker for backward compatibility).
	UseGoDeployer bool

	// ArtifactCacheDir is the directory to cache downloaded contract artifacts.
	// Only used when UseGoDeployer is true.
	ArtifactCacheDir string

	// NitroInfraRepo is the repository for Nitro infrastructure (RollupCreator addresses).
	// Only used when UseGoDeployer is true.
	NitroInfraRepo repository.NitroInfrastructureRepository
}

// Orchestrator coordinates Nitro/Orbit chain deployments.
// It manages the deployment lifecycle, integrating with POPSigner for
// transaction signing via mTLS and generating proper Nitro node config files.
type Orchestrator struct {
	repo               boostrepo.Repository
	certProvider       CertificateProvider
	config             OrchestratorConfig
	logger             *slog.Logger
	artifactDownloader *ContractArtifactDownloader
}

// NewOrchestrator creates a new Nitro deployment orchestrator.
func NewOrchestrator(
	repo boostrepo.Repository,
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

	// Force Go deployer as TypeScript worker has been removed
	config.UseGoDeployer = true

	o := &Orchestrator{
		repo:         repo,
		certProvider: certProvider,
		config:       config,
		logger:       logger,
	}

	// Initialize Go-based deployment components
	o.artifactDownloader = NewContractArtifactDownloader(config.ArtifactCacheDir)
	logger.Info("using Go-based Nitro deployer",
		slog.String("artifact_cache_dir", config.ArtifactCacheDir),
	)

	return o
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
		ChainID:                  config.ChainID,
		ChainName:                config.ChainName,
		ParentChainID:            config.GetParentChainID(),
		ParentChainRpc:           config.GetParentChainRpc(),
		Owner:                    config.DeployerAddress,
		BatchPosters:             config.BatchPosters,
		Validators:               config.Validators,
		StakeToken:               config.StakeToken,
		BaseStake:                config.BaseStake,
		DataAvailability:         config.GetDataAvailability(),
		NativeToken:              config.NativeToken,
		ConfirmPeriodBlocks:      config.ConfirmPeriodBlocks,
		ExtraChallengeTimeBlocks: config.ExtraChallengeTimeBlocks,
		MaxDataSize:              config.MaxDataSize,
		DeployFactoriesToL2:      config.DeployFactoriesToL2,
		PopsignerEndpoint:        o.config.POPSignerMTLSEndpoint,
		ClientCert:               certs.ClientCert,
		ClientKey:                certs.ClientKey,
		CaCert:                   certs.CaCert,
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

	// Execute deployment using Go-based deployer
	return o.deployWithGo(ctx, deploymentID, deployConfig, certs, reportProgress)
}

// deployWithGo executes deployment using the Go-based deployer.
func (o *Orchestrator) deployWithGo(
	ctx context.Context,
	deploymentID uuid.UUID,
	config *DeployConfig,
	certs *CertificateBundle,
	reportProgress func(stage string, progress float64, message string),
) error {
	reportProgress("artifacts", 0.35, "Downloading contract artifacts...")

	// 1. Download artifacts
	artifacts, err := o.artifactDownloader.DownloadDefault(ctx)
	if err != nil {
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("download artifacts: %w", err))
	}

	o.logger.Info("contract artifacts downloaded",
		slog.String("version", artifacts.Version),
	)

	reportProgress("signer", 0.40, "Initializing POPSigner...")

	// 2. Create signer
	signer, err := NewNitroSigner(SignerConfig{
		Endpoint:   o.config.POPSignerMTLSEndpoint,
		ClientCert: certs.ClientCert,
		ClientKey:  certs.ClientKey,
		CACert:     certs.CaCert,
		ChainID:    big.NewInt(config.ParentChainID),
		Address:    common.HexToAddress(config.Owner),
	})
	if err != nil {
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("create signer: %w", err))
	}

	reportProgress("infrastructure", 0.45, "Checking infrastructure...")

	// 3. Get or deploy RollupCreator
	var rollupCreatorAddr common.Address

	// Use well-known infrastructure if available (any version for now)
	// TODO: Implement full infrastructure deployment for v3.2+ with External DA support
	// For now, we use Arbitrum's existing RollupCreator which is v3.1 on most chains
	if addr, version, exists := GetWellKnownRollupCreatorAnyVersion(config.ParentChainID); exists {
		rollupCreatorAddr = addr
		o.logger.Info("using well-known RollupCreator",
			slog.String("address", addr.Hex()),
			slog.String("version", version),
			slog.Int64("chain_id", config.ParentChainID),
		)
		if version != TargetContractVersion {
			o.logger.Warn("well-known RollupCreator is older version, External DA (0x01 header) may not work",
				slog.String("well_known_version", version),
				slog.String("target_version", TargetContractVersion),
			)
		}
	}

	// Check database for our previously deployed infrastructure (if not already found)
	if rollupCreatorAddr == (common.Address{}) && o.config.NitroInfraRepo != nil {
		infra, err := o.config.NitroInfraRepo.Get(ctx, config.ParentChainID)
		if err != nil {
			o.logger.Warn("failed to check infrastructure registry",
				slog.String("error", err.Error()),
			)
		}
		if infra != nil {
			// Check if our deployed version is compatible
			if isVersionCompatible(infra.Version, TargetContractVersion) {
				rollupCreatorAddr = common.HexToAddress(infra.RollupCreatorAddress)
				o.logger.Info("using registered RollupCreator",
					slog.String("address", rollupCreatorAddr.Hex()),
					slog.String("version", infra.Version),
				)
			} else {
				o.logger.Info("registered RollupCreator version incompatible, will deploy new",
					slog.String("registered_version", infra.Version),
					slog.String("required_version", TargetContractVersion),
				)
			}
		}
	}

	// If no RollupCreator found, we need to deploy infrastructure
	if rollupCreatorAddr == (common.Address{}) {
		reportProgress("infrastructure", 0.50, "Deploying Nitro infrastructure...")

		infraDeployer := NewInfrastructureDeployer(
			artifacts,
			signer,
			o.config.NitroInfraRepo,
			o.logger,
		)

		infraResult, err := infraDeployer.EnsureInfrastructure(ctx, &InfrastructureConfig{
			ParentChainID: config.ParentChainID,
			ParentRPC:     config.ParentChainRpc,
		})
		if err != nil {
			return o.failDeployment(ctx, deploymentID, fmt.Errorf("deploy infrastructure: %w", err))
		}

		rollupCreatorAddr = infraResult.RollupCreatorAddress
	}

	reportProgress("deploying", 0.60, "Deploying rollup contracts...")

	// 4. Create RollupDeployer and deploy
	rollupDeployer, err := NewRollupDeployer(artifacts, signer, o.logger)
	if err != nil {
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("create rollup deployer: %w", err))
	}

	// Convert DeployConfig to RollupConfig
	rollupConfig := o.convertToRollupConfig(config)

	result, err := rollupDeployer.Deploy(ctx, rollupConfig, rollupCreatorAddr)
	if err != nil {
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("deploy rollup: %w", err))
	}

	if !result.Success {
		return o.failDeployment(ctx, deploymentID, fmt.Errorf("deployment failed: %s", result.Error))
	}

	reportProgress("artifacts", 0.90, "Generating deployment artifacts...")

	// 5. Save deployment result
	// Convert RollupContracts to CoreContracts for persistence
	coreContracts := &CoreContracts{
		Rollup:                 result.Contracts.Rollup.Hex(),
		Inbox:                  result.Contracts.Inbox.Hex(),
		Outbox:                 result.Contracts.Outbox.Hex(),
		Bridge:                 result.Contracts.Bridge.Hex(),
		SequencerInbox:         result.Contracts.SequencerInbox.Hex(),
		RollupEventInbox:       result.Contracts.RollupEventInbox.Hex(),
		ChallengeManager:       result.Contracts.ChallengeManager.Hex(),
		AdminProxy:             result.Contracts.AdminProxy.Hex(),
		UpgradeExecutor:        result.Contracts.UpgradeExecutor.Hex(),
		ValidatorWalletCreator: result.Contracts.ValidatorWalletCreator.Hex(),
		NativeToken:            result.Contracts.NativeToken.Hex(),
		DeployedAtBlockNumber:  int64(result.Contracts.DeployedAtBlockNumber),
	}

	// Persist to database
	tsResult := &DeployResult{
		Success:         true,
		CoreContracts:   coreContracts,
		TransactionHash: result.TransactionHash.Hex(),
		BlockNumber:     int64(result.BlockNumber),
		ChainConfig:     result.ChainConfig,
	}

	if err := o.saveDeploymentResult(ctx, deploymentID, config, tsResult, certs); err != nil {
		o.logger.Warn("failed to save deployment result",
			slog.String("error", err.Error()),
		)
	}

	reportProgress("completed", 1.0, fmt.Sprintf("Deployment successful! Rollup: %s", coreContracts.Rollup))

	o.logger.Info("Nitro deployment completed successfully (Go deployer)",
		slog.String("deployment_id", deploymentID.String()),
		slog.String("rollup", coreContracts.Rollup),
		slog.String("tx_hash", result.TransactionHash.Hex()),
	)

	return nil
}

// convertToRollupConfig converts the wrapper DeployConfig to RollupConfig.
func (o *Orchestrator) convertToRollupConfig(config *DeployConfig) *RollupConfig {
	rollupCfg := &RollupConfig{
		ChainID:                  config.ChainID,
		ChainName:                config.ChainName,
		ParentChainID:            config.ParentChainID,
		ParentChainRPC:           config.ParentChainRpc,
		Owner:                    common.HexToAddress(config.Owner),
		StakeToken:               common.HexToAddress(config.StakeToken),
		ConfirmPeriodBlocks:      int64(config.ConfirmPeriodBlocks),
		ExtraChallengeTimeBlocks: int64(config.ExtraChallengeTimeBlocks),
		MaxDataSize:              int64(config.MaxDataSize),
		DeployFactoriesToL2:      config.DeployFactoriesToL2,
	}

	// Convert base stake
	if config.BaseStake != "" {
		baseStake, ok := new(big.Int).SetString(config.BaseStake, 10)
		if ok {
			rollupCfg.BaseStake = baseStake
		}
	}
	if rollupCfg.BaseStake == nil {
		rollupCfg.BaseStake = big.NewInt(100000000000000000) // 0.1 ETH default
	}

	// Convert batch posters
	for _, addr := range config.BatchPosters {
		rollupCfg.BatchPosters = append(rollupCfg.BatchPosters, common.HexToAddress(addr))
	}

	// Convert validators
	for _, addr := range config.Validators {
		rollupCfg.Validators = append(rollupCfg.Validators, common.HexToAddress(addr))
	}

	// Convert native token
	if config.NativeToken != "" {
		rollupCfg.NativeToken = common.HexToAddress(config.NativeToken)
	}

	// Convert DA type
	switch config.DataAvailability {
	case "anytrust":
		rollupCfg.DataAvailability = DAModeAnytrust
	case "rollup":
		rollupCfg.DataAvailability = DAModeRollup
	default:
		rollupCfg.DataAvailability = DAModeCelestia
	}

	return rollupCfg
}

// saveDeploymentResult saves the deployment result to the database.
func (o *Orchestrator) saveDeploymentResult(
	ctx context.Context,
	deploymentID uuid.UUID,
	config *DeployConfig,
	result *DeployResult,
	certs *CertificateBundle,
) error {
	// Populate certificates in config for artifact generation
	configWithCerts := *config
	configWithCerts.ClientCert = certs.ClientCert
	configWithCerts.ClientKey = certs.ClientKey
	configWithCerts.CaCert = certs.CaCert

	// Use the artifact generator to save results
	generator := NewArtifactGenerator(o.repo)
	return generator.GenerateArtifacts(ctx, deploymentID, &configWithCerts, result)
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

	if updateErr := o.repo.UpdateDeploymentStatus(ctx, deploymentID, boostrepo.StatusFailed, nil); updateErr != nil {
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
