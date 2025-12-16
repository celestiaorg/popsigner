// Package repository provides data access layer implementations.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Bidon15/popsigner/control-plane/internal/models"
)

// CertificateStatusFilter defines filters for certificate listing.
type CertificateStatusFilter string

const (
	// CertificateFilterAll returns all certificates.
	CertificateFilterAll CertificateStatusFilter = "all"
	// CertificateFilterActive returns only active (non-revoked, non-expired) certificates.
	CertificateFilterActive CertificateStatusFilter = "active"
	// CertificateFilterRevoked returns only revoked certificates.
	CertificateFilterRevoked CertificateStatusFilter = "revoked"
	// CertificateFilterExpired returns only expired certificates.
	CertificateFilterExpired CertificateStatusFilter = "expired"
)

// CertificateRepository defines the interface for certificate data operations.
type CertificateRepository interface {
	Create(ctx context.Context, cert *models.Certificate) error
	GetByID(ctx context.Context, id string) (*models.Certificate, error)
	GetByFingerprint(ctx context.Context, fingerprint string) (*models.Certificate, error)
	GetBySerialNumber(ctx context.Context, serialNumber string) (*models.Certificate, error)
	GetByOrgAndName(ctx context.Context, orgID, name string) (*models.Certificate, error)
	ListByOrg(ctx context.Context, orgID string, filter CertificateStatusFilter) ([]*models.Certificate, error)
	ListActiveByOrg(ctx context.Context, orgID string) ([]*models.Certificate, error)
	CountByOrg(ctx context.Context, orgID string) (int, error)
	Revoke(ctx context.Context, id string, reason string) error
	Delete(ctx context.Context, id string) error
	IsValid(ctx context.Context, fingerprint string) (*models.Certificate, error)
	ListExpiringSoon(ctx context.Context, within time.Duration) ([]*models.Certificate, error)
}

type certificateRepo struct {
	pool *pgxpool.Pool
}

// NewCertificateRepository creates a new certificate repository.
func NewCertificateRepository(pool *pgxpool.Pool) CertificateRepository {
	return &certificateRepo{pool: pool}
}

// Create inserts a new certificate record.
func (r *certificateRepo) Create(ctx context.Context, cert *models.Certificate) error {
	query := `
		INSERT INTO client_certificates (
			id, org_id, name, fingerprint, common_name, serial_number,
			issued_at, expires_at, revoked_at, revocation_reason, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)`

	if cert.CreatedAt.IsZero() {
		cert.CreatedAt = time.Now()
	}

	_, err := r.pool.Exec(ctx, query,
		cert.ID,
		cert.OrgID,
		cert.Name,
		cert.Fingerprint,
		cert.CommonName,
		cert.SerialNumber,
		cert.IssuedAt,
		cert.ExpiresAt,
		cert.RevokedAt,
		cert.RevocationReason,
		cert.CreatedAt,
	)
	return err
}

// GetByID retrieves a certificate by ID.
func (r *certificateRepo) GetByID(ctx context.Context, id string) (*models.Certificate, error) {
	query := `
		SELECT id, org_id, name, fingerprint, common_name, serial_number,
		       issued_at, expires_at, revoked_at, revocation_reason, created_at
		FROM client_certificates WHERE id = $1`

	var cert models.Certificate
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&cert.ID,
		&cert.OrgID,
		&cert.Name,
		&cert.Fingerprint,
		&cert.CommonName,
		&cert.SerialNumber,
		&cert.IssuedAt,
		&cert.ExpiresAt,
		&cert.RevokedAt,
		&cert.RevocationReason,
		&cert.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// GetByFingerprint retrieves a certificate by fingerprint.
// This is used during mTLS authentication to validate client certificates.
func (r *certificateRepo) GetByFingerprint(ctx context.Context, fingerprint string) (*models.Certificate, error) {
	query := `
		SELECT id, org_id, name, fingerprint, common_name, serial_number,
		       issued_at, expires_at, revoked_at, revocation_reason, created_at
		FROM client_certificates WHERE fingerprint = $1`

	var cert models.Certificate
	err := r.pool.QueryRow(ctx, query, fingerprint).Scan(
		&cert.ID,
		&cert.OrgID,
		&cert.Name,
		&cert.Fingerprint,
		&cert.CommonName,
		&cert.SerialNumber,
		&cert.IssuedAt,
		&cert.ExpiresAt,
		&cert.RevokedAt,
		&cert.RevocationReason,
		&cert.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// GetBySerialNumber retrieves a certificate by serial number.
func (r *certificateRepo) GetBySerialNumber(ctx context.Context, serialNumber string) (*models.Certificate, error) {
	query := `
		SELECT id, org_id, name, fingerprint, common_name, serial_number,
		       issued_at, expires_at, revoked_at, revocation_reason, created_at
		FROM client_certificates WHERE serial_number = $1`

	var cert models.Certificate
	err := r.pool.QueryRow(ctx, query, serialNumber).Scan(
		&cert.ID,
		&cert.OrgID,
		&cert.Name,
		&cert.Fingerprint,
		&cert.CommonName,
		&cert.SerialNumber,
		&cert.IssuedAt,
		&cert.ExpiresAt,
		&cert.RevokedAt,
		&cert.RevocationReason,
		&cert.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// GetByOrgAndName retrieves a certificate by organization ID and name.
func (r *certificateRepo) GetByOrgAndName(ctx context.Context, orgID, name string) (*models.Certificate, error) {
	query := `
		SELECT id, org_id, name, fingerprint, common_name, serial_number,
		       issued_at, expires_at, revoked_at, revocation_reason, created_at
		FROM client_certificates WHERE org_id = $1 AND name = $2`

	var cert models.Certificate
	err := r.pool.QueryRow(ctx, query, orgID, name).Scan(
		&cert.ID,
		&cert.OrgID,
		&cert.Name,
		&cert.Fingerprint,
		&cert.CommonName,
		&cert.SerialNumber,
		&cert.IssuedAt,
		&cert.ExpiresAt,
		&cert.RevokedAt,
		&cert.RevocationReason,
		&cert.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// ListByOrg retrieves all certificates for an organization with optional status filter.
func (r *certificateRepo) ListByOrg(ctx context.Context, orgID string, filter CertificateStatusFilter) ([]*models.Certificate, error) {
	var query string

	switch filter {
	case CertificateFilterActive:
		query = `
			SELECT id, org_id, name, fingerprint, common_name, serial_number,
			       issued_at, expires_at, revoked_at, revocation_reason, created_at
			FROM client_certificates 
			WHERE org_id = $1 AND revoked_at IS NULL AND expires_at > NOW()
			ORDER BY created_at DESC`
	case CertificateFilterRevoked:
		query = `
			SELECT id, org_id, name, fingerprint, common_name, serial_number,
			       issued_at, expires_at, revoked_at, revocation_reason, created_at
			FROM client_certificates 
			WHERE org_id = $1 AND revoked_at IS NOT NULL
			ORDER BY created_at DESC`
	case CertificateFilterExpired:
		query = `
			SELECT id, org_id, name, fingerprint, common_name, serial_number,
			       issued_at, expires_at, revoked_at, revocation_reason, created_at
			FROM client_certificates 
			WHERE org_id = $1 AND expires_at <= NOW() AND revoked_at IS NULL
			ORDER BY created_at DESC`
	default: // CertificateFilterAll or unknown
		query = `
			SELECT id, org_id, name, fingerprint, common_name, serial_number,
			       issued_at, expires_at, revoked_at, revocation_reason, created_at
			FROM client_certificates 
			WHERE org_id = $1
			ORDER BY created_at DESC`
	}

	rows, err := r.pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []*models.Certificate
	for rows.Next() {
		var cert models.Certificate
		if err := rows.Scan(
			&cert.ID,
			&cert.OrgID,
			&cert.Name,
			&cert.Fingerprint,
			&cert.CommonName,
			&cert.SerialNumber,
			&cert.IssuedAt,
			&cert.ExpiresAt,
			&cert.RevokedAt,
			&cert.RevocationReason,
			&cert.CreatedAt,
		); err != nil {
			return nil, err
		}
		certs = append(certs, &cert)
	}
	return certs, rows.Err()
}

// ListActiveByOrg retrieves all non-revoked, non-expired certificates for an organization.
func (r *certificateRepo) ListActiveByOrg(ctx context.Context, orgID string) ([]*models.Certificate, error) {
	return r.ListByOrg(ctx, orgID, CertificateFilterActive)
}

// CountByOrg returns the number of certificates for an organization.
func (r *certificateRepo) CountByOrg(ctx context.Context, orgID string) (int, error) {
	query := `SELECT COUNT(*) FROM client_certificates WHERE org_id = $1`
	var count int
	err := r.pool.QueryRow(ctx, query, orgID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Revoke marks a certificate as revoked.
func (r *certificateRepo) Revoke(ctx context.Context, id string, reason string) error {
	now := time.Now()
	query := `
		UPDATE client_certificates 
		SET revoked_at = $1, revocation_reason = $2 
		WHERE id = $3 AND revoked_at IS NULL`

	result, err := r.pool.Exec(ctx, query, now, reason, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// Delete removes a certificate record permanently.
func (r *certificateRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM client_certificates WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// IsValid checks if a certificate is valid (not revoked and not expired).
// Returns the certificate if valid, nil if not found or invalid.
func (r *certificateRepo) IsValid(ctx context.Context, fingerprint string) (*models.Certificate, error) {
	cert, err := r.GetByFingerprint(ctx, fingerprint)
	if err != nil {
		return nil, err
	}

	if cert == nil {
		return nil, nil // Not found
	}

	if !cert.IsValid() {
		return nil, nil // Revoked or expired
	}

	return cert, nil
}

// ListExpiringSoon retrieves certificates expiring within the given duration.
func (r *certificateRepo) ListExpiringSoon(ctx context.Context, within time.Duration) ([]*models.Certificate, error) {
	query := `
		SELECT id, org_id, name, fingerprint, common_name, serial_number,
		       issued_at, expires_at, revoked_at, revocation_reason, created_at
		FROM client_certificates 
		WHERE revoked_at IS NULL 
		  AND expires_at > NOW()
		  AND expires_at < NOW() + $1::interval
		ORDER BY expires_at ASC`

	rows, err := r.pool.Query(ctx, query, within)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []*models.Certificate
	for rows.Next() {
		var cert models.Certificate
		if err := rows.Scan(
			&cert.ID,
			&cert.OrgID,
			&cert.Name,
			&cert.Fingerprint,
			&cert.CommonName,
			&cert.SerialNumber,
			&cert.IssuedAt,
			&cert.ExpiresAt,
			&cert.RevokedAt,
			&cert.RevocationReason,
			&cert.CreatedAt,
		); err != nil {
			return nil, err
		}
		certs = append(certs, &cert)
	}
	return certs, rows.Err()
}

// Compile-time check to ensure certificateRepo implements CertificateRepository.
var _ CertificateRepository = (*certificateRepo)(nil)

