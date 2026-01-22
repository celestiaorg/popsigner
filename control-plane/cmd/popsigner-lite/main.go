package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/api"
	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/jsonrpc"
	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/keystore"
	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/signer"
)

const (
	defaultJSONRPCPort = "8545"
	defaultRESTAPIPort = "3000"
	version            = "1.0.0"

	// HTTP server timeouts
	httpReadTimeout  = 30 * time.Second
	httpWriteTimeout = 30 * time.Second
	httpIdleTimeout  = 60 * time.Second
	shutdownTimeout  = 10 * time.Second
)

// serverManager manages the lifecycle of both HTTP servers.
type serverManager struct {
	logger       *slog.Logger
	keystore     *keystore.Keystore
	rpcServer    *http.Server
	restServer   *http.Server
	jsonrpcPort  string
	restAPIPort  string
	serverErrors chan error
}

func main() {
	logger := setupLogger()

	logger.Info("Starting popsigner-lite", slog.String("version", version))

	sm := &serverManager{
		logger:       logger,
		serverErrors: make(chan error, 2),
	}

	if err := sm.run(); err != nil {
		logger.Error("popsigner-lite failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// run executes the main service lifecycle.
func (sm *serverManager) run() error {
	if err := sm.initializeKeystore(); err != nil {
		return fmt.Errorf("initialize keystore: %w", err)
	}

	sm.setupServers()
	sm.startServers()

	sm.logger.Info("popsigner-lite is ready",
		slog.String("jsonrpc_url", fmt.Sprintf("http://localhost:%s", sm.jsonrpcPort)),
		slog.String("rest_api_url", fmt.Sprintf("http://localhost:%s", sm.restAPIPort)),
	)

	return sm.waitForShutdown()
}

// setupLogger creates and configures the structured logger.
func setupLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// initializeKeystore creates the keystore and loads Anvil keys.
func (sm *serverManager) initializeKeystore() error {
	sm.keystore = keystore.NewKeystore()

	sm.logger.Info("Loading Anvil deterministic keys...")
	anvilKeys, err := keystore.LoadAnvilKeys()
	if err != nil {
		return fmt.Errorf("load Anvil keys: %w", err)
	}

	for _, key := range anvilKeys {
		if err := sm.keystore.AddKey(key); err != nil {
			return fmt.Errorf("add key %s: %w", key.ID, err)
		}
	}

	sm.logger.Info("Loaded Anvil keys", slog.Int("count", len(anvilKeys)))

	keys := sm.keystore.ListKeys()
	addresses := make([]string, len(keys))
	for i, key := range keys {
		addresses[i] = key.Address
	}
	sm.logger.Info("Available addresses", slog.Any("addresses", addresses))

	return nil
}

// setupServers creates both HTTP servers with their handlers.
func (sm *serverManager) setupServers() {
	sm.jsonrpcPort = getEnv("JSONRPC_PORT", defaultJSONRPCPort)
	sm.restAPIPort = getEnv("REST_API_PORT", defaultRESTAPIPort)

	txSigner := signer.NewTransactionSigner()

	sm.logger.Info("Setting up JSON-RPC server", slog.String("port", sm.jsonrpcPort))
	rpcHandler := jsonrpc.NewServer(jsonrpc.ServerConfig{
		Keystore: sm.keystore,
		Signer:   txSigner,
		Logger:   sm.logger,
	})

	sm.rpcServer = &http.Server{
		Addr:         ":" + sm.jsonrpcPort,
		Handler:      rpcHandler,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
		IdleTimeout:  httpIdleTimeout,
	}

	sm.logger.Info("Setting up REST API server", slog.String("port", sm.restAPIPort))
	apiRouter := api.SetupRouter(sm.keystore)

	sm.restServer = &http.Server{
		Addr:         ":" + sm.restAPIPort,
		Handler:      apiRouter,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
		IdleTimeout:  httpIdleTimeout,
	}
}

// startServers starts both HTTP servers in separate goroutines with error propagation.
func (sm *serverManager) startServers() {
	go func() {
		sm.logger.Info("Starting JSON-RPC server", slog.String("address", sm.rpcServer.Addr))
		if err := sm.rpcServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sm.serverErrors <- fmt.Errorf("JSON-RPC server: %w", err)
		}
	}()

	go func() {
		sm.logger.Info("Starting REST API server", slog.String("address", sm.restServer.Addr))
		if err := sm.restServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sm.serverErrors <- fmt.Errorf("REST API server: %w", err)
		}
	}()
}

// waitForShutdown waits for either a signal or server error, then performs graceful shutdown.
func (sm *serverManager) waitForShutdown() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		sm.logger.Info("Received shutdown signal")
	case err := <-sm.serverErrors:
		sm.logger.Error("Server error", slog.String("error", err.Error()))
		return err
	}

	return sm.shutdown()
}

// shutdown performs graceful shutdown of both servers.
func (sm *serverManager) shutdown() error {
	sm.logger.Info("Shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	var shutdownErr error

	if err := sm.rpcServer.Shutdown(ctx); err != nil {
		sm.logger.Error("JSON-RPC server shutdown failed", slog.String("error", err.Error()))
		shutdownErr = fmt.Errorf("RPC shutdown: %w", err)
	} else {
		sm.logger.Info("JSON-RPC server stopped")
	}

	if err := sm.restServer.Shutdown(ctx); err != nil {
		sm.logger.Error("REST API server shutdown failed", slog.String("error", err.Error()))
		if shutdownErr != nil {
			shutdownErr = fmt.Errorf("%w; REST shutdown: %w", shutdownErr, err)
		} else {
			shutdownErr = fmt.Errorf("REST shutdown: %w", err)
		}
	} else {
		sm.logger.Info("REST API server stopped")
	}

	sm.logger.Info("popsigner-lite stopped")
	return shutdownErr
}

// getEnv gets an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
