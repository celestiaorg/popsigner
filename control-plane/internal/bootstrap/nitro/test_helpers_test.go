package nitro

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// MockRepository is a mock implementation of repository.Repository for testing.
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

func (m *MockRepository) GetDeploymentByChainIDAndOrg(ctx context.Context, chainID int64, orgID uuid.UUID) (*repository.Deployment, error) {
	args := m.Called(ctx, chainID, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Deployment), args.Error(1)
}

func (m *MockRepository) UpdateDeploymentStatus(ctx context.Context, id uuid.UUID, status repository.Status, stage *string) error {
	args := m.Called(ctx, id, status, stage)
	return args.Error(0)
}

func (m *MockRepository) UpdateDeploymentConfig(ctx context.Context, id uuid.UUID, config json.RawMessage) error {
	args := m.Called(ctx, id, config)
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

func (m *MockRepository) ListDeploymentsByOrg(ctx context.Context, orgID uuid.UUID) ([]*repository.Deployment, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Deployment), args.Error(1)
}

func (m *MockRepository) ListDeploymentsByOrgAndStatus(ctx context.Context, orgID uuid.UUID, status repository.Status) ([]*repository.Deployment, error) {
	args := m.Called(ctx, orgID, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Deployment), args.Error(1)
}

func (m *MockRepository) MarkStaleDeploymentsFailed(ctx context.Context, orgID uuid.UUID, timeout time.Duration) (int, error) {
	args := m.Called(ctx, orgID, timeout)
	return args.Int(0), args.Error(1)
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

// Verify MockRepository implements repository.Repository interface.
var _ repository.Repository = (*MockRepository)(nil)

// Test fixtures

func testConfig() *DeployConfig {
	return &DeployConfig{
		ChainID:           42069,
		ChainName:         "Test L3",
		ParentChainID:     421614,
		ParentChainRpc:    "https://sepolia-rollup.arbitrum.io/rpc",
		Owner:             "0x742d35Cc6634C0532925a3b844Bc454b332",
		BatchPosters:      []string{"0x742d35Cc6634C0532925a3b844Bc454b332"},
		Validators:        []string{"0x742d35Cc6634C0532925a3b844Bc454b332"},
		StakeToken:        "0x0000000000000000000000000000000000000000",
		BaseStake:         "100000000000000000",
		DataAvailability:  "celestia",
		PopsignerEndpoint: "https://rpc-mtls.popsigner.com",
		ClientCert:        "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
		ClientKey:         "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----",
	}
}

func testResult() *DeployResult {
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
			DeployedAtBlockNumber:  12345678,
		},
		TransactionHash: "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		BlockNumber:     12345678,
	}
}
