// Package service provides business logic implementations.
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"

	"github.com/Bidon15/popsigner/control-plane/internal/config"
	"github.com/Bidon15/popsigner/control-plane/internal/models"
	"github.com/Bidon15/popsigner/control-plane/internal/repository"
)

// OAuthUserInfo contains user information fetched from OAuth providers.
type OAuthUserInfo struct {
	ID        string
	Email     string
	Name      string
	AvatarURL string
}

// OAuthService defines the OAuth authentication interface.
type OAuthService interface {
	// GetAuthURL returns the OAuth authorization URL for the given provider.
	GetAuthURL(provider, state string) (string, error)

	// HandleCallback processes the OAuth callback and returns the user and session ID.
	HandleCallback(ctx context.Context, provider, code string) (*models.User, string, error)

	// GetSupportedProviders returns a list of configured OAuth providers.
	GetSupportedProviders() []string
}

type oauthService struct {
	configs        map[string]*oauth2.Config
	userRepo       repository.UserRepository
	sessionRepo    repository.SessionRepository
	sessionExpiry  time.Duration
	httpClient     HTTPClient
}

// HTTPClient interface for making HTTP requests (allows mocking in tests).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// NewOAuthService creates a new OAuth service with the given configuration.
func NewOAuthService(
	cfg *config.AuthConfig,
	userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
) OAuthService {
	callbackBaseURL := cfg.OAuthCallbackURL
	configs := make(map[string]*oauth2.Config)

	// Configure GitHub OAuth if credentials are provided
	if cfg.OAuthGitHubID != "" && cfg.OAuthGitHubSecret != "" {
		configs["github"] = &oauth2.Config{
			ClientID:     cfg.OAuthGitHubID,
			ClientSecret: cfg.OAuthGitHubSecret,
			Endpoint:     github.Endpoint,
			RedirectURL:  callbackBaseURL + "/auth/github/callback",
			Scopes:       []string{"user:email"},
		}
	}

	// Configure Google OAuth if credentials are provided
	if cfg.OAuthGoogleID != "" && cfg.OAuthGoogleSecret != "" {
		configs["google"] = &oauth2.Config{
			ClientID:     cfg.OAuthGoogleID,
			ClientSecret: cfg.OAuthGoogleSecret,
			Endpoint:     google.Endpoint,
			RedirectURL:  callbackBaseURL + "/auth/google/callback",
			Scopes:       []string{"email", "profile"},
		}
	}

	// Note: BanhBaoRing supports GitHub and Google OAuth only

	return &oauthService{
		configs:       configs,
		userRepo:      userRepo,
		sessionRepo:   sessionRepo,
		sessionExpiry: cfg.SessionExpiry,
		httpClient:    http.DefaultClient,
	}
}

// NewOAuthServiceWithClient creates a new OAuth service with a custom HTTP client.
// This is primarily used for testing.
func NewOAuthServiceWithClient(
	cfg *config.AuthConfig,
	userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
	httpClient HTTPClient,
) OAuthService {
	svc := NewOAuthService(cfg, userRepo, sessionRepo).(*oauthService)
	svc.httpClient = httpClient
	return svc
}

func (s *oauthService) GetAuthURL(provider, state string) (string, error) {
	cfg, ok := s.configs[provider]
	if !ok {
		return "", fmt.Errorf("unknown or unconfigured provider: %s", provider)
	}
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

func (s *oauthService) HandleCallback(ctx context.Context, provider, code string) (*models.User, string, error) {
	cfg, ok := s.configs[provider]
	if !ok {
		return nil, "", fmt.Errorf("unknown or unconfigured provider: %s", provider)
	}

	// Exchange authorization code for access token
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, "", fmt.Errorf("token exchange failed: %w", err)
	}

	// Fetch user info from the provider
	userInfo, err := s.fetchUserInfo(ctx, provider, token)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch user info: %w", err)
	}

	// Find or create user
	user, err := s.findOrCreateUser(ctx, provider, userInfo)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find or create user: %w", err)
	}

	// Create session
	sessionID, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	// Update last login time
	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	return user, sessionID, nil
}

func (s *oauthService) GetSupportedProviders() []string {
	providers := make([]string, 0, len(s.configs))
	for provider := range s.configs {
		providers = append(providers, provider)
	}
	return providers
}

func (s *oauthService) fetchUserInfo(ctx context.Context, provider string, token *oauth2.Token) (*OAuthUserInfo, error) {
	// Create HTTP client with the OAuth token
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	switch provider {
	case "github":
		return s.fetchGitHubUser(client)
	case "google":
		return s.fetchGoogleUser(client)
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

func (s *oauthService) fetchGitHubUser(client *http.Client) (*OAuthUserInfo, error) {
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GitHub user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var data struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub user response: %w", err)
	}

	// Fetch email if not public
	email := data.Email
	if email == "" {
		emails, err := s.fetchGitHubEmails(client)
		if err == nil && len(emails) > 0 {
			email = emails[0]
		}
	}

	name := data.Name
	if name == "" {
		name = data.Login
	}

	return &OAuthUserInfo{
		ID:        fmt.Sprintf("%d", data.ID),
		Email:     email,
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub emails API returned status %d", resp.StatusCode)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return nil, err
	}

	// Prioritize primary verified emails
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
		return nil, fmt.Errorf("failed to fetch Google user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google API returned status %d", resp.StatusCode)
	}

	var data struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode Google user response: %w", err)
	}

	return &OAuthUserInfo{
		ID:        data.ID,
		Email:     data.Email,
		Name:      data.Name,
		AvatarURL: data.Picture,
	}, nil
}

func (s *oauthService) findOrCreateUser(ctx context.Context, provider string, info *OAuthUserInfo) (*models.User, error) {
	// First, try to find user by OAuth provider ID
	user, err := s.userRepo.GetByOAuth(ctx, provider, info.ID)
	if err != nil {
		return nil, err
	}
	if user != nil {
		// Update user info (name, avatar may have changed)
		user.Name = &info.Name
		user.AvatarURL = &info.AvatarURL
		_ = s.userRepo.Update(ctx, user)
		return user, nil
	}

	// Try to find user by email (account linking)
	if info.Email != "" {
		user, err = s.userRepo.GetByEmail(ctx, info.Email)
		if err != nil {
			return nil, err
		}
		if user != nil {
			// Link OAuth provider to existing account
			if err := s.userRepo.UpdateOAuth(ctx, user.ID, provider, info.ID); err != nil {
				return nil, err
			}
			user.OAuthProvider = &provider
			user.OAuthProviderID = &info.ID
			return user, nil
		}
	}

	// Create new user
	user = &models.User{
		Email:           info.Email,
		Name:            &info.Name,
		AvatarURL:       &info.AvatarURL,
		EmailVerified:   true, // OAuth emails are pre-verified by the provider
		OAuthProvider:   &provider,
		OAuthProviderID: &info.ID,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *oauthService) createSession(ctx context.Context, userID uuid.UUID) (string, error) {
	// Generate a cryptographically secure random session ID
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate session ID: %w", err)
	}
	sessionID := base64.URLEncoding.EncodeToString(b)

	// Determine session expiry
	expiry := s.sessionExpiry
	if expiry == 0 {
		expiry = 7 * 24 * time.Hour // Default to 7 days
	}

	session := &models.Session{
		ID:        sessionID,
		UserID:    userID,
		Data:      make(map[string]interface{}),
		ExpiresAt: time.Now().Add(expiry),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return "", err
	}

	return sessionID, nil
}

// Compile-time check to ensure oauthService implements OAuthService.
var _ OAuthService = (*oauthService)(nil)

