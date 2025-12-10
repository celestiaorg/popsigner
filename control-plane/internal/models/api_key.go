package models

import (
	"time"

	"github.com/google/uuid"
)

// APIKey represents an API key for programmatic access.
type APIKey struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	OrgID      uuid.UUID  `json:"org_id" db:"org_id"`
	UserID     *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	Name       string     `json:"name" db:"name"`
	KeyPrefix  string     `json:"key_prefix" db:"key_prefix"` // bbr_xxxx (for display)
	KeyHash    string     `json:"-" db:"key_hash"`            // Argon2 hash
	Scopes     []string   `json:"scopes" db:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

// APIKeyScopes defines the available API key scopes.
var APIKeyScopes = []string{
	"api:keys:read",
	"api:keys:write",
	"api:keys:sign",
	"api:audit:read",
	"api:billing:read",
	"api:billing:write",
	"api:webhooks:read",
	"api:webhooks:write",
}

// IsValidScope checks if a scope is valid.
func IsValidScope(scope string) bool {
	for _, s := range APIKeyScopes {
		if s == scope {
			return true
		}
	}
	return false
}

// ValidateScopes checks if all scopes in a list are valid.
func ValidateScopes(scopes []string) bool {
	for _, s := range scopes {
		if !IsValidScope(s) {
			return false
		}
	}
	return true
}

// APIKeyResponse is the response format for API key operations.
// It includes the full key only on creation.
type APIKeyResponse struct {
	ID        uuid.UUID  `json:"id"`
	OrgID     uuid.UUID  `json:"org_id"`
	Name      string     `json:"name"`
	KeyPrefix string     `json:"key_prefix"`
	Key       string     `json:"key,omitempty"` // Only set on creation
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

