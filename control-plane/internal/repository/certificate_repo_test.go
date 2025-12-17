package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Bidon15/popsigner/control-plane/internal/models"
)

// MockCertificateRepository is a mock implementation of CertificateRepository for testing.
type MockCertificateRepository struct {
	mock.Mock
}

func (m *MockCertificateRepository) Create(ctx context.Context, cert *models.Certificate) error {
	args := m.Called(ctx, cert)
	if args.Error(0) == nil && cert.CreatedAt.IsZero() {
		cert.CreatedAt = time.Now()
	}
	return args.Error(0)
}

func (m *MockCertificateRepository) GetByID(ctx context.Context, id string) (*models.Certificate, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Certificate), args.Error(1)
}

func (m *MockCertificateRepository) GetByFingerprint(ctx context.Context, fingerprint string) (*models.Certificate, error) {
	args := m.Called(ctx, fingerprint)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Certificate), args.Error(1)
}

func (m *MockCertificateRepository) GetBySerialNumber(ctx context.Context, serialNumber string) (*models.Certificate, error) {
	args := m.Called(ctx, serialNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Certificate), args.Error(1)
}

func (m *MockCertificateRepository) GetByOrgAndName(ctx context.Context, orgID, name string) (*models.Certificate, error) {
	args := m.Called(ctx, orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Certificate), args.Error(1)
}

func (m *MockCertificateRepository) ListByOrg(ctx context.Context, orgID string, filter CertificateStatusFilter) ([]*models.Certificate, error) {
	args := m.Called(ctx, orgID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Certificate), args.Error(1)
}

func (m *MockCertificateRepository) ListActiveByOrg(ctx context.Context, orgID string) ([]*models.Certificate, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Certificate), args.Error(1)
}

func (m *MockCertificateRepository) CountByOrg(ctx context.Context, orgID string) (int, error) {
	args := m.Called(ctx, orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockCertificateRepository) Revoke(ctx context.Context, id string, reason string) error {
	args := m.Called(ctx, id, reason)
	return args.Error(0)
}

func (m *MockCertificateRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCertificateRepository) IsValid(ctx context.Context, fingerprint string) (*models.Certificate, error) {
	args := m.Called(ctx, fingerprint)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Certificate), args.Error(1)
}

func (m *MockCertificateRepository) ListExpiringSoon(ctx context.Context, within time.Duration) ([]*models.Certificate, error) {
	args := m.Called(ctx, within)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Certificate), args.Error(1)
}

// Verify MockCertificateRepository implements CertificateRepository
var _ CertificateRepository = (*MockCertificateRepository)(nil)

func TestMockCertificateRepository_Create(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	certID := uuid.New()
	orgID := uuid.New()
	cert := &models.Certificate{
		ID:           certID,
		OrgID:        orgID,
		Name:         "test-cert",
		Fingerprint:  "abc123def456fingerprint",
		CommonName:   "org_01HQXYZ123456789ABCDEF",
		SerialNumber: "1234567890",
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
	}

	mockRepo.On("Create", ctx, cert).Return(nil)

	err := mockRepo.Create(ctx, cert)
	assert.NoError(t, err)
	assert.False(t, cert.CreatedAt.IsZero())
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_GetByID(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	certID := uuid.New()
	orgID := uuid.New()
	certIDStr := certID.String()
	expectedCert := &models.Certificate{
		ID:           certID,
		OrgID:        orgID,
		Name:         "test-cert",
		Fingerprint:  "abc123def456fingerprint",
		CommonName:   "org_01HQXYZ123456789ABCDEF",
		SerialNumber: "1234567890",
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	mockRepo.On("GetByID", ctx, certIDStr).Return(expectedCert, nil)

	cert, err := mockRepo.GetByID(ctx, certIDStr)
	assert.NoError(t, err)
	assert.Equal(t, expectedCert, cert)
	assert.Equal(t, certID, cert.ID)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_GetByID_NotFound(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	certID := "nonexistent-id"

	mockRepo.On("GetByID", ctx, certID).Return(nil, nil)

	cert, err := mockRepo.GetByID(ctx, certID)
	assert.NoError(t, err)
	assert.Nil(t, cert)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_GetByFingerprint(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	fingerprint := "abc123def456fingerprint"
	certID := uuid.New()
	orgID := uuid.New()
	expectedCert := &models.Certificate{
		ID:           certID,
		OrgID:        orgID,
		Name:         "test-cert",
		Fingerprint:  fingerprint,
		CommonName:   "org_01HQXYZ123456789ABCDEF",
		SerialNumber: "1234567890",
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	mockRepo.On("GetByFingerprint", ctx, fingerprint).Return(expectedCert, nil)

	cert, err := mockRepo.GetByFingerprint(ctx, fingerprint)
	assert.NoError(t, err)
	assert.Equal(t, expectedCert, cert)
	assert.Equal(t, fingerprint, cert.Fingerprint)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_GetBySerialNumber(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	serial := "1234567890"
	certID := uuid.New()
	orgID := uuid.New()
	expectedCert := &models.Certificate{
		ID:           certID,
		OrgID:        orgID,
		Name:         "test-cert",
		Fingerprint:  "abc123def456fingerprint",
		CommonName:   "org_01HQXYZ123456789ABCDEF",
		SerialNumber: serial,
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	mockRepo.On("GetBySerialNumber", ctx, serial).Return(expectedCert, nil)

	cert, err := mockRepo.GetBySerialNumber(ctx, serial)
	assert.NoError(t, err)
	assert.Equal(t, expectedCert, cert)
	assert.Equal(t, serial, cert.SerialNumber)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_GetByOrgAndName(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	certID := uuid.New()
	orgID := uuid.New()
	orgIDStr := orgID.String()
	name := "test-cert"
	expectedCert := &models.Certificate{
		ID:           certID,
		OrgID:        orgID,
		Name:         name,
		Fingerprint:  "abc123def456fingerprint",
		CommonName:   orgIDStr,
		SerialNumber: "1234567890",
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	mockRepo.On("GetByOrgAndName", ctx, orgIDStr, name).Return(expectedCert, nil)

	cert, err := mockRepo.GetByOrgAndName(ctx, orgIDStr, name)
	assert.NoError(t, err)
	assert.Equal(t, expectedCert, cert)
	assert.Equal(t, orgID, cert.OrgID)
	assert.Equal(t, name, cert.Name)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_ListByOrg(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	orgID := uuid.New()
	orgIDStr := orgID.String()
	expectedCerts := []*models.Certificate{
		{
			ID:           uuid.New(),
			OrgID:        orgID,
			Name:         "cert-1",
			Fingerprint:  "fp1",
			CommonName:   orgIDStr,
			SerialNumber: "serial1",
			IssuedAt:     time.Now(),
			ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
			CreatedAt:    time.Now(),
		},
		{
			ID:           uuid.New(),
			OrgID:        orgID,
			Name:         "cert-2",
			Fingerprint:  "fp2",
			CommonName:   orgIDStr,
			SerialNumber: "serial2",
			IssuedAt:     time.Now(),
			ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
			CreatedAt:    time.Now(),
		},
	}

	mockRepo.On("ListByOrg", ctx, orgIDStr, CertificateFilterAll).Return(expectedCerts, nil)

	certs, err := mockRepo.ListByOrg(ctx, orgIDStr, CertificateFilterAll)
	assert.NoError(t, err)
	assert.Len(t, certs, 2)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_ListByOrg_WithFilter(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	orgID := uuid.New()
	orgIDStr := orgID.String()

	// Active certificates only
	activeCerts := []*models.Certificate{
		{
			ID:           uuid.New(),
			OrgID:        orgID,
			Name:         "active-cert",
			Fingerprint:  "fp1",
			CommonName:   orgIDStr,
			SerialNumber: "serial1",
			IssuedAt:     time.Now(),
			ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
			CreatedAt:    time.Now(),
		},
	}

	mockRepo.On("ListByOrg", ctx, orgIDStr, CertificateFilterActive).Return(activeCerts, nil)

	certs, err := mockRepo.ListByOrg(ctx, orgIDStr, CertificateFilterActive)
	assert.NoError(t, err)
	assert.Len(t, certs, 1)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_ListActiveByOrg(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	orgID := uuid.New()
	orgIDStr := orgID.String()
	activeCerts := []*models.Certificate{
		{
			ID:           uuid.New(),
			OrgID:        orgID,
			Name:         "active-cert",
			Fingerprint:  "fp1",
			CommonName:   orgIDStr,
			SerialNumber: "serial1",
			IssuedAt:     time.Now(),
			ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
			CreatedAt:    time.Now(),
		},
	}

	mockRepo.On("ListActiveByOrg", ctx, orgIDStr).Return(activeCerts, nil)

	certs, err := mockRepo.ListActiveByOrg(ctx, orgIDStr)
	assert.NoError(t, err)
	assert.Len(t, certs, 1)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_CountByOrg(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	orgID := "org_01HQXYZ123456789ABCDEF"

	mockRepo.On("CountByOrg", ctx, orgID).Return(5, nil)

	count, err := mockRepo.CountByOrg(ctx, orgID)
	assert.NoError(t, err)
	assert.Equal(t, 5, count)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_Revoke(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	certID := "01HQXYZ123456789ABCDEFGH"
	reason := "Key compromise"

	mockRepo.On("Revoke", ctx, certID, reason).Return(nil)

	err := mockRepo.Revoke(ctx, certID, reason)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_Delete(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	certID := "01HQXYZ123456789ABCDEFGH"

	mockRepo.On("Delete", ctx, certID).Return(nil)

	err := mockRepo.Delete(ctx, certID)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_IsValid(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	fingerprint := "abc123def456fingerprint"
	certID := uuid.New()
	orgID := uuid.New()
	validCert := &models.Certificate{
		ID:           certID,
		OrgID:        orgID,
		Name:         "valid-cert",
		Fingerprint:  fingerprint,
		CommonName:   "org_01HQXYZ123456789ABCDEF",
		SerialNumber: "1234567890",
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	mockRepo.On("IsValid", ctx, fingerprint).Return(validCert, nil)

	cert, err := mockRepo.IsValid(ctx, fingerprint)
	assert.NoError(t, err)
	assert.NotNil(t, cert)
	assert.True(t, cert.IsValid())
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_IsValid_Revoked(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	fingerprint := "revokedfingerprint"

	// IsValid returns nil for revoked certificate
	mockRepo.On("IsValid", ctx, fingerprint).Return(nil, nil)

	cert, err := mockRepo.IsValid(ctx, fingerprint)
	assert.NoError(t, err)
	assert.Nil(t, cert)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_IsValid_Expired(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	fingerprint := "expiredfingerprint"

	// IsValid returns nil for expired certificate
	mockRepo.On("IsValid", ctx, fingerprint).Return(nil, nil)

	cert, err := mockRepo.IsValid(ctx, fingerprint)
	assert.NoError(t, err)
	assert.Nil(t, cert)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_ListExpiringSoon(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	within := 30 * 24 * time.Hour
	certID := uuid.New()
	orgID := uuid.New()
	expiringSoonCerts := []*models.Certificate{
		{
			ID:           certID,
			OrgID:        orgID,
			Name:         "expiring-cert",
			Fingerprint:  "fp1",
			CommonName:   "org_01HQXYZ123456789ABCDEF",
			SerialNumber: "serial1",
			IssuedAt:     time.Now().Add(-330 * 24 * time.Hour),
			ExpiresAt:    time.Now().Add(7 * 24 * time.Hour), // Expires in 7 days
			CreatedAt:    time.Now().Add(-330 * 24 * time.Hour),
		},
	}

	mockRepo.On("ListExpiringSoon", ctx, within).Return(expiringSoonCerts, nil)

	certs, err := mockRepo.ListExpiringSoon(ctx, within)
	assert.NoError(t, err)
	assert.Len(t, certs, 1)
	mockRepo.AssertExpectations(t)
}

func TestMockCertificateRepository_OrgIsolation(t *testing.T) {
	mockRepo := new(MockCertificateRepository)
	ctx := context.Background()

	org1ID := uuid.New()
	org2ID := uuid.New()
	org1IDStr := org1ID.String()
	org2IDStr := org2ID.String()

	org1Cert := &models.Certificate{
		ID:           uuid.New(),
		OrgID:        org1ID,
		Name:         "cert-1",
		Fingerprint:  "fp1",
		CommonName:   org1IDStr,
		SerialNumber: "serial1",
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	// Certificate exists in org1 with name "cert-1"
	mockRepo.On("GetByOrgAndName", ctx, org1IDStr, "cert-1").Return(org1Cert, nil)
	// Same name does not exist in org2
	mockRepo.On("GetByOrgAndName", ctx, org2IDStr, "cert-1").Return(nil, nil)

	// Found in org1
	cert, err := mockRepo.GetByOrgAndName(ctx, org1IDStr, "cert-1")
	assert.NoError(t, err)
	assert.NotNil(t, cert)
	assert.Equal(t, org1ID, cert.OrgID)

	// Not found in org2
	cert, err = mockRepo.GetByOrgAndName(ctx, org2IDStr, "cert-1")
	assert.NoError(t, err)
	assert.Nil(t, cert)

	mockRepo.AssertExpectations(t)
}

func TestCertificateStatus_IsValid(t *testing.T) {
	// Test certificate that is valid
	validCert := &models.Certificate{
		ID:           uuid.New(),
		OrgID:        uuid.New(),
		Name:         "valid-cert",
		Fingerprint:  "fp1",
		CommonName:   "org_01HQXYZ123456789ABCDEF",
		SerialNumber: "serial1",
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CreatedAt:    time.Now(),
		RevokedAt:    nil,
	}
	assert.True(t, validCert.IsValid())
	assert.False(t, validCert.IsRevoked())
	assert.False(t, validCert.IsExpired())
	assert.Equal(t, models.CertificateStatusActive, validCert.Status())
}

func TestCertificateStatus_IsRevoked(t *testing.T) {
	revokedAt := time.Now()
	revokedCert := &models.Certificate{
		ID:           uuid.New(),
		OrgID:        uuid.New(),
		Name:         "revoked-cert",
		Fingerprint:  "fp1",
		CommonName:   "org_01HQXYZ123456789ABCDEF",
		SerialNumber: "serial1",
		IssuedAt:     time.Now().Add(-30 * 24 * time.Hour),
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CreatedAt:    time.Now().Add(-30 * 24 * time.Hour),
		RevokedAt:    &revokedAt,
	}
	assert.False(t, revokedCert.IsValid())
	assert.True(t, revokedCert.IsRevoked())
	assert.Equal(t, models.CertificateStatusRevoked, revokedCert.Status())
}

func TestCertificateStatus_IsExpired(t *testing.T) {
	expiredCert := &models.Certificate{
		ID:           uuid.New(),
		OrgID:        uuid.New(),
		Name:         "expired-cert",
		Fingerprint:  "fp1",
		CommonName:   "org_01HQXYZ123456789ABCDEF",
		SerialNumber: "serial1",
		IssuedAt:     time.Now().Add(-400 * 24 * time.Hour),
		ExpiresAt:    time.Now().Add(-30 * 24 * time.Hour), // Expired 30 days ago
		CreatedAt:    time.Now().Add(-400 * 24 * time.Hour),
		RevokedAt:    nil,
	}
	assert.False(t, expiredCert.IsValid())
	assert.True(t, expiredCert.IsExpired())
	assert.Equal(t, models.CertificateStatusExpired, expiredCert.Status())
}

