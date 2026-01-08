// Package repository provides data access layer implementations.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OPStackInfrastructure represents deployed OP Stack infrastructure on an L1 chain.
// This includes the OPCM (OP Contracts Manager) and related contracts that can be
// reused across multiple L2 chain deployments.
type OPStackInfrastructure struct {
	L1ChainID int64

	// Core OPCM addresses
	OPCMProxyAddress string
	OPCMImplAddress  string

	// Superchain contracts (shared across all chains on this L1)
	SuperchainConfigProxyAddress string
	ProtocolVersionsProxyAddress string

	// Implementation addresses
	OPCMContainerImplAddress          *string
	OPCMDeployerImplAddress           *string
	DelayedWETHImplAddress            *string
	OptimismPortalImplAddress         *string
	SystemConfigImplAddress           *string
	L1CrossDomainMessengerImplAddress *string
	L1StandardBridgeImplAddress       *string
	DisputeGameFactoryImplAddress     *string

	// Metadata
	Version          string
	Create2Salt      string
	DeployedAt       time.Time
	DeployedBy       *uuid.UUID
	DeploymentTxHash *string

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OPStackInfrastructureRepository defines the interface for OP Stack infrastructure data operations.
type OPStackInfrastructureRepository interface {
	// Get retrieves infrastructure for an L1 chain.
	Get(ctx context.Context, l1ChainID int64) (*OPStackInfrastructure, error)
	// GetByVersion retrieves infrastructure for an L1 chain with a specific version.
	GetByVersion(ctx context.Context, l1ChainID int64, version string) (*OPStackInfrastructure, error)
	// Create inserts a new infrastructure record.
	Create(ctx context.Context, infra *OPStackInfrastructure) error
	// Update updates an existing infrastructure record.
	Update(ctx context.Context, infra *OPStackInfrastructure) error
	// Upsert creates or updates infrastructure.
	Upsert(ctx context.Context, infra *OPStackInfrastructure) error
	// List returns all infrastructure records.
	List(ctx context.Context) ([]*OPStackInfrastructure, error)
	// Delete removes infrastructure for an L1 chain.
	Delete(ctx context.Context, l1ChainID int64) error
}

type opStackInfrastructureRepo struct {
	pool *pgxpool.Pool
}

// NewOPStackInfrastructureRepository creates a new OP Stack infrastructure repository.
func NewOPStackInfrastructureRepository(pool *pgxpool.Pool) OPStackInfrastructureRepository {
	return &opStackInfrastructureRepo{pool: pool}
}

// Get retrieves infrastructure for an L1 chain.
func (r *opStackInfrastructureRepo) Get(ctx context.Context, l1ChainID int64) (*OPStackInfrastructure, error) {
	query := `
		SELECT l1_chain_id, opcm_proxy_address, opcm_impl_address,
		       superchain_config_proxy_address, protocol_versions_proxy_address,
		       opcm_container_impl_address, opcm_deployer_impl_address,
		       delayed_weth_impl_address, optimism_portal_impl_address,
		       system_config_impl_address, l1_cross_domain_messenger_impl_address,
		       l1_standard_bridge_impl_address, dispute_game_factory_impl_address,
		       version, create2_salt, deployed_at, deployed_by, deployment_tx_hash,
		       created_at, updated_at
		FROM opstack_infrastructure
		WHERE l1_chain_id = $1`

	var infra OPStackInfrastructure
	err := r.pool.QueryRow(ctx, query, l1ChainID).Scan(
		&infra.L1ChainID,
		&infra.OPCMProxyAddress,
		&infra.OPCMImplAddress,
		&infra.SuperchainConfigProxyAddress,
		&infra.ProtocolVersionsProxyAddress,
		&infra.OPCMContainerImplAddress,
		&infra.OPCMDeployerImplAddress,
		&infra.DelayedWETHImplAddress,
		&infra.OptimismPortalImplAddress,
		&infra.SystemConfigImplAddress,
		&infra.L1CrossDomainMessengerImplAddress,
		&infra.L1StandardBridgeImplAddress,
		&infra.DisputeGameFactoryImplAddress,
		&infra.Version,
		&infra.Create2Salt,
		&infra.DeployedAt,
		&infra.DeployedBy,
		&infra.DeploymentTxHash,
		&infra.CreatedAt,
		&infra.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &infra, nil
}

// GetByVersion retrieves infrastructure for an L1 chain with a specific version.
func (r *opStackInfrastructureRepo) GetByVersion(ctx context.Context, l1ChainID int64, version string) (*OPStackInfrastructure, error) {
	query := `
		SELECT l1_chain_id, opcm_proxy_address, opcm_impl_address,
		       superchain_config_proxy_address, protocol_versions_proxy_address,
		       opcm_container_impl_address, opcm_deployer_impl_address,
		       delayed_weth_impl_address, optimism_portal_impl_address,
		       system_config_impl_address, l1_cross_domain_messenger_impl_address,
		       l1_standard_bridge_impl_address, dispute_game_factory_impl_address,
		       version, create2_salt, deployed_at, deployed_by, deployment_tx_hash,
		       created_at, updated_at
		FROM opstack_infrastructure
		WHERE l1_chain_id = $1 AND version = $2`

	var infra OPStackInfrastructure
	err := r.pool.QueryRow(ctx, query, l1ChainID, version).Scan(
		&infra.L1ChainID,
		&infra.OPCMProxyAddress,
		&infra.OPCMImplAddress,
		&infra.SuperchainConfigProxyAddress,
		&infra.ProtocolVersionsProxyAddress,
		&infra.OPCMContainerImplAddress,
		&infra.OPCMDeployerImplAddress,
		&infra.DelayedWETHImplAddress,
		&infra.OptimismPortalImplAddress,
		&infra.SystemConfigImplAddress,
		&infra.L1CrossDomainMessengerImplAddress,
		&infra.L1StandardBridgeImplAddress,
		&infra.DisputeGameFactoryImplAddress,
		&infra.Version,
		&infra.Create2Salt,
		&infra.DeployedAt,
		&infra.DeployedBy,
		&infra.DeploymentTxHash,
		&infra.CreatedAt,
		&infra.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &infra, nil
}

// Create inserts a new infrastructure record.
func (r *opStackInfrastructureRepo) Create(ctx context.Context, infra *OPStackInfrastructure) error {
	query := `
		INSERT INTO opstack_infrastructure (
			l1_chain_id, opcm_proxy_address, opcm_impl_address,
			superchain_config_proxy_address, protocol_versions_proxy_address,
			opcm_container_impl_address, opcm_deployer_impl_address,
			delayed_weth_impl_address, optimism_portal_impl_address,
			system_config_impl_address, l1_cross_domain_messenger_impl_address,
			l1_standard_bridge_impl_address, dispute_game_factory_impl_address,
			version, create2_salt, deployed_at, deployed_by, deployment_tx_hash
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING created_at, updated_at`

	if infra.DeployedAt.IsZero() {
		infra.DeployedAt = time.Now()
	}

	return r.pool.QueryRow(ctx, query,
		infra.L1ChainID,
		infra.OPCMProxyAddress,
		infra.OPCMImplAddress,
		infra.SuperchainConfigProxyAddress,
		infra.ProtocolVersionsProxyAddress,
		infra.OPCMContainerImplAddress,
		infra.OPCMDeployerImplAddress,
		infra.DelayedWETHImplAddress,
		infra.OptimismPortalImplAddress,
		infra.SystemConfigImplAddress,
		infra.L1CrossDomainMessengerImplAddress,
		infra.L1StandardBridgeImplAddress,
		infra.DisputeGameFactoryImplAddress,
		infra.Version,
		infra.Create2Salt,
		infra.DeployedAt,
		infra.DeployedBy,
		infra.DeploymentTxHash,
	).Scan(&infra.CreatedAt, &infra.UpdatedAt)
}

// Update updates an existing infrastructure record.
func (r *opStackInfrastructureRepo) Update(ctx context.Context, infra *OPStackInfrastructure) error {
	query := `
		UPDATE opstack_infrastructure
		SET opcm_proxy_address = $2,
		    opcm_impl_address = $3,
		    superchain_config_proxy_address = $4,
		    protocol_versions_proxy_address = $5,
		    opcm_container_impl_address = $6,
		    opcm_deployer_impl_address = $7,
		    delayed_weth_impl_address = $8,
		    optimism_portal_impl_address = $9,
		    system_config_impl_address = $10,
		    l1_cross_domain_messenger_impl_address = $11,
		    l1_standard_bridge_impl_address = $12,
		    dispute_game_factory_impl_address = $13,
		    version = $14,
		    create2_salt = $15,
		    deployed_at = $16,
		    deployed_by = $17,
		    deployment_tx_hash = $18,
		    updated_at = NOW()
		WHERE l1_chain_id = $1
		RETURNING updated_at`

	err := r.pool.QueryRow(ctx, query,
		infra.L1ChainID,
		infra.OPCMProxyAddress,
		infra.OPCMImplAddress,
		infra.SuperchainConfigProxyAddress,
		infra.ProtocolVersionsProxyAddress,
		infra.OPCMContainerImplAddress,
		infra.OPCMDeployerImplAddress,
		infra.DelayedWETHImplAddress,
		infra.OptimismPortalImplAddress,
		infra.SystemConfigImplAddress,
		infra.L1CrossDomainMessengerImplAddress,
		infra.L1StandardBridgeImplAddress,
		infra.DisputeGameFactoryImplAddress,
		infra.Version,
		infra.Create2Salt,
		infra.DeployedAt,
		infra.DeployedBy,
		infra.DeploymentTxHash,
	).Scan(&infra.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return pgx.ErrNoRows
	}
	return err
}

// Upsert creates or updates infrastructure.
func (r *opStackInfrastructureRepo) Upsert(ctx context.Context, infra *OPStackInfrastructure) error {
	query := `
		INSERT INTO opstack_infrastructure (
			l1_chain_id, opcm_proxy_address, opcm_impl_address,
			superchain_config_proxy_address, protocol_versions_proxy_address,
			opcm_container_impl_address, opcm_deployer_impl_address,
			delayed_weth_impl_address, optimism_portal_impl_address,
			system_config_impl_address, l1_cross_domain_messenger_impl_address,
			l1_standard_bridge_impl_address, dispute_game_factory_impl_address,
			version, create2_salt, deployed_at, deployed_by, deployment_tx_hash
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (l1_chain_id) DO UPDATE SET
			opcm_proxy_address = EXCLUDED.opcm_proxy_address,
			opcm_impl_address = EXCLUDED.opcm_impl_address,
			superchain_config_proxy_address = EXCLUDED.superchain_config_proxy_address,
			protocol_versions_proxy_address = EXCLUDED.protocol_versions_proxy_address,
			opcm_container_impl_address = EXCLUDED.opcm_container_impl_address,
			opcm_deployer_impl_address = EXCLUDED.opcm_deployer_impl_address,
			delayed_weth_impl_address = EXCLUDED.delayed_weth_impl_address,
			optimism_portal_impl_address = EXCLUDED.optimism_portal_impl_address,
			system_config_impl_address = EXCLUDED.system_config_impl_address,
			l1_cross_domain_messenger_impl_address = EXCLUDED.l1_cross_domain_messenger_impl_address,
			l1_standard_bridge_impl_address = EXCLUDED.l1_standard_bridge_impl_address,
			dispute_game_factory_impl_address = EXCLUDED.dispute_game_factory_impl_address,
			version = EXCLUDED.version,
			create2_salt = EXCLUDED.create2_salt,
			deployed_at = EXCLUDED.deployed_at,
			deployed_by = EXCLUDED.deployed_by,
			deployment_tx_hash = EXCLUDED.deployment_tx_hash,
			updated_at = NOW()
		RETURNING created_at, updated_at`

	if infra.DeployedAt.IsZero() {
		infra.DeployedAt = time.Now()
	}

	return r.pool.QueryRow(ctx, query,
		infra.L1ChainID,
		infra.OPCMProxyAddress,
		infra.OPCMImplAddress,
		infra.SuperchainConfigProxyAddress,
		infra.ProtocolVersionsProxyAddress,
		infra.OPCMContainerImplAddress,
		infra.OPCMDeployerImplAddress,
		infra.DelayedWETHImplAddress,
		infra.OptimismPortalImplAddress,
		infra.SystemConfigImplAddress,
		infra.L1CrossDomainMessengerImplAddress,
		infra.L1StandardBridgeImplAddress,
		infra.DisputeGameFactoryImplAddress,
		infra.Version,
		infra.Create2Salt,
		infra.DeployedAt,
		infra.DeployedBy,
		infra.DeploymentTxHash,
	).Scan(&infra.CreatedAt, &infra.UpdatedAt)
}

// List returns all infrastructure records.
func (r *opStackInfrastructureRepo) List(ctx context.Context) ([]*OPStackInfrastructure, error) {
	query := `
		SELECT l1_chain_id, opcm_proxy_address, opcm_impl_address,
		       superchain_config_proxy_address, protocol_versions_proxy_address,
		       opcm_container_impl_address, opcm_deployer_impl_address,
		       delayed_weth_impl_address, optimism_portal_impl_address,
		       system_config_impl_address, l1_cross_domain_messenger_impl_address,
		       l1_standard_bridge_impl_address, dispute_game_factory_impl_address,
		       version, create2_salt, deployed_at, deployed_by, deployment_tx_hash,
		       created_at, updated_at
		FROM opstack_infrastructure
		ORDER BY l1_chain_id`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var infraList []*OPStackInfrastructure
	for rows.Next() {
		var infra OPStackInfrastructure
		if err := rows.Scan(
			&infra.L1ChainID,
			&infra.OPCMProxyAddress,
			&infra.OPCMImplAddress,
			&infra.SuperchainConfigProxyAddress,
			&infra.ProtocolVersionsProxyAddress,
			&infra.OPCMContainerImplAddress,
			&infra.OPCMDeployerImplAddress,
			&infra.DelayedWETHImplAddress,
			&infra.OptimismPortalImplAddress,
			&infra.SystemConfigImplAddress,
			&infra.L1CrossDomainMessengerImplAddress,
			&infra.L1StandardBridgeImplAddress,
			&infra.DisputeGameFactoryImplAddress,
			&infra.Version,
			&infra.Create2Salt,
			&infra.DeployedAt,
			&infra.DeployedBy,
			&infra.DeploymentTxHash,
			&infra.CreatedAt,
			&infra.UpdatedAt,
		); err != nil {
			return nil, err
		}
		infraList = append(infraList, &infra)
	}
	return infraList, rows.Err()
}

// Delete removes infrastructure for an L1 chain.
func (r *opStackInfrastructureRepo) Delete(ctx context.Context, l1ChainID int64) error {
	query := `DELETE FROM opstack_infrastructure WHERE l1_chain_id = $1`
	result, err := r.pool.Exec(ctx, query, l1ChainID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// Compile-time check to ensure opStackInfrastructureRepo implements OPStackInfrastructureRepository.
var _ OPStackInfrastructureRepository = (*opStackInfrastructureRepo)(nil)
