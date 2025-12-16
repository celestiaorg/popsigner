// Package service provides business logic implementations.
package service

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Bidon15/popsigner/control-plane/internal/models"
	apierrors "github.com/Bidon15/popsigner/control-plane/internal/pkg/errors"
	"github.com/Bidon15/popsigner/control-plane/internal/repository"
)

// PKIInterface defines the interface for PKI/Certificate Authority operations.
// This allows the certificate service to work with the OpenBao PKI client.
type PKIInterface interface {
	// IssueCertificate issues a new client certificate.
	IssueCertificate(ctx context.Context, req *IssueCertRequest) (*IssuedCertificate, error)
	// RevokeCertificate revokes a certificate by serial number.
	RevokeCertificate(ctx context.Context, serialNumber string) error
	// GetCACertificate retrieves the CA certificate.
	GetCACertificate(ctx context.Context) (*CACertificate, error)
}

// IssueCertRequest represents a request to issue a certificate.
type IssueCertRequest struct {
	CommonName string // Format: org_{org_id}
	TTL        string // e.g., "8760h" for 1 year
}

// IssuedCertificate represents an issued certificate.
type IssuedCertificate struct {
	CertificatePEM string
	PrivateKeyPEM  string
	CACertPEM      string
	SerialNumber   string
	IssuedAt       time.Time
	ExpiresAt      time.Time
}

// CACertificate represents the CA certificate.
type CACertificate struct {
	CertificatePEM string
	ExpiresAt      time.Time
}

// CertificateService defines the interface for certificate management operations.
type CertificateService interface {
	// Issue creates a new client certificate for an organization.
	Issue(ctx context.Context, req *models.CreateCertificateRequest) (*models.CertificateBundle, error)
	// Revoke revokes a certificate.
	Revoke(ctx context.Context, orgID, certID, reason string) error
	// Get retrieves a certificate by ID.
	Get(ctx context.Context, orgID, certID string) (*models.Certificate, error)
	// List retrieves all certificates for an organization.
	List(ctx context.Context, orgID string, filter repository.CertificateStatusFilter) (*models.CertificateListResponse, error)
	// Delete removes a certificate (must be revoked first).
	Delete(ctx context.Context, orgID, certID string) error
	// ValidateForAuth checks if a certificate fingerprint is valid for authentication.
	ValidateForAuth(ctx context.Context, fingerprint string) (string, error)
	// GetCACertificate returns the CA certificate PEM.
	GetCACertificate(ctx context.Context) ([]byte, error)
	// DownloadBundle returns the certificate bundle for download (only available right after issue).
	DownloadBundle(ctx context.Context, orgID, certID string) (*models.CertificateBundle, error)
}

type certificateService struct {
	repo      repository.CertificateRepository
	pki       PKIInterface
	orgRepo   repository.OrgRepository
	auditRepo repository.AuditRepository
}

// NewCertificateService creates a new certificate service.
func NewCertificateService(
	repo repository.CertificateRepository,
	pki PKIInterface,
	orgRepo repository.OrgRepository,
	auditRepo repository.AuditRepository,
) CertificateService {
	return &certificateService{
		repo:      repo,
		pki:       pki,
		orgRepo:   orgRepo,
		auditRepo: auditRepo,
	}
}

// Issue creates a new client certificate for an organization.
func (s *certificateService) Issue(ctx context.Context, req *models.CreateCertificateRequest) (*models.CertificateBundle, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, apierrors.ErrBadRequest.WithMessage(err.Error())
	}

	// Check if org exists
	org, err := s.orgRepo.GetByID(ctx, req.OrgID)
	if err != nil {
		return nil, fmt.Errorf("checking organization: %w", err)
	}
	if org == nil {
		return nil, apierrors.NewNotFoundError("Organization")
	}

	// Check for duplicate name
	existing, err := s.repo.GetByOrgAndName(ctx, req.OrgID.String(), req.Name)
	if err != nil {
		return nil, fmt.Errorf("checking existing certificate: %w", err)
	}
	if existing != nil {
		return nil, apierrors.ErrBadRequest.WithMessage(
			fmt.Sprintf("certificate with name '%s' already exists", req.Name),
		)
	}

	// Issue certificate via OpenBao PKI
	cn := models.CNFromOrgID(req.OrgID.String())
	ttl := fmt.Sprintf("%dh", int(req.ValidityPeriod.Hours()))

	issued, err := s.pki.IssueCertificate(ctx, &IssueCertRequest{
		CommonName: cn,
		TTL:        ttl,
	})
	if err != nil {
		return nil, fmt.Errorf("issuing certificate: %w", err)
	}

	// Calculate fingerprint
	fingerprint, err := calculateFingerprint(issued.CertificatePEM)
	if err != nil {
		return nil, fmt.Errorf("calculating fingerprint: %w", err)
	}

	// Store certificate metadata
	cert := &models.Certificate{
		ID:           uuid.New(),
		OrgID:        req.OrgID,
		Name:         req.Name,
		Fingerprint:  fingerprint,
		CommonName:   cn,
		SerialNumber: issued.SerialNumber,
		IssuedAt:     issued.IssuedAt,
		ExpiresAt:    issued.ExpiresAt,
		CreatedAt:    time.Now(),
	}

	if err := s.repo.Create(ctx, cert); err != nil {
		// Revoke the certificate if we can't store metadata
		_ = s.pki.RevokeCertificate(ctx, issued.SerialNumber)
		return nil, fmt.Errorf("storing certificate: %w", err)
	}

	// Audit log (using key event type as placeholder until certificate events are added)
	s.auditLog(ctx, req.OrgID, models.AuditEventKeyCreated, "certificate")

	// Build configuration hint for Nitro
	nitroTip := `# Arbitrum Nitro configuration
--node.batch-poster.data-poster.external-signer.url=https://YOUR_POPSIGNER_HOST:8545/rpc
--node.batch-poster.data-poster.external-signer.address=YOUR_ETH_ADDRESS
--node.batch-poster.data-poster.external-signer.method=eth_signTransaction
--node.batch-poster.data-poster.external-signer.root-ca=/path/to/popsigner-ca.crt
--node.batch-poster.data-poster.external-signer.client-cert=/path/to/client.crt
--node.batch-poster.data-poster.external-signer.client-private-key=/path/to/client.key`

	return &models.CertificateBundle{
		ClientCert:     []byte(issued.CertificatePEM),
		ClientKey:      []byte(issued.PrivateKeyPEM),
		CACert:         []byte(issued.CACertPEM),
		Fingerprint:    fingerprint,
		ExpiresAt:      issued.ExpiresAt.Format(time.RFC3339),
		NitroConfigTip: nitroTip,
	}, nil
}

// Revoke revokes a certificate.
func (s *certificateService) Revoke(ctx context.Context, orgID, certID, reason string) error {
	// Get certificate
	cert, err := s.repo.GetByID(ctx, certID)
	if err != nil {
		return fmt.Errorf("getting certificate: %w", err)
	}
	if cert == nil {
		return apierrors.NewNotFoundError("Certificate")
	}

	// Verify ownership
	orgUUID, err := uuid.Parse(orgID)
	if err != nil || cert.OrgID != orgUUID {
		return apierrors.NewNotFoundError("Certificate") // Don't reveal it exists
	}

	// Check if already revoked
	if cert.IsRevoked() {
		return apierrors.ErrBadRequest.WithMessage("certificate already revoked")
	}

	// Revoke in OpenBao
	if err := s.pki.RevokeCertificate(ctx, cert.SerialNumber); err != nil {
		return fmt.Errorf("revoking in PKI: %w", err)
	}

	// Update database
	if err := s.repo.Revoke(ctx, certID, reason); err != nil {
		return fmt.Errorf("updating certificate: %w", err)
	}

	// Audit log (using key event type as placeholder until certificate events are added)
	s.auditLog(ctx, orgUUID, models.AuditEventKeyDeleted, "certificate")

	return nil
}

// Get retrieves a certificate by ID.
func (s *certificateService) Get(ctx context.Context, orgID, certID string) (*models.Certificate, error) {
	cert, err := s.repo.GetByID(ctx, certID)
	if err != nil {
		return nil, fmt.Errorf("getting certificate: %w", err)
	}
	if cert == nil {
		return nil, nil
	}

	// Verify ownership
	orgUUID, err := uuid.Parse(orgID)
	if err != nil || cert.OrgID != orgUUID {
		return nil, nil // Don't reveal it exists
	}

	return cert, nil
}

// List retrieves all certificates for an organization.
func (s *certificateService) List(ctx context.Context, orgID string, filter repository.CertificateStatusFilter) (*models.CertificateListResponse, error) {
	certs, err := s.repo.ListByOrg(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("listing certificates: %w", err)
	}

	// Convert to value slice for response
	certList := make([]models.Certificate, len(certs))
	for i, cert := range certs {
		certList[i] = *cert
	}

	return &models.CertificateListResponse{
		Certificates: certList,
		Total:        len(certList),
	}, nil
}

// Delete removes a certificate (must be revoked first).
func (s *certificateService) Delete(ctx context.Context, orgID, certID string) error {
	cert, err := s.repo.GetByID(ctx, certID)
	if err != nil {
		return fmt.Errorf("getting certificate: %w", err)
	}
	if cert == nil {
		return apierrors.NewNotFoundError("Certificate")
	}

	// Verify ownership
	orgUUID, err := uuid.Parse(orgID)
	if err != nil || cert.OrgID != orgUUID {
		return apierrors.NewNotFoundError("Certificate")
	}

	// Must be revoked before deletion
	if !cert.IsRevoked() {
		return apierrors.ErrBadRequest.WithMessage("certificate must be revoked before deletion")
	}

	if err := s.repo.Delete(ctx, certID); err != nil {
		return fmt.Errorf("deleting certificate: %w", err)
	}

	return nil
}

// ValidateForAuth checks if a certificate fingerprint is valid for authentication.
// Returns the organization ID if valid, empty string if not.
func (s *certificateService) ValidateForAuth(ctx context.Context, fingerprint string) (string, error) {
	cert, err := s.repo.IsValid(ctx, fingerprint)
	if err != nil {
		return "", fmt.Errorf("validating certificate: %w", err)
	}
	if cert == nil {
		return "", nil // Not valid
	}

	return cert.OrgID.String(), nil
}

// GetCACertificate returns the CA certificate PEM.
func (s *certificateService) GetCACertificate(ctx context.Context) ([]byte, error) {
	ca, err := s.pki.GetCACertificate(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting CA certificate: %w", err)
	}

	return []byte(ca.CertificatePEM), nil
}

// DownloadBundle returns the certificate bundle for download.
// Note: The private key is only available immediately after issue.
// This method is a placeholder for future implementation of secure key storage.
func (s *certificateService) DownloadBundle(ctx context.Context, orgID, certID string) (*models.CertificateBundle, error) {
	// Get certificate
	cert, err := s.repo.GetByID(ctx, certID)
	if err != nil {
		return nil, fmt.Errorf("getting certificate: %w", err)
	}
	if cert == nil {
		return nil, apierrors.NewNotFoundError("Certificate")
	}

	// Verify ownership
	orgUUID, err := uuid.Parse(orgID)
	if err != nil || cert.OrgID != orgUUID {
		return nil, apierrors.NewNotFoundError("Certificate")
	}

	// Get CA certificate
	ca, err := s.pki.GetCACertificate(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting CA certificate: %w", err)
	}

	// Note: We can't return the private key here as it was only provided during issue
	// The bundle will only include the CA cert for the client to verify
	return &models.CertificateBundle{
		CACert:      []byte(ca.CertificatePEM),
		Fingerprint: cert.Fingerprint,
		ExpiresAt:   cert.ExpiresAt.Format(time.RFC3339),
		NitroConfigTip: `# Note: Private key was only available during certificate issuance.
# If you need a new certificate, please issue a new one.`,
	}, nil
}

// calculateFingerprint computes SHA256 fingerprint from PEM certificate.
func calculateFingerprint(certPEM string) (string, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parsing certificate: %w", err)
	}

	hash := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(hash[:]), nil
}

// auditLog creates an audit log entry asynchronously.
func (s *certificateService) auditLog(ctx context.Context, orgID uuid.UUID, event models.AuditEvent, resourceType string) {
	go func() {
		resType := models.ResourceType(resourceType)
		_ = s.auditRepo.Create(context.Background(), &models.AuditLog{
			OrgID:        orgID,
			Event:        event,
			ActorType:    models.ActorTypeAPIKey,
			ResourceType: &resType,
		})
	}()
}

// Compile-time check to ensure certificateService implements CertificateService.
var _ CertificateService = (*certificateService)(nil)

