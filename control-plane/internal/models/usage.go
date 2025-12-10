package models

import (
	"time"

	"github.com/google/uuid"
)

// MetricType represents types of usage metrics.
type MetricType string

const (
	MetricTypeSignatures MetricType = "signatures"
	MetricTypeAPICalls   MetricType = "api_calls"
	MetricTypeKeys       MetricType = "keys"
)

// UsageMetric represents a usage metric for billing.
type UsageMetric struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	OrgID       uuid.UUID  `json:"org_id" db:"org_id"`
	Metric      MetricType `json:"metric" db:"metric"`
	Value       int64      `json:"value" db:"value"`
	PeriodStart time.Time  `json:"period_start" db:"period_start"`
	PeriodEnd   time.Time  `json:"period_end" db:"period_end"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// UsageSummary represents a summary of usage for an organization.
type UsageSummary struct {
	OrgID            uuid.UUID `json:"org_id"`
	Plan             Plan      `json:"plan"`
	Keys             int       `json:"keys"`
	KeysLimit        int       `json:"keys_limit"`
	SignaturesMonth  int64     `json:"signatures_month"`
	SignaturesLimit  int64     `json:"signatures_limit"`
	Namespaces       int       `json:"namespaces"`
	NamespacesLimit  int       `json:"namespaces_limit"`
	TeamMembers      int       `json:"team_members"`
	TeamMembersLimit int       `json:"team_members_limit"`
	PeriodStart      time.Time `json:"period_start"`
	PeriodEnd        time.Time `json:"period_end"`
}
