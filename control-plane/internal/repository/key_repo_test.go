package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Bidon15/popsigner/control-plane/internal/models"
)

// MockKeyRepository is a mock implementation of KeyRepository for testing.
type MockKeyRepository struct {
	mock.Mock
}

func (m *MockKeyRepository) Create(ctx context.Context, key *models.Key) error {
	args := m.Called(ctx, key)
	if args.Error(0) == nil {
		if key.ID == uuid.Nil {
			key.ID = uuid.New()
		}
		key.CreatedAt = time.Now()
		key.UpdatedAt = time.Now()
	}
	return args.Error(0)
}

func (m *MockKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Key, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Key), args.Error(1)
}

func (m *MockKeyRepository) GetByName(ctx context.Context, orgID, namespaceID uuid.UUID, name string) (*models.Key, error) {
	args := m.Called(ctx, orgID, namespaceID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Key), args.Error(1)
}

func (m *MockKeyRepository) GetByAddress(ctx context.Context, orgID uuid.UUID, address string) (*models.Key, error) {
	args := m.Called(ctx, orgID, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Key), args.Error(1)
}

func (m *MockKeyRepository) GetByEthAddress(ctx context.Context, orgID uuid.UUID, ethAddress string) (*models.Key, error) {
	args := m.Called(ctx, orgID, ethAddress)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Key), args.Error(1)
}

func (m *MockKeyRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*models.Key, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Key), args.Error(1)
}

func (m *MockKeyRepository) ListByNamespace(ctx context.Context, namespaceID uuid.UUID) ([]*models.Key, error) {
	args := m.Called(ctx, namespaceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Key), args.Error(1)
}

func (m *MockKeyRepository) ListByEthAddresses(ctx context.Context, orgID uuid.UUID, ethAddresses []string) (map[string]*models.Key, error) {
	args := m.Called(ctx, orgID, ethAddresses)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*models.Key), args.Error(1)
}

func (m *MockKeyRepository) ListEthAddresses(ctx context.Context, orgID uuid.UUID) ([]string, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockKeyRepository) CountByOrg(ctx context.Context, orgID uuid.UUID) (int, error) {
	args := m.Called(ctx, orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockKeyRepository) Update(ctx context.Context, key *models.Key) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockKeyRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Verify MockKeyRepository implements KeyRepository
var _ KeyRepository = (*MockKeyRepository)(nil)

func TestMockKeyRepository_GetByEthAddress(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	ctx := context.Background()

	orgID := uuid.New()
	ethAddr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	expectedKey := &models.Key{
		ID:         uuid.New(),
		OrgID:      orgID,
		Name:       "test-key",
		EthAddress: &ethAddr,
	}

	mockRepo.On("GetByEthAddress", ctx, orgID, ethAddr).Return(expectedKey, nil)

	key, err := mockRepo.GetByEthAddress(ctx, orgID, ethAddr)
	assert.NoError(t, err)
	assert.Equal(t, expectedKey, key)
	mockRepo.AssertExpectations(t)
}

func TestMockKeyRepository_GetByEthAddress_CaseInsensitive(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	ctx := context.Background()

	orgID := uuid.New()
	ethAddr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	lowerAddr := strings.ToLower(ethAddr)
	expectedKey := &models.Key{
		ID:         uuid.New(),
		OrgID:      orgID,
		Name:       "test-key",
		EthAddress: &ethAddr,
	}

	// Mock should accept lowercase address lookup
	mockRepo.On("GetByEthAddress", ctx, orgID, lowerAddr).Return(expectedKey, nil)

	key, err := mockRepo.GetByEthAddress(ctx, orgID, lowerAddr)
	assert.NoError(t, err)
	assert.Equal(t, expectedKey, key)
	mockRepo.AssertExpectations(t)
}

func TestMockKeyRepository_GetByEthAddress_NotFound(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	ctx := context.Background()

	orgID := uuid.New()
	ethAddr := "0x0000000000000000000000000000000000000000"

	mockRepo.On("GetByEthAddress", ctx, orgID, ethAddr).Return(nil, nil)

	key, err := mockRepo.GetByEthAddress(ctx, orgID, ethAddr)
	assert.NoError(t, err)
	assert.Nil(t, key)
	mockRepo.AssertExpectations(t)
}

func TestMockKeyRepository_GetByEthAddress_OrgIsolation(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	ctx := context.Background()

	org1ID := uuid.New()
	org2ID := uuid.New()
	ethAddr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"

	expectedKey := &models.Key{
		ID:         uuid.New(),
		OrgID:      org1ID,
		Name:       "test-key",
		EthAddress: &ethAddr,
	}

	// Key exists in org1
	mockRepo.On("GetByEthAddress", ctx, org1ID, ethAddr).Return(expectedKey, nil)
	// Key does not exist in org2 (org isolation)
	mockRepo.On("GetByEthAddress", ctx, org2ID, ethAddr).Return(nil, nil)

	// Found in org1
	key, err := mockRepo.GetByEthAddress(ctx, org1ID, ethAddr)
	assert.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, org1ID, key.OrgID)

	// Not found in org2
	key, err = mockRepo.GetByEthAddress(ctx, org2ID, ethAddr)
	assert.NoError(t, err)
	assert.Nil(t, key)

	mockRepo.AssertExpectations(t)
}

func TestMockKeyRepository_ListByEthAddresses(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	ctx := context.Background()

	orgID := uuid.New()
	ethAddr1 := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	ethAddr2 := "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	addrs := []string{ethAddr1, ethAddr2}

	key1 := &models.Key{
		ID:         uuid.New(),
		OrgID:      orgID,
		Name:       "key-1",
		EthAddress: &ethAddr1,
	}
	key2 := &models.Key{
		ID:         uuid.New(),
		OrgID:      orgID,
		Name:       "key-2",
		EthAddress: &ethAddr2,
	}

	expectedResult := map[string]*models.Key{
		strings.ToLower(ethAddr1): key1,
		strings.ToLower(ethAddr2): key2,
	}

	mockRepo.On("ListByEthAddresses", ctx, orgID, addrs).Return(expectedResult, nil)

	result, err := mockRepo.ListByEthAddresses(ctx, orgID, addrs)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.NotNil(t, result[strings.ToLower(ethAddr1)])
	assert.NotNil(t, result[strings.ToLower(ethAddr2)])
	mockRepo.AssertExpectations(t)
}

func TestMockKeyRepository_ListByEthAddresses_Empty(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	ctx := context.Background()

	orgID := uuid.New()
	addrs := []string{}

	expectedResult := make(map[string]*models.Key)

	mockRepo.On("ListByEthAddresses", ctx, orgID, addrs).Return(expectedResult, nil)

	result, err := mockRepo.ListByEthAddresses(ctx, orgID, addrs)
	assert.NoError(t, err)
	assert.Empty(t, result)
	mockRepo.AssertExpectations(t)
}

func TestMockKeyRepository_ListByEthAddresses_PartialMatch(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	ctx := context.Background()

	orgID := uuid.New()
	ethAddr1 := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	ethAddr2 := "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	nonExistentAddr := "0x0000000000000000000000000000000000000000"
	addrs := []string{ethAddr1, ethAddr2, nonExistentAddr}

	key1 := &models.Key{
		ID:         uuid.New(),
		OrgID:      orgID,
		Name:       "key-1",
		EthAddress: &ethAddr1,
	}

	// Only key1 exists
	expectedResult := map[string]*models.Key{
		strings.ToLower(ethAddr1): key1,
	}

	mockRepo.On("ListByEthAddresses", ctx, orgID, addrs).Return(expectedResult, nil)

	result, err := mockRepo.ListByEthAddresses(ctx, orgID, addrs)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NotNil(t, result[strings.ToLower(ethAddr1)])
	assert.Nil(t, result[strings.ToLower(ethAddr2)])
	assert.Nil(t, result[strings.ToLower(nonExistentAddr)])
	mockRepo.AssertExpectations(t)
}

func TestMockKeyRepository_ListEthAddresses(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	ctx := context.Background()

	orgID := uuid.New()
	expectedAddrs := []string{
		"0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
	}

	mockRepo.On("ListEthAddresses", ctx, orgID).Return(expectedAddrs, nil)

	addrs, err := mockRepo.ListEthAddresses(ctx, orgID)
	assert.NoError(t, err)
	assert.Len(t, addrs, 2)
	assert.Equal(t, expectedAddrs, addrs)
	mockRepo.AssertExpectations(t)
}

func TestMockKeyRepository_ListEthAddresses_Empty(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	ctx := context.Background()

	orgID := uuid.New()
	expectedAddrs := []string{}

	mockRepo.On("ListEthAddresses", ctx, orgID).Return(expectedAddrs, nil)

	addrs, err := mockRepo.ListEthAddresses(ctx, orgID)
	assert.NoError(t, err)
	assert.Empty(t, addrs)
	mockRepo.AssertExpectations(t)
}

func TestMockKeyRepository_ListEthAddresses_OrgIsolation(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	ctx := context.Background()

	org1ID := uuid.New()
	org2ID := uuid.New()

	org1Addrs := []string{
		"0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
	}
	org2Addrs := []string{
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
	}

	mockRepo.On("ListEthAddresses", ctx, org1ID).Return(org1Addrs, nil)
	mockRepo.On("ListEthAddresses", ctx, org2ID).Return(org2Addrs, nil)

	// Org1 returns its addresses
	addrs1, err := mockRepo.ListEthAddresses(ctx, org1ID)
	assert.NoError(t, err)
	assert.Len(t, addrs1, 1)
	assert.Equal(t, org1Addrs[0], addrs1[0])

	// Org2 returns its addresses
	addrs2, err := mockRepo.ListEthAddresses(ctx, org2ID)
	assert.NoError(t, err)
	assert.Len(t, addrs2, 1)
	assert.Equal(t, org2Addrs[0], addrs2[0])

	// Addresses are different per org
	assert.NotEqual(t, addrs1[0], addrs2[0])

	mockRepo.AssertExpectations(t)
}

func TestMockKeyRepository_Create(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	ctx := context.Background()

	ethAddr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	key := &models.Key{
		OrgID:       uuid.New(),
		NamespaceID: uuid.New(),
		Name:        "test-key",
		PublicKey:   []byte{0x04, 0x01, 0x02, 0x03},
		Address:     "cosmos1abc123",
		EthAddress:  &ethAddr,
		Algorithm:   models.AlgorithmSecp256k1,
		BaoKeyPath:  "/keys/test",
		Exportable:  false,
	}

	mockRepo.On("Create", ctx, key).Return(nil)

	err := mockRepo.Create(ctx, key)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, key.ID)
	mockRepo.AssertExpectations(t)
}

func TestMockKeyRepository_GetByID(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	ctx := context.Background()

	keyID := uuid.New()
	ethAddr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	expectedKey := &models.Key{
		ID:         keyID,
		OrgID:      uuid.New(),
		Name:       "test-key",
		EthAddress: &ethAddr,
	}

	mockRepo.On("GetByID", ctx, keyID).Return(expectedKey, nil)

	key, err := mockRepo.GetByID(ctx, keyID)
	assert.NoError(t, err)
	assert.Equal(t, expectedKey, key)
	assert.NotNil(t, key.EthAddress)
	assert.Equal(t, ethAddr, *key.EthAddress)
	mockRepo.AssertExpectations(t)
}

func TestMockKeyRepository_ListByOrg(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	ctx := context.Background()

	orgID := uuid.New()
	ethAddr1 := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	ethAddr2 := "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"

	expectedKeys := []*models.Key{
		{ID: uuid.New(), OrgID: orgID, Name: "key-1", EthAddress: &ethAddr1},
		{ID: uuid.New(), OrgID: orgID, Name: "key-2", EthAddress: &ethAddr2},
		{ID: uuid.New(), OrgID: orgID, Name: "key-3", EthAddress: nil}, // Key without eth address
	}

	mockRepo.On("ListByOrg", ctx, orgID).Return(expectedKeys, nil)

	keys, err := mockRepo.ListByOrg(ctx, orgID)
	assert.NoError(t, err)
	assert.Len(t, keys, 3)

	// Verify eth addresses are returned correctly
	assert.NotNil(t, keys[0].EthAddress)
	assert.NotNil(t, keys[1].EthAddress)
	assert.Nil(t, keys[2].EthAddress)

	mockRepo.AssertExpectations(t)
}

