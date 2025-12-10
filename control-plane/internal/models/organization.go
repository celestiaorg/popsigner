// Package models defines the data models for the Control Plane API.
package models

import (
	"time"

	"github.com/google/uuid"
)

// Plan represents a subscription plan.
type Plan string

const (
	PlanFree       Plan = "free"
	PlanPro        Plan = "pro"
	PlanEnterprise Plan = "enterprise"
)

// Organization represents a tenant in the system.
type Organization struct {
	ID                   uuid.UUID `json:"id" db:"id"`
	Name                 string    `json:"name" db:"name"`
	Slug                 string    `json:"slug" db:"slug"`
	Plan                 Plan      `json:"plan" db:"plan"`
	StripeCustomerID     *string   `json:"stripe_customer_id,omitempty" db:"stripe_customer_id"`
	StripeSubscriptionID *string   `json:"stripe_subscription_id,omitempty" db:"stripe_subscription_id"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time `json:"updated_at" db:"updated_at"`
}

// OrgMember represents a user's membership in an organization.
type OrgMember struct {
	OrgID     uuid.UUID  `json:"org_id" db:"org_id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	Role      Role       `json:"role" db:"role"`
	InvitedBy *uuid.UUID `json:"invited_by,omitempty" db:"invited_by"`
	JoinedAt  time.Time  `json:"joined_at" db:"joined_at"`
}

// Role represents a user's role within an organization.
type Role string

const (
	RoleOwner    Role = "owner"
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// Namespace represents an environment within an organization.
type Namespace struct {
	ID          uuid.UUID `json:"id" db:"id"`
	OrgID       uuid.UUID `json:"org_id" db:"org_id"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description,omitempty" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Invitation represents a pending invitation to join an organization.
type Invitation struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	OrgID      uuid.UUID  `json:"org_id" db:"org_id"`
	Email      string     `json:"email" db:"email"`
	Role       Role       `json:"role" db:"role"`
	Token      string     `json:"-" db:"token"`
	InvitedBy  uuid.UUID  `json:"invited_by" db:"invited_by"`
	ExpiresAt  time.Time  `json:"expires_at" db:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty" db:"accepted_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

