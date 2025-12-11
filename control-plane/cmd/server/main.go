// Package main is the entry point for the Control Plane API server.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/Bidon15/banhbaoring/control-plane/internal/config"
	"github.com/Bidon15/banhbaoring/control-plane/internal/database"
	"github.com/Bidon15/banhbaoring/control-plane/internal/middleware"
	"github.com/Bidon15/banhbaoring/control-plane/internal/pkg/response"
	"github.com/Bidon15/banhbaoring/control-plane/templates/pages"
)

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

	// OAuth routes
	r.Get("/auth/github", oauthGitHubHandler(cfg))
	r.Get("/auth/github/callback", oauthGitHubCallbackHandler(cfg))
	r.Get("/auth/google", oauthGoogleHandler(cfg))
	r.Get("/auth/google/callback", oauthGoogleCallbackHandler(cfg))

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

			// Billing (Agent 10B)
			// r.Mount("/billing", billingHandler.Routes())
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

// oauthGitHubHandler redirects to GitHub for OAuth.
func oauthGitHubHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID := cfg.Auth.OAuthGitHubID
		if clientID == "" {
			http.Error(w, "GitHub OAuth not configured", http.StatusServiceUnavailable)
			return
		}
		redirectURI := cfg.Auth.OAuthCallbackURL + "/auth/github/callback"
		authURL := fmt.Sprintf(
			"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=user:email",
			clientID, redirectURI,
		)
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// oauthGitHubCallbackHandler handles GitHub OAuth callback.
func oauthGitHubCallbackHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Redirect(w, r, "/login?error=No+authorization+code", http.StatusFound)
			return
		}
		// TODO: Exchange code for token and create session
		// For now, show success message
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html><head><title>GitHub Login</title></head>
<body style="background:#0f172a;color:#f1f5f9;font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;">
<div style="text-align:center;">
<h1>✅ GitHub OAuth Working!</h1>
<p>Authorization code received: ` + code[:8] + `...</p>
<p>Full OAuth flow will be implemented with session management.</p>
<a href="/" style="color:#fbbf24;">← Back to Home</a>
</div></body></html>`))
	}
}

// oauthGoogleHandler redirects to Google for OAuth.
func oauthGoogleHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID := cfg.Auth.OAuthGoogleID
		if clientID == "" {
			http.Error(w, "Google OAuth not configured", http.StatusServiceUnavailable)
			return
		}
		redirectURI := cfg.Auth.OAuthCallbackURL + "/auth/google/callback"
		authURL := fmt.Sprintf(
			"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=email%%20profile",
			clientID, redirectURI,
		)
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// oauthGoogleCallbackHandler handles Google OAuth callback.
func oauthGoogleCallbackHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Redirect(w, r, "/login?error=No+authorization+code", http.StatusFound)
			return
		}
		// TODO: Exchange code for token and create session
		// For now, show success message
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html><head><title>Google Login</title></head>
<body style="background:#0f172a;color:#f1f5f9;font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;">
<div style="text-align:center;">
<h1>✅ Google OAuth Working!</h1>
<p>Authorization code received: ` + code[:8] + `...</p>
<p>Full OAuth flow will be implemented with session management.</p>
<a href="/" style="color:#fbbf24;">← Back to Home</a>
</div></body></html>`))
	}
}

// loginPageHandler serves the login page.
func loginPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Login - BanhBaoRing</title>
	<link rel="preconnect" href="https://fonts.bunny.net"/>
	<link href="https://fonts.bunny.net/css?family=outfit:400,500,600,700" rel="stylesheet"/>
</head>
<body class="bg-slate-900 text-slate-100 min-h-screen flex items-center justify-center" style="font-family: 'Outfit', sans-serif;">
	<div class="w-full max-w-md p-8">
		<div class="text-center mb-8">
			<a href="/" class="text-4xl font-bold bg-gradient-to-r from-amber-400 to-rose-500 bg-clip-text text-transparent">🔔 BanhBaoRing</a>
			<p class="text-slate-400 mt-2">Sign in to continue</p>
		</div>
		<div class="bg-slate-800/50 rounded-2xl border border-slate-700 p-8">
			<div class="space-y-4">
				<a href="/auth/github" class="flex items-center justify-center gap-3 w-full py-3 px-4 bg-slate-700 hover:bg-slate-600 rounded-xl transition-colors">
					<svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
					Continue with GitHub
				</a>
				<a href="/auth/google" class="flex items-center justify-center gap-3 w-full py-3 px-4 bg-slate-700 hover:bg-slate-600 rounded-xl transition-colors">
					<svg class="w-5 h-5" viewBox="0 0 24 24"><path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/><path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/><path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/><path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/></svg>
					Continue with Google
				</a>
			</div>
		</div>
		<p class="text-center text-slate-500 text-sm mt-6">
			By signing in, you agree to our Terms of Service and Privacy Policy
		</p>
	</div>
</body>
</html>`))
	}
}

