// Package nitro provides Nitro chain deployment infrastructure.
package nitro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// ============================================================================
// Deployment Configuration Types
// ============================================================================

// DeployConfig contains configuration for a Nitro chain deployment.
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
	ConfirmPeriodBlocks int  `json:"confirmPeriodBlocks,omitempty"`
	MaxDataSize         int  `json:"maxDataSize,omitempty"`
	DeployFactoriesToL2 bool `json:"deployFactoriesToL2,omitempty"`

	// POPSigner mTLS configuration
	PopsignerEndpoint string `json:"popsignerEndpoint"`
	ClientCert        string `json:"clientCert"`
	ClientKey         string `json:"clientKey"`
	CaCert            string `json:"caCert,omitempty"`
}

// ============================================================================
// Contract Addresses
// ============================================================================

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

// ============================================================================
// Deployment Result
// ============================================================================

// DeployResult is the result of a deployment operation.
type DeployResult struct {
	Success         bool                   `json:"success"`
	CoreContracts   *CoreContracts         `json:"coreContracts,omitempty"`
	TransactionHash string                 `json:"transactionHash,omitempty"`
	BlockNumber     int64                  `json:"blockNumber,omitempty"`
	ChainConfig     map[string]interface{} `json:"chainConfig,omitempty"`
	Error           string                 `json:"error,omitempty"`
}

// ============================================================================
// Callback Types
// ============================================================================

// ProgressCallback is called to report deployment progress.
type ProgressCallback func(stage string, progress float64, message string)

// ============================================================================
// Certificate Bundle
// ============================================================================

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

// WriteCertificatesToDir writes certificates to a directory for filesystem access.
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

// ============================================================================
// Helper Functions
// ============================================================================

// BuildConfigFromDeployment creates a DeployConfig from a stored deployment.
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

// ============================================================================
// Chain Info Types (chain-info.json)
// ============================================================================

// ChainInfo is the structure expected by Nitro's --chain.info-files flag.
// This is an array with a single chain configuration.
type ChainInfo []ChainInfoEntry

// ChainInfoEntry represents a single chain's info in the array.
type ChainInfoEntry struct {
	ChainID       uint64                 `json:"chain-id"`
	ParentChainID uint64                 `json:"parent-chain-id"`
	ChainName     string                 `json:"chain-name"`
	ChainConfig   map[string]interface{} `json:"chain-config"`
	Rollup        RollupInfo             `json:"rollup"`
}

// RollupInfo contains rollup contract addresses.
type RollupInfo struct {
	Bridge                 string `json:"bridge"`
	Inbox                  string `json:"inbox"`
	SequencerInbox         string `json:"sequencer-inbox"`
	Rollup                 string `json:"rollup"`
	ValidatorWalletCreator string `json:"validator-wallet-creator,omitempty"`
	DeployedAt             uint64 `json:"deployed-at"`
	StakeToken             string `json:"stake-token,omitempty"`
	NativeToken            string `json:"native-token,omitempty"`
}

// ============================================================================
// Core Contracts Artifact (core-contracts.json)
// ============================================================================

// CoreContractsArtifact is the formatted core contracts artifact for output.
type CoreContractsArtifact struct {
	Rollup                 string `json:"rollup"`
	Inbox                  string `json:"inbox"`
	Outbox                 string `json:"outbox"`
	Bridge                 string `json:"bridge"`
	SequencerInbox         string `json:"sequencerInbox"`
	RollupEventInbox       string `json:"rollupEventInbox,omitempty"`
	ChallengeManager       string `json:"challengeManager,omitempty"`
	AdminProxy             string `json:"adminProxy"`
	UpgradeExecutor        string `json:"upgradeExecutor,omitempty"`
	ValidatorWalletCreator string `json:"validatorWalletCreator,omitempty"`
	NativeToken            string `json:"nativeToken,omitempty"`
	DeployedAtBlockNumber  uint64 `json:"deployedAtBlockNumber"`
	TransactionHash        string `json:"transactionHash"`
}
