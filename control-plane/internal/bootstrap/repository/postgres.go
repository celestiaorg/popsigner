package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// PostgresRepository implements Repository using PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// CreateDeployment inserts a new deployment record.
func (r *PostgresRepository) CreateDeployment(ctx context.Context, d *Deployment) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}

	query := `
		INSERT INTO deployments (id, chain_id, stack, status, current_stage, config, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at`

	err := r.pool.QueryRow(ctx, query,
		d.ID, d.ChainID, d.Stack, d.Status, d.CurrentStage, d.Config, d.ErrorMessage,
	).Scan(&d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("CreateDeployment: %w", err)
	}
	return nil
}

// GetDeployment retrieves a deployment by its UUID.
func (r *PostgresRepository) GetDeployment(ctx context.Context, id uuid.UUID) (*Deployment, error) {
	query := `
		SELECT id, chain_id, stack, status, current_stage, config, error_message, created_at, updated_at
		FROM deployments
		WHERE id = $1`

	var d Deployment
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&d.ID, &d.ChainID, &d.Stack, &d.Status, &d.CurrentStage,
		&d.Config, &d.ErrorMessage, &d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetDeployment: %w", err)
	}
	return &d, nil
}

// GetDeploymentByChainID retrieves a deployment by chain ID.
func (r *PostgresRepository) GetDeploymentByChainID(ctx context.Context, chainID int64) (*Deployment, error) {
	query := `
		SELECT id, chain_id, stack, status, current_stage, config, error_message, created_at, updated_at
		FROM deployments
		WHERE chain_id = $1`

	var d Deployment
	err := r.pool.QueryRow(ctx, query, chainID).Scan(
		&d.ID, &d.ChainID, &d.Stack, &d.Status, &d.CurrentStage,
		&d.Config, &d.ErrorMessage, &d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetDeploymentByChainID: %w", err)
	}
	return &d, nil
}

// UpdateDeploymentStatus updates the status and current stage of a deployment.
func (r *PostgresRepository) UpdateDeploymentStatus(ctx context.Context, id uuid.UUID, status Status, stage *string) error {
	query := `
		UPDATE deployments
		SET status = $2, current_stage = $3, updated_at = NOW()
		WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id, status, stage)
	if err != nil {
		return fmt.Errorf("UpdateDeploymentStatus: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateDeploymentConfig updates the config JSON of a deployment.
func (r *PostgresRepository) UpdateDeploymentConfig(ctx context.Context, id uuid.UUID, config json.RawMessage) error {
	query := `
		UPDATE deployments
		SET config = $2, updated_at = NOW()
		WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id, config)
	if err != nil {
		return fmt.Errorf("UpdateDeploymentConfig: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetDeploymentError sets the error message and marks the deployment as failed.
func (r *PostgresRepository) SetDeploymentError(ctx context.Context, id uuid.UUID, errMsg string) error {
	query := `
		UPDATE deployments
		SET status = $2, error_message = $3, updated_at = NOW()
		WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id, StatusFailed, errMsg)
	if err != nil {
		return fmt.Errorf("SetDeploymentError: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListDeploymentsByStatus retrieves all deployments with the given status.
func (r *PostgresRepository) ListDeploymentsByStatus(ctx context.Context, status Status) ([]*Deployment, error) {
	query := `
		SELECT id, chain_id, stack, status, current_stage, config, error_message, created_at, updated_at
		FROM deployments
		WHERE status = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("ListDeploymentsByStatus: %w", err)
	}
	defer rows.Close()

	var deployments []*Deployment
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(
			&d.ID, &d.ChainID, &d.Stack, &d.Status, &d.CurrentStage,
			&d.Config, &d.ErrorMessage, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("ListDeploymentsByStatus scan: %w", err)
		}
		deployments = append(deployments, &d)
	}
	return deployments, rows.Err()
}

// ListAllDeployments retrieves all deployments ordered by creation date.
func (r *PostgresRepository) ListAllDeployments(ctx context.Context) ([]*Deployment, error) {
	query := `
		SELECT id, chain_id, stack, status, current_stage, config, error_message, created_at, updated_at
		FROM deployments
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ListAllDeployments: %w", err)
	}
	defer rows.Close()

	var deployments []*Deployment
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(
			&d.ID, &d.ChainID, &d.Stack, &d.Status, &d.CurrentStage,
			&d.Config, &d.ErrorMessage, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("ListAllDeployments scan: %w", err)
		}
		deployments = append(deployments, &d)
	}
	return deployments, rows.Err()
}

// RecordTransaction inserts a new transaction record.
func (r *PostgresRepository) RecordTransaction(ctx context.Context, tx *Transaction) error {
	if tx.ID == uuid.Nil {
		tx.ID = uuid.New()
	}

	query := `
		INSERT INTO deployment_transactions (id, deployment_id, stage, tx_hash, description)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at`

	err := r.pool.QueryRow(ctx, query,
		tx.ID, tx.DeploymentID, tx.Stage, tx.TxHash, tx.Description,
	).Scan(&tx.CreatedAt)
	if err != nil {
		return fmt.Errorf("RecordTransaction: %w", err)
	}
	return nil
}

// GetTransactionsByDeployment retrieves all transactions for a deployment.
func (r *PostgresRepository) GetTransactionsByDeployment(ctx context.Context, deploymentID uuid.UUID) ([]Transaction, error) {
	query := `
		SELECT id, deployment_id, stage, tx_hash, description, created_at
		FROM deployment_transactions
		WHERE deployment_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, query, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("GetTransactionsByDeployment: %w", err)
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(
			&tx.ID, &tx.DeploymentID, &tx.Stage, &tx.TxHash, &tx.Description, &tx.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("GetTransactionsByDeployment scan: %w", err)
		}
		transactions = append(transactions, tx)
	}
	return transactions, rows.Err()
}

// GetTransactionByHash retrieves a transaction by its hash.
func (r *PostgresRepository) GetTransactionByHash(ctx context.Context, hash string) (*Transaction, error) {
	query := `
		SELECT id, deployment_id, stage, tx_hash, description, created_at
		FROM deployment_transactions
		WHERE tx_hash = $1`

	var tx Transaction
	err := r.pool.QueryRow(ctx, query, hash).Scan(
		&tx.ID, &tx.DeploymentID, &tx.Stage, &tx.TxHash, &tx.Description, &tx.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetTransactionByHash: %w", err)
	}
	return &tx, nil
}

// SaveArtifact inserts or updates an artifact (upsert by deployment_id + artifact_type).
func (r *PostgresRepository) SaveArtifact(ctx context.Context, a *Artifact) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}

	query := `
		INSERT INTO deployment_artifacts (id, deployment_id, artifact_type, content)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (deployment_id, artifact_type)
		DO UPDATE SET content = EXCLUDED.content
		RETURNING created_at`

	err := r.pool.QueryRow(ctx, query,
		a.ID, a.DeploymentID, a.ArtifactType, a.Content,
	).Scan(&a.CreatedAt)
	if err != nil {
		return fmt.Errorf("SaveArtifact: %w", err)
	}
	return nil
}

// GetArtifact retrieves an artifact by deployment ID and type.
func (r *PostgresRepository) GetArtifact(ctx context.Context, deploymentID uuid.UUID, artifactType string) (*Artifact, error) {
	query := `
		SELECT id, deployment_id, artifact_type, content, created_at
		FROM deployment_artifacts
		WHERE deployment_id = $1 AND artifact_type = $2`

	var a Artifact
	err := r.pool.QueryRow(ctx, query, deploymentID, artifactType).Scan(
		&a.ID, &a.DeploymentID, &a.ArtifactType, &a.Content, &a.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetArtifact: %w", err)
	}
	return &a, nil
}

// GetAllArtifacts retrieves all artifacts for a deployment.
func (r *PostgresRepository) GetAllArtifacts(ctx context.Context, deploymentID uuid.UUID) ([]Artifact, error) {
	query := `
		SELECT id, deployment_id, artifact_type, content, created_at
		FROM deployment_artifacts
		WHERE deployment_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, query, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("GetAllArtifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []Artifact
	for rows.Next() {
		var a Artifact
		if err := rows.Scan(
			&a.ID, &a.DeploymentID, &a.ArtifactType, &a.Content, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("GetAllArtifacts scan: %w", err)
		}
		artifacts = append(artifacts, a)
	}
	return artifacts, rows.Err()
}

// MarkStaleDeploymentsFailed marks deployments that have been "running" for longer
// than the timeout as "failed". This handles cases where the deployment pod crashed
// without properly updating the status.
func (r *PostgresRepository) MarkStaleDeploymentsFailed(ctx context.Context, timeout time.Duration) (int, error) {
	// Find and update all running deployments that haven't been updated within the timeout
	query := `
		UPDATE deployments
		SET status = $1, 
		    error_message = $2,
		    updated_at = NOW()
		WHERE status = $3
		  AND updated_at < NOW() - $4::interval`

	errorMsg := "Deployment timed out - worker may have crashed. Click 'Resume Deployment' to retry."
	result, err := r.pool.Exec(ctx, query,
		StatusFailed,
		errorMsg,
		StatusRunning,
		timeout.String(),
	)
	if err != nil {
		return 0, fmt.Errorf("MarkStaleDeploymentsFailed: %w", err)
	}

	return int(result.RowsAffected()), nil
}

// Compile-time check to ensure PostgresRepository implements Repository.
var _ Repository = (*PostgresRepository)(nil)

