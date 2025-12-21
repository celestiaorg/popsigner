package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepository is a mock implementation of Repository for testing.
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateDeployment(ctx context.Context, d *Deployment) error {
	args := m.Called(ctx, d)
	if args.Error(0) == nil {
		if d.ID == uuid.Nil {
			d.ID = uuid.New()
		}
		d.CreatedAt = time.Now()
		d.UpdatedAt = time.Now()
	}
	return args.Error(0)
}

func (m *MockRepository) GetDeployment(ctx context.Context, id uuid.UUID) (*Deployment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Deployment), args.Error(1)
}

func (m *MockRepository) GetDeploymentByChainID(ctx context.Context, chainID int64) (*Deployment, error) {
	args := m.Called(ctx, chainID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Deployment), args.Error(1)
}

func (m *MockRepository) UpdateDeploymentStatus(ctx context.Context, id uuid.UUID, status Status, stage *string) error {
	args := m.Called(ctx, id, status, stage)
	return args.Error(0)
}

func (m *MockRepository) SetDeploymentError(ctx context.Context, id uuid.UUID, errMsg string) error {
	args := m.Called(ctx, id, errMsg)
	return args.Error(0)
}

func (m *MockRepository) ListDeploymentsByStatus(ctx context.Context, status Status) ([]*Deployment, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Deployment), args.Error(1)
}

func (m *MockRepository) ListAllDeployments(ctx context.Context) ([]*Deployment, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Deployment), args.Error(1)
}

func (m *MockRepository) MarkStaleDeploymentsFailed(ctx context.Context, timeout time.Duration) (int, error) {
	args := m.Called(ctx, timeout)
	return args.Int(0), args.Error(1)
}

func (m *MockRepository) UpdateDeploymentConfig(ctx context.Context, id uuid.UUID, config json.RawMessage) error {
	args := m.Called(ctx, id, config)
	return args.Error(0)
}

func (m *MockRepository) RecordTransaction(ctx context.Context, tx *Transaction) error {
	args := m.Called(ctx, tx)
	if args.Error(0) == nil {
		if tx.ID == uuid.Nil {
			tx.ID = uuid.New()
		}
		tx.CreatedAt = time.Now()
	}
	return args.Error(0)
}

func (m *MockRepository) GetTransactionsByDeployment(ctx context.Context, deploymentID uuid.UUID) ([]Transaction, error) {
	args := m.Called(ctx, deploymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Transaction), args.Error(1)
}

func (m *MockRepository) GetTransactionByHash(ctx context.Context, hash string) (*Transaction, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Transaction), args.Error(1)
}

func (m *MockRepository) SaveArtifact(ctx context.Context, a *Artifact) error {
	args := m.Called(ctx, a)
	if args.Error(0) == nil {
		if a.ID == uuid.Nil {
			a.ID = uuid.New()
		}
		a.CreatedAt = time.Now()
	}
	return args.Error(0)
}

func (m *MockRepository) GetArtifact(ctx context.Context, deploymentID uuid.UUID, artifactType string) (*Artifact, error) {
	args := m.Called(ctx, deploymentID, artifactType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Artifact), args.Error(1)
}

func (m *MockRepository) GetAllArtifacts(ctx context.Context, deploymentID uuid.UUID) ([]Artifact, error) {
	args := m.Called(ctx, deploymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Artifact), args.Error(1)
}

// Verify MockRepository implements Repository
var _ Repository = (*MockRepository)(nil)

// --- Deployment Tests ---

func TestMockRepository_CreateDeployment(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	config := json.RawMessage(`{"l1ChainId": 1, "name": "test-chain"}`)
	d := &Deployment{
		ChainID: 12345,
		Stack:   StackOPStack,
		Status:  StatusPending,
		Config:  config,
	}

	mockRepo.On("CreateDeployment", ctx, d).Return(nil)

	err := mockRepo.CreateDeployment(ctx, d)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, d.ID)
	assert.False(t, d.CreatedAt.IsZero())
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_GetDeployment(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	deploymentID := uuid.New()
	expected := &Deployment{
		ID:      deploymentID,
		ChainID: 12345,
		Stack:   StackNitro,
		Status:  StatusRunning,
		Config:  json.RawMessage(`{}`),
	}

	mockRepo.On("GetDeployment", ctx, deploymentID).Return(expected, nil)

	d, err := mockRepo.GetDeployment(ctx, deploymentID)
	assert.NoError(t, err)
	assert.Equal(t, expected.ID, d.ID)
	assert.Equal(t, expected.Stack, d.Stack)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_GetDeployment_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	deploymentID := uuid.New()
	mockRepo.On("GetDeployment", ctx, deploymentID).Return(nil, ErrNotFound)

	d, err := mockRepo.GetDeployment(ctx, deploymentID)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Nil(t, d)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_GetDeploymentByChainID(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	chainID := int64(12345)
	expected := &Deployment{
		ID:      uuid.New(),
		ChainID: chainID,
		Stack:   StackOPStack,
		Status:  StatusCompleted,
		Config:  json.RawMessage(`{}`),
	}

	mockRepo.On("GetDeploymentByChainID", ctx, chainID).Return(expected, nil)

	d, err := mockRepo.GetDeploymentByChainID(ctx, chainID)
	assert.NoError(t, err)
	assert.Equal(t, chainID, d.ChainID)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_UpdateDeploymentStatus(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	deploymentID := uuid.New()
	stage := "deploy-contracts"

	mockRepo.On("UpdateDeploymentStatus", ctx, deploymentID, StatusRunning, &stage).Return(nil)

	err := mockRepo.UpdateDeploymentStatus(ctx, deploymentID, StatusRunning, &stage)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_SetDeploymentError(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	deploymentID := uuid.New()
	errMsg := "insufficient funds"

	mockRepo.On("SetDeploymentError", ctx, deploymentID, errMsg).Return(nil)

	err := mockRepo.SetDeploymentError(ctx, deploymentID, errMsg)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_ListDeploymentsByStatus(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	expected := []*Deployment{
		{ID: uuid.New(), ChainID: 1, Stack: StackOPStack, Status: StatusRunning},
		{ID: uuid.New(), ChainID: 2, Stack: StackNitro, Status: StatusRunning},
	}

	mockRepo.On("ListDeploymentsByStatus", ctx, StatusRunning).Return(expected, nil)

	deployments, err := mockRepo.ListDeploymentsByStatus(ctx, StatusRunning)
	assert.NoError(t, err)
	assert.Len(t, deployments, 2)
	mockRepo.AssertExpectations(t)
}

// --- Transaction Tests ---

func TestMockRepository_RecordTransaction(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	deploymentID := uuid.New()
	desc := "Deploy L1 contracts"
	tx := &Transaction{
		DeploymentID: deploymentID,
		Stage:        "deploy-l1",
		TxHash:       "0xabc123",
		Description:  &desc,
	}

	mockRepo.On("RecordTransaction", ctx, tx).Return(nil)

	err := mockRepo.RecordTransaction(ctx, tx)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, tx.ID)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_GetTransactionsByDeployment(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	deploymentID := uuid.New()
	expected := []Transaction{
		{ID: uuid.New(), DeploymentID: deploymentID, Stage: "stage-1", TxHash: "0x111"},
		{ID: uuid.New(), DeploymentID: deploymentID, Stage: "stage-2", TxHash: "0x222"},
	}

	mockRepo.On("GetTransactionsByDeployment", ctx, deploymentID).Return(expected, nil)

	txns, err := mockRepo.GetTransactionsByDeployment(ctx, deploymentID)
	assert.NoError(t, err)
	assert.Len(t, txns, 2)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_GetTransactionByHash(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	hash := "0xabc123"
	expected := &Transaction{
		ID:           uuid.New(),
		DeploymentID: uuid.New(),
		Stage:        "deploy-contracts",
		TxHash:       hash,
	}

	mockRepo.On("GetTransactionByHash", ctx, hash).Return(expected, nil)

	tx, err := mockRepo.GetTransactionByHash(ctx, hash)
	assert.NoError(t, err)
	assert.Equal(t, hash, tx.TxHash)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_GetTransactionByHash_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	hash := "0xnonexistent"
	mockRepo.On("GetTransactionByHash", ctx, hash).Return(nil, ErrNotFound)

	tx, err := mockRepo.GetTransactionByHash(ctx, hash)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Nil(t, tx)
	mockRepo.AssertExpectations(t)
}

// --- Artifact Tests ---

func TestMockRepository_SaveArtifact(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	deploymentID := uuid.New()
	content := json.RawMessage(`{"genesis": {"config": {}}}`)
	a := &Artifact{
		DeploymentID: deploymentID,
		ArtifactType: "genesis",
		Content:      content,
	}

	mockRepo.On("SaveArtifact", ctx, a).Return(nil)

	err := mockRepo.SaveArtifact(ctx, a)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, a.ID)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_GetArtifact(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	deploymentID := uuid.New()
	expected := &Artifact{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ArtifactType: "genesis",
		Content:      json.RawMessage(`{"config": {}}`),
	}

	mockRepo.On("GetArtifact", ctx, deploymentID, "genesis").Return(expected, nil)

	a, err := mockRepo.GetArtifact(ctx, deploymentID, "genesis")
	assert.NoError(t, err)
	assert.Equal(t, "genesis", a.ArtifactType)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_GetArtifact_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	deploymentID := uuid.New()
	mockRepo.On("GetArtifact", ctx, deploymentID, "nonexistent").Return(nil, ErrNotFound)

	a, err := mockRepo.GetArtifact(ctx, deploymentID, "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Nil(t, a)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_GetAllArtifacts(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	deploymentID := uuid.New()
	expected := []Artifact{
		{ID: uuid.New(), DeploymentID: deploymentID, ArtifactType: "genesis", Content: json.RawMessage(`{}`)},
		{ID: uuid.New(), DeploymentID: deploymentID, ArtifactType: "rollup_config", Content: json.RawMessage(`{}`)},
	}

	mockRepo.On("GetAllArtifacts", ctx, deploymentID).Return(expected, nil)

	artifacts, err := mockRepo.GetAllArtifacts(ctx, deploymentID)
	assert.NoError(t, err)
	assert.Len(t, artifacts, 2)
	mockRepo.AssertExpectations(t)
}

func TestMockRepository_GetAllArtifacts_Empty(t *testing.T) {
	mockRepo := new(MockRepository)
	ctx := context.Background()

	deploymentID := uuid.New()
	expected := []Artifact{}

	mockRepo.On("GetAllArtifacts", ctx, deploymentID).Return(expected, nil)

	artifacts, err := mockRepo.GetAllArtifacts(ctx, deploymentID)
	assert.NoError(t, err)
	assert.Empty(t, artifacts)
	mockRepo.AssertExpectations(t)
}

// --- Type Tests ---

func TestStackConstants(t *testing.T) {
	assert.Equal(t, Stack("opstack"), StackOPStack)
	assert.Equal(t, Stack("nitro"), StackNitro)
}

func TestStatusConstants(t *testing.T) {
	assert.Equal(t, Status("pending"), StatusPending)
	assert.Equal(t, Status("running"), StatusRunning)
	assert.Equal(t, Status("paused"), StatusPaused)
	assert.Equal(t, Status("completed"), StatusCompleted)
	assert.Equal(t, Status("failed"), StatusFailed)
}

