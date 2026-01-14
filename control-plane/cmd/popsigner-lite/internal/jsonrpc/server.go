package jsonrpc

import (
	"log/slog"
	"net/http"

	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/keystore"
	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/signer"
)

// ServerConfig holds the configuration for the JSON-RPC server.
type ServerConfig struct {
	Keystore *keystore.Keystore
	Signer   *signer.TransactionSigner
	Logger   *slog.Logger
}

// Server is the JSON-RPC server with all methods registered.
type Server struct {
	handler *Handler
	config  ServerConfig
}

// NewServer creates a new JSON-RPC server with all Ethereum methods registered.
func NewServer(cfg ServerConfig) *Server {
	handler := NewHandler(cfg.Logger)

	// Register health_status (required for OP Stack signer client initialization)
	healthHandler := NewHealthStatusHandler()
	handler.RegisterMethod("health_status", healthHandler.Handle)

	// Register eth_accounts
	ethAccountsHandler := NewEthAccountsHandler(cfg.Keystore)
	handler.RegisterMethod("eth_accounts", ethAccountsHandler.Handle)

	// Register eth_signTransaction (required for op-batcher and op-proposer)
	ethSignTxHandler := NewEthSignTransactionHandler(cfg.Keystore, cfg.Signer)
	handler.RegisterMethod("eth_signTransaction", ethSignTxHandler.Handle)

	// Register eth_sign
	ethSignHandler := NewEthSignHandler(cfg.Keystore, cfg.Signer)
	handler.RegisterMethod("eth_sign", ethSignHandler.HandleEthSign)

	// Register personal_sign
	handler.RegisterMethod("personal_sign", ethSignHandler.HandlePersonalSign)

	// Register OP Stack signer methods (required for op-node P2P sequencer)
	signBlockHandler := NewSignBlockPayloadHandler(cfg.Keystore, cfg.Signer)
	handler.RegisterMethod("opsigner_signBlockPayload", signBlockHandler.Handle)
	handler.RegisterMethod("opsigner_signBlockPayloadV2", signBlockHandler.HandleV2)

	// Log registered methods
	if cfg.Logger != nil {
		cfg.Logger.Info("Registered JSON-RPC methods",
			slog.Any("methods", []string{
				"health_status",
				"eth_accounts",
				"eth_signTransaction",
				"eth_sign",
				"personal_sign",
				"opsigner_signBlockPayload",
				"opsigner_signBlockPayloadV2",
			}),
		)
	}

	return &Server{
		handler: handler,
		config:  cfg,
	}
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// Handler returns the underlying JSON-RPC handler.
// This is useful for testing or adding additional methods.
func (s *Server) Handler() *Handler {
	return s.handler
}

// RegisterMethod registers an additional method handler.
func (s *Server) RegisterMethod(name string, handler MethodHandler) {
	s.handler.RegisterMethod(name, handler)
}

// RegisteredMethods returns a list of registered method names.
func (s *Server) RegisteredMethods() []string {
	return s.handler.RegisteredMethods()
}
