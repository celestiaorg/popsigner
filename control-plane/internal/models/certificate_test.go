package models

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

func TestCertificate_IsRevoked(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		cert     Certificate
		expected bool
	}{
		{
			name:     "not revoked",
			cert:     Certificate{RevokedAt: nil},
			expected: false,
		},
		{
			name:     "revoked",
			cert:     Certificate{RevokedAt: &now},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cert.IsRevoked(); got != tt.expected {
				t.Errorf("IsRevoked() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCertificate_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		cert     Certificate
		expected bool
	}{
		{
			name:     "not expired",
			cert:     Certificate{ExpiresAt: time.Now().Add(time.Hour)},
			expected: false,
		},
		{
			name:     "expired",
			cert:     Certificate{ExpiresAt: time.Now().Add(-time.Hour)},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cert.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCertificate_IsValid(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name     string
		cert     Certificate
		expected bool
	}{
		{
			name:     "valid - not expired and not revoked",
			cert:     Certificate{ExpiresAt: future, RevokedAt: nil},
			expected: true,
		},
		{
			name:     "invalid - expired",
			cert:     Certificate{ExpiresAt: past, RevokedAt: nil},
			expected: false,
		},
		{
			name:     "invalid - revoked",
			cert:     Certificate{ExpiresAt: future, RevokedAt: &now},
			expected: false,
		},
		{
			name:     "invalid - both expired and revoked",
			cert:     Certificate{ExpiresAt: past, RevokedAt: &now},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cert.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCertificate_Status(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		cert     Certificate
		expected CertificateStatus
	}{
		{
			name:     "active",
			cert:     Certificate{ExpiresAt: now.Add(time.Hour), RevokedAt: nil},
			expected: CertificateStatusActive,
		},
		{
			name:     "expired",
			cert:     Certificate{ExpiresAt: now.Add(-time.Hour), RevokedAt: nil},
			expected: CertificateStatusExpired,
		},
		{
			name:     "revoked takes precedence over active",
			cert:     Certificate{ExpiresAt: now.Add(time.Hour), RevokedAt: &now},
			expected: CertificateStatusRevoked,
		},
		{
			name:     "revoked takes precedence over expired",
			cert:     Certificate{ExpiresAt: now.Add(-time.Hour), RevokedAt: &now},
			expected: CertificateStatusRevoked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cert.Status(); got != tt.expected {
				t.Errorf("Status() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOrgIDFromCN(t *testing.T) {
	tests := []struct {
		name    string
		cn      string
		want    string
		wantErr bool
	}{
		{
			name:    "valid org CN",
			cn:      "org_01J5K7ABC123",
			want:    "org_01J5K7ABC123",
			wantErr: false,
		},
		{
			name:    "valid org CN with full ULID",
			cn:      "org_01HY7XQKJ3ABCDEFGH1234567",
			want:    "org_01HY7XQKJ3ABCDEFGH1234567",
			wantErr: false,
		},
		{
			name:    "invalid prefix - user",
			cn:      "user_123",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid prefix - none",
			cn:      "01J5K7ABC123",
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty",
			cn:      "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "just prefix",
			cn:      "org_",
			want:    "org_",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := OrgIDFromCN(tt.cn)
			if (err != nil) != tt.wantErr {
				t.Errorf("OrgIDFromCN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("OrgIDFromCN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCNFromOrgID(t *testing.T) {
	tests := []struct {
		name  string
		orgID string
		want  string
	}{
		{
			name:  "with prefix",
			orgID: "org_01J5K7ABC123",
			want:  "org_01J5K7ABC123",
		},
		{
			name:  "without prefix",
			orgID: "01J5K7ABC123",
			want:  "org_01J5K7ABC123",
		},
		{
			name:  "empty string",
			orgID: "",
			want:  "org_",
		},
		{
			name:  "partial prefix should add full prefix",
			orgID: "rg_123",
			want:  "org_rg_123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CNFromOrgID(tt.orgID); got != tt.want {
				t.Errorf("CNFromOrgID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFingerprintFromCert(t *testing.T) {
	// Create a self-signed certificate for testing
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "org_01J5K7ABC123",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(24 * time.Hour),
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	fingerprint := FingerprintFromCert(cert)

	// Check that fingerprint is a valid hex string of correct length (SHA256 = 32 bytes = 64 hex chars)
	if len(fingerprint) != 64 {
		t.Errorf("FingerprintFromCert() returned fingerprint of length %d, want 64", len(fingerprint))
	}

	// Check that fingerprint is consistent
	fingerprint2 := FingerprintFromCert(cert)
	if fingerprint != fingerprint2 {
		t.Errorf("FingerprintFromCert() is not consistent: %v != %v", fingerprint, fingerprint2)
	}

	// Create another certificate and verify fingerprints are different
	template2 := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "org_01J5K7ABC456",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(24 * time.Hour),
	}

	derBytes2, err := x509.CreateCertificate(rand.Reader, template2, template2, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create second certificate: %v", err)
	}

	cert2, err := x509.ParseCertificate(derBytes2)
	if err != nil {
		t.Fatalf("failed to parse second certificate: %v", err)
	}

	fingerprint3 := FingerprintFromCert(cert2)
	if fingerprint == fingerprint3 {
		t.Error("FingerprintFromCert() should return different fingerprints for different certificates")
	}
}

func TestCreateCertificateRequest_Validate(t *testing.T) {
	longName := make([]byte, 256)
	for i := range longName {
		longName[i] = 'a'
	}

	tests := []struct {
		name    string
		req     CreateCertificateRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid with defaults",
			req: CreateCertificateRequest{
				OrgID: "org_123",
				Name:  "test-cert",
			},
			wantErr: false,
		},
		{
			name: "valid with custom validity",
			req: CreateCertificateRequest{
				OrgID:          "org_123",
				Name:           "test-cert",
				ValidityPeriod: 30 * 24 * time.Hour, // 30 days
			},
			wantErr: false,
		},
		{
			name: "valid with max validity",
			req: CreateCertificateRequest{
				OrgID:          "org_123",
				Name:           "test-cert",
				ValidityPeriod: 5 * 365 * 24 * time.Hour, // 5 years
			},
			wantErr: false,
		},
		{
			name: "valid with min validity",
			req: CreateCertificateRequest{
				OrgID:          "org_123",
				Name:           "test-cert",
				ValidityPeriod: time.Hour,
			},
			wantErr: false,
		},
		{
			name: "missing org_id",
			req: CreateCertificateRequest{
				Name: "test-cert",
			},
			wantErr: true,
			errMsg:  "org_id is required",
		},
		{
			name: "missing name",
			req: CreateCertificateRequest{
				OrgID: "org_123",
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "name too long",
			req: CreateCertificateRequest{
				OrgID: "org_123",
				Name:  string(longName),
			},
			wantErr: true,
			errMsg:  "name must be 255 characters or less",
		},
		{
			name: "validity too short",
			req: CreateCertificateRequest{
				OrgID:          "org_123",
				Name:           "test-cert",
				ValidityPeriod: time.Minute, // Less than 1 hour
			},
			wantErr: true,
			errMsg:  "validity_period must be at least 1 hour",
		},
		{
			name: "validity too long",
			req: CreateCertificateRequest{
				OrgID:          "org_123",
				Name:           "test-cert",
				ValidityPeriod: 10 * 365 * 24 * time.Hour, // 10 years
			},
			wantErr: true,
			errMsg:  "validity_period cannot exceed 5 years",
		},
		{
			name: "empty org_id",
			req: CreateCertificateRequest{
				OrgID: "",
				Name:  "test-cert",
			},
			wantErr: true,
			errMsg:  "org_id is required",
		},
		{
			name: "empty name",
			req: CreateCertificateRequest{
				OrgID: "org_123",
				Name:  "",
			},
			wantErr: true,
			errMsg:  "name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestCreateCertificateRequest_Validate_DefaultsValidityPeriod(t *testing.T) {
	req := CreateCertificateRequest{
		OrgID: "org_123",
		Name:  "test-cert",
	}

	if req.ValidityPeriod != 0 {
		t.Errorf("ValidityPeriod should be 0 before validation, got %v", req.ValidityPeriod)
	}

	err := req.Validate()
	if err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}

	if req.ValidityPeriod != DefaultValidityPeriod {
		t.Errorf("ValidityPeriod should be %v after validation, got %v", DefaultValidityPeriod, req.ValidityPeriod)
	}
}

func TestCertificate_ToResponse(t *testing.T) {
	now := time.Now()
	expires := now.Add(24 * time.Hour)
	reason := "key compromise"
	revokedAt := now.Add(-time.Hour)

	tests := []struct {
		name string
		cert Certificate
	}{
		{
			name: "active certificate",
			cert: Certificate{
				ID:           "01HY7XQKJ3ABCDEFGH1234567",
				OrgID:        "org_01J5K7ABC123",
				Name:         "my-cert",
				Fingerprint:  "abc123def456",
				CommonName:   "org_01J5K7ABC123",
				SerialNumber: "123456789",
				IssuedAt:     now,
				ExpiresAt:    expires,
				RevokedAt:    nil,
				CreatedAt:    now,
			},
		},
		{
			name: "revoked certificate",
			cert: Certificate{
				ID:               "01HY7XQKJ3ABCDEFGH7654321",
				OrgID:            "org_01J5K7ABC456",
				Name:             "revoked-cert",
				Fingerprint:      "xyz789",
				CommonName:       "org_01J5K7ABC456",
				SerialNumber:     "987654321",
				IssuedAt:         now.Add(-48 * time.Hour),
				ExpiresAt:        expires,
				RevokedAt:        &revokedAt,
				RevocationReason: &reason,
				CreatedAt:        now.Add(-48 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.cert.ToResponse()

			if resp.ID != tt.cert.ID {
				t.Errorf("ID = %v, want %v", resp.ID, tt.cert.ID)
			}
			if resp.OrgID != tt.cert.OrgID {
				t.Errorf("OrgID = %v, want %v", resp.OrgID, tt.cert.OrgID)
			}
			if resp.Name != tt.cert.Name {
				t.Errorf("Name = %v, want %v", resp.Name, tt.cert.Name)
			}
			if resp.Fingerprint != tt.cert.Fingerprint {
				t.Errorf("Fingerprint = %v, want %v", resp.Fingerprint, tt.cert.Fingerprint)
			}
			if resp.CommonName != tt.cert.CommonName {
				t.Errorf("CommonName = %v, want %v", resp.CommonName, tt.cert.CommonName)
			}
			if resp.SerialNumber != tt.cert.SerialNumber {
				t.Errorf("SerialNumber = %v, want %v", resp.SerialNumber, tt.cert.SerialNumber)
			}
			if resp.Status != tt.cert.Status() {
				t.Errorf("Status = %v, want %v", resp.Status, tt.cert.Status())
			}
			if !resp.IssuedAt.Equal(tt.cert.IssuedAt) {
				t.Errorf("IssuedAt = %v, want %v", resp.IssuedAt, tt.cert.IssuedAt)
			}
			if !resp.ExpiresAt.Equal(tt.cert.ExpiresAt) {
				t.Errorf("ExpiresAt = %v, want %v", resp.ExpiresAt, tt.cert.ExpiresAt)
			}
			if resp.RevokedAt != tt.cert.RevokedAt {
				if resp.RevokedAt == nil || tt.cert.RevokedAt == nil {
					t.Errorf("RevokedAt = %v, want %v", resp.RevokedAt, tt.cert.RevokedAt)
				} else if !resp.RevokedAt.Equal(*tt.cert.RevokedAt) {
					t.Errorf("RevokedAt = %v, want %v", *resp.RevokedAt, *tt.cert.RevokedAt)
				}
			}
			if resp.RevocationReason != tt.cert.RevocationReason {
				if resp.RevocationReason == nil || tt.cert.RevocationReason == nil {
					t.Errorf("RevocationReason = %v, want %v", resp.RevocationReason, tt.cert.RevocationReason)
				} else if *resp.RevocationReason != *tt.cert.RevocationReason {
					t.Errorf("RevocationReason = %v, want %v", *resp.RevocationReason, *tt.cert.RevocationReason)
				}
			}
		})
	}
}

func TestCertificateStatusConstants(t *testing.T) {
	if CertificateStatusActive != "active" {
		t.Errorf("CertificateStatusActive = %q, want %q", CertificateStatusActive, "active")
	}
	if CertificateStatusExpired != "expired" {
		t.Errorf("CertificateStatusExpired = %q, want %q", CertificateStatusExpired, "expired")
	}
	if CertificateStatusRevoked != "revoked" {
		t.Errorf("CertificateStatusRevoked = %q, want %q", CertificateStatusRevoked, "revoked")
	}
}

func TestValidityPeriodConstants(t *testing.T) {
	if DefaultValidityPeriod != 365*24*time.Hour {
		t.Errorf("DefaultValidityPeriod = %v, want %v", DefaultValidityPeriod, 365*24*time.Hour)
	}
	if MaxValidityPeriod != 5*365*24*time.Hour {
		t.Errorf("MaxValidityPeriod = %v, want %v", MaxValidityPeriod, 5*365*24*time.Hour)
	}
	if MinValidityPeriod != time.Hour {
		t.Errorf("MinValidityPeriod = %v, want %v", MinValidityPeriod, time.Hour)
	}
	if MaxCertificateNameLength != 255 {
		t.Errorf("MaxCertificateNameLength = %v, want %v", MaxCertificateNameLength, 255)
	}
}

