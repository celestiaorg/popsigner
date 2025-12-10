package models

import (
	"encoding/json"
	"net"
	"time"

	"github.com/google/uuid"
)

// ActorType represents the type of entity performing an action.
type ActorType string

const (
	ActorTypeUser   ActorType = "user"
	ActorTypeAPIKey ActorType = "api_key"
	ActorTypeSystem ActorType = "system"
)

// AuditEvent represents the type of audit event.
type AuditEvent string

const (
	// Key events
	AuditEventKeyCreated  AuditEvent = "key.created"
	AuditEventKeyDeleted  AuditEvent = "key.deleted"
	AuditEventKeySigned   AuditEvent = "key.signed"
	AuditEventKeyExported AuditEvent = "key.exported"
	AuditEventKeyRotated  AuditEvent = "key.rotated"

	// Auth events
	AuditEventAuthLogin      AuditEvent = "auth.login"
	AuditEventAuthLogout     AuditEvent = "auth.logout"
	AuditEventAuthAPIKeyUsed AuditEvent = "auth.api_key_used"

	// Org events
	AuditEventOrgCreated AuditEvent = "org.created"
	AuditEventOrgUpdated AuditEvent = "org.updated"
	AuditEventOrgDeleted AuditEvent = "org.deleted"

	// Member events
	AuditEventMemberInvited AuditEvent = "member.invited"
	AuditEventMemberJoined  AuditEvent = "member.joined"
	AuditEventMemberRemoved AuditEvent = "member.removed"
	AuditEventMemberUpdated AuditEvent = "member.updated"

	// Billing events
	AuditEventBillingCharge          AuditEvent = "billing.charge"
	AuditEventBillingSubscriptionNew AuditEvent = "billing.subscription.new"
	AuditEventBillingSubscriptionUpd AuditEvent = "billing.subscription.updated"

	// Webhook events
	AuditEventWebhookCreated  AuditEvent = "webhook.created"
	AuditEventWebhookDeleted  AuditEvent = "webhook.deleted"
	AuditEventWebhookTriggered AuditEvent = "webhook.triggered"
)

// ResourceType represents the type of resource being acted upon.
type ResourceType string

const (
	ResourceTypeKey       ResourceType = "key"
	ResourceTypeUser      ResourceType = "user"
	ResourceTypeOrg       ResourceType = "organization"
	ResourceTypeAPIKey    ResourceType = "api_key"
	ResourceTypeNamespace ResourceType = "namespace"
	ResourceTypeWebhook   ResourceType = "webhook"
)

// AuditLog represents an audit log entry.
type AuditLog struct {
	ID           uuid.UUID       `json:"id" db:"id"`
	OrgID        uuid.UUID       `json:"org_id" db:"org_id"`
	Event        AuditEvent      `json:"event" db:"event"`
	ActorID      *uuid.UUID      `json:"actor_id,omitempty" db:"actor_id"`
	ActorType    ActorType       `json:"actor_type" db:"actor_type"`
	ResourceType *ResourceType   `json:"resource_type,omitempty" db:"resource_type"`
	ResourceID   *uuid.UUID      `json:"resource_id,omitempty" db:"resource_id"`
	IPAddress    *net.IP         `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent    *string         `json:"user_agent,omitempty" db:"user_agent"`
	Metadata     json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
}

// AuditLogQuery represents query parameters for fetching audit logs.
type AuditLogQuery struct {
	OrgID        uuid.UUID
	Event        *AuditEvent
	ActorID      *uuid.UUID
	ResourceType *ResourceType
	ResourceID   *uuid.UUID
	StartTime    *time.Time
	EndTime      *time.Time
	Limit        int
	Cursor       string
}

