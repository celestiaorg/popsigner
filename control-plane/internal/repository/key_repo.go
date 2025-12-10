// Package repository provides data access layer implementations.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Bidon15/banhbaoring/control-plane/internal/models"
)

// KeyRepository defines the interface for cryptographic key data operations.
type KeyRepository interface {
	Create(ctx context.Context, key *models.Key) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Key, error)
	GetByName(ctx context.Context, orgID, namespaceID uuid.UUID, name string) (*models.Key, error)
	GetByAddress(ctx context.Context, orgID uuid.UUID, address string) (*models.Key, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*models.Key, error)
	ListByNamespace(ctx context.Context, namespaceID uuid.UUID) ([]*models.Key, error)
	CountByOrg(ctx context.Context, orgID uuid.UUID) (int, error)
	Update(ctx context.Context, key *models.Key) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type keyRepo struct {
	pool *pgxpool.Pool
}

// NewKeyRepository creates a new key repository.
func NewKeyRepository(pool *pgxpool.Pool) KeyRepository {
	return &keyRepo{pool: pool}
}

// Create inserts a new key into the database.
func (r *keyRepo) Create(ctx context.Context, key *models.Key) error {
	query := `
		INSERT INTO keys (id, org_id, namespace_id, name, public_key, address, algorithm, bao_key_path, exportable, metadata, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at`

	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}
	if key.Version == 0 {
		key.Version = 1
	}

	return r.pool.QueryRow(ctx, query,
		key.ID,
		key.OrgID,
		key.NamespaceID,
		key.Name,
		key.PublicKey,
		key.Address,
		key.Algorithm,
		key.BaoKeyPath,
		key.Exportable,
		key.Metadata,
		key.Version,
	).Scan(&key.CreatedAt, &key.UpdatedAt)
}

// GetByID retrieves a key by its UUID.
func (r *keyRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Key, error) {
	query := `
		SELECT id, org_id, namespace_id, name, public_key, address, algorithm, 
		       bao_key_path, exportable, metadata, version, deleted_at, created_at, updated_at
		FROM keys WHERE id = $1`

	var key models.Key
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&key.ID,
		&key.OrgID,
		&key.NamespaceID,
		&key.Name,
		&key.PublicKey,
		&key.Address,
		&key.Algorithm,
		&key.BaoKeyPath,
		&key.Exportable,
		&key.Metadata,
		&key.Version,
		&key.DeletedAt,
		&key.CreatedAt,
		&key.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}

// GetByName retrieves a key by org, namespace, and name.
func (r *keyRepo) GetByName(ctx context.Context, orgID, namespaceID uuid.UUID, name string) (*models.Key, error) {
	query := `
		SELECT id, org_id, namespace_id, name, public_key, address, algorithm, 
		       bao_key_path, exportable, metadata, version, deleted_at, created_at, updated_at
		FROM keys 
		WHERE org_id = $1 AND namespace_id = $2 AND name = $3 AND deleted_at IS NULL`

	var key models.Key
	err := r.pool.QueryRow(ctx, query, orgID, namespaceID, name).Scan(
		&key.ID,
		&key.OrgID,
		&key.NamespaceID,
		&key.Name,
		&key.PublicKey,
		&key.Address,
		&key.Algorithm,
		&key.BaoKeyPath,
		&key.Exportable,
		&key.Metadata,
		&key.Version,
		&key.DeletedAt,
		&key.CreatedAt,
		&key.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}

// GetByAddress retrieves a key by its address within an organization.
func (r *keyRepo) GetByAddress(ctx context.Context, orgID uuid.UUID, address string) (*models.Key, error) {
	query := `
		SELECT id, org_id, namespace_id, name, public_key, address, algorithm, 
		       bao_key_path, exportable, metadata, version, deleted_at, created_at, updated_at
		FROM keys 
		WHERE org_id = $1 AND address = $2 AND deleted_at IS NULL`

	var key models.Key
	err := r.pool.QueryRow(ctx, query, orgID, address).Scan(
		&key.ID,
		&key.OrgID,
		&key.NamespaceID,
		&key.Name,
		&key.PublicKey,
		&key.Address,
		&key.Algorithm,
		&key.BaoKeyPath,
		&key.Exportable,
		&key.Metadata,
		&key.Version,
		&key.DeletedAt,
		&key.CreatedAt,
		&key.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}

// ListByOrg retrieves all non-deleted keys for an organization.
func (r *keyRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*models.Key, error) {
	query := `
		SELECT id, org_id, namespace_id, name, public_key, address, algorithm, 
		       bao_key_path, exportable, metadata, version, deleted_at, created_at, updated_at
		FROM keys 
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*models.Key
	for rows.Next() {
		var key models.Key
		if err := rows.Scan(
			&key.ID,
			&key.OrgID,
			&key.NamespaceID,
			&key.Name,
			&key.PublicKey,
			&key.Address,
			&key.Algorithm,
			&key.BaoKeyPath,
			&key.Exportable,
			&key.Metadata,
			&key.Version,
			&key.DeletedAt,
			&key.CreatedAt,
			&key.UpdatedAt,
		); err != nil {
			return nil, err
		}
		keys = append(keys, &key)
	}
	return keys, rows.Err()
}

// ListByNamespace retrieves all non-deleted keys for a namespace.
func (r *keyRepo) ListByNamespace(ctx context.Context, namespaceID uuid.UUID) ([]*models.Key, error) {
	query := `
		SELECT id, org_id, namespace_id, name, public_key, address, algorithm, 
		       bao_key_path, exportable, metadata, version, deleted_at, created_at, updated_at
		FROM keys 
		WHERE namespace_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, namespaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*models.Key
	for rows.Next() {
		var key models.Key
		if err := rows.Scan(
			&key.ID,
			&key.OrgID,
			&key.NamespaceID,
			&key.Name,
			&key.PublicKey,
			&key.Address,
			&key.Algorithm,
			&key.BaoKeyPath,
			&key.Exportable,
			&key.Metadata,
			&key.Version,
			&key.DeletedAt,
			&key.CreatedAt,
			&key.UpdatedAt,
		); err != nil {
			return nil, err
		}
		keys = append(keys, &key)
	}
	return keys, rows.Err()
}

// CountByOrg returns the count of non-deleted keys for an organization.
func (r *keyRepo) CountByOrg(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM keys WHERE org_id = $1 AND deleted_at IS NULL`
	var count int
	err := r.pool.QueryRow(ctx, query, orgID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Update updates a key's metadata.
func (r *keyRepo) Update(ctx context.Context, key *models.Key) error {
	query := `
		UPDATE keys 
		SET name = $2, metadata = $3, version = version + 1
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING version, updated_at`

	err := r.pool.QueryRow(ctx, query, key.ID, key.Name, key.Metadata).Scan(&key.Version, &key.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return pgx.ErrNoRows
	}
	return err
}

// SoftDelete marks a key as deleted.
func (r *keyRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE keys SET deleted_at = $2 WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.pool.Exec(ctx, query, id, time.Now())
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// Delete permanently removes a key.
func (r *keyRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM keys WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// Compile-time check to ensure keyRepo implements KeyRepository.
var _ KeyRepository = (*keyRepo)(nil)

