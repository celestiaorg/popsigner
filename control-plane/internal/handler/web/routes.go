// Package web provides HTTP handlers for the web dashboard.
package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"

	"github.com/Bidon15/banhbaoring/control-plane/internal/service"
)

// WebHandler handles HTTP requests for the web dashboard.
type WebHandler struct {
	authService    service.AuthService
	keyService     service.KeyService
	orgService     service.OrgService
	billingService service.BillingService
	auditService   service.AuditService
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
	sessionStore sessions.Store,
) *WebHandler {
	return &WebHandler{
		authService:    authService,
		keyService:     keyService,
		orgService:     orgService,
		billingService: billingService,
		auditService:   auditService,
		sessionStore:   sessionStore,
	}
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
			r.Post("/team/invite", h.SettingsTeamInvite)
			r.Delete("/team/{id}", h.SettingsTeamRemove)
			r.Get("/api-keys", h.SettingsAPIKeys)
			r.Post("/api-keys", h.SettingsAPIKeysCreate)
			r.Delete("/api-keys/{id}", h.SettingsAPIKeysDelete)
			r.Get("/billing", h.SettingsBilling)
			r.Post("/billing/portal", h.SettingsBillingPortal)
		})
	})

	return r
}

// ============================================
// Middleware
// ============================================

// RequireAuth middleware ensures the user is authenticated.
func (h *WebHandler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := h.sessionStore.Get(r, "session")
		if err != nil || session.Values["user_id"] == nil {
			// Check if this is an HTMX request
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ============================================
// Public Page Handlers
// ============================================

// LandingPage renders the public landing page.
func (h *WebHandler) LandingPage(w http.ResponseWriter, r *http.Request) {
	// Check if user is already logged in
	session, _ := h.sessionStore.Get(r, "session")
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
	// TODO: Render login page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- Login page placeholder - implement with templ -->`))
}

// Login handles the login form submission.
func (h *WebHandler) Login(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement login logic
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// SignupPage renders the signup page.
func (h *WebHandler) SignupPage(w http.ResponseWriter, r *http.Request) {
	// TODO: Render signup page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- Signup page placeholder - implement with templ -->`))
}

// Signup handles the signup form submission.
func (h *WebHandler) Signup(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement signup logic
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// ForgotPasswordPage renders the forgot password page.
func (h *WebHandler) ForgotPasswordPage(w http.ResponseWriter, r *http.Request) {
	// TODO: Render forgot password page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- Forgot password page placeholder - implement with templ -->`))
}

// ForgotPassword handles the forgot password form submission.
func (h *WebHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement forgot password logic
	http.Redirect(w, r, "/login", http.StatusFound)
}

// ResetPasswordPage renders the reset password page.
func (h *WebHandler) ResetPasswordPage(w http.ResponseWriter, r *http.Request) {
	// TODO: Render reset password page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- Reset password page placeholder - implement with templ -->`))
}

// ResetPassword handles the reset password form submission.
func (h *WebHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement reset password logic
	http.Redirect(w, r, "/login", http.StatusFound)
}

// OAuthStart initiates OAuth flow for the given provider.
func (h *WebHandler) OAuthStart(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement OAuth start
	provider := chi.URLParam(r, "provider")
	_ = provider
	http.Error(w, "OAuth not implemented", http.StatusNotImplemented)
}

// OAuthCallback handles OAuth callback from the provider.
func (h *WebHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement OAuth callback
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
	session, _ := h.sessionStore.Get(r, "session")
	session.Options.MaxAge = -1
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// Dashboard renders the main dashboard page.
func (h *WebHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	// TODO: Render dashboard page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- Dashboard page placeholder - implement with templ -->`))
}

// KeysList renders the keys list page.
func (h *WebHandler) KeysList(w http.ResponseWriter, r *http.Request) {
	// TODO: Render keys list page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- Keys list page placeholder - implement with templ -->`))
}

// KeysNew renders the new key form page.
func (h *WebHandler) KeysNew(w http.ResponseWriter, r *http.Request) {
	// TODO: Render new key page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- New key page placeholder - implement with templ -->`))
}

// KeysCreate handles creating a new key.
func (h *WebHandler) KeysCreate(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement key creation
	http.Redirect(w, r, "/keys", http.StatusFound)
}

// WorkerKeysNew renders the new worker keys form page.
func (h *WebHandler) WorkerKeysNew(w http.ResponseWriter, r *http.Request) {
	// TODO: Render new worker keys page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- New worker keys page placeholder - implement with templ -->`))
}

// WorkerKeysCreate handles creating worker keys in bulk.
func (h *WebHandler) WorkerKeysCreate(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement bulk key creation
	http.Redirect(w, r, "/keys", http.StatusFound)
}

// KeysDetail renders a key detail page.
func (h *WebHandler) KeysDetail(w http.ResponseWriter, r *http.Request) {
	// TODO: Render key detail page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- Key detail page placeholder - implement with templ -->`))
}

// KeysSignTest handles test signing with a key.
func (h *WebHandler) KeysSignTest(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement test signing
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"signature":"test"}`))
}

// KeysDelete handles key deletion.
func (h *WebHandler) KeysDelete(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement key deletion
	w.Header().Set("HX-Redirect", "/keys")
	w.WriteHeader(http.StatusOK)
}

// Usage renders the usage analytics page.
func (h *WebHandler) Usage(w http.ResponseWriter, r *http.Request) {
	// TODO: Render usage page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- Usage page placeholder - implement with templ -->`))
}

// AuditLog renders the audit log page.
func (h *WebHandler) AuditLog(w http.ResponseWriter, r *http.Request) {
	// TODO: Render audit log page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- Audit log page placeholder - implement with templ -->`))
}

// ============================================
// Settings Page Handlers
// ============================================

// SettingsProfile renders the profile settings page.
func (h *WebHandler) SettingsProfile(w http.ResponseWriter, r *http.Request) {
	// TODO: Render profile settings page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- Profile settings page placeholder - implement with templ -->`))
}

// SettingsProfileUpdate handles profile update.
func (h *WebHandler) SettingsProfileUpdate(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement profile update
	w.Header().Set("HX-Trigger", "profile-updated")
	w.WriteHeader(http.StatusOK)
}

// SettingsTeam renders the team settings page.
func (h *WebHandler) SettingsTeam(w http.ResponseWriter, r *http.Request) {
	// TODO: Render team settings page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- Team settings page placeholder - implement with templ -->`))
}

// SettingsTeamInvite handles team member invitation.
func (h *WebHandler) SettingsTeamInvite(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement team invitation
	w.Header().Set("HX-Trigger", "team-invited")
	w.WriteHeader(http.StatusOK)
}

// SettingsTeamRemove handles team member removal.
func (h *WebHandler) SettingsTeamRemove(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement team member removal
	w.Header().Set("HX-Trigger", "team-removed")
	w.WriteHeader(http.StatusOK)
}

// SettingsAPIKeys renders the API keys settings page.
func (h *WebHandler) SettingsAPIKeys(w http.ResponseWriter, r *http.Request) {
	// TODO: Render API keys settings page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- API keys settings page placeholder - implement with templ -->`))
}

// SettingsAPIKeysCreate handles API key creation.
func (h *WebHandler) SettingsAPIKeysCreate(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement API key creation
	w.Header().Set("HX-Trigger", "api-key-created")
	w.WriteHeader(http.StatusOK)
}

// SettingsAPIKeysDelete handles API key deletion.
func (h *WebHandler) SettingsAPIKeysDelete(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement API key deletion
	w.Header().Set("HX-Trigger", "api-key-deleted")
	w.WriteHeader(http.StatusOK)
}

// SettingsBilling renders the billing settings page.
func (h *WebHandler) SettingsBilling(w http.ResponseWriter, r *http.Request) {
	// TODO: Render billing settings page template using templ
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!-- Billing settings page placeholder - implement with templ -->`))
}

// SettingsBillingPortal handles redirect to Stripe billing portal.
func (h *WebHandler) SettingsBillingPortal(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement Stripe portal redirect
	http.Error(w, "Billing portal not implemented", http.StatusNotImplemented)
}

