package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// MockRepository is a mock implementation of repository.Repository for testing.
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateDeployment(ctx context.Context, d *repository.Deployment) error {
	args := m.Called(ctx, d)
	if args.Error(0) == nil {
		d.CreatedAt = time.Now()
		d.UpdatedAt = time.Now()
	}
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

func (m *MockRepository) ListDeploymentsByStatus(ctx context.Context, status repository.Status) ([]*repository.Deployment, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Deployment), args.Error(1)
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

// Verify MockRepository implements repository.Repository
var _ repository.Repository = (*MockRepository)(nil)

// MockOrchestrator is a mock implementation of Orchestrator for testing.
type MockOrchestrator struct {
	mock.Mock
}

func (m *MockOrchestrator) StartDeployment(ctx context.Context, deploymentID uuid.UUID) error {
	args := m.Called(ctx, deploymentID)
	return args.Error(0)
}

var _ Orchestrator = (*MockOrchestrator)(nil)

// Helper to create a test router with the handler
func setupTestRouter(repo *MockRepository, orch *MockOrchestrator) *chi.Mux {
	handler := NewDeploymentHandler(repo, orch)
	r := chi.NewRouter()
	r.Mount("/api/v1/deployments", handler.Routes())
	return r
}

// --- Create Deployment Tests ---

func TestCreate_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	mockRepo.On("GetDeploymentByChainID", mock.Anything, int64(12345)).Return(nil, repository.ErrNotFound)
	mockRepo.On("CreateDeployment", mock.Anything, mock.AnythingOfType("*repository.Deployment")).Return(nil)

	router := setupTestRouter(mockRepo, mockOrch)

	body := `{"chain_id": 12345, "stack": "opstack", "config": {"name": "test"}}`
	req := httptest.NewRequest("POST", "/api/v1/deployments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.NotNil(t, resp["data"])

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(12345), data["chain_id"])
	assert.Equal(t, "opstack", data["stack"])
	assert.Equal(t, "pending", data["status"])

	mockRepo.AssertExpectations(t)
}

func TestCreate_DuplicateChainID(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	existingDeployment := &repository.Deployment{
		ID:      uuid.New(),
		ChainID: 12345,
		Stack:   repository.StackOPStack,
		Status:  repository.StatusPending,
	}
	mockRepo.On("GetDeploymentByChainID", mock.Anything, int64(12345)).Return(existingDeployment, nil)

	router := setupTestRouter(mockRepo, mockOrch)

	body := `{"chain_id": 12345, "stack": "opstack", "config": {"name": "test"}}`
	req := httptest.NewRequest("POST", "/api/v1/deployments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
	mockRepo.AssertExpectations(t)
}

func TestCreate_InvalidStack(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	router := setupTestRouter(mockRepo, mockOrch)

	body := `{"chain_id": 12345, "stack": "invalid", "config": {"name": "test"}}`
	req := httptest.NewRequest("POST", "/api/v1/deployments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreate_MissingChainID(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	router := setupTestRouter(mockRepo, mockOrch)

	body := `{"stack": "opstack", "config": {"name": "test"}}`
	req := httptest.NewRequest("POST", "/api/v1/deployments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreate_MissingConfig(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	router := setupTestRouter(mockRepo, mockOrch)

	body := `{"chain_id": 12345, "stack": "opstack"}`
	req := httptest.NewRequest("POST", "/api/v1/deployments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Get Deployment Tests ---

func TestGet_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	deploymentID := uuid.New()
	deployment := &repository.Deployment{
		ID:        deploymentID,
		ChainID:   12345,
		Stack:     repository.StackNitro,
		Status:    repository.StatusRunning,
		Config:    json.RawMessage(`{"name": "test"}`),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(deployment, nil)

	router := setupTestRouter(mockRepo, mockOrch)

	req := httptest.NewRequest("GET", "/api/v1/deployments/"+deploymentID.String(), nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, deploymentID.String(), data["id"])
	assert.Equal(t, "nitro", data["stack"])
	assert.Equal(t, "running", data["status"])

	mockRepo.AssertExpectations(t)
}

func TestGet_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	deploymentID := uuid.New()
	mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(nil, repository.ErrNotFound)

	router := setupTestRouter(mockRepo, mockOrch)

	req := httptest.NewRequest("GET", "/api/v1/deployments/"+deploymentID.String(), nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	mockRepo.AssertExpectations(t)
}

func TestGet_InvalidID(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	router := setupTestRouter(mockRepo, mockOrch)

	req := httptest.NewRequest("GET", "/api/v1/deployments/not-a-uuid", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Start Deployment Tests ---

func TestStart_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	deploymentID := uuid.New()
	deployment := &repository.Deployment{
		ID:        deploymentID,
		ChainID:   12345,
		Stack:     repository.StackOPStack,
		Status:    repository.StatusPending,
		Config:    json.RawMessage(`{}`),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(deployment, nil)
	mockRepo.On("UpdateDeploymentStatus", mock.Anything, deploymentID, repository.StatusRunning, (*string)(nil)).Return(nil)
	mockOrch.On("StartDeployment", mock.Anything, deploymentID).Return(nil)

	router := setupTestRouter(mockRepo, mockOrch)

	req := httptest.NewRequest("POST", "/api/v1/deployments/"+deploymentID.String()+"/start", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "started", data["status"])

	mockRepo.AssertExpectations(t)
	// Don't assert orchestrator expectations since it runs async
}

func TestStart_AlreadyRunning(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	deploymentID := uuid.New()
	deployment := &repository.Deployment{
		ID:        deploymentID,
		ChainID:   12345,
		Stack:     repository.StackOPStack,
		Status:    repository.StatusRunning,
		Config:    json.RawMessage(`{}`),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(deployment, nil)

	router := setupTestRouter(mockRepo, mockOrch)

	req := httptest.NewRequest("POST", "/api/v1/deployments/"+deploymentID.String()+"/start", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockRepo.AssertExpectations(t)
}

func TestStart_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	deploymentID := uuid.New()
	mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(nil, repository.ErrNotFound)

	router := setupTestRouter(mockRepo, mockOrch)

	req := httptest.NewRequest("POST", "/api/v1/deployments/"+deploymentID.String()+"/start", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	mockRepo.AssertExpectations(t)
}

// --- Get Artifacts Tests ---

func TestGetArtifacts_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	deploymentID := uuid.New()
	deployment := &repository.Deployment{
		ID:        deploymentID,
		ChainID:   12345,
		Stack:     repository.StackOPStack,
		Status:    repository.StatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	artifacts := []repository.Artifact{
		{
			ID:           uuid.New(),
			DeploymentID: deploymentID,
			ArtifactType: "genesis",
			Content:      json.RawMessage(`{"config": {}}`),
			CreatedAt:    time.Now(),
		},
		{
			ID:           uuid.New(),
			DeploymentID: deploymentID,
			ArtifactType: "rollup_config",
			Content:      json.RawMessage(`{"l1": "test"}`),
			CreatedAt:    time.Now(),
		},
	}

	mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(deployment, nil)
	mockRepo.On("GetAllArtifacts", mock.Anything, deploymentID).Return(artifacts, nil)

	router := setupTestRouter(mockRepo, mockOrch)

	req := httptest.NewRequest("GET", "/api/v1/deployments/"+deploymentID.String()+"/artifacts", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	artifactsList := data["artifacts"].([]interface{})
	assert.Len(t, artifactsList, 2)

	mockRepo.AssertExpectations(t)
}

func TestGetArtifact_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	deploymentID := uuid.New()
	artifact := &repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ArtifactType: "genesis",
		Content:      json.RawMessage(`{"config": {}}`),
		CreatedAt:    time.Now(),
	}

	mockRepo.On("GetArtifact", mock.Anything, deploymentID, "genesis").Return(artifact, nil)

	router := setupTestRouter(mockRepo, mockOrch)

	req := httptest.NewRequest("GET", "/api/v1/deployments/"+deploymentID.String()+"/artifacts/genesis", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "genesis", data["type"])

	mockRepo.AssertExpectations(t)
}

func TestGetArtifact_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	deploymentID := uuid.New()
	mockRepo.On("GetArtifact", mock.Anything, deploymentID, "nonexistent").Return(nil, repository.ErrNotFound)

	router := setupTestRouter(mockRepo, mockOrch)

	req := httptest.NewRequest("GET", "/api/v1/deployments/"+deploymentID.String()+"/artifacts/nonexistent", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	mockRepo.AssertExpectations(t)
}

// --- Get Transactions Tests ---

func TestGetTransactions_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	deploymentID := uuid.New()
	deployment := &repository.Deployment{
		ID:        deploymentID,
		ChainID:   12345,
		Stack:     repository.StackOPStack,
		Status:    repository.StatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	desc := "Deploy contracts"
	transactions := []repository.Transaction{
		{
			ID:           uuid.New(),
			DeploymentID: deploymentID,
			Stage:        "deploy-l1",
			TxHash:       "0xabc123",
			Description:  &desc,
			CreatedAt:    time.Now(),
		},
	}

	mockRepo.On("GetDeployment", mock.Anything, deploymentID).Return(deployment, nil)
	mockRepo.On("GetTransactionsByDeployment", mock.Anything, deploymentID).Return(transactions, nil)

	router := setupTestRouter(mockRepo, mockOrch)

	req := httptest.NewRequest("GET", "/api/v1/deployments/"+deploymentID.String()+"/transactions", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp["data"].([]interface{})
	assert.Len(t, data, 1)

	tx := data[0].(map[string]interface{})
	assert.Equal(t, "0xabc123", tx["tx_hash"])
	assert.Equal(t, "deploy-l1", tx["stage"])

	mockRepo.AssertExpectations(t)
}

// --- List Deployments Tests ---

func TestList_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	deployments := []*repository.Deployment{
		{
			ID:        uuid.New(),
			ChainID:   12345,
			Stack:     repository.StackOPStack,
			Status:    repository.StatusPending,
			Config:    json.RawMessage(`{}`),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	mockRepo.On("ListDeploymentsByStatus", mock.Anything, repository.StatusPending).Return(deployments, nil)
	mockRepo.On("ListDeploymentsByStatus", mock.Anything, repository.StatusRunning).Return([]*repository.Deployment{}, nil)
	mockRepo.On("ListDeploymentsByStatus", mock.Anything, repository.StatusCompleted).Return([]*repository.Deployment{}, nil)
	mockRepo.On("ListDeploymentsByStatus", mock.Anything, repository.StatusFailed).Return([]*repository.Deployment{}, nil)
	mockRepo.On("ListDeploymentsByStatus", mock.Anything, repository.StatusPaused).Return([]*repository.Deployment{}, nil)

	router := setupTestRouter(mockRepo, mockOrch)

	req := httptest.NewRequest("GET", "/api/v1/deployments", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp["data"].([]interface{})
	assert.Len(t, data, 1)

	mockRepo.AssertExpectations(t)
}

func TestList_WithStatusFilter(t *testing.T) {
	mockRepo := new(MockRepository)
	mockOrch := new(MockOrchestrator)

	deployments := []*repository.Deployment{
		{
			ID:        uuid.New(),
			ChainID:   12345,
			Stack:     repository.StackOPStack,
			Status:    repository.StatusRunning,
			Config:    json.RawMessage(`{}`),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	mockRepo.On("ListDeploymentsByStatus", mock.Anything, repository.StatusRunning).Return(deployments, nil)

	router := setupTestRouter(mockRepo, mockOrch)

	req := httptest.NewRequest("GET", "/api/v1/deployments?status=running", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp["data"].([]interface{})
	assert.Len(t, data, 1)

	mockRepo.AssertExpectations(t)
}

