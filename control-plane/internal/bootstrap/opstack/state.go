// Package opstack provides OP Stack chain deployment infrastructure.
package opstack

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// Stage represents a deployment stage.
type Stage string

const (
	// StageInit is the initial deployment stage.
	StageInit Stage = "init"
	// StageSuperchain deploys superchain contracts.
	StageSuperchain Stage = "deploy_superchain"
	// StageImplementations deploys implementation contracts.
	StageImplementations Stage = "deploy_implementations"
	// StageOPChain deploys the OP chain contracts.
	StageOPChain Stage = "deploy_opchain"
	// StageAltDA deploys Alt-DA contracts (if enabled).
	StageAltDA Stage = "deploy_alt_da"
	// StageGenesis generates L2 genesis.
	StageGenesis Stage = "generate_genesis"
	// StageStartBlock sets the start block.
	StageStartBlock Stage = "set_start_block"
	// StageCompleted indicates deployment is complete.
	StageCompleted Stage = "completed"
)

// String returns the string representation of the stage.
func (s Stage) String() string {
	return string(s)
}

// StateWriter provides deployment state persistence for OP Stack deployments.
// It wraps the repository layer and tracks:
// - Current deployment stage
// - Transaction history for each stage
// - Deployment state (op-deployer's state.json)
// - Resume capability from last successful transaction
type StateWriter struct {
	repo         repository.Repository
	deploymentID uuid.UUID
	onUpdate     func(deploymentID uuid.UUID, stage string) // Optional callback for real-time updates
}

// NewStateWriter creates a new StateWriter for the given deployment.
func NewStateWriter(repo repository.Repository, deploymentID uuid.UUID) *StateWriter {
	return &StateWriter{
		repo:         repo,
		deploymentID: deploymentID,
	}
}

// SetUpdateCallback sets an optional callback for state updates.
// This can be used for WebSocket notifications or other real-time updates.
func (w *StateWriter) SetUpdateCallback(fn func(uuid.UUID, string)) {
	w.onUpdate = fn
}

// DeploymentID returns the deployment ID this writer is associated with.
func (w *StateWriter) DeploymentID() uuid.UUID {
	return w.deploymentID
}

// WriteState persists the current deployment state.
// This is called after each stage to save progress.
// The state is stored as JSON in the artifacts table.
func (w *StateWriter) WriteState(ctx context.Context, state interface{}) error {
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	artifact := &repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: w.deploymentID,
		ArtifactType: "deployment_state",
		Content:      stateBytes,
		CreatedAt:    time.Now(),
	}

	if err := w.repo.SaveArtifact(ctx, artifact); err != nil {
		return fmt.Errorf("save state artifact: %w", err)
	}

	return nil
}

// ReadState retrieves the last saved deployment state.
// Returns nil if no state exists yet.
func (w *StateWriter) ReadState(ctx context.Context) (json.RawMessage, error) {
	artifact, err := w.repo.GetArtifact(ctx, w.deploymentID, "deployment_state")
	if err != nil {
		return nil, fmt.Errorf("get state artifact: %w", err)
	}
	if artifact == nil {
		return nil, nil
	}

	return artifact.Content, nil
}

// UpdateStage updates the current deployment stage and optionally notifies listeners.
func (w *StateWriter) UpdateStage(ctx context.Context, stage Stage) error {
	stageStr := stage.String()
	if err := w.repo.UpdateDeploymentStatus(ctx, w.deploymentID, repository.StatusRunning, &stageStr); err != nil {
		return fmt.Errorf("update stage: %w", err)
	}

	// Notify listeners if callback is set
	if w.onUpdate != nil {
		w.onUpdate(w.deploymentID, stageStr)
	}

	return nil
}

// GetCurrentStage returns the current deployment stage.
func (w *StateWriter) GetCurrentStage(ctx context.Context) (Stage, error) {
	deployment, err := w.repo.GetDeployment(ctx, w.deploymentID)
	if err != nil {
		return "", fmt.Errorf("get deployment: %w", err)
	}
	if deployment == nil {
		return "", fmt.Errorf("deployment not found: %s", w.deploymentID)
	}

	if deployment.CurrentStage == nil {
		return StageInit, nil
	}

	return Stage(*deployment.CurrentStage), nil
}

// RecordTransaction records a transaction for a given stage.
// This enables idempotency - we can check if a transaction was already submitted.
func (w *StateWriter) RecordTransaction(ctx context.Context, stage Stage, txHash string, description string) error {
	var desc *string
	if description != "" {
		desc = &description
	}

	tx := &repository.Transaction{
		ID:           uuid.New(),
		DeploymentID: w.deploymentID,
		Stage:        stage.String(),
		TxHash:       txHash,
		Description:  desc,
		CreatedAt:    time.Now(),
	}

	if err := w.repo.RecordTransaction(ctx, tx); err != nil {
		return fmt.Errorf("record transaction: %w", err)
	}

	return nil
}

// HasTransaction checks if a transaction with the given hash was already recorded.
// This is used for idempotency checks during resume.
func (w *StateWriter) HasTransaction(ctx context.Context, txHash string) (bool, error) {
	tx, err := w.repo.GetTransactionByHash(ctx, txHash)
	if err != nil {
		// Treat errors as "not found" to allow retry
		return false, nil
	}
	return tx != nil, nil
}

// GetTransactions returns all transactions for this deployment.
func (w *StateWriter) GetTransactions(ctx context.Context) ([]repository.Transaction, error) {
	txs, err := w.repo.GetTransactionsByDeployment(ctx, w.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("get transactions: %w", err)
	}
	return txs, nil
}

// GetTransactionsByStage returns transactions for a specific stage.
func (w *StateWriter) GetTransactionsByStage(ctx context.Context, stage Stage) ([]repository.Transaction, error) {
	allTxs, err := w.repo.GetTransactionsByDeployment(ctx, w.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("get transactions: %w", err)
	}

	var stageTxs []repository.Transaction
	for _, tx := range allTxs {
		if tx.Stage == stage.String() {
			stageTxs = append(stageTxs, tx)
		}
	}

	return stageTxs, nil
}

// MarkComplete marks the deployment as successfully completed with real on-chain transactions.
func (w *StateWriter) MarkComplete(ctx context.Context) error {
	stageStr := StageCompleted.String()
	if err := w.repo.UpdateDeploymentStatus(ctx, w.deploymentID, repository.StatusCompleted, &stageStr); err != nil {
		return fmt.Errorf("mark complete: %w", err)
	}

	if w.onUpdate != nil {
		w.onUpdate(w.deploymentID, stageStr)
	}

	return nil
}

// MarkSimulated marks the deployment as completed in simulation mode (no real contracts deployed).
func (w *StateWriter) MarkSimulated(ctx context.Context) error {
	stageStr := StageCompleted.String()
	if err := w.repo.UpdateDeploymentStatus(ctx, w.deploymentID, repository.StatusSimulated, &stageStr); err != nil {
		return fmt.Errorf("mark simulated: %w", err)
	}

	if w.onUpdate != nil {
		w.onUpdate(w.deploymentID, "simulated")
	}

	return nil
}

// MarkFailed marks the deployment as failed with an error message.
func (w *StateWriter) MarkFailed(ctx context.Context, errMsg string) error {
	if err := w.repo.SetDeploymentError(ctx, w.deploymentID, errMsg); err != nil {
		return fmt.Errorf("set error: %w", err)
	}

	if err := w.repo.UpdateDeploymentStatus(ctx, w.deploymentID, repository.StatusFailed, nil); err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}

	if w.onUpdate != nil {
		w.onUpdate(w.deploymentID, "failed")
	}

	return nil
}

// MarkPaused marks the deployment as paused (can be resumed later).
func (w *StateWriter) MarkPaused(ctx context.Context) error {
	if err := w.repo.UpdateDeploymentStatus(ctx, w.deploymentID, repository.StatusPaused, nil); err != nil {
		return fmt.Errorf("mark paused: %w", err)
	}

	if w.onUpdate != nil {
		w.onUpdate(w.deploymentID, "paused")
	}

	return nil
}

// ClearError clears the error message from a deployment.
// This should be called when resuming a failed deployment.
func (w *StateWriter) ClearError(ctx context.Context) error {
	if err := w.repo.ClearDeploymentError(ctx, w.deploymentID); err != nil {
		return fmt.Errorf("clear error: %w", err)
	}
	return nil
}

// CanResume checks if the deployment can be resumed from a previous state.
func (w *StateWriter) CanResume(ctx context.Context) (bool, error) {
	deployment, err := w.repo.GetDeployment(ctx, w.deploymentID)
	if err != nil {
		return false, fmt.Errorf("get deployment: %w", err)
	}
	if deployment == nil {
		return false, nil
	}

	// Can resume if paused, running (crashed), or failed
	switch deployment.Status {
	case repository.StatusPaused, repository.StatusRunning, repository.StatusFailed:
		return true, nil
	default:
		return false, nil
	}
}

// GetResumePoint returns the stage and state to resume from.
// Returns the current stage and any saved state.
func (w *StateWriter) GetResumePoint(ctx context.Context) (Stage, json.RawMessage, error) {
	stage, err := w.GetCurrentStage(ctx)
	if err != nil {
		return "", nil, err
	}

	state, err := w.ReadState(ctx)
	if err != nil {
		return stage, nil, err
	}

	return stage, state, nil
}

// SaveArtifact saves a deployment artifact (genesis.json, rollup.json, etc.).
func (w *StateWriter) SaveArtifact(ctx context.Context, artifactType string, content json.RawMessage) error {
	artifact := &repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: w.deploymentID,
		ArtifactType: artifactType,
		Content:      content,
		CreatedAt:    time.Now(),
	}

	if err := w.repo.SaveArtifact(ctx, artifact); err != nil {
		return fmt.Errorf("save artifact %s: %w", artifactType, err)
	}

	return nil
}

// GetArtifact retrieves a deployment artifact by type.
func (w *StateWriter) GetArtifact(ctx context.Context, artifactType string) (json.RawMessage, error) {
	artifact, err := w.repo.GetArtifact(ctx, w.deploymentID, artifactType)
	if err != nil {
		return nil, fmt.Errorf("get artifact %s: %w", artifactType, err)
	}
	if artifact == nil {
		return nil, nil
	}

	return artifact.Content, nil
}

// StageOrder defines the order of deployment stages for determining progress.
var StageOrder = []Stage{
	StageInit,
	StageSuperchain,
	StageImplementations,
	StageOPChain,
	StageAltDA,
	StageGenesis,
	StageStartBlock,
	StageCompleted,
}

// StageIndex returns the index of a stage in the deployment order.
// Returns -1 if stage not found.
func StageIndex(stage Stage) int {
	for i, s := range StageOrder {
		if s == stage {
			return i
		}
	}
	return -1
}

// IsStageComplete returns true if the given stage has been completed.
// A stage is complete if the current stage index is greater than the given stage.
func (w *StateWriter) IsStageComplete(ctx context.Context, stage Stage) (bool, error) {
	currentStage, err := w.GetCurrentStage(ctx)
	if err != nil {
		return false, err
	}

	currentIdx := StageIndex(currentStage)
	targetIdx := StageIndex(stage)

	if currentIdx == -1 || targetIdx == -1 {
		return false, nil
	}

	return currentIdx > targetIdx, nil
}

