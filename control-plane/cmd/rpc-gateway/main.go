// Package main is the entry point for the JSON-RPC Gateway service.
// This standalone service handles Ethereum JSON-RPC signing requests
// for OP Stack integration (via API key) and Arbitrum Nitro integration (via mTLS),
// running separately from the main control plane.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Bidon15/popsigner/control-plane/cmd/rpc-gateway/internal/auth"
	"github.com/Bidon15/popsigner/control-plane/internal/config"
	"github.com/Bidon15/popsigner/control-plane/internal/database"
	"github.com/Bidon15/popsigner/control-plane/internal/handler/jsonrpc"
	"github.com/Bidon15/popsigner/control-plane/internal/middleware"
	"github.com/Bidon15/popsigner/control-plane/internal/openbao"
	"github.com/Bidon15/popsigner/control-plane/internal/repository"
	"github.com/Bidon15/popsigner/control-plane/internal/service"
)

const (
	serviceName       = "popsigner-rpc-gateway"
	defaultPort       = 8545
	defaultMTLSPort   = 8546
	defaultTimeout    = 30 * time.Second
	defaultCACertPath = "/etc/popsigner/ca/ca.crt"
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

	// Load port configurations
	apiKeyPort := getEnvInt("POPSIGNER_RPC_GATEWAY_PORT", defaultPort)
	mtlsPort := getEnvInt("POPSIGNER_MTLS_PORT", defaultMTLSPort)
	mtlsEnabled := os.Getenv("POPSIGNER_MTLS_ENABLED") == "true"

	logger.Info("Starting RPC Gateway",
		slog.String("service", serviceName),
		slog.Int("api_key_port", apiKeyPort),
		slog.Int("mtls_port", mtlsPort),
		slog.Bool("mtls_enabled", mtlsEnabled),
	)

	// Connect to PostgreSQL
	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	logger.Info("Connected to PostgreSQL")

	// Connect to Redis (for rate limiting)
	redis, err := database.NewRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()
	logger.Info("Connected to Redis")

	// Initialize OpenBao client
	baoClient := openbao.NewClient(&cfg.OpenBao)
	logger.Info("OpenBao client initialized", slog.String("address", cfg.OpenBao.Address))

	// Initialize repositories
	keyRepo := repository.NewKeyRepository(db.Pool())
	apiKeyRepo := repository.NewAPIKeyRepository(db.Pool())
	certRepo := repository.NewCertificateRepository(db.Pool())
	auditRepo := repository.NewAuditRepository(db.Pool())
	usageRepo := repository.NewUsageRepository(db.Pool())

	// Initialize services
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo)

	// Create JSON-RPC server
	rpcServer := jsonrpc.NewServer(jsonrpc.ServerConfig{
		KeyRepo:   keyRepo,
		AuditRepo: auditRepo,
		UsageRepo: usageRepo,
		BaoClient: baoClient,
		Logger:    logger,
	})

	// Rate limit config (shared between servers)
	rateLimitCfg := middleware.RPCRateLimitConfig{
		RequestsPerSecond: getEnvInt("POPSIGNER_RPC_RATE_LIMIT_RPS", 100),
		BurstSize:         getEnvInt("POPSIGNER_RPC_RATE_LIMIT_BURST", 200),
	}

	// ===========================================
	// Server 1: API Key authentication (Port 8545)
	// For OP Stack and general clients
	// ===========================================
	apiKeyRouter := createAPIKeyRouter(apiKeySvc, redis, rpcServer, rateLimitCfg, usageRepo, db, logger)

	apiKeySrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", apiKeyPort),
		Handler:      apiKeyRouter,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  time.Minute,
	}

	// Start API Key server
	go func() {
		logger.Info("API Key server listening",
			slog.Int("port", apiKeyPort),
			slog.String("auth_method", "api_key"),
		)
		if err := apiKeySrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API Key server error: %v", err)
		}
	}()

	// ===========================================
	// Server 2: mTLS authentication (Port 8546)
	// For Arbitrum Nitro integration
	// ===========================================
	var mtlsSrv *http.Server
	if mtlsEnabled {
		mtlsRouter := createMTLSRouter(certRepo, redis, rpcServer, rateLimitCfg, db, logger)

		tlsConfig, err := buildMTLSTLSConfig(logger)
		if err != nil {
			log.Fatalf("Failed to build mTLS config: %v", err)
		}

		mtlsSrv = &http.Server{
			Addr:         fmt.Sprintf(":%d", mtlsPort),
			Handler:      mtlsRouter,
			TLSConfig:    tlsConfig,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  time.Minute,
		}

		// Start mTLS server
		go func() {
			serverCertPath := getEnvString("POPSIGNER_SERVER_CERT_PATH", "")
			serverKeyPath := getEnvString("POPSIGNER_SERVER_KEY_PATH", "")

			if serverCertPath == "" || serverKeyPath == "" {
				log.Fatal("POPSIGNER_SERVER_CERT_PATH and POPSIGNER_SERVER_KEY_PATH must be set for mTLS")
			}

			logger.Info("mTLS server listening",
				slog.Int("port", mtlsPort),
				slog.String("auth_method", "mtls"),
				slog.String("cert_path", serverCertPath),
			)
			if err := mtlsSrv.ListenAndServeTLS(serverCertPath, serverKeyPath); err != nil && err != http.ErrServerClosed {
				log.Fatalf("mTLS server error: %v", err)
			}
		}()
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("Shutting down RPC Gateway", slog.String("signal", sig.String()))

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown API Key server
	if err := apiKeySrv.Shutdown(ctx); err != nil {
		logger.Error("API Key server shutdown error", slog.Any("error", err))
	}

	// Shutdown mTLS server if enabled
	if mtlsSrv != nil {
		if err := mtlsSrv.Shutdown(ctx); err != nil {
			logger.Error("mTLS server shutdown error", slog.Any("error", err))
		}
	}

	logger.Info("RPC Gateway stopped gracefully")
}

// createAPIKeyRouter creates the router for API Key authentication (Port 8545).
// Used by OP Stack components (op-batcher, op-proposer, op-node).
// Endpoint: POST https://rpc.popsigner.com/ with X-API-Key header
func createAPIKeyRouter(
	apiKeySvc service.APIKeyService,
	redis *database.Redis,
	rpcServer *jsonrpc.Server,
	rateLimitCfg middleware.RPCRateLimitConfig,
	usageRepo repository.UsageRepository,
	db *database.Postgres,
	logger *slog.Logger,
) chi.Router {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logging(logger))
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.CORS())
	r.Use(chimiddleware.Timeout(defaultTimeout))

	// Health check (no auth)
	r.Get("/health", healthHandler())

	// Ready check (verifies dependencies)
	r.Get("/ready", readyHandler(db, redis))

	// Metrics endpoint (no auth, but should be protected at ingress level)
	r.Handle("/metrics", promhttp.Handler())

	// JSON-RPC endpoint at root with API Key auth and rate limiting
	// OP Stack: --signer.endpoint="https://rpc.popsigner.com"
	r.Group(func(r chi.Router) {
		r.Use(middleware.APIKeyAuth(apiKeySvc))
		r.Use(middleware.TrackAPIUsage(usageRepo))
		r.Use(middleware.RPCRateLimit(redis, rateLimitCfg))
		r.Post("/", rpcServer.ServeHTTP)
	})

	return r
}

// createMTLSRouter creates the router for mTLS authentication (Port 8546).
// Used by Arbitrum Nitro components (batch-poster, staker).
// Endpoint: POST https://rpc-mtls.popsigner.com/ with client certificate
func createMTLSRouter(
	certRepo repository.CertificateRepository,
	redis *database.Redis,
	rpcServer *jsonrpc.Server,
	rateLimitCfg middleware.RPCRateLimitConfig,
	db *database.Postgres,
	logger *slog.Logger,
) chi.Router {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logging(logger))
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(defaultTimeout))

	// Health check (no auth)
	r.Get("/health", healthHandler())

	// Ready check (verifies dependencies)
	r.Get("/ready", readyHandler(db, redis))

	// JSON-RPC endpoint at root with mTLS auth and rate limiting
	// Nitro: --*.external-signer.url="https://rpc-mtls.popsigner.com"
	r.Group(func(r chi.Router) {
		r.Use(auth.MTLSOnlyMiddleware(certRepo, logger))
		r.Use(middleware.RPCRateLimit(redis, rateLimitCfg))
		r.Post("/", rpcServer.ServeHTTP)
	})

	return r
}

// buildMTLSTLSConfig creates the TLS configuration for the mTLS server.
func buildMTLSTLSConfig(logger *slog.Logger) (*tls.Config, error) {
	caCertPath := getEnvString("POPSIGNER_CA_CERT_PATH", defaultCACertPath)

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate from %s: %w", caCertPath, err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	// Get client auth type from environment
	clientAuthTypeStr := getEnvString("POPSIGNER_MTLS_CLIENT_AUTH_TYPE", "RequireAndVerifyClientCert")
	clientAuthType := auth.ParseClientAuthType(clientAuthTypeStr)

	logger.Info("mTLS TLS config built",
		slog.String("ca_cert_path", caCertPath),
		slog.String("client_auth_type", clientAuthTypeStr),
	)

	return &tls.Config{
		ClientAuth: clientAuthType,
		ClientCAs:  caPool,
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}, nil
}

// healthHandler returns a health check endpoint.
func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"popsigner-rpc-gateway"}`))
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

// getEnvInt returns an environment variable as int with default.
func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return defaultVal
}

// getEnvString returns an environment variable as string with default.
func getEnvString(key string, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

