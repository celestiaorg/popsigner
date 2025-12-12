// Package main is the entry point for the Control Plane API server.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/Bidon15/banhbaoring/control-plane/internal/config"
	"github.com/Bidon15/banhbaoring/control-plane/internal/database"
	"github.com/Bidon15/banhbaoring/control-plane/internal/middleware"
	"github.com/Bidon15/banhbaoring/control-plane/internal/models"
	"github.com/Bidon15/banhbaoring/control-plane/internal/pkg/response"
	"github.com/Bidon15/banhbaoring/control-plane/internal/repository"
	"github.com/Bidon15/banhbaoring/control-plane/internal/service"
	"github.com/Bidon15/banhbaoring/control-plane/templates/layouts"
	"github.com/Bidon15/banhbaoring/control-plane/templates/pages"
)

const sessionCookieName = "banhbao_session"

func main() {
	// Setup structured logger
	logLevel := slog.LevelInfo
	if os.Getenv("DEBUG") == "true" {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger.Info("Starting Control Plane API",
		slog.String("environment", cfg.Server.Environment),
		slog.Int("port", cfg.Server.Port),
	)

	// Connect to PostgreSQL
	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	logger.Info("Connected to PostgreSQL")

	// Run migrations
	if err := db.RunMigrations(cfg.Database); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	logger.Info("Database migrations completed")

	// Connect to Redis
	redis, err := database.NewRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()
	logger.Info("Connected to Redis")

	// Initialize repositories
	userRepo := repository.NewUserRepository(db.Pool())
	sessionRepo := repository.NewSessionRepository(db.Pool())

	// Initialize OAuth service
	oauthSvc := service.NewOAuthService(&cfg.Auth, userRepo, sessionRepo)

	logger.Info("OAuth providers configured",
		slog.Any("providers", oauthSvc.GetSupportedProviders()),
	)

	// Setup router
	r := chi.NewRouter()

	// Global middleware stack
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logging(logger))
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.CORS())
	r.Use(chimiddleware.Timeout(30 * time.Second))

	// Health check endpoints (no auth required)
	r.Get("/health", healthHandler(db, redis))
	r.Get("/ready", readyHandler(db, redis))

	// Static files for web dashboard
	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// Web dashboard landing page
	r.Get("/", landingPageHandler())
	r.Get("/login", loginPageHandler())
	r.Get("/signup", signupPageHandler())
	r.Get("/logout", logoutHandler(sessionRepo))

	// Dashboard (protected by session check)
	r.Get("/dashboard", dashboardHandler(sessionRepo, userRepo))

	// Protected dashboard pages
	r.Get("/keys", keysListHandler(sessionRepo, userRepo))
	r.Get("/keys/new", keysNewHandler(sessionRepo, userRepo))
	r.Get("/settings/api-keys", settingsAPIKeysHandler(sessionRepo, userRepo))
	r.Get("/settings/profile", settingsProfileHandler(sessionRepo, userRepo))
	r.Get("/docs", docsHandler())

	// OAuth routes - using the service
	r.Get("/auth/github", oauthRedirectHandler(oauthSvc, "github"))
	r.Get("/auth/github/callback", oauthCallbackHandler(oauthSvc, "github", cfg))
	r.Get("/auth/google", oauthRedirectHandler(oauthSvc, "google"))
	r.Get("/auth/google/callback", oauthCallbackHandler(oauthSvc, "google", cfg))

	// API v1 routes
	r.Route("/v1", func(r chi.Router) {
		// Rate limiting for API routes
		r.Use(middleware.RateLimit(redis, middleware.DefaultRateLimitConfig()))

		// Public endpoints (no auth)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			response.OK(w, map[string]string{
				"name":    "BanhBaoRing Control Plane API",
				"version": "1.0.0",
			})
		})

		// Auth routes will be mounted by Agent 08A/08B/08C
		// r.Mount("/auth", authHandler.Routes())

		// Protected routes (placeholder - will be implemented by other agents)
		r.Group(func(r chi.Router) {
			// Authentication middleware will be added by auth agents
			// r.Use(middleware.Auth(...))

			// Organizations (Agent 09A)
			// r.Mount("/orgs", orgsHandler.Routes())

			// Keys (Agent 09B)
			// r.Mount("/keys", keysHandler.Routes())

			// Namespaces (Agent 09A)
			// r.Mount("/namespaces", namespacesHandler.Routes())

			// Audit logs (Agent 10A)
			// r.Mount("/audit", auditHandler.Routes())

			// Webhooks (Agent 10A)
			// r.Mount("/webhooks", webhooksHandler.Routes())
		})
	})

	// Create server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  time.Minute,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Server listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("Shutting down server", slog.String("signal", sig.String()))

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	logger.Info("Server stopped gracefully")
}

// healthHandler returns a simple health check that always succeeds if the server is running.
func healthHandler(db *database.Postgres, redis *database.Redis) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}
}

// readyHandler returns a readiness check that verifies database and Redis connections.
func readyHandler(db *database.Postgres, redis *database.Redis) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Check database connection
		if err := db.Ping(ctx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"error","component":"database"}`))
			return
		}

		// Check Redis connection
		if err := redis.Ping(ctx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"error","component":"redis"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","database":"connected","redis":"connected"}`))
	}
}

// landingPageHandler serves the web dashboard landing page using templ.
func landingPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.LandingPage().Render(r.Context(), w)
	}
}

// loginPageHandler serves the login page using the templ template.
func loginPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		errorMsg := r.URL.Query().Get("error")
		pages.LoginPage(errorMsg).Render(r.Context(), w)
	}
}

// signupPageHandler serves the signup page using the templ template.
func signupPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		errorMsg := r.URL.Query().Get("error")
		pages.SignupPage(errorMsg).Render(r.Context(), w)
	}
}

// oauthRedirectHandler redirects the user to the OAuth provider.
func oauthRedirectHandler(oauthSvc service.OAuthService, provider string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Generate a random state for CSRF protection
		state := fmt.Sprintf("%d", time.Now().UnixNano())

		authURL, err := oauthSvc.GetAuthURL(provider, state)
		if err != nil {
			slog.Error("Failed to get OAuth URL", slog.String("provider", provider), slog.String("error", err.Error()))
			http.Redirect(w, r, "/login?error="+url.QueryEscape("OAuth provider not configured"), http.StatusFound)
			return
		}

		// Store state in a cookie for verification (short-lived)
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_state",
			Value:    state,
			Path:     "/",
			MaxAge:   300, // 5 minutes
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// oauthCallbackHandler handles the OAuth callback from the provider.
func oauthCallbackHandler(oauthSvc service.OAuthService, provider string, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			slog.Warn("OAuth callback missing code", slog.String("provider", provider))
			http.Redirect(w, r, "/login?error="+url.QueryEscape("Authorization failed"), http.StatusFound)
			return
		}

		// TODO: Verify state parameter matches cookie for CSRF protection
		// For now, we'll skip this for simplicity

		// Handle the OAuth callback - this exchanges code for token, fetches user info, creates/finds user, creates session
		user, sessionID, err := oauthSvc.HandleCallback(r.Context(), provider, code)
		if err != nil {
			slog.Error("OAuth callback failed",
				slog.String("provider", provider),
				slog.String("error", err.Error()),
			)
			http.Redirect(w, r, "/login?error="+url.QueryEscape("Authentication failed: "+err.Error()), http.StatusFound)
			return
		}

		slog.Info("User authenticated via OAuth",
			slog.String("provider", provider),
			slog.String("user_id", user.ID.String()),
			slog.String("email", user.Email),
		)

		// Set the session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    sessionID,
			Path:     "/",
			MaxAge:   int(cfg.Auth.SessionExpiry.Seconds()),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		// Clear the OAuth state cookie
		http.SetCookie(w, &http.Cookie{
			Name:   "oauth_state",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})

		// Redirect to dashboard
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	}
}

// dashboardHandler serves the dashboard page for authenticated users.
func dashboardHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session cookie
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Validate session
		session, err := sessionRepo.Get(r.Context(), cookie.Value)
		if err != nil || session == nil {
			// Invalid or expired session
			http.SetCookie(w, &http.Cookie{
				Name:   sessionCookieName,
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Check if session is expired
		if session.ExpiresAt.Before(time.Now()) {
			_ = sessionRepo.Delete(r.Context(), cookie.Value)
			http.SetCookie(w, &http.Cookie{
				Name:   sessionCookieName,
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			http.Redirect(w, r, "/login?error="+url.QueryEscape("Session expired"), http.StatusFound)
			return
		}

		// Get user info
		user, err := userRepo.GetByID(r.Context(), session.UserID)
		if err != nil || user == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Render dashboard
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.DashboardPage(user).Render(r.Context(), w)
	}
}

// logoutHandler logs the user out by deleting their session.
func logoutHandler(sessionRepo repository.SessionRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session cookie
		cookie, err := r.Cookie(sessionCookieName)
		if err == nil && cookie.Value != "" {
			// Delete session from database
			_ = sessionRepo.Delete(r.Context(), cookie.Value)
		}

		// Clear the session cookie
		http.SetCookie(w, &http.Cookie{
			Name:   sessionCookieName,
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// getAuthenticatedUser returns the authenticated user from the session, or redirects to login.
func getAuthenticatedUser(w http.ResponseWriter, r *http.Request, sessionRepo repository.SessionRepository, userRepo repository.UserRepository) *models.User {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return nil
	}

	session, err := sessionRepo.Get(r.Context(), cookie.Value)
	if err != nil || session == nil || session.ExpiresAt.Before(time.Now()) {
		http.SetCookie(w, &http.Cookie{
			Name:   sessionCookieName,
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		http.Redirect(w, r, "/login", http.StatusFound)
		return nil
	}

	user, err := userRepo.GetByID(r.Context(), session.UserID)
	if err != nil || user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return nil
	}

	return user
}

// buildDashboardData creates the common dashboard data from a user.
func buildDashboardData(user *models.User, activePath string) layouts.DashboardData {
	name := user.Email
	if user.Name != nil && *user.Name != "" {
		name = *user.Name
	}
	avatarURL := ""
	if user.AvatarURL != nil {
		avatarURL = *user.AvatarURL
	}
	return layouts.DashboardData{
		UserName:   name,
		UserEmail:  user.Email,
		AvatarURL:  avatarURL,
		OrgName:    "Personal", // TODO: Get from org membership
		OrgPlan:    "Free",     // TODO: Get from org
		ActivePath: activePath,
	}
}

// keysListHandler serves the keys list page.
func keysListHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		dashData := buildDashboardData(user, "/keys")
		data := pages.KeysPageData{
			UserName:   dashData.UserName,
			UserEmail:  dashData.UserEmail,
			AvatarURL:  dashData.AvatarURL,
			OrgName:    dashData.OrgName,
			OrgPlan:    dashData.OrgPlan,
			Keys:       []*models.Key{},       // TODO: Fetch from repository
			Namespaces: []*models.Namespace{}, // TODO: Fetch from repository
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.KeysListPage(data).Render(r.Context(), w)
	}
}

// keysNewHandler returns the create key modal content for HTMX, or redirects for direct access.
func keysNewHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Check if this is an HTMX request
		if r.Header.Get("HX-Request") == "true" {
			// Return the modal content for HTMX
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			pages.CreateKeyModal().Render(r.Context(), w)
			return
		}

		// For direct navigation, redirect to /keys (the modal should be opened via button)
		http.Redirect(w, r, "/keys", http.StatusFound)
	}
}

// settingsAPIKeysHandler serves the API keys settings page.
func settingsAPIKeysHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		data := pages.APIKeysPageData{
			DashboardData: buildDashboardData(user, "/settings/api-keys"),
			APIKeys:       []*models.APIKey{}, // TODO: Fetch from repository
			CanCreate:     true,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.SettingsAPIKeysPage(data).Render(r.Context(), w)
	}
}

// settingsProfileHandler serves the profile settings page.
func settingsProfileHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		avatarURL := ""
		if user.AvatarURL != nil {
			avatarURL = *user.AvatarURL
		}
		name := ""
		if user.Name != nil {
			name = *user.Name
		}
		oauthProvider := ""
		if user.OAuthProvider != nil {
			oauthProvider = *user.OAuthProvider
		}

		data := pages.ProfilePageData{
			DashboardData: buildDashboardData(user, "/settings/profile"),
			Email:         user.Email,
			Name:          name,
			AvatarURL:     avatarURL,
			EmailVerified: user.EmailVerified,
			OAuthProvider: oauthProvider,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.SettingsProfilePage(data).Render(r.Context(), w)
	}
}

// docsHandler redirects to documentation.
func docsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// For now, redirect to GitHub docs or show a simple page
		http.Redirect(w, r, "https://github.com/Bidon15/banhbaoring#readme", http.StatusFound)
	}
}
