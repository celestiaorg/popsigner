package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user account.
type User struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	Email           string     `json:"email" db:"email"`
	PasswordHash    *string    `json:"-" db:"password_hash"`
	Name            *string    `json:"name,omitempty" db:"name"`
	AvatarURL       *string    `json:"avatar_url,omitempty" db:"avatar_url"`
	EmailVerified   bool       `json:"email_verified" db:"email_verified"`
	OAuthProvider   *string    `json:"oauth_provider,omitempty" db:"oauth_provider"`
	OAuthProviderID *string    `json:"-" db:"oauth_provider_id"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

// Session represents an authenticated user session.
type Session struct {
	ID        string                 `json:"id" db:"id"`
	UserID    uuid.UUID              `json:"user_id" db:"user_id"`
	Data      map[string]interface{} `json:"data" db:"data"`
	ExpiresAt time.Time              `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
}

