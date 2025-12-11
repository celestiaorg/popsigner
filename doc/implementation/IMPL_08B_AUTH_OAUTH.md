# Implementation: Auth - OAuth Providers

## Agent: 08B - OAuth Authentication

> **Phase 5.1** - Can run in parallel with 08A, 08C after Agent 07 completes.

---

## 1. Overview

Implement OAuth 2.0 authentication with GitHub and Google. BanhBaoRing uses **OAuth-only** authentication - this is the primary login method for all users.

---

## 2. Scope

| Provider | Included |
|----------|----------|
| GitHub | ✅ |
| Google | ✅ |
| Discord | ❌ (Not planned) |
| Wallet Connect (SIWE) | ❌ (Not planned) |

---

## 3. OAuth Flow

```
1. User clicks "Sign in with GitHub"
2. Redirect to provider: GET /v1/auth/oauth/github
3. User authorizes on provider site
4. Provider redirects to callback: GET /v1/auth/oauth/github/callback?code=xxx
5. Exchange code for token
6. Fetch user info from provider
7. Create/update user in database
8. Create session
9. Redirect to dashboard with session cookie
```

---

## 4. Service

**File:** `internal/service/oauth_service.go`

```go
package service

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "strings"

    "github.com/google/uuid"
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/github"
    "golang.org/x/oauth2/google"

    "github.com/Bidon15/banhbaoring/control-plane/internal/config"
    "github.com/Bidon15/banhbaoring/control-plane/internal/models"
    "github.com/Bidon15/banhbaoring/control-plane/internal/repository"
)

type OAuthService interface {
    GetAuthURL(provider, state string) (string, error)
    HandleCallback(ctx context.Context, provider, code string) (*models.User, string, error)
}

type oauthService struct {
    configs     map[string]*oauth2.Config
    userRepo    repository.UserRepository
    sessionRepo repository.SessionRepository
}

func NewOAuthService(
    cfg *config.AuthConfig,
    userRepo repository.UserRepository,
    sessionRepo repository.SessionRepository,
    callbackBaseURL string,
) OAuthService {
    return &oauthService{
        configs: map[string]*oauth2.Config{
            "github": {
                ClientID:     cfg.OAuthGitHubID,
                ClientSecret: cfg.OAuthGitHubSecret,
                Endpoint:     github.Endpoint,
                RedirectURL:  callbackBaseURL + "/v1/auth/oauth/github/callback",
                Scopes:       []string{"user:email"},
            },
            "google": {
                ClientID:     cfg.OAuthGoogleID,
                ClientSecret: cfg.OAuthGoogleSecret,
                Endpoint:     google.Endpoint,
                RedirectURL:  callbackBaseURL + "/v1/auth/oauth/google/callback",
                Scopes:       []string{"email", "profile"},
            },
            "discord": {
                ClientID:     cfg.OAuthDiscordID,
                ClientSecret: cfg.OAuthDiscordSecret,
                Endpoint: oauth2.Endpoint{
                    AuthURL:  "https://discord.com/api/oauth2/authorize",
                    TokenURL: "https://discord.com/api/oauth2/token",
                },
                RedirectURL: callbackBaseURL + "/v1/auth/oauth/discord/callback",
                Scopes:      []string{"identify", "email"},
            },
        },
        userRepo:    userRepo,
        sessionRepo: sessionRepo,
    }
}

func (s *oauthService) GetAuthURL(provider, state string) (string, error) {
    cfg, ok := s.configs[provider]
    if !ok {
        return "", fmt.Errorf("unknown provider: %s", provider)
    }
    return cfg.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

func (s *oauthService) HandleCallback(ctx context.Context, provider, code string) (*models.User, string, error) {
    cfg, ok := s.configs[provider]
    if !ok {
        return nil, "", fmt.Errorf("unknown provider: %s", provider)
    }

    // Exchange code for token
    token, err := cfg.Exchange(ctx, code)
    if err != nil {
        return nil, "", fmt.Errorf("token exchange failed: %w", err)
    }

    // Fetch user info
    userInfo, err := s.fetchUserInfo(ctx, provider, token)
    if err != nil {
        return nil, "", err
    }

    // Find or create user
    user, err := s.findOrCreateUser(ctx, provider, userInfo)
    if err != nil {
        return nil, "", err
    }

    // Create session
    sessionID, err := s.createSession(ctx, user.ID)
    if err != nil {
        return nil, "", err
    }

    return user, sessionID, nil
}

type OAuthUserInfo struct {
    ID        string
    Email     string
    Name      string
    AvatarURL string
}

func (s *oauthService) fetchUserInfo(ctx context.Context, provider string, token *oauth2.Token) (*OAuthUserInfo, error) {
    client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

    switch provider {
    case "github":
        return s.fetchGitHubUser(client)
    case "google":
        return s.fetchGoogleUser(client)
    case "discord":
        return s.fetchDiscordUser(client)
    default:
        return nil, fmt.Errorf("unknown provider: %s", provider)
    }
}

func (s *oauthService) fetchGitHubUser(client *http.Client) (*OAuthUserInfo, error) {
    resp, err := client.Get("https://api.github.com/user")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var data struct {
        ID        int64  `json:"id"`
        Login     string `json:"login"`
        Email     string `json:"email"`
        Name      string `json:"name"`
        AvatarURL string `json:"avatar_url"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return nil, err
    }

    // Fetch email if not public
    if data.Email == "" {
        emails, err := s.fetchGitHubEmails(client)
        if err == nil && len(emails) > 0 {
            data.Email = emails[0]
        }
    }

    name := data.Name
    if name == "" {
        name = data.Login
    }

    return &OAuthUserInfo{
        ID:        fmt.Sprintf("%d", data.ID),
        Email:     data.Email,
        Name:      name,
        AvatarURL: data.AvatarURL,
    }, nil
}

func (s *oauthService) fetchGitHubEmails(client *http.Client) ([]string, error) {
    resp, err := client.Get("https://api.github.com/user/emails")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var emails []struct {
        Email    string `json:"email"`
        Primary  bool   `json:"primary"`
        Verified bool   `json:"verified"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
        return nil, err
    }

    var result []string
    for _, e := range emails {
        if e.Verified && e.Primary {
            result = append([]string{e.Email}, result...)
        } else if e.Verified {
            result = append(result, e.Email)
        }
    }
    return result, nil
}

func (s *oauthService) fetchGoogleUser(client *http.Client) (*OAuthUserInfo, error) {
    resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var data struct {
        ID      string `json:"id"`
        Email   string `json:"email"`
        Name    string `json:"name"`
        Picture string `json:"picture"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return nil, err
    }

    return &OAuthUserInfo{
        ID:        data.ID,
        Email:     data.Email,
        Name:      data.Name,
        AvatarURL: data.Picture,
    }, nil
}

func (s *oauthService) fetchDiscordUser(client *http.Client) (*OAuthUserInfo, error) {
    resp, err := client.Get("https://discord.com/api/users/@me")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var data struct {
        ID       string `json:"id"`
        Username string `json:"username"`
        Email    string `json:"email"`
        Avatar   string `json:"avatar"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return nil, err
    }

    avatarURL := ""
    if data.Avatar != "" {
        avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", data.ID, data.Avatar)
    }

    return &OAuthUserInfo{
        ID:        data.ID,
        Email:     data.Email,
        Name:      data.Username,
        AvatarURL: avatarURL,
    }, nil
}

func (s *oauthService) findOrCreateUser(ctx context.Context, provider string, info *OAuthUserInfo) (*models.User, error) {
    // Try to find by OAuth ID
    user, err := s.userRepo.GetByOAuth(ctx, provider, info.ID)
    if err != nil {
        return nil, err
    }
    if user != nil {
        // Update user info
        user.Name = info.Name
        user.AvatarURL = info.AvatarURL
        _ = s.userRepo.Update(ctx, user)
        return user, nil
    }

    // Try to find by email (link accounts)
    if info.Email != "" {
        user, err = s.userRepo.GetByEmail(ctx, info.Email)
        if err != nil {
            return nil, err
        }
        if user != nil {
            // Link OAuth to existing account
            user.OAuthProvider = provider
            user.OAuthID = info.ID
            _ = s.userRepo.UpdateOAuth(ctx, user.ID, provider, info.ID)
            return user, nil
        }
    }

    // Create new user
    user = &models.User{
        Email:         info.Email,
        Name:          info.Name,
        AvatarURL:     info.AvatarURL,
        EmailVerified: true, // OAuth emails are pre-verified
        OAuthProvider: provider,
        OAuthID:       info.ID,
    }

    if err := s.userRepo.Create(ctx, user); err != nil {
        return nil, err
    }

    return user, nil
}

func (s *oauthService) createSession(ctx context.Context, userID uuid.UUID) (string, error) {
    // Reuse session creation logic from auth_service
    panic("TODO(08B): implement session creation")
}
```

---

## 5. Handler

**File:** `internal/handler/oauth_handler.go`

```go
package handler

import (
    "crypto/rand"
    "encoding/base64"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"

    "github.com/Bidon15/banhbaoring/control-plane/internal/service"
    "github.com/Bidon15/banhbaoring/control-plane/internal/pkg/response"
    apierrors "github.com/Bidon15/banhbaoring/control-plane/internal/pkg/errors"
)

type OAuthHandler struct {
    oauthService service.OAuthService
    dashboardURL string
}

func NewOAuthHandler(oauthService service.OAuthService, dashboardURL string) *OAuthHandler {
    return &OAuthHandler{
        oauthService: oauthService,
        dashboardURL: dashboardURL,
    }
}

func (h *OAuthHandler) Routes() chi.Router {
    r := chi.NewRouter()

    r.Get("/{provider}", h.Authorize)
    r.Get("/{provider}/callback", h.Callback)

    return r
}

func (h *OAuthHandler) Authorize(w http.ResponseWriter, r *http.Request) {
    provider := chi.URLParam(r, "provider")

    // Generate state for CSRF protection
    state := generateState()
    http.SetCookie(w, &http.Cookie{
        Name:     "oauth_state",
        Value:    state,
        Path:     "/",
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteLaxMode,
        MaxAge:   300, // 5 minutes
    })

    authURL, err := h.oauthService.GetAuthURL(provider, state)
    if err != nil {
        response.Error(w, apierrors.ErrBadRequest)
        return
    }

    http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
    provider := chi.URLParam(r, "provider")
    code := r.URL.Query().Get("code")
    state := r.URL.Query().Get("state")

    // Verify state
    cookie, err := r.Cookie("oauth_state")
    if err != nil || cookie.Value != state {
        http.Redirect(w, r, h.dashboardURL+"/login?error=invalid_state", http.StatusTemporaryRedirect)
        return
    }

    // Clear state cookie
    http.SetCookie(w, &http.Cookie{
        Name:   "oauth_state",
        Value:  "",
        Path:   "/",
        MaxAge: -1,
    })

    // Handle OAuth callback
    user, sessionID, err := h.oauthService.HandleCallback(r.Context(), provider, code)
    if err != nil {
        http.Redirect(w, r, h.dashboardURL+"/login?error=oauth_failed", http.StatusTemporaryRedirect)
        return
    }

    // Set session cookie
    http.SetCookie(w, &http.Cookie{
        Name:     "session",
        Value:    sessionID,
        Path:     "/",
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteLaxMode,
        MaxAge:   int(7 * 24 * time.Hour / time.Second),
    })

    // Redirect to dashboard
    http.Redirect(w, r, h.dashboardURL+"/dashboard", http.StatusTemporaryRedirect)
}

func generateState() string {
    b := make([]byte, 16)
    rand.Read(b)
    return base64.URLEncoding.EncodeToString(b)
}
```

---

## 6. Deliverables

| File | Description |
|------|-------------|
| `internal/service/oauth_service.go` | OAuth business logic |
| `internal/handler/oauth_handler.go` | HTTP handlers |
| `internal/repository/user_repo.go` | Add OAuth methods |
| `internal/handler/oauth_handler_test.go` | Tests |

---

## 7. API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/auth/oauth/{provider}` | Start OAuth flow |
| GET | `/v1/auth/oauth/{provider}/callback` | OAuth callback |

Providers: `github`, `google`, `discord`

---

## 8. Success Criteria

- [ ] GitHub OAuth works end-to-end
- [ ] Google OAuth works end-to-end
- [ ] Discord OAuth works end-to-end
- [ ] State validation prevents CSRF
- [ ] Account linking works (same email)
- [ ] Tests pass

---

## 9. Agent Prompt

```
You are Agent 08B - OAuth Authentication. Implement OAuth 2.0 with GitHub, Google, and Discord.

Read the spec: doc/implementation/IMPL_08B_AUTH_OAUTH.md

Deliverables:
1. OAuth service with provider configs
2. User info fetching for each provider
3. Account linking by email
4. OAuth HTTP handlers
5. CSRF protection with state parameter
6. Tests

Dependencies: Agent 07 (Foundation) must complete first.

Test: go test ./internal/... -v
```

