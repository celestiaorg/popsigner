// Package opstack provides OP Stack chain deployment infrastructure.
package opstack

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-chain-ops/addresses"
	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
)

// CelestiaDACommitmentType is the commitment type for Celestia DA.
// Celestia uses GenericCommitment as it handles data availability externally.
const CelestiaDACommitmentType = "GenericCommitment"

// BuildIntent creates an op-deployer Intent from our DeploymentConfig.
// The Intent is the primary input to the op-deployer pipeline stages.
func BuildIntent(cfg *DeploymentConfig) (*state.Intent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	cfg.ApplyDefaults()

	deployerAddr := common.HexToAddress(cfg.DeployerAddress)

	// Convert chain ID to common.Hash (op-deployer uses Hash for chain IDs)
	chainIDHash := common.BigToHash(new(big.Int).SetUint64(cfg.ChainID))

	// Build chain intent with Celestia DA configuration
	chainIntent := &state.ChainIntent{
		ID:                         chainIDHash,
		BaseFeeVaultRecipient:      parseAddressOrDefault(cfg.BaseFeeVaultRecipient, deployerAddr),
		L1FeeVaultRecipient:        parseAddressOrDefault(cfg.L1FeeVaultRecipient, deployerAddr),
		SequencerFeeVaultRecipient: parseAddressOrDefault(cfg.SequencerFeeVaultRecipient, deployerAddr),
		OperatorFeeVaultRecipient:  deployerAddr, // Not exposed in our config yet
		GasLimit:                   cfg.GasLimit,
		Eip1559Denominator:         50,    // Standard values
		Eip1559DenominatorCanyon:   250,   // Standard values
		Eip1559Elasticity:          6,     // Standard values
		Roles:                      buildChainRoles(cfg, deployerAddr),
		// Celestia DA configuration - POPKins only supports Celestia
		DangerousAltDAConfig: genesis.AltDADeployConfig{
			UseAltDA:         true,
			DACommitmentType: CelestiaDACommitmentType,
			// GenericCommitment (Celestia) doesn't require challenge/resolve windows
			// as data availability is guaranteed by Celestia's consensus
			DAChallengeWindow:          0,
			DAResolveWindow:            0,
			DABondSize:                 0,
			DAResolverRefundPercentage: 0,
		},
	}

	// Build superchain roles - all default to deployer
	superchainRoles := &addresses.SuperchainRoles{
		SuperchainProxyAdminOwner: deployerAddr,
		ProtocolVersionsOwner:     deployerAddr,
		SuperchainGuardian:        deployerAddr,
		Challenger:                parseAddressOrDefault(cfg.ChallengerAddress, deployerAddr),
	}

	intent := &state.Intent{
		ConfigType:         state.IntentTypeCustom,
		L1ChainID:          cfg.L1ChainID,
		FundDevAccounts:    false, // Production deployments don't fund dev accounts
		SuperchainRoles:    superchainRoles,
		L1ContractsLocator: artifacts.DefaultL1ContractsLocator,
		L2ContractsLocator: artifacts.DefaultL2ContractsLocator,
		Chains:             []*state.ChainIntent{chainIntent},
	}

	return intent, nil
}

// buildChainRoles creates ChainRoles from our config.
func buildChainRoles(cfg *DeploymentConfig, deployerAddr common.Address) state.ChainRoles {
	return state.ChainRoles{
		L1ProxyAdminOwner: deployerAddr,
		L2ProxyAdminOwner: deployerAddr,
		SystemConfigOwner: deployerAddr,
		UnsafeBlockSigner: parseAddressOrDefault(cfg.SequencerAddress, deployerAddr),
		Batcher:           parseAddressOrDefault(cfg.BatcherAddress, deployerAddr),
		Proposer:          parseAddressOrDefault(cfg.ProposerAddress, deployerAddr),
		Challenger:        parseAddressOrDefault(cfg.ChallengerAddress, deployerAddr),
	}
}

// parseAddressOrDefault parses an address string, returning the default if empty.
func parseAddressOrDefault(addr string, defaultAddr common.Address) common.Address {
	if addr == "" {
		return defaultAddr
	}
	return common.HexToAddress(addr)
}

