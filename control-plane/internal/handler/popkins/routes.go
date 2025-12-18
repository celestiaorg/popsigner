package popkins

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

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
		http.Redirect(w, r, "/popkins/deployments", http.StatusFound)
	})

	// Deployment routes
	r.Route("/deployments", func(r chi.Router) {
		r.Get("/", h.DeploymentsList)           // GET /popkins/deployments
		r.Get("/new", h.DeploymentsNew)         // GET /popkins/deployments/new
		r.Post("/", h.DeploymentsCreate)        // POST /popkins/deployments

		// Individual deployment routes
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.DeploymentDetail)                   // GET /popkins/deployments/{id}
			r.Get("/status", h.DeploymentStatus)             // GET /popkins/deployments/{id}/status
			r.Get("/progress-partial", h.DeploymentProgressPartial) // GET /popkins/deployments/{id}/progress-partial (HTMX)
			r.Get("/complete", h.DeploymentComplete)         // GET /popkins/deployments/{id}/complete
			r.Get("/bundle", h.DownloadBundle)               // GET /popkins/deployments/{id}/bundle
			r.Post("/resume", h.DeploymentResume)            // POST /popkins/deployments/{id}/resume
		})
	})

	return r
}

// RequireAuth middleware ensures the user is authenticated.
func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := h.sessionStore.Get(r, SessionCookieName)
		if err != nil {
			h.handleAuthError(w, r)
			return
		}

		userIDStr, ok := session.Values["user_id"].(string)
		if !ok || userIDStr == "" {
			h.handleAuthError(w, r)
			return
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			h.handleAuthError(w, r)
			return
		}

		// Load user from database
		user, err := h.authService.GetUserByID(r.Context(), userID)
		if err != nil {
			// Session exists but user not found, clear session
			session.Options.MaxAge = -1
			session.Save(r, w)
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

// CreateSessionStore creates a new session store for POPKins.
// This uses the same session store as the main dashboard for SSO.
func CreateSessionStore(secret string) sessions.Store {
	store := sessions.NewCookieStore([]byte(secret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
	return store
}

