// Package web provides HTTP handlers for the web dashboard.
package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/Bidon15/banhbaoring/control-plane/internal/models"
	"github.com/Bidon15/banhbaoring/control-plane/internal/service"
	"github.com/Bidon15/banhbaoring/control-plane/templates/pages"
)

// Context keys for request context values.
type contextKey string

const (
	// ContextKeyUserID is the context key for the authenticated user ID.
	ContextKeyUserID contextKey = "user_id"
	// ContextKeyUser is the context key for the authenticated user.
	ContextKeyUser contextKey = "user"
	// ContextKeySessionID is the context key for the session ID.
	ContextKeySessionID contextKey = "session_id"
)

// Session cookie names.
const (
	SessionCookieName = "banhbao_session"
	OAuthStateCookie  = "banhbao_oauth_state"
)

// WebHandler handles HTTP requests for the web dashboard.
type WebHandler struct {
	authService    service.AuthService
	oauthService   service.OAuthService
	keyService     service.KeyService
	orgService     service.OrgService
	billingService service.BillingService
	auditService   service.AuditService
	apiKeyService  service.APIKeyService
	sessionStore   sessions.Store
}

// Config holds configuration for the web handler.
type Config struct {
	SessionName   string
	SessionSecret string
}

// NewWebHandler creates a new WebHandler instance.
func NewWebHandler(
	authService service.AuthService,
	keyService service.KeyService,
	orgService service.OrgService,
	billingService service.BillingService,
	auditService service.AuditService,
	apiKeyService service.APIKeyService,
	sessionStore sessions.Store,
) *WebHandler {
	return &WebHandler{
		authService:    authService,
		keyService:     keyService,
		orgService:     orgService,
		billingService: billingService,
		auditService:   auditService,
		apiKeyService:  apiKeyService,
		sessionStore:   sessionStore,
	}
}

// SetOAuthService sets the OAuth service (called after construction if needed).
func (h *WebHandler) SetOAuthService(oauthService service.OAuthService) {
	h.oauthService = oauthService
}

// Routes returns the chi router with all web routes configured.
func (h *WebHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Static files
	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// Public routes (no auth required)
	r.Group(func(r chi.Router) {
		// Landing page
		r.Get("/", h.LandingPage)

		// Auth pages
		r.Get("/login", h.LoginPage)
		r.Post("/login", h.Login)
		r.Get("/signup", h.SignupPage)
		r.Post("/signup", h.Signup)
		r.Get("/forgot-password", h.ForgotPasswordPage)
		r.Post("/forgot-password", h.ForgotPassword)
		r.Get("/reset-password", h.ResetPasswordPage)
		r.Post("/reset-password", h.ResetPassword)

		// OAuth routes
		r.Get("/auth/{provider}", h.OAuthStart)
		r.Get("/auth/{provider}/callback", h.OAuthCallback)

		// Health check
		r.Get("/health", h.Health)
	})

	// Protected routes (auth required)
	r.Group(func(r chi.Router) {
		r.Use(h.RequireAuth)

		// Logout
		r.Post("/logout", h.Logout)

		// Onboarding flow
		r.Get("/onboarding", h.OnboardingPage)
		r.Post("/onboarding/step1", h.OnboardingStep1)
		r.Get("/onboarding/step2", h.OnboardingStep2Page)
		r.Post("/onboarding/step2", h.OnboardingStep2)
		r.Get("/onboarding/step3", h.OnboardingStep3Page)

		// Dashboard
		r.Get("/dashboard", h.Dashboard)

		// Keys management
		r.Route("/keys", func(r chi.Router) {
			r.Get("/", h.KeysList)
			r.Get("/new", h.KeysNew)
			r.Post("/", h.KeysCreate)
			r.Get("/workers/new", h.WorkerKeysNew)
			r.Post("/workers", h.WorkerKeysCreate)
			r.Get("/{id}", h.KeysDetail)
			r.Post("/{id}/sign-test", h.KeysSignTest)
			r.Delete("/{id}", h.KeysDelete)
		})

		// Usage & Analytics
		r.Get("/usage", h.Usage)

		// Audit log
		r.Get("/audit", h.AuditLog)

		// Settings
		r.Route("/settings", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/settings/profile", http.StatusFound)
			})
			r.Get("/profile", h.SettingsProfile)
			r.Post("/profile", h.SettingsProfileUpdate)
			r.Get("/team", h.SettingsTeam)
			r.Get("/team/invite", h.SettingsTeamInviteModal)
			r.Post("/team/invite", h.SettingsTeamInvite)
			r.Get("/team/{id}/edit", h.SettingsTeamEditModal)
			r.Patch("/team/{id}", h.SettingsTeamUpdate)
			r.Delete("/team/{id}", h.SettingsTeamRemove)
			r.Get("/api-keys", h.SettingsAPIKeys)
			r.Get("/api-keys/new", h.SettingsAPIKeysNewModal)
			r.Post("/api-keys", h.SettingsAPIKeysCreate)
			r.Delete("/api-keys/{id}", h.SettingsAPIKeysDelete)
			r.Get("/billing", h.SettingsBilling)
			r.Get("/billing/upgrade", h.SettingsBillingUpgrade)
			r.Get("/billing/card", h.SettingsBillingCard)
			r.Post("/billing/card/confirm", h.SettingsBillingCardConfirm)
			r.Post("/billing/checkout", h.SettingsBillingCheckout)
			r.Post("/billing/portal", h.SettingsBillingPortal)
		})

		// Audit log detail
		r.Get("/audit/{id}", h.AuditLogDetail)
		r.Get("/audit/export", h.AuditLogExport)
	})

	return r
}

// ============================================
// Middleware
// ============================================

// RequireAuth middleware ensures the user is authenticated and loads user context.
func (h *WebHandler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := h.sessionStore.Get(r, SessionCookieName)
		if err != nil {
			h.handleAuthRedirect(w, r)
			return
		}

		userIDStr, ok := session.Values["user_id"].(string)
		if !ok || userIDStr == "" {
			h.handleAuthRedirect(w, r)
			return
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			h.handleAuthRedirect(w, r)
			return
		}

		// Load user from database
		user, err := h.authService.GetUserByID(r.Context(), userID)
		if err != nil {
			// Session exists but user not found, clear session
			session.Options.MaxAge = -1
			session.Save(r, w)
			h.handleAuthRedirect(w, r)
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), ContextKeyUserID, userIDStr)
		ctx = context.WithValue(ctx, ContextKeyUser, user)
		if sessionID, ok := session.Values["session_id"].(string); ok {
			ctx = context.WithValue(ctx, ContextKeySessionID, sessionID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *WebHandler) handleAuthRedirect(w http.ResponseWriter, r *http.Request) {
			// Check if this is an HTMX request
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
}

// ============================================
// Public Page Handlers
// ============================================

// LandingPage renders the public landing page.
func (h *WebHandler) LandingPage(w http.ResponseWriter, r *http.Request) {
	// Check if user is already logged in
	session, _ := h.sessionStore.Get(r, SessionCookieName)
	if session.Values["user_id"] != nil {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}

	// TODO: Render landing page template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>BanhBaoRing - Secure Key Management</title>
	<link rel="stylesheet" href="/static/css/output.css">
	<link rel="preconnect" href="https://fonts.bunny.net"/>
	<link href="https://fonts.bunny.net/css?family=outfit:400,500,600,700|jetbrains-mono:400" rel="stylesheet"/>
</head>
<body class="bg-bao-bg text-bao-text min-h-screen flex items-center justify-center">
	<div class="text-center">
		<h1 class="text-5xl font-heading font-bold mb-4">🔔 BanhBaoRing</h1>
		<p class="text-bao-muted mb-8">Secure key management for the decentralized web</p>
		<div class="flex gap-4 justify-center">
			<a href="/login" class="px-6 py-3 bg-gradient-to-r from-amber-400 to-rose-500 text-bao-bg font-semibold rounded-xl hover:shadow-lg transition-shadow">
				Login
			</a>
			<a href="/signup" class="px-6 py-3 border border-bao-accent/50 text-bao-accent rounded-xl hover:bg-bao-accent/10 transition-colors">
				Sign Up
			</a>
		</div>
	</div>
</body>
</html>`))
}

// LoginPage renders the login page.
func (h *WebHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// Check if user is already logged in
	session, _ := h.sessionStore.Get(r, SessionCookieName)
	if session.Values["user_id"] != nil {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}

	errorMsg := r.URL.Query().Get("error")
	component := pages.LoginPage(errorMsg)
	templ.Handler(component).ServeHTTP(w, r)
}

// Login handles the login form submission.
func (h *WebHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error=Invalid+form+data", http.StatusFound)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	if email == "" || password == "" {
		http.Redirect(w, r, "/login?error=Email+and+password+are+required", http.StatusFound)
		return
	}

	// Authenticate user
	user, sessionID, err := h.authService.Login(r.Context(), email, password)
	if err != nil {
		http.Redirect(w, r, "/login?error=Invalid+email+or+password", http.StatusFound)
		return
	}

	// Set session cookie
	h.setSessionCookie(w, r, user.ID.String(), sessionID)

	// Check if user needs onboarding
	orgs, err := h.orgService.ListUserOrgs(r.Context(), user.ID)
	if err != nil || len(orgs) == 0 {
		http.Redirect(w, r, "/onboarding", http.StatusFound)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// SignupPage renders the signup page.
func (h *WebHandler) SignupPage(w http.ResponseWriter, r *http.Request) {
	// Check if user is already logged in
	session, _ := h.sessionStore.Get(r, SessionCookieName)
	if session.Values["user_id"] != nil {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}

	errorMsg := r.URL.Query().Get("error")
	formValues := pages.SignupFormValues{
		Name:  r.URL.Query().Get("name"),
		Email: r.URL.Query().Get("email"),
	}
	component := pages.SignupPage(errorMsg, formValues)
	templ.Handler(component).ServeHTTP(w, r)
}

// Signup handles the signup form submission.
func (h *WebHandler) Signup(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/signup?error=Invalid+form+data", http.StatusFound)
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")

	// Validation
	if name == "" || email == "" || password == "" {
		http.Redirect(w, r, "/signup?error=All+fields+are+required&name="+name+"&email="+email, http.StatusFound)
		return
	}

	if len(password) < 8 {
		http.Redirect(w, r, "/signup?error=Password+must+be+at+least+8+characters&name="+name+"&email="+email, http.StatusFound)
		return
	}

	if password != passwordConfirm {
		http.Redirect(w, r, "/signup?error=Passwords+do+not+match&name="+name+"&email="+email, http.StatusFound)
		return
	}

	// Register user
	req := service.RegisterRequest{
		Email:    email,
		Password: password,
		Name:     name,
	}

	user, err := h.authService.Register(r.Context(), req)
	if err != nil {
		http.Redirect(w, r, "/signup?error=Email+already+registered&name="+name+"&email="+email, http.StatusFound)
		return
	}

	// Log the user in after registration
	_, sessionID, err := h.authService.Login(r.Context(), email, password)
	if err != nil {
		http.Redirect(w, r, "/login?success=Account+created", http.StatusFound)
		return
	}

	// Set session cookie
	h.setSessionCookie(w, r, user.ID.String(), sessionID)

	// Redirect to onboarding for new users
	http.Redirect(w, r, "/onboarding", http.StatusFound)
}

// ForgotPasswordPage renders the forgot password page.
func (h *WebHandler) ForgotPasswordPage(w http.ResponseWriter, r *http.Request) {
	errorMsg := r.URL.Query().Get("error")
	successMsg := r.URL.Query().Get("success")
	component := pages.ForgotPasswordPage(errorMsg, successMsg)
	templ.Handler(component).ServeHTTP(w, r)
}

// ForgotPassword handles the forgot password form submission.
func (h *WebHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/forgot-password?error=Invalid+form+data", http.StatusFound)
		return
	}

	email := r.FormValue("email")
	if email == "" {
		http.Redirect(w, r, "/forgot-password?error=Email+is+required", http.StatusFound)
		return
	}

	// Request password reset (always succeeds to prevent email enumeration)
	_, _ = h.authService.RequestPasswordReset(r.Context(), email)

	// Always show success to prevent email enumeration
	http.Redirect(w, r, "/forgot-password?success=If+an+account+exists+with+that+email,+we've+sent+a+password+reset+link.", http.StatusFound)
}

// ResetPasswordPage renders the reset password page.
func (h *WebHandler) ResetPasswordPage(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	errorMsg := r.URL.Query().Get("error")

	if token == "" {
		http.Redirect(w, r, "/forgot-password?error=Invalid+or+expired+reset+link", http.StatusFound)
		return
	}

	component := pages.ResetPasswordPage(token, errorMsg)
	templ.Handler(component).ServeHTTP(w, r)
}

// ResetPassword handles the reset password form submission.
func (h *WebHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/forgot-password?error=Invalid+form+data", http.StatusFound)
		return
	}

	token := r.FormValue("token")
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")

	if token == "" {
		http.Redirect(w, r, "/forgot-password?error=Invalid+reset+token", http.StatusFound)
		return
	}

	if len(password) < 8 {
		http.Redirect(w, r, "/reset-password?token="+token+"&error=Password+must+be+at+least+8+characters", http.StatusFound)
		return
	}

	if password != passwordConfirm {
		http.Redirect(w, r, "/reset-password?token="+token+"&error=Passwords+do+not+match", http.StatusFound)
		return
	}

	if err := h.authService.ResetPassword(r.Context(), token, password); err != nil {
		http.Redirect(w, r, "/forgot-password?error=Invalid+or+expired+reset+link", http.StatusFound)
		return
	}

	http.Redirect(w, r, "/login?success=Password+reset+successfully.+Please+log+in.", http.StatusFound)
}

// OAuthStart initiates OAuth flow for the given provider.
func (h *WebHandler) OAuthStart(w http.ResponseWriter, r *http.Request) {
	if h.oauthService == nil {
		http.Redirect(w, r, "/login?error=OAuth+not+configured", http.StatusFound)
		return
	}

	provider := chi.URLParam(r, "provider")

	// Generate state for CSRF protection
	state, err := generateSecureState()
	if err != nil {
		http.Redirect(w, r, "/login?error=Failed+to+initialize+OAuth", http.StatusFound)
		return
	}

	// Store state in session for verification
	session, _ := h.sessionStore.Get(r, OAuthStateCookie)
	session.Values["state"] = state
	session.Options.MaxAge = 300 // 5 minutes
	if err := session.Save(r, w); err != nil {
		http.Redirect(w, r, "/login?error=Failed+to+initialize+OAuth", http.StatusFound)
		return
	}

	// Get authorization URL
	authURL, err := h.oauthService.GetAuthURL(provider, state)
	if err != nil {
		http.Redirect(w, r, "/login?error=OAuth+provider+not+configured", http.StatusFound)
		return
	}

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// OAuthCallback handles OAuth callback from the provider.
func (h *WebHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	if h.oauthService == nil {
		http.Redirect(w, r, "/login?error=OAuth+not+configured", http.StatusFound)
		return
	}

	provider := chi.URLParam(r, "provider")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	// Handle OAuth error from provider
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		http.Redirect(w, r, "/login?error=OAuth+authentication+failed", http.StatusFound)
		return
	}

	if code == "" {
		http.Redirect(w, r, "/login?error=Missing+authorization+code", http.StatusFound)
		return
	}

	// Verify state
	session, _ := h.sessionStore.Get(r, OAuthStateCookie)
	savedState, ok := session.Values["state"].(string)
	if !ok || savedState != state {
		http.Redirect(w, r, "/login?error=Invalid+OAuth+state", http.StatusFound)
		return
	}

	// Clear OAuth state cookie
	session.Options.MaxAge = -1
	session.Save(r, w)

	// Exchange code for user info and session
	user, sessionID, err := h.oauthService.HandleCallback(r.Context(), provider, code)
	if err != nil {
		http.Redirect(w, r, "/login?error=OAuth+authentication+failed", http.StatusFound)
		return
	}

	// Set session cookie
	h.setSessionCookie(w, r, user.ID.String(), sessionID)

	// Check if user needs onboarding
	orgs, err := h.orgService.ListUserOrgs(r.Context(), user.ID)
	if err != nil || len(orgs) == 0 {
		http.Redirect(w, r, "/onboarding", http.StatusFound)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// Health returns a simple health check response.
func (h *WebHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// ============================================
// Protected Page Handlers
// ============================================

// Logout handles user logout.
func (h *WebHandler) Logout(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, SessionCookieName)

	// Invalidate server-side session if we have a session ID
	if sessionID, ok := session.Values["session_id"].(string); ok && sessionID != "" {
		_ = h.authService.Logout(r.Context(), sessionID)
	}

	// Clear cookie
	session.Options.MaxAge = -1
	session.Save(r, w)

	http.Redirect(w, r, "/login", http.StatusFound)
}

// ============================================
// Onboarding Handlers
// ============================================

// OnboardingPage renders the first onboarding step.
func (h *WebHandler) OnboardingPage(w http.ResponseWriter, r *http.Request) {
	user, ok := GetUserFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Check if user already has an org
	orgs, _ := h.orgService.ListUserOrgs(r.Context(), user.ID)
	if len(orgs) > 0 {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}

	data := pages.OnboardingData{
		UserName: getUserDisplayName(user),
		Step:     1,
	}

	component := pages.OnboardingStep1(data)
	templ.Handler(component).ServeHTTP(w, r)
}

// OnboardingStep1 handles organization creation.
func (h *WebHandler) OnboardingStep1(w http.ResponseWriter, r *http.Request) {
	user, ok := GetUserFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderOnboardingError(w, 1, user, "Invalid form data")
		return
	}

	orgName := r.FormValue("org_name")
	if orgName == "" {
		h.renderOnboardingError(w, 1, user, "Organization name is required")
		return
	}

	// Create organization
	_, err := h.orgService.Create(r.Context(), orgName, user.ID)
	if err != nil {
		h.renderOnboardingError(w, 1, user, "Failed to create organization")
		return
	}

	http.Redirect(w, r, "/onboarding/step2", http.StatusFound)
}

// OnboardingStep2Page renders the key creation step.
func (h *WebHandler) OnboardingStep2Page(w http.ResponseWriter, r *http.Request) {
	user, ok := GetUserFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	data := pages.OnboardingData{
		UserName: getUserDisplayName(user),
		Step:     2,
	}

	component := pages.OnboardingStep2(data)
	templ.Handler(component).ServeHTTP(w, r)
}

// OnboardingStep2 handles key creation.
func (h *WebHandler) OnboardingStep2(w http.ResponseWriter, r *http.Request) {
	user, ok := GetUserFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderOnboardingError(w, 2, user, "Invalid form data")
		return
	}

	keyName := r.FormValue("key_name")
	// keyType := r.FormValue("key_type") // For future use

	if keyName == "" {
		h.renderOnboardingError(w, 2, user, "Key name is required")
		return
	}

	// TODO: Create key using key service
	// For now, proceed to step 3

	http.Redirect(w, r, "/onboarding/step3?key_name="+keyName, http.StatusFound)
}

// OnboardingStep3Page renders the integration guide.
func (h *WebHandler) OnboardingStep3Page(w http.ResponseWriter, r *http.Request) {
	user, ok := GetUserFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	data := pages.OnboardingData{
		UserName:   getUserDisplayName(user),
		Step:       3,
		KeyName:    r.URL.Query().Get("key_name"),
		KeyAddress: r.URL.Query().Get("key_address"),
	}

	component := pages.OnboardingStep3(data)
	templ.Handler(component).ServeHTTP(w, r)
}

func (h *WebHandler) renderOnboardingError(w http.ResponseWriter, step int, user *models.User, errorMsg string) {
	data := pages.OnboardingData{
		UserName: getUserDisplayName(user),
		Step:     step,
		ErrorMsg: errorMsg,
	}

	var component templ.Component
	switch step {
	case 1:
		component = pages.OnboardingStep1(data)
	case 2:
		component = pages.OnboardingStep2(data)
	default:
		component = pages.OnboardingStep1(data)
	}
	templ.Handler(component).ServeHTTP(w, nil)
}

// Dashboard renders the main dashboard page.
func (h *WebHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	// TODO: Render dashboard page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- Dashboard page placeholder - implement with templ -->`))
}

// Key handlers are implemented in keys.go
// Usage and Audit handlers are implemented in audit.go

// Settings handlers are implemented in settings.go

// ============================================
// Helper Methods
// ============================================

func (h *WebHandler) setSessionCookie(w http.ResponseWriter, r *http.Request, userID, sessionID string) {
	session, _ := h.sessionStore.Get(r, SessionCookieName)
	session.Values["user_id"] = userID
	session.Values["session_id"] = sessionID
	session.Options.MaxAge = 7 * 24 * 60 * 60 // 7 days
	session.Options.HttpOnly = true
	session.Options.Secure = r.TLS != nil
	session.Options.SameSite = http.SameSiteLaxMode
	session.Save(r, w)
}

func getUserDisplayName(user *models.User) string {
	if user.Name != nil && *user.Name != "" {
		return *user.Name
	}
	return "there"
}

// generateSecureState generates a cryptographically secure random state for OAuth CSRF protection.
func generateSecureState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GetUserFromContext returns the user from context.
func GetUserFromContext(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(ContextKeyUser).(*models.User)
	return user, ok
}

// GetUserIDFromContext returns the user ID from context.
func GetUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userIDStr, ok := ctx.Value(ContextKeyUserID).(string)
	if !ok || userIDStr == "" {
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, false
	}
	return userID, true
}
