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
)

func main() {
	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("Starting popsigner-lite",
		slog.String("version", version),
	)

	// Create keystore
	ks := keystore.NewKeystore()

	// Load Anvil's deterministic keys
	logger.Info("Loading Anvil deterministic keys...")
	anvilKeys, err := keystore.LoadAnvilKeys()
	if err != nil {
		logger.Error("Failed to load Anvil keys", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Add all Anvil keys to keystore
	for _, key := range anvilKeys {
		if err := ks.AddKey(key); err != nil {
			logger.Error("Failed to add key to keystore",
				slog.String("key_id", key.ID),
				slog.String("error", err.Error()),
			)
			os.Exit(1)
		}
	}

	logger.Info("Loaded Anvil keys",
		slog.Int("count", len(anvilKeys)),
	)

	// List all loaded addresses
	keys := ks.ListKeys()
	addresses := make([]string, len(keys))
	for i, key := range keys {
		addresses[i] = key.Address
	}
	logger.Info("Available addresses",
		slog.Any("addresses", addresses),
	)

	// Get ports from environment or use defaults
	jsonrpcPort := getEnv("JSONRPC_PORT", defaultJSONRPCPort)
	restAPIPort := getEnv("REST_API_PORT", defaultRESTAPIPort)

	// Create signer
	txSigner := signer.NewTransactionSigner()

	// Setup JSON-RPC server
	logger.Info("Setting up JSON-RPC server", slog.String("port", jsonrpcPort))
	rpcServer := jsonrpc.NewServer(jsonrpc.ServerConfig{
		Keystore: ks,
		Signer:   txSigner,
		Logger:   logger,
	})

	// Setup REST API server
	logger.Info("Setting up REST API server", slog.String("port", restAPIPort))
	apiRouter := api.SetupRouter(ks)

	// Create HTTP servers
	rpcHTTPServer := &http.Server{
		Addr:         ":" + jsonrpcPort,
		Handler:      rpcServer,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	restHTTPServer := &http.Server{
		Addr:         ":" + restAPIPort,
		Handler:      apiRouter,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start servers in goroutines
	go func() {
		logger.Info("Starting JSON-RPC server",
			slog.String("address", rpcHTTPServer.Addr),
		)
		if err := rpcHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("JSON-RPC server failed",
				slog.String("error", err.Error()),
			)
			os.Exit(1)
		}
	}()

	go func() {
		logger.Info("Starting REST API server",
			slog.String("address", restHTTPServer.Addr),
		)
		if err := restHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("REST API server failed",
				slog.String("error", err.Error()),
			)
			os.Exit(1)
		}
	}()

	logger.Info("popsigner-lite is ready",
		slog.String("jsonrpc_url", fmt.Sprintf("http://localhost:%s", jsonrpcPort)),
		slog.String("rest_api_url", fmt.Sprintf("http://localhost:%s", restAPIPort)),
	)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down servers...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown JSON-RPC server
	if err := rpcHTTPServer.Shutdown(ctx); err != nil {
		logger.Error("JSON-RPC server shutdown failed",
			slog.String("error", err.Error()),
		)
	} else {
		logger.Info("JSON-RPC server stopped")
	}

	// Shutdown REST API server
	if err := restHTTPServer.Shutdown(ctx); err != nil {
		logger.Error("REST API server shutdown failed",
			slog.String("error", err.Error()),
		)
	} else {
		logger.Info("REST API server stopped")
	}

	logger.Info("popsigner-lite stopped")
}

// getEnv gets an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
