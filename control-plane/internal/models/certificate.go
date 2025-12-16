// Package models contains data models for the control plane.
package models

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Certificate represents a client certificate issued by POPSigner CA.
type Certificate struct {
	ID               uuid.UUID  `json:"id" db:"id"`                               // UUID
	OrgID            uuid.UUID  `json:"org_id" db:"org_id"`                       // Organization owner
	Name             string     `json:"name" db:"name"`                           // User-friendly name
	Fingerprint      string     `json:"fingerprint" db:"fingerprint"`             // SHA256 of DER-encoded cert
	CommonName       string     `json:"common_name" db:"common_name"`             // CN from certificate
	SerialNumber     string     `json:"serial_number" db:"serial_number"`         // Certificate serial
	IssuedAt         time.Time  `json:"issued_at" db:"issued_at"`                 // When cert was issued
	ExpiresAt        time.Time  `json:"expires_at" db:"expires_at"`               // Expiration time
	RevokedAt        *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`     // NULL if not revoked
	RevocationReason *string    `json:"revocation_reason,omitempty" db:"revocation_reason"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
}

// IsRevoked returns true if the certificate has been revoked.
func (c *Certificate) IsRevoked() bool {
	return c.RevokedAt != nil
}

// IsExpired returns true if the certificate has expired.
func (c *Certificate) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// IsValid returns true if the certificate is not revoked and not expired.
func (c *Certificate) IsValid() bool {
	return !c.IsRevoked() && !c.IsExpired()
}

// CertificateStatus represents the current status of a certificate.
type CertificateStatus string

const (
	CertificateStatusActive  CertificateStatus = "active"
	CertificateStatusExpired CertificateStatus = "expired"
	CertificateStatusRevoked CertificateStatus = "revoked"
)

// Status returns the current status of the certificate.
func (c *Certificate) Status() CertificateStatus {
	if c.IsRevoked() {
		return CertificateStatusRevoked
	}
	if c.IsExpired() {
		return CertificateStatusExpired
	}
	return CertificateStatusActive
}

// OrgIDFromCN extracts the organization ID from a certificate Common Name.
// Expected format: "org_01J5K7XXXXXXXXXXX" or "org_{ulid}"
func OrgIDFromCN(cn string) (string, error) {
	if !strings.HasPrefix(cn, "org_") {
		return "", fmt.Errorf("invalid CN format: must start with 'org_'")
	}
	return cn, nil
}

// CNFromOrgID creates a Common Name from an organization ID.
func CNFromOrgID(orgID string) string {
	// Ensure the org ID has the correct prefix
	if !strings.HasPrefix(orgID, "org_") {
		return "org_" + orgID
	}
	return orgID
}

// FingerprintFromCert computes the SHA256 fingerprint of a certificate.
func FingerprintFromCert(cert *x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(hash[:])
}

// CreateCertificateRequest represents a request to create a new certificate.
type CreateCertificateRequest struct {
	OrgID          uuid.UUID     `json:"org_id" validate:"required"`
	Name           string        `json:"name" validate:"required,min=1,max=255"`
	ValidityPeriod time.Duration `json:"validity_period,omitempty"` // Default: 365 days
}

// DefaultValidityPeriod is the default certificate validity (1 year).
const DefaultValidityPeriod = 365 * 24 * time.Hour

// MaxValidityPeriod is the maximum certificate validity (5 years).
const MaxValidityPeriod = 5 * 365 * 24 * time.Hour

// MinValidityPeriod is the minimum certificate validity (1 hour).
const MinValidityPeriod = time.Hour

// MaxCertificateNameLength is the maximum length for certificate names.
const MaxCertificateNameLength = 255

// Validate validates the create certificate request.
func (r *CreateCertificateRequest) Validate() error {
	if r.OrgID == uuid.Nil {
		return fmt.Errorf("org_id is required")
	}
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(r.Name) > MaxCertificateNameLength {
		return fmt.Errorf("name must be %d characters or less", MaxCertificateNameLength)
	}
	if r.ValidityPeriod == 0 {
		r.ValidityPeriod = DefaultValidityPeriod
	}
	if r.ValidityPeriod < MinValidityPeriod {
		return fmt.Errorf("validity_period must be at least 1 hour")
	}
	if r.ValidityPeriod > MaxValidityPeriod {
		return fmt.Errorf("validity_period cannot exceed 5 years")
	}
	return nil
}

// RevokeCertificateRequest represents a request to revoke a certificate.
type RevokeCertificateRequest struct {
	CertificateID string `json:"certificate_id" validate:"required"`
	Reason        string `json:"reason,omitempty"`
}

// CertificateBundle represents a downloadable certificate bundle.
type CertificateBundle struct {
	ClientCert     []byte `json:"client_cert"`      // PEM-encoded client certificate
	ClientKey      []byte `json:"client_key"`       // PEM-encoded private key
	CACert         []byte `json:"ca_cert"`          // PEM-encoded CA certificate
	Fingerprint    string `json:"fingerprint"`      // Certificate fingerprint
	ExpiresAt      string `json:"expires_at"`       // ISO8601 expiration
	NitroConfigTip string `json:"nitro_config_tip"` // Configuration hint for Nitro
}

// CertificateListResponse represents a list of certificates.
type CertificateListResponse struct {
	Certificates []Certificate `json:"certificates"`
	Total        int           `json:"total"`
}

// CertificateResponse is the API response format for a single certificate.
type CertificateResponse struct {
	ID               uuid.UUID         `json:"id"`
	OrgID            uuid.UUID         `json:"org_id"`
	Name             string            `json:"name"`
	Fingerprint      string            `json:"fingerprint"`
	CommonName       string            `json:"common_name"`
	SerialNumber     string            `json:"serial_number"`
	Status           CertificateStatus `json:"status"`
	IssuedAt         time.Time         `json:"issued_at"`
	ExpiresAt        time.Time         `json:"expires_at"`
	RevokedAt        *time.Time        `json:"revoked_at,omitempty"`
	RevocationReason *string           `json:"revocation_reason,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
}

// ToResponse converts a Certificate to a CertificateResponse.
func (c *Certificate) ToResponse() *CertificateResponse {
	return &CertificateResponse{
		ID:               c.ID,
		OrgID:            c.OrgID,
		Name:             c.Name,
		Fingerprint:      c.Fingerprint,
		CommonName:       c.CommonName,
		SerialNumber:     c.SerialNumber,
		Status:           c.Status(),
		IssuedAt:         c.IssuedAt,
		ExpiresAt:        c.ExpiresAt,
		RevokedAt:        c.RevokedAt,
		RevocationReason: c.RevocationReason,
		CreatedAt:        c.CreatedAt,
	}
}

