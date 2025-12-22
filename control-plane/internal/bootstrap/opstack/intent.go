// Package opstack provides OP Stack chain deployment infrastructure.
package opstack

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-chain-ops/addresses"
	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
)

// CelestiaDACommitmentType is the commitment type for Celestia DA.
// Celestia uses GenericCommitment as it handles data availability externally.
const CelestiaDACommitmentType = "GenericCommitment"

// chainNameRegex validates chain names: 2-64 chars, alphanumeric + hyphens, no leading/trailing hyphens
var chainNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,62}[a-zA-Z0-9]$|^[a-zA-Z0-9]{1,2}$`)

// reservedChainIDs contains L1 chain IDs that cannot be used as L2 chain IDs
var reservedChainIDs = map[uint64]string{
	1:        "Ethereum Mainnet",
	11155111: "Sepolia",
	17000:    "Holesky",
}

// ValidateChainName validates the chain name format.
func ValidateChainName(name string) error {
	if len(name) < 2 || len(name) > 64 {
		return errors.New("chain name must be 2-64 characters")
	}
	if !chainNameRegex.MatchString(name) {
		return errors.New("chain name must contain only letters, numbers, and hyphens (no leading/trailing hyphens)")
	}
	return nil
}

// ValidateChainID validates that the chain ID is not reserved.
func ValidateChainID(chainID uint64) error {
	if chainID == 0 {
		return errors.New("chain ID cannot be zero")
	}
	if network, reserved := reservedChainIDs[chainID]; reserved {
		return fmt.Errorf("chain ID %d is reserved for %s", chainID, network)
	}
	return nil
}

// GenerateDeploymentSalt creates a unique, deterministic salt for isolated OP chain deployment.
// Each deployment gets its own OPCM, blueprints, and infrastructure.
// Same inputs always produce same salt (idempotent deployments).
// Format: hash("isolated/<chainName>/<chainID>/<artifactVersion>")
func GenerateDeploymentSalt(chainName string, chainID uint64, artifactVersion string) common.Hash {
	saltInput := fmt.Sprintf("isolated/%s/%d/%s", chainName, chainID, artifactVersion)
	return crypto.Keccak256Hash([]byte(saltInput))
}

// BuildIntent creates an op-deployer Intent from our DeploymentConfig.
// The Intent is the primary input to the op-deployer pipeline stages.
//
// IMPORTANT: Each deployment gets ISOLATED infrastructure (own OPCM, blueprints, etc.)
// using a unique CREATE2 salt based on chainName + chainID + artifactVersion.
// This prevents reusing contracts from previous deployments.
func BuildIntent(cfg *DeploymentConfig) (*state.Intent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Additional validation for isolated deployments
	if err := ValidateChainName(cfg.ChainName); err != nil {
		return nil, fmt.Errorf("invalid chain name: %w", err)
	}
	if err := ValidateChainID(cfg.ChainID); err != nil {
		return nil, fmt.Errorf("invalid chain ID: %w", err)
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

	// Create ISOLATED intent - each chain gets its own infrastructure
	// - OPCMAddress is nil = deploy fresh OPCM (don't reuse existing)
	// - The caller must set Create2Salt on state.State (not Intent)
	intent := &state.Intent{
		ConfigType:      state.IntentTypeCustom,
		L1ChainID:       cfg.L1ChainID,
		FundDevAccounts: false, // Production deployments don't fund dev accounts
		SuperchainRoles: superchainRoles,
		// OPCMAddress: nil = deploy new OPCM (CRITICAL for isolation!)
		// L1ContractsLocator and L2ContractsLocator set by deployer
		Chains: []*state.ChainIntent{chainIntent},
	}

	return intent, nil
}

// GetDeploymentSalt returns the salt that will be used for a deployment.
// Useful for logging and debugging.
func GetDeploymentSalt(chainName string, chainID uint64) common.Hash {
	return GenerateDeploymentSalt(chainName, chainID, ArtifactVersion)
}

// BuildState creates an initial state.State with the deployment salt set.
// The salt ensures isolated contract deployments per chain.
func BuildState(chainName string, chainID uint64) *state.State {
	salt := GenerateDeploymentSalt(chainName, chainID, ArtifactVersion)
	return &state.State{
		Version:     1,
		Create2Salt: salt,
	}
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

