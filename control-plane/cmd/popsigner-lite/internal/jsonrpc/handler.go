package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
)

// MethodHandler is a function that handles a JSON-RPC method call.
type MethodHandler func(ctx context.Context, params json.RawMessage) (interface{}, *Error)

// Handler processes JSON-RPC 2.0 requests.
type Handler struct {
	methods map[string]MethodHandler
	mu      sync.RWMutex
	logger  *slog.Logger
}

// NewHandler creates a new JSON-RPC handler.
func NewHandler(logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		methods: make(map[string]MethodHandler),
		logger:  logger,
	}
}

// RegisterMethod registers a method handler.
func (h *Handler) RegisterMethod(name string, handler MethodHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.methods[name] = handler
	h.logger.Debug("registered JSON-RPC method", slog.String("method", name))
}

// RegisteredMethods returns a list of registered method names.
func (h *Handler) RegisteredMethods() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	methods := make([]string, 0, len(h.methods))
	for name := range h.methods {
		methods = append(methods, name)
	}
	return methods
}

// ServeHTTP implements http.Handler interface.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for development
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only accept POST requests
	if r.Method != http.MethodPost {
		h.writeError(w, nil, ErrInvalidRequest("only POST method is allowed"))
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read request body", slog.String("error", err.Error()))
		h.writeError(w, nil, ErrInvalidRequest("failed to read request body"))
		return
	}
	defer r.Body.Close()

	// Check if it's a batch request (starts with '[')
	if len(body) > 0 && body[0] == '[' {
		h.handleBatchRequest(w, r.Context(), body)
		return
	}

	// Handle single request
	h.handleSingleRequest(w, r.Context(), body)
}

// handleSingleRequest processes a single JSON-RPC request.
func (h *Handler) handleSingleRequest(w http.ResponseWriter, ctx context.Context, body []byte) {
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		h.logger.Error("failed to parse JSON-RPC request", slog.String("error", err.Error()))
		h.writeError(w, nil, ErrParseError("invalid JSON"))
		return
	}

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		h.writeError(w, req.ID, ErrInvalidRequest("jsonrpc must be '2.0'"))
		return
	}

	// Execute method
	result, rpcErr := h.executeMethod(ctx, req.Method, req.Params)

	// Write response
	if rpcErr != nil {
		h.writeError(w, req.ID, rpcErr)
	} else {
		h.writeResult(w, req.ID, result)
	}
}

// handleBatchRequest processes a batch of JSON-RPC requests.
func (h *Handler) handleBatchRequest(w http.ResponseWriter, ctx context.Context, body []byte) {
	var requests []Request
	if err := json.Unmarshal(body, &requests); err != nil {
		h.logger.Error("failed to parse JSON-RPC batch request", slog.String("error", err.Error()))
		h.writeError(w, nil, ErrParseError("invalid JSON"))
		return
	}

	if len(requests) == 0 {
		h.writeError(w, nil, ErrInvalidRequest("batch request cannot be empty"))
		return
	}

	// Process each request
	responses := make([]Response, 0, len(requests))
	for _, req := range requests {
		if req.JSONRPC != "2.0" {
			responses = append(responses, Response{
				JSONRPC: "2.0",
				Error:   ErrInvalidRequest("jsonrpc must be '2.0'"),
				ID:      req.ID,
			})
			continue
		}

		result, rpcErr := h.executeMethod(ctx, req.Method, req.Params)
		if rpcErr != nil {
			responses = append(responses, Response{
				JSONRPC: "2.0",
				Error:   rpcErr,
				ID:      req.ID,
			})
		} else {
			responses = append(responses, Response{
				JSONRPC: "2.0",
				Result:  result,
				ID:      req.ID,
			})
		}
	}

	// Write batch response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(responses)
}

// executeMethod executes a registered method handler.
func (h *Handler) executeMethod(ctx context.Context, method string, params json.RawMessage) (interface{}, *Error) {
	h.mu.RLock()
	handler, exists := h.methods[method]
	h.mu.RUnlock()

	if !exists {
		h.logger.Warn("method not found", slog.String("method", method))
		return nil, ErrMethodNotFound(method)
	}

	h.logger.Debug("executing method", slog.String("method", method))

	// Execute the handler
	result, err := handler(ctx, params)
	if err != nil {
		h.logger.Error("method execution failed",
			slog.String("method", method),
			slog.Int("code", err.Code),
			slog.String("message", err.Message),
		)
		return nil, err
	}

	return result, nil
}

// writeResult writes a successful JSON-RPC response.
func (h *Handler) writeResult(w http.ResponseWriter, id interface{}, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", slog.String("error", err.Error()))
	}
}

// writeError writes an error JSON-RPC response.
func (h *Handler) writeError(w http.ResponseWriter, id interface{}, rpcErr *Error) {
	resp := Response{
		JSONRPC: "2.0",
		Error:   rpcErr,
		ID:      id,
	}

	w.Header().Set("Content-Type", "application/json")

	// Use appropriate HTTP status code based on error
	statusCode := http.StatusOK // JSON-RPC errors are typically returned with 200
	if rpcErr.Code == ErrCodeParse || rpcErr.Code == ErrCodeInvalidRequest {
		statusCode = http.StatusBadRequest
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode error response", slog.String("error", err.Error()))
	}
}

// HealthHandler returns a simple health check handler.
func (h *Handler) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","methods":%d}`, len(h.methods))
	}
}
