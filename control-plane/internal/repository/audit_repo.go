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

// AuditRepository defines the interface for audit log operations.
type AuditRepository interface {
	Create(ctx context.Context, log *models.AuditLog) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error)
	List(ctx context.Context, query models.AuditLogQuery) ([]*models.AuditLog, error)
	DeleteBefore(ctx context.Context, orgID uuid.UUID, before time.Time) (int64, error)
}

type auditRepo struct {
	pool *pgxpool.Pool
}

// NewAuditRepository creates a new audit log repository.
func NewAuditRepository(pool *pgxpool.Pool) AuditRepository {
	return &auditRepo{pool: pool}
}

// Create inserts a new audit log entry.
func (r *auditRepo) Create(ctx context.Context, log *models.AuditLog) error {
	query := `
		INSERT INTO audit_logs (id, org_id, event, actor_id, actor_type, resource_type, resource_id, ip_address, user_agent, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at`

	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}

	return r.pool.QueryRow(ctx, query,
		log.ID,
		log.OrgID,
		log.Event,
		log.ActorID,
		log.ActorType,
		log.ResourceType,
		log.ResourceID,
		log.IPAddress,
		log.UserAgent,
		log.Metadata,
	).Scan(&log.CreatedAt)
}

// GetByID retrieves an audit log by ID.
func (r *auditRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	query := `
		SELECT id, org_id, event, actor_id, actor_type, resource_type, resource_id, ip_address, user_agent, metadata, created_at
		FROM audit_logs WHERE id = $1`

	var log models.AuditLog
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&log.ID,
		&log.OrgID,
		&log.Event,
		&log.ActorID,
		&log.ActorType,
		&log.ResourceType,
		&log.ResourceID,
		&log.IPAddress,
		&log.UserAgent,
		&log.Metadata,
		&log.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// List retrieves audit logs based on query parameters.
func (r *auditRepo) List(ctx context.Context, q models.AuditLogQuery) ([]*models.AuditLog, error) {
	// Build dynamic query
	baseQuery := `
		SELECT id, org_id, event, actor_id, actor_type, resource_type, resource_id, ip_address, user_agent, metadata, created_at
		FROM audit_logs 
		WHERE org_id = $1`

	args := []any{q.OrgID}
	argIndex := 2

	if q.Event != nil {
		baseQuery += ` AND event = $` + string(rune('0'+argIndex))
		args = append(args, *q.Event)
		argIndex++
	}

	if q.ActorID != nil {
		baseQuery += ` AND actor_id = $` + string(rune('0'+argIndex))
		args = append(args, *q.ActorID)
		argIndex++
	}

	if q.ResourceType != nil {
		baseQuery += ` AND resource_type = $` + string(rune('0'+argIndex))
		args = append(args, *q.ResourceType)
		argIndex++
	}

	if q.ResourceID != nil {
		baseQuery += ` AND resource_id = $` + string(rune('0'+argIndex))
		args = append(args, *q.ResourceID)
		argIndex++
	}

	if q.StartTime != nil {
		baseQuery += ` AND created_at >= $` + string(rune('0'+argIndex))
		args = append(args, *q.StartTime)
		argIndex++
	}

	if q.EndTime != nil {
		baseQuery += ` AND created_at <= $` + string(rune('0'+argIndex))
		args = append(args, *q.EndTime)
		argIndex++
	}

	baseQuery += ` ORDER BY created_at DESC`

	limit := q.Limit
	if limit == 0 || limit > 100 {
		limit = 100
	}
	baseQuery += ` LIMIT $` + string(rune('0'+argIndex))
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(
			&log.ID,
			&log.OrgID,
			&log.Event,
			&log.ActorID,
			&log.ActorType,
			&log.ResourceType,
			&log.ResourceID,
			&log.IPAddress,
			&log.UserAgent,
			&log.Metadata,
			&log.CreatedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, &log)
	}
	return logs, rows.Err()
}

// DeleteBefore deletes audit logs older than the given time for an organization.
// Used for retention policy enforcement.
func (r *auditRepo) DeleteBefore(ctx context.Context, orgID uuid.UUID, before time.Time) (int64, error) {
	query := `DELETE FROM audit_logs WHERE org_id = $1 AND created_at < $2`
	result, err := r.pool.Exec(ctx, query, orgID, before)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// Compile-time check to ensure auditRepo implements AuditRepository.
var _ AuditRepository = (*auditRepo)(nil)

