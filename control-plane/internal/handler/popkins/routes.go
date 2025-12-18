package popkins

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Bidon15/popsigner/control-plane/internal/models"
)

// contextKey for request context values
type contextKey string

const (
	// ContextKeyUser is the context key for the authenticated user.
	ContextKeyUser contextKey = "popkins_user"
)

// Routes returns a chi router with all POPKins routes configured.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// All POPKins routes require authentication
	r.Use(h.RequireAuth)

	// Root redirects to deployments list
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/deployments", http.StatusFound)
	})

	// Handle /dashboard redirect for users coming from OAuth with fallback URL
	r.Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/deployments", http.StatusFound)
	})

	// Handle /* paths for subdomain access
	// When accessed via subdomain, links may still point to /* paths
	// Strip the /popkins prefix and redirect to the correct path
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// Remove /popkins prefix and redirect
		newPath := r.URL.Path[8:] // len("/popkins") = 8
		if newPath == "" {
			newPath = "/deployments"
		}
		http.Redirect(w, r, newPath, http.StatusFound)
	})

	// Deployment routes
	r.Route("/deployments", func(r chi.Router) {
		r.Get("/", h.DeploymentsList)           // GET /deployments
		r.Get("/new", h.DeploymentsNew)         // GET /deployments/new
		r.Post("/", h.DeploymentsCreate)        // POST /deployments

		// Individual deployment routes
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.DeploymentDetail)                   // GET /deployments/{id}
			r.Get("/status", h.DeploymentStatus)             // GET /deployments/{id}/status
			r.Get("/progress-partial", h.DeploymentProgressPartial) // GET /deployments/{id}/progress-partial (HTMX)
			r.Get("/complete", h.DeploymentComplete)         // GET /deployments/{id}/complete
			r.Get("/bundle", h.DownloadBundle)               // GET /deployments/{id}/bundle
			r.Post("/resume", h.DeploymentResume)            // POST /deployments/{id}/resume
		})
	})

	return r
}

// RequireAuth middleware ensures the user is authenticated.
// Uses the same session mechanism as the main dashboard (cookie + DB lookup).
func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get session cookie (same as main dashboard)
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			h.handleAuthError(w, r)
			return
		}

		// Look up session in database
		session, err := h.sessionRepo.Get(r.Context(), cookie.Value)
		if err != nil || session == nil {
			h.handleAuthError(w, r)
			return
		}

		// Check if session is expired
		if session.ExpiresAt.Before(time.Now()) {
			h.handleAuthError(w, r)
			return
		}

		// Load user from database
		user, err := h.userRepo.GetByID(r.Context(), session.UserID)
		if err != nil || user == nil {
			h.handleAuthError(w, r)
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), ContextKeyUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserFromContext returns the user from context.
func GetUserFromContext(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(ContextKeyUser).(*models.User)
	return user, ok
}

