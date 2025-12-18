// Package opstack provides OP Stack chain deployment infrastructure.
package opstack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

// SignerFn is the type signature for op-deployer's signing callback.
// type SignerFn func(ctx context.Context, addr common.Address, tx *types.Transaction) (*types.Transaction, error)

// SignerConfig contains configuration for the POPSigner client.
type SignerConfig struct {
	// Endpoint is the POPSigner API endpoint (e.g., "https://rpc.popsigner.com")
	Endpoint string
	// APIKey is the API key for authentication (X-API-Key header)
	APIKey string
	// ChainID is the chain ID for EIP-155 signing
	ChainID *big.Int
	// MaxRetries is the maximum number of retry attempts (default: 3)
	MaxRetries int
	// InitialBackoff is the initial backoff duration (default: 1s)
	InitialBackoff time.Duration
	// MaxBackoff is the maximum backoff duration (default: 10s)
	MaxBackoff time.Duration
	// HTTPClient is an optional custom HTTP client (for testing)
	HTTPClient HTTPClient
}

// HTTPClient interface for HTTP operations (allows mocking in tests).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// POPSigner provides transaction signing via the POPSigner API.
type POPSigner struct {
	config SignerConfig
	client HTTPClient
}

// jsonRPCRequest represents a JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// jsonRPCResponse represents a JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// jsonRPCError represents a JSON-RPC 2.0 error.
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// transactionArgs represents the Ethereum transaction for JSON-RPC.
type transactionArgs struct {
	From                 string  `json:"from"`
	To                   *string `json:"to,omitempty"`
	Gas                  string  `json:"gas"`
	GasPrice             *string `json:"gasPrice,omitempty"`
	MaxFeePerGas         *string `json:"maxFeePerGas,omitempty"`
	MaxPriorityFeePerGas *string `json:"maxPriorityFeePerGas,omitempty"`
	Value                string  `json:"value"`
	Nonce                string  `json:"nonce"`
	Data                 string  `json:"data,omitempty"`
	ChainID              string  `json:"chainId"`
}

// NewPOPSigner creates a new POPSigner instance.
func NewPOPSigner(cfg SignerConfig) *POPSigner {
	// Apply defaults
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 10 * time.Second
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &POPSigner{
		config: cfg,
		client: client,
	}
}

// SignerFn returns a function compatible with op-deployer's SignerFn type.
// This is the main entry point for transaction signing.
func (s *POPSigner) SignerFn() func(ctx context.Context, addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
	return func(ctx context.Context, addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
		return s.SignTransaction(ctx, addr, tx)
	}
}

// SignTransaction signs a transaction via the POPSigner API.
func (s *POPSigner) SignTransaction(ctx context.Context, addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
	// Build transaction args for JSON-RPC
	txArgs := s.buildTransactionArgs(addr, tx)

	// Create JSON-RPC request
	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_signTransaction",
		Params:  []interface{}{txArgs},
		ID:      1,
	}

	// Execute with retry logic
	var lastErr error
	backoff := s.config.InitialBackoff

	for attempt := 0; attempt < s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			// Double the backoff, capped at max
			backoff = min(backoff*2, s.config.MaxBackoff)
		}

		signedTxHex, err := s.doJSONRPCCall(ctx, rpcReq)
		if err != nil {
			lastErr = err
			// Only retry on transient errors
			if !isRetryableError(err) {
				return nil, fmt.Errorf("signing failed: %w", err)
			}
			continue
		}

		// Decode the signed transaction
		signedTx, decodeErr := s.decodeSignedTransaction(signedTxHex)
		if decodeErr != nil {
			return nil, fmt.Errorf("failed to decode signed transaction: %w", decodeErr)
		}

		return signedTx, nil
	}

	return nil, fmt.Errorf("signing failed after %d attempts: %w", s.config.MaxRetries, lastErr)
}

// buildTransactionArgs converts a go-ethereum transaction to JSON-RPC args.
func (s *POPSigner) buildTransactionArgs(addr common.Address, tx *types.Transaction) transactionArgs {
	args := transactionArgs{
		From:    addr.Hex(),
		Gas:     hexutil.EncodeUint64(tx.Gas()),
		Value:   hexutil.EncodeBig(tx.Value()),
		Nonce:   hexutil.EncodeUint64(tx.Nonce()),
		ChainID: hexutil.EncodeBig(s.config.ChainID),
	}

	// Set recipient (nil for contract creation)
	if tx.To() != nil {
		to := tx.To().Hex()
		args.To = &to
	}

	// Set data/input
	if len(tx.Data()) > 0 {
		args.Data = hexutil.Encode(tx.Data())
	}

	// Handle gas pricing based on transaction type
	switch tx.Type() {
	case types.DynamicFeeTxType:
		// EIP-1559 transaction
		maxFee := hexutil.EncodeBig(tx.GasFeeCap())
		maxTip := hexutil.EncodeBig(tx.GasTipCap())
		args.MaxFeePerGas = &maxFee
		args.MaxPriorityFeePerGas = &maxTip
	default:
		// Legacy transaction
		gasPrice := hexutil.EncodeBig(tx.GasPrice())
		args.GasPrice = &gasPrice
	}

	return args
}

// doJSONRPCCall executes a JSON-RPC call to the POPSigner endpoint.
func (s *POPSigner) doJSONRPCCall(ctx context.Context, rpcReq jsonRPCRequest) (string, error) {
	// Marshal request
	reqBody, err := json.Marshal(rpcReq)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", s.config.APIKey)

	// Execute request
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return "", &RetryableError{Err: fmt.Errorf("http request failed: %w", err)}
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", &RetryableError{Err: fmt.Errorf("read response: %w", err)}
	}

	// Check HTTP status
	if resp.StatusCode >= 500 {
		return "", &RetryableError{Err: fmt.Errorf("server error: %d %s", resp.StatusCode, string(body))}
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("client error: %d %s", resp.StatusCode, string(body))
	}

	// Parse JSON-RPC response
	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	// Check for JSON-RPC error
	if rpcResp.Error != nil {
		// Certain error codes are retryable
		if isRetryableRPCError(rpcResp.Error.Code) {
			return "", &RetryableError{Err: fmt.Errorf("JSON-RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)}
		}
		return "", fmt.Errorf("JSON-RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Extract result (signed transaction hex)
	var signedTxHex string
	if err := json.Unmarshal(rpcResp.Result, &signedTxHex); err != nil {
		return "", fmt.Errorf("unmarshal result: %w", err)
	}

	return signedTxHex, nil
}

// decodeSignedTransaction decodes an RLP-encoded signed transaction.
func (s *POPSigner) decodeSignedTransaction(hexEncodedTx string) (*types.Transaction, error) {
	// Remove 0x prefix if present
	hexEncodedTx = strings.TrimPrefix(hexEncodedTx, "0x")

	// Decode hex to bytes
	txBytes, err := hexutil.Decode("0x" + hexEncodedTx)
	if err != nil {
		return nil, fmt.Errorf("decode hex: %w", err)
	}

	// Decode RLP-encoded transaction
	var tx types.Transaction
	if err := tx.UnmarshalBinary(txBytes); err != nil {
		return nil, fmt.Errorf("unmarshal transaction: %w", err)
	}

	return &tx, nil
}

// RetryableError indicates an error that can be retried.
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// isRetryableError checks if an error is retryable.
func isRetryableError(err error) bool {
	var retryErr *RetryableError
	if ok := errors.As(err, &retryErr); ok {
		return true
	}
	return false
}

// isRetryableRPCError checks if a JSON-RPC error code is retryable.
func isRetryableRPCError(code int) bool {
	// -32000 to -32099 are server errors that may be transient
	if code >= -32099 && code <= -32000 {
		return true
	}
	return false
}

