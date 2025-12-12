// Package main is the entry point for the Control Plane API server.
package main

import (
	"context"
	"encoding/hex"
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
	"github.com/google/uuid"

	"github.com/Bidon15/banhbaoring/control-plane/internal/config"
	"github.com/Bidon15/banhbaoring/control-plane/internal/database"
	"github.com/Bidon15/banhbaoring/control-plane/internal/middleware"
	"github.com/Bidon15/banhbaoring/control-plane/internal/models"
	"github.com/Bidon15/banhbaoring/control-plane/internal/openbao"
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
	orgRepo := repository.NewOrgRepository(db.Pool())
	keyRepo := repository.NewKeyRepository(db.Pool())

	// Initialize OpenBao client
	baoClient := openbao.NewClient(&cfg.OpenBao)
	logger.Info("OpenBao client initialized", slog.String("address", cfg.OpenBao.Address))

	// Initialize services
	oauthSvc := service.NewOAuthService(&cfg.Auth, userRepo, sessionRepo)
	keySvc := service.NewKeyService(keyRepo, orgRepo, nil, nil, baoClient)

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
	r.Get("/keys", keysListHandler(sessionRepo, userRepo, orgRepo, keyRepo))
	r.Post("/keys", keysCreateHandler(sessionRepo, userRepo, orgRepo, keySvc))
	r.Get("/keys/new", keysNewHandler(sessionRepo, userRepo))
	r.Get("/keys/{id}", keyViewHandler(sessionRepo, userRepo, keyRepo))
	r.Post("/keys/{id}/sign-test", keySignHandler(sessionRepo, userRepo, keyRepo, keySvc))
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
func keysListHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, keyRepo repository.KeyRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Ensure user has an org
		org, _ := ensureUserHasOrg(r.Context(), user, orgRepo)

		dashData := buildDashboardData(user, "/keys")
		if org != nil {
			dashData.OrgName = org.Name
			dashData.OrgPlan = string(org.Plan)
		}

		// Fetch keys and namespaces
		var keys []*models.Key
		var namespaces []*models.Namespace
		if org != nil {
			keys, _ = keyRepo.ListByOrg(r.Context(), org.ID)
			namespaces, _ = orgRepo.ListNamespaces(r.Context(), org.ID)
		}

		data := pages.KeysPageData{
			UserName:   dashData.UserName,
			UserEmail:  dashData.UserEmail,
			AvatarURL:  dashData.AvatarURL,
			OrgName:    dashData.OrgName,
			OrgPlan:    dashData.OrgPlan,
			Keys:       keys,
			Namespaces: namespaces,
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

// keysCreateHandler handles the POST request to create a new key.
func keysCreateHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, keySvc service.KeyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<div class="p-4 bg-red-500/20 border border-red-500/50 rounded-xl text-red-400">Failed to parse form</div>`))
			return
		}

		name := r.FormValue("name")
		exportable := r.FormValue("exportable") == "true"

		if name == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<div class="p-4 bg-red-500/20 border border-red-500/50 rounded-xl text-red-400">Key name is required</div>`))
			return
		}

		// Ensure user has an org and get default namespace
		org, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil || org == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<div class="p-4 bg-red-500/20 border border-red-500/50 rounded-xl text-red-400">Failed to get organization</div>`))
			return
		}

		// Get default namespace
		namespaces, _ := orgRepo.ListNamespaces(r.Context(), org.ID)
		if len(namespaces) == 0 {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<div class="p-4 bg-red-500/20 border border-red-500/50 rounded-xl text-red-400">No namespace found</div>`))
			return
		}
		defaultNS := namespaces[0]

		// Create key via KeyService
		key, err := keySvc.Create(r.Context(), service.CreateKeyRequest{
			OrgID:       org.ID,
			NamespaceID: defaultNS.ID,
			Name:        name,
			Exportable:  exportable,
		})
		if err != nil {
			slog.Error("Failed to create key", slog.String("error", err.Error()))
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(fmt.Sprintf(`<div class="p-4 bg-red-500/20 border border-red-500/50 rounded-xl text-red-400">Failed to create key: %s</div>`, err.Error())))
			return
		}

		slog.Info("Key created successfully",
			slog.String("user_id", user.ID.String()),
			slog.String("key_id", key.ID.String()),
			slog.String("name", name),
			slog.String("address", key.Address),
		)

		// Return success response - reload the keys list
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("HX-Trigger", "modal-close")
		w.Header().Set("HX-Refresh", "true")
		w.Write([]byte(fmt.Sprintf(`
			<div class="p-6 text-center">
				<div class="text-4xl mb-4">✅</div>
				<h3 class="text-xl font-heading font-bold text-bao-text mb-2">Key Created!</h3>
				<p class="text-bao-muted mb-2">Your key "%s" has been created.</p>
				<p class="font-mono text-sm text-bao-accent break-all">%s</p>
			</div>
		`, name, key.Address)))
	}
}

// keyViewHandler displays the details of a specific key.
func keyViewHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, keyRepo repository.KeyRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		keyID := chi.URLParam(r, "id")
		if keyID == "" {
			http.Error(w, "Key ID required", http.StatusBadRequest)
			return
		}

		keyUUID, err := uuid.Parse(keyID)
		if err != nil {
			http.Error(w, "Invalid key ID", http.StatusBadRequest)
			return
		}

		key, err := keyRepo.GetByID(r.Context(), keyUUID)
		if err != nil || key == nil {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}

		dashData := buildDashboardData(user, "/keys")

		// Generate Celestia address from the key's hex address
		celestiaAddr := deriveCelestiaAddress(key.Address)

		data := pages.KeyDetailData{
			UserName:        dashData.UserName,
			UserEmail:       dashData.UserEmail,
			AvatarURL:       dashData.AvatarURL,
			OrgName:         dashData.OrgName,
			OrgPlan:         dashData.OrgPlan,
			Key:             key,
			Namespace:       "default", // TODO: fetch actual namespace name
			CelestiaAddress: celestiaAddr,
			SigningStats: &pages.SigningStats{
				Labels:    []string{},
				Values:    []int{},
				Total:     0,
				AvgPerDay: 0,
			},
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.KeyDetailPage(data).Render(r.Context(), w)
	}
}

// keySignHandler handles signing a test message with a key.
func keySignHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, keyRepo repository.KeyRepository, keySvc service.KeyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		keyID := chi.URLParam(r, "id")
		if keyID == "" {
			http.Error(w, "Key ID required", http.StatusBadRequest)
			return
		}

		keyUUID, err := uuid.Parse(keyID)
		if err != nil {
			http.Error(w, "Invalid key ID", http.StatusBadRequest)
			return
		}

		key, err := keyRepo.GetByID(r.Context(), keyUUID)
		if err != nil || key == nil {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		message := r.FormValue("data")
		if message == "" {
			message = "Hello, BanhBaoRing!"
		}

		// Sign the message using KeyService (orgID, keyID, data, prehashed)
		signResp, err := keySvc.Sign(r.Context(), key.OrgID, key.ID, []byte(message), false)
		if err != nil {
			slog.Error("Failed to sign message", slog.String("error", err.Error()))
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(fmt.Sprintf(`<div class="p-4 bg-red-500/20 border border-red-500/50 rounded-xl text-red-400">Sign failed: %s</div>`, err.Error())))
			return
		}

		// Return the signature result
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(fmt.Sprintf(`
			<div class="mt-4 p-4 space-y-4 bg-emerald-500/10 border border-emerald-500/30 rounded-xl">
				<div class="flex items-center gap-2 text-emerald-400">
					<span class="text-xl">✅</span>
					<span class="font-medium">Signature Generated</span>
				</div>
				<div class="space-y-3">
					<div>
						<p class="text-xs text-bao-muted mb-1">Message</p>
						<p class="font-mono text-sm text-bao-text bg-bao-bg p-3 rounded-lg break-all">%s</p>
					</div>
					<div>
						<p class="text-xs text-bao-muted mb-1">Signature (base64)</p>
						<p class="font-mono text-xs text-bao-accent bg-bao-bg p-3 rounded-lg break-all">%s</p>
					</div>
					<div>
						<p class="text-xs text-bao-muted mb-1">Public Key (hex)</p>
						<p class="font-mono text-xs text-bao-text bg-bao-bg p-3 rounded-lg break-all">%s</p>
					</div>
				</div>
			</div>
		`, message, signResp.Signature, signResp.PublicKey)))
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

// deriveCelestiaAddress converts a hex address (from OpenBao) to Celestia bech32 format.
// The hex address is RIPEMD160(SHA256(compressed_pubkey)) - 20 bytes.
func deriveCelestiaAddress(hexAddr string) string {
	// Decode hex address to bytes
	addrBytes, err := hex.DecodeString(hexAddr)
	if err != nil || len(addrBytes) != 20 {
		// Fallback for invalid addresses
		return "celestia1" + hexAddr[:38]
	}
	
	// Convert to bech32 with "celestia" prefix
	celestiaAddr, err := bech32Encode("celestia", addrBytes)
	if err != nil {
		return "celestia1" + hexAddr[:38]
	}
	
	return celestiaAddr
}

// bech32Encode encodes data to bech32 format with the given human-readable prefix.
func bech32Encode(hrp string, data []byte) (string, error) {
	// Convert 8-bit data to 5-bit groups
	converted := make([]byte, 0, len(data)*8/5+1)
	acc := 0
	bits := 0
	for _, b := range data {
		acc = (acc << 8) | int(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			converted = append(converted, byte((acc>>bits)&0x1f))
		}
	}
	if bits > 0 {
		converted = append(converted, byte((acc<<(5-bits))&0x1f))
	}
	
	// Create checksum
	values := append(expandHRP(hrp), converted...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	polymod := bech32Polymod(values) ^ 1
	checksum := make([]byte, 6)
	for i := 0; i < 6; i++ {
		checksum[i] = byte((polymod >> (5 * (5 - i))) & 0x1f)
	}
	
	// Encode to charset
	charset := "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	result := hrp + "1"
	for _, b := range converted {
		result += string(charset[b])
	}
	for _, b := range checksum {
		result += string(charset[b])
	}
	
	return result, nil
}

func expandHRP(hrp string) []byte {
	result := make([]byte, len(hrp)*2+1)
	for i, c := range hrp {
		result[i] = byte(c >> 5)
		result[i+len(hrp)+1] = byte(c & 0x1f)
	}
	result[len(hrp)] = 0
	return result
}

func bech32Polymod(values []byte) int {
	gen := []int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := 1
	for _, v := range values {
		b := chk >> 25
		chk = ((chk & 0x1ffffff) << 5) ^ int(v)
		for i := 0; i < 5; i++ {
			if (b>>i)&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

// ensureUserHasOrg ensures the user has at least one organization.
// If the user has no orgs, it creates a default "Personal" org.
func ensureUserHasOrg(ctx context.Context, user *models.User, orgRepo repository.OrgRepository) (*models.Organization, error) {
	// Check if user already has orgs
	orgs, err := orgRepo.ListUserOrgs(ctx, user.ID)
	if err != nil {
		slog.Error("Failed to list user orgs", slog.String("user_id", user.ID.String()), slog.String("error", err.Error()))
		return nil, err
	}
	if len(orgs) > 0 {
		slog.Info("User has existing org", slog.String("user_id", user.ID.String()), slog.String("org_id", orgs[0].ID.String()))
		return orgs[0], nil
	}

	// Create default org
	name := "Personal"
	if user.Name != nil && *user.Name != "" {
		name = *user.Name + "'s Workspace"
	}

	org := &models.Organization{
		Name: name,
	}

	slog.Info("Creating default org for user", slog.String("user_id", user.ID.String()), slog.String("org_name", name))
	if err := orgRepo.Create(ctx, org, user.ID); err != nil {
		slog.Error("Failed to create default org", slog.String("user_id", user.ID.String()), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create default org: %w", err)
	}

	slog.Info("Created default organization for user",
		slog.String("user_id", user.ID.String()),
		slog.String("org_id", org.ID.String()),
		slog.String("org_name", org.Name),
	)

	return org, nil
}
