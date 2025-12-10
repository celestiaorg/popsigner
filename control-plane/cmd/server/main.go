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

