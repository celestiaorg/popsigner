package nitro

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// Test fixtures
func testDeployConfig() *DeployConfig {
	return &DeployConfig{
		ChainID:          42170,
		ChainName:        "test-orbit-chain",
		ParentChainID:    42161,
		ParentChainRpc:   "https://arb1.arbitrum.io/rpc",
		Owner:            "0x742d35Cc6634C0532925a3b844Bc454b332",
		BatchPosters:     []string{"0x742d35Cc6634C0532925a3b844Bc454b332"},
		Validators:       []string{"0x742d35Cc6634C0532925a3b844Bc454b332"},
		StakeToken:       "0x0000000000000000000000000000000000000000",
		BaseStake:        "100000000000000000",
		DataAvailability: "celestia",
	}
}

func testDeployResult() *DeployResult {
	return &DeployResult{
		Success: true,
		CoreContracts: &CoreContracts{
			Rollup:                 "0x1234567890123456789012345678901234567890",
			Inbox:                  "0x2345678901234567890123456789012345678901",
			Outbox:                 "0x3456789012345678901234567890123456789012",
			Bridge:                 "0x4567890123456789012345678901234567890123",
			SequencerInbox:         "0x5678901234567890123456789012345678901234",
			RollupEventInbox:       "0x6789012345678901234567890123456789012345",
			ChallengeManager:       "0x7890123456789012345678901234567890123456",
			AdminProxy:             "0x8901234567890123456789012345678901234567",
			UpgradeExecutor:        "0x9012345678901234567890123456789012345678",
			ValidatorWalletCreator: "0x0123456789012345678901234567890123456789",
			NativeToken:            "0x0000000000000000000000000000000000000000",
			DeployedAtBlockNumber:  12345678,
		},
		TransactionHash: "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		BlockNumber:     12345678,
	}
}

// ============================================================================
// ChainInfo Tests
// ============================================================================

func TestGenerateChainInfo(t *testing.T) {
	t.Run("generates valid chain info", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		chainInfo, err := GenerateChainInfo(config, result)
		require.NoError(t, err)
		require.NotNil(t, chainInfo)

		// Should be an array with one entry
		assert.Len(t, *chainInfo, 1)

		entry := (*chainInfo)[0]
		assert.Equal(t, uint64(42170), entry.ChainID)
		assert.Equal(t, uint64(42161), entry.ParentChainID)
		assert.Equal(t, "test-orbit-chain", entry.ChainName)

		// Check rollup info
		assert.Equal(t, result.CoreContracts.Bridge, entry.Rollup.Bridge)
		assert.Equal(t, result.CoreContracts.Inbox, entry.Rollup.Inbox)
		assert.Equal(t, result.CoreContracts.SequencerInbox, entry.Rollup.SequencerInbox)
		assert.Equal(t, result.CoreContracts.Rollup, entry.Rollup.Rollup)
		assert.Equal(t, uint64(12345678), entry.Rollup.DeployedAt)
	})

	t.Run("includes chain config with arbitrum settings", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		chainInfo, err := GenerateChainInfo(config, result)
		require.NoError(t, err)

		entry := (*chainInfo)[0]
		assert.NotNil(t, entry.ChainConfig)

		// Check arbitrum settings
		arbitrum, ok := entry.ChainConfig["arbitrum"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, true, arbitrum["EnableArbOS"])
		assert.Equal(t, false, arbitrum["AllowDebugPrecompiles"])
		assert.Equal(t, config.Owner, arbitrum["InitialChainOwner"])
	})

	t.Run("sets DataAvailabilityCommittee true for celestia (default)", func(t *testing.T) {
		config := testDeployConfig()
		config.DataAvailability = "celestia"
		result := testDeployResult()

		chainInfo, err := GenerateChainInfo(config, result)
		require.NoError(t, err)

		entry := (*chainInfo)[0]
		arbitrum := entry.ChainConfig["arbitrum"].(map[string]interface{})
		assert.Equal(t, true, arbitrum["DataAvailabilityCommittee"])
	})

	t.Run("sets DataAvailabilityCommittee true for empty DA (defaults to celestia)", func(t *testing.T) {
		config := testDeployConfig()
		config.DataAvailability = "" // Empty defaults to celestia behavior
		result := testDeployResult()

		chainInfo, err := GenerateChainInfo(config, result)
		require.NoError(t, err)

		entry := (*chainInfo)[0]
		arbitrum := entry.ChainConfig["arbitrum"].(map[string]interface{})
		assert.Equal(t, true, arbitrum["DataAvailabilityCommittee"])
	})

	t.Run("sets DataAvailabilityCommittee false for rollup", func(t *testing.T) {
		config := testDeployConfig()
		config.DataAvailability = "rollup"
		result := testDeployResult()

		chainInfo, err := GenerateChainInfo(config, result)
		require.NoError(t, err)

		entry := (*chainInfo)[0]
		arbitrum := entry.ChainConfig["arbitrum"].(map[string]interface{})
		assert.Equal(t, false, arbitrum["DataAvailabilityCommittee"])
	})

	t.Run("fails without successful deployment", func(t *testing.T) {
		config := testDeployConfig()

		// Test with nil result
		_, err := GenerateChainInfo(config, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "successful deployment")

		// Test with failed result
		failedResult := &DeployResult{Success: false, Error: "deployment failed"}
		_, err = GenerateChainInfo(config, failedResult)
		assert.Error(t, err)
	})

	t.Run("fails without core contracts", func(t *testing.T) {
		config := testDeployConfig()
		result := &DeployResult{Success: true} // No core contracts

		_, err := GenerateChainInfo(config, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing core contracts")
	})

	t.Run("serializes to valid JSON", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		chainInfo, err := GenerateChainInfo(config, result)
		require.NoError(t, err)

		jsonBytes, err := json.MarshalIndent(chainInfo, "", "  ")
		require.NoError(t, err)

		// Verify it's valid JSON that can be unmarshaled
		var parsed []map[string]interface{}
		err = json.Unmarshal(jsonBytes, &parsed)
		require.NoError(t, err)
		assert.Len(t, parsed, 1)

		// Check hyphenated keys (Nitro expects these)
		assert.Contains(t, string(jsonBytes), `"chain-id"`)
		assert.Contains(t, string(jsonBytes), `"parent-chain-id"`)
		assert.Contains(t, string(jsonBytes), `"chain-name"`)
		assert.Contains(t, string(jsonBytes), `"sequencer-inbox"`)
	})
}

// ============================================================================
// NodeConfig Tests
// ============================================================================

func TestGenerateNodeConfig(t *testing.T) {
	t.Run("generates valid node config", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		nodeConfig, err := GenerateNodeConfig(config, result)
		require.NoError(t, err)
		require.NotNil(t, nodeConfig)

		assert.Equal(t, uint64(42170), nodeConfig.Chain.ID)
		assert.Equal(t, "/config/chain-info.json", nodeConfig.Chain.InfoFiles)
	})

	t.Run("includes HTTP config", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		nodeConfig, err := GenerateNodeConfig(config, result)
		require.NoError(t, err)

		assert.Equal(t, "0.0.0.0", nodeConfig.HTTP.Addr)
		assert.Equal(t, 8547, nodeConfig.HTTP.Port)
		assert.Equal(t, "*", nodeConfig.HTTP.VHosts)
		assert.Contains(t, nodeConfig.HTTP.API, "eth")
		assert.Contains(t, nodeConfig.HTTP.API, "arb")
	})

	t.Run("includes WS config", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		nodeConfig, err := GenerateNodeConfig(config, result)
		require.NoError(t, err)

		assert.Equal(t, "0.0.0.0", nodeConfig.WS.Addr)
		assert.Equal(t, 8548, nodeConfig.WS.Port)
		assert.Contains(t, nodeConfig.WS.API, "eth")
	})

	t.Run("includes sequencer config", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		nodeConfig, err := GenerateNodeConfig(config, result)
		require.NoError(t, err)

		assert.True(t, nodeConfig.Node.Sequencer.Enable)
	})

	t.Run("includes batch poster with POPSigner mTLS", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		nodeConfig, err := GenerateNodeConfig(config, result)
		require.NoError(t, err)

		assert.True(t, nodeConfig.Node.BatchPoster.Enable)
		signer := nodeConfig.Node.BatchPoster.DataPoster.ExternalSigner
		assert.Equal(t, "${POPSIGNER_MTLS_URL}", signer.URL)
		assert.Equal(t, "eth_signTransaction", signer.Method)
		assert.Equal(t, "/certs/client.crt", signer.ClientCert)
		assert.Equal(t, "/certs/client.key", signer.ClientPrivateKey)
	})

	t.Run("includes staker with POPSigner mTLS", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		nodeConfig, err := GenerateNodeConfig(config, result)
		require.NoError(t, err)

		assert.True(t, nodeConfig.Node.Staker.Enable)
		assert.Equal(t, "MakeNodes", nodeConfig.Node.Staker.Strategy)
		signer := nodeConfig.Node.Staker.DataPoster.ExternalSigner
		assert.Equal(t, "${POPSIGNER_MTLS_URL}", signer.URL)
	})

	t.Run("includes metrics config", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		nodeConfig, err := GenerateNodeConfig(config, result)
		require.NoError(t, err)

		assert.Equal(t, "0.0.0.0", nodeConfig.Metrics.Server.Addr)
		assert.Equal(t, 9642, nodeConfig.Metrics.Server.Port)
	})

	t.Run("includes Celestia DA config by default", func(t *testing.T) {
		config := testDeployConfig()
		config.DataAvailability = "celestia"
		result := testDeployResult()

		nodeConfig, err := GenerateNodeConfig(config, result)
		require.NoError(t, err)

		// POPSigner deployments always use Celestia DA by default
		require.NotNil(t, nodeConfig.Node.DataAvailability)
		assert.True(t, nodeConfig.Node.DataAvailability.Enable)
		assert.Equal(t, result.CoreContracts.SequencerInbox, nodeConfig.Node.DataAvailability.SequencerInboxAddr)
		require.NotNil(t, nodeConfig.Node.DataAvailability.Celestia)
		assert.True(t, nodeConfig.Node.DataAvailability.Celestia.Enable)
		assert.Equal(t, "${CELESTIA_RPC_URL}", nodeConfig.Node.DataAvailability.Celestia.ServerURL)
	})

	t.Run("includes Celestia DA config for empty DA field (default)", func(t *testing.T) {
		config := testDeployConfig()
		config.DataAvailability = "" // Empty defaults to celestia
		result := testDeployResult()

		nodeConfig, err := GenerateNodeConfig(config, result)
		require.NoError(t, err)

		require.NotNil(t, nodeConfig.Node.DataAvailability)
		assert.True(t, nodeConfig.Node.DataAvailability.Enable)
		require.NotNil(t, nodeConfig.Node.DataAvailability.Celestia)
	})

	t.Run("no DA config for rollup mode", func(t *testing.T) {
		config := testDeployConfig()
		config.DataAvailability = "rollup"
		result := testDeployResult()

		nodeConfig, err := GenerateNodeConfig(config, result)
		require.NoError(t, err)

		// Only rollup mode has no DA config
		assert.Nil(t, nodeConfig.Node.DataAvailability)
	})

	t.Run("uses template variables for user config", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		nodeConfig, err := GenerateNodeConfig(config, result)
		require.NoError(t, err)

		// Check that templated values are present
		assert.Equal(t, "${L1_RPC_URL}", nodeConfig.ParentChain.Connection.URL)
	})

	t.Run("serializes to valid JSON with hyphenated keys", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		nodeConfig, err := GenerateNodeConfig(config, result)
		require.NoError(t, err)

		jsonBytes, err := json.MarshalIndent(nodeConfig, "", "  ")
		require.NoError(t, err)

		// Verify hyphenated keys
		assert.Contains(t, string(jsonBytes), `"parent-chain"`)
		assert.Contains(t, string(jsonBytes), `"info-files"`)
		assert.Contains(t, string(jsonBytes), `"batch-poster"`)
		assert.Contains(t, string(jsonBytes), `"data-poster"`)
		assert.Contains(t, string(jsonBytes), `"external-signer"`)
		assert.Contains(t, string(jsonBytes), `"client-cert"`)
	})
}

func TestGenerateValidatorNodeConfig(t *testing.T) {
	t.Run("disables sequencer and batch poster", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		nodeConfig, err := GenerateValidatorNodeConfig(config, result)
		require.NoError(t, err)

		assert.False(t, nodeConfig.Node.Sequencer.Enable)
		assert.False(t, nodeConfig.Node.BatchPoster.Enable)
	})

	t.Run("adds forwarding target", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		nodeConfig, err := GenerateValidatorNodeConfig(config, result)
		require.NoError(t, err)

		require.NotNil(t, nodeConfig.Execution)
		assert.Equal(t, "${SEQUENCER_URL}", nodeConfig.Execution.ForwardingTarget)
	})

	t.Run("keeps staker enabled", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		nodeConfig, err := GenerateValidatorNodeConfig(config, result)
		require.NoError(t, err)

		assert.True(t, nodeConfig.Node.Staker.Enable)
	})
}

// ============================================================================
// CoreContracts Tests
// ============================================================================

func TestGenerateCoreContractsArtifact(t *testing.T) {
	t.Run("generates valid core contracts", func(t *testing.T) {
		result := testDeployResult()

		contracts, err := GenerateCoreContractsArtifact(result)
		require.NoError(t, err)
		require.NotNil(t, contracts)

		assert.Equal(t, result.CoreContracts.Rollup, contracts.Rollup)
		assert.Equal(t, result.CoreContracts.Inbox, contracts.Inbox)
		assert.Equal(t, result.CoreContracts.Outbox, contracts.Outbox)
		assert.Equal(t, result.CoreContracts.Bridge, contracts.Bridge)
		assert.Equal(t, result.CoreContracts.SequencerInbox, contracts.SequencerInbox)
		assert.Equal(t, result.CoreContracts.AdminProxy, contracts.AdminProxy)
		assert.Equal(t, uint64(12345678), contracts.DeployedAtBlockNumber)
		assert.Equal(t, result.TransactionHash, contracts.TransactionHash)
	})

	t.Run("includes optional fields", func(t *testing.T) {
		result := testDeployResult()

		contracts, err := GenerateCoreContractsArtifact(result)
		require.NoError(t, err)

		assert.Equal(t, result.CoreContracts.ChallengeManager, contracts.ChallengeManager)
		assert.Equal(t, result.CoreContracts.UpgradeExecutor, contracts.UpgradeExecutor)
		assert.Equal(t, result.CoreContracts.ValidatorWalletCreator, contracts.ValidatorWalletCreator)
		assert.Equal(t, result.CoreContracts.NativeToken, contracts.NativeToken)
	})

	t.Run("fails without successful deployment", func(t *testing.T) {
		_, err := GenerateCoreContractsArtifact(nil)
		assert.Error(t, err)

		failedResult := &DeployResult{Success: false}
		_, err = GenerateCoreContractsArtifact(failedResult)
		assert.Error(t, err)
	})

	t.Run("fails without core contracts", func(t *testing.T) {
		result := &DeployResult{Success: true}

		_, err := GenerateCoreContractsArtifact(result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing core contracts")
	})

	t.Run("serializes to valid JSON", func(t *testing.T) {
		result := testDeployResult()

		contracts, err := GenerateCoreContractsArtifact(result)
		require.NoError(t, err)

		jsonBytes, err := json.MarshalIndent(contracts, "", "  ")
		require.NoError(t, err)

		// Check camelCase keys
		assert.Contains(t, string(jsonBytes), `"rollup"`)
		assert.Contains(t, string(jsonBytes), `"sequencerInbox"`)
		assert.Contains(t, string(jsonBytes), `"deployedAtBlockNumber"`)
		assert.Contains(t, string(jsonBytes), `"transactionHash"`)
	})
}

// ============================================================================
// GenerateAllArtifacts Tests
// ============================================================================

func TestGenerateAllArtifacts(t *testing.T) {
	t.Run("generates all three artifacts", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		artifacts, err := GenerateAllArtifacts(config, result)
		require.NoError(t, err)

		assert.Len(t, artifacts, 3)
		assert.Contains(t, artifacts, "chain-info.json")
		assert.Contains(t, artifacts, "node-config.json")
		assert.Contains(t, artifacts, "core-contracts.json")
	})

	t.Run("all artifacts are valid JSON", func(t *testing.T) {
		config := testDeployConfig()
		result := testDeployResult()

		artifacts, err := GenerateAllArtifacts(config, result)
		require.NoError(t, err)

		for name, content := range artifacts {
			var parsed interface{}
			err := json.Unmarshal(content, &parsed)
			assert.NoError(t, err, "artifact %s should be valid JSON", name)
		}
	})
}

// ============================================================================
// ArtifactGenerator Tests
// ============================================================================

func TestArtifactGenerator(t *testing.T) {
	t.Run("saves all artifacts to repository", func(t *testing.T) {
		mockRepo := new(MockRepository)
		generator := NewArtifactGenerator(mockRepo)

		ctx := context.Background()
		deploymentID := uuid.New()
		config := testDeployConfig()
		result := testDeployResult()

		// Expect SaveArtifact to be called 3 times
		mockRepo.On("SaveArtifact", ctx, mock.AnythingOfType("*repository.Artifact")).Return(nil).Times(3)

		err := generator.GenerateArtifacts(ctx, deploymentID, config, result)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("fails on nil result", func(t *testing.T) {
		mockRepo := new(MockRepository)
		generator := NewArtifactGenerator(mockRepo)

		ctx := context.Background()
		deploymentID := uuid.New()
		config := testDeployConfig()

		err := generator.GenerateArtifacts(ctx, deploymentID, config, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "successful deployment")
	})

	t.Run("fails on failed result", func(t *testing.T) {
		mockRepo := new(MockRepository)
		generator := NewArtifactGenerator(mockRepo)

		ctx := context.Background()
		deploymentID := uuid.New()
		config := testDeployConfig()
		result := &DeployResult{Success: false, Error: "deployment failed"}

		err := generator.GenerateArtifacts(ctx, deploymentID, config, result)
		assert.Error(t, err)
	})

	t.Run("saves artifacts with correct types", func(t *testing.T) {
		mockRepo := new(MockRepository)
		generator := NewArtifactGenerator(mockRepo)

		ctx := context.Background()
		deploymentID := uuid.New()
		config := testDeployConfig()
		result := testDeployResult()

		savedTypes := []string{}
		mockRepo.On("SaveArtifact", ctx, mock.AnythingOfType("*repository.Artifact")).
			Run(func(args mock.Arguments) {
				artifact := args.Get(1).(*repository.Artifact)
				savedTypes = append(savedTypes, artifact.ArtifactType)
				assert.Equal(t, deploymentID, artifact.DeploymentID)
			}).
			Return(nil)

		err := generator.GenerateArtifacts(ctx, deploymentID, config, result)
		assert.NoError(t, err)

		assert.Contains(t, savedTypes, "chain_info")
		assert.Contains(t, savedTypes, "node_config")
		assert.Contains(t, savedTypes, "core_contracts")
	})
}

