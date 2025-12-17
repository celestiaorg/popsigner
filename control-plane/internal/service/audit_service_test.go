package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Bidon15/popsigner/control-plane/internal/models"
)

// MockAuditRepository is a mock implementation of repository.AuditRepository.
type MockAuditRepository struct {
	mock.Mock
}

func (m *MockAuditRepository) Create(ctx context.Context, log *models.AuditLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

func (m *MockAuditRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AuditLog), args.Error(1)
}

func (m *MockAuditRepository) List(ctx context.Context, query models.AuditLogQuery) ([]*models.AuditLog, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AuditLog), args.Error(1)
}

func (m *MockAuditRepository) DeleteBefore(ctx context.Context, orgID uuid.UUID, before time.Time) (int64, error) {
	args := m.Called(ctx, orgID, before)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockAuditRepository) CountByOrgAndPeriod(ctx context.Context, orgID uuid.UUID, start, end time.Time) (int64, error) {
	args := m.Called(ctx, orgID, start, end)
	return args.Get(0).(int64), args.Error(1)
}

// MockOrgRepositoryForAudit is a mock implementation of repository.OrgRepository for audit tests.
type MockOrgRepositoryForAudit struct {
	mock.Mock
}

func (m *MockOrgRepositoryForAudit) Create(ctx context.Context, org *models.Organization, ownerID uuid.UUID) error {
	args := m.Called(ctx, org, ownerID)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Organization), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) GetBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Organization), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) Update(ctx context.Context, org *models.Organization) error {
	args := m.Called(ctx, org)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) AddMember(ctx context.Context, orgID, userID uuid.UUID, role models.Role, invitedBy *uuid.UUID) error {
	args := m.Called(ctx, orgID, userID, role, invitedBy)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	args := m.Called(ctx, orgID, userID)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.Role) error {
	args := m.Called(ctx, orgID, userID, role)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrgMember, error) {
	args := m.Called(ctx, orgID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OrgMember), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*models.OrgMember, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.OrgMember), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) ListUserOrgs(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Organization), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) CountMembers(ctx context.Context, orgID uuid.UUID) (int, error) {
	args := m.Called(ctx, orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) CreateNamespace(ctx context.Context, ns *models.Namespace) error {
	args := m.Called(ctx, ns)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) GetNamespace(ctx context.Context, id uuid.UUID) (*models.Namespace, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Namespace), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) GetNamespaceByName(ctx context.Context, orgID uuid.UUID, name string) (*models.Namespace, error) {
	args := m.Called(ctx, orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Namespace), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) ListNamespaces(ctx context.Context, orgID uuid.UUID) ([]*models.Namespace, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Namespace), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) DeleteNamespace(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) CountNamespaces(ctx context.Context, orgID uuid.UUID) (int, error) {
	args := m.Called(ctx, orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) CreateInvitation(ctx context.Context, inv *models.Invitation) error {
	args := m.Called(ctx, inv)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) GetInvitationByToken(ctx context.Context, token string) (*models.Invitation, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Invitation), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) GetInvitationByEmail(ctx context.Context, orgID uuid.UUID, email string) (*models.Invitation, error) {
	args := m.Called(ctx, orgID, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Invitation), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) ListPendingInvitations(ctx context.Context, orgID uuid.UUID) ([]*models.Invitation, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Invitation), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) error {
	args := m.Called(ctx, token, userID)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) DeleteInvitation(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) GetByStripeCustomer(ctx context.Context, customerID string) (*models.Organization, error) {
	args := m.Called(ctx, customerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Organization), args.Error(1)
}

func (m *MockOrgRepositoryForAudit) UpdateStripeCustomer(ctx context.Context, orgID uuid.UUID, customerID string) error {
	args := m.Called(ctx, orgID, customerID)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) UpdateStripeSubscription(ctx context.Context, orgID uuid.UUID, subscriptionID string) error {
	args := m.Called(ctx, orgID, subscriptionID)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) ClearStripeSubscription(ctx context.Context, orgID uuid.UUID) error {
	args := m.Called(ctx, orgID)
	return args.Error(0)
}

func (m *MockOrgRepositoryForAudit) UpdatePlan(ctx context.Context, orgID uuid.UUID, plan models.Plan) error {
	args := m.Called(ctx, orgID, plan)
	return args.Error(0)
}

func TestAuditService_Log(t *testing.T) {
	ctx := context.Background()
	mockAuditRepo := new(MockAuditRepository)
	mockOrgRepo := new(MockOrgRepositoryForAudit)

	svc := NewAuditService(mockAuditRepo, mockOrgRepo)

	orgID := uuid.New()
	actorID := uuid.New()
	resourceID := uuid.New()
	resourceType := models.ResourceTypeKey

	// Expect Create to be called with the log entry
	mockAuditRepo.On("Create", ctx, mock.AnythingOfType("*models.AuditLog")).Return(nil)

	err := svc.Log(ctx, AuditEntry{
		OrgID:        orgID,
		Event:        models.AuditEventKeyCreated,
		ActorID:      &actorID,
		ActorType:    models.ActorTypeUser,
		ResourceType: &resourceType,
		ResourceID:   &resourceID,
		IPAddress:    "192.168.1.1",
		UserAgent:    "Test-Agent/1.0",
		Metadata: map[string]any{
			"key_name": "test-key",
		},
	})

	require.NoError(t, err)
	mockAuditRepo.AssertExpectations(t)

	// Verify the log was created with correct values
	call := mockAuditRepo.Calls[0]
	log := call.Arguments.Get(1).(*models.AuditLog)
	assert.Equal(t, orgID, log.OrgID)
	assert.Equal(t, models.AuditEventKeyCreated, log.Event)
	assert.Equal(t, &actorID, log.ActorID)
	assert.Equal(t, models.ActorTypeUser, log.ActorType)
	assert.Equal(t, &resourceType, log.ResourceType)
	assert.Equal(t, &resourceID, log.ResourceID)
	assert.NotNil(t, log.IPAddress)
	assert.Equal(t, "192.168.1.1", log.IPAddress.String())
	assert.NotNil(t, log.UserAgent)
	assert.Equal(t, "Test-Agent/1.0", *log.UserAgent)
	assert.NotEmpty(t, log.Metadata)

	// Parse metadata
	var metadata map[string]any
	err = json.Unmarshal(log.Metadata, &metadata)
	require.NoError(t, err)
	assert.Equal(t, "test-key", metadata["key_name"])
}

func TestAuditService_Query(t *testing.T) {
	ctx := context.Background()
	mockAuditRepo := new(MockAuditRepository)
	mockOrgRepo := new(MockOrgRepositoryForAudit)

	svc := NewAuditService(mockAuditRepo, mockOrgRepo)

	orgID := uuid.New()
	now := time.Now()

	// Create test logs
	logs := []*models.AuditLog{
		{ID: uuid.New(), OrgID: orgID, Event: models.AuditEventKeyCreated, ActorType: models.ActorTypeUser, CreatedAt: now},
		{ID: uuid.New(), OrgID: orgID, Event: models.AuditEventKeySigned, ActorType: models.ActorTypeAPIKey, CreatedAt: now.Add(-time.Hour)},
		{ID: uuid.New(), OrgID: orgID, Event: models.AuditEventKeyDeleted, ActorType: models.ActorTypeUser, CreatedAt: now.Add(-2 * time.Hour)},
	}

	// Mock query with default limit + 1 to check for pagination
	mockAuditRepo.On("List", ctx, mock.AnythingOfType("models.AuditLogQuery")).Return(logs, nil)

	result, nextCursor, err := svc.Query(ctx, orgID, AuditFilter{
		Limit: 50,
	})

	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Empty(t, nextCursor) // No more pages since we got less than limit+1
	mockAuditRepo.AssertExpectations(t)
}

func TestAuditService_QueryWithPagination(t *testing.T) {
	ctx := context.Background()
	mockAuditRepo := new(MockAuditRepository)
	mockOrgRepo := new(MockOrgRepositoryForAudit)

	svc := NewAuditService(mockAuditRepo, mockOrgRepo)

	orgID := uuid.New()
	now := time.Now()

	// Create 11 test logs (more than the limit of 10 + 1)
	logs := make([]*models.AuditLog, 11)
	for i := 0; i < 11; i++ {
		logs[i] = &models.AuditLog{
			ID:        uuid.New(),
			OrgID:     orgID,
			Event:     models.AuditEventKeyCreated,
			ActorType: models.ActorTypeUser,
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
		}
	}

	mockAuditRepo.On("List", ctx, mock.AnythingOfType("models.AuditLogQuery")).Return(logs, nil)

	result, nextCursor, err := svc.Query(ctx, orgID, AuditFilter{
		Limit: 10,
	})

	require.NoError(t, err)
	assert.Len(t, result, 10)
	assert.NotEmpty(t, nextCursor) // Should have next cursor
	mockAuditRepo.AssertExpectations(t)
}

func TestAuditService_QueryWithFilters(t *testing.T) {
	ctx := context.Background()
	mockAuditRepo := new(MockAuditRepository)
	mockOrgRepo := new(MockOrgRepositoryForAudit)

	svc := NewAuditService(mockAuditRepo, mockOrgRepo)

	orgID := uuid.New()
	actorID := uuid.New()
	resourceID := uuid.New()
	event := models.AuditEventKeySigned
	resourceType := models.ResourceTypeKey
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	mockAuditRepo.On("List", ctx, mock.MatchedBy(func(q models.AuditLogQuery) bool {
		return q.OrgID == orgID &&
			q.Event != nil && *q.Event == event &&
			q.ActorID != nil && *q.ActorID == actorID &&
			q.ResourceType != nil && *q.ResourceType == resourceType &&
			q.ResourceID != nil && *q.ResourceID == resourceID
	})).Return([]*models.AuditLog{}, nil)

	_, _, err := svc.Query(ctx, orgID, AuditFilter{
		Event:        &event,
		ActorID:      &actorID,
		ResourceType: &resourceType,
		ResourceID:   &resourceID,
		StartTime:    &startTime,
		EndTime:      &endTime,
		Limit:        25,
	})

	require.NoError(t, err)
	mockAuditRepo.AssertExpectations(t)
}

func TestAuditService_GetByID(t *testing.T) {
	ctx := context.Background()
	mockAuditRepo := new(MockAuditRepository)
	mockOrgRepo := new(MockOrgRepositoryForAudit)

	svc := NewAuditService(mockAuditRepo, mockOrgRepo)

	logID := uuid.New()
	orgID := uuid.New()

	expectedLog := &models.AuditLog{
		ID:        logID,
		OrgID:     orgID,
		Event:     models.AuditEventKeyCreated,
		ActorType: models.ActorTypeUser,
		CreatedAt: time.Now(),
	}

	mockAuditRepo.On("GetByID", ctx, logID).Return(expectedLog, nil)

	result, err := svc.GetByID(ctx, logID)

	require.NoError(t, err)
	assert.Equal(t, expectedLog, result)
	mockAuditRepo.AssertExpectations(t)
}

func TestAuditService_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	mockAuditRepo := new(MockAuditRepository)
	mockOrgRepo := new(MockOrgRepositoryForAudit)

	svc := NewAuditService(mockAuditRepo, mockOrgRepo)

	logID := uuid.New()

	mockAuditRepo.On("GetByID", ctx, logID).Return(nil, nil)

	result, err := svc.GetByID(ctx, logID)

	require.NoError(t, err)
	assert.Nil(t, result)
	mockAuditRepo.AssertExpectations(t)
}

func TestLogKeyCreated_Helper(t *testing.T) {
	ctx := context.Background()
	mockAuditRepo := new(MockAuditRepository)
	mockOrgRepo := new(MockOrgRepositoryForAudit)

	svc := NewAuditService(mockAuditRepo, mockOrgRepo)

	orgID := uuid.New()
	keyID := uuid.New()
	actorID := uuid.New()

	mockAuditRepo.On("Create", ctx, mock.MatchedBy(func(log *models.AuditLog) bool {
		return log.OrgID == orgID &&
			log.Event == models.AuditEventKeyCreated &&
			*log.ActorID == actorID &&
			*log.ResourceID == keyID
	})).Return(nil)

	err := LogKeyCreated(svc, ctx, orgID, keyID, &actorID, models.ActorTypeUser, "my-key", "127.0.0.1", "TestAgent")

	require.NoError(t, err)
	mockAuditRepo.AssertExpectations(t)
}

func TestLogKeySigned_Helper(t *testing.T) {
	ctx := context.Background()
	mockAuditRepo := new(MockAuditRepository)
	mockOrgRepo := new(MockOrgRepositoryForAudit)

	svc := NewAuditService(mockAuditRepo, mockOrgRepo)

	orgID := uuid.New()
	keyID := uuid.New()
	actorID := uuid.New()

	mockAuditRepo.On("Create", ctx, mock.MatchedBy(func(log *models.AuditLog) bool {
		return log.OrgID == orgID &&
			log.Event == models.AuditEventKeySigned &&
			*log.ResourceID == keyID
	})).Return(nil)

	err := LogKeySigned(svc, ctx, orgID, keyID, &actorID, models.ActorTypeAPIKey, "10.0.0.1", "SDK/2.0")

	require.NoError(t, err)
	mockAuditRepo.AssertExpectations(t)
}
