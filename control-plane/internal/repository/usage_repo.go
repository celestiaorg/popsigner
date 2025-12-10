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

// UsageRepository defines the interface for usage metric operations.
type UsageRepository interface {
	// Increment adds to a metric's value for the current period.
	Increment(ctx context.Context, orgID uuid.UUID, metric string, value int64) error

	// GetCurrentPeriod returns the current period's value for a metric.
	GetCurrentPeriod(ctx context.Context, orgID uuid.UUID, metric string) (int64, error)

	// GetMetric retrieves a specific usage metric.
	GetMetric(ctx context.Context, orgID uuid.UUID, metric string, periodStart time.Time) (*models.UsageMetric, error)

	// ListByOrg lists all usage metrics for an organization in a period.
	ListByOrg(ctx context.Context, orgID uuid.UUID, periodStart, periodEnd time.Time) ([]*models.UsageMetric, error)

	// GetSummary returns a usage summary for an organization.
	GetSummary(ctx context.Context, orgID uuid.UUID, plan models.Plan) (*models.UsageSummary, error)
}

type usageRepo struct {
	pool *pgxpool.Pool
}

// NewUsageRepository creates a new usage metric repository.
func NewUsageRepository(pool *pgxpool.Pool) UsageRepository {
	return &usageRepo{pool: pool}
}

// currentPeriodStart returns the start of the current monthly billing period.
func currentPeriodStart() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// currentPeriodEnd returns the end of the current monthly billing period.
func currentPeriodEnd() time.Time {
	start := currentPeriodStart()
	return start.AddDate(0, 1, 0).Add(-time.Nanosecond)
}

// Increment adds to a metric's value for the current period.
// Uses INSERT ... ON CONFLICT for atomic upsert.
func (r *usageRepo) Increment(ctx context.Context, orgID uuid.UUID, metric string, value int64) error {
	periodStart := currentPeriodStart()
	periodEnd := currentPeriodEnd()

	query := `
		INSERT INTO usage_metrics (id, org_id, metric, value, period_start, period_end)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (org_id, metric, period_start)
		DO UPDATE SET value = usage_metrics.value + EXCLUDED.value, updated_at = NOW()`

	_, err := r.pool.Exec(ctx, query,
		uuid.New(),
		orgID,
		metric,
		value,
		periodStart,
		periodEnd,
	)
	return err
}

// GetCurrentPeriod returns the current period's value for a metric.
func (r *usageRepo) GetCurrentPeriod(ctx context.Context, orgID uuid.UUID, metric string) (int64, error) {
	periodStart := currentPeriodStart()

	query := `
		SELECT value FROM usage_metrics 
		WHERE org_id = $1 AND metric = $2 AND period_start = $3`

	var value int64
	err := r.pool.QueryRow(ctx, query, orgID, metric, periodStart).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return value, nil
}

// GetMetric retrieves a specific usage metric.
func (r *usageRepo) GetMetric(ctx context.Context, orgID uuid.UUID, metric string, periodStart time.Time) (*models.UsageMetric, error) {
	query := `
		SELECT id, org_id, metric, value, period_start, period_end, created_at, updated_at
		FROM usage_metrics 
		WHERE org_id = $1 AND metric = $2 AND period_start = $3`

	var m models.UsageMetric
	err := r.pool.QueryRow(ctx, query, orgID, metric, periodStart).Scan(
		&m.ID,
		&m.OrgID,
		&m.Metric,
		&m.Value,
		&m.PeriodStart,
		&m.PeriodEnd,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ListByOrg lists all usage metrics for an organization in a period.
func (r *usageRepo) ListByOrg(ctx context.Context, orgID uuid.UUID, periodStart, periodEnd time.Time) ([]*models.UsageMetric, error) {
	query := `
		SELECT id, org_id, metric, value, period_start, period_end, created_at, updated_at
		FROM usage_metrics 
		WHERE org_id = $1 AND period_start >= $2 AND period_end <= $3
		ORDER BY period_start, metric`

	rows, err := r.pool.Query(ctx, query, orgID, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []*models.UsageMetric
	for rows.Next() {
		var m models.UsageMetric
		if err := rows.Scan(
			&m.ID,
			&m.OrgID,
			&m.Metric,
			&m.Value,
			&m.PeriodStart,
			&m.PeriodEnd,
			&m.CreatedAt,
			&m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		metrics = append(metrics, &m)
	}
	return metrics, rows.Err()
}

// GetSummary returns a usage summary for an organization.
func (r *usageRepo) GetSummary(ctx context.Context, orgID uuid.UUID, plan models.Plan) (*models.UsageSummary, error) {
	periodStart := currentPeriodStart()
	periodEnd := currentPeriodEnd()
	limits := models.GetPlanLimits(plan)

	// Get signature count for current period
	sigQuery := `
		SELECT COALESCE(value, 0) FROM usage_metrics 
		WHERE org_id = $1 AND metric = 'signatures' AND period_start = $2`

	var sigCount int64
	err := r.pool.QueryRow(ctx, sigQuery, orgID, periodStart).Scan(&sigCount)
	if errors.Is(err, pgx.ErrNoRows) {
		sigCount = 0
	} else if err != nil {
		return nil, err
	}

	// Get key count
	keyQuery := `SELECT COUNT(*) FROM keys WHERE org_id = $1 AND deleted_at IS NULL`
	var keyCount int
	err = r.pool.QueryRow(ctx, keyQuery, orgID).Scan(&keyCount)
	if err != nil {
		return nil, err
	}

	// Get namespace count
	nsQuery := `SELECT COUNT(*) FROM namespaces WHERE org_id = $1`
	var nsCount int
	err = r.pool.QueryRow(ctx, nsQuery, orgID).Scan(&nsCount)
	if err != nil {
		return nil, err
	}

	// Get member count
	memberQuery := `SELECT COUNT(*) FROM org_members WHERE org_id = $1`
	var memberCount int
	err = r.pool.QueryRow(ctx, memberQuery, orgID).Scan(&memberCount)
	if err != nil {
		return nil, err
	}

	return &models.UsageSummary{
		OrgID:            orgID,
		Plan:             plan,
		Keys:             keyCount,
		KeysLimit:        limits.Keys,
		SignaturesMonth:  sigCount,
		SignaturesLimit:  limits.SignaturesPerMonth,
		Namespaces:       nsCount,
		NamespacesLimit:  limits.Namespaces,
		TeamMembers:      memberCount,
		TeamMembersLimit: limits.TeamMembers,
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
	}, nil
}

// Compile-time check to ensure usageRepo implements UsageRepository.
var _ UsageRepository = (*usageRepo)(nil)

