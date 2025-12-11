# Implementation: Auth - Users & Sessions

## Agent: 08A - User & Session Models

> **Phase 5.1** - Can run in parallel with 08B, 08C after Agent 07 completes.

---

## 1. Overview

Implement user models, session management, and JWT token handling. BanhBaoRing uses **OAuth-only authentication** (no email/password) - users authenticate via GitHub or Google.

---

## 2. Scope

| Feature | Included |
|---------|----------|
| User model (OAuth-based) | ✅ |
| Session management | ✅ |
| JWT token generation | ✅ |
| Session middleware | ✅ |
| Email + Password | ❌ (Not supported) |
| OAuth flows | ❌ (Agent 08B) |
| API Keys | ❌ (Agent 08C) |

---

## 3. Models

**File:** `internal/models/user.go`

```go
package models

import (
    "time"

    "github.com/google/uuid"
)

// User represents an authenticated user.
// Users are created via OAuth only - no password field.
type User struct {
    ID            uuid.UUID  `json:"id" db:"id"`
    Email         string     `json:"email" db:"email"`
    Name          string     `json:"name" db:"name"`
    AvatarURL     string     `json:"avatar_url,omitempty" db:"avatar_url"`
    OAuthProvider string     `json:"-" db:"oauth_provider"` // "github" or "google"
    OAuthID       string     `json:"-" db:"oauth_provider_id"`
    LastLoginAt   *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
    CreatedAt     time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

type Session struct {
    ID        string    `json:"id" db:"id"`
    UserID    uuid.UUID `json:"user_id" db:"user_id"`
    Data      []byte    `json:"-" db:"data"`
    ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
}
```

---

## 4. Repository

**File:** `internal/repository/user_repo.go`

```go
package repository

import (
    "context"
    "database/sql"

    "github.com/google/uuid"
    "github.com/Bidon15/banhbaoring/control-plane/internal/models"
)

type UserRepository interface {
    Create(ctx context.Context, user *models.User) error
    GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
    GetByEmail(ctx context.Context, email string) (*models.User, error)
    GetByOAuth(ctx context.Context, provider, oauthID string) (*models.User, error)
    Update(ctx context.Context, user *models.User) error
    UpdateLastLogin(ctx context.Context, id uuid.UUID) error
}

type userRepo struct {
    db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
    return &userRepo{db: db}
}

func (r *userRepo) Create(ctx context.Context, user *models.User) error {
    user.ID = uuid.New()
    user.CreatedAt = time.Now()
    user.UpdatedAt = time.Now()

    _, err := r.db.ExecContext(ctx, `
        INSERT INTO users (id, email, name, avatar_url, oauth_provider, oauth_provider_id, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `, user.ID, user.Email, user.Name, user.AvatarURL, user.OAuthProvider, user.OAuthID, user.CreatedAt, user.UpdatedAt)
    return err
}

func (r *userRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
    var user models.User
    err := r.db.QueryRowContext(ctx, `
        SELECT id, email, name, avatar_url, oauth_provider, oauth_provider_id, last_login_at, created_at, updated_at
        FROM users WHERE id = $1
    `, id).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.OAuthProvider, &user.OAuthID, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    return &user, err
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
    var user models.User
    err := r.db.QueryRowContext(ctx, `
        SELECT id, email, name, avatar_url, oauth_provider, oauth_provider_id, last_login_at, created_at, updated_at
        FROM users WHERE email = $1
    `, email).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.OAuthProvider, &user.OAuthID, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    return &user, err
}

func (r *userRepo) GetByOAuth(ctx context.Context, provider, oauthID string) (*models.User, error) {
    var user models.User
    err := r.db.QueryRowContext(ctx, `
        SELECT id, email, name, avatar_url, oauth_provider, oauth_provider_id, last_login_at, created_at, updated_at
        FROM users WHERE oauth_provider = $1 AND oauth_provider_id = $2
    `, provider, oauthID).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.OAuthProvider, &user.OAuthID, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    return &user, err
}

func (r *userRepo) Update(ctx context.Context, user *models.User) error {
    user.UpdatedAt = time.Now()
    _, err := r.db.ExecContext(ctx, `
        UPDATE users SET email = $2, name = $3, avatar_url = $4, updated_at = $5
        WHERE id = $1
    `, user.ID, user.Email, user.Name, user.AvatarURL, user.UpdatedAt)
    return err
}

func (r *userRepo) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
    _, err := r.db.ExecContext(ctx, `
        UPDATE users SET last_login_at = NOW() WHERE id = $1
    `, id)
    return err
}
```

---

## 5. Session Service

**File:** `internal/service/session_service.go`

```go
package service

import (
    "context"
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "time"

    "github.com/go-redis/redis/v8"
    "github.com/google/uuid"
    "github.com/Bidon15/banhbaoring/control-plane/internal/models"
)

type SessionService interface {
    Create(ctx context.Context, userID uuid.UUID) (*models.Session, error)
    Get(ctx context.Context, sessionID string) (*models.Session, error)
    Delete(ctx context.Context, sessionID string) error
    DeleteAllForUser(ctx context.Context, userID uuid.UUID) error
}

type sessionService struct {
    redis  *redis.Client
    expiry time.Duration
}

func NewSessionService(redis *redis.Client, expiry time.Duration) SessionService {
    return &sessionService{
        redis:  redis,
        expiry: expiry,
    }
}

func (s *sessionService) Create(ctx context.Context, userID uuid.UUID) (*models.Session, error) {
    // Generate secure random session ID
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return nil, err
    }
    sessionID := base64.URLEncoding.EncodeToString(b)

    session := &models.Session{
        ID:        sessionID,
        UserID:    userID,
        ExpiresAt: time.Now().Add(s.expiry),
        CreatedAt: time.Now(),
    }

    data, err := json.Marshal(session)
    if err != nil {
        return nil, err
    }

    // Store in Redis
    if err := s.redis.Set(ctx, "session:"+sessionID, data, s.expiry).Err(); err != nil {
        return nil, err
    }

    // Track user sessions for logout-all
    s.redis.SAdd(ctx, "user_sessions:"+userID.String(), sessionID)

    return session, nil
}

func (s *sessionService) Get(ctx context.Context, sessionID string) (*models.Session, error) {
    data, err := s.redis.Get(ctx, "session:"+sessionID).Bytes()
    if err == redis.Nil {
        return nil, ErrSessionNotFound
    }
    if err != nil {
        return nil, err
    }

    var session models.Session
    if err := json.Unmarshal(data, &session); err != nil {
        return nil, err
    }

    return &session, nil
}

func (s *sessionService) Delete(ctx context.Context, sessionID string) error {
    return s.redis.Del(ctx, "session:"+sessionID).Err()
}

func (s *sessionService) DeleteAllForUser(ctx context.Context, userID uuid.UUID) error {
    sessionIDs, err := s.redis.SMembers(ctx, "user_sessions:"+userID.String()).Result()
    if err != nil {
        return err
    }

    for _, sid := range sessionIDs {
        s.redis.Del(ctx, "session:"+sid)
    }
    s.redis.Del(ctx, "user_sessions:"+userID.String())

    return nil
}

var ErrSessionNotFound = errors.New("session not found")
```

---

## 6. JWT Service

**File:** `internal/service/jwt_service.go`

```go
package service

import (
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
)

type JWTService interface {
    Generate(userID uuid.UUID) (string, error)
    Validate(tokenString string) (*Claims, error)
}

type Claims struct {
    UserID uuid.UUID `json:"user_id"`
    jwt.RegisteredClaims
}

type jwtService struct {
    secret []byte
    expiry time.Duration
}

func NewJWTService(secret string, expiry time.Duration) JWTService {
    return &jwtService{
        secret: []byte(secret),
        expiry: expiry,
    }
}

func (s *jwtService) Generate(userID uuid.UUID) (string, error) {
    claims := &Claims{
        UserID: userID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.expiry)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "banhbaoring",
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.secret)
}

func (s *jwtService) Validate(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return s.secret, nil
    })
    if err != nil {
        return nil, err
    }

    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }

    return nil, jwt.ErrTokenInvalidClaims
}
```

---

## 7. Session Middleware

**File:** `internal/middleware/session.go`

```go
package middleware

import (
    "context"
    "net/http"
    "strings"

    "github.com/Bidon15/banhbaoring/control-plane/internal/service"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func SessionMiddleware(jwtSvc service.JWTService, sessionSvc service.SessionService) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Try Authorization header first (API/JWT)
            authHeader := r.Header.Get("Authorization")
            if strings.HasPrefix(authHeader, "Bearer ") {
                tokenString := strings.TrimPrefix(authHeader, "Bearer ")
                
                claims, err := jwtSvc.Validate(tokenString)
                if err == nil {
                    ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
                    next.ServeHTTP(w, r.WithContext(ctx))
                    return
                }
            }

            // Try session cookie (Dashboard)
            cookie, err := r.Cookie("session_id")
            if err == nil {
                session, err := sessionSvc.Get(r.Context(), cookie.Value)
                if err == nil {
                    ctx := context.WithValue(r.Context(), UserIDKey, session.UserID)
                    next.ServeHTTP(w, r.WithContext(ctx))
                    return
                }
            }

            // No valid auth
            http.Error(w, "unauthorized", http.StatusUnauthorized)
        })
    }
}

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
    userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
    return userID, ok
}
```

---

## 8. Database Schema Update

Update the users table to remove password fields:

**File:** `internal/database/migrations/002_oauth_only.up.sql`

```sql
-- Remove password-related columns (OAuth-only auth)
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;

-- Add unique constraint on OAuth provider + ID
ALTER TABLE users ADD CONSTRAINT users_oauth_unique UNIQUE (oauth_provider, oauth_provider_id);
```

**File:** `internal/database/migrations/002_oauth_only.down.sql`

```sql
-- Restore password columns
ALTER TABLE users ADD COLUMN password_hash VARCHAR(255);
ALTER TABLE users ADD COLUMN email_verified BOOLEAN DEFAULT FALSE;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_oauth_unique;
```

---

## 9. Deliverables

| File | Description |
|------|-------------|
| `internal/models/user.go` | User and Session models |
| `internal/repository/user_repo.go` | User database operations |
| `internal/service/session_service.go` | Redis session management |
| `internal/service/jwt_service.go` | JWT generation/validation |
| `internal/middleware/session.go` | Auth middleware |
| `internal/database/migrations/002_*.sql` | OAuth-only schema |

---

## 10. Dependencies

**After:** Agent 07 (Control Plane Foundation)

**Before:** Agent 08B (OAuth Implementation)

---

## 11. Verification

```bash
# Run tests
go test ./internal/service/... -v -run "Session|JWT"
go test ./internal/middleware/... -v -run Session

# Run migrations
go run ./cmd/migrate up
```

---

## 12. Checklist

- [ ] User model (no password field)
- [ ] User repository with OAuth lookup
- [ ] Session service with Redis
- [ ] JWT service
- [ ] Session middleware
- [ ] Migration to remove password columns
- [ ] Unit tests
