package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Bidon15/popsigner/control-plane/internal/models"
	"github.com/Bidon15/popsigner/control-plane/internal/repository"
)

// MockCertificateService is a mock implementation of CertificateService for testing.
type MockCertificateService struct {
	mock.Mock
}

func (m *MockCertificateService) Issue(ctx context.Context, req *models.CreateCertificateRequest) (*models.CertificateBundle, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CertificateBundle), args.Error(1)
}

func (m *MockCertificateService) Revoke(ctx context.Context, orgID, certID, reason string) error {
	args := m.Called(ctx, orgID, certID, reason)
	return args.Error(0)
}

func (m *MockCertificateService) Get(ctx context.Context, orgID, certID string) (*models.Certificate, error) {
	args := m.Called(ctx, orgID, certID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Certificate), args.Error(1)
}

func (m *MockCertificateService) List(ctx context.Context, orgID string, filter repository.CertificateStatusFilter) (*models.CertificateListResponse, error) {
	args := m.Called(ctx, orgID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CertificateListResponse), args.Error(1)
}

func (m *MockCertificateService) Delete(ctx context.Context, orgID, certID string) error {
	args := m.Called(ctx, orgID, certID)
	return args.Error(0)
}

func (m *MockCertificateService) ValidateForAuth(ctx context.Context, fingerprint string) (string, error) {
	args := m.Called(ctx, fingerprint)
	return args.String(0), args.Error(1)
}

func (m *MockCertificateService) GetCACertificate(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCertificateService) DownloadBundle(ctx context.Context, orgID, certID string) (*models.CertificateBundle, error) {
	args := m.Called(ctx, orgID, certID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CertificateBundle), args.Error(1)
}

// Verify MockCertificateService implements CertificateService
var _ CertificateService = (*MockCertificateService)(nil)

func TestMockCertificateService_Issue(t *testing.T) {
	mockSvc := new(MockCertificateService)
	ctx := context.Background()

	req := &models.CreateCertificateRequest{
		OrgID:          uuid.New(),
		Name:           "test-cert",
		ValidityPeriod: 365 * 24 * time.Hour,
	}

	expectedBundle := &models.CertificateBundle{
		ClientCert:  []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"),
		ClientKey:   []byte("-----BEGIN EC PRIVATE KEY-----\ntest\n-----END EC PRIVATE KEY-----\n"),
		CACert:      []byte("-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n"),
		Fingerprint: "abc123fingerprint",
		ExpiresAt:   time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
	}

	mockSvc.On("Issue", ctx, req).Return(expectedBundle, nil)

	bundle, err := mockSvc.Issue(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, bundle)
	assert.Equal(t, expectedBundle.Fingerprint, bundle.Fingerprint)
	assert.NotEmpty(t, bundle.ClientCert)
	assert.NotEmpty(t, bundle.ClientKey)
	assert.NotEmpty(t, bundle.CACert)
	mockSvc.AssertExpectations(t)
}

func TestMockCertificateService_Revoke(t *testing.T) {
	mockSvc := new(MockCertificateService)
	ctx := context.Background()

	orgID := "org_01HQXYZ123456789ABCDEF"
	certID := "01HQXYZ123456789ABCDEFGH"
	reason := "Key compromise"

	mockSvc.On("Revoke", ctx, orgID, certID, reason).Return(nil)

	err := mockSvc.Revoke(ctx, orgID, certID, reason)
	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMockCertificateService_Get(t *testing.T) {
	mockSvc := new(MockCertificateService)
	ctx := context.Background()

	orgID := uuid.New()
	certID := uuid.New()
	orgIDStr := orgID.String()
	certIDStr := certID.String()

	expectedCert := &models.Certificate{
		ID:           certID,
		OrgID:        orgID,
		Name:         "test-cert",
		Fingerprint:  "abc123fingerprint",
		CommonName:   orgIDStr,
		SerialNumber: "1234567890",
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	mockSvc.On("Get", ctx, orgIDStr, certIDStr).Return(expectedCert, nil)

	cert, err := mockSvc.Get(ctx, orgIDStr, certIDStr)
	assert.NoError(t, err)
	assert.NotNil(t, cert)
	assert.Equal(t, certID, cert.ID)
	assert.Equal(t, orgID, cert.OrgID)
	mockSvc.AssertExpectations(t)
}

func TestMockCertificateService_Get_NotFound(t *testing.T) {
	mockSvc := new(MockCertificateService)
	ctx := context.Background()

	orgID := "org_01HQXYZ123456789ABCDEF"
	certID := "nonexistent-id"

	mockSvc.On("Get", ctx, orgID, certID).Return(nil, nil)

	cert, err := mockSvc.Get(ctx, orgID, certID)
	assert.NoError(t, err)
	assert.Nil(t, cert)
	mockSvc.AssertExpectations(t)
}

func TestMockCertificateService_List(t *testing.T) {
	mockSvc := new(MockCertificateService)
	ctx := context.Background()

	orgID := uuid.New()
	orgIDStr := orgID.String()

	expectedResponse := &models.CertificateListResponse{
		Certificates: []models.Certificate{
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
		},
		Total: 2,
	}

	mockSvc.On("List", ctx, orgIDStr, repository.CertificateFilterAll).Return(expectedResponse, nil)

	resp, err := mockSvc.List(ctx, orgIDStr, repository.CertificateFilterAll)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Certificates, 2)
	assert.Equal(t, 2, resp.Total)
	mockSvc.AssertExpectations(t)
}

func TestMockCertificateService_List_WithFilter(t *testing.T) {
	mockSvc := new(MockCertificateService)
	ctx := context.Background()

	orgID := uuid.New()
	orgIDStr := orgID.String()

	expectedResponse := &models.CertificateListResponse{
		Certificates: []models.Certificate{
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
		},
		Total: 1,
	}

	mockSvc.On("List", ctx, orgIDStr, repository.CertificateFilterActive).Return(expectedResponse, nil)

	resp, err := mockSvc.List(ctx, orgIDStr, repository.CertificateFilterActive)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Certificates, 1)
	mockSvc.AssertExpectations(t)
}

func TestMockCertificateService_Delete(t *testing.T) {
	mockSvc := new(MockCertificateService)
	ctx := context.Background()

	orgID := "org_01HQXYZ123456789ABCDEF"
	certID := "01HQXYZ123456789ABCDEFGH"

	mockSvc.On("Delete", ctx, orgID, certID).Return(nil)

	err := mockSvc.Delete(ctx, orgID, certID)
	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMockCertificateService_ValidateForAuth(t *testing.T) {
	mockSvc := new(MockCertificateService)
	ctx := context.Background()

	fingerprint := "abc123fingerprint"
	expectedOrgID := "org_01HQXYZ123456789ABCDEF"

	mockSvc.On("ValidateForAuth", ctx, fingerprint).Return(expectedOrgID, nil)

	orgID, err := mockSvc.ValidateForAuth(ctx, fingerprint)
	assert.NoError(t, err)
	assert.Equal(t, expectedOrgID, orgID)
	mockSvc.AssertExpectations(t)
}

func TestMockCertificateService_ValidateForAuth_Invalid(t *testing.T) {
	mockSvc := new(MockCertificateService)
	ctx := context.Background()

	fingerprint := "invalidfingerprint"

	mockSvc.On("ValidateForAuth", ctx, fingerprint).Return("", nil)

	orgID, err := mockSvc.ValidateForAuth(ctx, fingerprint)
	assert.NoError(t, err)
	assert.Empty(t, orgID)
	mockSvc.AssertExpectations(t)
}

func TestMockCertificateService_GetCACertificate(t *testing.T) {
	mockSvc := new(MockCertificateService)
	ctx := context.Background()

	expectedCA := []byte("-----BEGIN CERTIFICATE-----\nCA CERT\n-----END CERTIFICATE-----\n")

	mockSvc.On("GetCACertificate", ctx).Return(expectedCA, nil)

	ca, err := mockSvc.GetCACertificate(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, ca)
	assert.Equal(t, expectedCA, ca)
	mockSvc.AssertExpectations(t)
}

func TestMockCertificateService_DownloadBundle(t *testing.T) {
	mockSvc := new(MockCertificateService)
	ctx := context.Background()

	orgID := "org_01HQXYZ123456789ABCDEF"
	certID := "01HQXYZ123456789ABCDEFGH"

	expectedBundle := &models.CertificateBundle{
		CACert:      []byte("-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n"),
		Fingerprint: "abc123fingerprint",
		ExpiresAt:   time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
	}

	mockSvc.On("DownloadBundle", ctx, orgID, certID).Return(expectedBundle, nil)

	bundle, err := mockSvc.DownloadBundle(ctx, orgID, certID)
	assert.NoError(t, err)
	assert.NotNil(t, bundle)
	assert.NotEmpty(t, bundle.CACert)
	mockSvc.AssertExpectations(t)
}

func TestCalculateFingerprint(t *testing.T) {
	// Generate a valid self-signed certificate for testing
	validPEM := generateTestCertificate(t)

	fingerprint, err := calculateFingerprint(validPEM)
	assert.NoError(t, err)
	assert.NotEmpty(t, fingerprint)
	assert.Len(t, fingerprint, 64) // SHA256 hex = 64 chars
}

// generateTestCertificate creates a valid self-signed certificate for testing.
func generateTestCertificate(t *testing.T) string {
	t.Helper()

	// Generate ECDSA P-256 key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "test-cert",
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Self-sign the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	// Encode to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	return string(certPEM)
}

func TestCalculateFingerprint_InvalidPEM(t *testing.T) {
	invalidPEM := "not a valid PEM"

	_, err := calculateFingerprint(invalidPEM)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM")
}

func TestCertificateRequestValidation(t *testing.T) {
	testOrgID := uuid.New()
	tests := []struct {
		name    string
		req     *models.CreateCertificateRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request",
			req: &models.CreateCertificateRequest{
				OrgID:          testOrgID,
				Name:           "test-cert",
				ValidityPeriod: 365 * 24 * time.Hour,
			},
			wantErr: false,
		},
		{
			name: "missing org_id",
			req: &models.CreateCertificateRequest{
				Name:           "test-cert",
				ValidityPeriod: 365 * 24 * time.Hour,
			},
			wantErr: true,
			errMsg:  "org_id is required",
		},
		{
			name: "missing name",
			req: &models.CreateCertificateRequest{
				OrgID:          testOrgID,
				ValidityPeriod: 365 * 24 * time.Hour,
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "validity too short",
			req: &models.CreateCertificateRequest{
				OrgID:          testOrgID,
				Name:           "test-cert",
				ValidityPeriod: time.Minute,
			},
			wantErr: true,
			errMsg:  "at least 1 hour",
		},
		{
			name: "validity too long",
			req: &models.CreateCertificateRequest{
				OrgID:          testOrgID,
				Name:           "test-cert",
				ValidityPeriod: 10 * 365 * 24 * time.Hour,
			},
			wantErr: true,
			errMsg:  "exceed 5 years",
		},
		{
			name: "default validity",
			req: &models.CreateCertificateRequest{
				OrgID: testOrgID,
				Name:  "test-cert",
				// ValidityPeriod not set, should default
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

