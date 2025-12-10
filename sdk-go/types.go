// Package banhbaoring provides the official Go SDK for the BanhBaoRing Control Plane API.
//
// BanhBaoRing is a secure key management and signing service backed by OpenBao,
// designed for blockchain applications like Celestia sequencers.
package banhbaoring

import (
	"time"

	"github.com/google/uuid"
)

// Algorithm represents a cryptographic algorithm.
type Algorithm string

const (
	// AlgorithmSecp256k1 is the secp256k1 elliptic curve algorithm.
	AlgorithmSecp256k1 Algorithm = "secp256k1"
	// AlgorithmEd25519 is the Ed25519 signature algorithm.
	AlgorithmEd25519 Algorithm = "ed25519"
)

// Key represents a cryptographic key.
type Key struct {
	ID          uuid.UUID         `json:"id"`
	NamespaceID uuid.UUID         `json:"namespace_id"`
	Name        string            `json:"name"`
	PublicKey   string            `json:"public_key"` // Hex encoded
	Address     string            `json:"address"`
	Algorithm   Algorithm         `json:"algorithm"`
	Exportable  bool              `json:"exportable"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Version     int               `json:"version"`
	CreatedAt   time.Time         `json:"created_at"`
}

// Organization represents an organization.
type Organization struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Namespace represents a key namespace within an organization.
type Namespace struct {
	ID          uuid.UUID `json:"id"`
	OrgID       uuid.UUID `json:"org_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Member represents an organization member.
type Member struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Role represents a user's role within an organization.
type Role string

const (
	RoleOwner    Role = "owner"
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// Invitation represents a pending organization invitation.
type Invitation struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"org_id"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// AuditEvent represents an audit event type.
type AuditEvent string

const (
	AuditEventKeyCreated  AuditEvent = "key.created"
	AuditEventKeyDeleted  AuditEvent = "key.deleted"
	AuditEventKeySigned   AuditEvent = "key.signed"
	AuditEventKeyExported AuditEvent = "key.exported"
	AuditEventKeyImported AuditEvent = "key.imported"
)

// ResourceType represents the type of resource in an audit log.
type ResourceType string

const (
	ResourceTypeKey       ResourceType = "key"
	ResourceTypeNamespace ResourceType = "namespace"
	ResourceTypeOrg       ResourceType = "organization"
	ResourceTypeUser      ResourceType = "user"
)

// ActorType represents who performed an action.
type ActorType string

const (
	ActorTypeUser   ActorType = "user"
	ActorTypeAPIKey ActorType = "api_key"
	ActorTypeSystem ActorType = "system"
)

// AuditLog represents an audit log entry.
type AuditLog struct {
	ID           uuid.UUID              `json:"id"`
	OrgID        uuid.UUID              `json:"org_id"`
	Event        AuditEvent             `json:"event"`
	ActorID      *uuid.UUID             `json:"actor_id,omitempty"`
	ActorType    ActorType              `json:"actor_type"`
	ResourceType *ResourceType          `json:"resource_type,omitempty"`
	ResourceID   *uuid.UUID             `json:"resource_id,omitempty"`
	IPAddress    *string                `json:"ip_address,omitempty"`
	UserAgent    *string                `json:"user_agent,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

// PlanLimits represents the limits for an organization's plan.
type PlanLimits struct {
	MaxKeys             int `json:"max_keys"`
	MaxNamespaces       int `json:"max_namespaces"`
	MaxMembers          int `json:"max_members"`
	MaxSignsPerMonth    int `json:"max_signs_per_month"`
	CurrentKeys         int `json:"current_keys"`
	CurrentNamespaces   int `json:"current_namespaces"`
	CurrentMembers      int `json:"current_members"`
	CurrentSignsThisMonth int `json:"current_signs_this_month"`
}

