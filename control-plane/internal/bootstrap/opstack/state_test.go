package opstack

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// MockRepository is a mock implementation of repository.Repository.
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateDeployment(ctx context.Context, d *repository.Deployment) error {
	args := m.Called(ctx, d)
	return args.Error(0)
}

func (m *MockRepository) GetDeployment(ctx context.Context, id uuid.UUID) (*repository.Deployment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Deployment), args.Error(1)
}

func (m *MockRepository) GetDeploymentByChainID(ctx context.Context, chainID int64) (*repository.Deployment, error) {
	args := m.Called(ctx, chainID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Deployment), args.Error(1)
}

func (m *MockRepository) UpdateDeploymentStatus(ctx context.Context, id uuid.UUID, status repository.Status, stage *string) error {
	args := m.Called(ctx, id, status, stage)
	return args.Error(0)
}

func (m *MockRepository) SetDeploymentError(ctx context.Context, id uuid.UUID, errMsg string) error {
	args := m.Called(ctx, id, errMsg)
	return args.Error(0)
}

func (m *MockRepository) ClearDeploymentError(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) ListDeploymentsByStatus(ctx context.Context, status repository.Status) ([]*repository.Deployment, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Deployment), args.Error(1)
}

func (m *MockRepository) ListAllDeployments(ctx context.Context) ([]*repository.Deployment, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Deployment), args.Error(1)
}

func (m *MockRepository) MarkStaleDeploymentsFailed(ctx context.Context, timeout time.Duration) (int, error) {
	args := m.Called(ctx, timeout)
	return args.Int(0), args.Error(1)
}

func (m *MockRepository) UpdateDeploymentConfig(ctx context.Context, id uuid.UUID, config json.RawMessage) error {
	args := m.Called(ctx, id, config)
	return args.Error(0)
}

func (m *MockRepository) RecordTransaction(ctx context.Context, tx *repository.Transaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}

func (m *MockRepository) GetTransactionsByDeployment(ctx context.Context, deploymentID uuid.UUID) ([]repository.Transaction, error) {
	args := m.Called(ctx, deploymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.Transaction), args.Error(1)
}

func (m *MockRepository) GetTransactionByHash(ctx context.Context, hash string) (*repository.Transaction, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Transaction), args.Error(1)
}

func (m *MockRepository) SaveArtifact(ctx context.Context, a *repository.Artifact) error {
	args := m.Called(ctx, a)
	return args.Error(0)
}

func (m *MockRepository) GetArtifact(ctx context.Context, deploymentID uuid.UUID, artifactType string) (*repository.Artifact, error) {
	args := m.Called(ctx, deploymentID, artifactType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Artifact), args.Error(1)
}

func (m *MockRepository) GetAllArtifacts(ctx context.Context, deploymentID uuid.UUID) ([]repository.Artifact, error) {
	args := m.Called(ctx, deploymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.Artifact), args.Error(1)
}

func TestNewStateWriter(t *testing.T) {
	t.Run("creates state writer with correct deployment ID", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()

		writer := NewStateWriter(mockRepo, deploymentID)

		assert.NotNil(t, writer)
		assert.Equal(t, deploymentID, writer.DeploymentID())
	})
}

func TestStateWriter_SetUpdateCallback(t *testing.T) {
	t.Run("callback is invoked on stage update", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		var callbackDeploymentID uuid.UUID
		var callbackStage string
		writer.SetUpdateCallback(func(id uuid.UUID, stage string) {
			callbackDeploymentID = id
			callbackStage = stage
		})

		mockRepo.On("UpdateDeploymentStatus", mock.Anything, deploymentID, repository.StatusRunning, mock.Anything).Return(nil)

		err := writer.UpdateStage(context.Background(), StageSuperchain)

		require.NoError(t, err)
		assert.Equal(t, deploymentID, callbackDeploymentID)
		assert.Equal(t, StageSuperchain.String(), callbackStage)
	})
}

func TestStateWriter_WriteState(t *testing.T) {
	t.Run("marshals and saves state as artifact", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		testState := map[string]interface{}{
			"version": 1,
			"chains":  []string{"chain1"},
		}

		mockRepo.On("SaveArtifact", mock.Anything, mock.MatchedBy(func(a *repository.Artifact) bool {
			return a.DeploymentID == deploymentID && a.ArtifactType == "deployment_state"
		})).Return(nil)

		err := writer.WriteState(context.Background(), testState)

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns error on marshal failure", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		// Channels cannot be marshaled to JSON
		invalidState := make(chan int)

		err := writer.WriteState(context.Background(), invalidState)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal state")
	})

	t.Run("returns error on save failure", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		mockRepo.On("SaveArtifact", mock.Anything, mock.Anything).Return(fmt.Errorf("database error"))

		err := writer.WriteState(context.Background(), map[string]string{"test": "data"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "save state artifact")
	})
}

func TestStateWriter_ReadState(t *testing.T) {
	t.Run("returns state from artifact", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		expectedState := json.RawMessage(`{"version":1,"chains":["chain1"]}`)
		mockRepo.On("GetArtifact", mock.Anything, deploymentID, "deployment_state").Return(&repository.Artifact{
			ID:           uuid.New(),
			DeploymentID: deploymentID,
			ArtifactType: "deployment_state",
			Content:      expectedState,
		}, nil)

		state, err := writer.ReadState(context.Background())

		require.NoError(t, err)
		assert.JSONEq(t, string(expectedState), string(state))
	})

	t.Run("returns nil for missing state", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		mockRepo.On("GetArtifact", mock.Anything, deploymentID, "deployment_state").Return(nil, nil)

		state, err := writer.ReadState(context.Background())

		require.NoError(t, err)
		assert.Nil(t, state)
	})
}

func TestStateWriter_UpdateStage(t *testing.T) {
	t.Run("updates deployment status", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		stageStr := StageSuperchain.String()
		mockRepo.On("UpdateDeploymentStatus", mock.Anything, deploymentID, repository.StatusRunning, &stageStr).Return(nil)

		err := writer.UpdateStage(context.Background(), StageSuperchain)

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestStateWriter_GetCurrentStage(t *testing.T) {
	t.Run("returns current stage", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		stage := StageSuperchain.String()
		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:           deploymentID,
			CurrentStage: &stage,
		}, nil)

		currentStage, err := writer.GetCurrentStage(context.Background())

		require.NoError(t, err)
		assert.Equal(t, StageSuperchain, currentStage)
	})

	t.Run("returns init for nil stage", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:           deploymentID,
			CurrentStage: nil,
		}, nil)

		currentStage, err := writer.GetCurrentStage(context.Background())

		require.NoError(t, err)
		assert.Equal(t, StageInit, currentStage)
	})

	t.Run("returns error for missing deployment", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(nil, nil)

		_, err := writer.GetCurrentStage(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "deployment not found")
	})
}

func TestStateWriter_RecordTransaction(t *testing.T) {
	t.Run("records transaction with description", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		mockRepo.On("RecordTransaction", mock.Anything, mock.MatchedBy(func(tx *repository.Transaction) bool {
			return tx.DeploymentID == deploymentID &&
				tx.Stage == StageSuperchain.String() &&
				tx.TxHash == "0xabc123" &&
				*tx.Description == "Deploy SuperchainConfig"
		})).Return(nil)

		err := writer.RecordTransaction(context.Background(), StageSuperchain, "0xabc123", "Deploy SuperchainConfig")

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("records transaction without description", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		mockRepo.On("RecordTransaction", mock.Anything, mock.MatchedBy(func(tx *repository.Transaction) bool {
			return tx.DeploymentID == deploymentID &&
				tx.Stage == StageSuperchain.String() &&
				tx.TxHash == "0xabc123" &&
				tx.Description == nil
		})).Return(nil)

		err := writer.RecordTransaction(context.Background(), StageSuperchain, "0xabc123", "")

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestStateWriter_HasTransaction(t *testing.T) {
	t.Run("returns true for existing transaction", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		mockRepo.On("GetTransactionByHash", mock.Anything, "0xabc123").Return(&repository.Transaction{
			ID:     uuid.New(),
			TxHash: "0xabc123",
		}, nil)

		exists, err := writer.HasTransaction(context.Background(), "0xabc123")

		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("returns false for missing transaction", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		mockRepo.On("GetTransactionByHash", mock.Anything, "0xabc123").Return(nil, nil)

		exists, err := writer.HasTransaction(context.Background(), "0xabc123")

		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("returns false on error (allows retry)", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		mockRepo.On("GetTransactionByHash", mock.Anything, "0xabc123").Return(nil, fmt.Errorf("database error"))

		exists, err := writer.HasTransaction(context.Background(), "0xabc123")

		require.NoError(t, err) // Error is swallowed
		assert.False(t, exists)
	})
}

func TestStateWriter_GetTransactions(t *testing.T) {
	t.Run("returns all transactions", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		txs := []repository.Transaction{
			{ID: uuid.New(), Stage: StageSuperchain.String(), TxHash: "0x1"},
			{ID: uuid.New(), Stage: StageImplementations.String(), TxHash: "0x2"},
		}
		mockRepo.On("GetTransactionsByDeployment", mock.Anything, deploymentID).Return(txs, nil)

		result, err := writer.GetTransactions(context.Background())

		require.NoError(t, err)
		assert.Len(t, result, 2)
	})
}

func TestStateWriter_GetTransactionsByStage(t *testing.T) {
	t.Run("filters transactions by stage", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		txs := []repository.Transaction{
			{ID: uuid.New(), Stage: StageSuperchain.String(), TxHash: "0x1"},
			{ID: uuid.New(), Stage: StageImplementations.String(), TxHash: "0x2"},
			{ID: uuid.New(), Stage: StageSuperchain.String(), TxHash: "0x3"},
		}
		mockRepo.On("GetTransactionsByDeployment", mock.Anything, deploymentID).Return(txs, nil)

		result, err := writer.GetTransactionsByStage(context.Background(), StageSuperchain)

		require.NoError(t, err)
		assert.Len(t, result, 2)
		for _, tx := range result {
			assert.Equal(t, StageSuperchain.String(), tx.Stage)
		}
	})
}

func TestStateWriter_MarkComplete(t *testing.T) {
	t.Run("updates status to completed", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		completedStr := StageCompleted.String()
		mockRepo.On("UpdateDeploymentStatus", mock.Anything, deploymentID, repository.StatusCompleted, &completedStr).Return(nil)

		err := writer.MarkComplete(context.Background())

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("invokes callback on complete", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		var callbackStage string
		writer.SetUpdateCallback(func(id uuid.UUID, stage string) {
			callbackStage = stage
		})

		completedStr := StageCompleted.String()
		mockRepo.On("UpdateDeploymentStatus", mock.Anything, deploymentID, repository.StatusCompleted, &completedStr).Return(nil)

		err := writer.MarkComplete(context.Background())

		require.NoError(t, err)
		assert.Equal(t, StageCompleted.String(), callbackStage)
	})
}

func TestStateWriter_MarkFailed(t *testing.T) {
	t.Run("sets error and updates status", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		mockRepo.On("SetDeploymentError", mock.Anything, deploymentID, "deployment failed: out of gas").Return(nil)
		mockRepo.On("UpdateDeploymentStatus", mock.Anything, deploymentID, repository.StatusFailed, (*string)(nil)).Return(nil)

		err := writer.MarkFailed(context.Background(), "deployment failed: out of gas")

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestStateWriter_MarkPaused(t *testing.T) {
	t.Run("updates status to paused", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		mockRepo.On("UpdateDeploymentStatus", mock.Anything, deploymentID, repository.StatusPaused, (*string)(nil)).Return(nil)

		err := writer.MarkPaused(context.Background())

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestStateWriter_CanResume(t *testing.T) {
	tests := []struct {
		name      string
		status    repository.Status
		canResume bool
	}{
		{"paused deployment can resume", repository.StatusPaused, true},
		{"running deployment can resume", repository.StatusRunning, true},
		{"failed deployment can resume", repository.StatusFailed, true},
		{"pending deployment cannot resume", repository.StatusPending, false},
		{"completed deployment cannot resume", repository.StatusCompleted, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			deploymentID := uuid.New()
			writer := NewStateWriter(mockRepo, deploymentID)

			mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
				ID:     deploymentID,
				Status: tt.status,
			}, nil)

			canResume, err := writer.CanResume(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.canResume, canResume)
		})
	}
}

func TestStateWriter_GetResumePoint(t *testing.T) {
	t.Run("returns stage and state", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		stage := StageSuperchain.String()
		state := json.RawMessage(`{"version":1}`)

		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:           deploymentID,
			CurrentStage: &stage,
		}, nil)
		mockRepo.On("GetArtifact", mock.Anything, deploymentID, "deployment_state").Return(&repository.Artifact{
			Content: state,
		}, nil)

		resumeStage, resumeState, err := writer.GetResumePoint(context.Background())

		require.NoError(t, err)
		assert.Equal(t, StageSuperchain, resumeStage)
		assert.JSONEq(t, string(state), string(resumeState))
	})
}

func TestStateWriter_SaveArtifact(t *testing.T) {
	t.Run("saves artifact with correct type", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		content := json.RawMessage(`{"genesis":"data"}`)

		mockRepo.On("SaveArtifact", mock.Anything, mock.MatchedBy(func(a *repository.Artifact) bool {
			return a.DeploymentID == deploymentID &&
				a.ArtifactType == "genesis" &&
				string(a.Content) == string(content)
		})).Return(nil)

		err := writer.SaveArtifact(context.Background(), "genesis", content)

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestStateWriter_GetArtifact(t *testing.T) {
	t.Run("returns artifact content", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		content := json.RawMessage(`{"genesis":"data"}`)
		mockRepo.On("GetArtifact", mock.Anything, deploymentID, "genesis").Return(&repository.Artifact{
			Content: content,
		}, nil)

		result, err := writer.GetArtifact(context.Background(), "genesis")

		require.NoError(t, err)
		assert.JSONEq(t, string(content), string(result))
	})

	t.Run("returns nil for missing artifact", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		mockRepo.On("GetArtifact", mock.Anything, deploymentID, "genesis").Return(nil, nil)

		result, err := writer.GetArtifact(context.Background(), "genesis")

		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestStageOrder(t *testing.T) {
	t.Run("stages are in correct order", func(t *testing.T) {
		assert.Equal(t, 0, StageIndex(StageInit))
		assert.Equal(t, 1, StageIndex(StageSuperchain))
		assert.Equal(t, 2, StageIndex(StageImplementations))
		assert.Equal(t, 3, StageIndex(StageOPChain))
		assert.Equal(t, 4, StageIndex(StageAltDA))
		assert.Equal(t, 5, StageIndex(StageGenesis))
		assert.Equal(t, 6, StageIndex(StageStartBlock))
		assert.Equal(t, 7, StageIndex(StageCompleted))
	})

	t.Run("unknown stage returns -1", func(t *testing.T) {
		assert.Equal(t, -1, StageIndex(Stage("unknown")))
	})
}

func TestStateWriter_IsStageComplete(t *testing.T) {
	t.Run("returns true when current stage is after target", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		stage := StageImplementations.String()
		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:           deploymentID,
			CurrentStage: &stage,
		}, nil)

		complete, err := writer.IsStageComplete(context.Background(), StageSuperchain)

		require.NoError(t, err)
		assert.True(t, complete)
	})

	t.Run("returns false when current stage is before target", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		stage := StageSuperchain.String()
		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:           deploymentID,
			CurrentStage: &stage,
		}, nil)

		complete, err := writer.IsStageComplete(context.Background(), StageImplementations)

		require.NoError(t, err)
		assert.False(t, complete)
	})

	t.Run("returns false when at same stage", func(t *testing.T) {
		mockRepo := new(MockRepository)
		deploymentID := uuid.New()
		writer := NewStateWriter(mockRepo, deploymentID)

		stage := StageSuperchain.String()
		mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(&repository.Deployment{
			ID:           deploymentID,
			CurrentStage: &stage,
		}, nil)

		complete, err := writer.IsStageComplete(context.Background(), StageSuperchain)

		require.NoError(t, err)
		assert.False(t, complete)
	})
}

func TestStage_String(t *testing.T) {
	tests := []struct {
		stage    Stage
		expected string
	}{
		{StageInit, "init"},
		{StageSuperchain, "deploy_superchain"},
		{StageImplementations, "deploy_implementations"},
		{StageOPChain, "deploy_opchain"},
		{StageAltDA, "deploy_alt_da"},
		{StageGenesis, "generate_genesis"},
		{StageStartBlock, "set_start_block"},
		{StageCompleted, "completed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.stage.String())
		})
	}
}

// Ensure MockRepository implements Repository interface
var _ repository.Repository = (*MockRepository)(nil)

