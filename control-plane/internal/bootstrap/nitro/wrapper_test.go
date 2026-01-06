package nitro

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// Mock repository for testing
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
			NativeToken:            "0x0000000000000000000000000000000000000000",
			DeployedAtBlockNumber:  12345678,
		},
		TransactionHash: "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		BlockNumber:     12345678,
	}
}

func TestNewDeployer(t *testing.T) {
	t.Run("creates deployer with defaults", func(t *testing.T) {
		d := NewDeployer("/path/to/worker")

		assert.Equal(t, "/path/to/worker", d.workerPath)
		assert.Equal(t, "node", d.nodeCmd)
		assert.NotNil(t, d.logger)
	})

	t.Run("applies options", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		mockRepo := new(MockRepository)

		d := NewDeployer("/path/to/worker",
			WithNodeCommand("custom-node"),
			WithLogger(logger),
			WithRepository(mockRepo),
		)

		assert.Equal(t, "custom-node", d.nodeCmd)
		assert.Equal(t, logger, d.logger)
		assert.Equal(t, mockRepo, d.repo)
	})
}

func TestDeployConfig_Serialization(t *testing.T) {
	t.Run("serializes to JSON correctly", func(t *testing.T) {
		config := testConfig()

		data, err := json.Marshal(config)
		require.NoError(t, err)

		// Verify key fields are present
		var parsed map[string]interface{}
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, float64(42069), parsed["chainId"])
		assert.Equal(t, "Test L3", parsed["chainName"])
		assert.Equal(t, float64(421614), parsed["parentChainId"])
		assert.Equal(t, "celestia", parsed["dataAvailability"])
		assert.Equal(t, "https://rpc-mtls.popsigner.com", parsed["popsignerEndpoint"])
	})

	t.Run("omits empty optional fields", func(t *testing.T) {
		config := &DeployConfig{
			ChainID:           42069,
			ChainName:         "Test",
			ParentChainID:     421614,
			ParentChainRpc:    "https://rpc.example.com",
			Owner:             "0x123",
			DataAvailability:  "celestia",
			PopsignerEndpoint: "https://rpc-mtls.popsigner.com",
			ClientCert:        "cert",
			ClientKey:         "key",
			// NativeToken is empty - should be omitted
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		_, hasNativeToken := parsed["nativeToken"]
		assert.False(t, hasNativeToken, "empty nativeToken should be omitted")
	})
}

func TestDeployResult_Serialization(t *testing.T) {
	t.Run("deserializes successful result", func(t *testing.T) {
		resultJSON := `{
			"success": true,
			"coreContracts": {
				"rollup": "0x1234",
				"inbox": "0x2345",
				"outbox": "0x3456",
				"bridge": "0x4567",
				"sequencerInbox": "0x5678",
				"rollupEventInbox": "0x6789",
				"challengeManager": "0x7890",
				"adminProxy": "0x8901",
				"upgradeExecutor": "0x9012",
				"validatorWalletCreator": "0x0123",
				"nativeToken": "0x0000",
				"deployedAtBlockNumber": 12345
			},
			"transactionHash": "0xabcd",
			"blockNumber": 12345
		}`

		var result DeployResult
		err := json.Unmarshal([]byte(resultJSON), &result)
		require.NoError(t, err)

		assert.True(t, result.Success)
		assert.NotNil(t, result.CoreContracts)
		assert.Equal(t, "0x1234", result.CoreContracts.Rollup)
		assert.Equal(t, "0xabcd", result.TransactionHash)
		assert.Equal(t, int64(12345), result.BlockNumber)
	})

	t.Run("deserializes failed result", func(t *testing.T) {
		resultJSON := `{
			"success": false,
			"error": "deployment failed: insufficient funds"
		}`

		var result DeployResult
		err := json.Unmarshal([]byte(resultJSON), &result)
		require.NoError(t, err)

		assert.False(t, result.Success)
		assert.Equal(t, "deployment failed: insufficient funds", result.Error)
		assert.Nil(t, result.CoreContracts)
	})
}

func TestCertificateBundle(t *testing.T) {
	t.Run("reads certificates from files", func(t *testing.T) {
		// Create temp directory
		tmpDir := t.TempDir()

		// Write test files
		certContent := "-----BEGIN CERTIFICATE-----\ntest-cert\n-----END CERTIFICATE-----"
		keyContent := "-----BEGIN PRIVATE KEY-----\ntest-key\n-----END PRIVATE KEY-----"
		caContent := "-----BEGIN CERTIFICATE-----\ntest-ca\n-----END CERTIFICATE-----"

		certPath := filepath.Join(tmpDir, "client.crt")
		keyPath := filepath.Join(tmpDir, "client.key")
		caPath := filepath.Join(tmpDir, "ca.crt")

		require.NoError(t, os.WriteFile(certPath, []byte(certContent), 0600))
		require.NoError(t, os.WriteFile(keyPath, []byte(keyContent), 0600))
		require.NoError(t, os.WriteFile(caPath, []byte(caContent), 0600))

		// Read bundle
		bundle, err := ReadCertificateBundle(certPath, keyPath, caPath)
		require.NoError(t, err)

		assert.Equal(t, certContent, bundle.ClientCert)
		assert.Equal(t, keyContent, bundle.ClientKey)
		assert.Equal(t, caContent, bundle.CaCert)
	})

	t.Run("reads certificates without CA", func(t *testing.T) {
		tmpDir := t.TempDir()

		certContent := "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"
		keyContent := "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----"

		certPath := filepath.Join(tmpDir, "client.crt")
		keyPath := filepath.Join(tmpDir, "client.key")

		require.NoError(t, os.WriteFile(certPath, []byte(certContent), 0600))
		require.NoError(t, os.WriteFile(keyPath, []byte(keyContent), 0600))

		bundle, err := ReadCertificateBundle(certPath, keyPath, "")
		require.NoError(t, err)

		assert.Equal(t, certContent, bundle.ClientCert)
		assert.Equal(t, keyContent, bundle.ClientKey)
		assert.Empty(t, bundle.CaCert)
	})

	t.Run("fails on missing cert file", func(t *testing.T) {
		_, err := ReadCertificateBundle("/nonexistent/client.crt", "/nonexistent/client.key", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read client cert")
	})

	t.Run("fails on missing key file", func(t *testing.T) {
		tmpDir := t.TempDir()
		certPath := filepath.Join(tmpDir, "client.crt")
		require.NoError(t, os.WriteFile(certPath, []byte("cert"), 0600))

		_, err := ReadCertificateBundle(certPath, "/nonexistent/client.key", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read client key")
	})
}

func TestWriteCertificatesToDir(t *testing.T) {
	t.Run("writes all certificates", func(t *testing.T) {
		tmpDir := t.TempDir()

		bundle := &CertificateBundle{
			ClientCert: "cert-content",
			ClientKey:  "key-content",
			CaCert:     "ca-content",
		}

		certPath, keyPath, caPath, err := WriteCertificatesToDir(tmpDir, bundle)
		require.NoError(t, err)

		// Verify files exist and have correct content
		cert, err := os.ReadFile(certPath)
		require.NoError(t, err)
		assert.Equal(t, "cert-content", string(cert))

		key, err := os.ReadFile(keyPath)
		require.NoError(t, err)
		assert.Equal(t, "key-content", string(key))

		ca, err := os.ReadFile(caPath)
		require.NoError(t, err)
		assert.Equal(t, "ca-content", string(ca))
	})

	t.Run("skips CA if empty", func(t *testing.T) {
		tmpDir := t.TempDir()

		bundle := &CertificateBundle{
			ClientCert: "cert-content",
			ClientKey:  "key-content",
			CaCert:     "", // Empty
		}

		certPath, keyPath, caPath, err := WriteCertificatesToDir(tmpDir, bundle)
		require.NoError(t, err)

		assert.NotEmpty(t, certPath)
		assert.NotEmpty(t, keyPath)
		assert.Empty(t, caPath)

		// Verify cert and key exist
		_, err = os.Stat(certPath)
		assert.NoError(t, err)

		_, err = os.Stat(keyPath)
		assert.NoError(t, err)
	})
}

func TestBuildConfigFromDeployment(t *testing.T) {
	t.Run("builds config from deployment", func(t *testing.T) {
		deploymentConfig := map[string]interface{}{
			"chainId":           42069,
			"chainName":         "Test L3",
			"parentChainId":     421614,
			"parentChainRpc":    "https://rpc.example.com",
			"owner":             "0x123",
			"batchPosters":      []string{"0x456"},
			"validators":        []string{"0x789"},
			"stakeToken":        "0x000",
			"baseStake":         "100000000000000000",
			"dataAvailability":  "anytrust",
			"popsignerEndpoint": "https://rpc-mtls.popsigner.com",
		}

		configJSON, err := json.Marshal(deploymentConfig)
		require.NoError(t, err)

		deployment := &repository.Deployment{
			ID:      uuid.New(),
			ChainID: 42069,
			Stack:   repository.StackNitro,
			Status:  repository.StatusPending,
			Config:  configJSON,
		}

		certs := CertificateBundle{
			ClientCert: "cert",
			ClientKey:  "key",
			CaCert:     "ca",
		}

		config, err := BuildConfigFromDeployment(deployment, certs)
		require.NoError(t, err)

		assert.Equal(t, int64(42069), config.ChainID)
		assert.Equal(t, "Test L3", config.ChainName)
		assert.Equal(t, "cert", config.ClientCert)
		assert.Equal(t, "key", config.ClientKey)
		assert.Equal(t, "ca", config.CaCert)
	})

	t.Run("fails on invalid JSON", func(t *testing.T) {
		deployment := &repository.Deployment{
			Config: json.RawMessage("invalid json"),
		}

		certs := CertificateBundle{}

		_, err := BuildConfigFromDeployment(deployment, certs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal deployment config")
	})
}

func TestDeployWithPersistence(t *testing.T) {
	t.Run("requires repository", func(t *testing.T) {
		d := NewDeployer("/path/to/worker")
		// No repository set

		ctx := context.Background()
		deploymentID := uuid.New()

		_, err := d.DeployWithPersistence(ctx, deploymentID, testConfig())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository not configured")
	})
}

func TestDeployWithRetry(t *testing.T) {
	t.Run("respects context cancellation", func(t *testing.T) {
		d := NewDeployer("/nonexistent/path")

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := d.DeployWithRetry(ctx, testConfig(), 3)
		assert.Error(t, err)
		// Should fail fast due to context cancellation or command failure
	})
}

func TestSaveArtifacts(t *testing.T) {
	t.Run("saves core contracts artifact", func(t *testing.T) {
		mockRepo := new(MockRepository)
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

		d := NewDeployer("/path/to/worker",
			WithRepository(mockRepo),
			WithLogger(logger),
		)

		ctx := context.Background()
		deploymentID := uuid.New()
		result := testResult()

		// Expect SaveArtifact to be called twice (core_contracts and chain_config if present)
		mockRepo.On("SaveArtifact", ctx, mock.AnythingOfType("*repository.Artifact")).Return(nil)

		err := d.saveArtifacts(ctx, deploymentID, result)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("saves chain config artifact", func(t *testing.T) {
		mockRepo := new(MockRepository)
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

		d := NewDeployer("/path/to/worker",
			WithRepository(mockRepo),
			WithLogger(logger),
		)

		ctx := context.Background()
		deploymentID := uuid.New()
		result := testResult()
		result.ChainConfig = map[string]interface{}{
			"chainId": 42069,
			"arbitrum": map[string]interface{}{
				"InitialChainOwner": "0x123",
			},
		}

		// Expect SaveArtifact to be called for both artifacts
		mockRepo.On("SaveArtifact", ctx, mock.AnythingOfType("*repository.Artifact")).Return(nil).Times(2)

		err := d.saveArtifacts(ctx, deploymentID, result)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})
}

func TestDeployWithPersistenceAndProgress(t *testing.T) {
	t.Run("calls progress callback", func(t *testing.T) {
		mockRepo := new(MockRepository)
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

		d := NewDeployer("/nonexistent/path",
			WithRepository(mockRepo),
			WithLogger(logger),
		)

		ctx := context.Background()
		deploymentID := uuid.New()

		// Set up mock expectations for status updates
		mockRepo.On("UpdateDeploymentStatus", ctx, deploymentID, repository.StatusRunning, mock.AnythingOfType("*string")).Return(nil)
		mockRepo.On("UpdateDeploymentStatus", ctx, deploymentID, repository.StatusFailed, mock.Anything).Return(nil)
		mockRepo.On("SetDeploymentError", ctx, deploymentID, mock.AnythingOfType("string")).Return(nil)

		// Track progress callbacks
		progressCalls := []string{}
		onProgress := func(stage string, progress float64, message string) {
			progressCalls = append(progressCalls, stage)
		}

		// Deploy will fail (no TS worker), but we should still get initial progress
		_, err := d.DeployWithPersistenceAndProgress(ctx, deploymentID, testConfig(), onProgress)
		assert.Error(t, err) // Expected to fail since worker doesn't exist

		// Should have called progress at least once for the initial stage
		assert.True(t, len(progressCalls) > 0, "should have received at least one progress callback")
		assert.Contains(t, progressCalls, "deploying")
	})

	t.Run("requires repository", func(t *testing.T) {
		d := NewDeployer("/path/to/worker")
		// No repository set

		ctx := context.Background()
		deploymentID := uuid.New()

		_, err := d.DeployWithPersistenceAndProgress(ctx, deploymentID, testConfig(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository not configured")
	})
}

func TestExecuteWorkerBuildCheck(t *testing.T) {
	t.Run("fails if TypeScript not built", func(t *testing.T) {
		// Create a temp directory without the dist/cli.js
		tmpDir := t.TempDir()

		d := NewDeployer(tmpDir)

		ctx := context.Background()
		_, err := d.executeWorker(ctx, testConfig())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TypeScript worker not built")
		assert.Contains(t, err.Error(), "npm run build")
	})
}

// Verify MockRepository implements the interface
var _ repository.Repository = (*MockRepository)(nil)

