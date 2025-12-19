package opstack

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// MockL1Client implements L1Client for testing.
type MockL1Client struct {
	mock.Mock
}

func (m *MockL1Client) ChainID(ctx context.Context) (*big.Int, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*big.Int), args.Error(1)
}

func (m *MockL1Client) BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	args := m.Called(ctx, account, blockNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*big.Int), args.Error(1)
}

func (m *MockL1Client) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	args := m.Called(ctx, account, blockNumber)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockL1Client) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	args := m.Called(ctx, account)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockL1Client) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*big.Int), args.Error(1)
}

func (m *MockL1Client) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*big.Int), args.Error(1)
}

func (m *MockL1Client) EstimateGas(ctx context.Context, call ethereum.CallMsg) (uint64, error) {
	args := m.Called(ctx, call)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockL1Client) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}

func (m *MockL1Client) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	args := m.Called(ctx, txHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Receipt), args.Error(1)
}

func (m *MockL1Client) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	args := m.Called(ctx, number)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Header), args.Error(1)
}

func (m *MockL1Client) Close() {}

// MockL1ClientFactory implements L1ClientFactory for testing.
type MockL1ClientFactory struct {
	client *MockL1Client
}

func (f *MockL1ClientFactory) Dial(ctx context.Context, rpcURL string) (L1Client, error) {
	return f.client, nil
}

// MockSignerFactory implements SignerFactory for testing.
type MockSignerFactory struct{}

func (f *MockSignerFactory) CreateSigner(endpoint, apiKey string, chainID *big.Int) *POPSigner {
	return NewPOPSigner(SignerConfig{
		Endpoint: endpoint,
		APIKey:   apiKey,
		ChainID:  chainID,
	})
}

// createTestDeploymentConfig creates a valid test configuration.
// POPKins only supports Celestia as the DA layer.
func createTestDeploymentConfig() *DeploymentConfig {
	cfg := &DeploymentConfig{
		ChainID:           42069,
		ChainName:         "test-chain",
		L1ChainID:         11155111, // Sepolia
		L1RPC:             "http://localhost:8545",
		POPSignerEndpoint: "http://localhost:8080",
		POPSignerAPIKey:   "test-api-key",
		DeployerAddress:   "0x1234567890123456789012345678901234567890",
		CelestiaRPC:       "http://localhost:26658", // Required for Celestia DA
	}
	cfg.ApplyDefaults()
	return cfg
}

func TestNewOrchestrator(t *testing.T) {
	t.Run("creates orchestrator with defaults", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockFactory := &MockSignerFactory{}
		mockL1Factory := &MockL1ClientFactory{}

		orch := NewOrchestrator(mockRepo, mockFactory, mockL1Factory, OrchestratorConfig{})

		assert.NotNil(t, orch)
		assert.Equal(t, 3, orch.config.RetryAttempts)
	})

	t.Run("uses custom configuration", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockFactory := &MockSignerFactory{}
		mockL1Factory := &MockL1ClientFactory{}

		orch := NewOrchestrator(mockRepo, mockFactory, mockL1Factory, OrchestratorConfig{
			RetryAttempts: 5,
		})

		assert.Equal(t, 5, orch.config.RetryAttempts)
	})
}

func TestOrchestrator_Deploy(t *testing.T) {
	// Note: Full deployment testing requires integration tests with op-deployer artifacts.
	// The op-deployer pipeline requires contract artifacts that aren't available in unit tests.
	// See integration tests for full deployment flow testing.

	t.Run("loads deployment and starts pipeline", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockL1Client := new(MockL1Client)
		mockL1Factory := &MockL1ClientFactory{client: mockL1Client}
		mockSignerFactory := &MockSignerFactory{}

		deploymentID := uuid.New()
		cfg := createTestDeploymentConfig()
		cfgJSON, _ := json.Marshal(cfg)

		// Setup mocks
		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:     deploymentID,
			Config: cfgJSON,
			Status: repository.StatusPending,
		}, nil)

		// StateWriter operations
		mockRepo.On("UpdateDeploymentStatus", mock.Anything, deploymentID, mock.Anything, mock.Anything).Return(nil)
		// Expect failure to be recorded since op-deployer artifacts aren't available in tests
		mockRepo.On("SetDeploymentError", mock.Anything, deploymentID, mock.Anything).Return(nil)

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		err := orch.Deploy(context.Background(), deploymentID, nil)

		// Deployment will fail because op-deployer artifacts aren't available in unit tests
		// This is expected - full deployment testing requires integration tests
		require.Error(t, err)
		assert.Contains(t, err.Error(), "op-deployer pipeline failed")
	})

	t.Run("returns error if deployment not found", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockL1Factory := &MockL1ClientFactory{client: new(MockL1Client)}
		mockSignerFactory := &MockSignerFactory{}

		deploymentID := uuid.New()

		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(nil, nil)

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		err := orch.Deploy(context.Background(), deploymentID, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("returns error on invalid config", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockL1Factory := &MockL1ClientFactory{client: new(MockL1Client)}
		mockSignerFactory := &MockSignerFactory{}

		deploymentID := uuid.New()
		invalidConfig := json.RawMessage(`{"chain_id": 0}`) // Invalid: chain_id is 0

		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:     deploymentID,
			Config: invalidConfig,
			Status: repository.StatusPending,
		}, nil)

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		err := orch.Deploy(context.Background(), deploymentID, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse config")
	})
}

// Note: Individual stage tests (stageInit, etc.) have been removed
// as the orchestrator now uses the op-deployer pipeline which handles
// all stages internally. Integration tests should be used for full
// deployment flow testing.

func TestOrchestrator_Resume(t *testing.T) {
	t.Run("validates resume capability and starts deployment", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockL1Client := new(MockL1Client)
		mockL1Factory := &MockL1ClientFactory{client: mockL1Client}
		mockSignerFactory := &MockSignerFactory{}

		deploymentID := uuid.New()
		cfg := createTestDeploymentConfig()
		cfgJSON, _ := json.Marshal(cfg)

		currentStage := StageSuperchain.String()

		// First call: check resume capability
		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:           deploymentID,
			Config:       cfgJSON,
			Status:       repository.StatusPaused,
			CurrentStage: &currentStage,
		}, nil).Once()

		// Second call: in Deploy()
		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:           deploymentID,
			Config:       cfgJSON,
			Status:       repository.StatusPaused,
			CurrentStage: &currentStage,
		}, nil)

		mockRepo.On("UpdateDeploymentStatus", mock.Anything, deploymentID, mock.Anything, mock.Anything).Return(nil)
		// Expect failure to be recorded since op-deployer artifacts aren't available
		mockRepo.On("SetDeploymentError", mock.Anything, deploymentID, mock.Anything).Return(nil)

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		err := orch.Resume(context.Background(), deploymentID, nil)

		// Resume delegates to Deploy which will fail due to missing op-deployer artifacts
		require.Error(t, err)
		assert.Contains(t, err.Error(), "op-deployer pipeline failed")
	})

	t.Run("returns error if cannot resume", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockL1Factory := &MockL1ClientFactory{client: new(MockL1Client)}
		mockSignerFactory := &MockSignerFactory{}

		deploymentID := uuid.New()

		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:     deploymentID,
			Status: repository.StatusCompleted, // Cannot resume completed deployment
		}, nil)

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		err := orch.Resume(context.Background(), deploymentID, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be resumed")
	})
}

func TestOrchestrator_Pause(t *testing.T) {
	t.Run("marks deployment as paused", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockL1Factory := &MockL1ClientFactory{client: new(MockL1Client)}
		mockSignerFactory := &MockSignerFactory{}

		deploymentID := uuid.New()

		mockRepo.On("UpdateDeploymentStatus", mock.Anything, deploymentID, repository.StatusPaused, (*string)(nil)).Return(nil)

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		err := orch.Pause(context.Background(), deploymentID)

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestOrchestrator_GetDeploymentStatus(t *testing.T) {
	t.Run("returns deployment status", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockL1Factory := &MockL1ClientFactory{client: new(MockL1Client)}
		mockSignerFactory := &MockSignerFactory{}

		deploymentID := uuid.New()
		currentStage := StageSuperchain.String()

		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:           deploymentID,
			Status:       repository.StatusRunning,
			CurrentStage: &currentStage,
		}, nil)

		mockRepo.On("GetTransactionsByDeployment", mock.Anything, deploymentID).Return([]repository.Transaction{
			{ID: uuid.New(), Stage: "init", TxHash: "0x1"},
			{ID: uuid.New(), Stage: "superchain", TxHash: "0x2"},
		}, nil)

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		status, err := orch.GetDeploymentStatus(context.Background(), deploymentID)

		require.NoError(t, err)
		assert.Equal(t, repository.StatusRunning, status.Status)
		assert.Equal(t, StageSuperchain, status.CurrentStage)
		assert.Equal(t, 2, status.TransactionCount)
	})

	t.Run("returns error for missing deployment", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockL1Factory := &MockL1ClientFactory{client: new(MockL1Client)}
		mockSignerFactory := &MockSignerFactory{}

		deploymentID := uuid.New()

		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(nil, nil)

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		status, err := orch.GetDeploymentStatus(context.Background(), deploymentID)

		require.Error(t, err)
		assert.Nil(t, status)
	})
}

// Note: TestOrchestrator_StageSkipping and TestOrchestrator_ExecuteStageWithRetry
// have been removed as the orchestrator now uses the op-deployer pipeline
// which handles all stages and retries internally.

func TestDeploymentConfig_Validate(t *testing.T) {
	t.Run("requires chain_id", func(t *testing.T) {
		cfg := &DeploymentConfig{}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "chain_id")
	})

	t.Run("requires l1_rpc", func(t *testing.T) {
		cfg := &DeploymentConfig{ChainID: 1, ChainName: "test", L1ChainID: 1}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "l1_rpc")
	})

	t.Run("requires celestia_rpc (POPKins only supports Celestia)", func(t *testing.T) {
		cfg := &DeploymentConfig{
			ChainID:           1,
			ChainName:         "test",
			L1ChainID:         1,
			L1RPC:             "http://localhost:8545",
			POPSignerEndpoint: "http://localhost:8080",
			POPSignerAPIKey:   "key",
			DeployerAddress:   "0x123",
			// CelestiaRPC is missing - should fail validation
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "celestia_rpc is required")
	})

	t.Run("accepts valid config", func(t *testing.T) {
		cfg := createTestDeploymentConfig()
		err := cfg.Validate()
		require.NoError(t, err)
	})
}

func TestDeploymentConfig_ApplyDefaults(t *testing.T) {
	t.Run("applies all defaults", func(t *testing.T) {
		cfg := &DeploymentConfig{
			DeployerAddress: "0x1234567890123456789012345678901234567890",
		}
		cfg.ApplyDefaults()

		assert.Equal(t, uint64(2), cfg.BlockTime)
		assert.Equal(t, uint64(600), cfg.MaxSequencerDrift)
		assert.Equal(t, uint64(3600), cfg.SequencerWindowSize)
		assert.Equal(t, uint64(30000000), cfg.GasLimit)
		// CelestiaNamespace should be auto-generated
		assert.NotEmpty(t, cfg.CelestiaNamespace)
		assert.Equal(t, cfg.DeployerAddress, cfg.BatcherAddress)
		assert.Equal(t, cfg.DeployerAddress, cfg.ProposerAddress)
		assert.NotNil(t, cfg.RequiredFundingWei)
	})

	t.Run("uses higher funding for mainnet", func(t *testing.T) {
		cfg := &DeploymentConfig{
			L1ChainID:       1,
			DeployerAddress: "0x123",
		}
		cfg.ApplyDefaults()

		mainnetFunding := new(big.Int).Mul(big.NewInt(5), big.NewInt(1e18))
		assert.Equal(t, 0, cfg.RequiredFundingWei.Cmp(mainnetFunding))
	})
}

func TestParseConfig(t *testing.T) {
	t.Run("parses valid JSON", func(t *testing.T) {
		jsonData := json.RawMessage(`{
			"chain_id": 42069,
			"chain_name": "test-chain",
			"l1_chain_id": 11155111,
			"l1_rpc": "http://localhost:8545",
			"popsigner_endpoint": "http://localhost:8080",
			"popsigner_api_key": "test-key",
			"deployer_address": "0x1234567890123456789012345678901234567890",
			"celestia_rpc": "http://localhost:26658"
		}`)

		cfg, err := ParseConfig(jsonData)

		require.NoError(t, err)
		assert.Equal(t, uint64(42069), cfg.ChainID)
		assert.Equal(t, "test-chain", cfg.ChainName)
		// Defaults should be applied
		assert.Equal(t, uint64(2), cfg.BlockTime)
		// Celestia namespace should be auto-generated
		assert.NotEmpty(t, cfg.CelestiaNamespace)
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		jsonData := json.RawMessage(`{invalid}`)

		cfg, err := ParseConfig(jsonData)

		require.Error(t, err)
		assert.Nil(t, cfg)
	})
}

