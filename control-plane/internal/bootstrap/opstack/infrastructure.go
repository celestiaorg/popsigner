// Package opstack provides OP Stack chain deployment infrastructure.
package opstack

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/repository"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
)

// InfrastructureManager handles OP Stack infrastructure reuse.
// It stores and retrieves deployed OPCM and superchain contracts to enable
// faster, cheaper subsequent L2 deployments on the same L1.
type InfrastructureManager struct {
	repo   repository.OPStackInfrastructureRepository
	logger *slog.Logger
}

// NewInfrastructureManager creates a new infrastructure manager.
func NewInfrastructureManager(
	repo repository.OPStackInfrastructureRepository,
	logger *slog.Logger,
) *InfrastructureManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &InfrastructureManager{
		repo:   repo,
		logger: logger,
	}
}

// InfrastructureInfo contains information about existing infrastructure.
type InfrastructureInfo struct {
	OPCMAddress                  common.Address
	SuperchainConfigProxyAddress common.Address
	ProtocolVersionsProxyAddress common.Address
	Version                      string
	Create2Salt                  common.Hash
}

// GetExistingInfrastructure retrieves existing infrastructure for an L1 chain.
// Returns nil if no infrastructure exists or if the version doesn't match.
func (m *InfrastructureManager) GetExistingInfrastructure(
	ctx context.Context,
	l1ChainID uint64,
	requiredVersion string,
) (*InfrastructureInfo, error) {
	if m.repo == nil {
		return nil, nil
	}

	infra, err := m.repo.GetByVersion(ctx, int64(l1ChainID), requiredVersion)
	if err != nil {
		return nil, fmt.Errorf("get infrastructure: %w", err)
	}
	if infra == nil {
		m.logger.Debug("no existing infrastructure found",
			slog.Uint64("l1_chain_id", l1ChainID),
			slog.String("version", requiredVersion),
		)
		return nil, nil
	}

	m.logger.Info("found existing infrastructure",
		slog.Uint64("l1_chain_id", l1ChainID),
		slog.String("opcm_address", infra.OPCMProxyAddress),
		slog.String("version", infra.Version),
	)

	return &InfrastructureInfo{
		OPCMAddress:                  common.HexToAddress(infra.OPCMProxyAddress),
		SuperchainConfigProxyAddress: common.HexToAddress(infra.SuperchainConfigProxyAddress),
		ProtocolVersionsProxyAddress: common.HexToAddress(infra.ProtocolVersionsProxyAddress),
		Version:                      infra.Version,
		Create2Salt:                  common.HexToHash(infra.Create2Salt),
	}, nil
}

// SaveInfrastructure saves deployed infrastructure for future reuse.
func (m *InfrastructureManager) SaveInfrastructure(
	ctx context.Context,
	l1ChainID uint64,
	deployResult *DeployResult,
	create2Salt common.Hash,
	deployedBy *uuid.UUID,
) error {
	if m.repo == nil {
		m.logger.Warn("no infrastructure repository configured, skipping save")
		return nil
	}

	if deployResult.State == nil {
		return fmt.Errorf("deploy result has no state")
	}

	st := deployResult.State

	// Extract addresses from deployment state
	infra := &repository.OPStackInfrastructure{
		L1ChainID:                    int64(l1ChainID),
		Version:                      ArtifactVersion,
		Create2Salt:                  create2Salt.Hex(),
		DeployedBy:                   deployedBy,
	}

	// Extract OPCM addresses from implementations deployment
	if impl := st.ImplementationsDeployment; impl != nil {
		infra.OPCMProxyAddress = impl.OpcmImpl.Hex() // The OPCM proxy
		infra.OPCMImplAddress = impl.OpcmImpl.Hex()

		// Optional implementation addresses
		if impl.OpcmContainerImpl != (common.Address{}) {
			addr := impl.OpcmContainerImpl.Hex()
			infra.OPCMContainerImplAddress = &addr
		}
		if impl.OpcmDeployerImpl != (common.Address{}) {
			addr := impl.OpcmDeployerImpl.Hex()
			infra.OPCMDeployerImplAddress = &addr
		}
		if impl.DelayedWethImpl != (common.Address{}) {
			addr := impl.DelayedWethImpl.Hex()
			infra.DelayedWETHImplAddress = &addr
		}
		if impl.OptimismPortalImpl != (common.Address{}) {
			addr := impl.OptimismPortalImpl.Hex()
			infra.OptimismPortalImplAddress = &addr
		}
		if impl.SystemConfigImpl != (common.Address{}) {
			addr := impl.SystemConfigImpl.Hex()
			infra.SystemConfigImplAddress = &addr
		}
		if impl.L1CrossDomainMessengerImpl != (common.Address{}) {
			addr := impl.L1CrossDomainMessengerImpl.Hex()
			infra.L1CrossDomainMessengerImplAddress = &addr
		}
		if impl.L1StandardBridgeImpl != (common.Address{}) {
			addr := impl.L1StandardBridgeImpl.Hex()
			infra.L1StandardBridgeImplAddress = &addr
		}
		if impl.DisputeGameFactoryImpl != (common.Address{}) {
			addr := impl.DisputeGameFactoryImpl.Hex()
			infra.DisputeGameFactoryImplAddress = &addr
		}
	} else {
		return fmt.Errorf("no implementations deployment in state")
	}

	// Extract superchain addresses
	if sc := st.SuperchainDeployment; sc != nil {
		infra.SuperchainConfigProxyAddress = sc.SuperchainConfigProxy.Hex()
		infra.ProtocolVersionsProxyAddress = sc.ProtocolVersionsProxy.Hex()
	} else {
		return fmt.Errorf("no superchain deployment in state")
	}

	// Save to database
	if err := m.repo.Upsert(ctx, infra); err != nil {
		return fmt.Errorf("save infrastructure: %w", err)
	}

	m.logger.Info("saved infrastructure for reuse",
		slog.Int64("l1_chain_id", infra.L1ChainID),
		slog.String("opcm_address", infra.OPCMProxyAddress),
		slog.String("version", infra.Version),
	)

	return nil
}

// PopulateConfigFromInfra populates deployment config with existing infrastructure addresses.
func (m *InfrastructureManager) PopulateConfigFromInfra(
	cfg *DeploymentConfig,
	infra *InfrastructureInfo,
) {
	if infra == nil {
		return
	}

	cfg.ExistingOPCMAddress = infra.OPCMAddress.Hex()
	cfg.ExistingSuperchainConfigAddress = infra.SuperchainConfigProxyAddress.Hex()
}

// PrepareStateForReuse prepares the state for an infrastructure reuse deployment.
// This sets up the state with the existing superchain and implementations from
// the saved infrastructure, allowing DeployOPChain to skip those stages.
func (m *InfrastructureManager) PrepareStateForReuse(
	ctx context.Context,
	l1ChainID uint64,
) (*state.State, error) {
	if m.repo == nil {
		return nil, fmt.Errorf("no infrastructure repository configured")
	}

	infra, err := m.repo.Get(ctx, int64(l1ChainID))
	if err != nil {
		return nil, fmt.Errorf("get infrastructure: %w", err)
	}
	if infra == nil {
		return nil, fmt.Errorf("no infrastructure found for L1 chain %d", l1ChainID)
	}

	// Create state with existing deployments
	st := &state.State{
		Version:     1,
		Create2Salt: common.HexToHash(infra.Create2Salt),
	}

	// Note: The caller should still call BuildIntent with ReuseInfrastructure=true
	// and the OPCMAddress will be set, which tells op-deployer to skip
	// DeploySuperchain and DeployImplementations stages.

	m.logger.Info("prepared state for infrastructure reuse",
		slog.Uint64("l1_chain_id", l1ChainID),
		slog.String("create2_salt", st.Create2Salt.Hex()),
	)

	return st, nil
}

// HasInfrastructure checks if infrastructure exists for an L1 chain.
func (m *InfrastructureManager) HasInfrastructure(ctx context.Context, l1ChainID uint64) (bool, error) {
	if m.repo == nil {
		return false, nil
	}

	infra, err := m.repo.Get(ctx, int64(l1ChainID))
	if err != nil {
		return false, err
	}
	return infra != nil, nil
}
