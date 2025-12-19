package opstack

import (
	"context"
	"encoding/json"
	"fmt"
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
	t.Run("completes full deployment successfully", func(t *testing.T) {
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

		// For CanResume check
		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:           deploymentID,
			Config:       cfgJSON,
			Status:       repository.StatusPending,
			CurrentStage: nil,
		}, nil)

		// L1 client mocks
		mockL1Client.On("ChainID", mock.Anything).Return(big.NewInt(11155111), nil)
		mockL1Client.On("BalanceAt", mock.Anything, mock.Anything, mock.Anything).Return(big.NewInt(5e18), nil)

		// StateWriter operations
		mockRepo.On("UpdateDeploymentStatus", mock.Anything, deploymentID, mock.Anything, mock.Anything).Return(nil)
		mockRepo.On("SaveArtifact", mock.Anything, mock.Anything).Return(nil)
		mockRepo.On("RecordTransaction", mock.Anything, mock.Anything).Return(nil)
		mockRepo.On("GetArtifact", mock.Anything, deploymentID, "deployment_state").Return(nil, nil)

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		// Track progress
		var progressStages []Stage
		onProgress := func(stage Stage, progress float64, message string) {
			progressStages = append(progressStages, stage)
		}

		err := orch.Deploy(context.Background(), deploymentID, onProgress)

		require.NoError(t, err)
		assert.NotEmpty(t, progressStages)
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

func TestOrchestrator_StageInit(t *testing.T) {
	t.Run("validates L1 chain ID", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockL1Client := new(MockL1Client)
		mockL1Factory := &MockL1ClientFactory{client: mockL1Client}
		mockSignerFactory := &MockSignerFactory{}

		deploymentID := uuid.New()
		cfg := createTestDeploymentConfig()

		// Wrong chain ID
		mockL1Client.On("ChainID", mock.Anything).Return(big.NewInt(1), nil)

		stateWriter := NewStateWriter(mockRepo, deploymentID)
		dctx := &DeploymentContext{
			DeploymentID: deploymentID,
			Config:       cfg,
			StateWriter:  stateWriter,
			L1Client:     mockL1Client,
		}

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		err := orch.stageInit(context.Background(), dctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "chain ID mismatch")
	})

	t.Run("checks deployer balance", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockL1Client := new(MockL1Client)
		mockL1Factory := &MockL1ClientFactory{client: mockL1Client}
		mockSignerFactory := &MockSignerFactory{}

		deploymentID := uuid.New()
		cfg := createTestDeploymentConfig()

		// Correct chain ID but insufficient balance
		mockL1Client.On("ChainID", mock.Anything).Return(big.NewInt(11155111), nil)
		mockL1Client.On("BalanceAt", mock.Anything, mock.Anything, mock.Anything).Return(big.NewInt(1000), nil)

		stateWriter := NewStateWriter(mockRepo, deploymentID)
		dctx := &DeploymentContext{
			DeploymentID: deploymentID,
			Config:       cfg,
			StateWriter:  stateWriter,
			L1Client:     mockL1Client,
		}

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		err := orch.stageInit(context.Background(), dctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient deployer balance")
	})

	t.Run("succeeds with valid configuration", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockL1Client := new(MockL1Client)
		mockL1Factory := &MockL1ClientFactory{client: mockL1Client}
		mockSignerFactory := &MockSignerFactory{}

		deploymentID := uuid.New()
		cfg := createTestDeploymentConfig()

		mockL1Client.On("ChainID", mock.Anything).Return(big.NewInt(11155111), nil)
		mockL1Client.On("BalanceAt", mock.Anything, mock.Anything, mock.Anything).Return(big.NewInt(5e18), nil)
		mockRepo.On("SaveArtifact", mock.Anything, mock.Anything).Return(nil)

		stateWriter := NewStateWriter(mockRepo, deploymentID)
		dctx := &DeploymentContext{
			DeploymentID: deploymentID,
			Config:       cfg,
			StateWriter:  stateWriter,
			L1Client:     mockL1Client,
		}

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		err := orch.stageInit(context.Background(), dctx)

		require.NoError(t, err)
	})
}

func TestOrchestrator_Resume(t *testing.T) {
	t.Run("resumes from previous stage", func(t *testing.T) {
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

		mockL1Client.On("ChainID", mock.Anything).Return(big.NewInt(11155111), nil)
		mockL1Client.On("BalanceAt", mock.Anything, mock.Anything, mock.Anything).Return(big.NewInt(5e18), nil)

		mockRepo.On("UpdateDeploymentStatus", mock.Anything, deploymentID, mock.Anything, mock.Anything).Return(nil)
		mockRepo.On("SaveArtifact", mock.Anything, mock.Anything).Return(nil)
		mockRepo.On("RecordTransaction", mock.Anything, mock.Anything).Return(nil)
		mockRepo.On("GetArtifact", mock.Anything, deploymentID, "deployment_state").Return(nil, nil)

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		err := orch.Resume(context.Background(), deploymentID, nil)

		require.NoError(t, err)
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

func TestOrchestrator_StageSkipping(t *testing.T) {
	t.Run("executes alt-da stage (POPKins always uses Celestia)", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockL1Client := new(MockL1Client)
		mockL1Factory := &MockL1ClientFactory{client: mockL1Client}
		mockSignerFactory := &MockSignerFactory{}

		deploymentID := uuid.New()
		cfg := createTestDeploymentConfig()
		// POPKins always uses Celestia DA - no need to set UseAltDA

		// For IsStageComplete check
		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:           deploymentID,
			CurrentStage: nil,
		}, nil)
		mockRepo.On("GetArtifact", mock.Anything, deploymentID, "deployment_state").Return(nil, nil)
		mockRepo.On("SaveArtifact", mock.Anything, mock.Anything).Return(nil)

		stateWriter := NewStateWriter(mockRepo, deploymentID)
		dctx := &DeploymentContext{
			DeploymentID: deploymentID,
			Config:       cfg,
			StateWriter:  stateWriter,
			L1Client:     mockL1Client,
		}

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{})

		err := orch.stageAltDA(context.Background(), dctx)

		require.NoError(t, err)
	})
}

func TestOrchestrator_ExecuteStageWithRetry(t *testing.T) {
	t.Run("retries on retryable error", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockL1Client := new(MockL1Client)
		mockL1Factory := &MockL1ClientFactory{client: mockL1Client}
		mockSignerFactory := &MockSignerFactory{}

		deploymentID := uuid.New()
		cfg := createTestDeploymentConfig()

		// First two calls fail with retryable error, third succeeds
		mockL1Client.On("ChainID", mock.Anything).Return(nil, &RetryableError{Err: fmt.Errorf("connection refused")}).Twice()
		mockL1Client.On("ChainID", mock.Anything).Return(big.NewInt(11155111), nil).Once()
		mockL1Client.On("BalanceAt", mock.Anything, mock.Anything, mock.Anything).Return(big.NewInt(5e18), nil)
		mockRepo.On("SaveArtifact", mock.Anything, mock.Anything).Return(nil)

		stateWriter := NewStateWriter(mockRepo, deploymentID)
		dctx := &DeploymentContext{
			DeploymentID: deploymentID,
			Config:       cfg,
			StateWriter:  stateWriter,
			L1Client:     mockL1Client,
		}

		orch := NewOrchestrator(mockRepo, mockSignerFactory, mockL1Factory, OrchestratorConfig{
			RetryAttempts: 3,
		})

		err := orch.executeStageWithRetry(context.Background(), dctx, StageInit)

		require.NoError(t, err)
		mockL1Client.AssertNumberOfCalls(t, "ChainID", 3)
	})
}

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
			"deployer_address": "0x1234567890123456789012345678901234567890"
		}`)

		cfg, err := ParseConfig(jsonData)

		require.NoError(t, err)
		assert.Equal(t, uint64(42069), cfg.ChainID)
		assert.Equal(t, "test-chain", cfg.ChainName)
		// Defaults should be applied
		assert.Equal(t, uint64(2), cfg.BlockTime)
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		jsonData := json.RawMessage(`{invalid}`)

		cfg, err := ParseConfig(jsonData)

		require.Error(t, err)
		assert.Nil(t, cfg)
	})
}

