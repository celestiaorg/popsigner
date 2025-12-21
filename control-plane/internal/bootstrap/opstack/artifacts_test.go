package opstack

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
)

// mockArtifactRepository is a mock implementation of repository.Repository for testing.
type mockArtifactRepository struct {
	mock.Mock
}

func (m *mockArtifactRepository) CreateDeployment(ctx context.Context, d *repository.Deployment) error {
	args := m.Called(ctx, d)
	return args.Error(0)
}

func (m *mockArtifactRepository) GetDeployment(ctx context.Context, id uuid.UUID) (*repository.Deployment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Deployment), args.Error(1)
}

func (m *mockArtifactRepository) GetDeploymentByChainID(ctx context.Context, chainID int64) (*repository.Deployment, error) {
	args := m.Called(ctx, chainID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Deployment), args.Error(1)
}

func (m *mockArtifactRepository) UpdateDeploymentStatus(ctx context.Context, id uuid.UUID, status repository.Status, stage *string) error {
	args := m.Called(ctx, id, status, stage)
	return args.Error(0)
}

func (m *mockArtifactRepository) SetDeploymentError(ctx context.Context, id uuid.UUID, errMsg string) error {
	args := m.Called(ctx, id, errMsg)
	return args.Error(0)
}

func (m *mockArtifactRepository) ListDeploymentsByStatus(ctx context.Context, status repository.Status) ([]*repository.Deployment, error) {
	args := m.Called(ctx, status)
	return args.Get(0).([]*repository.Deployment), args.Error(1)
}

func (m *mockArtifactRepository) ListAllDeployments(ctx context.Context) ([]*repository.Deployment, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Deployment), args.Error(1)
}

func (m *mockArtifactRepository) MarkStaleDeploymentsFailed(ctx context.Context, timeout time.Duration) (int, error) {
	args := m.Called(ctx, timeout)
	return args.Int(0), args.Error(1)
}

func (m *mockArtifactRepository) UpdateDeploymentConfig(ctx context.Context, id uuid.UUID, config json.RawMessage) error {
	args := m.Called(ctx, id, config)
	return args.Error(0)
}

func (m *mockArtifactRepository) RecordTransaction(ctx context.Context, tx *repository.Transaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}

func (m *mockArtifactRepository) GetTransactionsByDeployment(ctx context.Context, deploymentID uuid.UUID) ([]repository.Transaction, error) {
	args := m.Called(ctx, deploymentID)
	return args.Get(0).([]repository.Transaction), args.Error(1)
}

func (m *mockArtifactRepository) GetTransactionByHash(ctx context.Context, hash string) (*repository.Transaction, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Transaction), args.Error(1)
}

func (m *mockArtifactRepository) SaveArtifact(ctx context.Context, a *repository.Artifact) error {
	args := m.Called(ctx, a)
	return args.Error(0)
}

func (m *mockArtifactRepository) GetArtifact(ctx context.Context, deploymentID uuid.UUID, artifactType string) (*repository.Artifact, error) {
	args := m.Called(ctx, deploymentID, artifactType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Artifact), args.Error(1)
}

func (m *mockArtifactRepository) GetAllArtifacts(ctx context.Context, deploymentID uuid.UUID) ([]repository.Artifact, error) {
	args := m.Called(ctx, deploymentID)
	return args.Get(0).([]repository.Artifact), args.Error(1)
}

func TestNewArtifactExtractor(t *testing.T) {
	mockRepo := new(mockArtifactRepository)
	extractor := NewArtifactExtractor(mockRepo)

	assert.NotNil(t, extractor)
	assert.Equal(t, mockRepo, extractor.repo)
}

func TestArtifactExtractor_ExtractArtifacts(t *testing.T) {
	ctx := context.Background()
	deploymentID := uuid.New()
	mockRepo := new(mockArtifactRepository)
	extractor := NewArtifactExtractor(mockRepo)

	cfg := &DeploymentConfig{
		ChainID:           12345,
		ChainName:         "test-chain",
		L1ChainID:         11155111,
		L1RPC:             "https://eth-sepolia.example.com",
		POPSignerEndpoint: "https://rpc.popsigner.io",
		POPSignerAPIKey:   "test_api_key",
		DeployerAddress:   "0x1234567890123456789012345678901234567890",
		BatcherAddress:    "0x2222222222222222222222222222222222222222",
		ProposerAddress:   "0x3333333333333333333333333333333333333333",
		BlockTime:         2,
		GasLimit:          30000000,
	}
	cfg.ApplyDefaults()

	// Mock genesis artifact
	genesisContent := json.RawMessage(`{"config": {"chainId": 12345}}`)
	mockRepo.On("GetArtifact", ctx, deploymentID, "genesis").Return(&repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ArtifactType: "genesis",
		Content:      genesisContent,
		CreatedAt:    time.Now(),
	}, nil)

	// Mock state artifact
	stateContent := json.RawMessage(`{
		"optimism_portal_proxy": "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"l1_cross_domain_messenger_proxy": "0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		"system_config_proxy": "0xCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
	}`)
	mockRepo.On("GetArtifact", ctx, deploymentID, "deployment_state").Return(&repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ArtifactType: "deployment_state",
		Content:      stateContent,
		CreatedAt:    time.Now(),
	}, nil)

	// Mock rollup_config artifact (not found - will be built from config)
	mockRepo.On("GetArtifact", ctx, deploymentID, "rollup_config").Return(nil, nil)

	// Mock deployment for chain ID
	mockRepo.On("GetDeployment", ctx, deploymentID).Return(&repository.Deployment{
		ID:      deploymentID,
		ChainID: 12345,
		Status:  repository.StatusCompleted,
	}, nil)

	// Mock SaveArtifact calls
	mockRepo.On("SaveArtifact", ctx, mock.AnythingOfType("*repository.Artifact")).Return(nil)

	artifacts, err := extractor.ExtractArtifacts(ctx, deploymentID, cfg)
	require.NoError(t, err)
	require.NotNil(t, artifacts)

	// Verify genesis
	assert.Equal(t, genesisContent, artifacts.Genesis)

	// Verify rollup config
	assert.NotEmpty(t, artifacts.Rollup)
	var rollup RollupConfig
	err = json.Unmarshal(artifacts.Rollup, &rollup)
	require.NoError(t, err)
	assert.Equal(t, uint64(12345), rollup.L2ChainID)
	assert.Equal(t, uint64(2), rollup.BlockTime)

	// Verify addresses
	assert.Equal(t, "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", artifacts.Addresses.OptimismPortalProxy)
	assert.Equal(t, "0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB", artifacts.Addresses.L1CrossDomainMessengerProxy)

	// Verify JWT secret is generated
	assert.NotEmpty(t, artifacts.JWTSecret)
	assert.True(t, strings.HasPrefix(artifacts.JWTSecret, "0x"))
	assert.Len(t, artifacts.JWTSecret, 66) // 0x + 64 hex chars

	// Verify docker-compose
	assert.NotEmpty(t, artifacts.DockerCompose)
	assert.Contains(t, artifacts.DockerCompose, "op-node")
	assert.Contains(t, artifacts.DockerCompose, "op-geth")
	assert.Contains(t, artifacts.DockerCompose, "op-batcher")

	// Verify env example
	assert.NotEmpty(t, artifacts.EnvExample)
	assert.Contains(t, artifacts.EnvExample, "L1_RPC_URL")
	assert.Contains(t, artifacts.EnvExample, "POPSIGNER_ENDPOINT")
	// Verify Celestia configuration is included (POPKins only supports Celestia)
	assert.Contains(t, artifacts.EnvExample, "CELESTIA_CORE_GRPC")
	assert.Contains(t, artifacts.EnvExample, "CELESTIA_KEY_NAME")

	mockRepo.AssertExpectations(t)
}

func TestArtifactExtractor_ExtractArtifacts_MissingGenesis(t *testing.T) {
	ctx := context.Background()
	deploymentID := uuid.New()
	mockRepo := new(mockArtifactRepository)
	extractor := NewArtifactExtractor(mockRepo)

	cfg := &DeploymentConfig{
		ChainID:           12345,
		ChainName:         "test-chain",
		L1ChainID:         11155111,
		L1RPC:             "https://eth-sepolia.example.com",
		POPSignerEndpoint: "https://rpc.popsigner.io",
		POPSignerAPIKey:   "test_api_key",
		DeployerAddress:   "0x1234567890123456789012345678901234567890",
	}
	cfg.ApplyDefaults()

	// Mock genesis artifact not found
	mockRepo.On("GetArtifact", ctx, deploymentID, "genesis").Return(nil, nil)

	artifacts, err := extractor.ExtractArtifacts(ctx, deploymentID, cfg)
	assert.Error(t, err)
	assert.Nil(t, artifacts)
	assert.Contains(t, err.Error(), "genesis artifact not found")

	mockRepo.AssertExpectations(t)
}

func TestArtifactExtractor_CreateBundle(t *testing.T) {
	ctx := context.Background()
	deploymentID := uuid.New()
	mockRepo := new(mockArtifactRepository)
	extractor := NewArtifactExtractor(mockRepo)

	// Mock artifacts
	artifacts := []repository.Artifact{
		{
			ID:           uuid.New(),
			DeploymentID: deploymentID,
			ArtifactType: "genesis.json",
			Content:      json.RawMessage(`{"config": {"chainId": 12345}}`),
			CreatedAt:    time.Now(),
		},
		{
			ID:           uuid.New(),
			DeploymentID: deploymentID,
			ArtifactType: "rollup.json",
			Content:      json.RawMessage(`{"l2_chain_id": 12345}`),
			CreatedAt:    time.Now(),
		},
		{
			ID:           uuid.New(),
			DeploymentID: deploymentID,
			ArtifactType: "addresses.json",
			Content:      json.RawMessage(`{"optimism_portal_proxy": "0xAAAA"}`),
			CreatedAt:    time.Now(),
		},
		{
			ID:           uuid.New(),
			DeploymentID: deploymentID,
			ArtifactType: "docker-compose.yml",
			Content:      []byte("version: '3.8'\nservices:\n  op-node:"),
			CreatedAt:    time.Now(),
		},
		{
			ID:           uuid.New(),
			DeploymentID: deploymentID,
			ArtifactType: "jwt.txt",
			Content:      []byte("0xdeadbeef"),
			CreatedAt:    time.Now(),
		},
	}

	mockRepo.On("GetAllArtifacts", ctx, deploymentID).Return(artifacts, nil)

	bundle, err := extractor.CreateBundle(ctx, deploymentID, "my-chain")
	require.NoError(t, err)
	require.NotEmpty(t, bundle)

	// Verify bundle is valid ZIP
	zr, err := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	require.NoError(t, err)

	foundFiles := make(map[string]bool)
	for _, f := range zr.File {
		foundFiles[f.Name] = true
	}

	// Verify expected files are present
	assert.True(t, foundFiles["my-chain-opstack-bundle/genesis.json"])
	assert.True(t, foundFiles["my-chain-opstack-bundle/rollup.json"])
	assert.True(t, foundFiles["my-chain-opstack-bundle/addresses.json"])
	assert.True(t, foundFiles["my-chain-opstack-bundle/docker-compose.yml"])
	assert.True(t, foundFiles["my-chain-opstack-bundle/jwt.txt"])

	mockRepo.AssertExpectations(t)
}

func TestArtifactExtractor_CreateBundle_NoArtifacts(t *testing.T) {
	ctx := context.Background()
	deploymentID := uuid.New()
	mockRepo := new(mockArtifactRepository)
	extractor := NewArtifactExtractor(mockRepo)

	mockRepo.On("GetAllArtifacts", ctx, deploymentID).Return([]repository.Artifact{}, nil)

	bundle, err := extractor.CreateBundle(ctx, deploymentID, "my-chain")
	assert.Error(t, err)
	assert.Nil(t, bundle)
	assert.Contains(t, err.Error(), "no artifacts found")

	mockRepo.AssertExpectations(t)
}

func TestArtifactExtractor_GetArtifact(t *testing.T) {
	ctx := context.Background()
	deploymentID := uuid.New()
	mockRepo := new(mockArtifactRepository)
	extractor := NewArtifactExtractor(mockRepo)

	expectedContent := json.RawMessage(`{"test": "data"}`)
	mockRepo.On("GetArtifact", ctx, deploymentID, "genesis.json").Return(&repository.Artifact{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ArtifactType: "genesis.json",
		Content:      expectedContent,
		CreatedAt:    time.Now(),
	}, nil)

	content, err := extractor.GetArtifact(ctx, deploymentID, "genesis.json")
	require.NoError(t, err)
	assert.Equal(t, []byte(expectedContent), content)

	mockRepo.AssertExpectations(t)
}

func TestArtifactExtractor_GetArtifact_NotFound(t *testing.T) {
	ctx := context.Background()
	deploymentID := uuid.New()
	mockRepo := new(mockArtifactRepository)
	extractor := NewArtifactExtractor(mockRepo)

	mockRepo.On("GetArtifact", ctx, deploymentID, "missing.json").Return(nil, nil)

	content, err := extractor.GetArtifact(ctx, deploymentID, "missing.json")
	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "artifact missing.json not found")

	mockRepo.AssertExpectations(t)
}

func TestArtifactExtractor_ListArtifacts(t *testing.T) {
	ctx := context.Background()
	deploymentID := uuid.New()
	mockRepo := new(mockArtifactRepository)
	extractor := NewArtifactExtractor(mockRepo)

	artifacts := []repository.Artifact{
		{ArtifactType: "genesis.json"},
		{ArtifactType: "rollup.json"},
		{ArtifactType: "deployment_state"}, // Should be filtered out
		{ArtifactType: "addresses.json"},
	}

	mockRepo.On("GetAllArtifacts", ctx, deploymentID).Return(artifacts, nil)

	types, err := extractor.ListArtifacts(ctx, deploymentID)
	require.NoError(t, err)
	assert.Len(t, types, 3)
	assert.Contains(t, types, "genesis.json")
	assert.Contains(t, types, "rollup.json")
	assert.Contains(t, types, "addresses.json")
	assert.NotContains(t, types, "deployment_state")

	mockRepo.AssertExpectations(t)
}

func TestGenerateJWTSecret(t *testing.T) {
	secret1 := generateJWTSecret()
	secret2 := generateJWTSecret()

	// Should be 66 characters (0x + 64 hex chars = 32 bytes)
	assert.Len(t, secret1, 66)
	assert.Len(t, secret2, 66)

	// Should start with 0x
	assert.True(t, strings.HasPrefix(secret1, "0x"))
	assert.True(t, strings.HasPrefix(secret2, "0x"))

	// Should be unique
	assert.NotEqual(t, secret1, secret2)
}

func TestCalculateBatchInboxAddress(t *testing.T) {
	tests := []struct {
		chainID  uint64
		expected string
	}{
		{1, "0xff0000000000000000000000000000000000000001"},
		{12345, "0xff0000000000000000000000000000000000003039"},
		{420, "0xff00000000000000000000000000000000000001a4"},
	}

	for _, tc := range tests {
		t.Run(string(rune(tc.chainID)), func(t *testing.T) {
			result := calculateBatchInboxAddress(tc.chainID)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGenerateDockerCompose(t *testing.T) {
	cfg := &DeploymentConfig{
		ChainID:           12345,
		ChainName:         "test-chain",
		L1ChainID:         11155111,
		L1RPC:             "https://eth-sepolia.example.com",
		POPSignerEndpoint: "https://rpc.popsigner.io",
		POPSignerAPIKey:   "test_key",
		DeployerAddress:   "0x1234567890123456789012345678901234567890",
		BatcherAddress:    "0x2222222222222222222222222222222222222222",
		ProposerAddress:   "0x3333333333333333333333333333333333333333",
		CelestiaRPC:       "https://celestia.example.com", // Required for Celestia DA
	}
	cfg.ApplyDefaults()

	artifacts := &OPStackArtifacts{
		Addresses: ContractAddresses{
			L2OutputOracle: "0x4444444444444444444444444444444444444444",
		},
	}

	compose, err := GenerateDockerCompose(cfg, artifacts)
	require.NoError(t, err)
	assert.NotEmpty(t, compose)

	// Verify services
	assert.Contains(t, compose, "op-node:")
	assert.Contains(t, compose, "op-geth:")
	assert.Contains(t, compose, "op-batcher:")
	assert.Contains(t, compose, "op-proposer:")

	// Verify POPSigner integration via command-line flags
	assert.Contains(t, compose, "--signer.endpoint=${POPSIGNER_ENDPOINT}")
	assert.Contains(t, compose, "--signer.address=${BATCHER_ADDRESS}")
	assert.Contains(t, compose, "--signer.address=${PROPOSER_ADDRESS}")

	// Verify Celestia Alt-DA configuration (always enabled for POPKins)
	assert.Contains(t, compose, "op-alt-da:")
	assert.Contains(t, compose, "--altda.enabled=true")
	assert.Contains(t, compose, "--altda.da-service=http://op-alt-da:3100")

	// Verify network name
	assert.Contains(t, compose, "opstack-test-chain")
}

func TestGenerateDockerCompose_WithAltDA(t *testing.T) {
	cfg := &DeploymentConfig{
		ChainID:           12345,
		ChainName:         "test-chain",
		L1ChainID:         11155111,
		L1RPC:             "https://eth-sepolia.example.com",
		POPSignerEndpoint: "https://rpc.popsigner.io",
		POPSignerAPIKey:   "test_key",
		DeployerAddress:   "0x1234567890123456789012345678901234567890",
		CelestiaRPC:       "https://celestia.example.com",
	}
	cfg.ApplyDefaults()

	compose, err := GenerateDockerCompose(cfg, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, compose)

	// Verify Celestia Alt-DA service is included (POPKins always uses Celestia)
	assert.Contains(t, compose, "op-alt-da:")
	assert.Contains(t, compose, "config.toml")
	assert.Contains(t, compose, "--altda.enabled=true")
	assert.Contains(t, compose, "--altda.da-service=http://op-alt-da:3100")
}

func TestSanitizeChainName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test-chain", "test-chain"},
		{"Test Chain", "Test-Chain"},
		{"my_chain_123", "my_chain_123"},
		{"chain!@#$%", "chain"},
		{"", "opstack"},
		{"   ", "---"}, // spaces become hyphens
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := sanitizeChainName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGenerateEnvExample(t *testing.T) {
	cfg := &DeploymentConfig{
		ChainID:           12345,
		ChainName:         "test-chain",
		L1RPC:             "https://eth-sepolia.example.com",
		POPSignerEndpoint: "https://rpc.popsigner.io",
		BatcherAddress:    "0x2222222222222222222222222222222222222222",
		ProposerAddress:   "0x3333333333333333333333333333333333333333",
		SequencerAddress:  "0x1111111111111111111111111111111111111111",
		CelestiaRPC:       "https://celestia.example.com",
	}

	addrs := &ContractAddresses{
		L2OutputOracle: "0x4444444444444444444444444444444444444444",
	}

	env := GenerateEnvExample(cfg, addrs)

	// Verify L1 and POPSigner configuration
	assert.Contains(t, env, "L1_RPC_URL=https://eth-sepolia.example.com")
	assert.Contains(t, env, "POPSIGNER_ENDPOINT=https://rpc.popsigner.io")
	assert.Contains(t, env, "BATCHER_ADDRESS=0x2222222222222222222222222222222222222222")
	assert.Contains(t, env, "PROPOSER_ADDRESS=0x3333333333333333333333333333333333333333")
	assert.Contains(t, env, "CHAIN_ID=12345")

	// Verify Celestia configuration (POPKins always uses Celestia)
	assert.Contains(t, env, "CELESTIA_CORE_GRPC")
	assert.Contains(t, env, "CELESTIA_KEY_NAME")
	assert.Contains(t, env, "CELESTIA_NETWORK")
	assert.Contains(t, env, "CELESTIA_NAMESPACE")
}

func TestContractAddresses_JSONMarshaling(t *testing.T) {
	addrs := ContractAddresses{
		OptimismPortalProxy:         "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		L1CrossDomainMessengerProxy: "0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		SystemConfigProxy:           "0xCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC",
		BatchInbox:                  "0xff0000000000000000000000000000000000003039",
	}

	data, err := json.Marshal(addrs)
	require.NoError(t, err)

	var decoded ContractAddresses
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, addrs.OptimismPortalProxy, decoded.OptimismPortalProxy)
	assert.Equal(t, addrs.L1CrossDomainMessengerProxy, decoded.L1CrossDomainMessengerProxy)
	assert.Equal(t, addrs.SystemConfigProxy, decoded.SystemConfigProxy)
	assert.Equal(t, addrs.BatchInbox, decoded.BatchInbox)
}

func TestRollupConfig_JSONMarshaling(t *testing.T) {
	zero := uint64(0)
	rollup := RollupConfig{
		Genesis: RollupGenesisConfig{
			L1: GenesisBlockRef{
				Hash:   "0x1234",
				Number: 100,
			},
			L2: GenesisBlockRef{
				Hash:   "0x5678",
				Number: 0,
			},
			L2Time: 1700000000,
			SystemConfig: SystemConfig{
				BatcherAddr: "0xBATCHER",
				GasLimit:    30000000,
			},
		},
		BlockTime:           2,
		MaxSequencerDrift:   600,
		SequencerWindowSize: 3600,
		ChannelTimeout:      300,
		L1ChainID:           11155111,
		L2ChainID:           12345,
		RegolithTime:        &zero,
		BatchInboxAddress:   "0xff0000000000000000000000000000000000003039",
		DepositContractAddr: "0xDEPOSIT",
		L1SystemConfigAddr:  "0xSYSCONFIG",
	}

	data, err := json.Marshal(rollup)
	require.NoError(t, err)

	var decoded RollupConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, rollup.L1ChainID, decoded.L1ChainID)
	assert.Equal(t, rollup.L2ChainID, decoded.L2ChainID)
	assert.Equal(t, rollup.BlockTime, decoded.BlockTime)
	assert.Equal(t, rollup.Genesis.L1.Hash, decoded.Genesis.L1.Hash)
	assert.Equal(t, rollup.Genesis.SystemConfig.BatcherAddr, decoded.Genesis.SystemConfig.BatcherAddr)
}

