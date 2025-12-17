package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/middleware"
	"github.com/Bidon15/popsigner/control-plane/internal/models"
	"github.com/Bidon15/popsigner/control-plane/internal/repository"
)

// mockCertRepo implements a mock CertificateRepository for testing.
type mockCertRepo struct {
	cert *models.Certificate
	err  error
}

func (m *mockCertRepo) GetByFingerprint(ctx context.Context, fingerprint string) (*models.Certificate, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.cert != nil && m.cert.Fingerprint == fingerprint {
		return m.cert, nil
	}
	return nil, nil
}

// Implement other interface methods (not used in tests but required)
func (m *mockCertRepo) Create(ctx context.Context, cert *models.Certificate) error {
	return nil
}
func (m *mockCertRepo) GetByID(ctx context.Context, id string) (*models.Certificate, error) {
	return nil, nil
}
func (m *mockCertRepo) GetBySerialNumber(ctx context.Context, serialNumber string) (*models.Certificate, error) {
	return nil, nil
}
func (m *mockCertRepo) GetByOrgAndName(ctx context.Context, orgID, name string) (*models.Certificate, error) {
	return nil, nil
}
func (m *mockCertRepo) ListByOrg(ctx context.Context, orgID string, filter repository.CertificateStatusFilter) ([]*models.Certificate, error) {
	return nil, nil
}
func (m *mockCertRepo) ListActiveByOrg(ctx context.Context, orgID string) ([]*models.Certificate, error) {
	return nil, nil
}
func (m *mockCertRepo) CountByOrg(ctx context.Context, orgID string) (int, error) {
	return 0, nil
}
func (m *mockCertRepo) Revoke(ctx context.Context, id string, reason string) error {
	return nil
}
func (m *mockCertRepo) Delete(ctx context.Context, id string) error {
	return nil
}
func (m *mockCertRepo) IsValid(ctx context.Context, fingerprint string) (*models.Certificate, error) {
	return nil, nil
}
func (m *mockCertRepo) ListExpiringSoon(ctx context.Context, within time.Duration) ([]*models.Certificate, error) {
	return nil, nil
}

func TestMTLSAuthenticator_Authenticate(t *testing.T) {
	// Generate test CA
	caKey, caCert := generateTestCA(t)

	// Generate client cert with org ID in CN
	_, clientCert := generateTestClientCert(t, caKey, caCert, "org_test123")

	// Calculate fingerprint
	fingerprint := CalculateCertFingerprint(clientCert)

	// Create mock request with TLS info
	r := httptest.NewRequest("POST", "/rpc", nil)
	r.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{clientCert},
	}

	// Create authenticator with mock repo
	orgID := uuid.New()
	repo := &mockCertRepo{
		cert: &models.Certificate{
			OrgID:       orgID,
			Fingerprint: fingerprint,
			ExpiresAt:   time.Now().Add(time.Hour),
		},
	}
	auth := NewMTLSAuthenticator(repo)

	// Test authentication
	result, err := auth.Authenticate(context.Background(), r)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	if result.OrgID != "org_test123" {
		t.Errorf("OrgID = %s, want org_test123", result.OrgID)
	}
	if result.Method != "mtls" {
		t.Errorf("Method = %s, want mtls", result.Method)
	}
}

func TestMTLSAuthenticator_NoTLS(t *testing.T) {
	repo := &mockCertRepo{}
	auth := NewMTLSAuthenticator(repo)

	r := httptest.NewRequest("POST", "/rpc", nil)
	// r.TLS is nil

	_, err := auth.Authenticate(context.Background(), r)
	if err == nil {
		t.Error("Expected error for no TLS connection")
	}
}

func TestMTLSAuthenticator_NoPeerCerts(t *testing.T) {
	repo := &mockCertRepo{}
	auth := NewMTLSAuthenticator(repo)

	r := httptest.NewRequest("POST", "/rpc", nil)
	r.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{},
	}

	_, err := auth.Authenticate(context.Background(), r)
	if err == nil {
		t.Error("Expected error for no client certificate")
	}
}

func TestMTLSAuthenticator_RevokedCert(t *testing.T) {
	caKey, caCert := generateTestCA(t)
	_, clientCert := generateTestClientCert(t, caKey, caCert, "org_test123")
	fingerprint := CalculateCertFingerprint(clientCert)

	r := httptest.NewRequest("POST", "/rpc", nil)
	r.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{clientCert},
	}

	// Cert is revoked
	now := time.Now()
	orgID := uuid.New()
	repo := &mockCertRepo{
		cert: &models.Certificate{
			OrgID:       orgID,
			Fingerprint: fingerprint,
			ExpiresAt:   time.Now().Add(time.Hour),
			RevokedAt:   &now,
		},
	}
	auth := NewMTLSAuthenticator(repo)

	_, err := auth.Authenticate(context.Background(), r)
	if err == nil {
		t.Error("Expected error for revoked cert")
	}
	if err.Error() != "certificate has been revoked" {
		t.Errorf("Wrong error message: %s", err.Error())
	}
}

func TestMTLSAuthenticator_ExpiredCert(t *testing.T) {
	caKey, caCert := generateTestCA(t)
	_, clientCert := generateTestClientCert(t, caKey, caCert, "org_test123")
	fingerprint := CalculateCertFingerprint(clientCert)

	r := httptest.NewRequest("POST", "/rpc", nil)
	r.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{clientCert},
	}

	// Cert is expired
	orgID := uuid.New()
	repo := &mockCertRepo{
		cert: &models.Certificate{
			OrgID:       orgID,
			Fingerprint: fingerprint,
			ExpiresAt:   time.Now().Add(-time.Hour), // Expired
		},
	}
	auth := NewMTLSAuthenticator(repo)

	_, err := auth.Authenticate(context.Background(), r)
	if err == nil {
		t.Error("Expected error for expired cert")
	}
	if err.Error() != "certificate has expired" {
		t.Errorf("Wrong error message: %s", err.Error())
	}
}

func TestMTLSAuthenticator_UnregisteredCert(t *testing.T) {
	caKey, caCert := generateTestCA(t)
	_, clientCert := generateTestClientCert(t, caKey, caCert, "org_test123")

	r := httptest.NewRequest("POST", "/rpc", nil)
	r.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{clientCert},
	}

	// No cert in database
	repo := &mockCertRepo{
		cert: nil,
	}
	auth := NewMTLSAuthenticator(repo)

	_, err := auth.Authenticate(context.Background(), r)
	if err == nil {
		t.Error("Expected error for unregistered cert")
	}
	if err.Error() != "certificate not registered" {
		t.Errorf("Wrong error message: %s", err.Error())
	}
}

func TestMTLSAuthenticator_CNMismatch(t *testing.T) {
	caKey, caCert := generateTestCA(t)
	_, clientCert := generateTestClientCert(t, caKey, caCert, "org_test123")
	fingerprint := CalculateCertFingerprint(clientCert)

	r := httptest.NewRequest("POST", "/rpc", nil)
	r.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{clientCert},
	}

	// Cert in DB has different org ID
	orgID := uuid.New()
	repo := &mockCertRepo{
		cert: &models.Certificate{
			OrgID:       orgID, // Different from cert CN
			Fingerprint: fingerprint,
			ExpiresAt:   time.Now().Add(time.Hour),
		},
	}
	auth := NewMTLSAuthenticator(repo)

	_, err := auth.Authenticate(context.Background(), r)
	if err == nil {
		t.Error("Expected error for CN mismatch")
	}
	if err.Error() != "certificate CN does not match registered organization" {
		t.Errorf("Wrong error message: %s", err.Error())
	}
}

func TestMTLSAuthenticator_InvalidCN(t *testing.T) {
	caKey, caCert := generateTestCA(t)
	// Create cert with invalid CN (doesn't start with org_)
	_, clientCert := generateTestClientCert(t, caKey, caCert, "invalid_cn")
	fingerprint := CalculateCertFingerprint(clientCert)

	r := httptest.NewRequest("POST", "/rpc", nil)
	r.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{clientCert},
	}

	orgID := uuid.New()
	repo := &mockCertRepo{
		cert: &models.Certificate{
			OrgID:       orgID,
			Fingerprint: fingerprint,
			ExpiresAt:   time.Now().Add(time.Hour),
		},
	}
	auth := NewMTLSAuthenticator(repo)

	_, err := auth.Authenticate(context.Background(), r)
	if err == nil {
		t.Error("Expected error for invalid CN")
	}
}

func TestParseClientAuthType(t *testing.T) {
	tests := []struct {
		input    string
		expected tls.ClientAuthType
	}{
		{"NoClientCert", tls.NoClientCert},
		{"RequestClientCert", tls.RequestClientCert},
		{"RequireAnyClientCert", tls.RequireAnyClientCert},
		{"VerifyClientCertIfGiven", tls.VerifyClientCertIfGiven},
		{"RequireAndVerifyClientCert", tls.RequireAndVerifyClientCert},
		{"unknown", tls.VerifyClientCertIfGiven}, // Default
		{"", tls.VerifyClientCertIfGiven},        // Empty string defaults
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseClientAuthType(tt.input); got != tt.expected {
				t.Errorf("ParseClientAuthType(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCalculateCertFingerprint(t *testing.T) {
	caKey, caCert := generateTestCA(t)
	_, clientCert := generateTestClientCert(t, caKey, caCert, "org_test")

	fp1 := CalculateCertFingerprint(clientCert)
	fp2 := CalculateCertFingerprint(clientCert)

	// Same cert should produce same fingerprint
	if fp1 != fp2 {
		t.Error("Same certificate produced different fingerprints")
	}

	// Fingerprint should be hex encoded SHA256 (64 chars)
	if len(fp1) != 64 {
		t.Errorf("Fingerprint length = %d, want 64", len(fp1))
	}
}

func TestGetOrgID(t *testing.T) {
	ctx := context.Background()

	// No org ID
	if got := GetOrgID(ctx); got != "" {
		t.Errorf("GetOrgID(empty) = %s, want empty", got)
	}

	// With org ID (use middleware.OrgIDKey)
	ctx = context.WithValue(ctx, middleware.OrgIDKey, "org_test123")
	if got := GetOrgID(ctx); got != "org_test123" {
		t.Errorf("GetOrgID = %s, want org_test123", got)
	}
}

func TestGetAuthMethod(t *testing.T) {
	ctx := context.Background()

	// No auth method
	if got := GetAuthMethod(ctx); got != "" {
		t.Errorf("GetAuthMethod(empty) = %s, want empty", got)
	}

	// With auth method
	ctx = context.WithValue(ctx, AuthMethodKey, "mtls")
	if got := GetAuthMethod(ctx); got != "mtls" {
		t.Errorf("GetAuthMethod = %s, want mtls", got)
	}
}

func TestExtractAPIKey(t *testing.T) {
	tests := []struct {
		name          string
		headers       map[string]string
		expectedKey   string
	}{
		{
			name:        "X-API-Key header",
			headers:     map[string]string{"X-API-Key": "test_key_123"},
			expectedKey: "test_key_123",
		},
		{
			name:        "Bearer token",
			headers:     map[string]string{"Authorization": "Bearer test_key_456"},
			expectedKey: "test_key_456",
		},
		{
			name:        "ApiKey scheme",
			headers:     map[string]string{"Authorization": "ApiKey test_key_789"},
			expectedKey: "test_key_789",
		},
		{
			name:        "No key",
			headers:     map[string]string{},
			expectedKey: "",
		},
		{
			name:        "Bearer preferred over X-API-Key",
			headers:     map[string]string{"Authorization": "Bearer bearer_key", "X-API-Key": "xapi_key"},
			expectedKey: "bearer_key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/rpc", nil)
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}

			got := extractAPIKey(r)
			if got != tt.expectedKey {
				t.Errorf("extractAPIKey() = %s, want %s", got, tt.expectedKey)
			}
		})
	}
}

// Test helpers

func generateTestCA(t *testing.T) (*ecdsa.PrivateKey, *x509.Certificate) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate CA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test CA"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("Failed to create CA cert: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse CA cert: %v", err)
	}

	return key, cert
}

func generateTestClientCert(t *testing.T, caKey *ecdsa.PrivateKey, caCert *x509.Certificate, cn string) (*ecdsa.PrivateKey, *x509.Certificate) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate client key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("Failed to create client cert: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse client cert: %v", err)
	}

	return key, cert
}

