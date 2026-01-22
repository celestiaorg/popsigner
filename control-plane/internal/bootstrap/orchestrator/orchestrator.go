// Package orchestrator provides a unified orchestrator that dispatches
// deployments to stack-specific orchestrators (OP Stack or Nitro).
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/nitro"
	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/opstack"
	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/popdeployer"
	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
	"github.com/Bidon15/popsigner/control-plane/internal/models"
)

// KeyResolver resolves key UUIDs to key details.
type KeyResolver interface {
	Get(ctx context.Context, orgID, keyID uuid.UUID) (*models.Key, error)
}

// APIKeyManager manages API keys for organizations.
type APIKeyManager interface {
	// GetOrCreateForDeployment ensures an API key exists for deployment use.
	// Returns the raw API key string.
	GetOrCreateForDeployment(ctx context.Context, orgID uuid.UUID) (string, error)
}

// Orchestrator coordinates chain deployments for any supported stack.
// It dispatches to the appropriate stack-specific orchestrator based
// on the deployment configuration.
type Orchestrator struct {
	repo            repository.Repository
	opstackOrch     *opstack.Orchestrator
	nitroOrch       *nitro.Orchestrator
	popBundleOrch   *popdeployer.Orchestrator
	keyResolver     KeyResolver
	apiKeyManager   APIKeyManager
	signerEndpoint  string
	logger          *slog.Logger
	runningJobs     map[uuid.UUID]context.CancelFunc
}

// Config holds configuration for the orchestrator.
type Config struct {
	Logger         *slog.Logger
	SignerEndpoint string // POPSigner API endpoint (e.g., "https://api.popsigner.com")
}

// New creates a new unified orchestrator.
func New(
	repo repository.Repository,
	opstackOrch *opstack.Orchestrator,
	nitroOrch *nitro.Orchestrator,
	popBundleOrch *popdeployer.Orchestrator,
	keyResolver KeyResolver,
	apiKeyManager APIKeyManager,
	cfg Config,
) *Orchestrator {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Default signer endpoint if not provided
	signerEndpoint := cfg.SignerEndpoint
	if signerEndpoint == "" {
		signerEndpoint = "http://localhost:8080" // Default for local dev
	}

	return &Orchestrator{
		repo:            repo,
		opstackOrch:     opstackOrch,
		nitroOrch:       nitroOrch,
		popBundleOrch:   popBundleOrch,
		keyResolver:     keyResolver,
		apiKeyManager:   apiKeyManager,
		signerEndpoint:  signerEndpoint,
		logger:          logger,
		runningJobs:     make(map[uuid.UUID]context.CancelFunc),
	}
}

// StartDeployment starts a deployment asynchronously.
// This implements the handler.Orchestrator interface.
func (o *Orchestrator) StartDeployment(ctx context.Context, deploymentID uuid.UUID) error {
	// Get deployment to determine stack
	deployment, err := o.repo.GetDeployment(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}
	if deployment == nil {
		return fmt.Errorf("deployment not found: %s", deploymentID)
	}

	o.logger.Info("starting deployment",
		slog.String("deployment_id", deploymentID.String()),
		slog.String("stack", string(deployment.Stack)),
		slog.Int64("chain_id", deployment.ChainID),
	)

	// Enrich the config with POPSigner details
	enrichedConfig, err := o.enrichConfig(ctx, deployment.Config)
	if err != nil {
		o.logger.Error("failed to enrich deployment config",
			slog.String("deployment_id", deploymentID.String()),
			slog.String("error", err.Error()),
		)
		_ = o.repo.SetDeploymentError(ctx, deploymentID, fmt.Sprintf("config enrichment failed: %s", err.Error()))
		return fmt.Errorf("enrich config: %w", err)
	}

	// Update deployment with enriched config
	if err := o.repo.UpdateDeploymentConfig(ctx, deploymentID, enrichedConfig); err != nil {
		o.logger.Error("failed to update deployment config",
			slog.String("deployment_id", deploymentID.String()),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("update config: %w", err)
	}

	// Update status to running
	if err := o.repo.UpdateDeploymentStatus(ctx, deploymentID, repository.StatusRunning, nil); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// Create a cancellable context for this deployment
	deployCtx, cancel := context.WithCancel(context.Background())
	o.runningJobs[deploymentID] = cancel

	// Run the deployment in a goroutine
	go func() {
		defer func() {
			delete(o.runningJobs, deploymentID)
			cancel()
		}()

		var deployErr error
		switch deployment.Stack {
		case repository.StackOPStack:
			if o.opstackOrch != nil {
				deployErr = o.opstackOrch.Deploy(deployCtx, deploymentID, func(stage opstack.Stage, progress float64, message string) {
					o.logger.Info("deployment progress",
						slog.String("deployment_id", deploymentID.String()),
						slog.String("stage", stage.String()),
						slog.Float64("progress", progress),
						slog.String("message", message),
					)
				})
			} else {
				deployErr = fmt.Errorf("OP Stack orchestrator not configured")
			}

		case repository.StackNitro:
			if o.nitroOrch != nil {
				deployErr = o.nitroOrch.Deploy(deployCtx, deploymentID, func(stage string, progress float64, message string) {
					o.logger.Info("deployment progress",
						slog.String("deployment_id", deploymentID.String()),
						slog.String("stage", stage),
						slog.Float64("progress", progress),
						slog.String("message", message),
					)
				})
			} else {
				deployErr = fmt.Errorf("Nitro orchestrator not configured")
			}

		case repository.StackPopBundle:
			if o.popBundleOrch != nil {
				deployErr = o.popBundleOrch.Deploy(deployCtx, deploymentID, func(stage popdeployer.Stage, progress float64, message string) {
					o.logger.Info("deployment progress",
						slog.String("deployment_id", deploymentID.String()),
						slog.String("stage", stage.String()),
						slog.Float64("progress", progress),
						slog.String("message", message),
					)
				})
			} else {
				deployErr = fmt.Errorf("POPKins bundle orchestrator not configured")
			}

		default:
			deployErr = fmt.Errorf("unsupported stack: %s", deployment.Stack)
		}

		if deployErr != nil {
			o.logger.Error("deployment failed",
				slog.String("deployment_id", deploymentID.String()),
				slog.String("error", deployErr.Error()),
			)
			_ = o.repo.SetDeploymentError(context.Background(), deploymentID, deployErr.Error())
		} else {
			o.logger.Info("deployment completed successfully",
				slog.String("deployment_id", deploymentID.String()),
			)
		}
	}()

	return nil
}

// enrichConfig adds POPSigner endpoint, API key, and key addresses to the deployment config.
func (o *Orchestrator) enrichConfig(ctx context.Context, rawConfig json.RawMessage) (json.RawMessage, error) {
	// Parse the existing config
	var config map[string]interface{}
	if err := json.Unmarshal(rawConfig, &config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Get org ID from config
	orgIDStr, ok := config["org_id"].(string)
	if !ok || orgIDStr == "" {
		return nil, fmt.Errorf("org_id not found in config")
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid org_id: %w", err)
	}

	// Add POPSigner endpoint
	config["popsigner_endpoint"] = o.signerEndpoint + "/v1"

	// Check if this is a pop-bundle deployment (uses Anvil keys, no POPSigner needed)
	stackStr, _ := config["stack"].(string)
	isPopBundle := stackStr == "pop-bundle"

	// For pop-bundle, we don't need POPSigner API keys or key resolution
	// (it uses Anvil's deterministic accounts)
	if !isPopBundle {
		// Get or create API key for this org's deployments
		if o.apiKeyManager != nil {
			apiKey, err := o.apiKeyManager.GetOrCreateForDeployment(ctx, orgID)
			if err != nil {
				return nil, fmt.Errorf("get/create API key: %w", err)
			}
			config["popsigner_api_key"] = apiKey
		} else {
			o.logger.Warn("no API key manager configured, deployments may fail authentication")
			config["popsigner_api_key"] = "" // Will likely fail, but allows testing config parsing
		}

		// Resolve deployer key UUID to address
		if o.keyResolver != nil {
			deployerKeyStr, ok := config["deployer_key"].(string)
			if ok && deployerKeyStr != "" {
				deployerKeyID, err := uuid.Parse(deployerKeyStr)
				if err == nil {
					key, err := o.keyResolver.Get(ctx, orgID, deployerKeyID)
					if err != nil {
						return nil, fmt.Errorf("get deployer key: %w", err)
					}
					// Use EVM address if available, otherwise fall back to Address
					if key.EthAddress != nil && *key.EthAddress != "" {
						config["deployer_address"] = *key.EthAddress
					} else {
						config["deployer_address"] = key.Address
					}
					o.logger.Info("resolved deployer key",
						slog.String("key_id", deployerKeyID.String()),
						slog.String("address", config["deployer_address"].(string)),
					)
				}
			}

			// Resolve batcher key (optional, defaults to deployer if not found)
			batcherKeyStr, ok := config["batcher_key"].(string)
			if ok && batcherKeyStr != "" {
				batcherKeyID, err := uuid.Parse(batcherKeyStr)
				if err == nil {
					key, err := o.keyResolver.Get(ctx, orgID, batcherKeyID)
					if err == nil {
						if key.EthAddress != nil && *key.EthAddress != "" {
							config["batcher_address"] = *key.EthAddress
						} else {
							config["batcher_address"] = key.Address
						}
					}
				}
			}

			// Resolve proposer key (optional, defaults to deployer if not found)
			proposerKeyStr, ok := config["proposer_key"].(string)
			if ok && proposerKeyStr != "" {
				proposerKeyID, err := uuid.Parse(proposerKeyStr)
				if err == nil {
					key, err := o.keyResolver.Get(ctx, orgID, proposerKeyID)
					if err == nil {
						if key.EthAddress != nil && *key.EthAddress != "" {
							config["proposer_address"] = *key.EthAddress
						} else {
							config["proposer_address"] = key.Address
						}
					}
				}
			}
		} else {
			o.logger.Warn("no key resolver configured, key addresses must be in config")
		}
	} else {
		o.logger.Info("pop-bundle deployment: skipping POPSigner key resolution (uses Anvil accounts)")
	}

	// Marshal the enriched config
	enriched, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal enriched config: %w", err)
	}

	return enriched, nil
}

// StopDeployment cancels a running deployment.
func (o *Orchestrator) StopDeployment(deploymentID uuid.UUID) error {
	cancel, ok := o.runningJobs[deploymentID]
	if !ok {
		return fmt.Errorf("deployment not running: %s", deploymentID)
	}

	cancel()
	return o.repo.UpdateDeploymentStatus(context.Background(), deploymentID, repository.StatusPaused, nil)
}

// ProcessPendingDeployments starts any deployments that are in pending state.
// This is called at startup to resume any deployments that were interrupted.
func (o *Orchestrator) ProcessPendingDeployments(ctx context.Context) error {
	pending, err := o.repo.ListDeploymentsByStatus(ctx, repository.StatusPending)
	if err != nil {
		return fmt.Errorf("list pending deployments: %w", err)
	}

	for _, d := range pending {
		o.logger.Info("found pending deployment, starting",
			slog.String("deployment_id", d.ID.String()),
			slog.String("stack", string(d.Stack)),
		)
		if err := o.StartDeployment(ctx, d.ID); err != nil {
			o.logger.Error("failed to start pending deployment",
				slog.String("deployment_id", d.ID.String()),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}
