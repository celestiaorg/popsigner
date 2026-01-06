package nitro

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// MockCertificateProvider is a mock implementation of CertificateProvider.
type MockCertificateProvider struct {
	mock.Mock
}

func (m *MockCertificateProvider) GetCertificates(ctx context.Context, orgID uuid.UUID) (*CertificateBundle, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*CertificateBundle), args.Error(1)
}

func testNitroDeploymentConfig() *NitroDeploymentConfigRaw {
	return &NitroDeploymentConfigRaw{
		OrgID:            uuid.New().String(),
		ChainID:          42170,
		ChainName:        "test-orbit-chain",
		ParentChainID:    42161,
		ParentChainRpc:   "https://arb1.arbitrum.io/rpc",
		DeployerAddress:  "0x742d35Cc6634C0532925a3b844Bc454b332",
		BatchPosters:     []string{"0x742d35Cc6634C0532925a3b844Bc454b332"},
		Validators:       []string{"0x742d35Cc6634C0532925a3b844Bc454b332"},
		StakeToken:       "0x0000000000000000000000000000000000000000",
		BaseStake:        "100000000000000000",
		DataAvailability: "celestia",
	}
}

func TestNewOrchestrator(t *testing.T) {
	t.Run("creates orchestrator with defaults", func(t *testing.T) {
		mockRepo := new(MockRepository)

		orch := NewOrchestrator(mockRepo, nil, OrchestratorConfig{})

		assert.NotNil(t, orch)
		assert.Equal(t, "https://rpc-mtls.popsigner.com", orch.config.POPSignerMTLSEndpoint)
		assert.Equal(t, 3, orch.config.RetryAttempts)
		assert.Equal(t, 5*time.Second, orch.config.RetryDelay)
	})

	t.Run("applies custom config", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockCertProvider := new(MockCertificateProvider)

		orch := NewOrchestrator(mockRepo, mockCertProvider, OrchestratorConfig{
			WorkerPath:            "/custom/path",
			POPSignerMTLSEndpoint: "https://custom.popsigner.com:8546",
			RetryAttempts:         5,
			RetryDelay:            10 * time.Second,
		})

		assert.NotNil(t, orch)
		assert.Equal(t, "https://custom.popsigner.com:8546", orch.config.POPSignerMTLSEndpoint)
		assert.Equal(t, 5, orch.config.RetryAttempts)
		assert.Equal(t, 10*time.Second, orch.config.RetryDelay)
	})
}

func TestOrchestrator_Deploy(t *testing.T) {
	t.Run("fails if deployment not found", func(t *testing.T) {
		mockRepo := new(MockRepository)

		orch := NewOrchestrator(mockRepo, nil, OrchestratorConfig{})

		ctx := context.Background()
		deploymentID := uuid.New()

		mockRepo.On("GetDeployment", ctx, deploymentID).Return(nil, nil)
		mockRepo.On("SetDeploymentError", ctx, deploymentID, mock.AnythingOfType("string")).Return(nil)
		mockRepo.On("UpdateDeploymentStatus", ctx, deploymentID, repository.StatusFailed, mock.Anything).Return(nil)

		err := orch.Deploy(ctx, deploymentID, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "deployment not found")
	})

	t.Run("fails if config cannot be parsed", func(t *testing.T) {
		mockRepo := new(MockRepository)

		orch := NewOrchestrator(mockRepo, nil, OrchestratorConfig{})

		ctx := context.Background()
		deploymentID := uuid.New()

		deployment := &repository.Deployment{
			ID:     deploymentID,
			Stack:  repository.StackNitro,
			Config: json.RawMessage(`invalid json`),
		}

		mockRepo.On("GetDeployment", ctx, deploymentID).Return(deployment, nil)
		mockRepo.On("SetDeploymentError", ctx, deploymentID, mock.AnythingOfType("string")).Return(nil)
		mockRepo.On("UpdateDeploymentStatus", ctx, deploymentID, repository.StatusFailed, mock.Anything).Return(nil)

		err := orch.Deploy(ctx, deploymentID, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parse deployment config")
	})

	t.Run("fails if mTLS certificates not available", func(t *testing.T) {
		mockRepo := new(MockRepository)

		orch := NewOrchestrator(mockRepo, nil, OrchestratorConfig{})

		ctx := context.Background()
		deploymentID := uuid.New()

		config := testNitroDeploymentConfig()
		config.ClientCert = "" // No certs
		config.ClientKey = ""
		configJSON, _ := json.Marshal(config)

		deployment := &repository.Deployment{
			ID:     deploymentID,
			Stack:  repository.StackNitro,
			Config: configJSON,
		}

		mockRepo.On("GetDeployment", ctx, deploymentID).Return(deployment, nil)
		mockRepo.On("SetDeploymentError", ctx, deploymentID, mock.AnythingOfType("string")).Return(nil)
		mockRepo.On("UpdateDeploymentStatus", ctx, deploymentID, repository.StatusFailed, mock.Anything).Return(nil)

		err := orch.Deploy(ctx, deploymentID, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mTLS certificates not available")
	})

	t.Run("uses certificates from provider", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockCertProvider := new(MockCertificateProvider)

		orch := NewOrchestrator(mockRepo, mockCertProvider, OrchestratorConfig{
			WorkerPath: "/nonexistent/path", // Will fail deployment, but tests cert flow
		})

		ctx := context.Background()
		deploymentID := uuid.New()
		orgID := uuid.New()

		config := testNitroDeploymentConfig()
		config.OrgID = orgID.String()
		configJSON, _ := json.Marshal(config)

		deployment := &repository.Deployment{
			ID:     deploymentID,
			Stack:  repository.StackNitro,
			Config: configJSON,
		}

		certs := &CertificateBundle{
			ClientCert: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
			ClientKey:  "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----",
		}

		mockRepo.On("GetDeployment", ctx, deploymentID).Return(deployment, nil)
		mockCertProvider.On("GetCertificates", ctx, orgID).Return(certs, nil)
		mockRepo.On("UpdateDeploymentStatus", ctx, deploymentID, repository.StatusRunning, mock.AnythingOfType("*string")).Return(nil)
		mockRepo.On("SetDeploymentError", ctx, deploymentID, mock.AnythingOfType("string")).Return(nil)
		mockRepo.On("UpdateDeploymentStatus", ctx, deploymentID, repository.StatusFailed, mock.Anything).Return(nil)

		err := orch.Deploy(ctx, deploymentID, nil)
		// Will fail because worker doesn't exist, but certificates were fetched
		assert.Error(t, err)

		mockCertProvider.AssertCalled(t, "GetCertificates", ctx, orgID)
	})

	t.Run("calls progress callback", func(t *testing.T) {
		mockRepo := new(MockRepository)

		orch := NewOrchestrator(mockRepo, nil, OrchestratorConfig{})

		ctx := context.Background()
		deploymentID := uuid.New()

		config := testNitroDeploymentConfig()
		config.ClientCert = "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"
		config.ClientKey = "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----"
		configJSON, _ := json.Marshal(config)

		deployment := &repository.Deployment{
			ID:     deploymentID,
			Stack:  repository.StackNitro,
			Config: configJSON,
		}

		mockRepo.On("GetDeployment", ctx, deploymentID).Return(deployment, nil)
		mockRepo.On("UpdateDeploymentStatus", ctx, deploymentID, mock.Anything, mock.Anything).Return(nil)
		mockRepo.On("SetDeploymentError", ctx, deploymentID, mock.AnythingOfType("string")).Return(nil)

		progressCalls := []string{}
		onProgress := func(stage string, progress float64, message string) {
			progressCalls = append(progressCalls, stage)
		}

		// Will fail because worker doesn't exist
		_ = orch.Deploy(ctx, deploymentID, onProgress)

		// Should have received progress callbacks
		assert.True(t, len(progressCalls) > 0, "should have received progress callbacks")
		assert.Contains(t, progressCalls, "init")
	})
}

func TestNitroDeploymentConfigRaw(t *testing.T) {
	t.Run("serializes and deserializes correctly", func(t *testing.T) {
		config := testNitroDeploymentConfig()

		jsonBytes, err := json.Marshal(config)
		require.NoError(t, err)

		var parsed NitroDeploymentConfigRaw
		err = json.Unmarshal(jsonBytes, &parsed)
		require.NoError(t, err)

		assert.Equal(t, config.ChainID, parsed.ChainID)
		assert.Equal(t, config.ChainName, parsed.ChainName)
		assert.Equal(t, config.DataAvailability, parsed.DataAvailability)
	})
}

// Verify MockCertificateProvider implements the interface
var _ CertificateProvider = (*MockCertificateProvider)(nil)

