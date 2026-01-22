// Package main is the entry point for the Control Plane API server.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	bootstraphandler "github.com/Bidon15/popsigner/control-plane/internal/bootstrap/handler"
	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/nitro"
	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/opstack"
	bootstraporchestrator "github.com/Bidon15/popsigner/control-plane/internal/bootstrap/orchestrator"
	"github.com/Bidon15/popsigner/control-plane/internal/bootstrap/popdeployer"
	bootstraprepo "github.com/Bidon15/popsigner/control-plane/internal/bootstrap/repository"
	"github.com/Bidon15/popsigner/control-plane/internal/config"
	"github.com/Bidon15/popsigner/control-plane/internal/database"
	"github.com/Bidon15/popsigner/control-plane/internal/handler"
	"github.com/Bidon15/popsigner/control-plane/internal/handler/jsonrpc"
	"github.com/Bidon15/popsigner/control-plane/internal/handler/popkins"
	"github.com/Bidon15/popsigner/control-plane/internal/middleware"
	"github.com/Bidon15/popsigner/control-plane/internal/models"
	"github.com/Bidon15/popsigner/control-plane/internal/openbao"
	"github.com/Bidon15/popsigner/control-plane/internal/pkg/response"
	"github.com/Bidon15/popsigner/control-plane/internal/repository"
	"github.com/Bidon15/popsigner/control-plane/internal/service"
	"github.com/Bidon15/popsigner/control-plane/templates/components"
	"github.com/Bidon15/popsigner/control-plane/templates/layouts"
	"github.com/Bidon15/popsigner/control-plane/templates/pages"
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
	apiKeyRepo := repository.NewAPIKeyRepository(db.Pool())
	auditRepo := repository.NewAuditRepository(db.Pool())
	usageRepo := repository.NewUsageRepository(db.Pool())
	certRepo := repository.NewCertificateRepository(db.Pool())

	// Initialize OpenBao client
	baoClient := openbao.NewClient(&cfg.OpenBao)
	logger.Info("OpenBao client initialized", slog.String("address", cfg.OpenBao.Address))

	// Initialize PKI adapter for certificate management (implements service.PKIInterface)
	pkiAdapter := openbao.NewPKIAdapter(baoClient)
	logger.Info("PKI client initialized")

	// Initialize PKI secrets engine and CA (for mTLS certificate issuance)
	ctx := context.Background()
	if err := pkiAdapter.EnsurePKIEnabled(ctx); err != nil {
		logger.Warn("Failed to ensure PKI secrets engine is enabled (may already exist or require manual setup)", slog.String("error", err.Error()))
	} else {
		logger.Info("PKI secrets engine enabled")
	}
	if _, err := pkiAdapter.InitializeCA(ctx); err != nil {
		logger.Warn("Failed to initialize PKI CA (may already exist)", slog.String("error", err.Error()))
	} else {
		logger.Info("PKI CA initialized")
	}

	// Initialize services
	oauthSvc := service.NewOAuthService(&cfg.Auth, userRepo, sessionRepo)
	keySvc := service.NewKeyService(keyRepo, orgRepo, auditRepo, usageRepo, baoClient)
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo)
	certSvc := service.NewCertificateService(certRepo, pkiAdapter, orgRepo, auditRepo)

	// Initialize API handlers
	keyHandler := handler.NewKeyHandler(keySvc)
	signHandler := handler.NewSignHandler(keySvc)

	// Initialize JSON-RPC server for Ethereum signing (used by orchestrator)
	jsonRPCServer := jsonrpc.NewServer(jsonrpc.ServerConfig{
		KeyRepo:   keyRepo,
		AuditRepo: auditRepo,
		UsageRepo: usageRepo,
		BaoClient: baoClient,
		Logger:    logger,
	})
	logger.Info("JSON-RPC server initialized")

	// Initialize bootstrap (deployment) repository
	bootstrapRepo := bootstraprepo.NewPostgresRepository(db.Pool())

	// Initialize OP Stack orchestrator for chain deployments
	opstackOrch := opstack.NewOrchestrator(
		bootstrapRepo,
		&opstack.DefaultSignerFactory{},
		opstack.NewEthClientFactory(),
		opstack.OrchestratorConfig{
			Logger: logger,
		},
	)
	logger.Info("OP Stack orchestrator initialized")

	// Initialize Nitro orchestrator for Orbit chain deployments
	// Uses CertificateServiceProvider to auto-issue mTLS certs for each deployment
	nitroCertProvider := nitro.NewCertificateServiceProvider(certSvc)

	// Determine POPSigner mTLS endpoint for Nitro deployments
	// mTLS endpoint for Nitro integration
	// Uses dedicated hostname with passthrough LoadBalancer for client cert verification
	nitroMTLSEndpoint := "https://rpc-mtls.popsigner.com"
	if cfg.Server.Environment == "dev" {
		// In dev k8s, use the mTLS LoadBalancer IP which is in the cert's SANs
		nitroMTLSEndpoint = "https://51.15.112.44:8546"
	}

	// Initialize Nitro infrastructure repository (stores deployed RollupCreator addresses)
	nitroInfraRepo := repository.NewNitroInfrastructureRepository(db.Pool())

	nitroOrch := nitro.NewOrchestrator(
		bootstrapRepo,
		nitroCertProvider,
		nitro.OrchestratorConfig{
			Logger:                logger,
			WorkerPath:            "internal/bootstrap/nitro/worker",
			POPSignerMTLSEndpoint: nitroMTLSEndpoint,
			NitroInfraRepo:        nitroInfraRepo,
		},
	)
	logger.Info("Nitro orchestrator initialized",
		slog.String("mtls_endpoint", nitroMTLSEndpoint),
		slog.Bool("infra_repo_enabled", true),
	)

	// Initialize POPKins Bundle orchestrator for devnet bundle creation
	popBundleOrch := popdeployer.NewOrchestrator(
		bootstrapRepo,
		popdeployer.OrchestratorConfig{
			Logger:   logger,
			CacheDir: "/tmp/popdeployer",
			WorkDir:  "/tmp/popdeployer/work",
		},
	)
	logger.Info("POPKins Bundle orchestrator initialized")

	// Initialize key resolver and API key manager for orchestrator
	keyResolver := bootstraporchestrator.NewKeyServiceResolver(keySvc)
	apiKeyManager := bootstraporchestrator.NewDefaultAPIKeyManager(apiKeySvc, baoClient, logger)

	// Determine POPSigner endpoint (for signing requests during deployment)
	// The orchestrator uses the JSON-RPC endpoint at /v1/rpc for eth_signTransaction
	// This is the same server but accessed via internal Kubernetes DNS
	signerEndpoint := fmt.Sprintf("http://localhost:%d/v1/rpc", cfg.Server.Port)
	if cfg.Server.Environment == "production" {
		// In production, use the internal service name
		signerEndpoint = "http://production-api:8080/v1/rpc"
	} else if cfg.Server.Environment == "dev" {
		// In dev cluster, use the internal service name (not external DNS)
		signerEndpoint = "http://dev-api:8080/v1/rpc"
	}

	// Initialize unified orchestrator that dispatches to stack-specific orchestrators
	unifiedOrch := bootstraporchestrator.New(
		bootstrapRepo,
		opstackOrch,
		nitroOrch,
		popBundleOrch,
		keyResolver,
		apiKeyManager,
		bootstraporchestrator.Config{
			Logger:         logger,
			SignerEndpoint: signerEndpoint,
		},
	)
	logger.Info("Unified orchestrator initialized", slog.String("signer_endpoint", signerEndpoint))

	// Initialize POPKins (chain deployment) handler
	// Uses same session store as main dashboard for SSO
	authSvc := service.NewAuthService(userRepo, sessionRepo, service.DefaultAuthServiceConfig())
	orgSvc := service.NewOrgService(orgRepo, userRepo, service.DefaultOrgServiceConfig())

	// Initialize bootstrap (deployment) handler with the orchestrator
	deploymentHandler := bootstraphandler.NewDeploymentHandler(bootstrapRepo, unifiedOrch, orgSvc)
	// POPKins uses same session mechanism as main dashboard (cookie + DB lookup)
	// Pass the unified orchestrator so deployments are started automatically
	popkinsHandler := popkins.NewHandler(authSvc, orgSvc, keySvc, bootstrapRepo, unifiedOrch, sessionRepo, userRepo)
	logger.Info("POPKins handler initialized")

	// Process any pending deployments from previous server runs
	go func() {
		time.Sleep(5 * time.Second) // Wait for server to be fully up
		if err := unifiedOrch.ProcessPendingDeployments(context.Background()); err != nil {
			logger.Error("Failed to process pending deployments", slog.String("error", err.Error()))
		}
	}()

	logger.Info("OAuth providers configured",
		slog.Any("providers", oauthSvc.GetSupportedProviders()),
	)

	// Setup router
	r := chi.NewRouter()

	// Global middleware stack
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Metrics()) // Prometheus metrics and visitor tracking
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.CORS())
	r.Use(chimiddleware.Timeout(30 * time.Second))

	// Custom 404 Not Found handler
	r.NotFound(notFoundHandler())

	// Custom Method Not Allowed handler
	r.MethodNotAllowed(methodNotAllowedHandler())

	// Health check endpoints (no auth required)
	r.Get("/health", healthHandler(db, redis))
	r.Get("/ready", readyHandler(db, redis))

	// Prometheus metrics endpoint (protect via ingress in production)
	r.Handle("/metrics", promhttp.Handler())

	// Public status page (TODO: implement)
	// r.Get("/status", statusPageHandler(db, redis, baoClient))

	// Static files for web dashboard
	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// Web dashboard landing page
	r.Get("/", landingPageHandler())
	r.Get("/login", loginPageHandler())
	r.Get("/signup", signupPageHandler())
	r.Get("/logout", logoutHandler(sessionRepo))
	r.Post("/logout", logoutHandler(sessionRepo))

	// Dashboard (protected by session check)
	r.Get("/dashboard", dashboardHandler(sessionRepo, userRepo, orgRepo, keyRepo, auditRepo, usageRepo))

	// Protected dashboard pages
	r.Get("/keys", keysListHandler(sessionRepo, userRepo, orgRepo, keyRepo))
	r.Post("/keys", keysCreateHandler(sessionRepo, userRepo, orgRepo, keySvc))
	r.Get("/keys/new", keysNewHandler(sessionRepo, userRepo))
	r.Get("/keys/{id}", keyViewHandler(sessionRepo, userRepo, keyRepo))
	r.Delete("/keys/{id}", keyDeleteHandler(sessionRepo, userRepo, orgRepo, keyRepo, keySvc))
	r.Post("/keys/{id}/sign-test", keySignHandler(sessionRepo, userRepo, keyRepo, keySvc))
	r.Get("/settings/api-keys", settingsAPIKeysHandler(sessionRepo, userRepo, orgRepo, apiKeyRepo))
	r.Get("/settings/api-keys/new", settingsAPIKeysNewHandler(sessionRepo, userRepo, orgRepo))
	r.Post("/settings/api-keys", settingsAPIKeysCreateHandler(sessionRepo, userRepo, orgRepo, apiKeySvc))
	r.Delete("/settings/api-keys/{id}", settingsAPIKeysDeleteHandler(sessionRepo, userRepo, orgRepo, apiKeySvc))
	r.Get("/settings/profile", settingsProfileHandler(sessionRepo, userRepo))

	// Certificate management routes
	r.Get("/settings/certificates", settingsCertificatesHandler(sessionRepo, userRepo, orgRepo, certSvc))
	r.Get("/settings/certificates/new", settingsCertificatesNewHandler(sessionRepo, userRepo, orgRepo))
	r.Get("/settings/certificates/ca", settingsCertificatesCAHandler(sessionRepo, userRepo, orgRepo, certSvc))
	r.Get("/settings/certificates/{id}/download", settingsCertificatesDownloadHandler(sessionRepo, userRepo, orgRepo, certSvc))
	r.Post("/settings/certificates", settingsCertificatesCreateHandler(sessionRepo, userRepo, orgRepo, certSvc))
	r.Post("/settings/certificates/{id}/revoke", settingsCertificatesRevokeHandler(sessionRepo, userRepo, orgRepo, certSvc))
	r.Delete("/settings/certificates/{id}", settingsCertificatesDeleteHandler(sessionRepo, userRepo, orgRepo, certSvc))

	r.Get("/docs", docsHandler(sessionRepo, userRepo))

	// Usage & Analytics
	r.Get("/usage", usageHandler(sessionRepo, userRepo, orgRepo, keyRepo, usageRepo))

	// Audit log
	r.Get("/audit", auditHandler(sessionRepo, userRepo, orgRepo, auditRepo))

	// Team management
	r.Get("/settings/team", settingsTeamHandler(sessionRepo, userRepo, orgRepo))

	// POPKins - Chain deployment platform (separate product)
	// In production: popkins.popsigner.com
	// For development/fallback: /popkins/* path on any host
	popkinsRouter := popkinsHandler.Routes()
	r.Mount("/popkins", popkinsRouter) // Fallback path-based access

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

		// Protected API routes (require API key authentication)
		r.Group(func(r chi.Router) {
			// API key authentication middleware
			r.Use(middleware.APIKeyAuth(apiKeySvc))
			// Track API usage for billing/analytics
			r.Use(middleware.TrackAPIUsage(usageRepo))

			// Keys API - CRUD and signing operations
			r.Mount("/keys", keyHandler.Routes())

			// Batch signing endpoint
			r.Mount("/sign", signHandler.Routes())

			// JSON-RPC endpoint for Ethereum signing (eth_signTransaction, eth_sign, personal_sign)
			r.Mount("/rpc", jsonRPCServer)

			// Deployments API - chain deployment management
			r.Mount("/deployments", deploymentHandler.Routes())

			// Namespaces API - list namespaces for the authenticated org
			r.Get("/namespaces", namespacesListHandler(orgRepo))
		})
	})

	// Create host-based router for subdomain routing
	// - popkins.popsigner.com → POPKins routes (served at /)
	// - dashboard.popsigner.com (or any other) → Main dashboard
	hostRouter := createHostRouter(r, popkinsRouter, logger)

	// Create server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      hostRouter,
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

// TODO: statusPageHandler is disabled until pages.StatusPage template is created
// func statusPageHandler(db *database.Postgres, redis *database.Redis, baoClient *openbao.Client) http.HandlerFunc { ... }

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
		formValues := pages.SignupFormValues{
			Name:  r.URL.Query().Get("name"),
			Email: r.URL.Query().Get("email"),
		}
		pages.SignupPage(errorMsg, formValues).Render(r.Context(), w)
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

		// Store return URL based on subdomain or query param
		// This allows POPKins users to return to POPKins after login
		returnTo := r.URL.Query().Get("return_to")
		if returnTo == "" {
			// Check if coming from POPKins subdomain
			host := strings.ToLower(r.Host)
			if strings.HasPrefix(host, "popkins.") {
				returnTo = "https://popkins.popsigner.com/deployments"
			} else {
				returnTo = "/dashboard"
			}
		}

		// Set cookie domain to share across all subdomains
		// This is needed because OAuth callback comes back to popsigner.com
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_return_to",
			Value:    returnTo,
			Path:     "/",
			Domain:   ".popsigner.com", // Share across all subdomains
			MaxAge:   300,              // 5 minutes
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

		// Set the session cookie (domain shared across all subdomains)
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    sessionID,
			Path:     "/",
			Domain:   ".popsigner.com", // Share session across all subdomains
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

		// Get return URL from cookie (set during OAuth redirect)
		returnTo := "/dashboard" // default for main dashboard
		if cookie, err := r.Cookie("oauth_return_to"); err == nil && cookie.Value != "" {
			returnTo = cookie.Value
		} else {
			// If no cookie, check Referer header for subdomain hint
			referer := r.Header.Get("Referer")
			if strings.Contains(referer, "popkins.") {
				returnTo = "https://popkins.popsigner.com/deployments"
			}
		}

		// Clear the return URL cookie (must match domain used when setting)
		http.SetCookie(w, &http.Cookie{
			Name:   "oauth_return_to",
			Value:  "",
			Path:   "/",
			Domain: ".popsigner.com",
			MaxAge: -1,
		})

		// Redirect to the appropriate destination
		http.Redirect(w, r, returnTo, http.StatusFound)
	}
}

// dashboardHandler serves the dashboard page for authenticated users.
func dashboardHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, keyRepo repository.KeyRepository, auditRepo repository.AuditRepository, usageRepo repository.UsageRepository) http.HandlerFunc {
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
				Domain: ".popsigner.com",
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
				Domain: ".popsigner.com",
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

		// Get user's organization and key count
		keyCount := 0
		signatureLimit := 1000 // Default free tier
		var orgID uuid.UUID
		orgs, err := orgRepo.ListUserOrgs(r.Context(), user.ID)
		if err == nil && len(orgs) > 0 {
			orgID = orgs[0].ID
			// Get keys for the first org
			keys, err := keyRepo.ListByOrg(r.Context(), orgID)
			if err == nil {
				keyCount = len(keys)
			}
			// Get plan limits
			limits := models.PlanLimitsMap[orgs[0].Plan]
			signatureLimit = int(limits.SignaturesPerMonth)
		}

		// Fetch API call count from usage metrics
		apiCallCount := int64(0)
		if orgID != uuid.Nil && usageRepo != nil {
			if count, err := usageRepo.GetCurrentPeriod(r.Context(), orgID, "api_calls"); err == nil {
				apiCallCount = count
			}
		}

		// Fetch signature count from usage metrics
		signatureCount := int64(0)
		if orgID != uuid.Nil && usageRepo != nil {
			if count, err := usageRepo.GetCurrentPeriod(r.Context(), orgID, "signatures"); err == nil {
				signatureCount = count
			}
		}

		// Render dashboard with data
		data := pages.DashboardData{
			User:           user,
			KeyCount:       keyCount,
			SignatureCount: int(signatureCount),
			APICallCount:   int(apiCallCount),
			SignatureLimit: signatureLimit,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.DashboardPage(data).Render(r.Context(), w)
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
			Domain: ".popsigner.com",
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
			Domain: ".popsigner.com",
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
		networkType := r.FormValue("network_type")
		exportable := r.FormValue("exportable") == "true"

		// Default network type to "all" if not specified
		if networkType == "" {
			networkType = "all"
		}

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
			NetworkType: networkType,
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

		// Get EVM address if available
		ethAddr := ""
		if key.EthAddress != nil {
			ethAddr = *key.EthAddress
		}

		data := pages.KeyDetailData{
			UserName:        dashData.UserName,
			UserEmail:       dashData.UserEmail,
			AvatarURL:       dashData.AvatarURL,
			OrgName:         dashData.OrgName,
			OrgPlan:         dashData.OrgPlan,
			Key:             key,
			Namespace:       "default", // TODO: fetch actual namespace name
			CelestiaAddress: celestiaAddr,
			EthAddress:      ethAddr,
			NetworkType:     string(key.NetworkType),
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

// keyDeleteHandler handles deleting a key.
func keyDeleteHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, keyRepo repository.KeyRepository, keySvc service.KeyService) http.HandlerFunc {
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

		// Delete the key via KeyService
		if err := keySvc.Delete(r.Context(), key.OrgID, key.ID); err != nil {
			slog.Error("Failed to delete key", slog.String("error", err.Error()))
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(fmt.Sprintf(`<div class="fixed bottom-4 right-4 z-50 flex items-center gap-3 px-4 py-3 bg-black border border-[#FF3333] text-[#FF3333] min-w-[200px] font-mono uppercase" x-data="{ show: true }" x-show="show" x-init="setTimeout(() => { show = false; setTimeout(() => $el.remove(), 200) }, 5000)" x-transition><span>✗</span><span class="text-sm font-medium">%s</span></div>`, err.Error())))
			return
		}

		slog.Info("Key deleted successfully",
			slog.String("user_id", user.ID.String()),
			slog.String("key_id", keyID),
		)

		// Get org for the keys list
		org, _ := ensureUserHasOrg(r.Context(), user, orgRepo)

		dashData := buildDashboardData(user, "/keys")
		if org != nil {
			dashData.OrgName = org.Name
			dashData.OrgPlan = string(org.Plan)
		}

		// Fetch updated keys and namespaces
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
		pages.KeysPageContent(data).Render(r.Context(), w)
	}
}

// settingsAPIKeysHandler serves the API keys settings page.
func settingsAPIKeysHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, apiKeyRepo repository.APIKeyRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Ensure user has an org
		org, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil || org == nil {
			slog.Error("Failed to get/create org for API keys page", slog.String("error", err.Error()))
			http.Error(w, "Failed to get organization", http.StatusInternalServerError)
			return
		}

		// Fetch API keys for the organization
		apiKeys, err := apiKeyRepo.ListByOrg(r.Context(), org.ID)
		if err != nil {
			slog.Error("Failed to list API keys",
				slog.String("error", err.Error()),
				slog.String("org_id", org.ID.String()),
			)
			apiKeys = []*models.APIKey{}
		} else {
			slog.Info("Fetched API keys",
				slog.String("org_id", org.ID.String()),
				slog.Int("count", len(apiKeys)),
			)
		}

		dashData := buildDashboardData(user, "/settings/api-keys")
		dashData.OrgName = org.Name
		dashData.OrgPlan = string(org.Plan)

		data := pages.APIKeysPageData{
			DashboardData: layouts.DashboardData{
				UserName:   dashData.UserName,
				UserEmail:  dashData.UserEmail,
				AvatarURL:  dashData.AvatarURL,
				OrgName:    dashData.OrgName,
				OrgPlan:    dashData.OrgPlan,
				ActivePath: dashData.ActivePath,
			},
			APIKeys:   apiKeys,
			CanCreate: true,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.SettingsAPIKeysPage(data).Render(r.Context(), w)
	}
}

// settingsAPIKeysNewHandler returns the create API key modal.
func settingsAPIKeysNewHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Verify user has an org
		_, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil {
			http.Error(w, "Session expired or no organization", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.CreateAPIKeyModal().Render(r.Context(), w)
	}
}

// settingsAPIKeysCreateHandler handles creating a new API key.
func settingsAPIKeysCreateHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, apiKeySvc service.APIKeyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Ensure user has an org
		org, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil || org == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			pages.APIKeyCreateError("No organization found. Please refresh and try again.").Render(r.Context(), w)
			return
		}

		// Parse form
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			pages.APIKeyCreateError("Failed to parse form").Render(r.Context(), w)
			return
		}

		name := r.FormValue("name")
		if name == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			pages.APIKeyCreateError("Name is required").Render(r.Context(), w)
			return
		}

		scopes := r.Form["scopes"]
		if len(scopes) == 0 {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			pages.APIKeyCreateError("Please select at least one permission").Render(r.Context(), w)
			return
		}

		// Create the API key
		req := service.CreateAPIKeyRequest{
			Name:   name,
			Scopes: scopes,
		}
		apiKey, rawKey, err := apiKeySvc.Create(r.Context(), org.ID, req)
		if err != nil {
			slog.Error("Failed to create API key", slog.String("error", err.Error()))
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			pages.APIKeyCreateError("Failed to create API key: "+err.Error()).Render(r.Context(), w)
			return
		}

		slog.Info("API key created",
			slog.String("user_id", user.ID.String()),
			slog.String("org_id", org.ID.String()),
			slog.String("api_key_id", apiKey.ID.String()),
		)

		// Show the raw key (only shown once)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.APIKeyCreatedSuccess(rawKey, apiKey.KeyPrefix).Render(r.Context(), w)
	}
}

// settingsAPIKeysDeleteHandler handles revoking an API key.
func settingsAPIKeysDeleteHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, apiKeySvc service.APIKeyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Ensure user has an org
		org, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil || org == nil {
			http.Error(w, "No organization found", http.StatusBadRequest)
			return
		}

		keyID := chi.URLParam(r, "id")
		if keyID == "" {
			http.Error(w, "API key ID required", http.StatusBadRequest)
			return
		}

		keyUUID, err := uuid.Parse(keyID)
		if err != nil {
			http.Error(w, "Invalid API key ID", http.StatusBadRequest)
			return
		}

		// Revoke the API key
		if err := apiKeySvc.Revoke(r.Context(), org.ID, keyUUID); err != nil {
			slog.Error("Failed to revoke API key", slog.String("error", err.Error()))
			http.Error(w, "Failed to revoke API key", http.StatusInternalServerError)
			return
		}

		slog.Info("API key revoked",
			slog.String("user_id", user.ID.String()),
			slog.String("api_key_id", keyID),
		)

		// Return updated list via HTMX
		w.Header().Set("HX-Trigger", "api-key-deleted")
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
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

// docsHandler serves the in-app documentation page.
func docsHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		dashData := buildDashboardData(user, "/docs")
		data := pages.DocsPageData{
			UserName:      dashData.UserName,
			UserEmail:     dashData.UserEmail,
			AvatarURL:     dashData.AvatarURL,
			OrgName:       dashData.OrgName,
			OrgPlan:       dashData.OrgPlan,
			ActiveSection: "overview",
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.DocsPage(data).Render(r.Context(), w)
	}
}

// settingsCertificatesHandler serves the certificates settings page.
func settingsCertificatesHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, certSvc service.CertificateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Ensure user has an org
		org, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil || org == nil {
			slog.Error("Failed to get/create org for certificates page", slog.String("error", err.Error()))
			http.Error(w, "Failed to get organization", http.StatusInternalServerError)
			return
		}

		// Fetch certificates for the organization
		var certs []models.Certificate
		certList, err := certSvc.List(r.Context(), org.ID.String(), repository.CertificateFilterAll)
		if err != nil {
			slog.Error("Failed to list certificates",
				slog.String("error", err.Error()),
				slog.String("org_id", org.ID.String()),
			)
		} else if certList != nil {
			certs = certList.Certificates
		}

		dashData := buildDashboardData(user, "/settings/certificates")
		dashData.OrgName = org.Name
		dashData.OrgPlan = string(org.Plan)

		data := pages.CertificatesPageData{
			DashboardData: layouts.DashboardData{
				UserName:   dashData.UserName,
				UserEmail:  dashData.UserEmail,
				AvatarURL:  dashData.AvatarURL,
				OrgName:    dashData.OrgName,
				OrgPlan:    dashData.OrgPlan,
				ActivePath: dashData.ActivePath,
			},
			Certificates: certs,
			Total:        len(certs),
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.CertificatesPage(data).Render(r.Context(), w)
	}
}

// settingsCertificatesNewHandler returns the create certificate modal.
func settingsCertificatesNewHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Verify user has an org
		_, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil {
			http.Error(w, "Session expired or no organization", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.CreateCertificateModal().Render(r.Context(), w)
	}
}

// settingsCertificatesCreateHandler handles creating a new certificate.
func settingsCertificatesCreateHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, certSvc service.CertificateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Ensure user has an org
		org, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil || org == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			pages.CertificateCreateError("No organization found. Please refresh and try again.").Render(r.Context(), w)
			return
		}

		// Parse form
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			pages.CertificateCreateError("Failed to parse form").Render(r.Context(), w)
			return
		}

		name := r.FormValue("name")
		if name == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			pages.CertificateCreateError("Certificate name is required").Render(r.Context(), w)
			return
		}

		// Parse validity period
		validityPeriod := r.FormValue("validity_period")
		duration := models.DefaultValidityPeriod
		if validityPeriod != "" {
			d, err := time.ParseDuration(validityPeriod)
			if err == nil {
				duration = d
			}
		}

		// Create the certificate
		req := &models.CreateCertificateRequest{
			OrgID:          org.ID,
			Name:           name,
			ValidityPeriod: duration,
		}
		bundle, err := certSvc.Issue(r.Context(), req)
		if err != nil {
			slog.Error("Failed to create certificate", slog.String("error", err.Error()))
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			pages.CertificateCreateError("Failed to create certificate: "+err.Error()).Render(r.Context(), w)
			return
		}

		slog.Info("Certificate created",
			slog.String("user_id", user.ID.String()),
			slog.String("org_id", org.ID.String()),
			slog.String("fingerprint", bundle.Fingerprint),
		)

		// Return the certificate bundle modal for download
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		components.CertDownloadModal(bundle).Render(r.Context(), w)
	}
}

// settingsCertificatesRevokeHandler handles revoking a certificate.
func settingsCertificatesRevokeHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, certSvc service.CertificateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Ensure user has an org
		org, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil || org == nil {
			http.Error(w, "No organization found", http.StatusBadRequest)
			return
		}

		certID := chi.URLParam(r, "id")
		if certID == "" {
			http.Error(w, "Certificate ID required", http.StatusBadRequest)
			return
		}

		// Revoke the certificate
		if err := certSvc.Revoke(r.Context(), org.ID.String(), certID, "User requested revocation"); err != nil {
			slog.Error("Failed to revoke certificate", slog.String("error", err.Error()))
			http.Error(w, "Failed to revoke certificate", http.StatusInternalServerError)
			return
		}

		slog.Info("Certificate revoked",
			slog.String("user_id", user.ID.String()),
			slog.String("cert_id", certID),
		)

		// Return updated list via HTMX
		certList, _ := certSvc.List(r.Context(), org.ID.String(), repository.CertificateFilterAll)
		var certs []models.Certificate
		if certList != nil {
			certs = certList.Certificates
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.CertificatesList(certs).Render(r.Context(), w)
	}
}

// settingsCertificatesDeleteHandler handles deleting a certificate.
func settingsCertificatesDeleteHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, certSvc service.CertificateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Ensure user has an org
		org, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil || org == nil {
			http.Error(w, "No organization found", http.StatusBadRequest)
			return
		}

		certID := chi.URLParam(r, "id")
		if certID == "" {
			http.Error(w, "Certificate ID required", http.StatusBadRequest)
			return
		}

		// Delete the certificate (must be revoked first)
		if err := certSvc.Delete(r.Context(), org.ID.String(), certID); err != nil {
			slog.Error("Failed to delete certificate", slog.String("error", err.Error()))
			http.Error(w, "Failed to delete certificate: "+err.Error(), http.StatusInternalServerError)
			return
		}

		slog.Info("Certificate deleted",
			slog.String("user_id", user.ID.String()),
			slog.String("cert_id", certID),
		)

		// Return updated list via HTMX
		certList, _ := certSvc.List(r.Context(), org.ID.String(), repository.CertificateFilterAll)
		var certs []models.Certificate
		if certList != nil {
			certs = certList.Certificates
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.CertificatesList(certs).Render(r.Context(), w)
	}
}

// settingsCertificatesCAHandler serves the CA certificate for download.
func settingsCertificatesCAHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, certSvc service.CertificateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		caCert, err := certSvc.GetCACertificate(r.Context())
		if err != nil {
			slog.Error("Failed to get CA certificate", slog.String("error", err.Error()))
			http.Error(w, "Failed to retrieve CA certificate", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Header().Set("Content-Disposition", "attachment; filename=popsigner-ca.crt")
		w.Write(caCert)
	}
}

// settingsCertificatesDownloadHandler serves a client certificate for download.
func settingsCertificatesDownloadHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, certSvc service.CertificateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		org, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil || org == nil {
			http.Error(w, "Failed to get organization", http.StatusInternalServerError)
			return
		}

		certID := chi.URLParam(r, "id")
		if certID == "" {
			http.Error(w, "Certificate ID required", http.StatusBadRequest)
			return
		}

		bundle, err := certSvc.DownloadBundle(r.Context(), org.ID.String(), certID)
		if err != nil {
			slog.Error("Failed to get certificate bundle", slog.String("error", err.Error()), slog.String("cert_id", certID))
			http.Error(w, "Certificate not found: "+err.Error(), http.StatusNotFound)
			return
		}

		if len(bundle.ClientCert) == 0 {
			http.Error(w, "Certificate data not available (legacy certificate)", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Header().Set("Content-Disposition", "attachment; filename=client.crt")
		w.Write(bundle.ClientCert)
	}
}

// deriveCelestiaAddress converts a hex address (from OpenBao) to Celestia bech32 format.
// The hex address is RIPEMD160(SHA256(compressed_pubkey)) - 20 bytes.
func deriveCelestiaAddress(hexAddr string) string {
	// Handle old-style addresses (bao_xxxxx format)
	if len(hexAddr) < 40 || hexAddr[:4] == "bao_" {
		return "(legacy key - please create a new key for Celestia)"
	}

	// Decode hex address to bytes
	addrBytes, err := hex.DecodeString(hexAddr)
	if err != nil || len(addrBytes) != 20 {
		// Fallback for invalid addresses
		return "(invalid address format)"
	}

	// Convert to bech32 with "celestia" prefix
	celestiaAddr, err := bech32Encode("celestia", addrBytes)
	if err != nil {
		return "(bech32 encoding failed)"
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

// namespacesListHandler returns the namespaces for the authenticated organization.
// This is used by the API to get namespace IDs for key creation.
func namespacesListHandler(orgRepo repository.OrgRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := middleware.GetOrgIDFromContext(r.Context())
		if orgID == uuid.Nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}

		namespaces, err := orgRepo.ListNamespaces(r.Context(), orgID)
		if err != nil {
			slog.Error("Failed to list namespaces", slog.String("error", err.Error()))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"failed to list namespaces"}`))
			return
		}

		// Return namespaces as JSON
		type namespaceResponse struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
		}

		result := make([]namespaceResponse, len(namespaces))
		for i, ns := range namespaces {
			desc := ""
			if ns.Description != nil {
				desc = *ns.Description
			}
			result[i] = namespaceResponse{
				ID:          ns.ID.String(),
				Name:        ns.Name,
				Description: desc,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(result); err != nil {
			slog.Error("Failed to encode namespaces", slog.String("error", err.Error()))
		}
	}
}

// usageHandler serves the usage analytics page.
func usageHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, keyRepo repository.KeyRepository, usageRepo repository.UsageRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Ensure user has an org
		org, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil || org == nil {
			http.Error(w, "Failed to get organization", http.StatusInternalServerError)
			return
		}

		dashData := buildDashboardData(user, "/usage")
		dashData.OrgName = org.Name
		dashData.OrgPlan = string(org.Plan)

		// Get plan limits
		limits := models.GetPlanLimits(org.Plan)

		// Calculate current billing period
		now := time.Now()
		periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Second)

		// Get key count
		keys, _ := keyRepo.ListByOrg(r.Context(), org.ID)
		keyCount := int64(len(keys))

		// Get usage metrics
		signatureCount := int64(0)
		apiCallCount := int64(0)
		if usageRepo != nil {
			if count, err := usageRepo.GetCurrentPeriod(r.Context(), org.ID, "signatures"); err == nil {
				signatureCount = count
			}
			if count, err := usageRepo.GetCurrentPeriod(r.Context(), org.ID, "api_calls"); err == nil {
				apiCallCount = count
			}
		}

		// Generate placeholder data for charts
		var sigData []pages.UsageDataPoint
		current := periodStart
		for current.Before(now) {
			sigData = append(sigData, pages.UsageDataPoint{Date: current, Value: 0})
			current = current.AddDate(0, 0, 1)
		}

		data := pages.UsagePageData{
			DashboardData: layouts.DashboardData{
				UserName:   dashData.UserName,
				UserEmail:  dashData.UserEmail,
				AvatarURL:  dashData.AvatarURL,
				OrgName:    dashData.OrgName,
				OrgPlan:    dashData.OrgPlan,
				ActivePath: "/usage",
			},
			Signatures:       signatureCount,
			SignaturesLimit:  limits.SignaturesPerMonth,
			Keys:             keyCount,
			KeysLimit:        int64(limits.Keys),
			APICalls:         apiCallCount,
			TeamMembers:      1, // Default to 1 for the owner
			TeamMembersLimit: int64(limits.TeamMembers),
			SignaturesData:   sigData,
			APICallsData:     sigData,
			PeriodStart:      periodStart,
			PeriodEnd:        periodEnd,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.UsagePage(data).Render(r.Context(), w)
	}
}

// auditHandler serves the audit log page.
func auditHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository, auditRepo repository.AuditRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Ensure user has an org
		org, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil || org == nil {
			http.Error(w, "Failed to get organization", http.StatusInternalServerError)
			return
		}

		dashData := buildDashboardData(user, "/audit")
		dashData.OrgName = org.Name
		dashData.OrgPlan = string(org.Plan)

		// Parse filters from query params
		event := r.URL.Query().Get("event")
		period := r.URL.Query().Get("period")
		actor := r.URL.Query().Get("actor")

		// Calculate time range based on period
		var startTime *time.Time
		now := time.Now()
		switch period {
		case "30d":
			t := now.AddDate(0, 0, -30)
			startTime = &t
		case "90d":
			t := now.AddDate(0, 0, -90)
			startTime = &t
		default:
			t := now.AddDate(0, 0, -7)
			startTime = &t
			period = "7d"
		}

		// Build query
		query := models.AuditLogQuery{
			OrgID:     org.ID,
			StartTime: startTime,
			Limit:     50,
		}

		if event != "" {
			e := models.AuditEvent(event)
			query.Event = &e
		}

		// Get audit logs
		var logs []*models.AuditLog
		nextCursor := ""
		if auditRepo != nil {
			logs, _ = auditRepo.List(r.Context(), query)
		}

		filters := pages.AuditFilters{
			Event:  event,
			Period: period,
			Actor:  actor,
		}

		data := pages.AuditPageData{
			DashboardData: layouts.DashboardData{
				UserName:   dashData.UserName,
				UserEmail:  dashData.UserEmail,
				AvatarURL:  dashData.AvatarURL,
				OrgName:    dashData.OrgName,
				OrgPlan:    dashData.OrgPlan,
				ActivePath: "/audit",
			},
			Logs:       logs,
			NextCursor: nextCursor,
			Filters:    filters,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.AuditPage(data).Render(r.Context(), w)
	}
}

// notFoundHandler returns a custom 404 handler with themed page.
func notFoundHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if this is an API request
		if isAPIRequest(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not_found","message":"The requested resource was not found"}`))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		pages.Error404Page().Render(r.Context(), w)
	}
}

// methodNotAllowedHandler returns a custom 405 handler.
func methodNotAllowedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isAPIRequest(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"error":"method_not_allowed","message":"The requested method is not allowed for this resource"}`))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		pages.Error404Page().Render(r.Context(), w)
	}
}

// serviceUnavailableHandler returns a custom 503 handler with themed page.
func serviceUnavailableHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isAPIRequest(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"service_unavailable","message":"The service is temporarily unavailable"}`))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		pages.Error503Page().Render(r.Context(), w)
	}
}

// isAPIRequest checks if the request is for an API endpoint.
func isAPIRequest(r *http.Request) bool {
	// Check Accept header
	accept := r.Header.Get("Accept")
	if accept == "application/json" {
		return true
	}

	// Check if path starts with /v1 (API routes)
	if len(r.URL.Path) >= 3 && r.URL.Path[:3] == "/v1" {
		return true
	}

	// Check Content-Type for POST/PUT requests
	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" {
		return true
	}

	return false
}

// createHostRouter creates a hostname-based router that dispatches requests
// to different handlers based on the subdomain.
//
// Routing rules:
//   - popkins.popsigner.com → POPKins routes (chain deployment platform)
//   - popkins.localhost:* → POPKins routes (local development)
//   - Everything else → Main dashboard (dashboard.popsigner.com or default)
//
// Shared resources like /static/* and /health are served from the main router
// regardless of hostname.
func createHostRouter(dashboardRouter http.Handler, popkinsRouter http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract hostname (remove port if present)
		host := r.Host
		if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
			// Check if it's not an IPv6 address
			if bracketIdx := strings.LastIndex(host, "]"); bracketIdx == -1 || colonIdx > bracketIdx {
				host = host[:colonIdx]
			}
		}
		host = strings.ToLower(host)

		// Always serve shared resources from main router regardless of hostname:
		// - Static files (CSS, JS, images)
		// - Health/ready/metrics endpoints
		// - Authentication routes (OAuth callbacks need to work on any subdomain)
		path := r.URL.Path
		if strings.HasPrefix(path, "/static/") ||
			strings.HasPrefix(path, "/auth/") ||
			path == "/login" ||
			path == "/logout" ||
			path == "/health" ||
			path == "/ready" ||
			path == "/metrics" {
			dashboardRouter.ServeHTTP(w, r)
			return
		}

		// Route based on hostname
		switch {
		case strings.HasPrefix(host, "popkins."):
			// POPKins subdomain - serve POPKins routes at root
			logger.Debug("Routing to POPKins",
				slog.String("host", r.Host),
				slog.String("path", r.URL.Path),
			)
			popkinsRouter.ServeHTTP(w, r)

		default:
			// Default to main dashboard (dashboard.popsigner.com, localhost, etc.)
			dashboardRouter.ServeHTTP(w, r)
		}
	})
}

// settingsTeamHandler serves the team settings page.
func settingsTeamHandler(sessionRepo repository.SessionRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getAuthenticatedUser(w, r, sessionRepo, userRepo)
		if user == nil {
			return
		}

		// Ensure user has an org
		org, err := ensureUserHasOrg(r.Context(), user, orgRepo)
		if err != nil || org == nil {
			http.Error(w, "Failed to get organization", http.StatusInternalServerError)
			return
		}

		dashData := buildDashboardData(user, "/settings/team")
		dashData.OrgName = org.Name
		dashData.OrgPlan = string(org.Plan)

		// Get plan limits
		limits := models.GetPlanLimits(org.Plan)

		// Get current user as the only member (simplified - full team management requires OrgService)
		userName := ""
		if user.Name != nil {
			userName = *user.Name
		}
		avatarURL := ""
		if user.AvatarURL != nil {
			avatarURL = *user.AvatarURL
		}

		members := []*pages.TeamMemberDisplay{
			{
				ID:            user.ID,
				Name:          userName,
				Email:         user.Email,
				AvatarURL:     avatarURL,
				Role:          models.RoleOwner,
				JoinedAt:      user.CreatedAt.Format("Jan 2, 2006"),
				IsCurrentUser: true,
			},
		}

		data := pages.TeamPageData{
			DashboardData: layouts.DashboardData{
				UserName:   dashData.UserName,
				UserEmail:  dashData.UserEmail,
				AvatarURL:  dashData.AvatarURL,
				OrgName:    dashData.OrgName,
				OrgPlan:    dashData.OrgPlan,
				ActivePath: "/settings/team",
			},
			Members:     members,
			Invitations: nil,
			CurrentRole: models.RoleOwner,
			MemberLimit: limits.TeamMembers,
			MemberCount: 1,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		pages.SettingsTeamPage(data).Render(r.Context(), w)
	}
}
