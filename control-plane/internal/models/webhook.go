package models

import (
	"time"

	"github.com/google/uuid"
)

// WebhookEvent represents events that can trigger webhooks.
type WebhookEvent string

const (
	WebhookEventKeyCreated         WebhookEvent = "key.created"
	WebhookEventKeyDeleted         WebhookEvent = "key.deleted"
	WebhookEventSignatureCompleted WebhookEvent = "signature.completed"
	WebhookEventQuotaWarning       WebhookEvent = "quota.warning"
	WebhookEventQuotaExceeded      WebhookEvent = "quota.exceeded"
	WebhookEventPaymentSucceeded   WebhookEvent = "payment.succeeded"
	WebhookEventPaymentFailed      WebhookEvent = "payment.failed"
)

// Webhook represents a webhook configuration.
type Webhook struct {
	ID              uuid.UUID      `json:"id" db:"id"`
	OrgID           uuid.UUID      `json:"org_id" db:"org_id"`
	URL             string         `json:"url" db:"url"`
	Secret          string         `json:"-" db:"secret"` // Hidden in responses
	Events          []WebhookEvent `json:"events" db:"events"`
	Enabled         bool           `json:"enabled" db:"enabled"`
	LastTriggeredAt *time.Time     `json:"last_triggered_at,omitempty" db:"last_triggered_at"`
	FailureCount    int            `json:"failure_count" db:"failure_count"`
	CreatedAt       time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at" db:"updated_at"`
}

// WebhookDelivery represents a webhook delivery attempt.
type WebhookDelivery struct {
	ID           uuid.UUID     `json:"id"`
	WebhookID    uuid.UUID     `json:"webhook_id"`
	Event        WebhookEvent  `json:"event"`
	Payload      string        `json:"payload"`
	StatusCode   int           `json:"status_code,omitempty"`
	ResponseBody string        `json:"response_body,omitempty"`
	Duration     time.Duration `json:"duration,omitempty"`
	Success      bool          `json:"success"`
	Error        string        `json:"error,omitempty"`
	AttemptedAt  time.Time     `json:"attempted_at"`
}

// WebhookPayload represents the payload sent to webhooks.
type WebhookPayload struct {
	ID        string       `json:"id"`
	Event     WebhookEvent `json:"event"`
	OrgID     string       `json:"org_id"`
	Timestamp time.Time    `json:"timestamp"`
	Data      interface{}  `json:"data"`
}
